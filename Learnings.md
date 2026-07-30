# Distributed Systems Notes – Day 1
## Delivery Guarantees, Idempotency & Leases

---

# Goal

Understand why distributed systems cannot reliably determine whether work has completed, and why modern systems rely on **at-least-once delivery** with **idempotent processing**.

---

# 1. The Fundamental Problem: Unreliable Networks

A queue communicates with workers over an unreliable network.

Suppose:

1. Queue sends Task #42.
2. Worker processes it successfully.
3. Worker sends ACK.
4. ACK is lost.

From the queue's perspective:

- Did the worker crash?
- Was the ACK lost?
- Is the worker slow?
- Is the network delayed?

**The queue cannot know.**

### Core Principle

> A missing ACK tells you only one thing:
>
> **You didn't receive an ACK.**
>
> It does NOT tell you why.

This is the first principle of distributed systems.

---

# 2. Why Only Two Delivery Guarantees Exist

When the queue times out waiting for an ACK, it has only two choices.

## Option 1 — Retry

If the queue retries:

- Duplicate deliveries may happen.
- But no task is permanently lost.

This is called:

> **At-Least-Once Delivery**

---

## Option 2 — Don't Retry

If the queue assumes success:

- Duplicate deliveries never happen.
- But tasks may be lost forever.

This is called:

> **At-Most-Once Delivery**

---

## Key Insight

There is no third option that always knows the correct answer.

Because the queue cannot distinguish:

- slow worker
- crashed worker
- lost ACK
- network delay

---

# 3. Exactly-Once Delivery vs Exactly-Once Processing

These are completely different concepts.

## Delivery

Delivery asks:

> How many times was the message handed to the worker?

Possible answers:

- Once
- Twice
- Three times

---

## Processing

Processing asks:

> How many times did the business operation actually happen?

Desired answer:

> Exactly once.

---

## Important Interview Statement

> Exactly-once delivery is impossible over an unreliable network.

However,

> Exactly-once processing is achievable.

How?

```
At-Least-Once Delivery
+
Idempotent Consumer
=
Exactly-Once Processing
```

This is how modern distributed systems are built.

---

# 4. Idempotency

## Definition

An operation is **idempotent** if executing it multiple times produces the same final effect as executing it once.

Notice:

It does **NOT** mean:

> "The function executes only once."

Instead, it means:

> "The final state remains the same after repeated executions."

---

## Examples

### ✅ Idempotent

Set user status:

```
Status = ACTIVE
```

Running this multiple times keeps the same state.

---

Delete a file:

```
Delete(file)
```

Deleting an already deleted file changes nothing.

---

### ❌ Not Idempotent

Increment page views:

```
views++
```

Each execution changes state.

---

Add ₹1000 to balance:

```
balance += 1000
```

Each execution changes state again.

---

## How Workers Achieve Idempotency

Every task has a unique Task ID.

Example:

```
Task ID = 42
```

Worker checks:

```
Have I already processed Task #42?
```

If NO:

- Process
- Save Task ID

If YES:

- Skip processing

Duplicate delivery becomes harmless.

---

# 5. Visibility Timeout

When a worker receives a task:

**The queue does NOT delete it immediately.**

Instead:

- Hide the task temporarily.
- Wait for ACK.

If ACK arrives:

```
Delete permanently.
```

If ACK never arrives:

```
Make task visible again.
```

Another worker can retry it.

---

# 6. Lease

A visibility timeout is actually a **Lease**.

## Definition

A lease is:

> Time-bounded ownership.

The queue is effectively saying:

> "You own this task for the next 30 seconds."

Not forever.

Only temporarily.

---

When the lease expires:

The queue is free to give the task to another worker.

---

## Important Difference

A lease is NOT a lock.

It only guarantees temporary ownership.

It does NOT guarantee:

> Only one worker will ever execute this task.

Lease expiry can result in multiple workers processing the same task.

That is expected.

Idempotency handles duplicates.

---

# 7. Why Heartbeats Exist

Problem:

Suppose

- Lease = 30 seconds
- Task duration = 2 minutes

Timeline:

```
00:00 Worker A starts

00:30 Lease expires

00:31 Worker B starts

01:01 Lease expires

01:02 Worker C starts

02:00 Worker A finishes
```

Without heartbeats:

Many workers may process the same task.

---

## Solution

Workers periodically renew the lease.

Example:

```
00:00 Lease granted

00:20 Heartbeat

Lease extended

00:40 Heartbeat

Lease extended

01:00 Heartbeat

Lease extended
```

If worker crashes:

Heartbeats stop.

Lease eventually expires.

Task becomes visible again.

---

# 8. Relationship Between Concepts

```
Visibility Timeout
        │
        ▼
      Lease
        │
        ▼
Heartbeat extends Lease
        │
        ▼
Crash stops Heartbeats
        │
        ▼
Lease expires
        │
        ▼
Task becomes visible again
```

These are all manifestations of the same underlying idea:

**Time-bounded ownership.**

---

# Key Takeaways

- A missing ACK does not mean failure.
- The queue cannot distinguish slow from dead.
- Duplicate delivery is expected in distributed systems.
- Business logic—not the queue—handles duplicates.
- Idempotency means the same final state after repeated execution.
- Visibility timeout is a lease.
- Heartbeats renew leases.
- Lease expiry enables automatic recovery.

---

# Interview Summary

If asked:

**"How do distributed queues guarantee exactly-once execution?"**

A good answer is:

> "They don't guarantee exactly-once delivery. Distributed systems typically provide at-least-once delivery because the queue cannot reliably determine whether a worker completed a task or an acknowledgment was lost. Instead, they achieve exactly-once processing by making consumers idempotent using unique task IDs or deduplication. Visibility timeouts are implemented as leases, and workers periodically renew those leases using heartbeats."

