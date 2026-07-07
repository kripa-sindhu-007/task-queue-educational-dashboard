package queue

import (
	"context"
	"testing"
	"time"

	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/model"
	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/store"
)

func TestPromoteDueTasks(t *testing.T) {
	ctx := context.Background()
	tasks, q := newTestDeps(t)
	// Reuse the same client as the queue for the delayed scheduler.
	sched := NewDelayedScheduler(q.client, q, tasks, nil)

	due := model.Task{ID: "due", Priority: 2, Status: model.StatusPending}
	future := model.Task{ID: "future", Priority: 2, Status: model.StatusPending}
	if err := tasks.Save(ctx, due); err != nil {
		t.Fatalf("save due: %v", err)
	}
	if err := tasks.Save(ctx, future); err != nil {
		t.Fatalf("save future: %v", err)
	}

	// One task is already due (negative delay), one is far in the future.
	if err := sched.Schedule(ctx, due, -2*time.Second); err != nil {
		t.Fatalf("schedule due: %v", err)
	}
	if err := sched.Schedule(ctx, future, time.Hour); err != nil {
		t.Fatalf("schedule future: %v", err)
	}

	sched.promoteDueTasks(ctx)

	readySize, err := q.Size(ctx)
	if err != nil {
		t.Fatalf("ready size: %v", err)
	}
	if readySize != 1 {
		t.Fatalf("expected 1 promoted task in ready, got %d", readySize)
	}
	delayedSize, err := q.client.ZCard(ctx, store.KeyDelayed).Result()
	if err != nil {
		t.Fatalf("delayed size: %v", err)
	}
	if delayedSize != 1 {
		t.Fatalf("expected 1 task still delayed, got %d", delayedSize)
	}

	got, err := q.Dequeue(ctx)
	if err != nil {
		t.Fatalf("dequeue: %v", err)
	}
	if got == nil || got.ID != "due" {
		t.Fatalf("expected 'due' promoted, got %+v", got)
	}
}
