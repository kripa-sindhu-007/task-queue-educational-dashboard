package queue

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/model"
	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/store"
)

func newTestDeps(t *testing.T) (*store.TaskStore, *PriorityQueue) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	tasks := store.NewTaskStore(client)
	return tasks, NewPriorityQueue(client, tasks, DefaultSignalCap)
}

// enqueue persists the record (as the broker/handler would) then enqueues the ID.
func enqueue(t *testing.T, tasks *store.TaskStore, q *PriorityQueue, task model.Task) {
	t.Helper()
	ctx := context.Background()
	if err := tasks.Save(ctx, task); err != nil {
		t.Fatalf("save: %v", err)
	}
	if err := q.Enqueue(ctx, task); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
}

func TestEnqueueDequeueByID(t *testing.T) {
	ctx := context.Background()
	tasks, q := newTestDeps(t)

	enqueue(t, tasks, q, model.Task{ID: "task-1", Type: "sleep", Priority: 5, Status: model.StatusPending})

	got, err := q.Dequeue(ctx)
	if err != nil {
		t.Fatalf("dequeue: %v", err)
	}
	if got == nil || got.ID != "task-1" || got.Type != "sleep" || got.Priority != 5 {
		t.Fatalf("hydrated task mismatch: %+v", got)
	}

	// Queue is now empty.
	empty, err := q.Dequeue(ctx)
	if err != nil {
		t.Fatalf("dequeue empty: %v", err)
	}
	if empty != nil {
		t.Fatalf("expected nil on empty queue, got %+v", empty)
	}
}

// TestIdenticalFieldsAreDistinctEntries is the Phase 0 acceptance criterion:
// two tasks with identical fields (but distinct IDs) must create two queue
// entries, not silently dedup as the old full-JSON-member design did.
func TestIdenticalFieldsAreDistinctEntries(t *testing.T) {
	ctx := context.Background()
	tasks, q := newTestDeps(t)

	base := model.Task{Type: "sleep", Priority: 3, MaxRetries: 1, Status: model.StatusPending}
	a := base
	a.ID = "task-a"
	b := base
	b.ID = "task-b"

	enqueue(t, tasks, q, a)
	enqueue(t, tasks, q, b)

	size, err := q.Size(ctx)
	if err != nil {
		t.Fatalf("size: %v", err)
	}
	if size != 2 {
		t.Fatalf("expected 2 distinct ready entries, got %d", size)
	}
}

func TestDequeueRespectsPriority(t *testing.T) {
	ctx := context.Background()
	tasks, q := newTestDeps(t)

	enqueue(t, tasks, q, model.Task{ID: "low", Priority: 1, Status: model.StatusPending})
	enqueue(t, tasks, q, model.Task{ID: "high", Priority: 9, Status: model.StatusPending})

	got, err := q.Dequeue(ctx)
	if err != nil {
		t.Fatalf("dequeue: %v", err)
	}
	if got == nil || got.ID != "high" {
		t.Fatalf("expected highest-priority 'high' first, got %+v", got)
	}
}
