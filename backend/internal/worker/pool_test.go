package worker_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/model"
	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/store"
	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/worker"
)

// waitForStatus polls the task record until its status field reaches want or the
// deadline elapses. Returns the last-seen status.
func waitForStatus(t *testing.T, client *redis.Client, taskID, want string, timeout time.Duration) string {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var last string
	for time.Now().Before(deadline) {
		last, _ = client.HGet(context.Background(), store.TaskKey(taskID), "status").Result()
		if last == want {
			return last
		}
		time.Sleep(5 * time.Millisecond)
	}
	return last
}

// TestPool_DoorbellWake covers P3.4: a single worker started against an empty
// queue blocks on the doorbell; a broker.Enqueue (which rings the doorbell) is
// then picked up and run promptly — no PollInterval sleep in the path.
func TestPool_DoorbellWake(t *testing.T) {
	env := newExecEnv(t, slog.Default(), nil)
	exec := worker.NewExecutor(env.deps)
	// signalBlock generous (1s) so success comes from the doorbell wake, not a
	// fallback-poll timeout; pollInterval is only the error back-off.
	pool := worker.NewPool(env.broker, exec, 1, 500*time.Millisecond, time.Second, nil, slog.Default())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	pool.Start(ctx)
	// Let the worker reach the doorbell block on the empty queue.
	time.Sleep(50 * time.Millisecond)

	task := model.Task{ID: "wake-1", Type: "ok", MaxRetries: 1, Status: model.StatusPending, CreatedAt: time.Now()}
	if err := env.broker.Enqueue(ctx, task); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	// Should be completed well before the 1s signalBlock would have elapsed.
	if got := waitForStatus(t, env.client, "wake-1", "completed", 400*time.Millisecond); got != "completed" {
		t.Fatalf("expected task completed via doorbell wake, got status=%q", got)
	}

	cancel()
	pool.Wait()
}

// TestPool_FallbackPollWithoutToken covers the fallback backstop: a task placed
// directly in the ready set with NO doorbell token must still be picked up when
// the BLPOP times out after signalBlock. Proves correctness never depends on a
// token arriving.
func TestPool_FallbackPollWithoutToken(t *testing.T) {
	env := newExecEnv(t, slog.Default(), nil)
	exec := worker.NewExecutor(env.deps)
	// Tiny signalBlock so the fallback poll fires quickly (miniredis BLPOP timeouts
	// use real wall-clock; keep it small to stay fast).
	pool := worker.NewPool(env.broker, exec, 1, 500*time.Millisecond, 50*time.Millisecond, nil, slog.Default())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	pool.Start(ctx)
	// Let the worker block on the doorbell over the empty queue first.
	time.Sleep(80 * time.Millisecond)

	// Persist the record and place the ID directly in ready WITHOUT ringing the
	// doorbell — the executor path (ZAdd + Signal) is deliberately bypassed here.
	tasks := store.NewTaskStore(env.client)
	task := model.Task{ID: "fallback-1", Type: "ok", MaxRetries: 1, Status: model.StatusPending, CreatedAt: time.Now()}
	if err := tasks.Save(ctx, task); err != nil {
		t.Fatalf("save: %v", err)
	}
	if err := env.client.ZAdd(ctx, store.KeyReady, redis.Z{Score: 0, Member: "fallback-1"}).Err(); err != nil {
		t.Fatalf("zadd ready: %v", err)
	}

	// No token was pushed, so pickup must come from the BLPOP timeout + re-poll.
	if got := waitForStatus(t, env.client, "fallback-1", "completed", 2*time.Second); got != "completed" {
		t.Fatalf("expected task completed via fallback poll, got status=%q", got)
	}
	// The doorbell list stayed empty throughout (we never rang it).
	if n, _ := env.client.LLen(ctx, store.KeyReadySignal).Result(); n != 0 {
		t.Fatalf("expected no doorbell tokens in fallback path, got %d", n)
	}

	cancel()
	pool.Wait()
}