---

# Revision Questions

1. Why can't a queue distinguish a slow worker from a crashed worker?
2. Why are at-most-once and at-least-once the only practical delivery guarantees?
3. Why is exactly-once delivery impossible?
4. How is exactly-once processing achieved?
5. What is idempotency?
6. Why does a worker need to remember processed Task IDs?
7. Why doesn't the queue delete a task immediately after delivery?
8. What is a lease?
9. Why are leases used instead of permanent ownership?
10. Why do long-running tasks require heartbeats?

# Distributed Systems Notes – Day 2
## Two Generals Problem, Atomicity & Redis Lua

---

# Goal

Understand why **perfect certainty is impossible** in distributed systems, why **atomic operations** are required, and how **Redis Lua** solves multi-step race conditions.

---

# 1. The Two Generals Problem

Imagine two generals who must attack at exactly the same time.

They communicate through messengers.

The problem:

> Messengers can be lost.

---

## Scenario

General A sends:

> Attack at dawn.

General B receives it.

Now General B knows the plan.

But...

Does General A know that General B received the message?

No.

So B sends an ACK.

Now A knows.

But...

Does B know that A received the ACK?

No.

So A sends another ACK.

Now...

Does A know that B received the second ACK?

Again, no.

This process never ends.

---

## Core Insight

No matter how many acknowledgments are exchanged,

> There is always one final message whose delivery is uncertain.

Therefore,

> Perfect certainty can never be achieved over an unreliable network.

---

# 2. Common Knowledge

Knowing something yourself is not enough.

Distributed systems require:

```
A knows

↓

B knows

↓

A knows B knows

↓

B knows A knows B knows

↓

...
```

This infinite chain is called:

> Common Knowledge

The Two Generals Problem proves:

> Common knowledge cannot be established over an unreliable communication channel.

---

# 3. Why Exactly-Once Delivery is Impossible

Suppose:

Queue sends Task #42.

Worker processes it.

Worker sends ACK.

ACK is lost.

Now the queue asks:

- Did the worker finish?
- Did the ACK get lost?
- Did the worker crash?

It cannot know.

Sending another ACK does not solve the problem.

That ACK could also be lost.

This becomes the Two Generals Problem.

---

## Important Interview Statement

> Exactly-once delivery is impossible over an unreliable network.

However,

```
At-Least-Once Delivery
+
Idempotent Consumer
=
Exactly-Once Processing
```

Modern distributed systems guarantee processing—not delivery.

---

# 4. Atomicity

Suppose dequeue is implemented as:

1. Read highest priority task
2. Remove from Ready Queue
3. Add to Processing Queue
4. Update task status

Now imagine:

```
Read

↓

Remove

↓

💥 Application crashes

↓

Add never happens
```

Result:

The task exists:

- Not in Ready Queue
- Not in Processing Queue

It has been permanently lost.

This is called a:

> Partial Update

---

## Definition

Atomicity means:

> A group of operations behaves as one indivisible unit.

Either:

- Everything succeeds

or

- Nothing happens

Never a partial state.

---

# 5. Why Redis Lua Exists

Without Lua:

```go
redis.ZRem(...)
redis.ZAdd(...)
redis.HSet(...)
```

Each Redis command is sent separately.

If the application crashes between them,

partial updates occur.

---

With Lua:

```
EVAL dequeue.lua
```

Redis executes:

- Remove task
- Add to processing
- Update task hash

as one atomic operation.

No client can observe an intermediate state.

---

# 6. What "Atomic" Means in Redis Lua

A Redis Lua script executes as:

> One uninterrupted Redis command.

While the script runs:

- No other Redis commands execute.
- No other client can interleave operations.
- Other clients wait until the script completes.

Other clients observe only:

- State before the script
- State after the script

Never the half-finished state.

---

# 7. Redis Lua is NOT a SQL Transaction

Atomic does NOT mean rollback.

Example:

```lua
redis.call("SET", "x", "1")
error("oops")
```

The SET is **not automatically undone**.

Redis Lua provides:

- Atomic execution
- Isolation from other clients

It does NOT provide:

- Automatic rollback

---

# 8. Why Lua is Still Safe

Failures usually occur:

- Before sending the Lua script
- After Redis finishes executing it

They do NOT occur between Redis commands inside the script.

Instead of:

```
Go

↓

ZREM

↓

💥 Crash

↓

ZADD
```

We now have:

```
Go

↓

Send Lua script

↓

Redis executes everything atomically

↓

Go crashes
```

The dangerous crash window disappears.

---

# 9. Redis Execution Model

Suppose 100 workers call:

```
dequeue.lua
```

at exactly the same time.

Redis does NOT execute all scripts simultaneously.

Instead:

```
Worker 1

↓

Worker 2

↓

Worker 3

↓

...
```

Redis executes one script at a time.

This guarantees that only one worker can successfully claim a task.

Redis itself becomes the synchronization mechanism.

---

# 10. Why Lua Cannot Execute Blocking Commands

Redis processes only one command/script at a time.

If a Lua script blocks for 10 seconds:

- Every other client waits.
- Redis appears frozen.

Therefore:

Lua scripts cannot call blocking commands such as:

```
BZPOPMIN
```

Blocking operations must happen **outside** Lua.

A common pattern is:

```
Blocking Pop

↓

Wake worker

↓

Run Lua script

↓

Atomically claim task
```

---

# Key Takeaways

- Perfect certainty is impossible in distributed systems.
- The Two Generals Problem explains why acknowledgments can never provide absolute certainty.
- Exactly-once delivery is impossible.
- Exactly-once processing is achieved using at-least-once delivery and idempotent consumers.
- Atomicity prevents partial updates.
- Redis Lua moves multi-step operations into Redis itself.
- Redis executes one Lua script at a time.
- Redis Lua provides atomic execution, not rollback.
- Redis itself acts as the synchronization mechanism.
- Blocking commands are not allowed inside Lua scripts.

