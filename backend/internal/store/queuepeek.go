package store

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"

	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/model"
)

// QueuePeekStore renders read-only snapshots of the ready/delayed sets for the
// dashboard. The sets hold task IDs, so it hydrates records via the TaskStore.
type QueuePeekStore struct {
	client *redis.Client
	tasks  *TaskStore
}

func NewQueuePeekStore(client *redis.Client, tasks *TaskStore) *QueuePeekStore {
	return &QueuePeekStore{client: client, tasks: tasks}
}

// PeekReady returns up to limit tasks from the ready set without removing them.
func (q *QueuePeekStore) PeekReady(ctx context.Context, limit int64) ([]model.Task, error) {
	if limit <= 0 {
		limit = 20
	}
	ids, err := q.client.ZRange(ctx, KeyReady, 0, limit-1).Result()
	if err != nil {
		return nil, fmt.Errorf("zrange ready: %w", err)
	}
	return q.tasks.GetMany(ctx, ids)
}

// PeekDelayed returns up to limit delayed tasks with their execute_at timestamps.
func (q *QueuePeekStore) PeekDelayed(ctx context.Context, limit int64) ([]model.DelayedEntry, error) {
	if limit <= 0 {
		limit = 20
	}
	results, err := q.client.ZRangeWithScores(ctx, KeyDelayed, 0, limit-1).Result()
	if err != nil {
		return nil, fmt.Errorf("zrange delayed: %w", err)
	}
	entries := make([]model.DelayedEntry, 0, len(results))
	for _, z := range results {
		id, ok := z.Member.(string)
		if !ok {
			continue
		}
		task, err := q.tasks.Get(ctx, id)
		if err != nil {
			continue // record gone; skip from the snapshot
		}
		entries = append(entries, model.DelayedEntry{
			Task:      task,
			ExecuteAt: z.Score,
		})
	}
	return entries, nil
}

// ReadySize returns the number of tasks in the ready set (the queue depth).
func (q *QueuePeekStore) ReadySize(ctx context.Context) (int64, error) {
	return q.client.ZCard(ctx, KeyReady).Result()
}

// DelayedSize returns the number of tasks in the delayed set.
func (q *QueuePeekStore) DelayedSize(ctx context.Context) (int64, error) {
	return q.client.ZCard(ctx, KeyDelayed).Result()
}

// DeadLetterSize returns the number of tasks in the dead-letter queue.
func (q *QueuePeekStore) DeadLetterSize(ctx context.Context) (int64, error) {
	return q.client.LLen(ctx, KeyDeadLetter).Result()
}

// ProcessingSize returns the number of tasks currently leased (in the processing
// ZSET). This is the cluster-wide count of workers actively executing a task:
// the dequeue Lua inserts every leased task into KeyProcessing and the owning
// node's tasks SET in one step, so ZCARD(processing) equals the sum of per-node
// in-flight counts by construction — and is correct in both single-binary and
// distributed modes. It is the single source of truth for "active workers".
func (q *QueuePeekStore) ProcessingSize(ctx context.Context) (int64, error) {
	return q.client.ZCard(ctx, KeyProcessing).Result()
}
