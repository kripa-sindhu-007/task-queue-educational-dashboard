package queue

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/model"
	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/store"
)

func newSignalQueue(t *testing.T, cap int) (*redis.Client, *PriorityQueue) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { client.Close() })
	return client, NewPriorityQueue(client, store.NewTaskStore(client), cap)
}

// TestSignal_PushesExactlyOneToken proves each Signal rings the doorbell once.
func TestSignal_PushesExactlyOneToken(t *testing.T) {
	ctx := context.Background()
	client, q := newSignalQueue(t, DefaultSignalCap)

	if err := q.Signal(ctx); err != nil {
		t.Fatalf("signal: %v", err)
	}
	if n, _ := client.LLen(ctx, store.KeyReadySignal).Result(); n != 1 {
		t.Fatalf("expected signal LLEN=1 after one Signal, got %d", n)
	}
}

// TestSignal_RespectsCap proves the doorbell list never grows past SignalCap.
func TestSignal_RespectsCap(t *testing.T) {
	ctx := context.Background()
	const cap = 3
	client, q := newSignalQueue(t, cap)

	for i := 0; i < 10; i++ {
		if err := q.Signal(ctx); err != nil {
			t.Fatalf("signal %d: %v", i, err)
		}
	}
	if n, _ := client.LLen(ctx, store.KeyReadySignal).Result(); n != cap {
		t.Fatalf("expected signal LLEN capped at %d, got %d", cap, n)
	}
}

// TestEnqueue_RingsDoorbell proves Enqueue leaves exactly one token in the
// signal list, and that a worker blocked in WaitReady wakes as a result.
func TestEnqueue_RingsDoorbell(t *testing.T) {
	ctx := context.Background()
	client, q := newSignalQueue(t, DefaultSignalCap)

	if err := q.Enqueue(ctx, model.Task{ID: "t1", Priority: 1, Status: model.StatusPending}); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	if n, _ := client.LLen(ctx, store.KeyReadySignal).Result(); n != 1 {
		t.Fatalf("expected signal LLEN=1 after Enqueue, got %d", n)
	}

	woke, err := q.WaitReady(ctx, time.Second)
	if err != nil {
		t.Fatalf("waitready: %v", err)
	}
	if !woke {
		t.Fatal("expected a blocked WaitReady to return after Enqueue")
	}
}

// TestWaitReady_ReturnsTrueWhenTokenPresent proves a blocked worker wakes on a
// pending token.
func TestWaitReady_ReturnsTrueWhenTokenPresent(t *testing.T) {
	ctx := context.Background()
	_, q := newSignalQueue(t, DefaultSignalCap)

	if err := q.Signal(ctx); err != nil {
		t.Fatalf("signal: %v", err)
	}
	woke, err := q.WaitReady(ctx, time.Second)
	if err != nil {
		t.Fatalf("waitready: %v", err)
	}
	if !woke {
		t.Fatal("expected WaitReady to return true with a token present")
	}
}
