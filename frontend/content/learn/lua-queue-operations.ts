// Chapter body for "Lua Scripts & Queue Operations"
// Authored as a TS template string to avoid webpack raw-loader / file-import
// config. Key Takeaways and Revision Questions live as structured fields on the
// chapter (see lib/learn.ts) and are intentionally NOT duplicated here.
export const markdown = `
A queue looks simple from the outside — you push work in, a worker pulls work
out — but every one of those moves is really *several* Redis commands that must
happen together, or not at all. "Pop the next task **and** lease it to me" is
two operations. If another worker sneaks in between them, two workers run the
same task, or a crash strands it in limbo.

This playground solves that the way every serious Redis-backed queue does: each
core operation is a small **Lua script** that Redis runs **atomically, in one
round-trip**. No other command can interleave while the script runs, and the
whole thing counts as a single network call. This chapter walks through the six
scripts that drive the queue and why each one *has* to be a single script.

## Why atomic scripts beat multiple client commands

Imagine dequeue written as plain client calls:

\`\`\`text
  worker: ZPOPMIN ready        ← got task 42
  ── another worker interleaves here ──
  worker: ZADD    processing   ← lease task 42
\`\`\`

Between those two lines, task 42 exists **nowhere the system can see it**. If
the worker crashes there, the task is silently lost — it left \`ready\` but never
reached \`processing\`, so no lease will ever expire to rescue it. And even
without a crash, a second worker running \`ZPOPMIN\` at the wrong moment could
observe an inconsistent view of the queue.

Redis is single-threaded and runs a Lua script **to completion before serving
any other command**. So bundling "pop, lease, stamp status" into one script
gives you two guarantees for free:

- **No interleaving.** The pop and the lease are indivisible; there is no window
  for another worker or the reaper to see a half-finished state.
- **One round-trip.** Three or four Redis operations cost a single network
  hop instead of three or four, which matters when workers are dequeuing
  thousands of times a second.

This is the classic Redis pattern: don't pull data to the client, mutate it,
and write it back (a TOCTOU race waiting to happen) — send the *logic* to the
data and let Redis serialize it.

## The KEYS / ARGV convention and EVALSHA caching

Every script follows Redis's calling convention. Keys the script touches are
passed as \`KEYS[1..n]\`; everything else — deadlines, IDs, batch sizes — comes in
as \`ARGV[1..n]\`. Keeping keys in \`KEYS\` (not hard-coded inside the script) is
what lets Redis Cluster route the call correctly, and it keeps the scripts pure
functions of their inputs.

In this codebase the scripts live as \`.lua\` files next to the Go code and are
compiled into the binary with \`//go:embed\`, so there is no runtime file I/O and
no drift between "the script on disk" and "the script we run":

\`\`\`text
  //go:embed scripts/dequeue.lua
  var dequeueScript string
  ...
  dequeueCmd: redis.NewScript(dequeueScript)
\`\`\`

\`redis.NewScript\` wraps the source and uses **EVALSHA** under the hood: the
first call ships the whole script and Redis caches it by its SHA1 hash;
every call after that sends only the 40-character hash plus the arguments. The
client transparently falls back to a full \`EVAL\` if Redis reports \`NOSCRIPT\`
(e.g. after a restart flushed the script cache). So you get the safety of
server-side atomicity without re-uploading the script body on every call.

## The operations, one script at a time

### Dequeue — pop, lease, and stamp in one move

\`dequeue.lua\` is the heart of at-least-once delivery. In a single script it:

\`\`\`lua
-- Pop the highest-priority task (score = -priority, so ZPOPMIN wins)
local result = redis.call('ZPOPMIN', KEYS[1], 1)   -- KEYS[1] = ready
if #result == 0 then return nil end                 -- queue empty
local taskID = result[1]

redis.call('ZADD', KEYS[2], ARGV[1], taskID)        -- KEYS[2] = processing, ARGV[1] = lease deadline (ms)
local taskKey = KEYS[3] .. taskID                   -- KEYS[3] = taskqueue:task:
redis.call('HSET', taskKey, 'status', 'processing', 'owner', ARGV[2])
redis.call('SADD', KEYS[4], taskID)                 -- KEYS[4] = this node's in-flight SET
return taskID
\`\`\`

Popping from \`ready\` and adding to \`processing\` with a lease deadline as the
score, plus recording which node now owns the task, is one indivisible step. The
task is *never* absent from both sets, so a crash mid-dequeue can't lose it.
Ownership is tracked twice on purpose — an \`owner\` field on the task hash
(authoritative) and membership in the node's task SET (fast per-node reclaim by
the reaper).

### Ack — complete only if you still hold the lease

\`ack.lua\` removes the task from \`processing\` and marks the record \`completed\` —
but only after it checks that this node still owns the lease:

\`\`\`lua
if redis.call('ZSCORE', KEYS[1], ARGV[1]) == false then return 0 end  -- not leased at all
if redis.call('HGET', KEYS[2], 'owner') ~= ARGV[2] then return 0 end  -- someone else owns it now

redis.call('ZREM', KEYS[1], ARGV[1])
redis.call('SREM', KEYS[3], ARGV[1])
redis.call('HSET', KEYS[2], 'status', 'completed', 'error', '', 'owner', '')
return 1
\`\`\`

The check-then-act must be atomic: if the "is it still mine?" test and the
\`ZREM\` were separate client calls, the reaper could reclaim the task in the gap
and this worker would clobber the new owner's lease.

### Nack — release the lease so the task can be retried

\`nack.lua\` is ack's sibling: same ownership fence, but it only removes the task
from \`processing\` (releasing the lease) and leaves routing to the caller. The
executor then sends the task to **retry** (back onto the delayed set with a
backoff delay) or, once retries are exhausted, to the **dead-letter queue**.
Verifying ownership before releasing means a worker whose lease already expired
can't yank a task that now belongs to someone else.

### Extend — push back the deadline without a TOCTOU gap

Long-running tasks would otherwise have their lease expire while they're still
healthy. \`extend.lua\` bumps the visibility deadline in one script:

\`\`\`lua
if redis.call('ZSCORE', KEYS[1], ARGV[1]) == false then return 0 end  -- not in processing
redis.call('ZADD', KEYS[1], ARGV[2], ARGV[1])                          -- new deadline as score
return 1
\`\`\`

Because the existence check and the score update ride in the same script, there
is no window between "is it still leased?" and "push back its deadline."

### Promote — move due delayed tasks into ready, in batches

Scheduled and retried tasks wait in the \`delayed\` ZSET keyed by their
due-time. \`promote.lua\` fetches up to a batch limit of tasks whose time has come
(\`score <= now\`), and for each one that it successfully claims via \`ZREM\`, reads
the task's **priority from its hash** and inserts it into \`ready\` with
\`score = -priority\`:

\`\`\`lua
local ids = redis.call('ZRANGEBYSCORE', KEYS[1], '-inf', ARGV[1], 'LIMIT', 0, limit)
for _, id in ipairs(ids) do
    if redis.call('ZREM', KEYS[1], id) == 1 then          -- ZREM = concurrency guard
        local priority = tonumber(redis.call('HGET', KEYS[3] .. id, 'priority')) or 0
        redis.call('ZADD', KEYS[2], -priority, id)         -- into ready, highest priority first
        promoted = promoted + 1
    end
end
\`\`\`

The \`ZREM\` return value is the guard: only the caller that actually removes a
task promotes it, so two promotion ticks can run without double-promoting.
Batching (a bounded \`LIMIT\`) keeps each tick short so it never monopolizes the
single-threaded server.

### Reclaim — rescue expired leases, one at a time, batched per tick

\`reclaim.lua\` is what the **reaper** runs against tasks whose lease deadline has
passed. It claims a task by removing it from \`processing\` (again, \`ZREM\` as the
race guard), then reads \`retries\` / \`max_retries\` / \`priority\` from the
*authoritative* task hash and decides its fate:

\`\`\`text
  ZREM processing → lost the race?  → "lost_race" (another reaper/worker got it)
  no task record?                   → "orphan"    (flushed; just clean up)
  retries >= max_retries?           → "dead_lettered" (mark failed; caller pushes to DLQ)
  otherwise                         → "reclaimed" (retries++, status=pending, back to ready)
\`\`\`

The reaper reclaims up to ~100 tasks per tick so recovery is steady rather than a
stampede. Reading the counters from the hash instead of trusting caller-passed
values keeps the decision self-consistent with the record of truth.

## The ErrLeaseNotHeld race guard

Ack, nack, and extend all return \`1\` on success and \`0\` when the lease is no
longer held by this node — because it expired, the reaper reclaimed it, or it
was re-leased elsewhere. The Go broker turns that \`0\` into **\`ErrLeaseNotHeld\`**.
This is the fence that resolves the reaper-vs-worker race:

- On **ack**, \`ErrLeaseNotHeld\` means "the reaper already redelivered this; your
  completion is a no-op" — safe under at-least-once, the task simply runs again.
- On **nack**, it means "don't route to retry yourself — the reaper already
  handled redelivery," preventing a double requeue.

Without the ownership check baked *inside* the script, a slow worker could ack a
task the reaper had already handed to someone else, silently corrupting the new
owner's lease. Atomicity is what makes the fence trustworthy.

## Summary: what each script fuses and why

| Operation | Script | Redis commands it fuses | Why atomic matters |
| --- | --- | --- | --- |
| **Dequeue** | \`dequeue.lua\` | \`ZPOPMIN\` ready → \`ZADD\` processing → \`HSET\` status/owner → \`SADD\` node set | Task is never in *neither* set; no lost or double-leased work. |
| **Ack** | \`ack.lua\` | \`ZSCORE\` + \`HGET\` owner check → \`ZREM\` → \`SREM\` → \`HSET\` completed | Check-then-remove can't be interrupted by a reclaim. |
| **Nack** | \`nack.lua\` | \`ZSCORE\` + \`HGET\` owner check → \`ZREM\` → \`SREM\` | Only the true owner releases the lease before retry/DLQ routing. |
| **Extend** | \`extend.lua\` | \`ZSCORE\` existence check → \`ZADD\` new deadline | No TOCTOU gap between "still leased?" and "extend it." |
| **Promote** | \`promote.lua\` | \`ZRANGEBYSCORE\` due → \`ZREM\` → \`HGET\` priority → \`ZADD\` ready (batched) | Each due task is promoted exactly once, priority-correct. |
| **Reclaim** | \`reclaim.lua\` | \`ZREM\` processing → \`HGET\` retries → \`HSET\` + \`ZADD\` ready *or* mark failed (batched) | One reaper wins each task; retry/DLQ decision stays consistent. |

## How this playground fits

Every task you submit on the **Task Flow Pipeline** flows through these exact
scripts. Submitting a task calls \`Enqueue\`; a scheduled or retried task waits in
\`delayed\` until \`promote.lua\` moves it to \`ready\`; a worker claims it with
\`dequeue.lua\`; it finishes with \`ack.lua\`, fails back with \`nack.lua\`, or is
rescued by the reaper's \`reclaim.lua\` when its lease lapses. The stage
transitions you watch light up on screen are not a simulation of these scripts —
they *are* these scripts, running atomically inside Redis, one round-trip at a
time.
`.trim();
