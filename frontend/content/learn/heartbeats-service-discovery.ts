// Chapter body for "Heartbeats & Service Discovery"
// Authored as a TS template string to avoid webpack raw-loader / file-import
// config. Key Takeaways and Revision Questions live as structured fields on the
// chapter (see lib/learn.ts) and are intentionally NOT duplicated here.
export const markdown = `
A single worker is easy to reason about: it is either running or it isn't. But
the moment you run a *cluster* — several worker processes on different machines,
scaling up under load and down when it's quiet — you have a new, surprisingly
hard question to answer: **which workers are alive right now?**

Not "which workers *did we start*" — a list you wrote down once is stale the
instant a process crashes, a container is killed, or a network cable is pulled.
You need a live, self-correcting answer that a dashboard can poll and a reaper
can trust. This chapter is about how a distributed system keeps that answer
honest, using two small ideas: a **heartbeat** and a **TTL key**.

## The problem: membership is not a static list

In a cluster, workers come and go for reasons nobody announces:

- A container is scaled down, or the orchestrator reschedules it.
- A process crashes, OOMs, or gets \`SIGKILL\`ed — no chance to say goodbye.
- A host loses network, or a long GC pause freezes it mid-task.

A crashed worker cannot tell you it died — that is the whole difficulty. So you
cannot rely on workers *reporting* their own death. Instead you flip the
question around: require every worker to keep *proving it is still alive*, and
treat silence as death. That is a **dead man's switch** — the moment a worker
stops actively holding the lever, the system assumes the worst.

## The heartbeat + TTL-key pattern

A worker in this playground keeps one small key in Redis that says "I exist,"
and that key is set to **expire on its own** after a short time-to-live (TTL).
The worker then re-writes the key on a timer, resetting the TTL each time,
faster than it can expire. As long as the worker is healthy it keeps the key
alive; if it dies, nobody refreshes the key, Redis expires it, and the worker
silently vanishes from the cluster.

\`\`\`text
  worker alive: refresh beats out-run the TTL
   beat   beat   beat   beat        (worker dies here)
    │      │      │      │
    ▼      ▼      ▼      ▼           TTL counting down, no refresh...
  [hb TTL=10s][reset][reset][reset]  ────────► key EXPIRES ► node gone
    0s     3s     6s     9s              (~10s after last beat)
\`\`\`

Concretely, when a worker starts it calls \`Register\`, and while it runs a
background loop calls \`Heartbeat\` on a ticker. Both do the same two writes in a
Redis pipeline:

- \`SADD taskqueue:nodes {id}\` — add itself to the registry set.
- \`SET taskqueue:node:{id}:hb <record> EX <ttl>\` — (re)write its heartbeat key
  with a fresh TTL.

The heartbeat loop is deliberately independent of task execution, so a node
that is busy running tasks still refreshes its presence on time.

## What each key holds

Every node is tracked by exactly three Redis keys. Knowing what each one is for
makes the whole system legible:

| Key | Type | Holds | Meaning |
| --- | --- | --- | --- |
| \`taskqueue:nodes\` | SET | every known node ID | the **registry** — who has ever joined and not yet been cleaned up |
| \`taskqueue:node:{id}:hb\` | string + **TTL** | the node's JSON record (id, hostname, capacity, started-at) | **liveness** — its existence *is* the proof the node is alive |
| \`taskqueue:node:{id}:tasks\` | SET | task IDs the node currently leases | its **in-flight work**, used to reclaim tasks if it dies |

The clever part is that the \`:hb\` key doubles as both the liveness signal *and*
the node's record. Because the full record is stored as the key's *value*, one
\`GET\` tells you both "is this node alive?" (did the key exist?) and "what is it?"
(the hydrated record) in a single round-trip — no second lookup.

Note the split between the registry set and the heartbeat key. The set is the
durable membership list; the \`:hb\` key is the *perishable* liveness proof. When
a node dies its \`:hb\` key expires immediately, but its ID lingers in the
registry set until a reaper notices the missing heartbeat, reclaims its
in-flight tasks, and removes it. That gap is intentional — it is exactly the
window in which the dashboard shows a node "graying out."

## Node identity

Each worker computes an ID once at startup, of the form
\`{hostname}-{shorthex}\` — for example \`worker-7b3f-a1c2e4d6f890\`. The hostname
makes IDs human-readable in logs and on the dashboard; the random hex suffix
guarantees uniqueness when several replicas share a hostname (multiple
containers, or a restart that reuses a name). An ID is stable for the life of a
process and is what stamps a task's \`owner\` field when that node leases it — so
you can always trace an in-flight task back to the node responsible for it.

## Service discovery: reading the live set

**Service discovery** is just the read side of this pattern: instead of a static
config file listing your workers, you *ask the shared store* who is alive right
now. The API exposes this at \`GET /api/nodes\`, which calls \`ListNodes\`:

\`\`\`text
  GET /api/nodes
        │
        ▼
  SMEMBERS taskqueue:nodes            → [id1, id2, id3, ...]   (the registry)
        │
        ▼   for each id, pipelined:
  GET  taskqueue:node:{id}:hb         → record  (or miss ⇒ dead)
  SCARD taskqueue:node:{id}:tasks     → in-flight task count
        │
        ▼
  [ { id, hostname, capacity, alive, in_flight_tasks }, ... ]  → JSON
\`\`\`

For each registered ID it does a pipelined \`GET\` of the heartbeat key and an
\`SCARD\` of the tasks set. A hit means the node is **alive** and hands back its
hydrated record; a miss (the TTL expired) means the node is **dead** — it is
still returned, but with \`alive: false\`, so the UI can render it graying out
rather than making it disappear the instant it misses a beat. The dashboard
simply polls this endpoint and diffs the result: new IDs → cards appear, missing
heartbeats → cards gray out.

## The beat-interval vs timeout tradeoff

Two numbers govern the whole scheme, and their *ratio* is the real design
decision:

| Setting | Default | Env var | Role |
| --- | --- | --- | --- |
| Heartbeat interval | **3s** | \`HEARTBEAT_INTERVAL_MS\` | how often a worker refreshes its \`:hb\` key |
| Heartbeat TTL | **10s** | \`HEARTBEAT_TTL_MS\` | how long the key lives before expiring |

The TTL **must** exceed the interval — the config refuses to start otherwise —
because a node has to be able to miss a beat or two (a slow write, a brief
network blip) without being declared dead. A 3s beat under a 10s TTL tolerates
roughly two consecutive missed beats before the key lapses.

Choosing the TTL is a genuine tension:

- **Shorter TTL** → faster death detection (you notice a dead node in seconds),
  but more **false positives**: a healthy worker stalled by a stop-the-world GC
  pause, a momentary network hiccup, or CPU starvation can miss its window and
  be wrongly declared dead — triggering needless task reclaims and churn.
- **Longer TTL** → far fewer false positives, but **slower detection**: a truly
  dead node's tasks sit stranded, unclaimed, for longer before anyone recovers
  them.

The 3s / 10s default is a deliberate middle ground: frequent enough to detect a
real death within about ten seconds, forgiving enough that an ordinary GC pause
or packet loss doesn't evict a perfectly healthy worker.

## Push (heartbeat) vs pull (health-check)

There are two ways to learn that a node is alive, and it's worth knowing why
this system chose one:

| Model | How it works | Trade-offs |
| --- | --- | --- |
| **Push** (heartbeat) | each worker actively announces "I'm alive" into a shared store on a timer | scales cleanly — the coordinator does no work per node; the worker itself is the source of truth; survives a coordinator restart |
| **Pull** (health-check) | a central monitor probes each worker (e.g. hits a \`/healthz\`) on a schedule | the monitor must *know every address* up front and do O(nodes) work each round; awkward for workers behind NAT or without an inbound port |

A heartbeat is **push-based**: the worker reports into Redis, and anyone
interested just reads the shared set. That is why it discovers new workers for
free — a worker joining is nothing more than it starting to write its key. There
is no registry to update by hand and no list of addresses to probe.

## How this playground fits

The **Cluster Nodes** panel on the Playground is this chapter, live. Start a new
worker (the single binary runs an in-process node; \`cmd/worker\` runs a
standalone one that joins the same cluster) and within a beat a card appears,
showing its ID, hostname, capacity, and current in-flight task count — all read
straight from \`GET /api/nodes\`. Kill that worker ungracefully and watch the
card **gray out about ten seconds later**, as its \`:hb\` key expires and the
endpoint starts reporting \`alive: false\`. Shut one down gracefully instead and
it deregisters cleanly, so its card disappears right away rather than lingering.
Every appearance and every fade on that panel is a heartbeat being written — or
no longer being written — in Redis.
`.trim();
