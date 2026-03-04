# I Built a Distributed Task Queue From Scratch to Actually Understand How They Work

## And open-sourced the whole thing with a live dashboard that visualizes every internal

---

Most backend engineers use task queues daily — Celery, Bull, Sidekiq, Asynq. We enqueue jobs, configure retries, maybe set up a dead-letter queue. But how many of us have actually built one from scratch?

I hadn't. So last weekend, I did.

The result is an **educational distributed task queue** with a real-time dashboard that lets you *see* tasks flow through the system — from submission to priority scheduling, worker execution, retry logic, and dead-letter handling.

This article breaks down every design decision, data structure, and concept I implemented. Whether you're preparing for a system design interview or just want to understand task queues at a deeper level, this should help.

**GitHub**: https://github.com/kripa-sindhu-007/task-queue-educational-dashboard

---

## The Goal

I didn't want to build a production task queue. I wanted to build something that **teaches**.

The requirements I set:

- Priority-based task scheduling (higher priority = processed first)
- Delayed execution (schedule a task to run N seconds later)
- Automatic retries with exponential backoff
- Dead-letter queue for permanently failed tasks
- A live dashboard that visualizes every stage of the pipeline
- Single command to run everything (`docker compose up --build`)

The stack: **Go** for the backend (goroutines are perfect for worker pools), **Redis** for the queue engine (sorted sets give us O(log N) priority queues), **Next.js** for the dashboard, and **Docker** to tie it all together.

---

## System Architecture — The 30,000-Foot View

```
┌──────────────────┐     ┌──────────────────────┐     ┌───────────┐
│   Next.js 15     │────▶│   Go 1.22 Backend    │────▶│  Redis 7  │
│   Dashboard      │     │   HTTP API + Workers  │     │           │
│   :3000          │     │   :8080               │     │   :6379   │
└──────────────────┘     └──────────────────────┘     └───────────┘
```

Three containers, one `docker-compose.yml`. The frontend polls the backend's REST API. The backend manages the worker pool and talks to Redis for all state.

There's no database. Redis *is* the database. Every queue, every metric, every event log entry lives in Redis data structures. This keeps the system simple and fast.

---

## Low-Level Design — Data Structures and Redis Keys

This is where it gets interesting. The entire system runs on **6 Redis keys**, each using a different Redis data type chosen for its specific access pattern.

### The 6 Redis Keys

| Key | Redis Type | Purpose | Why This Type |
|-----|-----------|---------|---------------|
| `taskqueue:ready` | Sorted Set | Priority queue for tasks ready to execute | ZADD + ZPOPMIN gives atomic priority dequeue in O(log N) |
| `taskqueue:delayed` | Sorted Set | Tasks scheduled for future execution | Score = Unix timestamp, ZRANGEBYSCORE finds due tasks |
| `taskqueue:deadletter` | List | Permanently failed tasks | LPUSH for append, LRANGE for paginated reads |
| `taskqueue:metrics` | Hash | Counter metrics (processed, failed, retries, submitted) | HINCRBY for atomic increments, HGETALL for bulk read |
| `taskqueue:events` | List | Append-only event log (capped at 200) | LPUSH + LTRIM for bounded log, LRANGE for recent events |
| `taskqueue:workers` | Hash | Per-worker state (idle/processing) | HSET per worker, HGETALL for dashboard snapshot |

### The Task Model

Every task flowing through the system carries this structure:

```go
type Task struct {
    ID         string     `json:"id"`
    Priority   int        `json:"priority"`      // 1-10, higher = first
    Delay      int        `json:"delay"`          // seconds before eligible
    MaxRetries int        `json:"max_retries"`    // 0 = no retries
    Retries    int        `json:"retries"`        // current attempt count
    Status     TaskStatus `json:"status"`         // pending|processing|completed|failed
    CreatedAt  time.Time  `json:"created_at"`
    Error      string     `json:"error,omitempty"`
}
```

Simple and flat. No nested objects, no foreign keys, no joins. Just JSON in and out of Redis.

---

## Concept #1 — Priority Queue with Redis Sorted Sets

This is the heart of the system. A **priority queue** ensures that high-priority tasks get processed before low-priority ones, regardless of insertion order.

### Why Not a Regular List?

A Redis List (LPUSH/RPOP) gives you FIFO — first in, first out. But if a priority-10 task arrives after a thousand priority-1 tasks, it would wait behind all of them. That's not what we want.

