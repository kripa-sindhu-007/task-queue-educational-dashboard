package reaper_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/model"
	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/reaper"
	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/store"
)

func setup(t *testing.T) (*redis.Client, *store.TaskStore, *store.DeadLetterStore, *store.EventStore, *store.MetricsStore) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { client.Close() })

	tasks := store.NewTaskStore(client)
	dl := store.NewDeadLetterStore(client)
	events := store.NewEventStore(client)
	metrics := store.NewMetricsStore(client)
	return client, tasks, dl, events, metrics
}

// simulateLeasedTask saves a task record and puts its ID in the processing ZSET
// with the given deadline.
func simulateLeasedTask(ctx context.Context, t *testing.T, client *redis.Client, tasks *store.TaskStore, task model.Task, deadlineMs int64) {
	t.Helper()
	if err := tasks.Save(ctx, task); err != nil {
		t.Fatalf("save task: %v", err)
	}
	client.ZAdd(ctx, store.KeyProcessing, redis.Z{
		Score:  float64(deadlineMs),
		Member: task.ID,
	})
}

func TestReaper_ReclaimsExpiredLease(t *testing.T) {
	client, tasks, dl, events, metrics := setup(t)
	ctx := context.Background()

	task := model.Task{
		ID:         "task-expired",
		Type:       "test",
		Priority:   5,
		MaxRetries: 3,
		Retries:    0,
		Status:     model.StatusProcessing,
		CreatedAt:  time.Now(),
	}

	// Put task in processing with a deadline in the past (expired 10s ago)
	expiredDeadline := time.Now().Add(-10 * time.Second).UnixMilli()
	simulateLeasedTask(ctx, t, client, tasks, task, expiredDeadline)

	// Run the reaper with a short interval — use Start in a goroutine and cancel quickly
	r := reaper.New(client, tasks, dl, events, metrics, store.NewNodeStore(client, 10*time.Second), reaper.Config{
		Interval:  50 * time.Millisecond,
		BatchSize: 10,
	})

	reaperCtx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
	defer cancel()
	r.Start(reaperCtx)

	// After reaper runs: task should be back in ready with retries incremented
	readySize, _ := client.ZCard(ctx, store.KeyReady).Result()
	if readySize != 1 {
		t.Fatalf("expected ready=1 after reclaim, got %d", readySize)
	}

	// Processing should be empty
	procSize, _ := client.ZCard(ctx, store.KeyProcessing).Result()
	if procSize != 0 {
		t.Fatalf("expected processing=0, got %d", procSize)
	}

	// Task record should show retries=1 and status=pending
	retries, _ := client.HGet(ctx, store.TaskKey("task-expired"), "retries").Result()
	if retries != "1" {
		t.Fatalf("expected retries=1, got %s", retries)
	}
	status, _ := client.HGet(ctx, store.TaskKey("task-expired"), "status").Result()
	if status != "pending" {
		t.Fatalf("expected status=pending, got %s", status)
	}
}

func TestReaper_DeadLettersWhenRetriesExhausted(t *testing.T) {
	client, tasks, dl, events, metrics := setup(t)
	ctx := context.Background()

	task := model.Task{
		ID:         "task-exhausted",
		Type:       "test",
		Priority:   3,
		MaxRetries: 2,
		Retries:    2, // already at max
		Status:     model.StatusProcessing,
		CreatedAt:  time.Now(),
	}

	expiredDeadline := time.Now().Add(-5 * time.Second).UnixMilli()
	simulateLeasedTask(ctx, t, client, tasks, task, expiredDeadline)

	r := reaper.New(client, tasks, dl, events, metrics, store.NewNodeStore(client, 10*time.Second), reaper.Config{
		Interval:  50 * time.Millisecond,
		BatchSize: 10,
	})

	reaperCtx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
	defer cancel()
	r.Start(reaperCtx)

	// Task should NOT be in ready (retries exhausted)
	readySize, _ := client.ZCard(ctx, store.KeyReady).Result()
	if readySize != 0 {
		t.Fatalf("expected ready=0 (retries exhausted), got %d", readySize)
	}

	// Processing should be empty
	procSize, _ := client.ZCard(ctx, store.KeyProcessing).Result()
	if procSize != 0 {
		t.Fatalf("expected processing=0, got %d", procSize)
	}

	// Task record should show status=failed
	status, _ := client.HGet(ctx, store.TaskKey("task-exhausted"), "status").Result()
	if status != "failed" {
		t.Fatalf("expected status=failed, got %s", status)
	}

	// Should be in dead-letter list
	dlSize, _ := client.LLen(ctx, store.KeyDeadLetter).Result()
	if dlSize != 1 {
		t.Fatalf("expected DLQ size=1, got %d", dlSize)
	}
}

