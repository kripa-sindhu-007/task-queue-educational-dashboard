#!/usr/bin/env bash
#
# soak.sh — P3.7 zero-loss soak harness for the task queue.
#
# Drives a sustained load with cmd/loadgen, optionally kills+restores a worker
# mid-run (chaos), waits for the queue to fully drain, then asserts the
# zero-loss invariant from Redis ground truth:
#
#     ready == 0 && processing == 0 && delayed == 0
#     AND submitted == processed + failed        (accepted == completed + dead-lettered)
#
# `submitted` counts only ACCEPTED submissions (backpressure sheds before the
# counter increments), so it is the correct denominator even with MAX_QUEUE_DEPTH
# active. `retries` are re-attempts, not terminal, so they are NOT in the equation.
#
# Prints a PASS/FAIL summary and exits non-zero on FAIL. Reproducible: run it for
# 5 minutes to validate, or 30+ minutes for a publishable benchmark number.
#
# Usage (all config via env vars, with defaults):
#     ./scripts/soak.sh
#     SOAK_DURATION=30m SOAK_RATE=250 ./scripts/soak.sh
#     SOAK_CHAOS=1 SOAK_DURATION=5m ./scripts/soak.sh      # kill+restore a worker mid-run
#
# Env vars:
#     SOAK_RATE          submissions/sec (default 250; keep below saturation for a steady-state soak)
#     SOAK_DURATION      loadgen -duration (Go duration, e.g. 5m, 30m; default 5m)
#     SOAK_MIX           task-type mix (default hash:50,sleep:50)
#     SOAK_CONCURRENCY   loadgen submitters (default 80)
#     SOAK_WORKERS       worker replicas to run (default 3)
#     SOAK_CHAOS         1 = kill+restore one worker mid-run (default 0)
#     SOAK_CHAOS_KILL_AFTER  seconds into the run before the kill (default 30)
#     SOAK_CHAOS_DOWN    seconds the worker stays dead before restore (default 25)
#     SOAK_DRAIN_TIMEOUT seconds to wait for full drain after load stops (default 300)
#     SOAK_ENSURE_UP     1 = `compose up -d` the stack at the requested scale first (default 1)
#     SOAK_API           API base URL (default http://localhost:8080)
#
set -euo pipefail

SOAK_RATE="${SOAK_RATE:-250}"
SOAK_DURATION="${SOAK_DURATION:-5m}"
SOAK_MIX="${SOAK_MIX:-hash:50,sleep:50}"
SOAK_CONCURRENCY="${SOAK_CONCURRENCY:-80}"
SOAK_WORKERS="${SOAK_WORKERS:-3}"
SOAK_CHAOS="${SOAK_CHAOS:-0}"
SOAK_CHAOS_KILL_AFTER="${SOAK_CHAOS_KILL_AFTER:-30}"
SOAK_CHAOS_DOWN="${SOAK_CHAOS_DOWN:-25}"
SOAK_DRAIN_TIMEOUT="${SOAK_DRAIN_TIMEOUT:-300}"
SOAK_ENSURE_UP="${SOAK_ENSURE_UP:-1}"
SOAK_API="${SOAK_API:-http://localhost:8080}"

PROJECT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_DIR"
COMPOSE="docker compose"

log() { printf '\033[36m[soak %s]\033[0m %s\n' "$(date +%H:%M:%S)" "$*"; }
err() { printf '\033[31m[soak %s] %s\033[0m\n' "$(date +%H:%M:%S)" "$*" >&2; }

# redis-cli inside the redis container; never let a transient failure kill the script.
rc() { $COMPOSE exec -T redis redis-cli "$@" 2>/dev/null | tr -d '[:space:]' || true; }
# HGET a metrics field, defaulting empty/nil to 0.
metric() { local v; v="$(rc HGET taskqueue:metrics "$1")"; echo "${v:-0}"; }
zcard()  { local v; v="$(rc ZCARD "$1")"; echo "${v:-0}"; }

# --- preflight -------------------------------------------------------------
if [ "$SOAK_ENSURE_UP" = "1" ]; then
  log "ensuring stack up at worker scale=$SOAK_WORKERS ..."
  $COMPOSE up -d --scale worker="$SOAK_WORKERS" redis backend worker >/dev/null
fi
if ! curl -fsS "$SOAK_API/api/health" >/dev/null 2>&1; then
  err "backend not reachable at $SOAK_API — start it with: docker compose up -d --scale worker=$SOAK_WORKERS redis backend worker"
  exit 1
fi

log "config: rate=$SOAK_RATE/s duration=$SOAK_DURATION mix=$SOAK_MIX conc=$SOAK_CONCURRENCY workers=$SOAK_WORKERS chaos=$SOAK_CHAOS"

# --- baseline: flush so counters start at zero -----------------------------
log "flushing to a clean baseline ..."
curl -fsS -X DELETE "$SOAK_API/api/flush" >/dev/null
sleep 1

