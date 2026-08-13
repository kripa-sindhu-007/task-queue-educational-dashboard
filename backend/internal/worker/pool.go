package worker

import (
	"context"
	"errors"
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
	pollInterval time.Duration // P3.4: only the short back-off after a Dequeue *error* now
	signalBlock  time.Duration // P3.4: how long a worker blocks on the doorbell when the queue is empty
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
	signalBlock time.Duration,
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
		signalBlock:  signalBlock,
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
	p.logger.Info("started workers", "count", p.workerCount, "signal_block", p.signalBlock, "poll_interval", p.pollInterval)
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
			// Queue empty: block on the doorbell (P3.4) instead of sleep-polling.
			// WaitForReady returns when a producer rings the doorbell (a task may be
			// ready — loop and re-Dequeue) or after signalBlock with no token (the
			// fallback-poll backstop — loop and re-Dequeue anyway, so a missed token
			// costs at most signalBlock of latency and never a lost task). The
			// bounded block also caps shutdown latency: on ctx cancellation the block
			// returns within signalBlock (or immediately with context.Canceled), the
			// loop re-checks ctx.Done() at the top, and the worker exits cleanly.
			if err := p.broker.WaitForReady(ctx, p.signalBlock); err != nil {
				if ctx.Err() != nil || errors.Is(err, context.Canceled) {
					return // shutdown — clean exit, no error log
				}
				// A real Redis fault: log and fall back to the poll-interval back-off
				// so we don't hot-loop on a persistent error.
				logger.Error("worker wait-for-ready error", "error", err)
				time.Sleep(p.pollInterval)
			}
			continue
		}

		p.activeCount.Add(1)
		p.executor.Execute(task, id)
		p.activeCount.Add(-1)
	}
}