func TestReaper_IgnoresNonExpiredLeases(t *testing.T) {
	client, tasks, dl, events, metrics := setup(t)
	ctx := context.Background()

	task := model.Task{
		ID:         "task-active",
		Type:       "test",
		Priority:   5,
		MaxRetries: 3,
		Retries:    0,
		Status:     model.StatusProcessing,
		CreatedAt:  time.Now(),
	}

	// Put task with a deadline 30s in the future (not expired)
	futureDeadline := time.Now().Add(30 * time.Second).UnixMilli()
	simulateLeasedTask(ctx, t, client, tasks, task, futureDeadline)

	r := reaper.New(client, tasks, dl, events, metrics, store.NewNodeStore(client, 10*time.Second), reaper.Config{
		Interval:  50 * time.Millisecond,
		BatchSize: 10,
	})

	reaperCtx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
	defer cancel()
	r.Start(reaperCtx)

	// Task should still be in processing (not reclaimed)
	procSize, _ := client.ZCard(ctx, store.KeyProcessing).Result()
	if procSize != 1 {
		t.Fatalf("expected processing=1 (not expired), got %d", procSize)
	}

	// Ready should still be empty
	readySize, _ := client.ZCard(ctx, store.KeyReady).Result()
	if readySize != 0 {
		t.Fatalf("expected ready=0, got %d", readySize)
	}
}

func TestReaper_BatchesLargeReclaims(t *testing.T) {
	client, tasks, dl, events, metrics := setup(t)
	ctx := context.Background()

	// Create 15 expired tasks with batch size 10
	for i := 0; i < 15; i++ {
		task := model.Task{
			ID:         "task-" + strconv.Itoa(i),
			Type:       "test",
			Priority:   1,
			MaxRetries: 5,
			Retries:    0,
			Status:     model.StatusProcessing,
			CreatedAt:  time.Now(),
		}
		deadline := time.Now().Add(-10 * time.Second).UnixMilli()
		simulateLeasedTask(ctx, t, client, tasks, task, deadline)
	}

	r := reaper.New(client, tasks, dl, events, metrics, store.NewNodeStore(client, 10*time.Second), reaper.Config{
		Interval:  50 * time.Millisecond,
		BatchSize: 10, // only 10 per tick
	})

	// Run long enough for 2+ ticks to process all 15
	reaperCtx, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
	defer cancel()
	r.Start(reaperCtx)

	// All 15 should eventually be reclaimed
	readySize, _ := client.ZCard(ctx, store.KeyReady).Result()
	if readySize != 15 {
		t.Fatalf("expected ready=15 after batched reclaim, got %d", readySize)
	}

	procSize, _ := client.ZCard(ctx, store.KeyProcessing).Result()
	if procSize != 0 {
		t.Fatalf("expected processing=0, got %d", procSize)
	}
}


