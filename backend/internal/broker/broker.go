// Package broker defines the queue abstraction that workers and the API depend
// on, decoupling them from the concrete Redis implementation. This is the seam
// (P0.2) that Phase 1 grows into leased, at-least-once delivery.
package broker

import (
	"context"
	"errors"
	"time"

	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/model"
	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/queue"
	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/store"
)

// ErrNotImplemented marks broker operations whose real semantics arrive in
// Phase 1 (leased delivery). They are declared now so the interface is stable.
var ErrNotImplemented = errors.New("broker: not implemented until Phase 1")

// Broker is the delivery abstraction. Enqueue/Dequeue are live today; Ack, Nack
// and ExtendLease become meaningful in Phase 1 when Dequeue starts leasing tasks
// into a processing set instead of removing them outright.
type Broker interface {
	// Enqueue persists the canonical record and makes the task ready to run.
	Enqueue(ctx context.Context, task model.Task) error
	// Dequeue returns the next task to run, or (nil, nil) if none are ready.
	Dequeue(ctx context.Context) (*model.Task, error)
	// Ack marks a leased task as successfully completed (P1.3).
	Ack(ctx context.Context, taskID string) error
	// Nack returns a leased task to retry (delayed) or the DLQ (P1.3).
	Nack(ctx context.Context, taskID string, requeue bool) error
	// ExtendLease pushes back a leased task's visibility deadline (P1).
	ExtendLease(ctx context.Context, taskID string, extension time.Duration) error
}

// RedisBroker is the Redis-backed Broker. It composes the existing ready queue,
// delayed scheduler and canonical task store built in Phase 0.
type RedisBroker struct {
	tasks   *store.TaskStore
	ready   *queue.PriorityQueue
	delayed *queue.DelayedScheduler
}

func NewRedisBroker(tasks *store.TaskStore, ready *queue.PriorityQueue, delayed *queue.DelayedScheduler) *RedisBroker {
	return &RedisBroker{tasks: tasks, ready: ready, delayed: delayed}
}

// compile-time assertion that RedisBroker satisfies Broker.
var _ Broker = (*RedisBroker)(nil)

func (b *RedisBroker) Enqueue(ctx context.Context, task model.Task) error {
	if err := b.tasks.Save(ctx, task); err != nil {
		return err
	}
	return b.ready.Enqueue(ctx, task)
}

func (b *RedisBroker) Dequeue(ctx context.Context) (*model.Task, error) {
	return b.ready.Dequeue(ctx)
}

// Ack is a no-op until Phase 1 introduces the processing set: with today's
// ZPOPMIN dequeue the task is already removed from the ready queue.
func (b *RedisBroker) Ack(ctx context.Context, taskID string) error {
	return nil
}

func (b *RedisBroker) Nack(ctx context.Context, taskID string, requeue bool) error {
	return ErrNotImplemented
}

func (b *RedisBroker) ExtendLease(ctx context.Context, taskID string, extension time.Duration) error {
	return ErrNotImplemented
}