### How Sorted Sets Solve This

A Redis Sorted Set (ZSET) stores members with a numeric score. Members are ordered by score, and you can pop the lowest score atomically.

The trick: **store the score as negative priority**.

```go
func (q *PriorityQueue) Enqueue(ctx context.Context, task model.Task) error {
    data, _ := json.Marshal(task)
    return q.client.ZAdd(ctx, "taskqueue:ready", redis.Z{
        Score:  float64(-task.Priority),  // priority 10 → score -10
        Member: string(data),
    }).Err()
}
```

- Priority 10 → score -10
- Priority 1 → score -1
- ZPOPMIN always pops the lowest score → highest priority task

```go
func (q *PriorityQueue) Dequeue(ctx context.Context) (*model.Task, error) {
    results, err := q.client.ZPopMin(ctx, "taskqueue:ready", 1).Result()
    if len(results) == 0 {
        return nil, nil  // queue empty
    }
    var task model.Task
    json.Unmarshal([]byte(results[0].Member.(string)), &task)
    return &task, nil
}
```

**ZPOPMIN is atomic** — if 5 workers call it simultaneously, each gets a different task. No locks needed. Redis handles the concurrency for us.

### Time Complexity

| Operation | Complexity |
|-----------|-----------|
| Enqueue (ZADD) | O(log N) |
| Dequeue (ZPOPMIN) | O(log N) |
| Size (ZCARD) | O(1) |

For comparison, a naive "scan all items for highest priority" approach would be O(N). Sorted sets give us logarithmic time regardless of queue size.

---

## Concept #2 — Delayed Execution

Some tasks shouldn't run immediately. Maybe you want to schedule a notification for 30 seconds from now, or retry a failed task after a backoff period.

### Design

Delayed tasks go into a separate sorted set where the **score is the Unix timestamp** when the task should become eligible:

```go
func (d *DelayedScheduler) Schedule(ctx context.Context, task model.Task, delay time.Duration) error {
    data, _ := json.Marshal(task)
    executeAt := time.Now().Add(delay).Unix()
    return d.client.ZAdd(ctx, "taskqueue:delayed", redis.Z{
        Score:  float64(executeAt),
        Member: string(data),
    }).Err()
}
```

A background goroutine runs every second, checking for due tasks:

```go
func (d *DelayedScheduler) promoteDueTasks(ctx context.Context) {
    now := float64(time.Now().Unix())
    results, _ := d.client.ZRangeByScoreWithScores(ctx, "taskqueue:delayed", &redis.ZRangeBy{
        Min: "-inf",
        Max: fmt.Sprintf("%f", now),
    }).Result()

    for _, z := range results {
        removed, _ := d.client.ZRem(ctx, "taskqueue:delayed", z.Member).Result()
        if removed == 0 {
            continue  // another instance already grabbed it
        }
        // Promote to ready queue
        d.queue.Enqueue(ctx, task)
    }
}
```

The `ZRem` check is crucial — if you're running multiple instances, two schedulers might see the same due task. The `removed == 0` check ensures only one actually promotes it. This is **optimistic concurrency control** without locks.

---

## Concept #3 — Worker Pool with Goroutines

The worker pool is where Go really shines. Each worker is a goroutine that runs an infinite loop: dequeue → execute → repeat.

```go
func (p *Pool) Start(ctx context.Context) {
    for i := 0; i < p.workerCount; i++ {
        p.wg.Add(1)
        go p.worker(ctx, i)
    }
}

func (p *Pool) worker(ctx context.Context, id int) {
    defer p.wg.Done()
    for {
        select {
        case <-ctx.Done():
            return  // graceful shutdown
        default:
        }

        task, _ := p.queue.Dequeue(ctx)
        if task == nil {
            time.Sleep(p.pollInterval)  // back off when empty
            continue
        }

        p.activeCount.Add(1)
        p.executor.Execute(ctx, task, id)
        p.activeCount.Add(-1)
    }
}
```

### Key Design Decisions

**Polling vs. blocking**: Workers poll Redis with ZPOPMIN every 500ms when the queue is empty. An alternative is Redis' BZPOPMIN (blocking pop), but polling gives us cleaner shutdown semantics and lets us track idle workers.

