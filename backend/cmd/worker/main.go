// Command worker is a standalone worker node (P2.1). It connects to Redis,
// registers itself in the cluster with a heartbeat, and runs a pool of executor
// goroutines that lease tasks through the broker. Its only HTTP surface is a
// minimal /metrics endpoint (METRICS_PORT) for Prometheus — the server owns the
// API, delayed scheduler and reaper. Scale it with
// `docker compose up --scale worker=N`.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/broker"
	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/config"
	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/handler"
	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/queue"
	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/store"
	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/telemetry"
	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/worker"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("config error", "error", err)
		os.Exit(1)
	}

	redisClient := store.NewRedisClient(cfg.RedisAddr, cfg.RedisPass, cfg.WorkerCount, logger)
	defer redisClient.Close()

	// Prometheus metrics on a dedicated registry. The worker emits the task and
	// reaper-independent counters; queue_depth is the server's (it owns the
	// QueuePeekStore).
	registry := prometheus.NewRegistry()
	metrics := telemetry.New(registry)

	// Stores this node needs. Note: no API/queue-peek stores — this is a pure
	// worker. WorkerState (the legacy per-goroutine hash) is intentionally unused;
	// cluster visibility comes from the node registry.
	taskStore := store.NewTaskStore(redisClient)
	deadLetterStore := store.NewDeadLetterStore(redisClient)
	metricsStore := store.NewMetricsStore(redisClient)
	eventStore := store.NewEventStore(redisClient)
	nodeStore := store.NewNodeStore(redisClient, cfg.HeartbeatTTL)

	priorityQueue := queue.NewPriorityQueue(redisClient, taskStore, cfg.SignalCap)
	delayedScheduler := queue.NewDelayedScheduler(redisClient, priorityQueue, taskStore, eventStore, logger)

	nodeID := worker.NewNodeID()
	nodeLogger := logger.With("node_id", nodeID)
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
		Logger:       nodeLogger,
		Telemetry:    metrics,
	})
	pool := worker.NewPool(redisBroker, executor, cfg.WorkerCount, cfg.PollInterval, cfg.SignalBlock, nil, nodeLogger)

	node := worker.NewNode(worker.NodeConfig{
		Nodes:             nodeStore,
		Pool:              pool,
		Events:            eventStore,
		NodeID:            nodeID,
		Hostname:          worker.Hostname(),
		Capacity:          cfg.WorkerCount,
		HeartbeatInterval: cfg.HeartbeatInterval,
		Logger:            logger,
	})

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Minimal /metrics server for Prometheus to scrape this worker. Started before
	// the blocking node.Run and gracefully shut down after it returns so
	// `docker compose kill`/drain still exits promptly.
	metricsMux := http.NewServeMux()
	metricsMux.Handle("GET /metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
	metricsServer := &http.Server{
		Addr:    ":" + cfg.MetricsPort,
		Handler: metricsMux,
	}
	go func() {
		logger.Info("metrics server listening", "port", cfg.MetricsPort)
		if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("metrics server error", "error", err)
		}
	}()

	logger.Info("worker node starting", "node_id", nodeID, "workers", cfg.WorkerCount)
	node.Run(ctx) // blocks until signal + drain + deregister

	// Shut down the metrics server with a fresh (bounded) context — ctx is already
	// cancelled by the time node.Run returns.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := metricsServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("metrics server shutdown error", "error", err)
	}
	logger.Info("worker shutdown complete")
}
