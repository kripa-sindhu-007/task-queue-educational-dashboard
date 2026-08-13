package reaper_test

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/broker"
	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/model"
	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/queue"
	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/reaper"
	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/store"
)

// TestIntegration_NodeDeath_TasksReassigned is the P2.8 "money demo" as an
// integration test: two nodes share one Redis (miniredis). Node A leases several
// tasks, then dies (its heartbeat expires). The reaper detects the dead node and
// eagerly reclaims all of A's in-flight tasks back to ready, where Node B picks
// them up. Owner fencing then rejects A's late Ack, proving no cross-node
// double-completion.
//
// This drives the real Broker + Reaper + NodeStore end-to-end. A true
// testcontainers-go variant (separate OS processes against a Redis container) is
// intentionally deferred: it requires a running Docker daemon and a new test
// dependency, and would exercise the same Redis-side logic this test already
// covers via miniredis.
func TestIntegration_NodeDeath_TasksReassigned(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { client.Close() })

	ctx := context.Background()

	tasks := store.NewTaskStore(client)
	dl := store.NewDeadLetterStore(client)
	events := store.NewEventStore(client)
	metrics := store.NewMetricsStore(client)
	nodes := store.NewNodeStore(client, 10*time.Second)
	pq := queue.NewPriorityQueue(client, tasks)
	delayed := queue.NewDelayedScheduler(client, pq, tasks, events, nil)

	// Two nodes sharing the same Redis, each with its own broker identity.
	brokerA := broker.NewRedisBroker(client, tasks, pq, delayed, 30*time.Second, "node-A")
	brokerB := broker.NewRedisBroker(client, tasks, pq, delayed, 30*time.Second, "node-B")

	// Node A joins the cluster.
	if err := nodes.Register(ctx, model.Node{
		ID: "node-A", Hostname: "host-a", Capacity: 5, StartedAt: time.Now(),
	}); err != nil {
		t.Fatalf("register node-A: %v", err)
	}

	// Enqueue and have Node A lease three tasks.
	const n = 3
	ids := []string{"t1", "t2", "t3"}
	for _, id := range ids {
		if err := brokerA.Enqueue(ctx, model.Task{
			ID: id, Type: "sleep", Priority: 1, MaxRetries: 3, Status: model.StatusPending, CreatedAt: time.Now(),
		}); err != nil {
			t.Fatalf("enqueue %s: %v", id, err)
		}
	}
	for i := 0; i < n; i++ {
		task, err := brokerA.Dequeue(ctx)
		if err != nil || task == nil {
			t.Fatalf("node-A dequeue %d: task=%v err=%v", i, task, err)
		}
	}

	// Node A now holds all three in flight.
	owned, _ := nodes.OwnedTaskIDs(ctx, "node-A")
	if len(owned) != n {
		t.Fatalf("expected node-A to own %d tasks, got %d", n, len(owned))
	}
	if size, _ := pq.Size(ctx); size != 0 {
		t.Fatalf("expected ready empty while A holds leases, got %d", size)
	}

	// Node A dies: its heartbeat key expires. (Lease deadlines are far in the
	// future, so ONLY dead-node reclaim — not lease expiry — can recover these.)
	mr.FastForward(11 * time.Second)

	// Run the reaper for a few ticks.
	r := reaper.New(client, tasks, dl, events, metrics, nodes, reaper.Config{
		Interval:  50 * time.Millisecond,
		BatchSize: 10,
	})
	reaperCtx, cancel := context.WithTimeout(ctx, 250*time.Millisecond)
	defer cancel()
	r.Start(reaperCtx)

	// All three tasks reclaimed to ready; processing drained; node-A removed.
	if size, _ := pq.Size(ctx); size != int64(n) {
		t.Fatalf("expected ready=%d after dead-node reclaim, got %d", n, size)
	}
	if proc, _ := client.ZCard(ctx, store.KeyProcessing).Result(); proc != 0 {
		t.Fatalf("expected processing=0, got %d", proc)
	}
	if regIDs, _ := nodes.RegisteredIDs(ctx); len(regIDs) != 0 {
		t.Fatalf("expected node-A removed from registry, got %v", regIDs)
	}

	// Node B picks up all reassigned tasks.
	reassigned := make(map[string]bool)
	for i := 0; i < n; i++ {
		task, err := brokerB.Dequeue(ctx)
		if err != nil || task == nil {
			t.Fatalf("node-B dequeue %d: task=%v err=%v", i, task, err)
		}
		reassigned[task.ID] = true
	}
	if len(reassigned) != n {
		t.Fatalf("expected node-B to pick up %d distinct tasks, got %d", n, len(reassigned))
	}

	// Owner fencing: node-A's late Ack of a now-B-owned task must be rejected,
	// while node-B's Ack succeeds — no cross-node double-completion.
	if err := brokerA.Ack(ctx, "t1"); err != broker.ErrLeaseNotHeld {
		t.Fatalf("expected node-A stale Ack to be fenced (ErrLeaseNotHeld), got %v", err)
	}
	if err := brokerB.Ack(ctx, "t1"); err != nil {
		t.Fatalf("expected node-B Ack to succeed, got %v", err)
	}
}
