// Chapter body for "What Is a Task Queue?"
// Authored as a TS template string to avoid webpack raw-loader / file-import
// config. Key Takeaways and Revision Questions live as structured fields on the
// chapter (see lib/learn.ts) and are intentionally NOT duplicated here.
export const markdown = `
A **task queue** is a system that lets one program hand off units of work to be
processed *later*, by *separate* workers — instead of doing that work inline,
right now, while someone waits.

If you have ever clicked "Sign up" and immediately landed on your dashboard,
yet still received a welcome email a few seconds later, you have already used a
task queue without knowing it. The website did not make you wait for the email
to be sent. It quietly wrote down "send a welcome email to this person" and let
a background worker handle it.

## The vocabulary

Task queues have a small, consistent vocabulary. Learn these five words and the
rest of the topic becomes easy to read about.

| Term | What it is |
| --- | --- |
| **Producer** | The code that creates work and puts it on the queue (often a web request handler). |
| **Task** (or **Job**) | One unit of work — "resize image 42", "send email to Sam". Just data describing what to do. |
| **Queue** | An ordered holding area where tasks wait until a worker is free. |
| **Broker** | The storage/coordination system that actually holds the queue (e.g. Redis, RabbitMQ). |
| **Worker** (or **Consumer**) | A separate process that pulls tasks off the queue and executes them. |

The flow is always the same shape:

\`\`\`text
  Producer  ──enqueue──▶  [ Queue in the Broker ]  ──dequeue──▶  Worker
 (web app)                  task task task task                 (background)
     │                                                              │
     └── returns to the user immediately          does the slow work later
\`\`\`

The producer and the worker never call each other directly. They only ever talk
*through* the queue. That single idea is the source of almost every benefit
below.

## The core problem it solves

Imagine an HTTP handler that runs when a user signs up. Inline, it might need to:

1. Create the user record.
2. Send a welcome email.
3. Generate and resize a profile avatar.
4. Kick off a video-intro transcode.

Only step 1 actually needs to finish before you can answer the user. Steps 2–4
are slow, and worse, they depend on things outside your control — an email
provider that might be down, an image library that might be slow, a transcoder
that might take a minute.

If you do all of this **synchronously** (inline, in the request):

- The user stares at a spinner for as long as the *slowest* step takes.
- If the email provider times out, the whole signup appears to fail — even
  though the account was created fine.
- Your web server thread is tied up doing heavy work instead of serving other
  requests.

A task queue breaks this apart. The handler creates the user, **enqueues** the
other three as tasks, and returns "success" instantly. Workers pick up those
tasks and do them in the background. The user's experience is fast, and a flaky
email provider can no longer break signup.

> Rule of thumb: if a piece of work does not need to finish before you reply to
> the user, it is a candidate for a task queue.

## Why teams use them

| Benefit | What it means | Example |
| --- | --- | --- |
| **Decoupling** | Producers and workers evolve independently; they only share the task format. | The web team ships without waiting on the email team. |
| **Async / background processing** | Slow work happens off the request path. | Return the page now; send the receipt later. |
| **Load leveling** | A sudden spike of tasks piles up in the queue; workers drain it at a steady pace. | A flash sale enqueues 10k emails; 5 workers send them over a few minutes. |
| **Reliability & retries** | A failed task can be retried instead of lost. | A transient network error retries automatically. |
| **Scalability** | Add more workers to process more tasks in parallel. | Double throughput by doubling workers — no code change. |
| **Scheduling / deferred work** | Run a task at a future time. | "Send a reminder in 24 hours." |
| **Rate limiting** | Control how fast work hits a fragile downstream system. | Never call the payment API more than 10×/sec. |
| **Fan-out** | One event spawns many independent tasks. | One upload → thumbnail + transcode + virus scan tasks. |

## Real-world use cases

- **Notifications** — sending email, SMS, or push messages ("your order shipped").
- **Image & video processing** — generating thumbnails, transcoding uploads.
- **Reports & documents** — building PDFs, invoices, or nightly CSV exports.
- **Order & payment pipelines** — charging a card, then updating inventory and
  emailing a receipt as separate steps.
- **Webhook delivery** — calling a customer's URL and retrying on failure.
- **ML inference & batch jobs** — running a model or crunching a dataset offline.
- **Third-party sync** — pushing data to a CRM or analytics API without blocking
  the user.

## Popular existing tools

Task queues are a mature ecosystem. Here are the ones you are most likely to meet:

| Tool | Language / Ecosystem | Backing store / notes |
| --- | --- | --- |
| **Sidekiq** | Ruby | Backed by Redis. |
| **Celery** | Python | Pluggable brokers — RabbitMQ or Redis. |
| **RQ** (Redis Queue) | Python | Redis. |
| **BullMQ** | Node.js / TypeScript | Redis. |
| **Resque** | Ruby | Redis (older; Sidekiq's predecessor in spirit). |
| **RabbitMQ** | Language-agnostic (AMQP) | A message *broker*, not a job framework itself, but very commonly used as the queue. |
| **Apache Kafka** | Distributed event streaming / log | High-throughput pipelines; semantics differ from a classic task queue (see note). |
| **Amazon SQS** | Managed (AWS) | Fully managed cloud queue. |
| **Google Cloud Tasks / Pub/Sub** | Managed (GCP) | Managed task dispatch / messaging. |
| **Azure Service Bus / Storage Queues** | Managed (Azure) | Managed enterprise messaging & queues. |
| **NATS / JetStream** | Language-agnostic | Lightweight, fast messaging. |

A quick distinction worth internalizing: **Kafka** is an append-only *log* built
for very high-throughput event streaming, where many consumers replay an ordered
stream. A classic **task queue** instead hands each task to exactly one worker
that then removes it. They overlap, but they are optimized for different jobs.

Notice the **Redis-backed family** — Sidekiq, Celery (in Redis mode), RQ,
BullMQ. That is exactly the design this playground imitates: Redis as the shared
coordination bus that producers write to and workers read from.

## How this playground fits

This project is an educational, Redis-backed distributed task queue that you can
watch working *live*. The **Task Flow Pipeline** on the Playground shows real
tasks moving through their stages — submitted → delayed → ready → picked up by
workers → completed, retried, or sent to the dead-letter queue. Everything you
just read has a moving counterpart on that screen. Open the Playground, submit a
few tasks, and watch the vocabulary above come to life.
`.trim();