---

# Interview Summary

If asked:

**"Why do distributed queues use Redis Lua?"**

A strong answer:

> "Queue operations such as dequeue are multi-step operations involving multiple Redis data structures. Executing those commands individually from the client introduces crash windows and race conditions. Redis Lua allows these related operations to execute atomically on the Redis server, ensuring that other clients never observe intermediate states. Redis processes Lua scripts one at a time, making them an effective synchronization mechanism. While Lua provides atomic execution, it does not provide automatic rollback like SQL transactions."

---

# Revision Questions

1. What is the Two Generals Problem?
2. Why can acknowledgments never provide absolute certainty?
3. What is Common Knowledge?
4. Why is exactly-once delivery impossible?
5. Why is exactly-once processing still achievable?
6. What is a partial update?
7. What is atomicity?
8. Why is implementing dequeue from Go using multiple Redis commands unsafe?
9. What guarantees does Redis Lua provide?
10. Why doesn't Redis Lua support blocking commands?
11. Why does Redis execute only one Lua script at a time?
12. What is the difference between Redis Lua atomicity and SQL transactions?

# Distributed Systems Notes – Day 3
## Redis Sorted Sets (ZSET), Queue Design & Redis Data Model

---

# Goal

Understand why **Redis Sorted Sets (ZSETs)** are the backbone of distributed task queues and how production systems model queues using a combination of **ZSETs**, **Hashes**, and **Lua scripts**.

---

# 1. Why Redis Introduced Sorted Sets

A normal Redis Set stores only unique members.

Example:

```
Alice
Bob
Charlie
```

A Set has:

- Unique members
- No ordering

Therefore it cannot answer:

> "Which task should execute first?"

---

A Sorted Set stores:

```
(Member, Score)
```

Example:

```
TaskA -> 100
TaskB -> 20
TaskC -> 50
```

Redis automatically keeps members sorted by score.

---

# 2. Member vs Score

Every Sorted Set entry contains:

```
(Member, Score)
```

Example:

```
(Task42, 10)
```

Member:

- Actual object identifier

Score:

- Numeric value used only for ordering

Redis does **not** know what the score means.

The application decides its meaning.

---

# 3. The Score Has No Meaning

The score is simply a number.

Its interpretation depends entirely on the application.

Examples:

### Priority Queue

```
TaskA -> 1
TaskB -> 5
TaskC -> 10
```

Score means:

> Priority

---

### Delayed Queue

```
TaskA -> 1752438000000
TaskB -> 1752438010000
```

Score means:

> Scheduled execution timestamp

---

### Processing Queue

```
TaskA -> 1752438030000
```

Score means:

> Lease expiration timestamp

---

The same Redis data structure can therefore implement:

- Priority Queue
- Delayed Scheduler
- Processing Queue
- Leaderboard

Only the interpretation of the score changes.

---

# 4. Ordering Rules

Redis sorts members using:

1. Score (primary ordering)
2. Member name (lexicographical tie-breaker)

Example:

```
TaskA -> 10
TaskB -> 10
TaskC -> 20
```

Redis order becomes:

```
TaskA
TaskB
TaskC
```

because:

```
TaskA < TaskB
```

lexicographically.

---

## Why Millisecond Timestamps?

Using seconds increases score collisions.

Example:

```
TaskA -> 1752438000
TaskB -> 1752438000
TaskC -> 1752438000
```

Redis falls back to lexicographical ordering.

Instead use:

```
1752438000123
1752438000456
1752438000789
```

Millisecond timestamps greatly reduce collisions.

---

# 5. Core Sorted Set Commands

## ZADD

Purpose:

> Add a member with a score.

Example:

```
ZADD ready 10 TaskA
```

Redis automatically places the member in the correct position.

---

## ZPOPMIN

Purpose:

> Atomically remove and return the member with the smallest score.

Example:

```
ZPOPMIN ready
```

Returns:

```
TaskA
Score = 10
```

and removes it from the Ready Queue.

---

## Why ZPOPMIN Exists

Without it:

```
ZRANGE

↓

ZREM
```

Two workers could both read the same task.

ZPOPMIN combines:

- Find minimum
- Remove minimum

into one atomic Redis command.

---

## ZRANGEBYSCORE

Purpose:

> Return every member whose score lies within a range.

Example:

```
ZRANGEBYSCORE processing -inf now
```

Used by the Reaper to find expired leases.

---

## ZREM

Purpose:

> Remove a specific member.

Example:

```
ZREM processing Task42
```

Return value:

```
1
```

Member removed successfully.

or

```
0
```

Member already removed.

---

### Why ZREM's Return Value Matters

Multiple workers or reapers may race.

Only one receives:

```
1
```

The others receive:

```
0
```

This becomes a natural concurrency guard.

No distributed lock is required.

---

## ZSCORE

Purpose:

> Return the score of a member.

Example:

```
ZSCORE processing Task42
```

Returns:

```
1752438030000
```

Useful for:

- Debugging
- Dashboard
- Lease inspection

---

# 6. Queue Design Using ZSETs

The queue consists of three Sorted Sets.

---

## Ready Queue

```
Task42 -> Priority
```

Score means:

> Priority

---

## Delayed Queue

```
Task42 -> Scheduled Timestamp
```

Score means:

> Execute after this time

---

## Processing Queue

```
Task42 -> Lease Expiry Timestamp
```

Score means:

> Lease expires at this time

---

The same Redis data structure powers all three queues.

Only the score semantics differ.

---

