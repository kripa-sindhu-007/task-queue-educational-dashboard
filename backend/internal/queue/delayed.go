package queue

import (
	"context"
	_ "embed"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/model"
	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/store"
)

//go:embed scripts/promote.lua
var promoteScript string

// DelayedScheduler holds the "delayed" set. Members are task IDs; score = the
// unix timestamp at which the task should become ready.
type DelayedScheduler struct {
	client     *redis.Client
	queue      *PriorityQueue
	tasks      *store.TaskStore
	events     *store.EventStore
	logger     *slog.Logger
	promoteCmd *redis.Script
}

func NewDelayedScheduler(client *redis.Client, queue *PriorityQueue, tasks *store.TaskStore, events *store.EventStore, logger *slog.Logger) *DelayedScheduler {
	if logger == nil {
		logger = slog.Default()
	}
	return &DelayedScheduler{
		client:     client,
		queue:      queue,
		tasks:      tasks,
		events:     events,
		logger:     logger,
		promoteCmd: redis.NewScript(promoteScript),
	}
}

// Schedule adds a task ID to the delayed set. The caller must have persisted the
// canonical record via the TaskStore first.
func (d *DelayedScheduler) Schedule(ctx context.Context, task model.Task, delay time.Duration) error {
	executeAt := time.Now().Add(delay).Unix()
	return d.client.ZAdd(ctx, store.KeyDelayed, redis.Z{
		Score:  float64(executeAt),
		Member: task.ID,
	}).Err()
}

// Start polls the delayed set every second, moving due task IDs to the ready set
// atomically via a Lua script (no loss window between ZREM and ZADD).
func (d *DelayedScheduler) Start(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			d.logger.Info("delayed scheduler stopped")
			return
		case <-ticker.C:
			d.promoteDueTasks(ctx)
		}
	}
}

func (d *DelayedScheduler) promoteDueTasks(ctx context.Context) {
	now := fmt.Sprintf("%d", time.Now().Unix())
	batchSize := int64(100)

	promoted, err := d.promoteCmd.Run(ctx, d.client,
		[]string{store.KeyDelayed, store.KeyReady, store.KeyTaskPrefix},
		now, batchSize,
	).Int64()
	if err != nil && err != redis.Nil {
		d.logger.Error("error promoting delayed tasks", "error", err)
		return
	}

	if promoted > 0 {
		d.logger.Info("promoted delayed tasks to ready queue", "count", promoted)
		// Emit events for promoted tasks. Since the Lua script handles the batch
		// atomically, we emit a summary event rather than per-task events to avoid
		// needing the ID list returned from Lua (keeping the script simple).
		if d.events != nil {
			event := model.TaskEvent{
				ID:        fmt.Sprintf("evt-%d", time.Now().UnixNano()),
				TaskID:    "",
				Type:      "promoted",
				WorkerID:  -1,
				Detail:    fmt.Sprintf("Promoted %d task(s) from delayed to ready", promoted),
				Timestamp: time.Now(),
			}
			d.events.Push(ctx, event)
		}
	}
}
