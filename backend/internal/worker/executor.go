package worker

import (
	"context"
	"fmt"
	"log"
	"math"
	"time"

	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/broker"
	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/handler"
	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/model"
	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/queue"
	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/store"
)

// DefaultDrainTimeout bounds how long post-work Redis writes may take once the
// server is shutting down. See ExecutorDeps.DrainTimeout.
const DefaultDrainTimeout = 5 * time.Second

// ExecutorDeps bundles the collaborators an Executor needs, replacing a long
// positional constructor (P0.8).
type ExecutorDeps struct {
	Broker       broker.Broker
	Handlers     *handler.Registry
	Tasks        *store.TaskStore
	Delayed      *queue.DelayedScheduler
	DeadLetter   *store.DeadLetterStore
	Metrics      *store.MetricsStore
	Events       *store.EventStore
	WorkerState  *store.WorkerStateStore
	DrainTimeout time.Duration
}

type Executor struct {
	deps ExecutorDeps
}

func NewExecutor(deps ExecutorDeps) *Executor {
	if deps.DrainTimeout <= 0 {
		deps.DrainTimeout = DefaultDrainTimeout
	}
	if deps.Handlers == nil {
		deps.Handlers = handler.NewDefaultRegistry()
	}
	return &Executor{deps: deps}
}

// BackoffDelay returns the retry delay for a given (post-increment) retry count:
// exponential 2^retries seconds, capped at 60s.
func BackoffDelay(retries int) time.Duration {
	backoff := math.Min(math.Pow(2, float64(retries)), 60)
	return time.Duration(backoff) * time.Second
}

// Execute runs the task and records its outcome. All Redis writes use a fresh
// drain context (background + timeout) rather than the worker's context, so
// completion/failure state is persisted even when the server is shutting down
// and the worker context has already been cancelled (P0.5).
//
// Phase 1: after work completes, the executor calls Ack (success) or Nack
// (failure) to release the lease from the processing set. If the lease was
// already reclaimed by the reaper (ErrLeaseNotHeld), the executor logs and
// skips further state changes — the task will be re-delivered.
func (e *Executor) Execute(task *model.Task, workerID int) {
	ctx, cancel := context.WithTimeout(context.Background(), e.deps.DrainTimeout)
	defer cancel()

	log.Printf("Executing task %s (priority=%d, attempt=%d/%d)",
		task.ID, task.Priority, task.Retries+1, task.MaxRetries+1)

	// Mark worker as processing (task record status already set by dequeue Lua script).
	if e.deps.WorkerState != nil {
		e.deps.WorkerState.Set(ctx, model.WorkerState{
			ID:        workerID,
			Status:    "processing",
			TaskID:    task.ID,
			StartedAt: time.Now(),
		})
	}
	e.emitEvent(ctx, task.ID, "started", workerID, fmt.Sprintf("Worker %d picked up task", workerID))

	// Dispatch to the handler registered for this task's Type (P2.4). The work
	// context (ctx) carries the drain timeout so a shutting-down worker doesn't
	// run forever; handlers are expected to honor cancellation.
	start := time.Now()
	result, workErr := e.deps.Handlers.Dispatch(ctx, *task)
	elapsed := time.Since(start)

	if workErr != nil {
		errMsg := workErr.Error()
		log.Printf("Task %s failed: %s", task.ID, errMsg)
		task.Error = errMsg
		e.emitEvent(ctx, task.ID, "failed", workerID, errMsg)

		// Release the lease via Nack. If the reaper already reclaimed it,
		// skip retry routing — the task will be re-delivered automatically.
		if err := e.deps.Broker.Nack(ctx, task.ID, true); err != nil {
			if err == broker.ErrLeaseNotHeld {
				log.Printf("Task %s: lease already reclaimed by reaper, skipping retry routing", task.ID)
			} else {
				log.Printf("Task %s: nack error: %v", task.ID, err)
			}
			// Either way, don't do retry routing — we don't own the task.
		} else {
			// We successfully released the lease — handle retry/DLQ routing.
			e.handleFailure(ctx, task, workerID)
		}
	} else {
		// Success path: Ack releases the lease and atomically marks completed.
		if err := e.deps.Broker.Ack(ctx, task.ID); err != nil {
			if err == broker.ErrLeaseNotHeld {
				log.Printf("Task %s: lease already reclaimed by reaper (completed work will be redone)", task.ID)
			} else {
				log.Printf("Task %s: ack error: %v", task.ID, err)
			}
			// Don't record completion — we lost the lease race.
		} else {
			// Ack succeeded — we own the completion. Record metrics/events.
			// Note: ack.lua already set status=completed in the hash.
			detail := fmt.Sprintf("Completed in %v", elapsed.Round(time.Millisecond))
			if result.Detail != "" {
				detail = fmt.Sprintf("%s (%s)", detail, result.Detail)
			}
			log.Printf("Task %s completed successfully", task.ID)
			e.emitEvent(ctx, task.ID, "completed", workerID, detail)
			if err := e.deps.Metrics.IncrProcessed(ctx); err != nil {
				log.Printf("Error incrementing processed metric: %v", err)
			}
		}
	}

	// Return worker to idle.
	if e.deps.WorkerState != nil {
		e.deps.WorkerState.Set(ctx, model.WorkerState{
			ID:     workerID,
			Status: "idle",
		})
	}
}