**Active count tracking**: An `atomic.Int64` tracks how many workers are currently executing. No mutex needed — `atomic.Add` is lock-free and safe from any goroutine.

**Graceful shutdown**: When the context is cancelled (SIGINT/SIGTERM), workers finish their current task before exiting. The `p.wg.Wait()` in the main function blocks until all workers are done.

---

## Concept #4 — Retry with Exponential Backoff

When a task fails, we don't just retry immediately. That would hammer the system if the failure is caused by a temporary issue (like a downstream service being overloaded).

Instead, we use **exponential backoff**: each retry waits exponentially longer.

```go
func (e *Executor) handleFailure(ctx context.Context, task *model.Task, workerID int) {
    if task.Retries < task.MaxRetries {
        task.Retries++
        backoff := math.Min(math.Pow(2, float64(task.Retries)), 60)
        delay := time.Duration(backoff) * time.Second
        e.delayed.Schedule(ctx, *task, delay)
    } else {
        // Exhausted retries → dead-letter
        e.deadLetter.Push(ctx, model.FailedTask{
            Task:     *task,
            FailedAt: time.Now(),
            Reason:   task.Error,
        })
    }
}
```

### The Backoff Curve

| Retry # | Backoff | Total Wait |
|---------|---------|-----------|
| 1 | 2s | 2s |
| 2 | 4s | 6s |
| 3 | 8s | 14s |
| 4 | 16s | 30s |
| 5 | 32s | 62s |
| 6+ | 60s (capped) | +60s each |

The cap at 60 seconds prevents absurdly long waits. In production systems, you'd also add **jitter** (random offset) to prevent thundering herd problems when many tasks retry simultaneously.

### The Retry Flow

Failed task → increment retry counter → calculate backoff → push to delayed queue → delayed scheduler promotes it when ready → worker picks it up → try again.

The task re-enters the same pipeline. The delayed queue doesn't care whether a task is a first-time delayed submission or a retry — it's the same ZADD operation.

---

## Concept #5 — Dead-Letter Queue

When a task exhausts all retries, it's "dead." We don't discard it — we move it to a **dead-letter queue** (DLQ) for investigation.

```go
type FailedTask struct {
    Task     Task      `json:"task"`
    FailedAt time.Time `json:"failed_at"`
    Reason   string    `json:"reason"`
}
```

The DLQ is a simple Redis List. LPUSH to add (newest first), LRANGE to paginate.

In production systems, dead-letter queues serve multiple purposes:
- **Debugging**: Inspect why tasks failed
- **Replay**: Fix the bug, then re-enqueue DLQ tasks
- **Alerting**: Monitor DLQ size as a health signal
- **Auditing**: Track failure patterns over time

Our dashboard shows the DLQ as a table with task ID, priority, attempt count, failure reason, and timestamp — making it easy to understand what went wrong.

---

## Concept #6 — Event Sourcing (Lite)

To power the Activity Log, every state change in the system emits an event:

```go
type TaskEvent struct {
    ID        string    `json:"id"`
    TaskID    string    `json:"task_id"`
    Type      string    `json:"type"`       // submitted|started|completed|failed|retrying|dead_lettered|promoted
    WorkerID  int       `json:"worker_id"`
    Detail    string    `json:"detail"`
    Timestamp time.Time `json:"timestamp"`
}
```

Events are pushed to a Redis List, capped at 200 entries via LTRIM:

```go
func (e *EventStore) Push(ctx context.Context, event model.TaskEvent) error {
    data, _ := json.Marshal(event)
    pipe := e.client.Pipeline()
    pipe.LPush(ctx, "taskqueue:events", string(data))
    pipe.LTrim(ctx, "taskqueue:events", 0, 199)
    _, err := pipe.Exec(ctx)
    return err
}
```

The pipeline batches both commands into a single Redis round-trip. The list never grows beyond 200 entries — old events are automatically discarded.

This gives us a complete trace of every task's lifecycle:

```
12:26:27 PM  submitted   batch-119-965 — Priority=3, Delay=2s, MaxRetries=3
12:26:27 PM  promoted    batch-119-965 — Moved from delayed to ready queue
12:26:28 PM  started     batch-119-965 W2 — Worker 2 picked up task
12:26:28 PM  failed      batch-119-965 W2 — simulated failure
12:26:28 PM  retrying    batch-119-965 W2 — Retry 1/3 in 2s
12:26:30 PM  promoted    batch-119-965 — Moved from delayed to ready queue
12:26:30 PM  started     batch-119-965 W4 — Worker 4 picked up task
12:26:31 PM  completed   batch-119-965 W4 — Completed in 354ms
```

