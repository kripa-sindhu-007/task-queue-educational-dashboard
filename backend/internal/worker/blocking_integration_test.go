//go:build integration

// This file is compiled only under `-tags=integration` (see the Makefile's
// test-integration target). It exercises the P3.4 doorbell against a REAL Redis
// because miniredis's BLPOP block-timeout is not advanced by mr.FastForward and
// uses real wall-clock — so the timeout / ctx-cancel semantics and the latency
// numbers must be proven against a real server, not miniredis.
//
// Point it at a Redis with REDIS_ADDR (default localhost:6379). Bring one up with:
//
//	docker run -d --rm -p 6399:6379 --name tq-p34-redis redis:7-alpine
//	REDIS_ADDR=localhost:6399 go test -tags=integration ./internal/worker/ -run Integration -v
package worker_test

import (
	"context"
	"log/slog"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/broker"
	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/handler"
	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/queue"
	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/store"
	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/worker"
)

func integrationRedis(t *testing.T) *redis.Client {
	t.Helper()
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}
	// Use a dedicated DB so we can flush freely without touching a running stack.
	client := redis.NewClient(&redis.Options{Addr: addr, DB: 15})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		client.Close()
		t.Skipf("no Redis at %s (set REDIS_ADDR): %v", addr, err)
	}
	if err := client.FlushDB(ctx).Err(); err != nil {
		t.Fatalf("flushdb: %v", err)
	}
	t.Cleanup(func() {
		client.FlushDB(context.Background())
		client.Close()
	})
	return client
}

func newIntegrationBroker(t *testing.T, client *redis.Client) (*broker.RedisBroker, *queue.PriorityQueue) {
	t.Helper()
	tasks := store.NewTaskStore(client)
	pq := queue.NewPriorityQueue(client, tasks, queue.DefaultSignalCap)
	delayed := queue.NewDelayedScheduler(client, pq, tasks, nil, slog.Default())
	return broker.NewRedisBroker(client, tasks, pq, delayed, 30*time.Second, "it-node"), pq
}

// TestIntegration_WaitForReady_BlockTimeout proves a real BLPOP blocks for
// ~signalBlock when no token is pushed, then returns nil (the fallback-poll
// backstop). miniredis cannot prove this — its BLPOP timeout ignores FastForward.
//
// It also documents a real go-redis constraint: BLPop truncates any sub-second
// timeout up to 1s (Redis BLPOP's practical resolution), which is exactly why the
// SignalBlock default is 1s — the smallest block go-redis will honor.
func TestIntegration_WaitForReady_BlockTimeout(t *testing.T) {
	client := integrationRedis(t)
	b, _ := newIntegrationBroker(t, client)

	const block = time.Second // go-redis floors BLPOP timeouts at 1s
	start := time.Now()
	if err := b.WaitForReady(context.Background(), block); err != nil {
		t.Fatalf("WaitForReady returned error on empty doorbell: %v", err)
	}
	elapsed := time.Since(start)
	if elapsed < block-100*time.Millisecond {
		t.Fatalf("WaitForReady returned after %v; expected it to block ~%v", elapsed, block)
	}
	if elapsed > block+time.Second {
		t.Fatalf("WaitForReady blocked %v; far longer than the %v timeout", elapsed, block)
	}
}

// TestIntegration_WaitForReady_AlreadyCancelledCtx proves that issuing the
// doorbell block with an already-cancelled context returns an error promptly
// (context.Canceled), which pool.worker treats as a clean shutdown exit.
func TestIntegration_WaitForReady_AlreadyCancelledCtx(t *testing.T) {
	client := integrationRedis(t)
	b, _ := newIntegrationBroker(t, client)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before the call

	start := time.Now()
	err := b.WaitForReady(ctx, 10*time.Second)
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("expected WaitForReady to return an error for an already-cancelled ctx, got nil")
	}
	if elapsed > time.Second {
		t.Fatalf("WaitForReady took %v with a pre-cancelled ctx; expected a prompt return", elapsed)
	}
}

