package store

import (
	"context"
	"fmt"
	"log"

	"github.com/redis/go-redis/v9"
)

const (
	KeyReady      = "taskqueue:ready"
	KeyDelayed    = "taskqueue:delayed"
	KeyProcessing = "taskqueue:processing" // Phase 1: leased tasks, score = lease deadline (ms)
	KeyDeadLetter = "taskqueue:deadletter"
	KeyMetrics    = "taskqueue:metrics"
	KeyEvents     = "taskqueue:events"
	KeyWorkers    = "taskqueue:workers"
	KeyNodes      = "taskqueue:nodes" // Phase 2: registry SET of known node IDs
)

// NodeHeartbeatKey returns the TTL key whose existence signals a node is alive.
func NodeHeartbeatKey(nodeID string) string { return "taskqueue:node:" + nodeID + ":hb" }

// NodeTasksKey returns the SET key holding the task IDs a node currently leases.
func NodeTasksKey(nodeID string) string { return "taskqueue:node:" + nodeID + ":tasks" }

func NewRedisClient(addr, password string) *redis.Client {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       0,
	})

	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		log.Fatalf("failed to connect to redis at %s: %v", addr, err)
	}
	fmt.Printf("Connected to Redis at %s\n", addr)
	return client
}
