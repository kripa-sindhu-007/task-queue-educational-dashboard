package store_test

import (
	"testing"

	"github.com/alicebob/miniredis/v2"

	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/store"
)

// TestNewRedisClient_PoolSizeExceedsWorkerCount asserts the P3.4 pool-sizing
// invariant: each idle worker holds a connection while blocked on the doorbell
// BLPOP, so the pool must be strictly larger than WorkerCount to leave headroom
// for heartbeats/claims/acks/reaper/scheduler (a starved heartbeat looks like a
// dead node).
func TestNewRedisClient_PoolSizeExceedsWorkerCount(t *testing.T) {
	mr := miniredis.RunT(t)

	for _, workerCount := range []int{1, 5, 50} {
		client := store.NewRedisClient(mr.Addr(), "", workerCount, nil)
		got := client.Options().PoolSize
		if got <= workerCount {
			t.Errorf("workerCount=%d: PoolSize=%d, want > workerCount", workerCount, got)
		}
		client.Close()
	}
}
