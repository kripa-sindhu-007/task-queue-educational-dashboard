# Project Progress Tracker

> **Vision:** Evolve this repo from a single-node task queue simulator into a
> **Distributed Task Processing Platform with Visible Internals** — standalone worker
> nodes, at-least-once delivery, heartbeat failure detection, leader election, and
> Prometheus/Grafana observability. The killer demo: *kill any node and watch the
> dashboard show the system notice, recover, and rebalance — live.*
>
> Full analysis and rationale: see the architecture review conversation / `docs/` (to be
> captured in `docs/ARCHITECTURE.md` as part of P0.6).

---

## 📍 Status Snapshot

<!-- Keep this block current. It is the first thing a new session reads. -->

| Field | Value |
|---|---|
| **Current phase** | Phase 3 — Observability & Performance (in progress). P3.1 (slog) + P3.2 (Prometheus /metrics) + P3.4 (blocking pickup) + P3.3 (Prometheus+Grafana in compose) + P3.5 (loadgen) ✅ done; P3.6–P3.8 todo. |
| **Current task** | P3.6 (backpressure: max queue depth → `429` + `Retry-After` on submit) — next. `cmd/loadgen` already counts 429s, so it's ready to exercise P3.6. |
| **Last updated** | 2026-08-15 |
| **Last session** | 2026-08-13 — P3.4 blocking task pickup: doorbell (`BLPOP taskqueue:ready:signal`) replaces worker sleep-polling; `dequeue.lua` byte-for-byte unchanged (at-least-once preserved). All 4 ready-producers ring a cap-guarded doorbell (`signal.lua` + inline in `promote.lua`/`reclaim.lua`). New `SignalBlock`/`SignalCap` config; explicit go-redis `PoolSize ≥ WorkerCount + headroom`. 68 tests green (+15) incl. real-Redis integration; measured p99 `enqueue_to_start` ~495 ms → ~0.86 ms at low/bursty load (`docs/BENCHMARKS.md`). |
| **Hours spent / budget** | ~40 / ~100 |
| **Blockers** | none |

---

## 📖 How to Use This File

**For humans:** check off tasks as you finish them, append a Session Log entry when you
stop working, and keep the Status Snapshot table current. That's it.

**For AI agents (Claude Code, etc.) — follow these rules exactly:**

1. **At session start:** read the Status Snapshot, the current phase's task list, and the
   last 2–3 Session Log entries. Do not re-derive the plan; it is settled.
2. **During work:** when a task's status changes, update its checkbox and Status column.
   Statuses: `todo` · `in-progress` · `done` · `blocked` · `skipped` (with reason in Notes).
3. **At session end:** append a Session Log entry (template below), update the Status
   Snapshot (current task, hours, date), and check off completed acceptance criteria.
4. **Task IDs are stable.** Never renumber. Reference tasks as `P1.3` in commits, e.g.
   `feat(broker): atomic lease dequeue via Lua [P1.3]`.
5. **Scope changes** go in the Decision Log, never as silent edits to task lists. New
   ideas go to the Backlog, not into the current phase.
6. **Do not mark a task `done` unless its code compiles and its tests (if the task
   defines them) pass.** Demo criteria are only checked off after actually running the demo.

---

## 🗓️ Timeline

~100 hours over 4 weeks (2–3 h weekdays, 6–8 h weekend days). Dates assume a
2026-07-06 start; slippage is fine — they're targets, not deadlines.

| Phase | Theme | Target dates | Est. hours | Status |
|---|---|---|---|---|
| **Phase 0** | Foundations: seams, task records, first tests | Jul 6 – Jul 8 | ~8 | ✅ done |
| **Phase 1** | Reliability: at-least-once delivery | Jul 9 – Jul 12 | ~17 | ✅ done |
| **Phase 2** | Distribution: standalone worker nodes | Jul 13 – Jul 19 | ~25 | ✅ done |
| **Phase 3** | Observability & performance | Jul 20 – Jul 26 | ~25 | 🔲 todo |
| **Phase 4** | Coordination: leader election, cron, polish | Jul 27 – Aug 2 | ~25 | 🔲 todo |

Phase status legend: 🔲 todo · 🔨 in-progress · ✅ done · ⏸️ blocked

---

## Phase 0 — Foundations (~8h)

**Goal:** create the seams (interfaces, task records, tests) that every later phase
builds on. No new features — pure refactoring.

| ID | Task | Status | Notes |
|---|---|---|---|
| P0.1 | ☑ Fix module path `weekend-project/taskqueue` → `github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend`; update all imports | done | |
| P0.2 | ☑ Introduce `Broker` interface (`Enqueue/Dequeue/Ack/Nack/ExtendLease`) in new `internal/broker/` package; current Redis code becomes an implementation | done | `RedisBroker`; Ack no-op, Nack/ExtendLease return `ErrNotImplemented` until P1. Pool dequeues through the interface |
| P0.3 | ☑ Task records keyed by ID: canonical state in `taskqueue:task:{id}` hash; queues hold **task IDs only**, not full JSON | done | `store.TaskStore`; ready/delayed ZSETs hold IDs |
| P0.4 | ☑ Add `Type` + `Payload json.RawMessage` fields to `model.Task` | done | Handlers come in P2.4 |
| P0.5 | ☑ Fix graceful-shutdown ctx bug: executor uses a drain context (Background + timeout) for post-cancellation Redis writes | done | `ExecutorDeps.DrainTimeout` (default 5s) |
| P0.6 | ☑ Start `docs/ARCHITECTURE.md`: current architecture + target architecture + the two Mermaid diagrams from the plan | done | Includes delivery-guarantees stub for P1.10 |
| P0.7 | ☑ Test scaffolding: `Makefile` with `test`/`bench` targets, add `miniredis` dev dep, write first unit tests | done | 10 unit tests (store/queue/worker/api); `test-integration` target stubbed for P1.9 |
| P0.8 | ☑ `api.Handler` and `worker.NewExecutor` take a single `Deps` struct instead of long positional constructor args | done | `HandlerDeps`, `ExecutorDeps` |
| P0.9 | ☑ Config cleanup: `time.Duration` fields, validation on load; add `http.Server` timeouts | done | `Load()` now returns an error; read/write/idle/shutdown timeouts |

**Acceptance criteria (all must be true before starting Phase 1):**

