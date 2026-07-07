package worker

import (
	"context"
	"fmt"
	"log"
	"math"
	"math/rand"
	"time"

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
func (e *Executor) Execute(task *model.Task, workerID int) {
	ctx, cancel := context.WithTimeout(context.Background(), e.deps.DrainTimeout)
	defer cancel()

	log.Printf("Executing task %s (priority=%d, attempt=%d/%d)",
		task.ID, task.Priority, task.Retries+1, task.MaxRetries+1)

	// Mark task + worker as processing.
	task.Status = model.StatusProcessing
	if err := e.deps.Tasks.Save(ctx, *task); err != nil {
		log.Printf("Error saving processing state for %s: %v", task.ID, err)
	}
	e.deps.WorkerState.Set(ctx, model.WorkerState{
		ID:        workerID,
		Status:    "processing",
		TaskID:    task.ID,
		StartedAt: time.Now(),
	})
	e.emitEvent(ctx, task.ID, "started", workerID, fmt.Sprintf("Worker %d picked up task", workerID))

	// Simulate work (200-800ms). Handlers keyed by task.Type arrive in P2.4.
	workDuration := time.Duration(200+rand.Intn(600)) * time.Millisecond
	time.Sleep(workDuration)

	// ~30% simulated failure rate.
	if rand.Float64() < 0.3 {
		errMsg := fmt.Sprintf("simulated failure for task %s", task.ID)
		log.Printf("Task %s failed: %s", task.ID, errMsg)
		task.Error = errMsg
		e.emitEvent(ctx, task.ID, "failed", workerID, errMsg)
		e.handleFailure(ctx, task, workerID)
	} else {
		task.Status = model.StatusCompleted
		task.Error = ""
		if err := e.deps.Tasks.Save(ctx, *task); err != nil {
			log.Printf("Error saving completed state for %s: %v", task.ID, err)
		}
		log.Printf("Task %s completed successfully", task.ID)
		e.emitEvent(ctx, task.ID, "completed", workerID, fmt.Sprintf("Completed in %v", workDuration))
		if err := e.deps.Metrics.IncrProcessed(ctx); err != nil {
			log.Printf("Error incrementing processed metric: %v", err)
		}
	}

	// Return worker to idle.
	e.deps.WorkerState.Set(ctx, model.WorkerState{
		ID:     workerID,
		Status: "idle",
	})
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