# 7. Why ZPOPMIN Alone Is Not Enough

Suppose:

```
Worker

↓

ZPOPMIN Ready

↓

💥 Worker crashes
```

Task disappears forever.

This violates:

> At-Least-Once Delivery

The task must first enter a Processing Queue before being handed to the worker.

---

# 8. Why Lua Is Required

A dequeue operation actually consists of multiple Redis operations.

Conceptually:

```
1. Remove from Ready

↓

2. Add to Processing

↓

3. Update task status

↓

4. Set lease expiry

↓

5. Return task
```

If Go sends these commands individually:

```
ZPOPMIN

↓

💥 Crash

↓

ZADD never happens
```

Task is lost.

Instead:

```
EVAL dequeue.lua
```

Redis executes every step atomically.

---

# 9. Redis Hashes

A Sorted Set should store only:

```
Task ID

+

Score
```

Task metadata belongs elsewhere.

Example:

```
task:42

status = processing
type = send_email
payload = {...}
priority = 5
leaseExpiry = ...
retryCount = 2
```

Stored as a Redis Hash.

---

## Why Separate Hashes?

### Separation of Responsibilities

ZSET:

> Ordering

Hash:

> Metadata

---

### Cheap Updates

Only update:

```
status
```

instead of rewriting the whole task.

---

### Avoid Duplicate Copies

The same task may move through:

```
Ready

↓

Processing

↓

Delayed
```

Only the queue changes.

The task record remains the same.

---

### Memory Efficient

Queues store only Task IDs.

Large payloads remain inside the Hash.

---

# 10. Redis Schema

```
Ready Queue (ZSET)

↓

Task ID

↓

Task Hash (HASH)

↑

Processing Queue (ZSET)

↑

Delayed Queue (ZSET)
```

ZSETs act like indexes.

Hashes store the complete task.

---

# 11. The Reaper

Definition:

A Reaper is a background recovery process.

Its job:

> Find expired leases and return abandoned tasks back to the Ready Queue.

---

Every few seconds:

```
ZRANGEBYSCORE processing -inf now

↓

Find expired tasks

↓

ZREM processing

↓

ZADD ready

↓

Update task status
```

Workers process tasks.

The Reaper recovers abandoned tasks.

---

# 12. Leases Solve Uncertainty

Suppose:

```
Redis executes dequeue.lua

↓

Moves Task42 to Processing

↓

Returns Task42

↓

Network drops response
```

Redis cannot know whether:

- Worker received it
- Network dropped it
- Worker crashed

Redis simply waits.

If:

```
ACK arrives
```

Delete task.

Otherwise:

```
Lease expires

↓

Reaper retries
```

The same lease mechanism solves:

- Worker crashes
- Lost ACKs
- Lost dequeue responses
- Network failures

---

# Key Takeaways

- A Sorted Set stores (Member, Score).
- Redis does not know what a score means.
- The application defines score semantics.
- One ZSET can represent priorities, timestamps or lease expiry.
- Redis orders first by score, then lexicographically.
- Millisecond timestamps reduce score collisions.
- ZPOPMIN atomically claims work.
- ZRANGEBYSCORE finds expired leases.
- ZREM's return value acts as a concurrency guard.
- Hashes store metadata; ZSETs store ordering.
- A dequeue operation requires Lua because it spans multiple Redis commands.
- A Reaper periodically recovers abandoned tasks.
- Leases solve uncertainty, not just worker crashes.

---

# Interview Summary

If asked:

**"Why do production Redis queues use ZSET + HASH together?"**

A strong answer:

> "The Sorted Set is responsible only for ordering tasks using scores such as priority or lease expiration timestamps. The complete task metadata is stored separately in Redis Hashes. This keeps queues lightweight, avoids duplicate task copies as tasks move between queues, and allows metadata updates without modifying queue entries."

---

If asked:

**"What is a Reaper?"**

A strong answer:

> "A Reaper is a background recovery process that periodically scans the processing queue for expired leases. It assumes those tasks were abandoned due to worker failure or communication problems and safely moves them back to the ready queue, enabling at-least-once delivery."

---

# Revision Questions

1. Why does Redis have Sorted Sets instead of only Sets?
2. What is the difference between a Member and a Score?
3. Why does Redis not attach meaning to a score?
4. Why are millisecond timestamps preferred over seconds?
5. Why does ZPOPMIN exist instead of using ZRANGE + ZREM?
6. Why is ZREM's return value useful during races?
7. Why does a task queue need three different ZSETs?
8. Why should task metadata live in a Redis Hash?
9. Why is Lua required for dequeue?
10. What is the role of the Reaper?
11. How do leases recover from lost dequeue responses?
12. Why is ZSET + HASH a better design than storing everything inside the ZSET?

# Distributed Systems Notes – Day 4
## Redis Lua, Atomic Operations & Queue Script Design

---

# Goal

Understand why Redis Lua exists, how Lua scripts are executed, why Redis separates `KEYS` and `ARGV`, how `EVALSHA` works, and design the three core queue operations (`enqueue`, `dequeue`, `ack`) from first principles.

---

# 1. Why Redis Uses Lua

Redis needed a way to execute multiple Redis commands as **one atomic operation**.

Instead of inventing a new scripting language, Redis embedded **Lua**, a lightweight and mature scripting language.

The goal is **not** to program in Lua.

The goal is to package multiple Redis commands into one atomic operation.

---

# 2. Why Go Alone Is Not Enough

Suppose Go performs:

```
ZPOPMIN ready

↓

ZADD processing

↓

HSET task metadata
```

Problems:

### Crash Window

```
ZPOPMIN

↓

💥 Application crashes

↓

Task is permanently lost
```

---

### Command Interleaving

Two workers:

