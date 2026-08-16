#!/usr/bin/env bash
#
# chaos.sh — P4.5 chaos harness for the task queue.
#
# Hammers a running cluster with REPEATED RANDOM KILLS of both workers and
# schedulers while sustained load runs and a cron job fires, then asserts the
# system's invariants from Redis ground truth after everything drains:
#
#     ready == 0 && processing == 0 && delayed == 0
#     submitted == processed + failed            (zero loss — also catches double-processing)
#     cron kept firing through the chaos (N > 0 slots, all terminal once drained)
#
# It exercises three recovery mechanisms at once:
#   * worker kill    -> the reaper reclaims the dead node's leases (P1/P2)
#   * scheduler kill -> leader failover; a standby scheduler takes the crown (P4.1-3)
#   * cron running   -> the leader keeps materializing instances, exactly once (P4.4)
#
# Killed containers auto-revive via compose `restart: unless-stopped`. The script
# NEVER kills the last running worker or the last running scheduler, so there is
# always >=1 of each. Prints a PASS/FAIL summary and exits non-zero on violation.
#
# Usage (all config via env, with defaults):
#     ./scripts/chaos.sh
#     CHAOS_DURATION=10m ./scripts/chaos.sh
#     CHAOS_INTERVAL=15 CHAOS_RATE=60 ./scripts/chaos.sh
#
# Env vars:
#     CHAOS_DURATION      total load+chaos duration (Go duration; default 5m)
#     CHAOS_INTERVAL      seconds between kills (default 20)
#     CHAOS_RATE          loadgen submissions/sec (default 40)
#     CHAOS_MIX           loadgen task mix (default hash:50,sleep:50)
#     CHAOS_WORKERS       worker replicas (default 3)
#     CHAOS_SCHEDULERS    scheduler replicas (default 2)
#     CHAOS_CRON          cron schedule to run through the chaos (default "*/10 * * * * *"; "" disables)
#     CHAOS_DRAIN_TIMEOUT seconds to wait for full drain after load stops (default 300)
#     CHAOS_ENSURE_UP     1 = compose up -d at the requested scale first (default 1)
#     CHAOS_API           API base URL (default http://localhost:8080)
#
set -euo pipefail

CHAOS_DURATION="${CHAOS_DURATION:-5m}"
CHAOS_INTERVAL="${CHAOS_INTERVAL:-20}"
CHAOS_RATE="${CHAOS_RATE:-40}"
CHAOS_MIX="${CHAOS_MIX:-hash:50,sleep:50}"
CHAOS_WORKERS="${CHAOS_WORKERS:-3}"
CHAOS_SCHEDULERS="${CHAOS_SCHEDULERS:-2}"
CHAOS_CRON="${CHAOS_CRON:-*/10 * * * * *}"
CHAOS_DRAIN_TIMEOUT="${CHAOS_DRAIN_TIMEOUT:-300}"
CHAOS_ENSURE_UP="${CHAOS_ENSURE_UP:-1}"
CHAOS_API="${CHAOS_API:-http://localhost:8080}"

PROJECT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_DIR"
COMPOSE="docker compose"

log()  { printf '\033[36m[chaos %s]\033[0m %s\n' "$(date +%H:%M:%S)" "$*"; }
kill_log() { printf '\033[33m[chaos %s]\033[0m %s\n' "$(date +%H:%M:%S)" "$*"; }
err()  { printf '\033[31m[chaos %s] %s\033[0m\n' "$(date +%H:%M:%S)" "$*" >&2; }

rc()     { $COMPOSE exec -T redis redis-cli "$@" 2>/dev/null | tr -d '[:space:]' || true; }
metric() { local v; v="$(rc HGET taskqueue:metrics "$1")"; echo "${v:-0}"; }
zcard()  { local v; v="$(rc ZCARD "$1")"; echo "${v:-0}"; }
leader() { curl -fsS "$CHAOS_API/api/leader" 2>/dev/null | sed -n 's/.*"leader_id":"\([^"]*\)".*/\1/p'; }

to_seconds() {
  local d="$1"
  case "$d" in
    *h) echo $(( ${d%h} * 3600 ));;
    *m) echo $(( ${d%m} * 60 ));;
    *s) echo "${d%s}";;
    *)  echo "$d";;
  esac
}
DURATION_SEC="$(to_seconds "$CHAOS_DURATION")"

# --- preflight -------------------------------------------------------------
if [ "$CHAOS_ENSURE_UP" = "1" ]; then
  log "ensuring stack up: scheduler=$CHAOS_SCHEDULERS worker=$CHAOS_WORKERS ..."
  $COMPOSE up -d --scale scheduler="$CHAOS_SCHEDULERS" --scale worker="$CHAOS_WORKERS" \
    redis backend scheduler worker >/dev/null