START_EPOCH="$(date +%s)"

# --- run the load ----------------------------------------------------------
run_loadgen() {
  $COMPOSE run --rm loadgen \
    -rate "$SOAK_RATE" -duration "$SOAK_DURATION" \
    -mix "$SOAK_MIX" -concurrency "$SOAK_CONCURRENCY"
}

if [ "$SOAK_CHAOS" = "1" ]; then
  log "starting loadgen (background) for chaos run ..."
  run_loadgen &
  LG_PID=$!

  sleep "$SOAK_CHAOS_KILL_AFTER"
  WID="$($COMPOSE ps -q worker 2>/dev/null | head -1)"
  if [ -n "$WID" ]; then
    log "CHAOS: killing worker $WID (down ${SOAK_CHAOS_DOWN}s) ..."
    docker kill "$WID" >/dev/null || true
    sleep "$SOAK_CHAOS_DOWN"
    log "CHAOS: restoring worker scale=$SOAK_WORKERS ..."
    $COMPOSE up -d --scale worker="$SOAK_WORKERS" worker >/dev/null || true
  else
    err "CHAOS: no worker container found to kill; continuing without chaos"
  fi

  wait "$LG_PID"
else
  log "starting loadgen (foreground) ..."
  run_loadgen
fi

# --- wait for full drain ---------------------------------------------------
log "load stopped; waiting for drain (timeout ${SOAK_DRAIN_TIMEOUT}s) ..."
DRAIN_OK=0
DEADLINE=$(( $(date +%s) + SOAK_DRAIN_TIMEOUT ))
while :; do
  R="$(zcard taskqueue:ready)"; P="$(zcard taskqueue:processing)"; D="$(zcard taskqueue:delayed)"
  if [ "$R" = 0 ] && [ "$P" = 0 ] && [ "$D" = 0 ]; then DRAIN_OK=1; break; fi
  if [ "$(date +%s)" -ge "$DEADLINE" ]; then break; fi
  log "  draining… ready=$R processing=$P delayed=$D"
  sleep 3
done

# --- read final ground truth ----------------------------------------------
SUBMITTED="$(metric submitted)"
PROCESSED="$(metric processed)"
FAILED="$(metric failed)"
RETRIES="$(metric retries)"
DLQ="$(rc LLEN taskqueue:deadletter)"; DLQ="${DLQ:-0}"
R="$(zcard taskqueue:ready)"; P="$(zcard taskqueue:processing)"; D="$(zcard taskqueue:delayed)"
ELAPSED=$(( $(date +%s) - START_EPOCH ))
TERMINAL=$(( PROCESSED + FAILED ))

# Optional: reaper reclaims from Prometheus (chaos signal), if reachable.
RECLAIMS="n/a"
if RCL="$(curl -fsS -G http://localhost:9090/api/v1/query --data-urlencode 'query=sum(reaper_reclaims_total)' 2>/dev/null)"; then
  RCL="$(printf '%s' "$RCL" | sed -n 's/.*"value":\[[0-9.]*,"\([0-9.]*\)"\].*/\1/p')"
  [ -n "$RCL" ] && RECLAIMS="$RCL"
fi

# --- report ----------------------------------------------------------------
echo
echo "──────────────── SOAK RESULT ────────────────"
printf '  elapsed (incl. drain) : %ss\n' "$ELAPSED"
printf '  submitted (accepted)  : %s\n' "$SUBMITTED"
printf '  processed (completed) : %s\n' "$PROCESSED"
printf '  failed (dead-lettered): %s   (DLQ len: %s)\n' "$FAILED" "$DLQ"
printf '  retries (re-attempts) : %s\n' "$RETRIES"
printf '  reaper reclaims       : %s\n' "$RECLAIMS"
printf '  terminal (proc+failed): %s\n' "$TERMINAL"
printf '  queues at end         : ready=%s processing=%s delayed=%s\n' "$R" "$P" "$D"
echo "──────────────────────────────────────────────"

FAIL=0
if [ "$DRAIN_OK" != 1 ]; then
  err "DRAIN TIMEOUT — queue did not empty within ${SOAK_DRAIN_TIMEOUT}s"
  FAIL=1
fi
if [ "$SUBMITTED" != "$TERMINAL" ]; then
  err "ZERO-LOSS VIOLATED — submitted ($SUBMITTED) != processed+failed ($TERMINAL); delta=$(( SUBMITTED - TERMINAL ))"
  FAIL=1
fi

if [ "$FAIL" = 0 ]; then
  printf '\033[32m  ✔ PASS — zero loss: %s submitted == %s completed + %s dead-lettered\033[0m\n' "$SUBMITTED" "$PROCESSED" "$FAILED"
  exit 0
else
  printf '\033[31m  FAIL — see errors above\033[0m\n'
  exit 1
fi