```
Worker A : ZPOPMIN

Worker B : ZPOPMIN

Worker A : ZADD

Worker B : ZADD

Worker A : HSET

Worker B : HSET
```

Although each Redis command is atomic, the **logical dequeue operation** is split apart.

---

## Solution

Move the entire dequeue operation into a single Lua script.

Redis executes the complete script atomically.

---

# 3. What Atomic Means in Redis Lua

A Lua script executes as one uninterrupted Redis operation.

```
Worker

↓

EVAL dequeue.lua

↓

Redis executes

ZPOPMIN

↓

ZADD

↓

HSET

↓

HSET

↓

Return Task
```

No other Redis command can execute until the script completes.

---

## Important

Lua provides:

- Atomic execution
- No command interleaving

Lua does **not** provide:

- Automatic rollback

If an error occurs after some commands have executed, Redis does not undo earlier commands.

Therefore scripts should be designed carefully.

---

# 4. EVAL

Redis executes Lua scripts using:

```
EVAL
```

Conceptually:

```
Go

↓

EVAL

↓

Lua Script

↓

Redis Commands

↓

Return Result
```

EVAL is simply the entry point into Redis's Lua engine.

---

# 5. KEYS vs ARGV

Lua scripts receive two categories of inputs.

## KEYS

Every Redis key the script touches.

Examples:

```
ready

processing

task:42
```

These represent Redis objects.

---

## ARGV

Ordinary values.

Examples:

```
leaseExpiry

workerID

currentTime

priority

payload
```

These are data values.

---

## Rule

Ask:

> "Is this a Redis key?"

If yes:

```
KEYS
```

Otherwise:

```
ARGV
```

---

## Why Redis Separates Them

Redis Cluster must know every Redis key before executing the script.

It can then:

- Route the script correctly
- Verify all keys belong to the correct hash slot
- Reject invalid scripts early

Even in a single-node setup, scripts should always be written cluster-safe.

---

# 6. redis.call() vs redis.pcall()

Redis exposes two APIs inside Lua.

---

## redis.call()

Behavior:

```
Execute command

↓

If error

↓

Abort script immediately
```

Use when every Redis operation is essential.

Example:

```
ZPOPMIN

↓

ZADD

↓

HSET
```

If any step fails, the dequeue operation should fail.

---

## redis.pcall()

Behavior:

```
Execute command

↓

If error

↓

Return error object

↓

Script decides what to do
```

Useful for optional or recoverable operations.

---

## Project Decision

For the queue implementation, almost every Redis operation should use:

```
redis.call()
```

because dequeue, enqueue and ACK are all-or-nothing operations.

---

# 7. EVALSHA

Sending a large Lua script every request is wasteful.

Example:

```
Worker

↓

300-line Lua Script

↓

Redis
```

Repeated thousands of times.

---

Redis instead computes a SHA-1 hash of the script.

First execution:

```
EVAL

↓

Redis caches script

↓

SHA generated
```

Future executions:

```
EVALSHA

↓

SHA

↓

Redis executes cached script
```

No script transmission.

No parsing.

No recompilation.

---

## Redis Restart

Script cache lives in memory.

After Redis restarts:

```
EVALSHA

↓

Script missing

↓

Client sends EVAL

↓

Redis caches again
```

Most Redis clients perform this fallback automatically.

---

## Important

Changing even one character in the Lua script produces a completely different SHA.

Redis treats it as a brand-new script.

---

# 8. Designing dequeue.lua

The dequeue operation is a business operation, not merely a Redis command.

Algorithm:

```
1. Remove highest priority task from Ready Queue.

2. If queue is empty:
      return nil

3. Calculate lease expiry timestamp.

4. Add task to Processing Queue.
      Score = leaseExpiry

5. Update task metadata.
      status = processing
      leaseExpiry = calculated value

6. Return Task ID.
```

Returning the Task ID allows the worker to fetch the task metadata separately.

Returning `nil` indicates there is no work available.

---

# 9. Why ACK Needs Lua

ACK is also a multi-step operation.

Algorithm:

```
1. Remove task from Processing Queue.

2. If task was not removed:
      return (no-op)

3. Mark task completed
   or delete task.

4. Return success.
```

Again, this should execute atomically.

---

# 10. ACK Race Condition

Scenario:

```
Lease expires

↓

Reaper reclaims task

↓

Worker sends ACK
```

ACK performs:

```
ZREM processing
```

If Redis returns:

```
1
```

Worker still owns the task.

Continue ACK.

---

If Redis returns:

```
0
```

Ownership has already been transferred.

ACK becomes a no-op.

Never update the task.

---

## Important

ACK should **not** check the Ready Queue.

Ownership is determined solely by membership in the Processing Queue.

---

# 11. Designing enqueue.lua

Algorithm:

```
1. Verify task does not already exist.

2. Create task metadata.

3. Insert Task ID into Ready Queue.

4. Return success.
```

Creating metadata before publishing the task ensures workers never observe a task without its metadata.

---

# 12. Idempotent Enqueue

Suppose:

```
Client

↓

Enqueue

↓

Redis succeeds

↓

HTTP response lost
```

Client retries.

Without duplicate detection:

```
Ready Queue

Task42

Task42
```

Task executes twice.

---

Instead:

```
EXISTS task:42

↓

Exists?

↓

Yes

↓

Reject request

↓

No

↓

Create task
```

Executing enqueue multiple times produces the same final state.

Therefore enqueue becomes idempotent.

---

# 13. Why EXISTS Instead of HSETNX

A task is a complete business entity.

```
task:42

status

payload

priority

retryCount
```

Checking a single field using HSETNX is insufficient.

The correct business question is:

```
Does task:42 already exist?
```

Therefore:

```
EXISTS task:42
```

is the correct approach.

---

# Queue Operations Designed So Far