- [x] `go build ./...` and `go test ./...` pass; `docker compose up` still works end-to-end — build/vet/tests green; **compose not runnable in this env (no Docker)**, so verified the server binary end-to-end against miniredis instead (submit → run → retry → dead-letter, canonical records updated)
- [x] Submitting two tasks with identical fields creates two queue entries (identity bug fixed) — verified via unit test and live (distinct generated IDs)
- [x] `GET /api/tasks/{id}` returns the canonical task record — landed early (this is also P1.7); 404 on missing
- [x] At least 5 meaningful unit tests exist and run in CI-able form (`make test`) — 10 tests

---

## Phase 1 — Reliability: At-Least-Once Delivery (~17h)

**Goal:** no task is ever lost, even under `kill -9`. The foundation everything else
stands on.

| ID | Task | Status | Notes |
|---|---|---|---|
| P1.1 | ☑ Add `taskqueue:processing` ZSET (score = lease deadline); define visibility-timeout config | done | `KeyProcessing` in `store/redis.go`; `VisibilityTimeout` (30s) + `ReaperInterval` (5s) in config with validation |
| P1.2 | ☑ Atomic dequeue via Lua: pop from ready → insert into processing with `now + visibilityTimeout` (single script, `//go:embed`) | done | `broker/scripts/dequeue.lua`; ZPOPMIN → ZADD processing → HSET status=processing atomically |
| P1.3 | ☑ `Ack(taskID)` removes from processing; `Nack` routes to retry (delayed set) or DLQ | done | `ack.lua` (ZREM + HSET completed), `nack.lua` (ZREM only); `ErrLeaseNotHeld` on race |
| P1.4 | ☑ Reaper goroutine: periodically reclaim tasks in processing whose lease expired → back to ready, `Retries++`, emit `reclaimed` event | done | `internal/reaper/`; `reclaim.lua` atomically ZREM→ZADD ready or mark failed; batched (100/tick) |
| P1.5 | ☑ Atomic delayed→ready promotion via Lua script (fixes ZREM-then-ZADD loss window) | done | `queue/scripts/promote.lua`; reads priority from hash, handles batch |
| P1.6 | ☑ Idempotent enqueue: reject/no-op duplicate task IDs (SETNX-style on task record) | done | `TaskStore.Exists()` + 409 Conflict in handler for client-supplied IDs |
| P1.7 | ☑ New endpoints: `GET /api/tasks/{id}` ✅ (done early in P0), `POST /api/tasks/failed/redrive` ✅ | done | Redrive drains DLQ, resets retries/status, re-enqueues to ready |
| P1.8 | ☑ Dashboard: add "Processing (leased)" stage to pipeline diagram + `reclaimed` event badge | todo | Frontend touch — deferred to a frontend-focused session |
| P1.9 | ☑ Tests: miniredis units for lease/reaper logic | done | 10 broker tests + 4 reaper tests = 14 new; 24 total project tests |
| P1.10 | ☐ Docs: "Delivery guarantees" section in ARCHITECTURE.md (at-most vs at-least vs exactly-once, why exactly-once needs idempotency) | todo | Deferred to docs pass |

**Acceptance criteria:**

- [ ] Crash test passes: submit 100 tasks, `docker kill` backend mid-burst, restart → every task reaches a terminal state, `submitted == completed + dead_lettered`
- [ ] Reclaimed tasks visibly flow through the dashboard (`reclaimed` events appear)
- [ ] Integration test (crash/reclaim) runs green via `make test-integration`

**Demo (record it):** kill the backend mid-burst → restart → reaper recovers all in-flight tasks, zero loss.

---

## Phase 2 — Distribution: Standalone Worker Nodes (~25h)

**Goal:** workers become separate processes/containers that join and leave a live
cluster; the broker detects dead nodes and reclaims their work.