func (e *Executor) handleFailure(ctx context.Context, task *model.Task, workerID int) {
	if task.Retries < task.MaxRetries {
		// Retry with exponential backoff.
		task.Retries++
		task.Status = model.StatusPending
		delay := BackoffDelay(task.Retries)

		log.Printf("Retrying task %s in %v (attempt %d/%d)",
			task.ID, delay, task.Retries+1, task.MaxRetries+1)

		e.emitEvent(ctx, task.ID, "retrying", workerID,
			fmt.Sprintf("Retry %d/%d in %v", task.Retries, task.MaxRetries, delay))

		if err := e.deps.Tasks.Save(ctx, *task); err != nil {
			log.Printf("Error saving retry state for %s: %v", task.ID, err)
		}
		if err := e.deps.Metrics.IncrRetries(ctx); err != nil {
			log.Printf("Error incrementing retries metric: %v", err)
		}
		if err := e.deps.Delayed.Schedule(ctx, *task, delay); err != nil {
			log.Printf("Error scheduling retry for task %s: %v", task.ID, err)
		}
		return
	}

	// Exhausted retries — dead-letter.
	task.Status = model.StatusFailed
	log.Printf("Task %s exhausted retries, moving to dead-letter", task.ID)

	e.emitEvent(ctx, task.ID, "dead_lettered", workerID,
		fmt.Sprintf("Exhausted %d retries", task.MaxRetries))

	if err := e.deps.Tasks.Save(ctx, *task); err != nil {
		log.Printf("Error saving dead-letter state for %s: %v", task.ID, err)
	}
	ft := model.FailedTask{
		Task:     *task,
		FailedAt: time.Now(),
		Reason:   task.Error,
	}
	if err := e.deps.DeadLetter.Push(ctx, ft); err != nil {
		log.Printf("Error pushing to dead-letter: %v", err)
	}
	if err := e.deps.Metrics.IncrFailed(ctx); err != nil {
		log.Printf("Error incrementing failed metric: %v", err)
	}
}

func (e *Executor) emitEvent(ctx context.Context, taskID, eventType string, workerID int, detail string) {
	event := model.TaskEvent{
		ID:        fmt.Sprintf("evt-%d", time.Now().UnixNano()),
		TaskID:    taskID,
		Type:      eventType,
		WorkerID:  workerID,
		Detail:    detail,
		Timestamp: time.Now(),
	}
	if err := e.deps.Events.Push(ctx, event); err != nil {
		log.Printf("Error pushing event: %v", err)
	}
}
