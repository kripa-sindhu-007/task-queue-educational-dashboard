// Package broker defines the queue abstraction that workers and the API depend
// on, decoupling them from the concrete Redis implementation. Phase 1 introduces
// lease-based at-least-once delivery: Dequeue atomically moves a task from the
// ready set into a processing set with a visibility timeout (lease deadline).
package broker

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/model"
	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/queue"
	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/store"
)

//go:embed scripts/dequeue.lua
var dequeueScript string

//go:embed scripts/ack.lua
var ackScript string

//go:embed scripts/nack.lua
var nackScript string

//go:embed scripts/extend.lua
var extendScript string

// ErrLeaseNotHeld is returned when Ack/Nack/ExtendLease is called for a task
// that is not in the processing set (already acked, reclaimed, or never leased).
var ErrLeaseNotHeld = errors.New("broker: lease not held (task not in processing)")

// Broker is the delivery abstraction. Dequeue leases tasks into a processing
// set; Ack/Nack release the lease; the reaper reclaims expired leases.
type Broker interface {
	// Enqueue persists the canonical record and makes the task ready to run.
	Enqueue(ctx context.Context, task model.Task) error
	// Dequeue atomically pops from ready and leases into processing.
	// Returns (nil, nil) if no tasks are ready.
	Dequeue(ctx context.Context) (*model.Task, error)
	// Ack marks a leased task as successfully completed, removing it from processing.
	Ack(ctx context.Context, taskID string) error
	// Nack releases the lease. If requeue is true, the task goes back to retry
	// (delayed with backoff) or the DLQ if retries are exhausted.
	Nack(ctx context.Context, taskID string, requeue bool) error
	// ExtendLease pushes back a leased task's visibility deadline.
	ExtendLease(ctx context.Context, taskID string, extension time.Duration) error
}

// RedisBroker is the Redis-backed Broker with lease-based delivery. Each broker
// instance belongs to one node (worker process, or the server in in-process
// mode); its nodeID is stamped on every task it leases so the reaper can
// reclaim this node's work if it dies, and so Ack/Nack can fence against a lost
// lease that was re-leased elsewhere.
type RedisBroker struct {
	client            *redis.Client
	tasks             *store.TaskStore
	ready             *queue.PriorityQueue
	delayed           *queue.DelayedScheduler
	visibilityTimeout time.Duration
	nodeID            string
	dequeueCmd        *redis.Script
	ackCmd            *redis.Script
	nackCmd           *redis.Script
	extendCmd         *redis.Script
}

func NewRedisBroker(
	client *redis.Client,
	tasks *store.TaskStore,
	ready *queue.PriorityQueue,
	delayed *queue.DelayedScheduler,
	visibilityTimeout time.Duration,
	nodeID string,
) *RedisBroker {
	return &RedisBroker{
		client:            client,
		tasks:             tasks,
		ready:             ready,
		delayed:           delayed,
		visibilityTimeout: visibilityTimeout,
		nodeID:            nodeID,
		dequeueCmd:        redis.NewScript(dequeueScript),
		ackCmd:            redis.NewScript(ackScript),
		nackCmd:           redis.NewScript(nackScript),
		extendCmd:         redis.NewScript(extendScript),
	}
}

// compile-time assertion that RedisBroker satisfies Broker.
var _ Broker = (*RedisBroker)(nil)

func (b *RedisBroker) Enqueue(ctx context.Context, task model.Task) error {
	if err := b.tasks.Save(ctx, task); err != nil {
		return err
	}
	return b.ready.Enqueue(ctx, task)
}

// Dequeue atomically pops the highest-priority task ID from the ready set and
// inserts it into the processing set with a lease deadline. The Lua script
// ensures no interleaving — if the process crashes between pop and insert, the
// task is never lost.
func (b *RedisBroker) Dequeue(ctx context.Context) (*model.Task, error) {
	deadline := time.Now().Add(b.visibilityTimeout).UnixMilli()

	result, err := b.dequeueCmd.Run(ctx, b.client,
		[]string{store.KeyReady, store.KeyProcessing, store.KeyTaskPrefix, store.NodeTasksKey(b.nodeID)},
		deadline, b.nodeID,
	).Result()

	if err == redis.Nil || result == nil {
		return nil, nil // queue empty
	}
	if err != nil {
		return nil, fmt.Errorf("dequeue lua: %w", err)
	}

	taskID, ok := result.(string)
	if !ok {
		return nil, fmt.Errorf("dequeue lua: unexpected result type %T", result)
	}

	// Hydrate the full task record.
	task, err := b.tasks.Get(ctx, taskID)
	if err != nil {
		if err == store.ErrTaskNotFound {
			// Record vanished (flushed) — remove the orphan from processing.
			b.client.ZRem(ctx, store.KeyProcessing, taskID)
			return nil, nil
		}
		return nil, fmt.Errorf("hydrate task %s: %w", taskID, err)
	}
	return &task, nil
}

// Ack atomically removes a task from the processing set and marks it completed
// in the canonical record. The Lua script's ZREM return value is the concurrency
// guard: if the reaper already reclaimed the task, ZREM returns 0 and we surface
// ErrLeaseNotHeld — the executor should treat this as a no-op (the task will be
// re-delivered, which is expected under at-least-once semantics).
func (b *RedisBroker) Ack(ctx context.Context, taskID string) error {
	result, err := b.ackCmd.Run(ctx, b.client,
		[]string{store.KeyProcessing, store.TaskKey(taskID), store.NodeTasksKey(b.nodeID)},
		taskID, b.nodeID,
	).Int()
	if err != nil {
		return fmt.Errorf("ack task %s: %w", taskID, err)
	}
	if result == 0 {
		return ErrLeaseNotHeld
	}
	return nil
}

// Nack atomically removes a task from the processing set (releasing the lease).
// The caller (executor) is responsible for routing the task to retry/DLQ after
// Nack confirms the lease was released. If the reaper already reclaimed it,
// ErrLeaseNotHeld is returned — the executor should skip retry routing because
// the reaper already handled redelivery.
func (b *RedisBroker) Nack(ctx context.Context, taskID string, requeue bool) error {
	result, err := b.nackCmd.Run(ctx, b.client,
		[]string{store.KeyProcessing, store.TaskKey(taskID), store.NodeTasksKey(b.nodeID)},
		taskID, b.nodeID,
	).Int()
	if err != nil {
		return fmt.Errorf("nack task %s: %w", taskID, err)
	}
	if result == 0 {
		return ErrLeaseNotHeld
	}
	return nil
}

// ExtendLease pushes back a leased task's visibility deadline via a single
// atomic Lua script (existence check + score update in one round trip). Useful
// for long-running tasks that need more time than the default visibility timeout.
func (b *RedisBroker) ExtendLease(ctx context.Context, taskID string, extension time.Duration) error {
	newDeadline := time.Now().Add(extension).UnixMilli()

	result, err := b.extendCmd.Run(ctx, b.client,
		[]string{store.KeyProcessing},
		taskID, newDeadline,
	).Int()
	if err != nil {
		return fmt.Errorf("extend lease %s: %w", taskID, err)
	}
	if result == 0 {
		return ErrLeaseNotHeld
	}
	return nil
}
