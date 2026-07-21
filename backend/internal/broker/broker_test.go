package broker_test

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/broker"
	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/model"
	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/queue"
	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/store"
)

func setup(t *testing.T) (*redis.Client, *broker.RedisBroker, *store.TaskStore) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { client.Close() })

	taskStore := store.NewTaskStore(client)
	pq := queue.NewPriorityQueue(client, taskStore)
	delayed := queue.NewDelayedScheduler(client, pq, taskStore, nil)
	b := broker.NewRedisBroker(client, taskStore, pq, delayed, 30*time.Second)

	return client, b, taskStore
}

func makeTask(id string, priority, maxRetries int) model.Task {
	return model.Task{
		ID:         id,
		Type:       "test",
		Priority:   priority,
		MaxRetries: maxRetries,
		Status:     model.StatusPending,
		CreatedAt:  time.Now(),
	}
}

func TestDequeue_MovesToProcessingSet(t *testing.T) {
	client, b, _ := setup(t)
	ctx := context.Background()

	task := makeTask("task-1", 5, 3)
	if err := b.Enqueue(ctx, task); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	// Dequeue should move task from ready to processing
	got, err := b.Dequeue(ctx)
	if err != nil {
		t.Fatalf("dequeue: %v", err)
	}
	if got == nil {
		t.Fatal("dequeue returned nil")
	}
	if got.ID != "task-1" {
		t.Fatalf("expected task-1, got %s", got.ID)
	}

	// Ready set should be empty
	readySize, _ := client.ZCard(ctx, store.KeyReady).Result()
	if readySize != 0 {
		t.Fatalf("expected ready=0, got %d", readySize)
	}

	// Processing set should have the task
	procSize, _ := client.ZCard(ctx, store.KeyProcessing).Result()
	if procSize != 1 {
		t.Fatalf("expected processing=1, got %d", procSize)
	}

	// Verify the score is a future deadline (within 30s + some tolerance)
	score, _ := client.ZScore(ctx, store.KeyProcessing, "task-1").Result()
	nowMs := float64(time.Now().UnixMilli())
	if score < nowMs || score > nowMs+31000 {
		t.Fatalf("expected deadline within 30s from now, got score=%f, now=%f", score, nowMs)
	}

	// Task record should show status=processing
	record, _ := client.HGet(ctx, store.TaskKey("task-1"), "status").Result()
	if record != "processing" {
		t.Fatalf("expected status=processing, got %s", record)
	}
}

func TestDequeue_EmptyQueue_ReturnsNil(t *testing.T) {
	_, b, _ := setup(t)
	ctx := context.Background()

	got, err := b.Dequeue(ctx)
	if err != nil {
		t.Fatalf("dequeue: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil on empty queue, got %+v", got)
	}
}

func TestDequeue_PriorityOrder(t *testing.T) {
	_, b, _ := setup(t)
	ctx := context.Background()

	// Enqueue low priority first, then high priority
	b.Enqueue(ctx, makeTask("low", 1, 0))
	b.Enqueue(ctx, makeTask("high", 10, 0))

	// Should dequeue high priority first (score = -priority, lowest score pops first)
	got, _ := b.Dequeue(ctx)
	if got.ID != "high" {
		t.Fatalf("expected high-priority task first, got %s", got.ID)
	}
	got, _ = b.Dequeue(ctx)
	if got.ID != "low" {
		t.Fatalf("expected low-priority task second, got %s", got.ID)
	}
}

func TestAck_Success(t *testing.T) {
	client, b, _ := setup(t)
	ctx := context.Background()

	b.Enqueue(ctx, makeTask("task-1", 5, 3))
	b.Dequeue(ctx) // moves to processing

	err := b.Ack(ctx, "task-1")
	if err != nil {
		t.Fatalf("ack: %v", err)
	}

	// Processing set should be empty
	procSize, _ := client.ZCard(ctx, store.KeyProcessing).Result()
	if procSize != 0 {
		t.Fatalf("expected processing=0 after ack, got %d", procSize)
	}

	// Task record should show status=completed
	status, _ := client.HGet(ctx, store.TaskKey("task-1"), "status").Result()
	if status != "completed" {
		t.Fatalf("expected status=completed after ack, got %s", status)
	}
}

func TestAck_LeaseNotHeld(t *testing.T) {
	_, b, _ := setup(t)
	ctx := context.Background()

	// Ack a task that was never leased
	err := b.Ack(ctx, "nonexistent")
	if err != broker.ErrLeaseNotHeld {
		t.Fatalf("expected ErrLeaseNotHeld, got %v", err)
	}
}

func TestAck_DoubleAck_SecondFails(t *testing.T) {
	_, b, _ := setup(t)
	ctx := context.Background()

	b.Enqueue(ctx, makeTask("task-1", 5, 3))
	b.Dequeue(ctx)

	// First ack succeeds
	if err := b.Ack(ctx, "task-1"); err != nil {
		t.Fatalf("first ack: %v", err)
	}
	// Second ack should fail (lease already released)
	if err := b.Ack(ctx, "task-1"); err != broker.ErrLeaseNotHeld {
		t.Fatalf("expected ErrLeaseNotHeld on double ack, got %v", err)
	}
}

func TestNack_Success(t *testing.T) {
	client, b, _ := setup(t)
	ctx := context.Background()

	b.Enqueue(ctx, makeTask("task-1", 5, 3))
	b.Dequeue(ctx)

	err := b.Nack(ctx, "task-1", true)
	if err != nil {
		t.Fatalf("nack: %v", err)
	}

	// Processing set should be empty
	procSize, _ := client.ZCard(ctx, store.KeyProcessing).Result()
	if procSize != 0 {
		t.Fatalf("expected processing=0 after nack, got %d", procSize)
	}
}

func TestNack_LeaseNotHeld(t *testing.T) {
	_, b, _ := setup(t)
	ctx := context.Background()

	err := b.Nack(ctx, "nonexistent", true)
	if err != broker.ErrLeaseNotHeld {
		t.Fatalf("expected ErrLeaseNotHeld, got %v", err)
	}
}

func TestExtendLease_Success(t *testing.T) {
	client, b, _ := setup(t)
	ctx := context.Background()

	b.Enqueue(ctx, makeTask("task-1", 5, 3))
	b.Dequeue(ctx) // lease with 30s timeout

	// Extend by 60s
	err := b.ExtendLease(ctx, "task-1", 60*time.Second)
	if err != nil {
		t.Fatalf("extend lease: %v", err)
	}

	// Score should be updated to ~60s from now
	score, _ := client.ZScore(ctx, store.KeyProcessing, "task-1").Result()
	expected := float64(time.Now().Add(60 * time.Second).UnixMilli())
	// Allow 1s tolerance
	if score < expected-1000 || score > expected+1000 {
		t.Fatalf("expected deadline ~60s from now, got score=%f, expected~%f", score, expected)
	}
}

func TestExtendLease_NotHeld(t *testing.T) {
	_, b, _ := setup(t)
	ctx := context.Background()

	err := b.ExtendLease(ctx, "nonexistent", 30*time.Second)
	if err != broker.ErrLeaseNotHeld {
		t.Fatalf("expected ErrLeaseNotHeld, got %v", err)
	}
}
