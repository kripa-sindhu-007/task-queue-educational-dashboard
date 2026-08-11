// Chapter body for "Eager Node-Death Reclaim & Worker Ownership"
// Authored as a TS template string to avoid webpack raw-loader / file-import
// config. Key Takeaways and Revision Questions live as structured fields on the
// chapter (see lib/learn.ts) and are intentionally NOT duplicated here.
export const markdown = `
A worker just leased a batch of tasks, started processing, and then its process
was killed — a crash, an OOM, a \`kill -9\`, a machine that fell off the network.
Those tasks are now sitting in the processing set, marked as owned by a node
that will never ack them. What happens to them?

The previous chapter answered this for a *single* stalled task: the lease
expires and the reaper puts it back. That is the safety net, and it always
works. But a dead worker is rarely holding just one task — it might be holding
dozens. Waiting for each lease to time out one by one is slow, and slow recovery
is exactly what you cannot afford when a node dies mid-burst. This chapter is
about the fast path: the moment a node's heartbeat stops, reclaim *everything*
it was holding, all at once, with zero task loss.

## Two recovery paths, and why you want both

There are two independent mechanisms that rescue a dead worker's in-flight work,
and they are deliberately layered.

| Path | Trigger | Granularity | Recovery time | Role |
| --- | --- | --- | --- | --- |
| **Lease expiry** | A task's per-task visibility deadline passes with no ack | One task at a time | Up to \`VisibilityTimeout\` (**~30s**) | The safety net — always correct, even if we never noticed the node died |
| **Eager node-death reclaim** | A node's heartbeat key expires | *All* of that node's in-flight tasks at once | Bounded by heartbeat TTL + one reaper tick (**~10–15s**) | The fast path — the "kill a worker, watch it recover" money demo |

The lease path is the floor: even if the cluster-membership machinery were
completely broken, a lease would still eventually expire and the task would
still come back. The node-death path is the accelerator: instead of waiting out
each 30-second lease independently, we notice the *node* is gone and reclaim its
entire in-flight set in a single sweep, bounded by how long a heartbeat is
allowed to go stale — not by the visibility timeout.

You keep both because they fail in different ways. Heartbeats can be fooled (a
node that is hung but still beating, or a network partition that hides a healthy
node); the lease is the backstop that does not depend on any of that. Belt and
suspenders.

## Worker ownership: the per-node \`:tasks\` set

The eager path only works because the system knows *exactly which tasks each
node holds*. That bookkeeping lives in three Redis keys per node:

| Key | Type | Meaning |
| --- | --- | --- |
| \`taskqueue:nodes\` | SET | The registry — every known node ID (alive or not) |
| \`taskqueue:node:{id}:hb\` | string + TTL | Heartbeat; **its existence means "alive"** |
| \`taskqueue:node:{id}:tasks\` | SET | The task IDs this node currently leases (its in-flight set) |

Ownership is maintained atomically as part of the lease lifecycle. When a worker
dequeues a task, the same Lua script that moves it into the processing set also
\`SADD\`s the task ID into that node's \`:tasks\` set and stamps \`owner\` on the task
record. When the worker acks or the reaper reclaims, the task is \`SREM\`ed from
that set. So the \`:tasks\` set is always a live picture of what a node is holding
— which means when a node dies, we do not have to scan the entire processing set
guessing which tasks belonged to it. We just read its \`:tasks\` set.

A node refreshes its heartbeat every \`HeartbeatInterval\` (**3s**), and the
heartbeat key has a TTL of \`HeartbeatTTL\` (**10s**) — long enough to tolerate a
couple of missed beats without a false alarm. Miss enough beats and the key
expires; the node stays listed in the registry (so the dashboard can gray it
out) but is now detectably dead.

## The reaper tick: dead nodes first, then leases

The reaper wakes up every \`ReaperInterval\` (**5s**) and does two things, in this
order:

\`\`\`text
  every 5s (ReaperInterval):
    ┌─────────────────────────────────────────────┐
    │ 1. reapDeadNodes()   ← eager path, runs FIRST │
    │      for each node in registry whose          │
    │      heartbeat key is gone:                   │
    │        read node:{id}:tasks  (its in-flight)  │
    │        emit node_dead                         │
    │        reclaim EACH task  → ready (Retries++) │
    │        remove node from registry              │
    ├─────────────────────────────────────────────┤
    │ 2. reap()            ← lease sweep, runs SECOND│
    │      ZRANGEBYSCORE processing  -inf .. now     │
    │      reclaim each expired lease (batched)      │
    └─────────────────────────────────────────────┘
\`\`\`

The order matters. Running \`reapDeadNodes\` **before** the lease sweep means that
when a node has died, its whole in-flight set is reclaimed proactively in this
tick — the tasks are back in \`ready\` and available for rebalancing immediately,
rather than lingering in processing until each one's individual 30-second lease
happens to expire and gets caught by the second phase. The lease sweep then only
has to mop up genuinely stalled individual tasks (a hung-but-alive worker, or
edge cases the node scan did not cover).

Detecting dead nodes is cheap: \`DeadNodeIDs\` pipelines an \`EXISTS\` on each
registered node's heartbeat key and collects the ones that return 0. For each
dead node it reads the \`:tasks\` set via \`OwnedTaskIDs\`, reclaims every member,
then \`Remove\`s the node — dropping it from the registry and deleting its
(now-drained) heartbeat and tasks keys.

## Atomic reclaim: Retries++ or dead-letter

Both recovery paths funnel every task through the *same* atomic reclaim script
(\`reclaim.lua\`), so a task is reclaimed identically whether its lease expired or
its whole node died. The script's first act is a \`ZREM\` on the processing set,
and that \`ZREM\` is the concurrency guard: whoever removes the member owns the
reclaim. That single line safely resolves every race — reaper vs. worker, and
reaper vs. a second reaper.

\`\`\`text
  reclaim.lua(taskID):
    ZREM processing taskID
      └─ removed 0? → "lost_race"   (someone already handled it — expected)
    SREM node:{owner}:tasks taskID  (task leaves the owning node's set)
    task record gone? → "orphan"    (flushed after claim; just drop it)
    read retries / max_retries / priority from the task hash
      retries >= max_retries ?
        yes → HSET status=failed, owner=""        → "dead_lettered"
        no  → HSET retries+1, status=pending, owner=""
              ZADD ready  (score = -priority)      → "reclaimed"
\`\`\`

Two important details. First, the script reads \`retries\`, \`max_retries\`, and
\`priority\` **from the task hash itself**, not from anything the caller passes —
so the retry-vs-dead-letter decision is always self-consistent with the
authoritative record. Second, a reclaimed task goes back to \`ready\` at its
original priority (\`ZADD\` with score \`-priority\`), so a high-priority task that
got reclaimed does not lose its place — it re-enters ahead of low-priority work.

When a reclaim increments \`retries\` it emits a \`reclaimed\` event and bumps the
retries metric. When \`retries\` has already hit \`max_retries\`, the script marks
the record \`failed\` and the reaper pushes it to the **dead-letter queue** with a
reason like \`"lease expired, retries exhausted (reclaimed by reaper)"\` — so a
task that keeps dying with its worker eventually stops being redelivered and is
set aside for a human, exactly as in the reliability chapter.

## Owner fencing: stopping the zombie worker

Here is the subtle failure the whole design has to defend against. A node's
heartbeat lapses, the reaper reclaims its tasks, and one of those tasks gets
re-leased to a *healthy* node — which starts processing it. Then the original
"dead" worker comes back to life (it was only GC-paused, or briefly partitioned)
and tries to ack the task it thinks it still owns. If that ack succeeded, the
task would be marked completed even though the new owner is *also* running it:
a double-completion, and the new owner's work is silently discarded.

The ack script (\`ack.lua\`) prevents this with lightweight **owner fencing**.
Before completing anything, it checks two things:

\`\`\`text
  ack.lua(taskID, expectedOwner = this node):
    ZSCORE processing taskID == false ?   → return 0   (not leased anymore)
    HGET task owner        != expectedOwner ? → return 0 (reclaimed & re-leased)
    ── otherwise ──
    ZREM processing / SREM node:tasks / HSET status=completed → return 1
\`\`\`

If the task is no longer in processing, or its \`owner\` field no longer matches
the acking node, the script returns \`0\` and the broker surfaces
\`ErrLeaseNotHeld\`. The executor treats that as a harmless no-op: the reaper
already handled redelivery, so this worker simply drops the result. The same
guard protects \`Nack\` and \`ExtendLease\`. Because \`reclaim.lua\` clears the
\`owner\` field (\`owner=""\`) and the re-lease stamps a *new* owner, a resurrected
worker can never win this check for a task that has moved on. That is what turns
"at-least-once with reclaim" into something safe rather than chaotic.

## The events you will see

Every step narrates itself onto the event stream, which is what the dashboard
renders:

| Event | Emitted when | Detail it carries |
| --- | --- | --- |
| \`node_dead\` | The reaper detects an expired heartbeat | Which node, and how many in-flight tasks it is about to reclaim |
| \`reclaimed\` | A task is moved processing → ready with \`Retries++\` | The attempt count, e.g. \`(attempt 2/4)\` |
| \`dead_lettered\` | A reclaimed task had already exhausted its retries | Retry budget, e.g. \`(3/3)\` |

Reaper-emitted events carry \`WorkerID: -1\` to mark them as coming from the reaper
rather than any worker — a small tell that distinguishes system recovery from
normal task flow in the timeline.

## How this playground fits

This is the payoff — the landing page's **Recover** beat, live. Open the
Playground, submit a burst of tasks, and watch the **Cluster Nodes & Workers**
panel: several nodes light up, each holding a slice of the in-flight work in its
\`:tasks\` set. Now **kill a worker mid-burst.** Its heartbeat stops; within a few
seconds its heartbeat key expires and the node grays out. On the next reaper tick
(≤5s) you see \`node_dead\` fire, followed immediately by a fan of \`reclaimed\`
events as its entire in-flight set is atomically pushed back to \`ready\` — and
then the surviving workers pick that work straight back up. Detection → reclaim →
rebalance, bounded by the heartbeat TTL rather than the 30-second lease, with a
task counter that never drops a single task. If you bring the "dead" worker back
and let it try to finish an old task, you will see it hit \`ErrLeaseNotHeld\` and
quietly stand down. That whole loop — the money demo — is this chapter running
in front of you.
`.trim();