## Enqueue

```
EXISTS

↓

HSET task metadata

↓

ZADD Ready Queue

↓

Return Success
```

---

## Dequeue

```
ZPOPMIN Ready Queue

↓

If empty

↓

Return nil

↓

ZADD Processing Queue

↓

Update metadata

↓

Return Task ID
```

---

## ACK

```
ZREM Processing Queue

↓

Removed?

↓

No

↓

No-op

↓

Yes

↓

Mark completed / Delete task

↓

Return Success
```

---

# Key Takeaways

- Lua exists to execute multiple Redis commands atomically.
- Atomicity prevents crash windows and command interleaving.
- Lua does not provide automatic rollback.
- EVAL executes Lua scripts.
- KEYS contains Redis keys.
- ARGV contains ordinary values.
- Scripts should always be written cluster-safe.
- redis.call() aborts on error.
- redis.pcall() allows manual error handling.
- EVALSHA executes cached scripts using their SHA-1 hash.
- Any change to the script creates a new SHA.
- Dequeue, Enqueue and ACK are business operations, not individual Redis commands.
- ACK ownership is determined only by the Processing Queue.
- ZREM returning 0 naturally resolves ACK vs Reaper races.
- Enqueue must be idempotent.
- EXISTS is the correct duplicate detection mechanism.

---

# Interview Summary

## Why use Lua instead of multiple Redis commands from Go?

> Lua executes multiple Redis commands atomically inside Redis. This eliminates both application crash windows between commands and command interleaving between concurrent workers, making multi-step queue operations behave as one logical operation.

---

## Why are KEYS and ARGV separated?

> KEYS contains every Redis key the script accesses, allowing Redis Cluster to route and validate scripts before execution. ARGV contains ordinary values such as timestamps, priorities and worker IDs.

---

## Why does Redis provide EVALSHA?

> Redis caches Lua scripts using the SHA-1 hash of their contents. EVALSHA executes the cached version, reducing network traffic and eliminating repeated parsing of identical scripts.

---

## How does ACK handle lease-expiry races?

> ACK first attempts to remove the task from the Processing Queue. If ZREM returns 0, ownership has already been transferred (typically by the reaper), so ACK becomes a no-op and must not modify the task.

---

# Revision Questions

1. Why is Lua needed even though Redis commands are atomic?
2. What problems does Lua solve besides application crashes?
3. Why doesn't Redis provide rollback for Lua scripts?
4. What is the difference between EVAL and EVALSHA?
5. Why does Redis separate KEYS and ARGV?
6. When should redis.call() be preferred over redis.pcall()?
7. Why should dequeue return a Task ID instead of only success?
8. Why should ACK become a no-op after lease expiry?
9. Why shouldn't ACK inspect the Ready Queue?
10. Why should metadata be created before publishing a task?
11. How does EXISTS make enqueue idempotent?
12. Why is HSETNX insufficient for duplicate detection? 

# Distributed Systems Notes – Phase 2 Day 1
## Heartbeats, Service Discovery & Worker Leases

---

# Goal

Understand how distributed workers register themselves, how the system detects node failures, and why heartbeats are implemented using Redis TTL keys instead of explicit "alive/dead" messages.

---

# 1. The Fundamental Problem

In Phase 1 we asked:

> "Did the worker complete the task?"

Now in Phase 2 we ask:

> "Is the worker still alive?"

The answer is exactly the same.

**We can never know with certainty.**

Reasons:

- Network delay
- Network partition
- Packet loss
- GC pause
- OS scheduling delay
- Temporary Redis overload

A missing heartbeat **does not prove** the worker has crashed.

This is the same distributed systems impossibility discussed in Phase 1.

---

# 2. Heartbeats

Instead of proving a worker is alive, every worker periodically announces:

```
"I'm alive"
```

For example:

```
Heartbeat every 3 seconds
```

The worker continuously refreshes its presence in Redis.

---

# 3. Service Discovery Using Redis TTL Keys

Each worker creates a Redis key:

```
taskqueue:node:<workerID>
```

Example:

```
SET taskqueue:node:worker-A online EX 10
```

Meaning:

- Create the key.
- Automatically expire it after 10 seconds.

Every heartbeat executes the same command again, refreshing the TTL.

Timeline:

```
Heartbeat

↓

TTL = 10

↓

9

↓

8

↓

Heartbeat

↓

TTL = 10

↓

9

↓

Heartbeat

↓

TTL = 10
```

As long as heartbeats continue, the key never expires.

---

# 4. Worker Crash

Suppose the worker crashes.

No more heartbeats arrive.

Redis automatically counts down:

```
10

↓

9

↓

...

↓

1

↓

0

↓

Key automatically deleted
```

Nobody manually removes the worker.

Redis handles expiration automatically.

These are called **ephemeral keys** because they only exist while someone keeps renewing them.

---

# 5. Worker Lease

A heartbeat is simply another lease.

Phase 1:

```
Worker renews Task Lease
```

Phase 2:

```
Worker renews Worker Lease
```

Both follow the exact same pattern.

When renewal stops:

- Task lease expires → Task reclaimed.
- Worker lease expires → Worker considered unavailable.

---

# 6. Lost Heartbeats

Example:

Heartbeat interval:

```
3 seconds
```

TTL:

```
10 seconds
```

Timeline:

```
0s

Heartbeat

↓

3s

Heartbeat

↓

6s

Heartbeat lost

↓

9s

Heartbeat arrives
```

The worker is **not** considered dead.

Why?

Because Redis deletes the key only after the TTL reaches zero.

A single missed heartbeat is completely acceptable.

---

# 7. Heartbeat Interval vs TTL

Suppose:

```
Heartbeat = 3 seconds

TTL = 10 seconds
```