---

## The Dashboard — Designing for Understanding

The dashboard isn't an afterthought — it's the whole point. Every panel is designed to answer a specific question about how the system works.

### Layout

```
┌─────────────────────────────────────────────────────────┐
│  Task Queue Dashboard                                   │
│  "An educational view into distributed task processing" │
├─────────────────────────────────────────────────────────┤
│  TASK FLOW DIAGRAM (full width)                         │
│  [Submit]→[Delayed Queue]→[Ready Queue]→[Workers]→[Out] │
├──────────────────┬──────────────────────────────────────┤
│  Submit Form     │  Enhanced Metrics (8 stat cards +    │
│  (350px)         │  success rate bar)                   │
├──────────────────┴──────────────────────────────────────┤
│  WORKER POOL (full width)                               │
│  [W0: idle] [W1: processing task-x] [W2: idle] ...     │
├──────────────────┬──────────────────────────────────────┤
│  Queue Contents  │  Activity Log (terminal-style)       │
│  Ready + Delayed │  Live event stream                   │
├──────────────────┴──────────────────────────────────────┤
│  Failed Tasks Table (full width)                        │
└─────────────────────────────────────────────────────────┘
```

### Panel Breakdown

**Task Flow Pipeline** (polls every 3s) — A horizontal visualization showing task counts at each stage: Submitted → Delayed Queue → Ready Queue → Workers Active → Outcomes (Completed/Retried/Dead Letter). This answers: *"Where are my tasks right now?"*

![Dashboard Top — Pipeline, Submit Form, Metrics](docs/dashboard-top.png)

**Worker Pool** (polls every 1s) — Cards for each worker goroutine. Idle workers show a grey badge. Processing workers pulse blue with the current task ID and an elapsed time counter. This answers: *"What is each worker doing?"*

**Queue Contents** (polls every 2s) — Peek into the ready queue (sorted by priority) and delayed queue (with countdown timers). Tasks appear and disappear as they're enqueued and dequeued. This answers: *"What's waiting to be processed?"*

**Activity Log** (polls every 1s) — A terminal-style live event stream. Each event type has a color-coded badge: green for completed, red for failed, amber for retrying, blue for started. This answers: *"What just happened?"*

![Dashboard Middle — Workers, Queues, Activity Log](docs/dashboard-middle.png)

**Failed Tasks Table** (polls every 5s) — Dead-letter entries with task ID, priority, attempt count (e.g., "4/4"), failure reason, and timestamp. This answers: *"Which tasks permanently failed, and why?"*

![Dashboard Bottom — Activity Log, Dead Letter Table](docs/dashboard-bottom.png)

### Frontend Tech Choices

| Choice | Reasoning |
|--------|-----------|
| **Polling (not WebSockets)** | Simpler to implement, easier to understand, good enough for educational purposes. Each panel polls independently at different rates. |
| **Tailwind CSS + shadcn/ui** | Consistent dark theme with minimal custom CSS. Cards, Badges, Buttons, Inputs — all pre-built. |
| **Framer Motion** | Smooth animations for worker state changes, event entries appearing, queue items entering/leaving. Makes the dashboard feel alive. |
| **Custom `usePolling` hook** | Replaces repetitive useEffect + setInterval pattern across 6 components. |

```typescript
function usePolling<T>(fetcher: () => Promise<T>, intervalMs: number) {
    const [data, setData] = useState<T | null>(null);
    const [error, setError] = useState<string | null>(null);

    useEffect(() => {
        const poll = async () => {
            try {
                setData(await fetcher());
                setError(null);
            } catch (err) {
                setError(err.message);
            }
        };
        poll();
        const id = setInterval(poll, intervalMs);
        return () => clearInterval(id);
    }, [intervalMs]);

    return { data, error };
}
```

---

## The Complete Task Lifecycle — Traced End to End

Let's trace a single task through the entire system:

**1. Submission** — User clicks "Submit Task" with priority=8, delay=5s, max_retries=3.

The API handler creates the task, emits a "submitted" event, increments the submitted counter, and adds it to the delayed queue with score = now + 5.