fi
if ! curl -fsS "$CHAOS_API/api/health" >/dev/null 2>&1; then
  err "backend not reachable at $CHAOS_API"
  exit 1
fi
# wait for a leader to be elected
for _ in $(seq 1 20); do [ -n "$(leader)" ] && break; sleep 1; done
[ -z "$(leader)" ] && { err "no leader elected — is a scheduler running?"; exit 1; }

log "config: duration=$CHAOS_DURATION (${DURATION_SEC}s) interval=${CHAOS_INTERVAL}s rate=$CHAOS_RATE mix=$CHAOS_MIX cron='${CHAOS_CRON:-off}'"

# --- baseline --------------------------------------------------------------
log "flushing to a clean baseline ..."
curl -fsS -X DELETE "$CHAOS_API/api/flush" >/dev/null
sleep 1

if [ -n "$CHAOS_CRON" ]; then
  log "registering cron job 'chaos-cron' ($CHAOS_CRON) ..."
  curl -fsS -X POST "$CHAOS_API/api/cron" -H 'Content-Type: application/json' \
    -d "{\"id\":\"chaos-cron\",\"schedule\":\"$CHAOS_CRON\",\"task\":{\"type\":\"hash\",\"payload\":{\"rounds\":2000},\"priority\":5,\"max_retries\":3},\"enabled\":true}" >/dev/null
fi

START_EPOCH="$(date +%s)"

# --- start the load (background) -------------------------------------------
log "starting loadgen (rate=$CHAOS_RATE for $CHAOS_DURATION) ..."
$COMPOSE run --rm -d loadgen -rate "$CHAOS_RATE" -duration "$CHAOS_DURATION" -mix "$CHAOS_MIX" >/dev/null

# running container IDs for a scaled service
running_ids() { $COMPOSE ps -q --status running "$1" 2>/dev/null; }

# --- chaos loop ------------------------------------------------------------
KILLS=0; WKILLS=0; SKILLS=0; MIGRATIONS=0
LAST_LEADER="$(leader)"
DEADLINE=$(( START_EPOCH + DURATION_SEC ))

while [ "$(date +%s)" -lt "$DEADLINE" ]; do
  sleep "$CHAOS_INTERVAL"
  [ "$(date +%s)" -ge "$DEADLINE" ] && break

  # track leader migrations
  CUR="$(leader)"
  if [ -n "$CUR" ] && [ "$CUR" != "$LAST_LEADER" ]; then
    MIGRATIONS=$(( MIGRATIONS + 1 )); log "leader migrated -> ${CUR%%-*}"; LAST_LEADER="$CUR"
  fi

  # choose a victim service at random, but only if it has >=2 running (keep one alive)
  if [ $(( RANDOM % 2 )) -eq 0 ]; then SVC=worker; OTHER=scheduler; else SVC=scheduler; OTHER=worker; fi
  MAP_SVC() {
    local ids; ids="$(running_ids "$1")"
    local n; n="$(printf '%s\n' "$ids" | grep -c . || true)"
    if [ "$n" -ge 2 ]; then printf '%s' "$ids"; return 0; fi
    return 1
  }
  IDS=""
  if IDS="$(MAP_SVC "$SVC")"; then :; elif IDS="$(MAP_SVC "$OTHER")"; then SVC="$OTHER"; else
    log "skip kill — only one worker and one scheduler alive"; continue
  fi

  # pick a random running container of the chosen service (portable: no mapfile — macOS bash 3.2)
  # shellcheck disable=SC2206
  ARR=($IDS)
  VICTIM="${ARR[$(( RANDOM % ${#ARR[@]} ))]}"
  kill_log "KILL $SVC ($(docker inspect --format '{{.Name}}' "$VICTIM" 2>/dev/null | sed 's,^/,,')) — auto-revives via restart policy"
  docker kill "$VICTIM" >/dev/null 2>&1 || true
  KILLS=$(( KILLS + 1 ))
  if [ "$SVC" = worker ]; then WKILLS=$(( WKILLS + 1 )); else SKILLS=$(( SKILLS + 1 )); fi
done

log "chaos window over: $KILLS kills ($WKILLS worker, $SKILLS scheduler), $MIGRATIONS leader migrations"

# --- stop cron, let load finish, drain -------------------------------------
[ -n "$CHAOS_CRON" ] && { curl -fsS -X DELETE "$CHAOS_API/api/cron/chaos-cron" >/dev/null 2>&1 || true; log "deleted chaos-cron"; }
log "ensuring all replicas back for drain ..."
$COMPOSE up -d --scale scheduler="$CHAOS_SCHEDULERS" --scale worker="$CHAOS_WORKERS" scheduler worker >/dev/null 2>&1 || true
sleep 3

