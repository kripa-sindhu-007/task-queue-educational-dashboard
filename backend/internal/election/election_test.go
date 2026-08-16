package election

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/store"
)

// eventually polls cond until it returns true or the timeout elapses. It fails
// the test (with msg) if cond never becomes true.
func eventually(t *testing.T, cond func() bool, timeout time.Duration, msg string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("condition not met within %v: %s", timeout, msg)
}

// never asserts cond stays false for the whole window.
func never(t *testing.T, cond func() bool, window time.Duration, msg string) {
	t.Helper()
	deadline := time.Now().Add(window)
	for time.Now().Before(deadline) {
		if cond() {
			t.Fatalf("condition became true within %v: %s", window, msg)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func leaderKey(t *testing.T, client *redis.Client) string {
	t.Helper()
	v, err := client.Get(context.Background(), store.KeyLeader).Result()
	if err == redis.Nil {
		return ""
	}
	if err != nil {
		t.Fatalf("get leader key: %v", err)
	}
	return v
}

// TestElector_FailoverAfterCrash exercises the full protocol: A acquires and its
// onElected runs; B does NOT run while A holds the lease; A "crashes" (client
// closed, no graceful release) so the key persists; only after the lease lapses
// (forced via miniredis FastForward — it has no wall-clock expiry) does B acquire
// and run its onElected.
func TestElector_FailoverAfterCrash(t *testing.T) {
	mr := miniredis.RunT(t)
	ttl := 300 * time.Millisecond

	clientA := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	clientB := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { clientB.Close() })

	var aRunning, bRunning atomic.Bool

	ctxA, cancelA := context.WithCancel(context.Background())
	defer cancelA()
	ctxB, cancelB := context.WithCancel(context.Background())
	defer cancelB()

	electorA := New(clientA, "node-A", Config{TTL: ttl, Eligible: true})
	electorB := New(clientB, "node-B", Config{TTL: ttl, Eligible: true})

	// onElected sets a "running" flag and clears it when the leader-scoped context
	// is cancelled — a faithful stand-in for the singleton loops.
	runFn := func(flag *atomic.Bool) func(context.Context) {
		return func(lctx context.Context) {
			flag.Store(true)
			go func() {
				<-lctx.Done()
				flag.Store(false)
			}()
		}
	}

	go electorA.RunWhenLeader(ctxA, runFn(&aRunning))

	// A acquires the lease and its onElected runs.
	eventually(t, electorA.IsLeader, 2*time.Second, "A did not become leader")
	eventually(t, aRunning.Load, time.Second, "A's onElected did not run")
	if got := leaderKey(t, clientB); got != "node-A" {
		t.Fatalf("leader key = %q, want node-A", got)
	}

	// B starts competing but must NOT take over while A holds the lease.
	go electorB.RunWhenLeader(ctxB, runFn(&bRunning))
	never(t, func() bool { return electorB.IsLeader() || bRunning.Load() }, 500*time.Millisecond,
		"B became leader while A still held the lease")

	// Crash A: close its client so it can neither renew nor release, then stop its
	// loop. The lease key persists in miniredis (no graceful DEL).
	clientA.Close()
	cancelA()

	// Before the lease lapses, B still cannot take over — miniredis does not expire
	// keys on wall-clock time, so the key remains until FastForward.
	never(t, electorB.IsLeader, 400*time.Millisecond, "B took over before the lease expired")
	if bRunning.Load() {
		t.Fatalf("B's onElected ran before the lease expired")
	}

	// Force the lease to lapse; now B acquires and its onElected runs.
	mr.FastForward(ttl + time.Second)
	eventually(t, func() bool { return electorB.IsLeader() && bRunning.Load() }, 3*time.Second,
		"B did not take over after the lease expired")
	if got := leaderKey(t, clientB); got != "node-B" {
		t.Fatalf("leader key after failover = %q, want node-B", got)
	}
}

// TestScripts_OwnerOnly verifies renew.lua and release.lua are strict CAS: a
// non-owner can neither renew nor release another node's lease.
func TestScripts_OwnerOnly(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { client.Close() })
	ctx := context.Background()

	a := New(client, "node-A", Config{TTL: time.Second, Eligible: true})
	b := New(client, "node-B", Config{TTL: time.Second, Eligible: true})

	acquired, err := a.acquire(ctx)
	if err != nil {
		t.Fatalf("A acquire: %v", err)
	}
	if !acquired {
		t.Fatal("A failed to acquire an unheld lease")
	}

	// Non-owner B cannot renew.
	renewed, err := b.renew(ctx)
	if err != nil {
		t.Fatalf("B renew: %v", err)
	}
	if renewed {
		t.Fatal("non-owner B renewed A's lease")
	}
	if got := leaderKey(t, client); got != "node-A" {
		t.Fatalf("leader key after B renew = %q, want node-A", got)
	}

	// Non-owner B cannot release (key remains A's).
	b.release(ctx)
	if got := leaderKey(t, client); got != "node-A" {
		t.Fatalf("leader key after B release = %q, want node-A", got)
	}

	// Owner A can renew and release.
	renewed, err = a.renew(ctx)
	if err != nil {
		t.Fatalf("A renew: %v", err)
	}
	if !renewed {
		t.Fatal("owner A failed to renew its own lease")
	}
	a.release(ctx)
	if got := leaderKey(t, client); got != "" {
		t.Fatalf("leader key after A release = %q, want empty", got)
	}
}

// TestElector_IneligibleNeverAcquires confirms a leader-ineligible node never
// acquires the lease or runs onElected — it just parks until ctx is cancelled.
func TestElector_IneligibleNeverAcquires(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { client.Close() })

	e := New(client, "node-X", Config{TTL: 200 * time.Millisecond, Eligible: false})

	ctx, cancel := context.WithCancel(context.Background())
	var ran atomic.Bool
	done := make(chan struct{})
	go func() {
		defer close(done)
		e.RunWhenLeader(ctx, func(context.Context) { ran.Store(true) })
	}()

	never(t, func() bool { return e.IsLeader() || ran.Load() }, 400*time.Millisecond,
		"ineligible node acquired leadership or ran onElected")
	if got := leaderKey(t, client); got != "" {
		t.Fatalf("ineligible node wrote the leader key: %q", got)
	}

	cancel()
	<-done
}
