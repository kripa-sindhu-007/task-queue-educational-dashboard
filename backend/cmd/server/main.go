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

	// Queue + broker
	priorityQueue := queue.NewPriorityQueue(redisClient, taskStore)
	delayedScheduler := queue.NewDelayedScheduler(redisClient, priorityQueue, taskStore, eventStore)
	redisBroker := broker.NewRedisBroker(redisClient, taskStore, priorityQueue, delayedScheduler, cfg.VisibilityTimeout)

	// Workers
	executor := worker.NewExecutor(worker.ExecutorDeps{
		Broker:       redisBroker,
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
	handler := api.NewHandler(api.HandlerDeps{
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
	})
	router := api.NewRouter(handler)

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

	// Start reaper (reclaims expired leases from processing set)
	taskReaper := reaper.New(redisClient, taskStore, deadLetterStore, eventStore, metricsStore, reaper.Config{
		Interval:  cfg.ReaperInterval,
		BatchSize: 100,
	})
	go taskReaper.Start(ctx)

	// Start worker pool
	pool.Start(ctx)

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

	// Wait for workers to finish in-progress tasks
	pool.Wait()
	log.Println("Shutdown complete")
}
