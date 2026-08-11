// Chapter body for "The Two Generals Problem, Atomicity & Redis Lua"
// Authored as a TS template string to avoid webpack raw-loader / file-import
// config. Key Takeaways and Revision Questions live as structured fields on the
// chapter (see lib/learn.ts) and are intentionally NOT duplicated here.
export const markdown = `
Two allied generals sit on opposite hills, an enemy city in the valley between
them. They can only win if they attack at the *same time*, and the only way to
agree on a time is to send a messenger through the valley — where the messenger
might be captured. General A sends "attack at dawn". Did it arrive? A cannot
know until B sends back "confirmed". Did *that* arrive? B cannot know until A
confirms the confirmation. The acknowledgements never end, and at no point is
either general certain the other will attack.

This is the **Two Generals Problem**, and its lesson is permanent: *you cannot
achieve guaranteed agreement between two parties over a channel that can lose
messages.* We met it already when we saw why exactly-once *delivery* is
impossible — a lost ack leaves the broker unable to tell "done" from "never
ran". This chapter follows that same lesson *inside* a single queue, where the
"lossy channel" becomes something subtler: the gap **between two operations**
that were supposed to happen together.

## The danger: multi-step operations have gaps

The Two Generals problem is about coordination across a network. But a race
window can open even on one machine, whenever a single logical action is
implemented as *several* separate steps. Between any two steps, another actor
can slip in — or the process can crash. Two patterns bite queues constantly.

### Check-then-act: two workers grab the same task

Imagine dequeue implemented naively, as a *read* followed by a *write*:

\`\`\`text
  Worker A                     Worker B
    │  peek ready → "task-42"     │
    │                             │  peek ready → "task-42"   ◀ same task!
    │  remove "task-42"           │
    │                             │  remove "task-42"         ◀ already gone
    ▼                             ▼
  runs task-42                  runs task-42                  ◀ duplicate work
\`\`\`

Both workers read the same head-of-queue before either removed it. Nothing is
"broken" in any single command — the *gap* between peek and remove is the bug.
The task runs twice.

### ZREM-then-ZADD: a task briefly in neither set

Now imagine promoting a delayed task to ready as two commands — remove it from
\`taskqueue:delayed\`, then add it to \`taskqueue:ready\`:

\`\`\`text
  ZREM delayed task-42   ✔     ← task-42 now in NO set
        · · · · · · · · · · ·  ← crash / failover HERE loses task-42 forever
  ZADD ready  task-42    ✔     ← (never reached)
\`\`\`

For the instant between the two commands, \`task-42\` exists in *neither* set.
If the process dies in that window, the task is simply gone — it was removed
from delayed but never landed in ready. This is not hypothetical: an earlier
version of this queue's promotion path (before P1.5) did exactly ZREM-then-ZADD
and had this loss window. The fix was to make the two steps **one**.

## The fix: make the multi-step operation atomic

"Atomic" means **all-or-nothing and uninterruptible**: the whole operation
either happens completely or not at all, and no other operation can observe a
half-finished intermediate state. If dequeue is atomic, there is no moment when
a second worker can peek the same task, and no moment when a task sits in no
set. The gaps disappear.

The question is *how* to get atomicity across several Redis commands. Redis
gives us a beautifully simple lever.

## Why Redis Lua scripts are atomic

Redis executes commands on a **single thread**, one at a time, start to finish.
There is no command-level interleaving to begin with. When you send a script
via \`EVAL\` / \`EVALSHA\`, Redis runs that *entire script* as if it were one
command: every \`redis.call\` inside it executes back-to-back with **no other
client's command running in between**.

\`\`\`text
  ── Redis single command loop (one thread) ─────────────────────────▶
     ... │  ██████ your Lua script (many redis.call) ██████  │ ...
         │  nothing else runs until the whole script returns │
     other clients wait here ──────────────────────────────-─┘
\`\`\`

So a Lua script is the unit of atomicity we wanted. Inside it we can ZPOPMIN,
then ZADD, then HSET, and be *guaranteed* that no worker, reaper, or promoter
saw the in-between state. Multi-step logic, executed indivisibly.

> The tradeoff: because the whole server is blocked while a script runs, scripts
> must be **short and fast** — no slow loops, no unbounded work. Coordination
> logic yes; heavy computation no.

## How the code ships scripts: //go:embed + EVALSHA

The Lua lives in real \`.lua\` files under \`backend/internal/**/scripts/\`, and
Go compiles them straight into the binary with the \`//go:embed\` directive — no
separate files to deploy, no drift between "the script on disk" and "the script
running":

\`\`\`text
  //go:embed scripts/dequeue.lua
  var dequeueScript string        ← file contents baked into the binary
\`\`\`

At startup the broker wraps each embedded string in \`redis.NewScript(...)\`.
The go-redis \`Script\` helper first tries \`EVALSHA <sha1>\` (run the script
Redis has already cached by its hash); if Redis replies "NOSCRIPT", it falls
back to \`EVAL\` (send the full body, which also caches it). After the first
call every invocation is just the 40-char SHA on the wire — cheap and atomic.

| Concern | How it's handled |
| --- | --- |
| Where the script lives | A real \`.lua\` file, so it's readable and testable. |
| Getting it into the binary | \`//go:embed\` bakes the file's bytes into a \`string\`. |
| Registering it | \`redis.NewScript(body)\` at broker construction. |
| Running it | \`EVALSHA\` by hash, auto-falling back to \`EVAL\` once. |
| Atomicity | Redis's single thread runs the whole script indivisibly. |

## Real example 1: dequeue.lua — claim a task with no race

Dequeue is the canonical check-then-act danger, and \`dequeue.lua\` collapses it
into three commands that run as one:

\`\`\`lua
local result = redis.call('ZPOPMIN', KEYS[1], 1)   -- pop head of ready
if #result == 0 then return nil end                -- ready empty
local taskID = result[1]
redis.call('ZADD', KEYS[2], ARGV[1], taskID)       -- lease into processing
redis.call('HSET', KEYS[3] .. taskID,              -- stamp canonical state
           'status', 'processing', 'owner', ARGV[2])
return taskID
\`\`\`

\`ZPOPMIN\` reads *and removes* the head in a single command, so two workers can
never pop the same ID — the peek/remove gap is gone. Then the same task ID is
added to \`taskqueue:processing\` with its lease deadline as the score, and the
canonical hash \`taskqueue:task:{id}\` is marked \`processing\`. Because all
three run indivisibly, a crash cannot strand the task "popped from ready but not
yet in processing" — the exact loss window we were afraid of.

## Real example 2: promote.lua — closing the ZREM-then-ZADD window

Promotion moves due tasks from \`taskqueue:delayed\` to \`taskqueue:ready\`. The
old racy shape was ZREM-then-ZADD with a gap in the middle. The fixed
\`promote.lua\` still does a ZREM and a ZADD — but *inside one script*, so the
"task in neither set" instant is never observable and a crash can't happen
between them:

\`\`\`lua
local removed = redis.call('ZREM', delayed_key, id)  -- claim this task
if removed == 1 then                                 -- we won the claim
    local priority = tonumber(redis.call('HGET', prefix .. id, 'priority')) or 0
    redis.call('ZADD', ready_key, -priority, id)     -- and land it in ready
end
\`\`\`

Notice a second trick: the \`ZREM\` return value is itself a **concurrency
guard**. \`ZREM\` returns \`1\` only for the caller that actually removed the
member; a racing promoter gets \`0\` and skips the ZADD. So even if two promoters
run, each due task is promoted exactly once. The reaper's \`reclaim.lua\` uses
the identical pattern — \`ZREM\` from \`taskqueue:processing\` as the claim, and
whoever gets the \`1\` owns the reclaim, cleanly settling reaper-vs-worker and
reaper-vs-reaper races.

## The pattern to remember

| Non-atomic (racy) | Atomic (Lua) |
| --- | --- |
| Peek ready, then remove → two workers grab one task. | \`ZPOPMIN\` inside a script pops-and-removes as one step. |
| ZREM delayed, then ZADD ready → crash leaves task in no set. | ZREM + ZADD in one script; no observable gap. |
| Read state, decide, then write → decision is stale by write time. | Read + decide + write in one script, on state that can't change under you. |

The through-line: whenever correctness depends on several steps happening
*together*, an unprotected gap between them is a bug waiting for the worst
possible moment. Redis's single thread plus a Lua script turns "several steps"
into "one indivisible step", which is how this queue keeps its invariants under
concurrent workers, promoters, and reapers.

## How this playground fits

Every atomic operation described here is a real, readable file you can open in
this repo under \`backend/internal/**/scripts/*.lua\`: \`dequeue.lua\`,
\`ack.lua\`, and \`nack.lua\` under the broker; \`promote.lua\` under the queue
scheduler; and \`reclaim.lua\` under the reaper. They are embedded into the Go
binary with \`//go:embed\` (see \`broker.go\`) and run via \`EVALSHA\`. The keys
they touch — \`taskqueue:ready\`, \`taskqueue:delayed\`, \`taskqueue:processing\`
(sorted sets of task IDs) and \`taskqueue:task:{id}\` (the canonical hash) — are
exactly the ones defined in \`store/redis.go\`. When you watch the **Task Flow
Pipeline** on the Playground move tasks between stages with several workers
running at once and never lose or duplicate one, that reliability is these
scripts doing their all-or-nothing work — the Two Generals lesson, defused by
atomicity.
`.trim();
