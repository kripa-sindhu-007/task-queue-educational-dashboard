// Chapter body for "Delivery Guarantees, Idempotency & Leases"
// Authored as a TS template string to avoid webpack raw-loader / file-import
// config. Key Takeaways and Revision Questions live as structured fields on the
// chapter (see lib/learn.ts) and are intentionally NOT duplicated here.
export const markdown = `
You hand a task to the queue and a worker picks it up. But what did the system
actually *promise* you? That the task runs? That it runs **exactly once**? What
happens if the worker crashes halfway through?

These questions are the heart of building a *reliable* queue. The answers are
less comforting than you might hope — and understanding them is what separates a
toy queue from one you would trust with real money.

## The three delivery guarantees

Every messaging or queue system makes one of three promises about how many times
a task gets delivered to a worker:

| Guarantee | Promise | You risk | When it fits |
| --- | --- | --- | --- |
| **At-most-once** | Deliver zero or one time; never retry. | **Losing** tasks. | Metrics, logs, best-effort telemetry where a dropped item is harmless. |
| **At-least-once** | Keep redelivering until the worker confirms success. | **Duplicate** runs. | Almost everything that matters — the practical default. |
| **Exactly-once** | Each task runs once, no loss, no duplication. | (The catch — see below.) | The thing everyone *wants* and nobody quite gets. |

The difference between the first two comes down to **when you acknowledge** a
task:

- **Ack _before_ processing** → if the worker then crashes, the task is already
  gone. That is *at-most-once*: fast, but lossy.
- **Ack _after_ processing** → if the worker crashes before acking, the task is
  redelivered. That is *at-least-once*: safe, but you might run it twice.

## Why "exactly-once delivery" is a myth

Exactly-once *delivery* is impossible over an unreliable network, and the reason
is beautifully simple. Picture a worker that finishes a task and then must tell
the broker "done":

\`\`\`text
  Worker                         Broker
    │   process task ✔             │
    │ ─────── ACK ──────▶ ✗ (lost) │   did the worker finish, or not?
    │                              │   the broker CANNOT tell.
\`\`\`

If that ACK is lost — network blip, worker crash a millisecond later — the
broker is stuck with an unanswerable question: *did the work happen?* It has
only two choices, and both are imperfect:

1. Assume it failed and **redeliver** → the task might run twice (**at-least-once**).
2. Assume it succeeded and **drop** it → the task might never have run (**at-most-once**).

There is no third option. This is the same impossibility as the classic
**Two Generals Problem**: no finite exchange of messages over a lossy channel
can give both sides certainty. So no honest system offers exactly-once
*delivery*.

> What people *actually* mean by "exactly-once" is **effectively-once
> processing** — and that is achievable. The recipe: **at-least-once delivery**
> plus **idempotent handlers**. Deliver possibly-twice, but make running twice
> harmless.

## At-least-once is the sane default

Given the choice between *losing* work and *repeating* work, most systems choose
to repeat — because you can defend against duplicates, but you cannot recover
work that silently vanished. So the broker keeps redelivering a task until a
worker explicitly acks it, and this playground does exactly that.

The cost of that choice is that **your handlers will occasionally run twice.**
Which brings us to the property that makes at-least-once safe.

## Idempotency: making duplicates harmless

A handler is **idempotent** if running it twice has the same effect as running
it once. Some operations are naturally idempotent; many important ones are not.

| Operation | Idempotent? | Why |
| --- | --- | --- |
| \`SET user.status = "active"\` | ✅ Yes | Running it again lands on the same state. |
| \`balance = balance - 10\` | ❌ No | Twice charges \\$20. |
| \`counter += 1\` | ❌ No | Twice counts two. |
| "Send welcome email" | ❌ No | Twice spams the user. |
| \`INSERT order (id=42)\` with a unique key | ✅ Yes | The second insert is rejected as a duplicate. |

When an operation is not naturally idempotent, you *make* it idempotent:

- **Idempotency key** — attach a stable unique id to the task (e.g. the order
  id). Before doing the work, record "I have handled key X"; if you see X again,
  skip it. A \`processed_keys\` table or a Redis \`SET key NX\` is enough.
- **Upsert / conditional writes** — \`INSERT … ON CONFLICT DO NOTHING\`, or
  "update only if status is still pending", so a replay is a no-op.
- **Natural dedup** — lean on a unique constraint the database already enforces.
- **Delegate to the downstream** — many payment and email APIs accept an
  \`Idempotency-Key\` header and dedupe for you.

> Design rule: assume every handler will run **at least twice**, because
> eventually one of them will. If a second run would double-charge, double-email,
> or double-count, it needs an idempotency guard.

## Leases: how at-least-once actually works

Here is the mechanism that lets a queue redeliver a crashed worker's task
*without* losing it. When a worker claims a task, the broker does **not** delete
it. Instead it **hides** the task and starts a **lease** (also called a
*visibility timeout*): *"this task is claimed for the next N seconds."*

\`\`\`text
  worker claims task
        │
        ▼
  ┌───────────────── lease (N seconds) ─────────────────┐
  │ task is hidden — invisible to other workers         │
  └─────────────────────────────────────────────────────┘
        │                                   │
   ACK before expiry                   lease EXPIRES (no ack)
        │                                   │
        ▼                                   ▼
   task deleted ✔                  task becomes visible again →
   (done, gone)                    redelivered to another worker
\`\`\`

Two outcomes, and only two:

- **The worker acks in time** → the broker deletes the task. Done, exactly the
  once.
- **The lease expires with no ack** → the broker assumes the worker died (crash,
  hang, GC pause, lost network) and makes the task visible again, so another
  worker can pick it up. **No task is stranded on a dead worker.**

A worker can also **nack** (negative-ack) a task it knows it failed, sending it
straight back for retry instead of waiting for the lease to time out.

### Lease renewal for long tasks

What if a task legitimately takes longer than the lease? The worker
**renews** (extends) the lease periodically — a heartbeat that says "still
working, don't reclaim me." If those heartbeats stop, the lease lapses and the
task is reclaimed. This is how the system tells "slow but alive" from "dead."

### The lease-duration tradeoff

Picking the lease length is a real tension:

- **Too short** → a slow-but-healthy worker gets its task stolen and run again,
  manufacturing needless duplicates (and hammering your idempotency guards).
- **Too long** → after a *real* crash the task sits invisible for ages before
  anyone retries it, hurting recovery time.

Tune the lease to comfortably exceed normal processing time, and renew for the
long tail.

## Putting it together

\`\`\`text
  at-least-once delivery   +   idempotent handlers   =   effectively-once
     (leases + retries)         (safe to replay)          processing
\`\`\`

That equation is the reliability contract of essentially every serious task
queue. Neither half is optional: leases keep work from being lost, idempotency
keeps replays from doing damage. And after some bounded number of retries, a
task that simply refuses to succeed is moved aside to a **dead-letter queue** so
it stops blocking healthy work and can be inspected by a human.

## How this playground fits

This playground is deliberately **at-least-once with idempotency — not
exactly-once**, exactly as described above. Redis holds each in-flight task
under a **lease**; a background **reaper** watches for leases that expire and
requeues those tasks, so a crashed worker never strands its work. Tasks that
exhaust their retry budget land in the **dead-letter queue**.

Open the Playground and watch it happen: submit a task and see a worker lease it;
if the worker never acks, the lease expires and the reaper puts the task back —
the same *"lease expired → reaped → requeued, no work lost"* beat you saw on the
landing page. That single loop is this entire chapter, running live.
`.trim();
