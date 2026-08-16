# Benchmarks

Honest, machine-specific performance numbers with their caveats. This file is the
home for the "measure & record" half of Phase 3. It starts with the P3.4 blocking
task-pickup result and continues with the P3.7 zero-loss soak and P3.8 throughput /
latency numbers.

## Machine / environment

| Field | Value |
|---|---|
| CPU | Apple M5 (10 cores) |
| Memory | 16 GB |
| OS | macOS (darwin/arm64) |
| Go | go1.26.x |
| Redis | `redis:7-alpine` in Docker, published on a local port |
| Date | 2026-08-13 (P3.4 micro-benchmark) · 2026-08-16 (P3.7 soak + P3.8 throughput/latency) |

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

## P3.7 — Zero-loss soak (harness)

`scripts/soak.sh` drives a sustained load with `cmd/loadgen`, optionally kills and
restores a worker mid-run (chaos), waits for the queue to fully drain, and then
asserts the **zero-loss invariant** from Redis ground truth:

```
ready == 0 && processing == 0 && delayed == 0
AND submitted == processed + failed      (accepted == completed + dead-lettered)
```

`submitted` counts only *accepted* submissions — backpressure (P3.6) sheds before
the counter increments — so it is the correct denominator even with
`MAX_QUEUE_DEPTH` active. `retries` are re-attempts, not terminal, and are excluded.

Reproduce (from the repo root, with the stack running at `--scale worker=3`):

```bash
# steady-state — below saturation, loss-free with margin (the ~49/s mix ceiling
# on this machine; pick a rate under your own measured ceiling)
SOAK_DURATION=20m SOAK_RATE=40 ./scripts/soak.sh

# kill+restore a worker mid-run (chaos) — proves zero loss across a node death
SOAK_DURATION=8m SOAK_RATE=40 SOAK_CHAOS=1 ./scripts/soak.sh

# sustained overload — exercises backpressure: the ready queue pins at
# MAX_QUEUE_DEPTH and excess submissions shed as 429 + Retry-After. RAISE the
# drain timeout well above its 300 s default, or the post-load drain of a full
# ~20k-deep queue at ~49/s (~7 min) trips a *false* FAIL on the drain timer even
# though the zero-loss invariant holds.
SOAK_DURATION=45m SOAK_RATE=250 SOAK_DRAIN_TIMEOUT=900 ./scripts/soak.sh
```

Config is env-driven (`SOAK_RATE`, `SOAK_DURATION`, `SOAK_MIX`, `SOAK_CONCURRENCY`,
`SOAK_WORKERS`, `SOAK_CHAOS`, `SOAK_DRAIN_TIMEOUT`, …; see the script header). It
prints a PASS/FAIL summary and exits non-zero on a loss/drain violation, so it
doubles as a check. Choose `SOAK_RATE` relative to your machine's throughput
ceiling (below → steady-state, above → backpressure); the numbers below are for
this machine at `--scale worker=3`.

### Results

Measured 2026-08-16 on the machine above, 3 worker replicas
(`docker compose up -d --scale worker=3`), mix `hash:50,sleep:50` unless noted.
All three soaks satisfied the zero-loss invariant with the queue fully drained.

#### Zero-loss soak

| Regime | Offered rate | Duration | Accepted | Completed | Dead-lettered | Retries | Reaper reclaims | Shed (429) | Zero-loss |
|---|---|---|---|---|---|---|---|---|---|
| **Steady-state** | 40/s | 20 min | 47,999 | 47,784 | 215 | 10,064 | 4 | 0 | ✅ 47,999 = 47,784 + 215 |
| **Chaos** (kill + restore one worker) | 40/s | 8 min | 19,199 | 19,116 | 83 | 4,106 | 9 | 0 | ✅ 19,199 = 19,116 + 83 |
| **Sustained overload** | 250/s | 45 min | 132,385 | 131,883 | 502 | 27,655 | 0 | 542,613 | ✅ 132,385 = 131,883 + 502 |

- *Accepted* = submissions the broker admitted (backpressure sheds **before** the
  counter increments), so it is the correct zero-loss denominator even with
  `MAX_QUEUE_DEPTH` active. *Dead-lettered* tasks exhausted their retries and are
  terminal — they land in the DLQ, not lost. *Retries* are re-attempts and are
  deliberately excluded from the invariant.
- **Chaos:** killing a worker for 30 s mid-run cost zero tasks. The reaper reclaimed
  the dead node's **9** in-flight leases on visibility-timeout expiry and they were
  retried to completion — the kill-a-worker demo, proven loss-free from Redis ground
  truth.
- **Sustained overload:** offering 250/s against a mix that sustains only ~49/s pinned
  the ready queue at the `MAX_QUEUE_DEPTH = 20000` cap for the entire run; backpressure
  shed **542,613** excess submissions as `429 + Retry-After` with **0 errors**, and
  every *accepted* task still reached a terminal state. This is the backpressure
  regime, not a steady state. (The harness's own 300 s drain timer expires mid-drain
  here — a false FAIL on the timer, not data loss; the full drain to `ready=0` confirms
  `132,385 = 131,883 + 502`.)

#### Throughput ceiling (max sustainable, 3 workers)

| Task type | Payload | Max sustained | Bounded by |
|---|---|---|---|
| `hash` (CPU-bound) | 100,000 SHA rounds | **~580 tasks/s** (peak 612) | CPU across cores |
| `sleep` (latency-bound) | fixed 500 ms | **~21 tasks/s** | worker concurrency slots (~10–11) ÷ per-task latency |
| mixed `hash:50,sleep:50` | handler defaults | **~49 tasks/s** | the `sleep` half — the slow leg caps the mix |

The ~27× spread between `hash` and `sleep` is the headline. A cheap CPU task streams
through at hundreds/s; a task that *holds a worker slot* for 500 ms is capped by
(slots ÷ latency) no matter how much CPU is idle. That is exactly why the 50/50 mix
saturates at ~49/s, why **40/s** was chosen for the steady soak (loss-free with
margin), and why **250/s** exercises backpressure rather than steady throughput.
Under an oversubscribed offer (2000/s of `hash`) the queue simply pins at the 20k
backpressure cap while workers drain at their ~580/s ceiling.

#### Latency

*Task processing time* — `task_duration_seconds`, handler work only:

| Task type | p50 | p95 | p99 |
|---|---|---|---|
| `hash` (100k rounds, under CPU contention at saturation) | 21.8 ms | 47.9 ms | 74.9 ms |
| `sleep` (500 ms handler) | ~0.75 s | ~0.98 s | ~0.99 s |

*Pickup latency* — `enqueue_to_start_seconds`, ready → claimed — is governed by
**queue depth, not the pickup mechanism**:

- **Sub-capacity load (8/s):** median ≈ **3 ms** — the doorbell (P3.4) wakes an idle
  worker within a few ms of a task becoming ready.
- **Near-capacity / overload (40/s mixed, 250/s):** p99 climbs to **seconds** because
  tasks wait *behind the queue*, not for a worker to notice them. This is fundamental
  queueing (Little's law), **not** a doorbell regression — the isolated doorbell
  micro-benchmark (§P3.4) still measures the pickup step itself at p50 ≈ 0.35 ms /
  p99 ≈ 0.86 ms.

**Takeaway.** The queue is loss-free across steady, chaos, and sustained-overload
regimes; sustainable throughput is set by the *slowest task type in the mix*; and low
pickup latency requires the offered load to stay below that throughput ceiling —
above it, **backpressure (not loss)** is the release valve.
