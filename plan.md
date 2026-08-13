# Plan — Phase 3 slice: structured logging (P3.1) + Prometheus metrics (P3.2)

**Project:** task-queue-educational-dashboard · **Track:** full · **Date:** 2026-08-13

## Approach

Two backend-only tasks, done in order (P3.1 then P3.2) because P3.2's metric
call sites sit right next to the log lines P3.1 rewrites, so touching each
source file once is cheaper.

**P3.1 — inject a `*slog.Logger`, no globals.** Each `main` builds one root
JSON logger (`slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))`)
and threads it through the existing `Deps`/`Config` structs — the same
dependency-injection seam Phase 0 established (`ExecutorDeps`, `reaper.Config`,
`worker.NodeConfig`, `api.HandlerDeps`). No package-level logger; every
consumer takes a `Logger` field and derives a child with `logger.With("task_id", id)`
or `logger.With("node_id", nodeID)` at the point where that key is known
(executor per task, node/pool per node, reaper per task). To keep the ~40
existing tests compiling and running without threading a logger everywhere, each
constructor falls back to `slog.Default()` when its `Logger` field is nil (same
nil-tolerant pattern already used for `WorkerState`/`Events`). The three
free-function middlewares in `api/middleware.go` become logger-aware: `NewRouter`
gains a `*slog.Logger` param and builds the `Logging`/`Recovery` closures around
it. `store.NewRedisClient` currently calls `log.Fatalf` on a failed ping; it
takes a `*slog.Logger` and does `logger.Error(...)` + `os.Exit(1)` instead.

**P3.2 — new `internal/telemetry/` package + `/metrics` on both binaries.**
One dependency added: `github.com/prometheus/client_golang v1.24.1` (confirmed
current stable via the Go module proxy; the one justified non-stdlib dep per the
Decision Log). `telemetry.New(reg *prometheus.Registry) *Metrics` builds and
registers the imperatively-updated collectors on a **dedicated** registry (not
the global default — avoids duplicate-registration panics across tests and lets
each test use a fresh registry):
- `tasks_processed_total{type,status}` — `*prometheus.CounterVec`
- `task_duration_seconds{type}` — `*prometheus.HistogramVec` (default buckets)
- `enqueue_to_start_seconds` — `prometheus.Histogram`
- `reaper_reclaims_total` — `prometheus.Counter`

