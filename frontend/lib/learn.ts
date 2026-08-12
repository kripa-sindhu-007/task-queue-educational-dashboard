import { markdown as whatIsATaskQueue } from "@/content/learn/what-is-a-task-queue";
import { markdown as deliveryGuarantees } from "@/content/learn/delivery-guarantees";
import { markdown as twoGeneralsAtomicityLua } from "@/content/learn/two-generals-atomicity-lua";
import { markdown as sortedSetsQueueDesign } from "@/content/learn/sorted-sets-queue-design";
import { markdown as luaQueueOperations } from "@/content/learn/lua-queue-operations";
import { markdown as heartbeatsServiceDiscovery } from "@/content/learn/heartbeats-service-discovery";
import { markdown as nodeDeathReclaimOwnership } from "@/content/learn/node-death-reclaim-ownership";

export type PhaseGroup =
  | "Foundations"
  | "Phase 1 · Reliability"
  | "Phase 2 · Distribution";

export type ChapterStatus = "available" | "coming-soon";

/** A self-check question paired with an ideal, candidate-quality answer. */
export interface RevisionQuestion {
  q: string;
  /** The model answer revealed by the "Show answer" chip. */
  a: string;
}

export interface Chapter {
  slug: string;
  title: string;
  subtitle: string;
  phaseGroup: PhaseGroup;
  order: number;
  readingMinutes: number;
  status: ChapterStatus;
  relatedPanel?: { label: string; href: string };
  /** Rendered markdown body (available chapters only). */
  body?: string;
  keyTakeaways?: string[];
  revisionQuestions?: RevisionQuestion[];
  interviewSummary?: string;
}

/**
 * The full intended curriculum. The index renders all of these so learners can
 * see the whole roadmap, but only chapters marked `available` are readable.
 */
