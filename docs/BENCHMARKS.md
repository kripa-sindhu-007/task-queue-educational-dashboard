# Benchmarks

Honest, machine-specific performance numbers with their caveats. This file is the
home for the "measure & record" half of Phase 3. It starts with the P3.4 blocking
task-pickup result; P3.7/P3.8 will extend it with throughput and a zero-loss soak.

## Machine / environment

| Field | Value |
|---|---|
| CPU | Apple M5 (10 cores) |
| Memory | 16 GB |
| OS | macOS (darwin/arm64) |
| Go | go1.26.x |
| Redis | `redis:7-alpine` in Docker, published on a local port |
| Date | 2026-08-13 |

Reproduce with:

```bash
docker run -d --rm -p 6399:6379 --name tq-p34-redis redis:7-alpine
cd backend
REDIS_ADDR=localhost:6399 go test -tags=integration ./internal/worker/ -run Integration -v
docker rm -f tq-p34-redis
```

## P3.4 — Blocking task pickup (doorbell) vs sleep-polling

**What changed.** Idle workers used to `time.Sleep(PollInterval)` (default 500 ms)
between empty `Dequeue`s. A task submitted just after a worker went to sleep waited
out the rest of that sleep before it was even *seen*. P3.4 replaces the sleep with a
**doorbell**: the worker blocks on `BLPOP taskqueue:ready:signal <SignalBlock>` and
every ready-producer rings the doorbell (one capped token per newly-ready task), so a
freshly-enqueued task wakes a blocked worker almost immediately. Correctness is
unchanged — the task still leaves `ready` only through the unchanged atomic claim in
`dequeue.lua`; the token is a pure wake-up (see the Decision Log entry and
`plan.md`).

**Metric.** `enqueue_to_start` — the time from a task becoming ready to a worker
picking it up. Measured directly (Signal → `WaitForReady` returns) against real Redis
over 200 iterations, contrasted with a faithful model of the old sleep-poll loop (a
task arriving at a uniform phase within the poll cycle waits out the remainder of the
cycle).

| Regime | p50 | p99 |
|---|---|---|
| **Doorbell (after)** | ~0.35 ms | ~0.86 ms |
| **Sleep-poll (before), `PollInterval = 500 ms`** | ~250 ms | ~495 ms |

(Representative run: doorbell p50 = 354 µs, p99 = 862 µs; sleep-poll p50 = 250 ms,
p99 = 495 ms. Absolute numbers vary run-to-run and by machine; the ~1000× wake-time
reduction is the point.)

### The honest caveat

**This win only appears at low / bursty load — the regime where a worker would
otherwise be mid-sleep.** Under sustained saturation (the ready queue is never
empty), a worker never blocks on the doorbell at all: it drains task after task in a
tight `Dequeue` loop, and the two builds are equivalent. The doorbell removes
*polling dead-time*, not per-task processing cost. So:

- **Bursty / trickle traffic:** p50/p99 `enqueue_to_start` drops from ~`PollInterval/2`
  / ~`PollInterval` to sub-millisecond. This is the headline.
- **Saturated traffic:** no measurable difference — throughput is bounded by handler
  work and Redis round-trips, not by the pickup mechanism.

### Real-Redis constraints worth knowing

- **`SignalBlock` has a practical floor of 1 s.** go-redis truncates any BLPOP timeout
  below 1 s up to 1 s, so the default `SIGNAL_BLOCK_MS=1000` is the smallest block the
  client will actually honor. This is also the shutdown-check granularity and the
  fallback-poll backstop — all one knob.
- **Mid-block ctx-cancel is *not* honored by go-redis.** A `BLPOP` already in flight
  runs to its timeout even if the context is cancelled; go-redis does not abort it. So
  graceful shutdown is bounded by `SignalBlock` returning (then the worker loop
  re-checks `ctx.Done()` and exits), **not** by interrupting the block. A call issued
  with an already-cancelled context does return `context.Canceled` promptly. Both
  behaviors are proven in `internal/worker/blocking_integration_test.go`
  (`//go:build integration`), because miniredis's BLPOP timeout uses real wall-clock
  and is not advanced by `mr.FastForward`.

## TODO (P3.7 / P3.8)

- Throughput under a `cmd/loadgen` ramp (100 → 2000 tasks/sec) with Grafana p50/p99.
- 30-minute sustained soak asserting `submitted == completed + dead_lettered` (zero
  loss), including a kill-a-worker chaos interval.