**2. Waiting** — The task sits in the delayed sorted set. The dashboard's Queue Contents panel shows it with a "5s" countdown timer.

**3. Promotion** — After 5 seconds, the delayed scheduler's tick finds the task (score <= now), atomically removes it from the delayed set, and enqueues it in the ready queue. A "promoted" event is emitted.

**4. Pickup** — Worker 2 calls ZPOPMIN, gets the task (it's priority 8, so it jumped ahead of lower-priority tasks). Worker state is set to "processing" with the task ID. A "started" event is emitted. The Worker Pool panel shows W2 pulsing blue.

**5a. Success (70% chance)** — After 200-800ms of simulated work, the task completes. Status set to "completed", processed counter incremented, worker state reset to "idle". A "completed" event is emitted.

**5b. Failure (30% chance)** — The task fails. A "failed" event is emitted. Since retries (0) < maxRetries (3), the retry counter increments to 1, and the task is scheduled in the delayed queue with a 2-second backoff. A "retrying" event is emitted. The cycle repeats from step 2.

**5c. Dead Letter** — If the task fails on its 4th attempt (retries=3, maxRetries=3), it's pushed to the dead-letter list. A "dead_lettered" event is emitted. It appears in the Failed Tasks table.

---

## Batch Testing — Stress the System

The dashboard includes batch submit buttons (1k, 5k, 10k) that open a configuration dialog:

- **Priority range** (min/max) — Control the priority distribution
- **Delay chance** (%) — What percentage of tasks should be delayed
- **Max delay** (seconds) — Upper bound for random delay
- **Max retries** — Retry limit per task

Submitting 5,000 tasks floods the system and produces beautiful chaos on the dashboard — queues filling up, all 5 workers processing simultaneously, events streaming in, retry counts climbing, and the occasional task landing in the dead-letter queue.

There's also a "Clear All Data" button (with a confirmation dialog) to reset Redis and start fresh.

---

## What I'd Do Differently in Production

This is an educational system. Here's what a production version would need:

| Educational Version | Production Version |
|---|---|
| Polling (1-5s intervals) | WebSockets or SSE for real-time push |
| In-memory Redis (volatile) | Redis with AOF persistence + replicas |
| Simulated work (random sleep) | Actual task handlers with business logic |
| Single binary, all-in-one | Separate API server and worker processes |
| Fixed 5 workers | Auto-scaling based on queue depth |
| No authentication | API keys, rate limiting, RBAC |
| LTRIM at 200 events | Proper event store (Kafka, NATS) |
| No jitter on backoff | Backoff + jitter to prevent thundering herd |
| `json.Marshal` for queue items | Protobuf or MessagePack for efficiency |

---

## On Using AI as a Coding Partner

I used **Claude** throughout this build. To be transparent about the workflow:

- **Mine**: System design, architecture decisions, data structure choices, what to build and why
- **Claude's help**: Go syntax when I was rusty, React component boilerplate, wiring repetitive CRUD code faster

It's like pair programming with someone who has infinite patience and encyclopedic syntax knowledge. The thinking is still yours — the typing just gets faster.

---

## Key Takeaways

1. **Redis sorted sets are underrated**. ZADD + ZPOPMIN gives you a concurrent-safe priority queue with zero application-level locking.

2. **Exponential backoff is simple to implement, powerful in practice**. Three lines of math (`2^retries`, cap at 60) prevent cascade failures.

3. **Dead-letter queues are non-negotiable**. Tasks will fail permanently. Having a place to inspect and replay them is essential.

4. **Building it beats reading about it**. I've used task queues for years, but building one from scratch filled gaps I didn't know I had — especially around the delayed scheduling and retry re-entry logic.

5. **Visualization makes complexity tangible**. Watching 5 workers drain a queue in real time teaches more than any architecture diagram.

---

## Try It Yourself

```bash
git clone https://github.com/kripa-sindhu-007/task-queue-educational-dashboard.git
cd task-queue-educational-dashboard
docker compose up --build
```

Open http://localhost:3000, click **5k**, and watch the system work.

The repo is MIT licensed. Issues, PRs, and feedback are all welcome.

---

*If this helped you understand task queues better, share it with someone who's learning distributed systems. And if you spot something I got wrong — please tell me. That's the whole point of open-sourcing it.*