`queue_depth` is a **scrape-time custom collector** (`prometheus.Collector`
reading `ZCARD` via the existing `QueuePeekStore`) rather than a periodically-set
gauge — no background goroutine, never stale, and it follows the existing
`ProcessingSize`/`DelayedSize`/`DeadLetterSize` read pattern. It is registered
**only on the broker/server**, where a `QueuePeekStore` exists; workers don't
construct one. `Metrics` is injected through the same `Deps` structs as the
logger and is nil-tolerant (a nil `*Metrics` no-ops), so existing tests and the
worker (which doesn't emit `queue_depth`) stay simple. The Prometheus counters
are **additive** to the existing Redis-backed `store.MetricsStore` (which still
feeds the dashboard's `/api/metrics`) — we do not remove it.

`/metrics` exposure: the **server** already runs an HTTP mux, so `NewRouter`
mounts `GET /metrics` → `promhttp.HandlerFor(reg, …)` alongside the `/api/*`
routes (metrics stay outside the `/api` prefix, per convention). The **worker**
has no HTTP server today, so `cmd/worker/main.go` gets a minimal
`http.Server` on a configurable port (`METRICS_PORT`, default `:9100`) started
in a goroutine before the blocking `node.Run(ctx)` and gracefully
`Shutdown`-ed after it returns.

**`enqueue_to_start_seconds` source — see Open decision 2.** No dedicated
"entered ready" timestamp exists on `model.Task`; `CreatedAt` (submission time,
already persisted in the hash and hydrated by `Dequeue`) is the closest
available signal. Recommendation: observe `time.Since(task.CreatedAt)` at
executor start for this slice — exact for the zero-delay loadgen workload P3.5
will drive, with a documented caveat for delayed/retried tasks — and defer a
precise `ready_at` (which needs Lua changes in `promote.lua`/`reclaim.lua`) to
when it's actually needed.

## Touched surface

### P3.1 — structured logging (all modified)
- `cmd/server/main.go` — build root `slog.Logger`; pass to `NewRedisClient`,
  `ExecutorDeps.Logger`, `reaper.New`/`Config`, `worker.NodeConfig.Logger`,
  `api.NewRouter`; replace the ~8 `log.Printf/Println` startup/shutdown lines.
- `cmd/worker/main.go` — same root logger + threading; replace its `log.*` lines.
- `internal/api/middleware.go` — `Logging`/`Recovery` take a `*slog.Logger`
  (built by `NewRouter`); structured request line (method/path/status/dur).
- `internal/api/router.go` — `NewRouter(h *Handler, logger *slog.Logger, metrics http.Handler)`.
- `internal/api/handler.go` — `HandlerDeps.Logger *slog.Logger` (nil→Default).
- `internal/worker/executor.go` — `ExecutorDeps.Logger`; per-task child logger
  `Logger.With("task_id", task.ID)`; replace all `log.Printf`.
- `internal/worker/node.go` — `NodeConfig.Logger`; `Logger.With("node_id", …)`.
- `internal/worker/pool.go` — `NewPool` gains a logger (or reuses executor's);
  replace `log.Printf` worker-lifecycle lines.
- `internal/queue/delayed.go` — logger field on `DelayedScheduler`; replace `log.*`.
- `internal/reaper/reaper.go` — `Logger` on `Reaper` (via `New`); per-task
  `Logger.With("task_id", …)`; replace all `log.Printf`.
- `internal/store/redis.go` — `NewRedisClient(addr, pass, logger)`; `log.Fatalf`
  → `logger.Error` + `os.Exit(1)`; `fmt.Printf("Connected…")` → `logger.Info`.

### P3.2 — Prometheus metrics
- `internal/telemetry/metrics.go` — **new.** `Metrics` struct, `New(reg)`,
  collector definitions, nil-tolerant helper methods
  (`ObserveTaskDuration`, `IncTasksProcessed`, `ObserveEnqueueToStart`, `IncReaperReclaims`).
- `internal/telemetry/queuedepth.go` — **new.** `queueDepthCollector`
  implementing `prometheus.Collector`, reading `QueuePeekStore` at scrape time;
  `RegisterQueueDepth(reg, source)`.
- `internal/telemetry/metrics_test.go` — **new.** Registry + collector unit tests.
- `internal/config/config.go` — **modified.** Add `MetricsPort` (`METRICS_PORT`,
  default `9100`) + validation.
- `cmd/server/main.go` — **modified.** Build `prometheus.NewRegistry()`,
  `telemetry.New(reg)`, `RegisterQueueDepth(reg, queuePeekStore)`; inject
  `*Metrics` into `ExecutorDeps`/`reaper.New`; pass promhttp handler to `NewRouter`.
- `cmd/worker/main.go` — **modified.** Registry + `telemetry.New`; inject into
  executor; start the minimal `/metrics` HTTP server on `METRICS_PORT`.
- `internal/worker/executor.go` — **modified.** `ExecutorDeps.Metrics *telemetry.Metrics`;
  observe duration + enqueue-to-start; `IncTasksProcessed(type, status)` on
  completed / retried / dead-lettered.
- `internal/reaper/reaper.go` — **modified.** `telemetry.Metrics` field;
  `IncReaperReclaims()` on the `reclaimed` (and dead-node reclaim) path.

## Impact & risk
- **Hot-path cost:** Prometheus `Inc`/`Observe` are in-process atomic ops, orders
  of magnitude cheaper than the Redis round-trips already on each task path —
  negligible. The `queue_depth` collector does one `ZCARD` per scrape (~every
  15s), not per task.
- **Label cardinality:** labels are `type` (bounded: sleep/http_fetch/hash +
  unknown) and `status` (completed/retried/failed) — both low-cardinality.
  Explicitly NOT labelling by `task_id`/`node_id` (unbounded) — those stay in
  logs only.
- **Worker HTTP server (new surface):** adds a listening socket to `cmd/worker`.
  Bind to `METRICS_PORT` (default `:9100`); must shut down cleanly on SIGTERM so
  `docker compose kill`/drain still exits promptly. Verify it doesn't clash with
  the API's `:8080` in single-binary mode (server serves `/metrics` on `:8080`,
  worker on `:9100`).
- **`model.Task` schema:** no change in this slice (using `CreatedAt`); flagged
  as Open decision 2 so we don't silently ship an imprecise metric.
- **Behavior to preserve:** existing `/api/metrics` + `/api/metrics/enhanced`
  (Redis-backed dashboard numbers) unchanged; the executor's Ack/Nack/reaper
  control flow unchanged — metrics/log calls are added, not reordered around the
  lease-fencing logic.
- **CI/gofmt:** new files must be `gofmt`-clean and `go vet`/`test -race` green
  (the gate is strict on `main`). `go.mod`/`go.sum` update from the one new dep;
  run `go mod tidy` and commit `go.sum`.
- **Regression areas:** submit → execute → retry → dead-letter pipeline; reaper
  reclaim + dead-node reclaim; graceful shutdown of both binaries (now with an
  extra HTTP server on the worker); the full existing test suite.

## Tasks
- [ ] **P3.1a** Thread `*slog.Logger` through `store.NewRedisClient`, `ExecutorDeps`,
  `reaper` (`New`/`Config`), `worker.NodeConfig`/`Pool`, `DelayedScheduler`,
  `api.HandlerDeps`/`NewRouter`/middleware; nil→`slog.Default()`. Replace every
  `log.Printf/Println/Fatalf` with structured `slog` calls keyed by
  `task_id`/`node_id` via child loggers. `(api)` — *verify:* `rg '"log"|log\.(Printf|Println|Fatalf)' cmd internal` returns nothing; `go build ./...` + full suite green.
- [ ] **P3.1b** Root JSON logger built in both `main.go`s and injected. `(api)` —
  *verify:* run the server binary against miniredis/a manual Redis and confirm
  stdout is JSON lines carrying `task_id` on task events and `node_id` on node
  events (assert shape in a small `slog` handler unit test capturing a `bytes.Buffer`).
- [ ] **P3.2a** Add `client_golang v1.24.1` (`go get` + `go mod tidy`); create
  `internal/telemetry` with `New(reg)`, the four imperative collectors, and
  nil-tolerant helper methods. `(api)` — *verify:* `telemetry_test.go` registers
  on a fresh registry, calls each helper, and asserts via `testutil.CollectAndCount`
  / `testutil.ToFloat64` that samples land with the right labels.
- [ ] **P3.2b** `queueDepthCollector` + `RegisterQueueDepth`; scrape reads
  `ZCARD` through `QueuePeekStore`. `(api)` — *verify:* miniredis test seeds the
  ready set, registers the collector, and asserts `queue_depth` via
  `testutil.CollectAndCompare`.
- [ ] **P3.2c** Wire metrics at sources: executor observes `task_duration_seconds`,
  `enqueue_to_start_seconds`, and `tasks_processed_total{type,status}`; reaper
  increments `reaper_reclaims_total`. `(api)` — *verify:* miniredis executor test
  drives a success + a dead-letter and asserts the counters advanced with correct
  labels; reaper test asserts `reaper_reclaims_total` after a reclaim.
- [ ] **P3.2d** Expose `/metrics`: `NewRouter` mounts it on the server; add
  `MetricsPort` config + a minimal `/metrics` HTTP server to `cmd/worker/main.go`
  started/stopped around `node.Run`. `(api)` — *verify:* `httptest` request to
  the server router returns 200 with `text/plain` Prometheus exposition and
  contains the metric names; `go build ./cmd/...` for both binaries.

## Verification
- `gofmt -l cmd internal` → empty; `go vet ./...` clean; `go build ./...` and
  `go build ./cmd/server ./cmd/worker` clean.
- `go test ./... -race -count=1` — existing ~40 tests still green **plus** the
  new telemetry/executor/reaper/logging assertions.
- `rg -n '"log"|log\.(Printf|Println|Fatalf)' cmd internal` → no matches (P3.1 done).
- Manual (no Docker needed): run `cmd/server` against a local/miniredis-backed
  Redis, `curl :8080/metrics` and confirm all five metric names appear and
  `queue_depth` reflects seeded ready tasks; submit a task and watch a JSON log
  line with `task_id`. (Kripa runs the full Docker stack + Prometheus scrape and
  the loadgen ramp separately — that's P3.3/P3.5, out of this slice.)

## Open decisions (max 1–2, each with a recommendation)
1. **Worker `/metrics` port.** Recommend `METRICS_PORT` env, default `:9100`
   (fixed container port so Prometheus scrapes scaled workers over the compose
   network; server keeps `/metrics` on its existing `:8080`). Alternative `:2112`
   (client_golang example convention) — no functional difference; `:9100` reads
   as "the metrics port" and avoids the API port.
2. **`enqueue_to_start_seconds` source now vs precise `ready_at` field.**
   Recommend using `CreatedAt` this slice (zero schema/Lua change; exact for the
   zero-delay loadgen benchmark, documented caveat for delayed/retried tasks) and
   deferring a dedicated `ready_at` timestamp — which requires writing the field
   inside `promote.lua`/`reclaim.lua` — until a later Phase 3 task actually needs
   sub-metric precision. If you'd rather have it exact from day one, say so and
   I'll fold the `model.Task.ReadyAt` + Lua changes into P3.2c.
