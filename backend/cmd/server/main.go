// Command server hosts the HTTP API, the delayed scheduler and the reaper. In
// single-binary/dev mode (RUN_WORKERS=true, the default) it also runs an
// in-process worker node so `docker compose up` works with one backend
// container. In distributed mode (RUN_WORKERS=false) task execution is handled
// by separate `cmd/worker` processes and the server is API + scheduler + reaper
// only.
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os/signal"
	"syscall"

	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/api"
	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/broker"
	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/config"
	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/handler"
	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/queue"
	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/reaper"
	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/store"
	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/worker"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	// Redis
	redisClient := store.NewRedisClient(cfg.RedisAddr, cfg.RedisPass)
	defer redisClient.Close()

	// Stores
	taskStore := store.NewTaskStore(redisClient)
	deadLetterStore := store.NewDeadLetterStore(redisClient)
	metricsStore := store.NewMetricsStore(redisClient)
	eventStore := store.NewEventStore(redisClient)
	workerStateStore := store.NewWorkerStateStore(redisClient)
	queuePeekStore := store.NewQueuePeekStore(redisClient, taskStore)
	nodeStore := store.NewNodeStore(redisClient, cfg.HeartbeatTTL)

	// Queue + broker. The server's broker carries its own node identity so that
	// leases taken by in-process workers are attributed to (and reclaimable from)
	// this node.
	priorityQueue := queue.NewPriorityQueue(redisClient, taskStore)
	delayedScheduler := queue.NewDelayedScheduler(redisClient, priorityQueue, taskStore, eventStore)
	nodeID := worker.NewNodeID()
	redisBroker := broker.NewRedisBroker(redisClient, taskStore, priorityQueue, delayedScheduler, cfg.VisibilityTimeout, nodeID)

	// Worker pool (always constructed so the API can report ActiveWorkers; only
	// started when RUN_WORKERS is true).
	executor := worker.NewExecutor(worker.ExecutorDeps{
		Broker:       redisBroker,
		Handlers:     handler.NewDefaultRegistry(),
		Tasks:        taskStore,
		Delayed:      delayedScheduler,
		DeadLetter:   deadLetterStore,
		Metrics:      metricsStore,
		Events:       eventStore,
		WorkerState:  workerStateStore,
		DrainTimeout: cfg.DrainTimeout,
	})
	pool := worker.NewPool(redisBroker, executor, cfg.WorkerCount, cfg.PollInterval, workerStateStore)

	// API
	apiHandler := api.NewHandler(api.HandlerDeps{
		Queue:       priorityQueue,
		Delayed:     delayedScheduler,
		DeadLetter:  deadLetterStore,
		Metrics:     metricsStore,
		Pool:        pool,
		Redis:       redisClient,
		Events:      eventStore,
		WorkerState: workerStateStore,
		QueuePeek:   queuePeekStore,
		Tasks:       taskStore,
		Nodes:       nodeStore,
	})
	router := api.NewRouter(apiHandler)

	server := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.ServerPort),
		Handler:      router,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  cfg.IdleTimeout,
	}

	// Graceful shutdown context
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Start delayed scheduler
	go delayedScheduler.Start(ctx)

	// Start reaper (reclaims expired leases and dead nodes' in-flight work)
	taskReaper := reaper.New(redisClient, taskStore, deadLetterStore, eventStore, metricsStore, nodeStore, reaper.Config{
		Interval:        cfg.ReaperInterval,
		BatchSize:       100,
		NodeGraceWindow: cfg.NodeGraceWindow,
	})
	go taskReaper.Start(ctx)

	// Start in-process worker node when enabled (single-binary/dev mode).
	var nodeDone chan struct{}
	if cfg.RunWorkers {
		node := worker.NewNode(worker.NodeConfig{
			Nodes:             nodeStore,
			Pool:              pool,
			Events:            eventStore,
			NodeID:            nodeID,
			Hostname:          worker.Hostname(),
			Capacity:          cfg.WorkerCount,
			HeartbeatInterval: cfg.HeartbeatInterval,
		})
		nodeDone = make(chan struct{})
		go func() {
			defer close(nodeDone)
			node.Run(ctx) // blocks: register -> heartbeat + pool -> drain -> deregister
		}()
		log.Printf("In-process worker node enabled (%d workers)", cfg.WorkerCount)
	} else {
		log.Println("RUN_WORKERS=false — server runs API + scheduler + reaper only")
	}

	// Start HTTP server
	go func() {
		log.Printf("Server listening on :%s", cfg.ServerPort)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Wait for shutdown signal
	<-ctx.Done()
	log.Println("Shutdown signal received")

	// Shutdown HTTP server with timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}

	// Wait for the in-process node to drain in-flight work and deregister.
	if nodeDone != nil {
		<-nodeDone
	}
	log.Println("Shutdown complete")
}