// TestIntegration_ShutdownWithinSignalBlock proves the graceful-shutdown
// contract: a full pool blocked on the doorbell over an empty queue drains within
// ~SignalBlock of ctx cancellation. go-redis does NOT interrupt an in-flight
// BLPOP mid-block (verified separately), so shutdown latency is bounded by the
// block timeout returning, after which pool.worker re-checks ctx.Done() and
// exits — never longer than SignalBlock plus a small margin.
func TestIntegration_ShutdownWithinSignalBlock(t *testing.T) {
	client := integrationRedis(t)
	b, pq := newIntegrationBroker(t, client)

	tasks := store.NewTaskStore(client)
	exec := worker.NewExecutor(worker.ExecutorDeps{
		Broker:     b,
		Handlers:   handler.NewDefaultRegistry(),
		Tasks:      tasks,
		Delayed:    queue.NewDelayedScheduler(client, pq, tasks, nil, slog.Default()),
		DeadLetter: store.NewDeadLetterStore(client),
		Metrics:    store.NewMetricsStore(client),
		Events:     store.NewEventStore(client),
		Logger:     slog.Default(),
	})

	const signalBlock = time.Second
	pool := worker.NewPool(b, exec, 5, 500*time.Millisecond, signalBlock, nil, slog.Default())

	ctx, cancel := context.WithCancel(context.Background())
	pool.Start(ctx)
	// Let all workers reach the doorbell block on the empty queue.
	time.Sleep(150 * time.Millisecond)

	cancel()
	done := make(chan struct{})
	go func() { pool.Wait(); close(done) }()

	select {
	case <-done:
		// drained within the bound — good
	case <-time.After(signalBlock + time.Second):
		t.Fatalf("pool did not drain within SignalBlock (%v) + margin after ctx cancel", signalBlock)
	}
}

// TestIntegration_WaitForReady_WakeLatency measures the doorbell wake latency
// (Signal -> WaitForReady unblocks) against real Redis and prints p50/p99, and
// contrasts it with a faithful model of the OLD sleep-poll loop (a task arriving
// at a uniform phase within the poll cycle waits out the remainder of the cycle)
// so docs/BENCHMARKS.md carries an honest before/after captured on one machine.
//
// This is a measurement, not a hard assertion (latency is machine-dependent); it
// only sanity-asserts the doorbell p99 is well under the poll interval.
func TestIntegration_WaitForReady_WakeLatency(t *testing.T) {
	client := integrationRedis(t)
	b, pq := newIntegrationBroker(t, client)
	ctx := context.Background()

	const iterations = 200
	const pollInterval = 500 * time.Millisecond

	// --- AFTER: doorbell wake latency (Signal -> WaitForReady returns) ---
	doorbell := make([]time.Duration, 0, iterations)
	for i := 0; i < iterations; i++ {
		done := make(chan time.Duration, 1)
		go func() {
			start := time.Now()
			_ = b.WaitForReady(ctx, 5*time.Second)
			done <- time.Since(start)
		}()
		// Let the goroutine reach the BLPOP block, then ring the doorbell.
		time.Sleep(2 * time.Millisecond)
		start := time.Now()
		if err := pq.Signal(ctx); err != nil {
			t.Fatalf("signal: %v", err)
		}
		waitElapsed := <-done
		// Subtract the settle sleep: the wake latency is Signal -> return.
		if waitElapsed > time.Since(start) {
			waitElapsed = time.Since(start)
		}
		doorbell = append(doorbell, waitElapsed)
		client.Del(ctx, store.KeyReadySignal)
	}

	// --- BEFORE: faithful model of the old sleep-poll loop. A task arriving at a
	// uniform phase in [0, pollInterval) is only seen on the next poll tick, so
	// its latency is the remainder of the cycle. ---
	poll := make([]time.Duration, 0, iterations)
	for i := 0; i < iterations; i++ {
		phase := time.Duration(int64(pollInterval) * int64(i) / int64(iterations))
		poll = append(poll, pollInterval-phase)
	}

	dp50, dp99 := percentile(doorbell, 50), percentile(doorbell, 99)
	pp50, pp99 := percentile(poll, 50), percentile(poll, 99)

	t.Logf("enqueue_to_start wake latency over %d iterations:", iterations)
	t.Logf("  doorbell (after):  p50=%v  p99=%v", dp50.Round(time.Microsecond), dp99.Round(time.Microsecond))
	t.Logf("  sleep-poll (before, PollInterval=%v): p50=%v  p99=%v", pollInterval, pp50.Round(time.Millisecond), pp99.Round(time.Millisecond))

	if dp99 > pollInterval/2 {
		t.Fatalf("doorbell p99=%v not meaningfully below poll interval %v", dp99, pollInterval)
	}
}

func percentile(ds []time.Duration, p int) time.Duration {
	if len(ds) == 0 {
		return 0
	}
	sorted := make([]time.Duration, len(ds))
	copy(sorted, ds)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	idx := (p * (len(sorted) - 1)) / 100
	return sorted[idx]
}
