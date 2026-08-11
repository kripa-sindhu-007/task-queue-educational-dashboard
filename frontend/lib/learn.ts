import { markdown as whatIsATaskQueue } from "@/content/learn/what-is-a-task-queue";

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
    status: "coming-soon",
  },
  {
    slug: "two-generals-atomicity-lua",
    title: "The Two Generals Problem, Atomicity & Redis Lua",
    subtitle: "Why coordination is hard and how atomic ops help",
    phaseGroup: "Phase 1 · Reliability",
    order: 3,
    readingMinutes: 8,
    status: "coming-soon",
  },
  {
    slug: "sorted-sets-queue-design",
    title: "Sorted Sets & Queue Design",
    subtitle: "Using scores for delayed and priority queues",
    phaseGroup: "Phase 1 · Reliability",
    order: 4,
    readingMinutes: 7,
    status: "coming-soon",
  },
  {
    slug: "lua-queue-operations",
    title: "Lua Scripts & Queue Operations",
    subtitle: "Atomic enqueue, dequeue, and requeue in one round-trip",
    phaseGroup: "Phase 1 · Reliability",
    order: 5,
    readingMinutes: 7,
    status: "coming-soon",
  },
  {
    slug: "heartbeats-service-discovery",
    title: "Heartbeats & Service Discovery",
    subtitle: "Knowing which workers are alive right now",
    phaseGroup: "Phase 2 · Distribution",
    order: 6,
    readingMinutes: 7,
    status: "coming-soon",
  },
  {
    slug: "node-death-reclaim-ownership",
    title: "Eager Node-Death Reclaim & Worker Ownership",
    subtitle: "Recovering in-flight tasks when a worker dies",
    phaseGroup: "Phase 2 · Distribution",
    order: 7,
    readingMinutes: 8,
    status: "coming-soon",
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
