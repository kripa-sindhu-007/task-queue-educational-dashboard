// Chapter body for "Sorted Sets & Queue Design"
// Authored as a TS template string to avoid webpack raw-loader / file-import
// config. Key Takeaways and Revision Questions live as structured fields on the
// chapter (see lib/learn.ts) and are intentionally NOT duplicated here.
export const markdown = `
Most people picture a queue as a straight line: things join at the back, leave
from the front, strictly first-in-first-out. That mental model is fine for the
simplest case — but real queues need more. Some tasks are more urgent than
others. Some should not run until a future moment. And some are "in flight" on a
worker and must be reclaimed if that worker dies. A plain line cannot express
any of that.

This playground does not use a plain line. It builds every queue out of a Redis
**sorted set** (a **ZSET**), and lets a single number — the **score** — decide
the order. Once you see that one idea, the whole design collapses into something
almost obvious: *a queue is just a sorted set where the score is whatever you
want to sort by.*

## What a sorted set actually is

A Redis sorted set is a collection of unique **members**, where every member
carries a floating-point **score**. Redis keeps the members permanently ordered
by that score. You never sort it yourself; you add and remove, and the structure
stays sorted for free.

| Property | Plain queue (list) | Sorted set (ZSET) |
| --- | --- | --- |
| Order determined by | insertion order only | a numeric **score** you choose |
| Members | can repeat | **unique** (a set) |
| "Take the front" | pop head | pop lowest (or highest) score |
| Re-prioritise later | hard | just re-\`ZADD\` a new score |
| Range by value | no | \`ZRANGEBYSCORE\` — "all scores ≤ X" |

That fourth and fifth row are the superpowers. Because the score is *yours to
define*, you can make it mean "priority", or "the time this becomes due", or
"the deadline by which a worker must finish" — and the same handful of commands
work for all three.

## The core commands (and why they are cheap)

You only need a few operations, and each is O(log n) — fast even with millions
of members, because a ZSET is backed by a skip list plus a hash.

| Command | What it does | Cost |
| --- | --- | --- |
| \`ZADD key score member\` | Insert a member, or move it to a new score. | O(log n) |
| \`ZPOPMIN key\` | Remove and return the **lowest**-scored member. | O(log n) |
| \`ZPOPMAX key\` | Remove and return the **highest**-scored member. | O(log n) |
| \`ZRANGEBYSCORE key min max\` | Return every member whose score is in [min, max]. | O(log n + m) |
| \`ZREM key member\` | Remove a specific member. | O(log n) |

"Pop the most important task" and "find everything that is now due" both become
single, cheap, atomic Redis calls. No scanning, no client-side sorting.

## Three queues, one data structure

This backend runs **three** sorted sets. They hold nothing but task IDs; the
canonical record for each task lives in a Redis hash at \`taskqueue:task:{id}\`.
What makes each queue behave differently is purely *what the score means.*

\`\`\`text
   producer
      │  ZADD (score = -priority)         ZADD (score = due-time)
      ▼                                        │
  taskqueue:ready ◀──── promote when due ──── taskqueue:delayed
      │  ZPOPMIN (lease it)               (score = unix seconds until due)
      ▼
  taskqueue:processing   (score = lease deadline, ms)
      │        ▲
   ACK│        │ reaper: ZRANGEBYSCORE 0..now → requeue
   ZREM│       └──────────── lease expired ───────────┐
      ▼                                               │
    done ✔                                     back to ready
\`\`\`

### 1. Priority queue — the "ready" set

\`taskqueue:ready\` holds tasks that are ready to run *right now*. Its score
encodes priority. The subtle-but-important detail: the backend stores
**score = -priority**. A task with priority 9 gets score -9; priority 1 gets
score -1. Since -9 < -1, the higher-priority task sorts *earlier*.

Workers then claim the next task with \`ZPOPMIN\` — pop the **lowest** score —
which, thanks to the negation, is always the **highest** priority. Enqueue is a
single \`ZADD\`; dequeue is a single \`ZPOPMIN\`. That is the entire priority
queue. Ties (equal score) fall back to lexicographic order of the member, giving
a stable, predictable result.

> Why negate instead of using \`ZPOPMAX\`? Either works; this codebase picked
> "smaller score = more urgent" and pops the minimum everywhere, so the delayed
> and processing sets (which also sort ascending) stay consistent.

### 2. Delayed / scheduled queue — the "delayed" set

\`taskqueue:delayed\` holds tasks that should not run yet. Here the score is a
**timestamp**: the Unix time (in seconds) at which the task becomes due, computed
as \`now + delay\`. Members sit sorted by *when* they should fire.

A background scheduler ticks once a second and asks a single question:
*which tasks are due?* That is exactly what \`ZRANGEBYSCORE delayed -inf now\`
answers — every member whose due-time has passed. Those IDs are then moved into
\`ready\`. The move is done in one atomic Lua script: for each due ID it
\`ZREM\`s from delayed and \`ZADD\`s into ready with \`score = -priority\` (read
from the task hash), so there is no window where a task exists in neither set or
both.

This is how "run this in 24 hours" or "retry with backoff" is implemented — not
with a sleeping timer per task, but with one number per task and one range query.

### 3. Lease-expiry index — the "processing" set

When a worker claims a task, the backend does not delete it. It \`ZADD\`s the ID
into \`taskqueue:processing\` with **score = the lease deadline** — the moment by
which the worker must finish, computed as \`now + VisibilityTimeout\` (30s by
default), expressed in **milliseconds**. So the processing set is a live index of
in-flight work, sorted by *when each lease runs out.*

A **reaper** wakes every \`ReaperInterval\` (5s) and runs the same trick as the
scheduler, just with a different meaning for the score:
\`ZRANGEBYSCORE processing -inf now\` returns every task whose deadline has
already passed — i.e. every worker that was supposed to be done and is not
(crashed, hung, or lost the network). Those tasks are reclaimed back to \`ready\`
(or dead-lettered if out of retries). A healthy worker instead calls \`ZREM\` to
remove its task on a successful ACK before the deadline arrives.

Notice the pattern: the *scheduler* and the *reaper* are the same algorithm.
Both are "range-scan a ZSET for scores ≤ now, then move what you find." The only
difference is whether the score means *become ready* or *lease expired*.

## Why store only IDs?

Each ZSET member is just a task ID string, never the task's full JSON. The
full record — payload, status, retry count, owner, priority — lives once in the
hash at \`taskqueue:task:{id}\`. This matters for three reasons:

- **Single source of truth.** A task can appear in different sets over its life
  (delayed → ready → processing), but there is exactly one place its data lives,
  so it can never drift out of sync between copies.
- **Cheap moves.** Promoting or reclaiming a task shuffles a short ID between
  sets; the heavy payload never moves.
- **Small, fast indexes.** Scores + short IDs keep each ZSET compact, so
  \`ZRANGEBYSCORE\` and \`ZPOPMIN\` stay quick even under load.

The ZSETs are *indexes over* the tasks, ordered by three different axes; the hash
is the tasks themselves.

## How this playground fits

Everything above is visible on the Playground. When you submit a task with a
future delay, it lands in the **delayed** stage — that is \`taskqueue:delayed\`,
sorted by due-time, waiting for the once-a-second promoter. When it becomes due
(or if you submit with no delay), it moves to **queued/ready** — that is
\`taskqueue:ready\`, sorted by priority, and a worker grabs the top of it with
\`ZPOPMIN\`. The moment a worker claims it, it appears as **leased/processing** —
that is \`taskqueue:processing\`, sorted by lease deadline, ticking down its 30s
visibility timeout. Kill a worker and watch the reaper's 5-second scan find the
expired lease and push the task back to queued. Every stage you see on screen is
one sorted set, and every transition is a \`ZADD\`, \`ZPOPMIN\`, \`ZRANGEBYSCORE\`,
or \`ZREM\`.
`.trim();
