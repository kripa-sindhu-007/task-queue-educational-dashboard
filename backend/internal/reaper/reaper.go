// Package reaper implements the lease-expiry scanner that reclaims tasks from
// the processing ZSET when their visibility timeout expires (worker presumed
// dead). This is the safety net of at-least-once delivery.
package reaper

import (
	"context"
	_ "embed"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/model"
	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/store"
)

//go:embed scripts/reclaim.lua
var reclaimScript string

// Config holds reaper operational parameters.
type Config struct {
	Interval  time.Duration // how often the reaper scans (e.g. 5s)
	BatchSize int64         // max expired tasks to process per tick (e.g. 100)
}

// Reaper periodically scans the processing ZSET for expired leases and reclaims
// tasks back to the ready queue or dead-letters them.
type Reaper struct {
	client     *redis.Client
	tasks      *store.TaskStore
	deadLetter *store.DeadLetterStore
	events     *store.EventStore
	metrics    *store.MetricsStore
	cfg        Config
	reclaimCmd *redis.Script
}

func New(
	client *redis.Client,
	tasks *store.TaskStore,
	deadLetter *store.DeadLetterStore,
	events *store.EventStore,
	metrics *store.MetricsStore,
	cfg Config,
) *Reaper {
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 100
	}
	return &Reaper{
		client:     client,
		tasks:      tasks,
		deadLetter: deadLetter,
		events:     events,
		metrics:    metrics,
		cfg:        cfg,
		reclaimCmd: redis.NewScript(reclaimScript),
	}
}

// Start runs the reaper loop until ctx is cancelled.
func (r *Reaper) Start(ctx context.Context) {
	ticker := time.NewTicker(r.cfg.Interval)
	defer ticker.Stop()

	log.Printf("Reaper started (interval=%v, batch=%d)", r.cfg.Interval, r.cfg.BatchSize)

	for {
		select {
		case <-ctx.Done():
			log.Println("Reaper stopped")
			return
		case <-ticker.C:
			r.reap(ctx)
		}
	}
}

// reap scans for expired leases and reclaims them in bounded batches.
func (r *Reaper) reap(ctx context.Context) {
	now := time.Now().UnixMilli()

	// Find tasks whose lease deadline (score) is <= now — they've expired.
	ids, err := r.client.ZRangeByScore(ctx, store.KeyProcessing, &redis.ZRangeBy{
		Min:   "-inf",
		Max:   strconv.FormatInt(now, 10),
		Count: r.cfg.BatchSize,
	}).Result()
	if err != nil {
		log.Printf("Reaper: error scanning processing set: %v", err)
		return
	}

	if len(ids) == 0 {
		return
	}

	log.Printf("Reaper: found %d expired leases to reclaim", len(ids))

	for _, taskID := range ids {
		r.reclaimTask(ctx, taskID)
	}
}

// reclaimTask atomically reclaims a single task via Lua script.
func (r *Reaper) reclaimTask(ctx context.Context, taskID string) {
	// Load the task record to get maxRetries, retries, priority for the Lua script.
	task, err := r.tasks.Get(ctx, taskID)
	if err != nil {
		if err == store.ErrTaskNotFound {
			// Orphan in processing — just remove it.
			r.client.ZRem(ctx, store.KeyProcessing, taskID)
			log.Printf("Reaper: removed orphan %s from processing (no record)", taskID)
			return
		}
		log.Printf("Reaper: error loading task %s: %v", taskID, err)
		return
	}

	// The script reads retries/max_retries/priority from the hash itself, so it
	// is the source of truth for the retry-vs-dead-letter decision. We only pass
	// the task ID; the loaded record is used for logging and the DLQ entry.
	result, err := r.reclaimCmd.Run(ctx, r.client,
		[]string{store.KeyProcessing, store.KeyReady, store.TaskKey(taskID)},
		taskID,
	).Text()
	if err != nil {
		log.Printf("Reaper: reclaim script error for %s: %v", taskID, err)
		return
	}

	switch result {
	case "reclaimed":
		log.Printf("Reaper: reclaimed task %s (retries now %d/%d)", taskID, task.Retries+1, task.MaxRetries)
		r.emitEvent(ctx, taskID, "reclaimed",
			fmt.Sprintf("Lease expired, reclaimed (attempt %d/%d)", task.Retries+2, task.MaxRetries+1))
		if r.metrics != nil {
			r.metrics.IncrRetries(ctx)
		}

	case "dead_lettered":
		log.Printf("Reaper: dead-lettered task %s (retries exhausted)", taskID)
		// The Lua script already marked status=failed in the hash. Build the DLQ
		// entry from the record we already loaded (patching status) rather than
		// re-reading — a failed reload must not cause us to silently drop the task
		// from the dead-letter list, which would make it un-redriveable.
		task.Status = model.StatusFailed
		ft := model.FailedTask{
			Task:     task,
			FailedAt: time.Now(),
			Reason:   "lease expired, retries exhausted (reclaimed by reaper)",
		}
		if err := r.deadLetter.Push(ctx, ft); err != nil {
			log.Printf("Reaper: error pushing %s to DLQ: %v", taskID, err)
		}
		r.emitEvent(ctx, taskID, "dead_lettered",
			fmt.Sprintf("Lease expired, retries exhausted (%d/%d)", task.MaxRetries, task.MaxRetries))
		if r.metrics != nil {
			r.metrics.IncrFailed(ctx)
		}

	case "orphan":
		// Record vanished (flushed) after we claimed it; already removed from
		// processing by the script. Nothing else to do.
		log.Printf("Reaper: task %s had no record (orphan), removed from processing", taskID)

	case "lost_race":
		// Another reaper instance or the worker already handled it — expected.
		log.Printf("Reaper: lost race for task %s (already handled)", taskID)
	}
}

func (r *Reaper) emitEvent(ctx context.Context, taskID, eventType, detail string) {
	if r.events == nil {
		return
	}
	event := model.TaskEvent{
		ID:        fmt.Sprintf("evt-%d", time.Now().UnixNano()),
		TaskID:    taskID,
		Type:      eventType,
		WorkerID:  -1, // reaper, not a worker
		Detail:    detail,
		Timestamp: time.Now(),
	}
	if err := r.events.Push(ctx, event); err != nil {
		log.Printf("Reaper: error pushing event: %v", err)
	}
}