export const chapters: Chapter[] = [
  {
    slug: "what-is-a-task-queue",
    title: "What Is a Task Queue?",
    subtitle: "Async work, decoupling, and the tools that do it",
    phaseGroup: "Foundations",
    order: 1,
    readingMinutes: 6,
    status: "available",
    relatedPanel: { label: "Task Flow Pipeline", href: "/playground" },
    body: whatIsATaskQueue,
    keyTakeaways: [
      "A task queue lets a producer hand off work to be run later by a separate worker, instead of doing it inline while the user waits.",
      "The five core roles are producer, task/job, queue, broker, and worker — producers and workers only ever talk through the queue.",
      "It solves the problem of slow, fragile synchronous work: enqueue the job, return immediately, and let a worker do the heavy lifting.",
      "Key benefits are decoupling, async processing, load leveling, retries/reliability, horizontal scalability, scheduling, rate limiting, and fan-out.",
      "Redis-backed tools like Sidekiq, Celery, RQ, and BullMQ share the design this playground imitates: Redis as the shared coordination bus.",
      "Kafka is an event-streaming log with different semantics — related to, but not the same as, a classic one-task-per-worker queue.",
    ],
    revisionQuestions: [
      {
        q: "In your own words, what is a task queue and what problem does it solve?",
        a: "A task queue is a system where a producer hands off a unit of work (a task) to a broker, and separate worker processes pick it up and run it later, asynchronously. It solves the problem of slow or unreliable work blocking the main request path: instead of making a user wait while you send an email or transcode a video — and risking the whole request failing if that side effect fails — you enqueue the work, return immediately, and let a worker handle it in the background.",
      },
      {
        q: "Name the five core roles in a task queue and describe what each one does.",
        a: "Producer — creates work and enqueues it (often a web handler). Task/Job — the unit of work, just data describing what to do. Queue — the ordered holding area where tasks wait. Broker — the storage/coordination system that actually holds the queue (e.g. Redis, RabbitMQ). Worker/Consumer — a separate process that dequeues and executes tasks. The producer and worker never call each other directly; they communicate only through the queue.",
      },
      {
        q: "Why does doing a welcome email inline in an HTTP request hurt both speed and reliability?",
        a: "Speed: the user waits for the slowest step, so a slow email provider makes signup feel slow, and the web thread stays tied up instead of serving other requests. Reliability: the request's success becomes coupled to a side effect you don't control — if the email provider times out, the whole signup appears to fail even though the account was created fine. Enqueuing the email decouples them: signup succeeds instantly and the email is retried independently.",
      },
      {
        q: "What is load leveling, and how does a queue absorb a sudden traffic spike?",
        a: "Load leveling means smoothing bursty demand into a steady processing rate. When a spike of tasks arrives (say a flash sale enqueuing 10k emails), they pile up in the queue instead of overwhelming the system, and the workers drain the backlog at their own sustainable pace. The queue acts as a buffer between unpredictable producers and finite worker capacity, so a spike causes delay rather than failure.",
      },
      {
        q: "How does adding more workers increase throughput without changing producer code?",
        a: "Because producers and workers are decoupled through the queue, the producer just keeps enqueuing tasks regardless of who consumes them. Throughput is bounded by how fast workers pull and process tasks, so starting more worker processes lets more tasks run in parallel. You scale horizontally by adding workers — no producer changes — until you hit a downstream bottleneck like the database or the broker.",
      },
      {
        q: "Which popular tools are Redis-backed, and why does that matter for this playground?",
        a: "Sidekiq (Ruby), RQ and Celery in its Redis mode (Python), and BullMQ (Node) are all Redis-backed. It matters because this playground uses the same design: Redis is the shared coordination bus that producers write to and workers read from. Understanding the Redis-backed model here maps directly onto how those production tools actually work.",
      },
      {
        q: "How does Apache Kafka differ from a classic task queue?",
        a: "Kafka is an append-only, ordered event log built for very high-throughput streaming: consumers read (and can replay) the stream by offset, and messages aren't removed when read. A classic task queue instead hands each task to exactly one worker, which removes it once done and acks it. Kafka is optimized for event streaming and fan-out to consumer groups; a task queue is optimized for one-task-one-worker job execution with acking and retries. They overlap but solve different problems.",
      },
    ],
    interviewSummary:
      "A task queue decouples work creation from work execution: a producer enqueues a task into a broker (often Redis), and separate workers dequeue and run it asynchronously. This keeps request latency low, isolates failures in slow side effects, and enables load leveling, retries, horizontal scaling, scheduling, and fan-out. Redis-backed frameworks (Sidekiq, Celery, RQ, BullMQ) use Redis as a shared coordination bus — the same model this playground demonstrates — whereas systems like Kafka are append-only event logs with replay semantics rather than one-task-per-worker delivery.",
  },
  {
    slug: "delivery-guarantees",
    title: "Delivery Guarantees, Idempotency & Leases",
    subtitle: "At-least-once, exactly-once myths, and safe reprocessing",
    phaseGroup: "Phase 1 · Reliability",
    order: 2,
    readingMinutes: 8,
    status: "available",
    relatedPanel: { label: "Task Flow & Dead-Letter", href: "/playground" },
    body: deliveryGuarantees,
    keyTakeaways: [
      "There are three delivery guarantees — at-most-once (may lose tasks), at-least-once (may run tasks twice), and exactly-once — and the choice is decided by whether you ack before or after processing.",
      "Exactly-once *delivery* is impossible over an unreliable network: a lost ack leaves the broker unable to tell 'done' from 'never ran', so it must either risk losing the task or risk redelivering it (the Two Generals Problem).",
      "At-least-once is the practical default because you can defend against duplicates, but you cannot recover work that silently vanished.",
      "Idempotency makes duplicate runs harmless — running a handler twice has the same effect as running it once — achieved via idempotency keys, upserts/conditional writes, unique constraints, or downstream Idempotency-Key headers.",
      "'Effectively-once' processing = at-least-once delivery + idempotent handlers; that combination, not magic delivery, is what real systems ship.",
      "A lease (visibility timeout) hides a claimed task and auto-returns it if the worker dies without acking; renew the lease for long tasks, and tune its length to trade off false reclaims (too short) against slow crash recovery (too long).",
    ],
    revisionQuestions: [
      {
        q: "Name the three delivery guarantees and explain what decides which one you get.",
        a: "At-most-once (deliver zero or one time, never retry — you risk losing tasks), at-least-once (redeliver until acked — you risk duplicate runs), and exactly-once (run once, no loss, no duplication). In practice the choice between at-most-once and at-least-once comes down to when you acknowledge: ack BEFORE processing and a crash loses the task (at-most-once); ack AFTER processing and a crash causes redelivery (at-least-once).",
      },
      {
        q: "Why is exactly-once delivery impossible over an unreliable network?",
        a: "Because acknowledgements can be lost. When a worker finishes a task and sends an ack, that ack might never arrive (network blip, or the worker crashes right after). The broker then cannot distinguish 'the work happened but the ack was lost' from 'the work never happened'. It has only two options — redeliver (risking a second run) or drop (risking zero runs) — and neither is exactly-once. This is the Two Generals Problem: no finite message exchange over a lossy channel gives both sides certainty.",
      },
      {
        q: "What do people actually mean when they say a system is 'exactly-once', and how is it achieved?",
        a: "They mean effectively-once *processing*, not exactly-once delivery. It's achieved by combining at-least-once delivery with idempotent handlers: the broker may deliver a task more than once, but because running the handler twice has the same effect as running it once, the duplicate is harmless. Deliver possibly-twice, process effectively-once.",
      },
      {
        q: "Why do most systems (including this one) default to at-least-once rather than at-most-once?",
        a: "Because the two failure modes are not equally bad. With at-most-once you can silently lose work, and lost work is often unrecoverable and hard to even detect. With at-least-once you get occasional duplicates, which you can defend against with idempotency. Given the choice, repeating work is safer than losing it — so at-least-once is the sane default for anything that matters.",
      },
      {
        q: "What is idempotency, which operations lack it, and how do you add it?",
        a: "An operation is idempotent if running it twice has the same effect as once. Naturally idempotent: setting a field to a fixed value, or an insert guarded by a unique key. NOT idempotent: charging a card, incrementing a counter, sending an email. You add idempotency with an idempotency key (record 'handled key X', skip if seen again), upserts/conditional writes (INSERT … ON CONFLICT DO NOTHING, or update-only-if-still-pending), a database unique constraint, or by passing an Idempotency-Key to a downstream API that dedupes for you.",
      },
      {
        q: "What is a lease (visibility timeout), and how does it prevent losing a task when a worker crashes?",
        a: "When a worker claims a task, the broker doesn't delete it — it hides the task and starts a lease for N seconds. If the worker acks within the lease, the task is deleted (done). If the lease expires with no ack, the broker assumes the worker died and makes the task visible again for another worker. So a crash just means the task reappears and gets retried — it's never stranded on the dead worker. Long-running tasks renew (heartbeat) the lease so a slow-but-alive worker isn't reclaimed.",
      },
      {
        q: "What goes wrong if the lease is too short or too long?",
        a: "Too short: a healthy worker that's simply slow loses its task to the timeout, so the task runs again needlessly — manufacturing duplicates and leaning hard on your idempotency guards. Too long: after a real crash the task stays invisible for a long time before anyone retries it, so recovery is sluggish. You want the lease to comfortably exceed normal processing time, with renewal for the long tail.",
      },
    ],
    interviewSummary:
      "Delivery guarantees come in three flavours — at-most-once, at-least-once, and exactly-once — chosen mainly by whether you ack before or after processing. Exactly-once *delivery* is impossible over a lossy network (a dropped ack leaves the broker unable to tell success from failure — the Two Generals Problem), so real systems target 'effectively-once' processing = at-least-once delivery + idempotent handlers. At-least-once is the default because losing work is worse than repeating it, and idempotency (keys, upserts, unique constraints, downstream Idempotency-Key headers) makes the repeats safe. Leases (visibility timeouts) implement at-least-once: a claimed task is hidden and auto-requeued if its lease expires without an ack, with renewal for long tasks and a length tuned to trade false reclaims against crash-recovery latency. This playground is at-least-once + idempotency (not exactly-once), with a reaper reclaiming expired-lease tasks and a dead-letter queue for tasks that exhaust their retries.",
  },
  {
    slug: "two-generals-atomicity-lua",
    title: "The Two Generals Problem, Atomicity & Redis Lua",
    subtitle: "Why coordination is hard and how atomic ops help",
    phaseGroup: "Phase 1 · Reliability",
    order: 3,
    readingMinutes: 8,
    status: "available",
    relatedPanel: {
      "label": "Task Flow Pipeline",
      "href": "/playground"
    },
    body: twoGeneralsAtomicityLua,
    keyTakeaways: [
      "The Two Generals Problem is the permanent lesson that two parties cannot reach guaranteed agreement over a channel that can lose messages — a lost acknowledgement always leaves one side uncertain.",
      "Inside a single queue the same danger reappears as race windows: any logical action implemented as several separate steps has a gap where another actor can interleave or the process can crash.",
      "Check-then-act (peek ready, then remove) lets two workers grab the same task; ZREM-then-ZADD leaves a task briefly in neither set, so a crash in that gap loses it entirely.",
      "Atomicity — all-or-nothing and uninterruptible, with no observable intermediate state — closes those gaps, and Redis provides it because it runs commands on a single thread.",
      "A Lua script sent via EVAL/EVALSHA runs as one indivisible unit: every redis.call inside it executes back-to-back with no other client's command in between, so multi-step logic becomes one atomic step.",
      "This queue ships its scripts as real .lua files embedded with //go:embed and run via EVALSHA; dequeue.lua (ZPOPMIN → ZADD processing → HSET) and promote.lua (ZREM + ZADD in one script) are the concrete atomic operations, with the ZREM/ZPOPMIN return value doubling as a concurrency guard."
    ],
    revisionQuestions: [
      {
        "q": "State the Two Generals Problem and its general lesson.",
        "a": "Two allied generals can only win by attacking simultaneously, but must coordinate over a valley where messengers can be captured. General A sends a time; even if it arrives, A can't know it did until B acknowledges; B then can't know its acknowledgement arrived until A acknowledges that, and so on forever. No finite exchange of messages ever makes both sides certain. The general lesson is that you cannot achieve guaranteed agreement between two parties over a channel that can lose messages — the same impossibility that makes exactly-once delivery unattainable when an ack can be lost."
      },
      {
        "q": "How does that networking lesson reappear as a bug inside a single queue, even on one machine?",
        "a": "The 'lossy channel' becomes the gap between two operations that were meant to happen together. Whenever a single logical action is implemented as several separate steps, another actor can slip in between them, or the process can crash between them. So the coordination hazard isn't only about networks — any non-atomic multi-step operation has a race window at exactly the wrong moment."
      },
      {
        "q": "Give the two classic non-atomic race patterns in a queue and what each causes.",
        "a": "Check-then-act: dequeue done as peek-then-remove lets two workers both read the same head-of-queue task before either removes it, so the task runs twice (duplicate work). ZREM-then-ZADD: promoting a task by removing it from delayed and then adding it to ready leaves the task, for the instant between the two commands, in neither set — a crash or failover in that window loses the task permanently. This queue's promotion path actually had that loss window before P1.5."
      },
      {
        "q": "What exactly does 'atomic' mean, and why is a Redis Lua script atomic?",
        "a": "Atomic means all-or-nothing and uninterruptible: the whole operation either completes fully or not at all, and no other operation can observe a half-finished intermediate state. Redis executes commands on a single thread, one at a time from start to finish, so there is no command-level interleaving to begin with. When you send a script via EVAL or EVALSHA, Redis runs the entire script as if it were one command — every redis.call inside runs back-to-back with no other client's command in between — so a multi-step script is itself the unit of atomicity. The tradeoff is that the whole server is blocked while the script runs, so scripts must be short and fast, not heavy computation."
      },
      {
        "q": "How are the Lua scripts shipped and executed in this codebase (//go:embed and EVALSHA)?",
        "a": "The Lua lives in real .lua files under backend/internal/**/scripts/. Go bakes each file's bytes into a string at compile time with the //go:embed directive (e.g. //go:embed scripts/dequeue.lua over a var dequeueScript string), so there's no separate file to deploy and no drift between the file on disk and the code running. At startup the broker wraps each embedded string in redis.NewScript(...). The go-redis Script helper runs it via EVALSHA (execute the script Redis already cached by its SHA1 hash), and if Redis replies NOSCRIPT it falls back to EVAL, which sends the full body and caches it. After the first call, every invocation is just the 40-character hash on the wire — cheap and atomic."
      },
      {
        "q": "Walk through dequeue.lua and promote.lua and explain how each closes a race window.",
        "a": "dequeue.lua runs three commands as one: ZPOPMIN on taskqueue:ready reads and removes the head in a single command (killing the peek/remove gap so two workers can't pop the same ID), then ZADD into taskqueue:processing with the lease deadline as the score, then HSET on taskqueue:task:{id} to mark it processing and stamp the owner. Because they're indivisible, a crash can't leave a task popped-from-ready-but-not-in-processing. promote.lua fixes the old ZREM-then-ZADD loss window by doing the ZREM from taskqueue:delayed and the ZADD into taskqueue:ready inside one script, so the 'task in neither set' instant is never observable. It also uses the ZREM return value as a concurrency guard: ZREM returns 1 only for the caller that actually removed the member, so racing promoters get 0 and skip the ZADD, promoting each task exactly once. The reaper's reclaim.lua uses the identical ZREM-as-claim pattern on taskqueue:processing."
      }
    ],
    interviewSummary: "The Two Generals Problem shows that two parties can never reach guaranteed agreement over a channel that can lose messages — a lost acknowledgement always leaves someone uncertain — and the same hazard reappears inside a single queue as race windows: any logical action built from several separate steps has a gap where another actor can interleave or the process can crash. Two classic offenders are check-then-act (peek-then-remove dequeue lets two workers grab the same task) and ZREM-then-ZADD (a task sits in neither set for an instant, so a crash loses it — the exact bug this queue's promotion path had before P1.5). The fix is atomicity: all-or-nothing, uninterruptible operations with no observable intermediate state. Redis provides this because it runs commands on a single thread, so a Lua script sent via EVAL/EVALSHA executes as one indivisible unit with no other client's command in between (the tradeoff being that scripts must stay short since they block the whole server). This codebase embeds real .lua files into the Go binary with //go:embed and runs them via redis.NewScript → EVALSHA (falling back to EVAL on NOSCRIPT): dequeue.lua does ZPOPMIN(ready) → ZADD(processing, deadline) → HSET(status) atomically, and promote.lua does ZREM(delayed) + ZADD(ready) in one script, using the ZREM return value as a concurrency guard so each task is promoted exactly once. Those scripts, over the keys taskqueue:ready/delayed/processing and the taskqueue:task:{id} hash, are what let the playground move tasks between stages under many concurrent workers without losing or duplicating any.",
  },
  {
    slug: "sorted-sets-queue-design",
    title: "Sorted Sets & Queue Design",
    subtitle: "Using scores for delayed and priority queues",
    phaseGroup: "Phase 1 · Reliability",
    order: 4,
    readingMinutes: 7,
    status: "available",
    relatedPanel: {
      "label": "Queue Depth",
      "href": "/playground"
    },
    body: sortedSetsQueueDesign,
    keyTakeaways: [
      "A Redis sorted set (ZSET) stores unique members each carrying a float score, and keeps them permanently ordered by that score — so 'a queue' is just a sorted set where the score is whatever you want to sort by.",
      "This backend runs three ZSETs that differ only in what the score means: taskqueue:ready (score = priority), taskqueue:delayed (score = due timestamp), and taskqueue:processing (score = lease deadline).",
      "Priority is stored as score = -priority and tasks are popped with ZPOPMIN, so the highest priority becomes the lowest (most negative) score and is claimed first; ties break lexicographically for a stable order.",
      "The delayed queue is drained by a once-a-second promoter that runs ZRANGEBYSCORE delayed -inf now to find due tasks, then atomically ZREMs them from delayed and ZADDs them into ready with score = -priority.",
      "The processing set is a lease-expiry index sorted by deadline (now + VisibilityTimeout, 30s, in ms); a reaper every ReaperInterval (5s) runs ZRANGEBYSCORE processing -inf now to reclaim tasks whose leases lapsed — the same range-scan algorithm as the promoter, with a different meaning for the score.",
      "The ZSETs hold task IDs only; the canonical record lives once in the hash taskqueue:task:{id}, giving a single source of truth, cheap moves between sets, and compact O(log n) indexes."
    ],
    revisionQuestions: [
      {
        "q": "What is a Redis sorted set, and why is it a better queue substrate than a plain list?",
        "a": "A sorted set (ZSET) is a collection of unique members, each with a floating-point score, that Redis keeps permanently ordered by score. Unlike a list — which orders strictly by insertion — a ZSET lets you define the ordering with a number you control. That means you can pop by priority (ZPOPMIN/ZPOPMAX), or range-query 'everything due before now' (ZRANGEBYSCORE), or re-prioritise a member by simply re-ZADDing a new score. Members are unique and every core operation is O(log n), so it stays fast at scale."
      },
      {
        "q": "The system uses three sorted sets. Name them and state what the score means in each.",
        "a": "taskqueue:ready is the priority queue, where score = -priority (so higher priority sorts earlier). taskqueue:delayed is the scheduled queue, where score = the Unix timestamp in seconds at which the task becomes due (now + delay). taskqueue:processing is the lease-expiry index, where score = the lease deadline (now + VisibilityTimeout, ~30s) expressed in milliseconds. All three hold only task IDs; the ordering axis is the only thing that differs."
      },
      {
        "q": "Why is the ready-set score stored as -priority, and how does a worker claim the most important task?",
        "a": "The backend chose the convention 'smaller score = more urgent' so it can pop the minimum everywhere. Storing score = -priority maps a high priority (say 9) to a low score (-9), so it sorts before a low priority. A worker claims work with ZPOPMIN, which removes the lowest-scored member — always the highest-priority task. Enqueue is one ZADD, dequeue is one ZPOPMIN. Equal scores break ties lexicographically by member for a stable result. (You could equivalently store +priority and use ZPOPMAX; the negation just keeps ready, delayed, and processing all sorting ascending.)"
      },
      {
        "q": "How does the delayed queue become ready, and why is the move done in a Lua script?",
        "a": "A scheduler ticks once per second and runs ZRANGEBYSCORE delayed -inf now to fetch every task whose due-timestamp has passed. For each due ID it ZREMs it from delayed and ZADDs it into ready with score = -priority (read from the task hash). This ZREM-then-ZADD pair runs inside a single atomic Lua script so there is no window where a task exists in neither set (lost) or both (duplicated); the ZREM also acts as a concurrency guard so only one caller promotes each task."
      },
      {
        "q": "How does the reaper use the processing set to guarantee at-least-once delivery?",
        "a": "When a worker claims a task it is ZADDed into taskqueue:processing with score = its lease deadline (now + VisibilityTimeout, in ms) rather than being deleted. The reaper wakes every ReaperInterval (5s) and runs ZRANGEBYSCORE processing -inf now — every returned task has a deadline already in the past, meaning its worker failed to ACK in time (crash, hang, lost network). Those tasks are reclaimed back to ready (or dead-lettered if retries are exhausted). A healthy worker instead ZREMs its task on a successful ACK before the deadline, so it is never reaped."
      },
      {
        "q": "Why do the queues store only task IDs instead of the full task payload?",
        "a": "The full record — payload, status, retries, owner, priority — lives once in the hash at taskqueue:task:{id}, and the ZSETs are indexes over those tasks. Storing IDs only gives three benefits: a single source of truth (a task moves delayed → ready → processing but its data never gets copied or drifts out of sync), cheap moves (promoting or reclaiming shuffles a short string, not the heavy payload), and compact indexes so ZRANGEBYSCORE and ZPOPMIN stay fast under load."
      }
    ],
    interviewSummary: "A Redis sorted set stores unique members each with a float score and keeps them ordered by that score, which turns 'design a queue' into 'pick what the score means' — the same structure with O(log n) ZADD, ZPOPMIN/ZPOPMAX, ZRANGEBYSCORE, and ZREM serves every need. This task queue runs three ZSETs that differ only in the meaning of the score: taskqueue:ready is a priority queue storing score = -priority so ZPOPMIN pops the most urgent task first; taskqueue:delayed is a scheduled queue whose score is the Unix due-timestamp, drained by a once-a-second promoter that ZRANGEBYSCOREs -inf..now and atomically (via Lua) ZREM/ZADDs due tasks into ready; and taskqueue:processing is a lease-expiry index whose score is the lease deadline (now + VisibilityTimeout, 30s, in ms), swept by a reaper every 5s with the very same ZRANGEBYSCORE -inf..now scan to reclaim tasks whose leases lapsed. The promoter and reaper are literally the same range-scan-and-move algorithm with a different meaning for the score. Crucially, the ZSETs hold task IDs only — the canonical record lives once in the taskqueue:task:{id} hash — so a task can move between sets without its data ever being duplicated or drifting, moves are cheap, and the indexes stay compact and fast.",
  },
  {
    slug: "lua-queue-operations",
    title: "Lua Scripts & Queue Operations",
    subtitle: "Atomic enqueue, dequeue, and requeue in one round-trip",
    phaseGroup: "Phase 1 · Reliability",
    order: 5,
    readingMinutes: 7,
    status: "available",
    relatedPanel: {
      "label": "Task Flow Pipeline",
      "href": "/playground"
    },
    body: luaQueueOperations,
    keyTakeaways: [
      "Each core queue operation is a single Lua script Redis runs atomically in one round-trip, so no other command can interleave and every multi-step mutation is all-or-nothing.",
      "dequeue.lua fuses ZPOPMIN(ready) -> ZADD(processing, lease deadline) -> HSET(status/owner) -> SADD(node set); the task is never absent from both sets, so a mid-dequeue crash can neither lose nor double-lease it.",
      "ack.lua and nack.lua check ownership (ZSCORE presence + HGET owner) before mutating, then ZREM from processing; ack also marks the record completed while nack leaves retry/DLQ routing to the executor.",
      "The scripts return 1 on success and 0 when the lease is no longer held, which the Go broker surfaces as ErrLeaseNotHeld -- the fence that resolves the reaper-vs-worker race (ack becomes a harmless no-op, nack skips a double requeue).",
      "promote.lua (delayed -> ready) and reclaim.lua (expired leases -> ready with retries++ or mark failed) both use ZREM as a concurrency guard and run in bounded batches so a tick never monopolizes single-threaded Redis.",
      "Scripts are //go:embed-ed into the binary and run via redis.NewScript, which uses EVALSHA caching -- the body is uploaded once and later calls send only the SHA1 hash plus arguments, with automatic EVAL fallback on NOSCRIPT."
    ],
    revisionQuestions: [
      {
        "q": "Why must dequeue be a single Lua script rather than a client-side ZPOPMIN followed by a ZADD?",
        "a": "Because Redis runs the script to completion before serving any other command, the pop and the lease are indivisible. If they were two separate client calls, a crash between them would leave the task in neither the ready nor the processing set -- silently lost, with no lease to expire and rescue it -- and another worker could observe an inconsistent view. The single script guarantees the task is always in exactly one set and can never be double-leased."
      },
      {
        "q": "What is the KEYS/ARGV convention and why are keys passed as KEYS instead of hard-coded in the script?",
        "a": "Redis's scripting API passes the keys a script touches as KEYS[1..n] and all other parameters (deadlines, IDs, batch sizes) as ARGV[1..n]. Declaring keys through KEYS rather than hard-coding them lets Redis Cluster route the call to the correct shard and keeps each script a pure function of its inputs, which makes it deterministic and safe to cache and replay."
      },
      {
        "q": "How does EVALSHA caching reduce cost, and what happens if Redis has forgotten the script?",
        "a": "redis.NewScript wraps the source and uses EVALSHA: the first call ships the full body and Redis caches it under its SHA1 hash, so every subsequent call sends only the 40-character hash plus arguments instead of re-uploading the script. If Redis reports NOSCRIPT (for example after a restart flushed the script cache), the client transparently falls back to a full EVAL and the cache is repopulated."
      },
      {
        "q": "What is ErrLeaseNotHeld, when is it returned, and how should ack versus nack callers react?",
        "a": "The ack/nack/extend scripts return 0 when this node no longer holds the lease -- it expired, the reaper reclaimed it, or it was re-leased elsewhere -- and the broker maps that to ErrLeaseNotHeld. On ack it means the reaper already redelivered the task, so completing it is a harmless no-op (safe under at-least-once, the task just runs again). On nack it means the executor must not route to retry itself, because the reaper already handled redelivery, preventing a double requeue."
      },
      {
        "q": "Why do promote.lua and reclaim.lua rely on ZREM as a concurrency guard, and why are they batched?",
        "a": "ZREM returns whether it actually removed the member, so only the caller that removes a given task proceeds to act on it; this lets two promotion ticks, or the reaper racing a worker or another reaper, run concurrently without double-promoting or double-reclaiming. Both scripts process a bounded batch per tick (promote uses a LIMIT, reclaim caps around 100) so that a large backlog is drained steadily instead of one long script monopolizing the single-threaded server."
      },
      {
        "q": "In ack.lua, why is the ownership check performed inside the same script as the ZREM rather than as a separate client call first?",
        "a": "If the worker first read the owner client-side, then issued a ZREM, the reaper could reclaim and re-lease the task to another node in the gap between the two calls. The worker would then remove the task and mark it completed, clobbering the new owner's live lease. Bundling the ZSCORE presence check, the HGET owner comparison, and the ZREM into one atomic script closes that TOCTOU window so the fence is actually trustworthy."
      }
    ],
    interviewSummary: "In this Redis-backed queue every core operation is a small Lua script that Redis executes atomically in a single round-trip, because Redis is single-threaded and runs a script to completion before serving any other command. That serialization gives two properties at once: no interleaving (multi-step mutations are all-or-nothing) and one network hop instead of several. dequeue.lua pops the highest-priority task from the ready ZSET and leases it into the processing ZSET with a deadline score while stamping owner and status, so a task is never in neither set; ack.lua and nack.lua verify the node still owns the lease (ZSCORE presence plus HGET owner) before ZREM-ing it, with ack marking the record completed and nack leaving retry/DLQ routing to the executor; extend.lua bumps the lease deadline with no TOCTOU gap; promote.lua moves due tasks from delayed to ready in priority order; and reclaim.lua rescues expired leases back to ready with retries incremented or marks them failed for the dead-letter queue. Ack/nack/extend return 0 when the lease is no longer held, which the broker surfaces as ErrLeaseNotHeld -- the fence that resolves the reaper-versus-worker race by turning a stale ack into a no-op and suppressing a double requeue on nack. The scripts are //go:embed-ed and run through redis.NewScript, which uses EVALSHA caching so only a SHA1 hash and arguments travel on the wire after the first upload, with automatic EVAL fallback on NOSCRIPT. The through-line I would give an interviewer: don't pull data to the client and write it back (a race waiting to happen) -- send the logic to the data, keep keys in KEYS for cluster routing, use ZREM/ownership returns as concurrency guards, and batch the sweeps so they never monopolize the single-threaded server.",
  },
  {
    slug: "heartbeats-service-discovery",
    title: "Heartbeats & Service Discovery",
    subtitle: "Knowing which workers are alive right now",
    phaseGroup: "Phase 2 · Distribution",
    order: 6,
    readingMinutes: 7,
    status: "available",
    relatedPanel: {
      "label": "Cluster Nodes & Workers",
      "href": "/playground"
    },
    body: heartbeatsServiceDiscovery,
    keyTakeaways: [
      "In a cluster where workers come and go, you cannot trust a static list of who was started — a crashed worker can't report its own death, so you require every worker to keep proving it is alive and treat silence as death (a dead man's switch).",
      "The pattern is a heartbeat plus a TTL key: a worker writes a Redis key that auto-expires, then re-writes it on a timer (default every 3s) faster than the TTL (default 10s); stop beating and the key expires and the node vanishes.",
      "Each node is tracked by three keys — the registry SET 'taskqueue:nodes' (membership), the string 'taskqueue:node:{id}:hb' with a TTL (liveness, and its value is the node's full record), and the SET 'taskqueue:node:{id}:tasks' (its in-flight tasks).",
      "Node IDs are '{hostname}-{shorthex}': the hostname is human-readable in logs and the dashboard, and the random hex suffix keeps replicas that share a hostname unique; the ID stamps each task's owner field.",
      "Service discovery is the read side: GET /api/nodes (ListNodes) does SMEMBERS on the registry, then a pipelined GET of each :hb key and SCARD of each :tasks set, returning dead nodes with alive:false so the UI grays them out instead of dropping them instantly.",
      "The TTL must exceed the beat interval so a node can miss a beat without dying; a shorter TTL detects death faster but yields more false positives from GC pauses or network blips, while a longer TTL is more forgiving but strands a dead node's work for longer. This is push-based (workers announce themselves) rather than pull-based health-checking."
    ],
    revisionQuestions: [
      {
        "q": "Why can't you just keep a static list of the workers you started to know which are alive?",
        "a": "Because membership is not static: workers crash, get OOM-killed or SIGKILLed, get scaled down by an orchestrator, or lose network — and a crashed worker cannot announce its own death, so it can never remove itself from your list. A list written once is stale the instant any of that happens. Instead you invert the problem: require every worker to continuously prove it is alive, and treat silence (a missing proof) as death. That is a dead man's switch — the answer self-corrects instead of drifting out of date."
      },
      {
        "q": "Explain the heartbeat + TTL-key pattern and why the two timers must relate the way they do.",
        "a": "A worker keeps a Redis key ('taskqueue:node:{id}:hb') set to expire after a TTL (default 10s), and a background loop re-writes that key on an interval (default 3s), resetting the TTL each time. While the worker is healthy the refreshes out-run the expiry, so the key persists; if the worker dies, nothing refreshes it, Redis expires it, and the node disappears. The TTL must be larger than the interval — the config refuses to start otherwise — so a node can miss a beat or two (a slow write, a brief blip) without being wrongly declared dead. A 3s beat under a 10s TTL tolerates about two consecutive missed beats."
      },
      {
        "q": "What are the three Redis keys that track a node, and what does each one hold?",
        "a": "'taskqueue:nodes' is a SET — the registry of every known node ID (durable membership). 'taskqueue:node:{id}:hb' is a string key with a TTL whose existence is the liveness proof, and whose value is the node's JSON record (id, hostname, capacity, started-at) so one GET returns both 'is it alive?' and 'what is it?'. 'taskqueue:node:{id}:tasks' is a SET of the task IDs the node currently leases — its in-flight work, used to reclaim tasks if it dies. The registry is perishable-membership plus perishable-liveness split apart: when a node dies its :hb key expires at once, but its ID lingers in the registry set until a reaper reclaims its tasks and removes it."
      },
      {
        "q": "How does service discovery work here, and why are dead nodes still returned by GET /api/nodes?",
        "a": "Service discovery is the read side of the heartbeat pattern: rather than a static config, you ask the shared store who is alive now. GET /api/nodes calls ListNodes, which does SMEMBERS on 'taskqueue:nodes' to get every registered ID, then for each ID a pipelined GET of its :hb key and SCARD of its :tasks set. A GET hit means alive and hands back the hydrated record; a GET miss means the heartbeat expired, so the node is dead — but it is still returned with alive:false and its in-flight count, so the dashboard can render it graying out during the window before the reaper removes it, rather than making it vanish the instant it misses one beat."
      },
      {
        "q": "What is the format of a node ID and why is it built that way?",
        "a": "It is '{hostname}-{shorthex}', e.g. 'worker-7b3f-a1c2e4d6f890', computed once at startup. The hostname makes IDs human-readable in logs and on the dashboard, while the random hex suffix guarantees uniqueness when several replicas share a hostname — multiple containers, or a restart that reuses a name. The ID is stable for the life of the process and is what gets written into a task's owner field when that node leases it, so any in-flight task can be traced back to the node responsible for it."
      },
      {
        "q": "Contrast push-based heartbeats with pull-based health-checks, and give the tradeoff of choosing a short vs long TTL.",
        "a": "Push (heartbeat): each worker actively announces 'I'm alive' into a shared store on a timer, and anyone interested just reads the set — the coordinator does no per-node work, new workers are discovered for free simply by starting to write their key, and it survives a coordinator restart. Pull (health-check): a central monitor probes each worker on a schedule, which means it must know every address up front, do O(nodes) work each round, and cope with workers behind NAT or without an inbound port. This system is push-based. On the TTL: a shorter TTL detects a real death faster but produces more false positives — a healthy worker frozen by a stop-the-world GC pause or a momentary network hiccup can miss its window and be wrongly evicted, causing needless task reclaims; a longer TTL is more forgiving of those blips but leaves a genuinely dead node's tasks stranded and unrecovered for longer. The 3s/10s default is the middle ground."
      }
    ],
    interviewSummary: "In a distributed task queue, cluster membership is not a static list — workers crash, get killed, or lose network without ever announcing it — so the system uses a heartbeat plus a TTL key as a dead man's switch: each worker writes a Redis key ('taskqueue:node:{id}:hb') that auto-expires after a TTL (default 10s) and re-writes it on a shorter interval (default 3s, and the TTL must exceed the interval so a missed beat isn't fatal); stop beating and the key expires and the node vanishes. Each node is tracked by three keys — a registry SET 'taskqueue:nodes', the TTL heartbeat key whose value is the node's record (id, hostname, capacity, started-at) and whose mere existence signals liveness, and a SET 'taskqueue:node:{id}:tasks' of its in-flight tasks. Node IDs are '{hostname}-{shorthex}' for readable-yet-unique identity, and that ID owns the tasks it leases. Service discovery is the read side: GET /api/nodes reads the registry set, then pipelines a GET of each heartbeat key and an SCARD of each tasks set, returning dead nodes with alive:false so a dashboard can gray them out during the window before a reaper reclaims their work. This is push-based (workers announce themselves, giving free discovery and O(1) coordinator work) rather than pull-based health-checking, and the core tuning knob is the beat/TTL ratio: a shorter timeout detects death faster but yields more false positives from GC pauses and network blips, while a longer timeout is more forgiving but strands a dead node's tasks longer.",
  },
  {
    slug: "node-death-reclaim-ownership",
    title: "Eager Node-Death Reclaim & Worker Ownership",
    subtitle: "Recovering in-flight tasks when a worker dies",
    phaseGroup: "Phase 2 · Distribution",
    order: 7,
    readingMinutes: 8,
    status: "available",
    relatedPanel: {
      "label": "Cluster Nodes & Workers",
      "href": "/playground"
    },
    body: nodeDeathReclaimOwnership,
    keyTakeaways: [
      "When a worker dies mid-task, its in-flight work must not be stranded. There are two layered recovery paths: per-task lease expiry (the always-correct safety net, bounded by the ~30s VisibilityTimeout) and eager node-death reclaim (the fast path, bounded by the heartbeat TTL plus one reaper tick).",
      "Each node's in-flight tasks are tracked in a per-node Redis set, taskqueue:node:{id}:tasks. The dequeue Lua script SADDs the task there and stamps owner on the record; ack/reclaim SREMs it. So when a node dies the reaper knows exactly which tasks to reclaim without scanning the whole processing set.",
      "A node is alive while its heartbeat key (taskqueue:node:{id}:hb, refreshed every 3s, TTL 10s) exists. When it expires the node is detectably dead; DeadNodeIDs finds it by EXISTS returning 0, and the reaper reclaims its whole :tasks set and removes it from the registry.",
      "The reaper ticks every 5s and runs reapDeadNodes BEFORE the lease sweep. Doing dead-node reclaim first means a dead node's entire in-flight set is pushed back to ready in one tick, instead of each task waiting out its own 30s lease.",
      "Every reclaim goes through the same atomic reclaim.lua: ZREM on processing is the race guard, the task is SREMed from its owner's :tasks set, and then either Retries is incremented and the task returns to ready at its original priority (reclaimed), or, if retries are exhausted, it is marked failed and pushed to the dead-letter queue (dead_lettered).",
      "Owner fencing in ack.lua prevents double-completion: an ack only succeeds if the task is still in processing AND its owner field still equals the acking node. A resurrected 'dead' worker whose task was reclaimed and re-leased hits ErrLeaseNotHeld and stands down. Recovery narrates itself via node_dead and reclaimed events (WorkerID -1)."
    ],
    revisionQuestions: [
      {
        "q": "Why isn't lease expiry alone enough to recover a dead worker's tasks quickly?",
        "a": "Lease expiry is per-task: each in-flight task only comes back once its own visibility deadline (~30s) passes with no ack. A dead worker is usually holding many tasks, so relying on leases alone means waiting out each 30-second timeout, and recovery time scales with the visibility timeout rather than with how fast you noticed the node died. The eager node-death path fixes this by reclaiming the node's entire in-flight set the moment its heartbeat expires, so recovery is bounded by the heartbeat TTL (~10s) plus one reaper tick (5s). You keep both because they fail differently: heartbeats can be fooled by a hung-but-beating node or a network partition, and the lease is the backstop that does not depend on membership being correct."
      },
      {
        "q": "How does the system know which tasks a specific node was holding when it died?",
        "a": "Ownership is tracked in a per-node Redis SET, taskqueue:node:{id}:tasks. The lease is maintained atomically: the dequeue Lua script that moves a task into the processing ZSET also SADDs the task ID into the owning node's :tasks set and stamps the owner field on the task record; ack and reclaim SREM it back out. So that set is always a live picture of what the node holds. On death the reaper just reads the set via OwnedTaskIDs — no scanning the global processing set trying to guess which tasks belonged to the dead node."
      },
      {
        "q": "What is the order of operations inside a single reaper tick, and why does the order matter?",
        "a": "Every ReaperInterval (5s) the reaper runs reapDeadNodes() first and then reap() (the lease sweep). reapDeadNodes finds registered nodes whose heartbeat key is gone, reads each one's :tasks set, reclaims every task, and removes the node from the registry. The lease sweep then ZRANGEBYSCOREs the processing set for deadlines <= now and reclaims those in bounded batches. Doing dead-node reclaim first means a dead node's whole in-flight set is proactively pushed back to ready this tick instead of lingering until each task's individual 30-second lease expires; the lease sweep is left to mop up genuinely stalled individual tasks like a hung-but-alive worker."
      },
      {
        "q": "Walk through what reclaim.lua does and how it decides between requeueing and dead-lettering.",
        "a": "It first does ZREM on the processing set for the task; if ZREM removes nothing it returns lost_race, because whoever removed the member owns the reclaim — that single line safely resolves reaper-vs-worker and reaper-vs-reaper races. It then SREMs the task from the owning node's :tasks set. If the task record no longer exists it returns orphan. Otherwise it reads retries, max_retries, and priority from the task hash itself (so the decision is self-consistent with the authoritative record). If retries >= max_retries it sets status=failed and clears owner, returning dead_lettered (the reaper then pushes it to the DLQ). Otherwise it increments retries, sets status=pending, clears owner, and ZADDs the task back to ready at score -priority so it keeps its original priority, returning reclaimed."
      },
      {
        "q": "What is owner fencing, and what disaster does it prevent?",
        "a": "Owner fencing is the check in ack.lua that a task may only be acked by the node that still owns it. Before completing, the script verifies the task is still in the processing set (ZSCORE not false) and that the task record's owner field still equals the acking node's ID; if either check fails it returns 0, which the broker surfaces as ErrLeaseNotHeld. It prevents double-completion: if a node was presumed dead and its task got reclaimed and re-leased to a healthy node, the original worker coming back to life (after a GC pause or partition) must not be allowed to ack the task it thinks it still owns, which would mark it completed while the new owner is also running it and silently discard the new owner's work. Because reclaim clears owner and the re-lease stamps a new owner, the zombie worker can never win the check and simply drops its result as a no-op."
      },
      {
        "q": "Which events narrate node-death recovery, and how can you tell reaper activity from normal task flow?",
        "a": "The reaper emits node_dead when it detects an expired heartbeat (carrying the node ID and how many in-flight tasks it is about to reclaim), reclaimed for each task moved processing → ready with Retries incremented (carrying the attempt count), and dead_lettered when a reclaimed task had already exhausted its retries. All reaper-emitted events carry WorkerID -1 to mark them as coming from the reaper rather than any real worker, so in the event timeline you can distinguish system recovery from ordinary task processing."
      },
      {
        "q": "Why is a reclaimed task put back at its original priority instead of the back of the queue?",
        "a": "reclaim.lua re-inserts the task with ZADD score = -priority — the same scoring scheme the ready set uses — so a reclaimed high-priority task re-enters ahead of low-priority work rather than losing its place because its worker happened to die. This keeps the queue's priority contract intact across failures: a worker crash is an accident of infrastructure and should not silently demote important work."
      }
    ],
    interviewSummary: "Node-death reclaim is the reliability payoff of a distributed task queue: when a worker dies holding leased tasks, those in-flight tasks must be recovered with zero loss and zero double-execution. The design layers two recovery mechanisms. The always-correct safety net is per-task lease expiry — a reaper sweeps the processing set every 5s and requeues any task whose visibility deadline (~30s) has passed with no ack. The fast path is eager node-death reclaim, which hinges on membership: each node refreshes a heartbeat key (3s beat, 10s TTL) and every task it leases is recorded in a per-node Redis set, taskqueue:node:{id}:tasks, maintained atomically alongside the processing set. On each tick the reaper runs dead-node reclaim BEFORE the lease sweep, so when a heartbeat key has expired it reads the dead node's :tasks set and reclaims the entire in-flight batch at once — bounding recovery by the heartbeat TTL rather than by each lease. All reclaims funnel through one atomic Lua script whose leading ZREM is the race guard; it either increments retries and returns the task to ready at its original priority, or, once retries are exhausted, marks it failed and dead-letters it. The final safety property is owner fencing: ack only succeeds if the task is still in processing and its owner field still matches the acking node, so a resurrected worker whose task was already reclaimed and re-leased hits ErrLeaseNotHeld and stands down instead of causing a double-completion. The whole loop — detection, atomic reclaim, rebalance onto surviving workers, narrated by node_dead and reclaimed events — is what lets you kill a worker mid-burst and watch the cluster recover without dropping a single task.",
  },
];

