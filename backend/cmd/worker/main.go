// Command worker is a standalone worker node (P2.1). It connects to Redis,
// registers itself in the cluster with a heartbeat, and runs a pool of executor
// goroutines that lease tasks through the broker. It carries no HTTP API — the
// server owns the API, delayed scheduler and reaper. Scale it with
// `docker compose up --scale worker=N`.
package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/broker"
	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/config"
	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/handler"
	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/queue"
	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/store"
	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/worker"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	redisClient := store.NewRedisClient(cfg.RedisAddr, cfg.RedisPass)
	defer redisClient.Close()

	// Stores this node needs. Note: no API/queue-peek stores — this is a pure
	// worker. WorkerState (the legacy per-goroutine hash) is intentionally unused;
	// cluster visibility comes from the node registry.
	taskStore := store.NewTaskStore(redisClient)
	deadLetterStore := store.NewDeadLetterStore(redisClient)
	metricsStore := store.NewMetricsStore(redisClient)
	eventStore := store.NewEventStore(redisClient)
	nodeStore := store.NewNodeStore(redisClient, cfg.HeartbeatTTL)

	priorityQueue := queue.NewPriorityQueue(redisClient, taskStore)
	delayedScheduler := queue.NewDelayedScheduler(redisClient, priorityQueue, taskStore, eventStore)

	nodeID := worker.NewNodeID()
	redisBroker := broker.NewRedisBroker(redisClient, taskStore, priorityQueue, delayedScheduler, cfg.VisibilityTimeout, nodeID)

	executor := worker.NewExecutor(worker.ExecutorDeps{
		Broker:       redisBroker,
		Handlers:     handler.NewDefaultRegistry(),
		Tasks:        taskStore,
		Delayed:      delayedScheduler,
		DeadLetter:   deadLetterStore,
		Metrics:      metricsStore,
		Events:       eventStore,
		WorkerState:  nil, // node cards supersede the legacy per-goroutine view
		DrainTimeout: cfg.DrainTimeout,
	})
	pool := worker.NewPool(redisBroker, executor, cfg.WorkerCount, cfg.PollInterval, nil)

	node := worker.NewNode(worker.NodeConfig{
		Nodes:             nodeStore,
		Pool:              pool,
		NodeID:            nodeID,
		Hostname:          worker.Hostname(),
		Capacity:          cfg.WorkerCount,
		HeartbeatInterval: cfg.HeartbeatInterval,
	})

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log.Printf("Worker node %s starting (workers=%d)", nodeID, cfg.WorkerCount)
	node.Run(ctx) // blocks until signal + drain + deregister
	log.Println("Worker shutdown complete")
}