This allows the system to tolerate:

- One missed heartbeat
- Temporary network delay
- Minor GC pauses
- Scheduler delays

The worker only needs **one successful heartbeat before the TTL expires**.

---

# 8. False Positives

Suppose instead:

```
Heartbeat = 1 second

TTL = 2 seconds
```

Now a small GC pause delays the heartbeat.

Timeline:

```
0s

Heartbeat

↓

2s

TTL expires

↓

Redis deletes worker key

↓

2.2s

Heartbeat finally arrives
```

The worker never crashed.

But the system temporarily believed it was dead.

This is called a **false positive**.

---

# 9. Consequences of False Positives

If the system incorrectly concludes a worker died:

```
Worker executing task

↓

Heartbeat expires

↓

Reaper reclaims task

↓

Another worker executes task

↓

Original worker finishes later
```

Result:

```
Duplicate task execution
```

This is exactly the same trade-off as Phase 1's lease expiry.

---

# 10. Design Philosophy

Distributed systems should **tolerate temporary failures**, not react to every delay.

Good systems assume:

- Packets can be delayed.
- Networks can be slow.
- GC pauses happen.
- Scheduling delays happen.

The goal is not:

```
Fastest failure detection
```

The goal is:

```
Fast failure detection

while minimizing

False positives
```

---

# 11. Why TTL > Heartbeat

Example:

```
Heartbeat = 3s

TTL = 10s
```

The larger TTL provides slack.

This allows:

- Missed heartbeat
- Temporary overload
- Network jitter

without incorrectly declaring the worker dead.

Rule of thumb:

```
TTL > 3 × Heartbeat Interval
```

Not because it is mathematically required, but because it provides a practical safety margin.

---

# Key Takeaways

- Missing a heartbeat does not prove a worker crashed.
- Heartbeats are periodic "I'm alive" signals.
- Redis TTL keys provide automatic service discovery.
- Worker registration is temporary and automatically expires.
- Heartbeats are simply leases applied to workers instead of tasks.
- The heartbeat interval and TTL must balance fast recovery with tolerance for normal delays.
- False positives can cause duplicate task execution.
- Distributed systems should optimize for **fast enough detection with low false positives**, not the fastest possible detection.

---

# Interview Summary

## Why can't the server know whether a worker has crashed?

> Because distributed systems cannot distinguish between a crashed worker and a slow worker. Missing heartbeats may be caused by network delay, GC pauses, scheduler delays, or temporary overload.

---

## Why use Redis TTL keys for service discovery?

> Workers periodically refresh a Redis key with a TTL. If the worker stops sending heartbeats, Redis automatically expires the key. This provides simple, lightweight service discovery without requiring an external coordination service.

---

## Why not use Heartbeat = 1s and TTL = 2s?

> Although failure detection becomes faster, the system becomes extremely sensitive to normal GC pauses, network jitter and scheduling delays, producing false positives. These false positives can trigger unnecessary task reclamation and duplicate execution.

---

## Biggest Mental Model

```
Phase 1

Task Lease

↓

Worker renews lease

↓

Lease expires

↓

Task reclaimed
```

```
Phase 2

Worker Lease (Heartbeat)

↓

Worker renews heartbeat

↓

TTL expires

↓

Worker considered unavailable
```

Both are the same distributed systems pattern applied to different entities.

---

# Revision Questions

1. Why can't missing heartbeats prove a worker has crashed?
2. What problem do Redis TTL keys solve?
3. Why are heartbeat keys called ephemeral?
4. How is a heartbeat similar to a lease?
5. Why is a TTL much larger than the heartbeat interval?
6. What is a false positive in node failure detection?
7. How can a false positive lead to duplicate task execution?
8. Why is "fastest failure detection" usually not the right goal?
9. Why is TTL > 3× heartbeat a common rule of thumb?
10. How are worker leases and task leases conceptually identical? 

# Distributed Systems Notes – Phase 2 Day 2
## Eager Node-Death Reclaim & Worker Ownership

---

# Goal

Understand why Phase 1 recovery is considered **lazy**, why Phase 2 introduces **eager recovery**, and why the system must maintain **worker → task ownership** information.

---

# 1. Phase 1 Recovery

In Phase 1:

```
Worker dequeues task

↓

Task enters Processing Queue

↓

Worker crashes

↓

System waits for lease expiry

↓

Reaper reclaims task
```

Example:

```
Lease = 30 seconds

Worker crashes after 1 second

↓

Task recovered after 29 more seconds
```

This approach is called **lazy recovery** because the system waits for each individual task lease to expire before taking action.

---

# 2. Drawback of Lazy Recovery

Suppose:

```
Worker crashes after 1 second.
```

The system already knows the worker is no longer available (heartbeat expired later), but it still waits for every task's lease to expire.

Result:

```
Slow recovery

↓

Tasks remain unavailable

↓

Workers stay idle
```

The recovery speed depends entirely on the lease timeout.

---

# 3. Phase 2 Improvement

Phase 2 introduces **heartbeats**.

Timeline:

```
Lease = 30s

Heartbeat TTL = 10s

0s

Worker dequeues task

↓

1s

Worker crashes

↓

11s

Heartbeat expires

↓

System now knows the worker is unavailable
```

At this point, waiting another 20 seconds for the task lease makes little sense.

---

# 4. Eager Node-Death Reclaim

Instead of waiting for every lease to expire:

```
Heartbeat expires

↓

Immediately reclaim ALL tasks owned by that worker

↓

Move them back to Ready Queue
```

This is called:

```
Eager Node-Death Reclaim
```

Recovery becomes much faster.

---

# 5. New Problem

Suppose Worker A owns:

```
Task1

Task5

Task42

Task99
```

Heartbeat expires.

Question:

