package worker

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/broker"
	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/store"
)

type Pool struct {
	broker       broker.Broker
	executor     *Executor
	workerCount  int
	pollInterval time.Duration
	activeCount  atomic.Int64
	wg           sync.WaitGroup
	workerState  *store.WorkerStateStore
	logger       *slog.Logger
}

func NewPool(
	b broker.Broker,
	executor *Executor,
	workerCount int,
	pollInterval time.Duration,
	workerState *store.WorkerStateStore,
	logger *slog.Logger,
) *Pool {
	if logger == nil {
		logger = slog.Default()
	}
	return &Pool{
		broker:       b,
		executor:     executor,
		workerCount:  workerCount,
		pollInterval: pollInterval,
		workerState:  workerState,
		logger:       logger,
	}
}

// Start launches worker goroutines. They poll until ctx is cancelled, then finish in-progress work.
func (p *Pool) Start(ctx context.Context) {
	for i := 0; i < p.workerCount; i++ {
		p.wg.Add(1)
		// Initialize worker state as idle (legacy per-goroutine view; optional —
		// the standalone worker binary relies on node cards instead).
		if p.workerState != nil {
			p.workerState.Set(ctx, store.WorkerIdleState(i))
		}
		go p.worker(ctx, i)
	}
	p.logger.Info("started workers", "count", p.workerCount, "poll_interval", p.pollInterval)
}

// Wait blocks until all workers have finished.
func (p *Pool) Wait() {
	p.wg.Wait()
	p.logger.Info("all workers stopped")
}

// ActiveWorkers returns the current number of workers executing a task.
func (p *Pool) ActiveWorkers() int64 {
	return p.activeCount.Load()
}

func (p *Pool) worker(ctx context.Context, id int) {
	defer p.wg.Done()
	logger := p.logger.With("worker", id)
	logger.Info("worker started")

	for {
		select {
		case <-ctx.Done():
			logger.Info("worker shutting down")
			return
		default:
		}

		task, err := p.broker.Dequeue(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			logger.Error("worker dequeue error", "error", err)
			time.Sleep(p.pollInterval)
			continue
		}

		if task == nil {
			// Queue empty, wait before polling again
			time.Sleep(p.pollInterval)
			continue
		}

		p.activeCount.Add(1)
		p.executor.Execute(task, id)
		p.activeCount.Add(-1)
	}
}
