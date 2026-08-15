# Plan — P3.4: Blocking task pickup (doorbell) replacing sleep-polling

**Project:** task-queue-educational-dashboard (Go backend) · **Track:** full · **Date:** 2026-08-13
**Task (verbatim):** *Replace worker sleep-polling with blocking `BZPOPMIN` (timeout = shutdown-check interval); measure & record the latency improvement.*

## Approach

**Chosen: (A) doorbell / wake-up signal, not a literal `BZPOPMIN` on the ready set.**

The at-least-once guarantee (P1.2) rests entirely on one fact: a task only ever leaves
`taskqueue:ready` via the **atomic Lua claim** in `internal/broker/scripts/dequeue.lua`
(`ZPOPMIN ready → ZADD processing(deadline) → HSET status/owner → SADD node:tasks`). Nothing
may pop a task ID out of `ready` except that script — otherwise there is a window where the
task is in neither `ready` nor `processing` and a crash loses it. Redis scripts cannot block,
so `BZPOPMIN` **cannot** live inside `dequeue.lua`.

So we keep `dequeue.lua` byte-for-byte unchanged and add a **pure notification channel**: a
Redis list `taskqueue:ready:signal` used as a doorbell. An idle worker, after a `Dequeue` that
returns empty, blocks on `BLPOP taskqueue:ready:signal <SignalBlock>` instead of
`time.Sleep(PollInterval)`. Every producer of a *ready* task pushes one token onto that list;
`BLPOP` hands each token to exactly one blocked worker, which then wakes and runs the
**existing unchanged atomic claim**. The token is a wake-up only — it is discarded; the claim
(`ZPOPMIN`) remains the sole source of truth, so **priority ordering is preserved** (the Lua
`ZPOPMIN` still pops the highest-priority task regardless of FIFO token order) and
**at-least-once is preserved** (no task leaves `ready` except through the atomic claim).

Correctness never depends on a token arriving. `SignalBlock` doubles as a **fallback poll
backstop**: if a token is ever missed, dropped, or trimmed, `BLPOP` times out after
`SignalBlock`, the worker loops, and re-runs the atomic claim anyway. A lost token therefore
adds *at most `SignalBlock` of latency* and can *never* lose a task. The doorbell is a pure
optimization layered over the existing poll-correct design.

Why `BLPOP` on a side list rather than `BZPOPMIN` directly: the task's literal wording is
`BZPOPMIN`, but `BZPOPMIN taskqueue:ready` would pop the *actual task ID* out of the priority
ZSET, which is exactly rejected **path (B)** below. We use `BLPOP` (a go-redis call that already
exists, no new deps) on a dedicated signal list to get the blocking behavior without touching
the claim. This is the honest way to honor "blocking pickup" while keeping the Phase-1 headline
intact — a point worth calling out in `docs/BENCHMARKS.md`.

**Rejected: (B) `BZPOPMIN taskqueue:ready` + immediate processing-write.** Simpler (no producer
signalling, no side list), but it pops the task ID into worker memory and only *then* writes it
to `processing` in a second round trip. A `kill -9` in that gap loses the task — it is in neither
set. That directly reintroduces the crash-loss window P1.2 was built to close and weakens the
at-least-once guarantee this project markets as its honest headline. The reaper cannot save it
(the task is not in `processing`, so nothing reclaims it). Not worth the small simplicity.

## Touched surface

**Modified — consumer (blocking pickup):**
- `internal/worker/pool.go` — replace the two `time.Sleep(p.pollInterval)` empty-queue waits in
  `worker()` with a blocking wait on the doorbell. New loop shape: try `Dequeue`; on a task,
  execute and `continue` (drain fast, never block while work remains); on empty, call the new
  blocking wait for up to `SignalBlock`; loop. Keep `pollInterval` only as the short back-off
  sleep on a `Dequeue` **error**. Distinguish three `BLPOP` outcomes: token/`redis.Nil`
  (timeout — normal, just loop), `context.Canceled`/`ctx.Err()` (shutdown — clean return, no
  error log), real error (log + back-off).
- `internal/broker/broker.go` — add one method to the `Broker` interface,
  `WaitForReady(ctx, timeout) error`, implemented by `RedisBroker` by delegating to the ready
  queue's `BLPOP` wrapper. Interface grows by one method; the existing
  `var _ Broker = (*RedisBroker)(nil)` assertion covers it. **No mock Broker exists** (tests use
  the real `RedisBroker` against miniredis), so no test doubles need updating. Keeps `Pool`
  depending only on `broker.Broker` as it does today.