```
How does the reaper know which tasks belonged to Worker A?
```

Phase 1 only knew:

```
Processing Queue

↓

Task IDs
```

It did **not** know which worker owned each task.

---

# 6. Option A – Store workerID inside every task

Example:

```
task:42

status = processing

workerID = worker-A

leaseExpiry = ...
```

When Worker A dies:

```
Scan entire Processing Queue

↓

Read every task

↓

Find workerID == worker-A
```

Advantages:

- Simple

Disadvantages:

- Requires scanning every processing task.
- Does not scale well.

Time Complexity:

```
O(number of processing tasks)
```

---

# 7. Option B – Maintain Worker Ownership Sets

Maintain an index:

```
worker:worker-A

↓

Task1

Task5

Task42

Task99
```

When Worker A dies:

```
Lookup worker:worker-A

↓

Immediately obtain all owned tasks

↓

Reclaim only those tasks
```

Advantages:

- Fast recovery.
- No global scan.
- Scales to millions of tasks.

Time Complexity:

```
O(number of tasks owned by that worker)
```

This is much better than scanning the complete Processing Queue.

---

# 8. Worker Ownership is an Index

The ownership set is conceptually identical to a database index.

Without an index:

```
Need every task owned by Worker A

↓

Scan entire dataset
```

With an index:

```
Worker A

↓

Owned Tasks
```

Fast lookup.

---

# 9. Phase 2 Changes Dequeue

Phase 1:

```
Ready Queue

↓

Processing Queue

↓

Task Hash
```

Phase 2:

```
Ready Queue

↓

Processing Queue

↓

Task Hash

↓

Worker Ownership Set
```

When a task is dequeued:

- Add to Processing Queue.
- Update task metadata.
- Add task to the worker's ownership set.

All inside the same Lua script.

---

# 10. ACK Changes in Phase 2

Phase 1:

```
Processing Queue

↓

Completed
```

Phase 2:

```
Processing Queue

↓

Remove from Worker Ownership Set

↓

Completed
```

Why?

If the ownership set is not updated, the reaper may later incorrectly reclaim an already completed task.

---

# 11. NACK Changes in Phase 2

Phase 1:

```
Processing Queue

↓

Delayed Queue
```

Phase 2:

```
Processing Queue

↓

Remove from Worker Ownership Set

↓

Delayed Queue
```

Reason:

Once the task leaves execution, the worker no longer owns it.

When it returns from the Delayed Queue, any worker should be able to process it.

---

# 12. Lease Expiry Reclaim

When the reaper reclaims an expired lease:

```
Processing Queue

↓

Remove from Worker Ownership Set

↓

Ready Queue
```

Ownership must be removed because the original worker no longer owns the task.

---

# 13. Eager Node-Death Reclaim

When a heartbeat expires:

```
Worker A

↓

Ownership Set

↓

Task1

Task5

Task42
```

The reaper:

- Reads the ownership set.
- Removes each task from Processing.
- Removes each task from the ownership set.
- Returns each task to the Ready Queue.

This avoids waiting for each lease individually.

---

# 14. Ownership Rule

A worker owns a task **only while actively executing it.**

Ownership ends when:

- ACK
- NACK
- Lease expiry reclaim
- Node-death reclaim

Every path that ends execution must remove the task from the ownership set.

---

# 15. System Invariant

The ownership set must always satisfy:

> **A task exists in a worker's ownership set if and only if that worker is currently executing the task.**

This invariant must hold after every queue operation.

---

# Key Takeaways

- Phase 1 uses lazy recovery based on task lease expiry.
- Phase 2 introduces eager recovery using worker heartbeats.
- Heartbeats provide additional information about worker availability.
- Fast recovery requires tracking worker → task ownership.
- A dedicated ownership index scales much better than scanning the entire Processing Queue.
- Every queue operation must keep the ownership index consistent.
- Ownership exists only while a worker is actively executing a task.
- Good distributed systems maintain invariants across all state transitions.

---

# Interview Summary

## Why is Phase 1 recovery considered lazy?

> Phase 1 waits for each task's lease to expire individually before reclaiming it. Even if the worker has already become unavailable, recovery is delayed until the lease timeout.

---

## What is eager node-death reclaim?

> When a worker's heartbeat expires, the system immediately reclaims all tasks owned by that worker instead of waiting for each task lease to expire.

---

## Why maintain a worker ownership set?

> Without ownership tracking, the reaper must scan the entire Processing Queue to determine which tasks belong to a failed worker. Maintaining a per-worker ownership set provides an efficient index, making recovery proportional to the failed worker's tasks rather than all processing tasks.

---

## Why must ACK and NACK update the ownership set?

> Ownership only exists while a task is actively executing. Once execution ends—whether by success or failure—the task must be removed from the worker's ownership set to prevent stale ownership and incorrect future recovery.

---

# Biggest Mental Model

```
Processing Queue

↓

Who owns each task?

↓

Worker Ownership Set

↓

Fast node recovery
```

Ownership tracking is not added for information.

It is added to make failure recovery scalable.

---

# Revision Questions

1. Why is Phase 1 recovery called lazy?
2. What new information do heartbeats provide that Phase 1 lacked?
3. What is eager node-death reclaim?
4. Why can't the reaper efficiently recover tasks without ownership tracking?
5. Compare storing `workerID` in each task vs maintaining a per-worker ownership set.
6. Why is the ownership set conceptually similar to a database index?
7. Which Lua script must be modified to populate the ownership set?
8. Why must ACK remove tasks from the ownership set?
9. Why must NACK remove tasks from the ownership set?
10. State the ownership invariant introduced in Phase 2.
11. Why does a per-worker ownership set scale better than scanning the entire Processing Queue?
12. In which four situations does task ownership end?