/** Ordered list of phase groups, for grouping the index. */
export const phaseGroups: PhaseGroup[] = [
  "Foundations",
  "Phase 1 · Reliability",
  "Phase 2 · Distribution",
];

/** All chapters sorted by their curriculum order. */
export function getOrderedChapters(): Chapter[] {
  return [...chapters].sort((a, b) => a.order - b.order);
}

/** Look up a single chapter by slug. */
export function getChapter(slug: string): Chapter | undefined {
  return chapters.find((c) => c.slug === slug);
}

/** Only the chapters that are readable today. */
export function getAvailableChapters(): Chapter[] {
  return getOrderedChapters().filter((c) => c.status === "available");
}

/** Chapters grouped by phase, preserving curriculum order within each group. */
export function getChaptersByPhase(): { phase: PhaseGroup; chapters: Chapter[] }[] {
  const ordered = getOrderedChapters();
  return phaseGroups.map((phase) => ({
    phase,
    chapters: ordered.filter((c) => c.phaseGroup === phase),
  }));
}

/** Previous / next neighbours in curriculum order (undefined at the ends). */
export function getChapterNav(slug: string): {
  prev?: Chapter;
  next?: Chapter;
} {
  const ordered = getOrderedChapters();
  const index = ordered.findIndex((c) => c.slug === slug);
  if (index === -1) return {};
  return {
    prev: index > 0 ? ordered[index - 1] : undefined,
    next: index < ordered.length - 1 ? ordered[index + 1] : undefined,
  };
}

/** Per-phase tone used by the index + reader chips. */
export const phaseTone: Record<
  PhaseGroup,
  { chip: string; ink: string; ring: string }
> = {
  Foundations: {
    chip: "bg-state-queued/15",
    ink: "text-state-queued",
    ring: "ring-state-queued/30",
  },
  "Phase 1 · Reliability": {
    chip: "bg-state-running/15",
    ink: "text-state-running",
    ring: "ring-state-running/30",
  },
  "Phase 2 · Distribution": {
    chip: "bg-state-succeeded/15",
    ink: "text-state-succeeded",
    ring: "ring-state-succeeded/30",
  },
};