log "waiting for drain (timeout ${CHAOS_DRAIN_TIMEOUT}s) ..."
DRAIN_OK=0
DDEADLINE=$(( $(date +%s) + CHAOS_DRAIN_TIMEOUT ))
while :; do
  R="$(zcard taskqueue:ready)"; P="$(zcard taskqueue:processing)"; D="$(zcard taskqueue:delayed)"
  if [ "$R" = 0 ] && [ "$P" = 0 ] && [ "$D" = 0 ]; then DRAIN_OK=1; break; fi
  [ "$(date +%s)" -ge "$DDEADLINE" ] && break
  log "  draining… ready=$R processing=$P delayed=$D"
  sleep 3
done

# --- ground truth ----------------------------------------------------------
SUBMITTED="$(metric submitted)"; PROCESSED="$(metric processed)"; FAILED="$(metric failed)"
RETRIES="$(metric retries)"; DLQ="$(rc LLEN taskqueue:deadletter)"; DLQ="${DLQ:-0}"
R="$(zcard taskqueue:ready)"; P="$(zcard taskqueue:processing)"; D="$(zcard taskqueue:delayed)"
TERMINAL=$(( PROCESSED + FAILED ))
ELAPSED=$(( $(date +%s) - START_EPOCH ))

# cron slots fired (unique instance records)
CRON_SLOTS=0
if [ -n "$CHAOS_CRON" ]; then
  CRON_SLOTS="$($COMPOSE exec -T redis redis-cli --scan --pattern 'taskqueue:task:chaos-cron-*' 2>/dev/null | grep -c . || true)"
  CRON_SLOTS="${CRON_SLOTS:-0}"
fi

RECLAIMS="n/a"
if RCL="$(curl -fsS -G http://localhost:9090/api/v1/query --data-urlencode 'query=sum(reaper_reclaims_total)' 2>/dev/null)"; then
  RCL="$(printf '%s' "$RCL" | sed -n 's/.*"value":\[[0-9.]*,"\([0-9.]*\)"\].*/\1/p')"
  [ -n "$RCL" ] && RECLAIMS="$RCL"
fi

# --- report ----------------------------------------------------------------
echo
echo "──────────────── CHAOS RESULT ────────────────"
printf '  elapsed (incl. drain)  : %ss\n' "$ELAPSED"
printf '  kills                  : %s  (%s worker, %s scheduler)\n' "$KILLS" "$WKILLS" "$SKILLS"
printf '  leader migrations      : %s\n' "$MIGRATIONS"
printf '  reaper reclaims        : %s\n' "$RECLAIMS"
printf '  cron slots fired       : %s\n' "$CRON_SLOTS"
printf '  submitted (accepted)   : %s\n' "$SUBMITTED"
printf '  processed (completed)  : %s\n' "$PROCESSED"
printf '  failed (dead-lettered) : %s   (DLQ len: %s)\n' "$FAILED" "$DLQ"
printf '  retries (re-attempts)  : %s\n' "$RETRIES"
printf '  terminal (proc+failed) : %s\n' "$TERMINAL"
printf '  queues at end          : ready=%s processing=%s delayed=%s\n' "$R" "$P" "$D"
echo "──────────────────────────────────────────────"

FAIL=0
[ "$DRAIN_OK" != 1 ] && { err "DRAIN TIMEOUT — queue did not empty within ${CHAOS_DRAIN_TIMEOUT}s"; FAIL=1; }
if [ "$SUBMITTED" != "$TERMINAL" ]; then
  err "ZERO-LOSS VIOLATED — submitted ($SUBMITTED) != processed+failed ($TERMINAL); delta=$(( SUBMITTED - TERMINAL ))"
  FAIL=1
fi
if [ -n "$CHAOS_CRON" ] && [ "$CRON_SLOTS" -lt 1 ]; then
  err "CRON DID NOT FIRE — expected >=1 chaos-cron instance"
  FAIL=1
fi

if [ "$FAIL" = 0 ]; then
  printf '\033[32m  ✔ PASS — survived %s kills / %s leader migrations: zero loss (%s == %s + %s), cron fired %s slots, queues drained\033[0m\n' \
    "$KILLS" "$MIGRATIONS" "$SUBMITTED" "$PROCESSED" "$FAILED" "$CRON_SLOTS"
  exit 0
else
  printf '\033[31m  FAIL — see errors above\033[0m\n'
  exit 1
fi