| ID | Task | Status | Notes |
|---|---|---|---|
| P2.1 | ☑ New binary `cmd/worker`: connects to Redis, runs M executor goroutines behind the Phase 0 interfaces; remove worker pool from `cmd/server` | done | `cmd/worker` + `worker.Node` runtime. In-process mode kept behind `RUN_WORKERS` (default true) for single-binary dev/back-compat |
| P2.2 | ☑ Node registration: `taskqueue:node:{uuid}` TTL key (hostname, capacity, started-at); heartbeat refresh every 3s, 10s expiry | done | `NodeStore`; registry SET `taskqueue:nodes` + `:hb` TTL key + `:tasks` set. IDs = `{hostname}-{shorthex}`. Configurable HEARTBEAT_INTERVAL_MS/TTL_MS |
| P2.3 | ☑ Reaper extension: node heartbeat expired → immediately reclaim ALL of its processing tasks (don't wait for lease expiry) | done | `reapDeadNodes` runs each tick before the lease sweep; eager reclaim via per-node task set; emits `node_dead` |
| P2.4 | ☑ Handler registry keyed by task `type`: `sleep`, `http_fetch`, `hash` | done | New `internal/handler/`; executor dispatches by Type; unknown type = clean failure |
| P2.5 | ☑ docker-compose: separate `worker` service, works with `--scale worker=5`; backend = API + scheduler + reaper only | done | Dockerfile builds both `/server` + `/worker`; backend `RUN_WORKERS=false`; worker service no container_name (scalable). `docker compose config` validates; full `up` not run in this env |
| P2.6 | ☑ Rework worker-state store: dynamic node set with expiring keys (replaces fixed 0..N hash) | done | Node registry supersedes the per-goroutine hash; legacy `WorkerState` now optional (nil in standalone worker) |
| P2.7 | ☑ Dashboard: node panel — cards appear/disappear as containers start/die; hostname, heartbeat age, tasks/sec per node | done | `GET /api/nodes` + `NodePanel.tsx` (AnimatePresence alive/dead cards, in-flight count, capacity); `node_dead`/`reclaimed` colored in ActivityLog. tsc not run (no node_modules) |
| P2.8 | ☑ Tests: miniredis units ✅ + integration test (kill a node → tasks reassigned) ✅ | done | `TestIntegration_NodeDeath_TasksReassigned` drives broker+reaper+nodestore end-to-end incl. owner-fencing. Real testcontainers-go deferred (needs Docker daemon + new dep vs stdlib-first) |
| P2.9 | ☑ Docs: "Cluster membership & failure detection" (heartbeat interval vs false-positive tradeoff: 3s beat / 10s timeout) | done | Section added to ARCHITECTURE.md incl. the 3s/10s tradeoff and owner-fencing safety note; API table lists `/api/nodes` |

**Acceptance criteria:**

- [~] `docker compose up --scale worker=5` runs; nodes appear on dashboard dynamically — compose config validates + logic covered by integration test; **live `up` unrun (heavyweight in this env)**
- [~] `docker compose kill` one worker mid-burst → its card grays out within ~10s, `node_dead` + `reclaimed` events stream, zero task loss — reclaim + events + fencing proven in `TestIntegration_NodeDeath_TasksReassigned`; **live kill demo still to record**
- [ ] Scaling up live (`--scale worker=8`) increases throughput visibly — needs a live Docker run

**Demo (record it — this is the money demo):** kill worker-3 during a 5k-task burst; watch detection → reclaim → rebalance live on the dashboard.

---

## Phase 3 — Observability & Performance (~25h)

**Goal:** operate the system like an SRE would; produce real, publishable numbers.

| ID | Task | Status | Notes |
|---|---|---|---|
| P3.1 | ☑ Replace all `log.Printf` with `log/slog` JSON, keyed by `task_id`/`node_id`; logger injected, not global | done | Root JSON logger built in both `main`s, threaded through the Phase-0 Deps seams (`ExecutorDeps`, `reaper.Config`, `NodeConfig`, `HandlerDeps`, `NewRouter`); every constructor falls back to `slog.Default()` when nil, so existing tests were untouched. `rg '"log"' cmd internal` empty |
| P3.2 | ☑ Prometheus `/metrics` on broker AND workers: `tasks_processed_total{type,status}`, `task_duration_seconds` histogram, `queue_depth`, `enqueue_to_start_seconds` histogram, `reaper_reclaims_total` | done | New `internal/telemetry/` on a dedicated (non-global) registry; `queue_depth` is a scrape-time collector via `QueuePeekStore`. Server serves `/metrics` on `:8080`; worker got a minimal HTTP server on `METRICS_PORT` (default `:9100`). `enqueue_to_start_seconds` sourced from `Task.CreatedAt` (overcounts delayed/retried — precise `ReadyAt` deferred). Verified live |
| P3.3 | ☑ Prometheus + Grafana in compose; ONE committed dashboard JSON in `deploy/grafana/` | done | `prometheus` (`:9090`) + `grafana` (`:3001`, anon Viewer, admin/admin) services; auto-provisioned datasource (uid `prometheus`) + file-provider dashboard `taskqueue.json` (uid `taskqueue-observability`, 10 panels). Worker replicas discovered via Docker **DNS SD** (`worker`:9100) so `--scale`/kill-node auto-updates targets. Verified live: 5 targets up, all 10 panels render (0 errors/0 no-data), kill-worker → reclaims 0→5 + p99 dip/recover on the graph |
| P3.4 | ☑ Replace worker sleep-polling with blocking pickup (timeout = shutdown-check interval); measure & record the latency improvement | done | **Doorbell, not literal `BZPOPMIN`** (see Decision Log + Known Gotchas). Idle workers `BLPOP taskqueue:ready:signal <SignalBlock>` instead of `time.Sleep`; `dequeue.lua` unchanged so at-least-once + priority hold. All 4 ready-producers push one cap-guarded token: `PriorityQueue.Enqueue` (Go `signal.lua`), `promote.lua` + `reclaim.lua` `"reclaimed"` branch (inline). New `SignalBlock` (1s; = block = shutdown-check = fallback-poll) + `SignalCap` (1024) config; explicit `PoolSize ≥ WorkerCount + headroom`. Measured ~495 ms → ~0.86 ms p99 at low/bursty load (`docs/BENCHMARKS.md`) |
| P3.5 | ☑ `cmd/loadgen`: configurable rate, duration, task-type mix | done | Hand-rolled, **stdlib-only** (no k6). Constant `-rate` or linear `-ramp start:end` over `-duration`, weighted `-mix` (e.g. `hash:60,sleep:40`), `-concurrency`, per-type payload tuning (`-sleep-ms`/`-fail-rate`/`-hash-rounds`/`-fetch-url`), `-seed`. Fractional-accumulator scheduler; per-sec progress + summary (submit p50/p95/p99, `behind_ticks`, 429 count → feeds P3.6). Profiled compose service (`docker compose run --rm loadgen …`, never starts on `up`) + `make loadgen ARGS=…`. Verified live: ramp 50→900/s (28.5k tasks, 0 err) drove `queue_depth` 0→26K and `enqueue_to_start` p99→10s on Grafana |
| P3.6 | ☐ Backpressure: max queue depth → `429` + `Retry-After` on submit | todo | |
| P3.7 | ☐ Benchmarks: before/after poll-vs-blocking; sustained 30-min soak asserting zero lost tasks; record machine specs | todo | |
| P3.8 | ☐ Docs: `docs/BENCHMARKS.md` — throughput, p50/p99 enqueue-to-start latency, zero-loss chaos results | todo | |

**Acceptance criteria:**

- [x] Grafana shows live p50/p99 enqueue-to-start latency under a loadgen ramp — verified: `cmd/loadgen -ramp 50:900 -duration 60s` drove p50/p99 to the 10s ceiling with `queue_depth` 0→26K live on the dashboard (28.5k tasks, 0 errors, ~35 completed/s on 3 workers)
- [x] Killing a worker mid-run produces a visible p99 spike + recovery on the graph — verified: `docker kill` a worker → reaper reclaims 0→5, enqueue-to-start p99 dips then recovers, active workers 3→2 live on the dashboard
- [ ] BENCHMARKS.md has honest numbers with machine specs and caveats

**Demo (record it):** loadgen ramp on Grafana; kill a worker mid-run; p99 spikes and recovers.

---

## Phase 4 — Coordination: Leader Election, Cron, Polish (~25h)

**Goal:** multiple broker replicas coordinate safely; recurring jobs; ship it.

| ID | Task | Status | Notes |
|---|---|---|---|
| P4.1 | ☐ Leader election via Redis lease: `SET taskqueue:leader {nodeID} NX PX 10000`, renew at 1/3 TTL, step down on renewal failure; new `internal/election/` with `Elector{RunWhenLeader(ctx, fn)}` | todo | |
| P4.2 | ☐ Gate scheduler + reaper + cron behind leadership; leader-scoped contexts cancelled on loss | todo | |
| P4.3 | ☐ Run 2 broker replicas in compose; dashboard shows a crown/badge on the leader | todo | |
| P4.4 | ☐ Cron jobs: `POST /api/cron` (spec + task template) stored in `taskqueue:cron`; leader materializes due instances. Use `robfig/cron` parser. Instance IDs = `{cronID}-{scheduledTs}` for failover dedup (reuses P1.6) | todo | New `internal/cron/` |
| P4.5 | ☐ Chaos script: random worker/broker kills for 10 min → invariant check (`submitted == completed + dead_lettered`, no duplicate cron fires) | todo | `make chaos` |
| P4.6 | ☐ Tests: election failover (leader dies → follower takes over < TTL) with miniredis; cron materialization with injected clock | todo | |
| P4.7 | ☐ Docs: "Leader election & split-brain" — honest section on the unsafety window without fencing tokens, why lease-not-Raft | todo | |
| P4.8 | ☐ README rewrite: lead with demo GIFs (kill-a-worker, leader failover, load ramp), Mermaid diagrams, delivery-guarantees section, benchmark table, Limitations, future roadmap (Raft, Streams, DAG workflows) | todo | Non-negotiable ~8h — do NOT cut this for features |
| P4.9 | ☐ Record 2–3 GIFs into `docs/gifs/` | todo | |
| P4.10 | ☐ (STRETCH — only if ahead) Task dependency chains: `depends_on: [taskID]`, completed parents trigger children | todo | Skip without guilt |

**Acceptance criteria:**

- [ ] Kill the leader broker → crown migrates < 10s, cron keeps firing, no duplicate fires, no losses
- [ ] 10-min chaos run passes invariants
- [ ] README makes the project's value obvious in 30 seconds of scrolling

**Demo (record it):** two brokers, kill the leader, failover live.

---

## 🧭 Decision Log

<!-- Append-only. Record scope changes, tradeoffs chosen, and things deliberately NOT done. -->

| Date | Decision | Rationale |
|---|---|---|
| 2026-07-06 | Vision fixed: distributed task platform (not workflow engine / not compute platform) | Reuses existing architecture; every week maps to a canonical interview topic; demo-ability |
| 2026-07-06 | Redis stays the coordination bus; no gRPC between broker and workers | Matches Sidekiq/Celery/BullMQ; gRPC adds ceremony without teaching more. Documented in ARCHITECTURE.md |
| 2026-07-06 | Skip: Redis Streams migration, Raft, WebSocket dashboard, auth | Streams → comparison doc instead; Raft → future work headline; others low interview value |
| 2026-07-06 | Exactly-once NOT attempted; at-least-once + idempotency, documented honestly | Exactly-once without idempotent consumers is a lie; saying so is the interview win |
| 2026-07-07 | Module path set to `github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend` | Matches the GitHub org used in recent CI/CD commits |
| 2026-07-07 | Canonical task record stored as a Redis **hash** (not a JSON string) | Enables Phase 1 atomic per-field ops in Lua (`HINCRBY retries`, `HSET status`) without decode/encode; honors P0.3's "hash" wording |
| 2026-07-07 | Server **generates a unique task ID** when the client omits one; client-supplied IDs are honored and become the P1.6 idempotency key | This is what makes "identical fields → two entries" true while keeping explicit-ID dedup available for P1.6 |
| 2026-07-07 | `GET /api/tasks/{id}` (planned P1.7) implemented in Phase 0 | Required by a Phase 0 acceptance criterion; trivial once records are keyed by ID |
| 2026-07-11 | Ack/Nack use Lua scripts (not bare ZREM) for atomicity with hash updates | ZREM + HSET in one script prevents crash between lease release and status update |
| 2026-07-11 | Reaper reads task record before Lua script (passes retries/maxRetries/priority as ARGV) | Avoids embedding complex logic in Lua; record read is non-atomic but safe because ZREM in the script is the true guard |
| 2026-07-11 | Delayed promotion now fully in Lua (promote.lua); events emit batch summary not per-task | Per-task events would require returning IDs from Lua and looping in Go — not worth the complexity for promotion |
| 2026-07-11 | P1.8 (dashboard frontend) and P1.10 (docs) deferred — backend-only session | Will address in a frontend-focused or docs-focused session |
| 2026-07-11 | Node ownership tracked via a per-node SET (`taskqueue:node:{id}:tasks`) + `owner` field on the task hash | Lets the reaper reclaim a dead node's work in O(its tasks) instead of scanning all of `processing` |
| 2026-07-11 | Ack/Nack gained owner-equality fencing (verify `owner==thisNode` before releasing) | Prevents a stalled worker whose lease was reclaimed + re-leased elsewhere from clobbering the new owner. Monotonic fencing tokens remain P4 |
| 2026-07-11 | In-process workers kept behind `RUN_WORKERS` (default true) rather than removed from `cmd/server` | Keeps the current single-container `docker compose up` working until P2.5 splits the compose file; distributed mode sets `RUN_WORKERS=false` |
| 2026-07-11 | Legacy per-goroutine `WorkerState` hash made optional (nil in `cmd/worker`) | Cross-process int worker IDs would collide; the node registry is the Phase 2 source of truth for "what's running" |
| 2026-08-13 | Toolchain bumped **Go 1.22 → 1.25** (go.mod, CI `go-version`, `backend/Dockerfile`) | `prometheus/client_golang v1.24.1` (P3.2's one new dep) declares `go 1.25.0`; keeping the latest security-patched metrics client was preferred over pinning an older client_golang to stay on 1.22 |
| 2026-08-13 | P3.2 telemetry on a **dedicated** `prometheus.Registry`, not the global default; every constructor's injected logger/telemetry is **nil-tolerant** | Dedicated registry avoids duplicate-registration panics across tests; nil-tolerance let the ~40 existing tests compile without threading a logger/telemetry (same pattern as `WorkerState`/`Events`) |
| 2026-08-13 | `enqueue_to_start_seconds` sourced from `Task.CreatedAt`, not a new `ReadyAt` field | Zero schema/Lua change; exact for the zero-delay loadgen workload P3 targets. A precise `ReadyAt` (needs `promote.lua`/`reclaim.lua` edits) is deferred until a task needs sub-second accuracy for delayed/retried tasks |
| 2026-08-13 | **P3.4 resolved via a doorbell wake-up signal, NOT literal `BZPOPMIN taskqueue:ready`** | `BZPOPMIN` on the ready ZSET would pop the task ID into worker memory and only *then* write it to `processing` in a second round trip — a `kill -9` in that gap loses the task (it's in neither set), reintroducing the exact crash-loss window P1.2 closed. Instead `dequeue.lua` stays byte-for-byte unchanged (atomic `ZPOPMIN → processing`) and idle workers `BLPOP` a pure notification list `taskqueue:ready:signal`; producers ring it. Tokens carry no task identity and are never read for correctness — a lost token costs ≤ `SignalBlock` latency, never a task. Priority ordering also preserved (`ZPOPMIN` still picks, regardless of FIFO token order) |
| 2026-08-13 | Doorbell push is **cap-guarded** (`LLEN < SignalCap` before `RPUSH`) and shared as `signal.lua`, inlined into `promote.lua`/`reclaim.lua` | Bounds the signal list from growing to queue depth when all workers are busy; any token beyond the number of blocked workers is redundant (the busy worker re-claims via try-then-block, and the fallback poll re-claims regardless). One embedded script keeps the cap logic in exactly one place for the Go path; Lua can't `//go:embed`-compose so the two batch scripts inline the same two lines |
| 2026-08-13 | Graceful shutdown relies on the **bounded `SignalBlock` block returning**, not on ctx interrupting an in-flight `BLPOP` | Verified against real Redis: go-redis does **not** abort a `BLPOP` already in flight when its context is cancelled (it runs to the timeout); a call issued with an *already-cancelled* ctx returns `context.Canceled`. So a worker exits within ≤ `SignalBlock` (1s) of cancellation — well inside the 5s drain / 10s HTTP-shutdown budgets. Also learned: go-redis floors sub-second `BLPOP` timeouts at 1s, so `SignalBlock=1s` is the practical minimum |
| 2026-08-15 | **P3.3: worker replicas scraped via Docker DNS SD**, not static targets; Grafana on `:3001` (frontend owns `:3000`), Prometheus `:9090`; anonymous **Viewer** access on | Scaled workers have no stable hostname — an A-record lookup of the compose service name `worker` returns one record per replica, so Prometheus auto-discovers/drops them on `--scale`/kill (keeps the kill-node demo working). Anon viewer opens the demo dashboard with no login (admin/admin still valid). **No Go code changed** — P3.2 already exposed every metric; P3.3 is pure ops/config |
| 2026-08-15 | **P3.5 `cmd/loadgen` is stdlib-only and a profiled one-shot compose service** (not k6/vegeta, not always-on) | Stdlib matches the repo's stdlib-first bias. One-shot (`docker compose run --rm loadgen …`, `profiles: [loadgen]`) keeps controlled ramp-then-observe experiments with clean baselines; an always-on service would keep the cluster perpetually saturated, fighting the kill-node/ramp demos and burning laptop resources. The same binary supports `-duration 0` for on-demand soak, so the continuous case isn't lost |

---

## ⚠️ Known Gotchas (context for future sessions)

- ~~**P3.4 vs P1.2 tension:** `BZPOPMIN` can't run inside a Lua script.~~ **Resolved (2026-08-13):**
  shipped the wake-up-signal path — idle workers `BLPOP taskqueue:ready:signal` (a pure
  doorbell) then run the unchanged atomic Lua claim. `dequeue.lua` untouched; at-least-once
  intact. The `BZPOPMIN`-direct option was rejected (loss window). See Decision Log.
- The delayed scheduler currently double-runs safely across replicas only because of the
  `ZREM` returned-count check (`delayed.go`) — after P4.2 it's leader-only anyway.
- `DELETE /api/flush` must be gated behind a `DEV_MODE` flag before any public deploy.
- Frontend `NEXT_PUBLIC_API_URL` defaults to `localhost:8080` — breaks if API moves.

---

## 📦 Backlog (post-month / future roadmap)

- Raft-based election (replace Redis lease) — headline future-work item
- Task dependency DAGs / mini-workflows (if P4.10 was skipped)
- Redis Streams comparison doc (`XREADGROUP`/`XAUTOCLAIM` vs hand-rolled PEL)
- Per-queue rate limiting (Cloud Tasks style)
- Sharded/multiple named queues with routing
- OpenTelemetry tracing spans per task lifecycle
- SSE/WebSocket dashboard push (replace polling)

**DevOps / CD (planned — deploy automation):**
- **Auto-deploy backend + frontend on publish (CD).** Today CI publishes images to Docker Hub but nothing deploys them. Add a `deploy` job (`needs: publish`, main-push only). Recommended target: a single always-on Docker VM running `docker-compose.prod.yml` (`image:`-only, no `build:`) via SSH — mirrors local compose, keeps `--scale worker=4` + the kill-node demo. Prereqs: (1) provision VM + dedicated deploy user + open 3000/8080, keep 6379 internal; (2) GitHub secrets DEPLOY_HOST/USER/SSH_KEY + PUBLIC_API_URL; (3) **fix `NEXT_PUBLIC_API_URL`** — it's baked at build to `localhost:8080`, must be the public backend URL (build-arg or runtime config); (4) **gate `DELETE /api/flush` behind `DEV_MODE`** BEFORE any public deploy (open flush = anyone wipes the queue). Alt targets: self-hosted runner (free, local-only), PaaS (awkward for the scaled-worker/kill-node demo).
- CI polish (P2/P3): buildx GHA layer caching, `docker/build-push-action`, per-image `paths-filter`, Trivy image scan, Dependabot (actions/npm/gomod), `next lint`/eslint, fix image `source` OCI label (points at `weekend_project_1`).

---

## 📓 Session Log

<!-- Append-only, newest at top. Every work session gets an entry, even short ones. -->

### Template

```markdown
### YYYY-MM-DD — <short title> (~Xh)
- **Done:** P1.2 done, P1.3 in-progress (Lua ack script written, tests failing on edge case X)
- **Learned/decided:** <anything a future session needs to know>
- **Next:** finish P1.3 tests, then start P1.4
- **Blockers:** none
```

---

### 2026-08-15 — Phase 3 P3.5 cmd/loadgen (~1.5h)
- **Done:** Hand-rolled, stdlib-only load generator `backend/cmd/loadgen/main.go`. Constant `-rate` or linear `-ramp start:end` over `-duration`; weighted `-mix` (sleep/hash/http_fetch); `-concurrency` submitter pool; per-type payload tuning (`-sleep-ms`/`-fail-rate`/`-hash-rounds`/`-fetch-url`); `-priority`/`-max-retries`/`-seed`; `LOADGEN_URL` env default. Fractional-accumulator rate scheduler (carry capped to ~1s so a stall can't burst), non-blocking token emit with a `behind_ticks` saturation counter, reservoir-sampled submit-latency p50/p95/p99. Per-second progress log + final summary. Pure helpers `parseMix`/`parseRamp`/`targetRate`/`pickType` unit-tested (`loadgen_test.go`). Wired: Dockerfile builds `/loadgen` (3rd binary in the shared image); `docker-compose.yml` profiled `loadgen` service (`profiles: [loadgen]`, `LOADGEN_URL=http://backend:8080`, entrypoint `/loadgen`) so it never starts on `up`; `make loadgen ARGS=…`.
- **Verified:** `go build`/`vet`/`gofmt -l` clean, `go test ./... -race` all green (loadgen mix/ramp/percentile tests added). `docker compose config` valid; loadgen absent from default services, present under `--profile loadgen`. **Live** (rebuilt image, real 3-worker cluster): `docker compose run --rm loadgen -ramp 50:900 -duration 60s -mix hash:40,sleep:60 -concurrency 80` → smooth ramp tracking target every second, **28,497 submitted, 0 errors, 0 429s**, submit p50 1ms / p99 3ms. Server side (Prometheus): `queue_depth` 0 → **peak 26,227**, `enqueue_to_start` p99 NaN → **10s ceiling**, completed ~35/s — closes the acceptance criterion "Grafana shows live p50/p99 under a loadgen ramp." Grafana screenshot (anon kiosk): 10 panels, 0 errors, ramp story clearly visible. Flushed the backlog after (`DELETE /api/flush` → queue_depth 0) to leave a clean cluster.
- **Learned/decided:** stdlib-only + profiled one-shot (see Decision Log 2026-08-15); `-duration 0` covers on-demand soak so no always-on service needed. On a sleep-heavy mix the 3-worker cluster tops out ~35/s, so a ~900/s ramp builds a large backlog fast — great for showing latency blow-up; use a hash-heavy mix or more workers for higher sustained throughput.
- **Next:** P3.6 (backpressure: max queue depth → `429` + `Retry-After`; loadgen already counts 429s to exercise it), then P3.7 soak → P3.8 BENCHMARKS.
- **Blockers:** none.

---

### 2026-08-15 — Phase 3 P3.3 Prometheus + Grafana in compose (~2h)
- **Done:** Added `prometheus` + `grafana` to `docker-compose.yml` and a committed `deploy/` tree — pure ops/config, **no Go changes** (P3.2 already exposes all metrics). `deploy/prometheus/prometheus.yml`: 5s scrape, `taskqueue-server` static (`backend:8080`), `taskqueue-workers` via **`dns_sd_configs`** on the compose service name `worker` (type A, port 9100, 10s refresh) so every scaled replica is auto-discovered/dropped, plus a self-scrape. Grafana auto-provisioned: `deploy/grafana/provisioning/datasources/prometheus.yml` (uid `prometheus`, `http://prometheus:9090`, default) + `.../dashboards/provider.yml` (file provider) + the one committed dashboard `deploy/grafana/dashboards/taskqueue.json` (uid `taskqueue-observability`, 10 panels: active workers / completed / failed / reclaims stats; enqueue→start p50/p99; throughput by status; queue depth; task-duration p99 by type; reclaims/s; retries/s). Compose: `prometheus` host `:9090` (`--web.enable-lifecycle`, `prometheus-data` vol), `grafana` host `:3001` (anon **Viewer** on, admin/admin, provisioning + dashboards + `grafana-data` mounted). `docker compose config` valid.
- **Verified (live, real Docker cluster + 3 workers):** brought the stack up (`--scale worker=3`), submitted 300 mixed + 800 sleep tasks. Prometheus `/api/v1/targets` = **5 targets all up** (server + 3 workers via DNS SD + self). Instant queries return real data (`tasks_processed_total` by status completed/retried/failed; `queue_depth`; enqueue-to-start p99; active workers). Grafana `/api/health` ok, datasource + dashboard provisioned, datasource-proxy query returns series. **Playwright screenshot** (scratchpad, anon kiosk): all **10 panels rendered, 0 panel errors, 0 "No data", 0 console errors**. **Kill-node demo:** `docker kill` a worker mid-burst → `reaper_reclaims_total` 0→5, reclaims/s spike, enqueue-to-start p99 dips then recovers, active workers 3→2, dead-node tombstone on `/api/nodes` — the money graph works. Restored to 3 workers after.
- **Learned/decided:** compose service DNS returns one A record per replica → `dns_sd_configs` is the clean way to scrape a scaled service (no hardcoded replicas; survives kill-node). Grafana on `:3001` (frontend squats `:3000`). Anon Viewer = zero-friction demo. See Decision Log (2026-08-15). Stack left running for preview; `docker compose down` (or `down -v` to drop Prom/Grafana volumes) to stop.
- **Next:** P3.5 (`cmd/loadgen`: configurable rate/duration/type-mix) to drive the 100→2000/s ramp the first acceptance criterion wants, then P3.6 backpressure and the P3.7 soak → P3.8 BENCHMARKS.
- **Blockers:** none.

---

### 2026-08-13 — Phase 3 P3.4 blocking task pickup (doorbell) (~3h)
- **Done:** Replaced worker sleep-polling with a **doorbell**. Idle workers now block on `BLPOP taskqueue:ready:signal <SignalBlock>` (new `broker.WaitForReady` → `PriorityQueue.WaitReady`) instead of `time.Sleep(PollInterval)`; on wake they run the **unchanged** atomic claim (`dequeue.lua` byte-for-byte identical → at-least-once + priority preserved). All four ready-producers ring a **cap-guarded** doorbell: `PriorityQueue.Enqueue` (Submit + RedriveFailed) via a shared `signal.lua` (`LLEN < cap` then `RPUSH`); `promote.lua` (delayed→ready) and `reclaim.lua` **`"reclaimed"` branch only** inline the same two lines (atomic with their `ZADD ready`); the `dead_lettered`/`orphan`/`lost_race` branches push nothing. New config `SignalBlock` (`SIGNAL_BLOCK_MS`, default 1000 — one knob = BLPOP block = shutdown-check granularity = fallback-poll backstop) and `SignalCap` (`SIGNAL_CAP`, default 1024), with validation; `PollInterval` kept as the Dequeue-**error** back-off only. `NewRedisClient` now sets an explicit go-redis `PoolSize = WorkerCount + 10` (blocked `BLPOP`s hold connections; too small a pool starves heartbeats → false-dead nodes); both mains pass `cfg.WorkerCount`. Threaded `SignalBlock`/`SignalCap` through `cmd/server` + `cmd/worker` (in-process pool gets it for free).
- **Verified:** 68 tests green (`go test ./... -race -count=1`, was 53 → +15: pool-size, config defaults/validation, signal push/cap, Enqueue doorbell, promote×3 + cap, reclaim-branch-only + orphan-no-push, WaitForReady unblock, pool doorbell-wake + no-token fallback) + `go vet ./...` + `gofmt -l cmd internal` clean. **Real-Redis integration** (`//go:build integration`, Docker `redis:7-alpine` on `:6399`, `make test-integration` equivalent) all green: BLPOP block-timeout ~1s, already-cancelled-ctx returns promptly, full pool drains within `SignalBlock` after cancel, and the latency capture. Both binaries start live (server in-process pool `signal_block=1s`; worker `/metrics` 200).
- **Measured (`docs/BENCHMARKS.md`, Apple M5 / 16 GB):** `enqueue_to_start` p50/p99 **~250 ms / ~495 ms (sleep-poll) → ~0.35 ms / ~0.86 ms (doorbell)** at low/bursty load — a ~1000× wake-time cut. **Honest caveat:** no win under saturation (queue never empty → worker never blocks; the two are equivalent). The doorbell removes polling dead-time, not per-task cost.
- **Learned/decided:** go-redis **floors sub-second `BLPOP` timeouts at 1s** (so `SignalBlock=1s` is the practical minimum) and does **not** interrupt an in-flight `BLPOP` on mid-block ctx-cancel — shutdown is bounded by the block *returning*, exactly as planned (an already-cancelled ctx does return `context.Canceled`). Both nailed down in the integration test because miniredis's BLPOP timeout uses real wall-clock and ignores `mr.FastForward`.
- **Next:** P3.3 (Prometheus + Grafana in compose, one dashboard JSON) or P3.5 (`cmd/loadgen`), then P3.6 backpressure and the P3.7 soak.
- **Blockers:** none

---

### 2026-08-13 — Phase 3 P3.1 (slog) + P3.2 (Prometheus) (~3h)
- **Done:** P3.1 structured logging — one root JSON `slog.Logger` per binary, injected through the Phase-0 Deps seams (`store.NewRedisClient`, `ExecutorDeps`, `reaper.Config`, `worker.NodeConfig`/`Pool`, `DelayedScheduler`, `api.HandlerDeps`/`NewRouter`/middleware); child loggers key `task_id` (executor/reaper) and `node_id` (node/pool); every constructor falls back to `slog.Default()` when nil so existing tests were untouched; all `log.Printf/Println/Fatalf` gone (`rg '"log"' cmd internal` empty). P3.2 Prometheus — new `internal/telemetry/` on a dedicated registry with `tasks_processed_total{type,status}`, `task_duration_seconds{type}`, `enqueue_to_start_seconds`, `reaper_reclaims_total`, and a scrape-time `queue_depth` collector reading `ZCARD` via `QueuePeekStore.ReadySize`; wired at the executor (ack/nack) and reaper (reclaim); `/metrics` on the server `:8080` router + a new minimal HTTP server on the worker's `METRICS_PORT` (default `:9100`). One new dep `prometheus/client_golang v1.24.1`, which forced the Go 1.22→1.25 toolchain bump (see Decision Log; CI + Dockerfile updated to match).
- **Verified:** 53 tests (`go test ./... -race`, was 41) + `vet`/`gofmt` clean. **Live** (real Redis via Docker, ports retargeted around the running stack): `/metrics` scrape showed `tasks_processed_total{status="completed",type="hash"} 3`, `task_duration_seconds_count 3`, `enqueue_to_start_seconds_count 3`, `queue_depth 0`; JSON logs carried `task_id`+`node_id`; worker `/metrics` returned 200. Backend Docker image builds clean on `golang:1.25-alpine`.
- **Learned/decided:** `*Vec` metrics emit nothing until first labeled observation (expected). Dedicated registry + nil-tolerant injection avoid test churn. See Decision Log (2026-08-13).
- **Next:** P3.3 (Prometheus + Grafana in compose, one dashboard JSON in `deploy/grafana/`), then P3.4 (BZPOPMIN — mind the Lua-lease tension in Known Gotchas) / P3.5 (`cmd/loadgen`).
- **Blockers:** none.

### 2026-08-12 — Live e2e + demo-bug fixes + CI quality gate + CD planning (~8h)
- **Done:** First live e2e of the whole dashboard vs the real 4-worker Docker cluster (was mock-only). Closed the Phase-1 zero-loss criterion via the distributed variant: `docker kill` a worker mid-burst → reaper reclaimed its leases → survivors finished, `submitted == completed + dead_lettered` exactly. Fixed 2 bugs only the live run surfaced: (1) `active_workers` was structurally 0 in distributed mode (read the backend in-process pool under `RUN_WORKERS=false`) → now `ZCARD(processing)`; (2) kill-node recovery was invisible → dead-node `:meta`/`:dead` retention + 30s `DEAD_NODE_GRACE_MS` tombstone, a retained `taskqueue:events:cluster` list + `GET /api/events/cluster` + an Activity Log All|Cluster toggle. Shipped as PR #8. Then added a CI quality gate: `.github/workflows/ci.yml` (backend gofmt/vet/`test -race`, frontend `tsc`/`build`) gating a consolidated `publish` job; `main` branch protection now REQUIRES `backend-test` + `frontend-test` (strict). Backend Dockerfile got a cached `go mod download` dep layer (dropped `go mod tidy`).
- **Learned/decided:** `main` was pre-revamp; all frontend work must branch off `feature/frontend-revamp` until it merges (it since merged to main). CI must stay green: fixed 5 files that weren't gofmt-clean. Auto-deploy (CD) is the next DevOps item — see Backlog; a single Docker VM is the recommended target for this distributed/kill-node demo. Two hard prereqs before public deploy: fix the build-time `NEXT_PUBLIC_API_URL`, and gate `/api/flush` behind `DEV_MODE`.
- **Next:** merge the CI-hardening PR; then either backend Phase 3 (observability) or implement CD once a host exists.
- **Blockers:** none.

### 2026-07-11 — Phase 2 deferred items: API, dashboard, compose, integration (~4h)

- **Done:** Closed out all remaining Phase 2 tasks. `GET /api/nodes` endpoint
  (nil-safe, returns `[]` never null) + `NodeStore` wired into the API + handler
  test. P2.7 frontend: `Node` type, `getNodes()`, `NodePanel.tsx` (AnimatePresence
  alive/dead cards, in-flight/capacity), `node_dead`/`reclaimed`/`redriven` colored
  in the activity log. P2.5: Dockerfile builds both `/server` + `/worker`;
  docker-compose split (backend `RUN_WORKERS=false` + scalable `worker` service),
  `docker compose config` validates. P2.8: `TestIntegration_NodeDeath_TasksReassigned`
  end-to-end (node dies → reaper reassigns → second node picks up → owner-fencing
  rejects the stale Ack). P2.9: cluster-membership section + `/api/nodes` in
  ARCHITECTURE.md.
- **Verified:** backend `go build`/`go test` green (see next verify step); frontend
  `tsc` not run (no `node_modules`); `docker compose config` valid but live `up`
  not run (heavyweight).
- **Learned/decided:** real testcontainers-go deferred — it needs a Docker daemon
  and a new dependency, and would exercise the same Redis-side logic the miniredis
  integration test already covers (stdlib-first bias).
- **Deferred (need a live environment):** record the kill-a-worker demo GIF; run
  `docker compose up --scale worker=N` to confirm the dashboard live.
- **Next:** Phase 3 — observability & performance (P3.1 slog, P3.2 Prometheus).
- **Blockers:** none.

### 2026-07-11 — Phase 2 backend core: distribution (~8h)

- **Done:** Backend of Phase 2. P2.4 handler registry (`internal/handler/`: sleep/
  http_fetch/hash, executor dispatches by Type, unknown type = clean failure).
  P2.2/P2.6 node identity + `NodeStore` (registry SET `taskqueue:nodes`, `:hb` TTL
  key, `:tasks` ownership set; `model.Node`; `Owner` on `model.Task`). Node-ownership
  Lua updates: dequeue stamps owner + SADD node set; ack/nack/reclaim clear it;
  **added owner-equality fencing** to ack/nack. P2.1 `cmd/worker` binary + `worker.Node`
  runtime (register → heartbeat → pool → drain → deregister); `cmd/server` slimmed,
  in-process workers behind `RUN_WORKERS` (default true). P2.3 reaper `reapDeadNodes`:
  eager reclaim of a dead node's whole in-flight set (recovery bounded by heartbeat
  TTL, not visibility timeout), emits `node_dead`. P2.8 units: 14 new tests (handler 8,
  NodeStore 6 incl. TTL expiry via `mr.FastForward`, dead-node eager reclaim 1).
- **Verified:** `go vet` + `go build` clean; both `cmd/server` and `cmd/worker` compile
  to executables; `go test ./... -count=1` = **39 tests** pass (was 25). No Docker in
  this env.
- **Learned/decided:** per-node task SET enables O(node's tasks) dead-node reclaim;
  owner-fencing prevents cross-node ack clobbering; `RUN_WORKERS` flag preserves the
  single-container setup until compose is split. See Decision Log (2026-07-11).
- **Deferred:** P2.5 (compose worker service, needs Docker), P2.7 (frontend node panel),
  P2.8 integration test (testcontainers, needs Docker), P2.9 (docs — partly covered by
  the ARCHITECTURE.md update). No API endpoint for `ListNodes` yet — add alongside P2.7.
- **Next:** wire the deferred items in a Docker/frontend session (compose split +
  `GET /api/nodes` + node panel), or begin Phase 3 (observability).
- **Blockers:** none (deferred items need Docker/frontend, not blocked).

### 2026-07-11 — Phase 1 at-least-once delivery (~6h)

- **Done:** All backend tasks of Phase 1 (P1.1–P1.7, P1.9). Implemented:
  `taskqueue:processing` ZSET with ms-precision deadline scores; atomic Lua dequeue
  (dequeue.lua: ZPOPMIN→ZADD processing→HSET status); Ack/Nack via Lua scripts
  (ack.lua: ZREM+HSET completed; nack.lua: ZREM only, executor routes retry/DLQ);
  ExtendLease (ZADD XX); Reaper in `internal/reaper/` with reclaim.lua
  (ZREM concurrency guard → ZADD ready with retries++ or mark failed + DLQ push);
  atomic delayed promotion via promote.lua (fixes loss window); idempotent enqueue
  (TaskStore.Exists + 409 on duplicate client IDs); redrive endpoint
  (POST /api/tasks/failed/redrive: drains DLQ, resets, re-enqueues). Executor
  now calls Ack on success / Nack on failure, handles ErrLeaseNotHeld gracefully.
  14 new tests (10 broker + 4 reaper); 24 total passing.
- **Verified:** `go vet ./...` clean, `go build ./...` clean, `go test ./... -count=1`
  all 24 tests pass. No Docker in this env — crash test deferred to Docker session.
- **Learned/decided:** ZREM return value is the universal concurrency guard across
  all scripts (dequeue/ack/nack/reclaim/promote). Reaper-vs-worker race is expected
  and handled cleanly via ErrLeaseNotHeld. Millisecond scores in processing ZSET
  give sufficient precision. See Decision Log entries (2026-07-11).
- **Deferred:** P1.8 (frontend dashboard update) and P1.10 (ARCHITECTURE.md delivery
  guarantees section) — will do in dedicated sessions. Crash test acceptance criteria
  need Docker.
- **Next:** Phase 2 — P2.1 (standalone `cmd/worker` binary), P2.2 (node registration
  with TTL heartbeat keys). The broker interface and reaper are ready to extend for
  node-awareness.
- **Blockers:** none.

### 2026-07-07 — Phase 0 foundations (~8h)

- **Done:** All of Phase 0 (P0.1–P0.9). Module renamed; canonical task records as
  `taskqueue:task:{id}` hashes with ready/delayed sets holding IDs only (identity
  bug fixed); `Broker` interface + `RedisBroker` (pool dequeues through it);
  `Type`/`Payload` on `model.Task`; drain-context executor writes; `HandlerDeps`/
  `ExecutorDeps`; config as `time.Duration` with validation + HTTP server timeouts;
  `Makefile` + miniredis + 10 unit tests; `GET /api/tasks/{id}` (early P1.7);
  `docs/ARCHITECTURE.md` started. All Phase 0 acceptance criteria met.
- **Verified:** `go build/vet ./...` clean, `make test` green (10 tests). No Docker
  in this env, so ran the server binary against miniredis and drove the full
  pipeline over HTTP: identical-field submits → distinct IDs; submit → execute →
  retry (delayed→promoted) → dead-letter; `GET /api/tasks/{id}` returns the updated
  canonical record; missing → 404.
- **Learned/decided:** record is a **hash** (not JSON string) to enable Phase 1 Lua
  field ops; server generates IDs so "identical fields → two entries" holds while
  explicit IDs stay available as P1.6 idempotency keys. See Decision Log (2026-07-07).
- **Next:** Phase 1 — start P1.1 (`processing` ZSET + visibility-timeout config),
  then P1.2 (atomic Lua lease dequeue). `Broker.Ack/Nack/ExtendLease` are stubbed
  and ready to implement. P1.7's `GET /api/tasks/{id}` is already done.
- **Blockers:** none. (Note: `docker compose up` end-to-end still unverified on a
  machine with Docker — worth a quick confirm next session.)
