package worker

import (
	"context"
	"log"
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
}

func NewPool(
	b broker.Broker,
	executor *Executor,
	workerCount int,
	pollInterval time.Duration,
	workerState *store.WorkerStateStore,
) *Pool {
	return &Pool{
		broker:       b,
		executor:     executor,
		workerCount:  workerCount,
		pollInterval: pollInterval,
		workerState:  workerState,
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
	log.Printf("Started %d workers (poll interval: %v)", p.workerCount, p.pollInterval)
}

// Wait blocks until all workers have finished.
func (p *Pool) Wait() {
	p.wg.Wait()
	log.Println("All workers stopped")
}

// ActiveWorkers returns the current number of workers executing a task.
func (p *Pool) ActiveWorkers() int64 {
	return p.activeCount.Load()
}

func (p *Pool) worker(ctx context.Context, id int) {
	defer p.wg.Done()
	log.Printf("Worker %d started", id)

	for {
		select {
		case <-ctx.Done():
			log.Printf("Worker %d shutting down", id)
			return
		default:
		}

		task, err := p.broker.Dequeue(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("Worker %d dequeue error: %v", id, err)
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
