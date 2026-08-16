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
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/api"
	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/broker"
	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/config"
	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/cron"
	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/election"
	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/handler"
	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/queue"
	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/reaper"
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

	// Redis
	redisClient := store.NewRedisClient(cfg.RedisAddr, cfg.RedisPass, cfg.WorkerCount, logger)
	defer redisClient.Close()

	// Prometheus metrics registered on a dedicated registry (not the global
	// default), so the server owns exactly the collectors it exposes on /metrics.
	registry := prometheus.NewRegistry()
	metrics := telemetry.New(registry)

	// Stores
	taskStore := store.NewTaskStore(redisClient)
	deadLetterStore := store.NewDeadLetterStore(redisClient)
	metricsStore := store.NewMetricsStore(redisClient)
	eventStore := store.NewEventStore(redisClient)
	workerStateStore := store.NewWorkerStateStore(redisClient)
	queuePeekStore := store.NewQueuePeekStore(redisClient, taskStore)
	nodeStore := store.NewNodeStore(redisClient, cfg.HeartbeatTTL)
	cronStore := cron.NewCronStore(redisClient)

	// queue_depth is a scrape-time collector reading ZCARD(ready) via the
	// QueuePeekStore; only the server registers it (workers have no such store).
	telemetry.RegisterQueueDepth(registry, queuePeekStore)

	// Queue + broker. The server's broker carries its own node identity so that
	// leases taken by in-process workers are attributed to (and reclaimable from)
	// this node.
	priorityQueue := queue.NewPriorityQueue(redisClient, taskStore, cfg.SignalCap)
	delayedScheduler := queue.NewDelayedScheduler(redisClient, priorityQueue, taskStore, eventStore, logger)
	nodeID := worker.NewNodeID()
	nodeLogger := logger.With("node_id", nodeID)
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
		Logger:       nodeLogger,
		Telemetry:    metrics,
	})
	pool := worker.NewPool(redisBroker, executor, cfg.WorkerCount, cfg.PollInterval, cfg.SignalBlock, workerStateStore, nodeLogger)

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
		Cron:        cronStore,
		Telemetry:   metrics,
		NodeID:      nodeID,

		MaxQueueDepth:     cfg.MaxQueueDepth,
		RetryAfterSeconds: cfg.RetryAfterSeconds,

		Logger: logger,
	})
	router := api.NewRouter(apiHandler, logger, promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))

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

	// Reaper (reclaims expired leases and dead nodes' in-flight work). Constructed
	// here but only started under leadership (see RunWhenLeader below).
	taskReaper := reaper.New(redisClient, taskStore, deadLetterStore, eventStore, metricsStore, nodeStore, reaper.Config{
		Interval:        cfg.ReaperInterval,
		BatchSize:       100,
		NodeGraceWindow: cfg.NodeGraceWindow,
		SignalCap:       cfg.SignalCap,
		Logger:          logger,
		Metrics:         metrics,
	})

	// Cron materializer (P4.4). Like the scheduler and reaper it is a singleton
	// background loop constructed here but only started under leadership, bound to
	// leaderCtx, so exactly one node fires scheduled tasks.
	cronMat := cron.NewCronMaterializer(cronStore, taskStore, priorityQueue, eventStore, metricsStore, cfg.CronTick, nil, logger)

	// Leader election (P4.1). The delayed scheduler and reaper are singletons: only
	// the leader runs them, bound to a leader-scoped context so they stop the
	// instant this node loses the lease and restart when it re-acquires. A
	// leader-ineligible node (LEADER_ELIGIBLE=false) parks in RunWhenLeader without
	// ever competing — it serves the API only. Single-binary dev keeps working:
	// the lone eligible node wins the lease-of-one immediately and runs both loops.
	elector := election.New(redisClient, nodeID, election.Config{
		TTL:      cfg.LeaderTTL,
		Eligible: cfg.LeaderEligible,
		Logger:   logger,
	})
	go func() {
		elector.RunWhenLeader(ctx, func(leaderCtx context.Context) {
			logger.Info("leadership acquired — starting delayed scheduler + reaper + cron", "node_id", nodeID)
			go delayedScheduler.Start(leaderCtx)
			go taskReaper.Start(leaderCtx)
			go cronMat.Start(leaderCtx)
		})
	}()

	// Cluster presence. In single-binary/dev mode (RUN_WORKERS=true) this process
	// runs the worker pool and registers as a "worker". Otherwise it registers as a
	// presence-only "server" node (capacity 0, no pool) so scheduler/API replicas
	// still appear in /api/nodes and can wear the leader crown, without starting the
	// worker execution path.
	var nodeCfg worker.NodeConfig
	if cfg.RunWorkers {
		nodeCfg = worker.NodeConfig{
			Nodes:             nodeStore,
			Pool:              pool,
			Events:            eventStore,
			NodeID:            nodeID,
			Hostname:          worker.Hostname(),
			Role:              "worker",
			Capacity:          cfg.WorkerCount,
			HeartbeatInterval: cfg.HeartbeatInterval,
			Logger:            logger,
		}
		logger.Info("in-process worker node enabled", "workers", cfg.WorkerCount)
	} else {
		nodeCfg = worker.NodeConfig{
			Nodes:             nodeStore,
			Pool:              nil, // presence-only: no executor loop
			Events:            eventStore,
			NodeID:            nodeID,
			Hostname:          worker.Hostname(),
			Role:              "server",
			Capacity:          0,
			HeartbeatInterval: cfg.HeartbeatInterval,
			Logger:            logger,
		}
		logger.Info("RUN_WORKERS=false — registered as presence-only server node (API + scheduler + reaper when leader)")
	}
	node := worker.NewNode(nodeCfg)
	nodeDone := make(chan struct{})
	go func() {
		defer close(nodeDone)
		node.Run(ctx) // blocks: register -> heartbeat (+ pool) -> drain -> deregister
	}()

	// Start HTTP server
	go func() {
		logger.Info("server listening", "port", cfg.ServerPort)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for shutdown signal
	<-ctx.Done()
	logger.Info("shutdown signal received")

	// Shutdown HTTP server with timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("HTTP server shutdown error", "error", err)
	}

	// Wait for the in-process node to drain in-flight work and deregister.
	if nodeDone != nil {
		<-nodeDone
	}
	logger.Info("shutdown complete")
}
