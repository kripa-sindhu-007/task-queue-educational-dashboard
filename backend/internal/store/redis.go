package store

import (
	"context"
	"log/slog"
	"os"

	"github.com/redis/go-redis/v9"
)

const (
	KeyReady         = "taskqueue:ready"
	KeyDelayed       = "taskqueue:delayed"
	KeyProcessing    = "taskqueue:processing" // Phase 1: leased tasks, score = lease deadline (ms)
	KeyDeadLetter    = "taskqueue:deadletter"
	KeyMetrics       = "taskqueue:metrics"
	KeyEvents        = "taskqueue:events"
	KeyWorkers       = "taskqueue:workers"
	KeyNodes         = "taskqueue:nodes"          // Phase 2: registry SET of known node IDs
	KeyEventsCluster = "taskqueue:events:cluster" // retained list of lifecycle-only events (node_joined/node_dead/reclaimed)
	KeyReadySignal   = "taskqueue:ready:signal"   // P3.4: doorbell list — one wake-up token per newly-ready task (see queue.Signal)
	KeyLeader        = "taskqueue:leader"         // P4.1: leader-election lease — value = leader node ID, PX = LeaderTTL
	KeyCron          = "taskqueue:cron"           // P4.4: cron-job registry hash — {job id → json(CronJob)}
)

// poolHeadroom is the number of connections reserved on top of WorkerCount when
// sizing the go-redis pool. Each idle worker holds a connection for up to
// SignalBlock while blocked on the doorbell BLPOP, so the pool must leave room
// for heartbeats, claims/acks, the reaper and the delayed scheduler to run
// without starving — a starved heartbeat looks like a dead node and triggers a
// spurious reclaim.
const poolHeadroom = 10

// NodeHeartbeatKey returns the TTL key whose existence signals a node is alive.
func NodeHeartbeatKey(nodeID string) string { return "taskqueue:node:" + nodeID + ":hb" }

// NodeTasksKey returns the SET key holding the task IDs a node currently leases.
func NodeTasksKey(nodeID string) string { return "taskqueue:node:" + nodeID + ":tasks" }

// NodeMetaKey returns the non-TTL key holding a node's descriptor JSON. It
// outlives the heartbeat key so a dead node still resolves its hostname/capacity
// until the reaper prunes it.
func NodeMetaKey(nodeID string) string { return "taskqueue:node:" + nodeID + ":meta" }

// NodeDeadKey returns the tombstone key written (SET NX) when the reaper first
// observes a node as dead. Its value is the death timestamp (ms) and it guards
// reclaim-once plus the prune grace window.
func NodeDeadKey(nodeID string) string { return "taskqueue:node:" + nodeID + ":dead" }

// NewRedisClient dials Redis and sizes the connection pool for the doorbell
// (P3.4). workerCount idle workers each hold a connection while blocked on the
// BLPOP doorbell, so PoolSize is set to workerCount + poolHeadroom to keep
// heartbeats/claims/acks/reaper/scheduler from starving.
func NewRedisClient(addr, password string, workerCount int, logger *slog.Logger) *redis.Client {
	if logger == nil {
		logger = slog.Default()
	}
	poolSize := workerCount + poolHeadroom
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       0,
		PoolSize: poolSize,
	})

	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		logger.Error("failed to connect to redis", "addr", addr, "error", err)
		os.Exit(1)
	}
	logger.Info("connected to Redis", "addr", addr)
	return client
}