**Modified — ready producers (each must push exactly one token per newly-ready task):**
- `internal/queue/queue.go` — `PriorityQueue.Enqueue` (used by handler *Submit* with no delay
  and by *RedriveFailed*) pushes a token after its `ZADD ready`. Add a shared `Signal(ctx)`
  method and a `WaitReady(ctx, timeout) (bool, error)` `BLPOP` wrapper (owns the ready + signal
  keys). Pattern followed: the existing `//go:embed` Lua-script members on this struct
  (`promoteScript`).
- `internal/queue/scripts/promote.lua` — delayed→ready batch: push one token per promoted ID
  (RPUSH inside the existing per-ID loop, guarded by the cap). Atomic with the `ZADD ready` in
  the same script — no Go-side widening of the loss-free window.
- `internal/reaper/scripts/reclaim.lua` — on the `"reclaimed"` branch (task goes back to
  `ready`), push one token, atomic with the `ZADD ready`. The `dead_lettered`/`orphan`/
  `lost_race` branches push nothing (nothing became ready).
- New `internal/queue/scripts/signal.lua` — the one shared, cap-guarded push used by the Go
  `Enqueue`/`RedriveFailed` path: `if LLEN(sig) < cap then RPUSH(sig, "1") end`. The two batch
  scripts inline the same two lines rather than call out (Lua can't `//go:embed`-compose).

**Modified — keys / client / config:**
- `internal/store/redis.go` — add `KeyReadySignal = "taskqueue:ready:signal"`. Set an explicit
  `PoolSize` on the go-redis client `≥ WorkerCount + headroom`: each blocked `BLPOP` holds a
  connection for up to `SignalBlock`, and heartbeat / claim / ack must not be starved of
  connections (a starved heartbeat = a false-dead node — see risks). `NewRedisClient` takes a
  `poolSize int` (or min-headroom) arg; both `main`s pass `cfg.WorkerCount`.
- `internal/config/config.go` — add `SignalBlock time.Duration` (`SIGNAL_BLOCK_MS`, default
  `1000`) = the `BLPOP` block timeout = shutdown-check granularity = fallback-poll interval, all
  one knob; and `SignalCap int` (`SIGNAL_CAP`, default `1024`) = doorbell list bound. Validate
  `SignalBlock > 0`, `SignalCap > 0`. Keep `PollInterval` (now only the error back-off).

**Modified — construction (thread the new config through):**
- `cmd/worker/main.go`, `cmd/server/main.go` — pass `SignalBlock`/`SignalCap` and the pool-size
  to the relevant constructors; server's in-process pool (`RUN_WORKERS=true`) gets the doorbell
  for free via the same `pool.go`.

**New — tests & docs:**
- `internal/worker/pool_test.go` (or extend existing) — doorbell wake + fallback (miniredis).
- `internal/queue/*_test.go`, `internal/reaper/*_test.go` — each producer pushes a token; cap
  holds; claim/reclaim scripts still behave (regression).
- `internal/worker/blocking_integration_test.go` — `//go:build integration`, real Redis (Docker),
  runs under `make test-integration`: proves `BLPOP` ctx-cancel + block-timeout behavior and
  captures the before/after `enqueue_to_start_seconds` numbers. Pattern followed: the existing
  `integration` build-tag convention already in the `Makefile`.
- `docs/BENCHMARKS.md` (P3.7/P3.8 stub) — record the measured p50/p99 improvement + the honest
  caveat (win appears at low/bursty load, not at saturation).

## Impact & risk

- **At-least-once preservation (the core argument).** `dequeue.lua`, `ack.lua`, `nack.lua`,
  `extend.lua`, and the `ZREM`-guard in `reclaim.lua` are all **unchanged**. A task still leaves
  `ready` only via the atomic claim, and the reaper still reclaims any expired lease. The signal
  list carries no task identity and is never read for correctness — worst case a missing token
  costs `SignalBlock` latency. So the delivery guarantee is byte-for-byte what P1.2 shipped.
- **Priority ordering preserved.** Tokens are fungible and FIFO; the actual pick is still
  `ZPOPMIN` on the priority ZSET, so higher priority still runs first regardless of token order.
- **Thundering herd / fairness.** `BLPOP` pops one token to exactly one blocked worker, so one
  enqueue wakes one worker (N tokens for a batch of N → up to N woken). No `PUBLISH`-style
  broadcast that wakes all N to fight over one task. A *spurious* wake (token exists but another
  worker already claimed the task) simply re-blocks — non-fatal, bounded by the cap.
- **Unbounded signal list.** If all workers are busy while producers keep enqueuing, tokens could
  pile up to queue depth. Bounded by the cap-guarded push (`LLEN < cap` before `RPUSH`): once
  `cap` tokens are pending, further pushes no-op, because any token beyond the number of blocked
  workers is redundant and the busy worker re-claims via try-then-block when it frees up. The
  fallback poll makes the exact cap non-critical.
- **Graceful shutdown / drain.** Today `worker()` checks `ctx.Done()` at loop top and on
  `Dequeue` error. With blocking: `BLPOP` returns every `SignalBlock` (`redis.Nil` on timeout),
  the loop re-checks `ctx.Done()`, so a worker exits within ≤ `SignalBlock` (1s) — well inside
  `HTTP_SHUTDOWN_TIMEOUT` (10s), `DrainTimeout` (5s), and the node deregister path in `node.go`.
  We do **not** rely on `BLPOP` being interrupted mid-block by ctx cancellation; the bounded
  timeout guarantees return regardless (and a `BLPOP` issued with an already-cancelled ctx
  returns `context.Canceled`, handled as a clean exit). Executor drain-context writes (P0.5) are
  untouched.
- **Redis connection-pool starvation (real, must fix).** N blocked `BLPOP`s hold N connections
  for up to `SignalBlock`. If `PoolSize < WorkerCount + heartbeat/claim/ack concurrency`, node
  heartbeats can starve → **false-dead nodes** → spurious reclaims. Mitigation: explicit
  `PoolSize ≥ WorkerCount + headroom` in `NewRedisClient`. (Alternative considered and rejected
  for complexity: a single per-node doorbell goroutine fanning out to idle workers via a Go
  channel — cleaner on connections but adds a coordination layer; with the default
  `WORKER_COUNT=5` and an explicit pool size, per-worker `BLPOP` is simpler and matches today's
  per-worker poll loop.)
- **In-process mode (`RUN_WORKERS=true`).** Same `pool.go`, so the server binary benefits
  automatically; its co-located scheduler/reaper are producers that signal regardless of consumer.
- **miniredis limitation.** v2.38.0 **does** implement `BLPOP`/`BRPOP`/`BZPOPMIN` (verified in
  its command table), and the tests already drive miniredis via a real go-redis client over its
  TCP addr, so the **wake path is unit-testable**. Caveat: miniredis `BLPOP` *timeouts* use real
  wall-clock and are **not** advanced by `mr.FastForward`. So unit tests exercise the wake path
  (push token → immediate claim) and use a *tiny* block (e.g. 50ms) for the fallback-timeout
  assertion; the full block-timeout + ctx-cancel behavior at the 1s setting is proven in the
  `//go:build integration` real-Redis test. This keeps `go test -race` (unit CI) hermetic and
  fast and green.
- **CI (gofmt/vet/`test -race`).** No new deps (`BLPOP` is an existing go-redis call). Blocking
  unit tests keep block durations tiny and prefer the deterministic wake path to avoid `-race`
  flakiness; the real-Redis test is build-tagged out of the default `test` target.

**Regression areas to re-verify:** the full submit→execute→retry→dead-letter pipeline; delayed
promotion (`promote.lua`); reaper lease-expiry and eager dead-node reclaim (`reclaim.lua`);
redrive; graceful shutdown/drain on both `cmd/server` and `cmd/worker`; the `docker compose
--scale worker=N` kill-a-worker zero-loss demo.

## Tasks

- [ ] Add `KeyReadySignal` + explicit `PoolSize` to `NewRedisClient`; both `main`s pass worker
      count. Prove: unit asserts pool size ≥ worker count; build green. `(api)`
- [ ] Add `SignalBlock` + `SignalCap` config with defaults + validation. Prove: config test for
      defaults and rejection of non-positive values. `(api)`
- [ ] Add `signal.lua` + `PriorityQueue.Signal(ctx)` and `WaitReady(ctx, timeout)` (`BLPOP`
      wrapper). Prove: unit — `Signal` pushes exactly one token and respects the cap; `WaitReady`
      returns true when a token is present. `(api)`
- [ ] Make `PriorityQueue.Enqueue` signal. Prove: unit — after `Enqueue`, `LLEN` of the signal
      list is 1; a blocked `WaitReady` returns. `(api)`
- [ ] Add the cap-guarded token push to `promote.lua` (per promoted ID). Prove: unit — promote 3
      delayed tasks ⇒ 3 tokens (capped); promotion counts unchanged. `(api)`
- [ ] Add the cap-guarded token push to `reclaim.lua` `"reclaimed"` branch only. Prove: unit —
      lease-expiry reclaim pushes 1 token; `dead_lettered`/`lost_race`/`orphan` push none;
      existing reaper tests still green. `(api)`
- [ ] Extend the `Broker` interface with `WaitForReady`; implement on `RedisBroker`. Prove:
      compile-time assertion + a unit that `WaitForReady` unblocks after an `Enqueue`. `(api)`
- [ ] Rewrite the `pool.go` empty-queue branch to block on `WaitForReady(ctx, SignalBlock)`;
      keep `pollInterval` for error back-off; handle timeout/cancel/error distinctly. Prove:
      unit (miniredis) — worker picks up an enqueued task promptly via the doorbell, and (short
      block) still picks up a task placed directly in `ready` with **no** token (fallback). `(api)`
- [ ] Thread `SignalBlock`/`SignalCap`/pool-size through `cmd/server` + `cmd/worker`. Prove:
      both binaries build; server in-process pool + standalone worker both start. `(api)`
- [ ] `//go:build integration` real-Redis test (Docker): `BLPOP` block-timeout + ctx-cancel
      shutdown within `SignalBlock`; capture before/after `enqueue_to_start_seconds`. Prove:
      `make test-integration` green against a local Redis. `(api)`
- [ ] Record numbers + honest caveat in `docs/BENCHMARKS.md`; update `PROGRESS.md` (P3.4 done,
      Decision Log entry: doorbell chosen over `BZPOPMIN`-direct, reasons). `(api)`

## Verification

- `make test` (miniredis, `-race`) and `make vet`, `gofmt -l` clean — full unit suite green
  including the new doorbell/producer/cap tests and all existing broker/reaper/queue tests.
- `make test-integration` against a local Docker Redis — blocking pickup, ctx-cancel shutdown
  ≤ `SignalBlock`, and the latency capture pass.
- **Latency measurement (the "measure & record" half).** The instrument already exists: the
  executor observes `enqueue_to_start_seconds` from `Task.CreatedAt` (P3.2). Drive a low/moderate
  arrival rate (queue usually empty — the regime where a worker would otherwise be mid-`Sleep`)
  against (1) the old poll build and (2) the doorbell build, and scrape `/metrics`. Expect the
  `enqueue_to_start_seconds` p50/p99 to drop by roughly the polling dead-time (up to ~`PollInterval`
  ≈ 500ms at p99 under bursty arrivals) to sub-millisecond wake. Record p50/p99 + machine specs
  in `docs/BENCHMARKS.md`, with the caveat that under sustained saturation (queue never empty) the
  two are equivalent — the win is at low/bursty load.
- Manual smoke: `docker compose up --scale worker=5`, submit a burst, confirm processing; `SIGTERM`
  a worker and confirm it drains and exits within ~1s; `docker kill` a worker mid-burst and
  confirm the existing zero-loss reclaim demo still holds (`submitted == completed + dead_lettered`).

## Open decisions (max 2, each with a recommendation)

1. **Merge shutdown-check interval and fallback-poll into one `SignalBlock` knob at 1s?**
   Recommend **yes, 1s**. The task names two timers (shutdown-check timeout, 1–5s fallback);
   a single `BLPOP` block value satisfies both — 1s bounds worst-case shutdown latency and
   worst-case dropped-token latency, comfortably inside the 5s drain / 10s shutdown budgets, while
   giving near-instant normal wake. Splitting them buys nothing here. (This is the only knob that
   plausibly wants your sign-off; everything else is decided above.)
2. **Signal push for the Go `Enqueue` path — shared `signal.lua` vs plain pipelined `LLEN`+`RPUSH`.**
   Recommend the **shared `signal.lua`** (atomic cap-check-and-push, one embedded script reused by
   `Enqueue`/`RedriveFailed`, same two lines inlined into `promote.lua`/`reclaim.lua`). A non-atomic
   Go `LLEN`+`RPUSH` would be harmless given the fallback backstop, but the Lua version is tidy,
   race-free, and keeps the cap logic in exactly one place.