// TestReaper_OrphanInProcessing covers the case where a task ID sits in the
// processing set but its canonical record was flushed. The reaper must remove
// the orphan from processing without re-enqueueing it to ready or dead-lettering.
func TestReaper_OrphanInProcessing(t *testing.T) {
	client, tasks, dl, events, metrics := setup(t)
	ctx := context.Background()

	// Put an ID in processing with an expired deadline but NO task record.
	expiredDeadline := time.Now().Add(-10 * time.Second).UnixMilli()
	client.ZAdd(ctx, store.KeyProcessing, redis.Z{
		Score:  float64(expiredDeadline),
		Member: "task-orphan",
	})

	r := reaper.New(client, tasks, dl, events, metrics, store.NewNodeStore(client, 10*time.Second), reaper.Config{
		Interval:  50 * time.Millisecond,
		BatchSize: 10,
	})

	reaperCtx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
	defer cancel()
	r.Start(reaperCtx)

	// Orphan should be gone from processing.
	procSize, _ := client.ZCard(ctx, store.KeyProcessing).Result()
	if procSize != 0 {
		t.Fatalf("expected processing=0 after orphan removal, got %d", procSize)
	}
	// Must NOT have been re-enqueued to ready.
	readySize, _ := client.ZCard(ctx, store.KeyReady).Result()
	if readySize != 0 {
		t.Fatalf("expected ready=0 (orphan not reclaimed), got %d", readySize)
	}
	// Must NOT have been dead-lettered.
	dlSize, _ := client.LLen(ctx, store.KeyDeadLetter).Result()
	if dlSize != 0 {
		t.Fatalf("expected DLQ=0 (orphan not dead-lettered), got %d", dlSize)
	}
}


// TestReaper_DeadNodeEagerReclaim covers P2.3: a node whose heartbeat has
// expired (still in the registry, no hb key) has ALL its in-flight tasks
// reclaimed immediately — regardless of each task's individual lease deadline —
// and the node is removed from the registry afterwards.
func TestReaper_DeadNodeEagerReclaim(t *testing.T) {
	client, tasks, dl, events, metrics := setup(t)
	ctx := context.Background()
	nodes := store.NewNodeStore(client, 10*time.Second)

	const deadNode = "dead-node-1"
	// Register the node in the registry but give it NO heartbeat key => dead.
	client.SAdd(ctx, store.KeyNodes, deadNode)

	// Two in-flight tasks owned by the dead node, both with a FUTURE lease
	// deadline (so lease-expiry reclaim would NOT touch them — only dead-node
	// reclaim should).
	futureDeadline := time.Now().Add(1 * time.Hour).UnixMilli()
	for _, id := range []string{"dn-task-1", "dn-task-2"} {
		task := model.Task{
			ID: id, Type: "test", Priority: 3, MaxRetries: 3, Retries: 0,
			Status: model.StatusProcessing, Owner: deadNode, CreatedAt: time.Now(),
		}
		if err := tasks.Save(ctx, task); err != nil {
			t.Fatalf("save %s: %v", id, err)
		}
		client.ZAdd(ctx, store.KeyProcessing, redis.Z{Score: float64(futureDeadline), Member: id})
		client.SAdd(ctx, store.NodeTasksKey(deadNode), id)
	}

	r := reaper.New(client, tasks, dl, events, metrics, nodes, reaper.Config{
		Interval:  50 * time.Millisecond,
		BatchSize: 10,
	})
	reaperCtx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
	defer cancel()
	r.Start(reaperCtx)

	// Both tasks should be reclaimed to ready.
	readySize, _ := client.ZCard(ctx, store.KeyReady).Result()
	if readySize != 2 {
		t.Fatalf("expected ready=2 after dead-node reclaim, got %d", readySize)
	}
	// Processing should be empty (both leases released).
	procSize, _ := client.ZCard(ctx, store.KeyProcessing).Result()
	if procSize != 0 {
		t.Fatalf("expected processing=0, got %d", procSize)
	}
	// The dead node should be removed from the registry.
	ids, _ := nodes.RegisteredIDs(ctx)
	if len(ids) != 0 {
		t.Fatalf("expected dead node removed from registry, got %v", ids)
	}
	// Its task set should be gone.
	if n, _ := client.Exists(ctx, store.NodeTasksKey(deadNode)).Result(); n != 0 {
		t.Fatal("expected dead node's task set deleted")
	}
	// Reclaimed tasks should have retries incremented and be pending.
	for _, id := range []string{"dn-task-1", "dn-task-2"} {
		retries, _ := client.HGet(ctx, store.TaskKey(id), "retries").Result()
		if retries != "1" {
			t.Fatalf("task %s: expected retries=1, got %s", id, retries)
		}
		owner, _ := client.HGet(ctx, store.TaskKey(id), "owner").Result()
		if owner != "" {
			t.Fatalf("task %s: expected owner cleared, got %q", id, owner)
		}
	}
}
