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
	Interval        time.Duration // how often the reaper scans (e.g. 5s)
	BatchSize       int64         // max expired tasks to process per tick (e.g. 100)
	NodeGraceWindow time.Duration // how long a dead node stays visible (alive:false) before it's pruned
}

// Reaper periodically scans for (a) dead nodes, eagerly reclaiming all of their
// in-flight work, and (b) individually expired leases. Together these guarantee
// at-least-once delivery: no task is stranded whether a whole node dies or a
// single lease lapses.
type Reaper struct {
	client     *redis.Client
	tasks      *store.TaskStore
	deadLetter *store.DeadLetterStore
	events     *store.EventStore
	metrics    *store.MetricsStore
	nodes      *store.NodeStore
	cfg        Config
	reclaimCmd *redis.Script
}

func New(
	client *redis.Client,
	tasks *store.TaskStore,
	deadLetter *store.DeadLetterStore,
	events *store.EventStore,
	metrics *store.MetricsStore,
	nodes *store.NodeStore,
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
		nodes:      nodes,
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
			r.reapDeadNodes(ctx)
			r.reap(ctx)
		}
	}
}

// reapDeadNodes detects registered nodes whose heartbeat has expired and eagerly
// reclaims every task they hold in flight — without waiting for each individual
// lease to expire. This is what makes the "kill a worker, watch it recover"
// demo fast: recovery is bounded by the heartbeat TTL, not the visibility
// timeout.
//
// A dead node is not pruned immediately. On first detection (MarkDead SET NX
// wins) the reaper reclaims the node's in-flight tasks and emits node_dead
// exactly once; the node then stays in the registry as alive:false — with its
// metadata intact from the :meta descriptor — until NodeGraceWindow elapses, so
// the dashboard can show it dying and recovering its tasks before it disappears.
// A stalled node whose heartbeat returns clears its own tombstone (in Heartbeat)
// and drops out of DeadNodeIDs, so it is never pruned.
func (r *Reaper) reapDeadNodes(ctx context.Context) {
	if r.nodes == nil {
		return
	}
	dead, err := r.nodes.DeadNodeIDs(ctx)
	if err != nil {
		log.Printf("Reaper: error scanning for dead nodes: %v", err)
		return
	}
	now := time.Now()
	for _, nodeID := range dead {
		firstTime, err := r.nodes.MarkDead(ctx, nodeID)
		if err != nil {
			log.Printf("Reaper: error marking node %s dead: %v", nodeID, err)
			continue
		}

		if firstTime {
			// First observation of this death: reclaim its work and announce it once.
			taskIDs, err := r.nodes.OwnedTaskIDs(ctx, nodeID)
			if err != nil {
				log.Printf("Reaper: error listing tasks for dead node %s: %v", nodeID, err)
				continue
			}
			if len(taskIDs) > 0 {
				log.Printf("Reaper: node %s is dead — reclaiming %d in-flight task(s)", nodeID, len(taskIDs))
				r.emitEvent(ctx, "", "node_dead",
					fmt.Sprintf("Node %s heartbeat expired; reclaiming %d task(s)", nodeID, len(taskIDs)))
				for _, taskID := range taskIDs {
					r.reclaimTask(ctx, taskID)
				}
			} else {
				r.emitEvent(ctx, "", "node_dead", fmt.Sprintf("Node %s heartbeat expired (no in-flight tasks)", nodeID))
			}
			continue
		}

		// Already reclaimed on a prior tick: prune only once the grace window has
		// elapsed so the DEAD card stays visible for a while.
		deadSince, ok, err := r.nodes.DeadSince(ctx, nodeID)
		if err != nil {
			log.Printf("Reaper: error reading dead-since for node %s: %v", nodeID, err)
			continue
		}
		if !ok || now.Sub(deadSince) >= r.cfg.NodeGraceWindow {
			if err := r.nodes.Remove(ctx, nodeID); err != nil {
				log.Printf("Reaper: error removing dead node %s: %v", nodeID, err)
			} else {
				log.Printf("Reaper: pruned dead node %s after grace window", nodeID)
			}
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
	// is the source of truth for the retry-vs-dead-letter decision. We pass the
	// task ID and the owning node's task set (from the record we loaded) so the
	// reclaim also clears the task from that node's in-flight set.
	result, err := r.reclaimCmd.Run(ctx, r.client,
		[]string{store.KeyProcessing, store.KeyReady, store.TaskKey(taskID), store.NodeTasksKey(task.Owner)},
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
