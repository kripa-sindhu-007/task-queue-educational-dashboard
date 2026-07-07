package queue

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"

	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/model"
	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/store"
)

// PriorityQueue is the "ready" set. Members are task IDs (not full JSON);
// the canonical record lives in the TaskStore. Score = -priority so that
// higher-priority tasks pop first.
type PriorityQueue struct {
	client *redis.Client
	tasks  *store.TaskStore
}

func NewPriorityQueue(client *redis.Client, tasks *store.TaskStore) *PriorityQueue {
	return &PriorityQueue{client: client, tasks: tasks}
}

// Enqueue adds a task ID to the ready set. The caller is responsible for having
// persisted the canonical record via the TaskStore first.
func (q *PriorityQueue) Enqueue(ctx context.Context, task model.Task) error {
	return q.client.ZAdd(ctx, store.KeyReady, redis.Z{
		Score:  float64(-task.Priority),
		Member: task.ID,
	}).Err()
}

// Dequeue pops the highest-priority task ID (lowest score) and hydrates its
// canonical record. Returns (nil, nil) when the ready set is empty.
func (q *PriorityQueue) Dequeue(ctx context.Context) (*model.Task, error) {
	results, err := q.client.ZPopMin(ctx, store.KeyReady, 1).Result()
	if err != nil {
		return nil, fmt.Errorf("zpopmin: %w", err)
	}
	if len(results) == 0 {
		return nil, nil // queue empty
	}
	id, ok := results[0].Member.(string)
	if !ok {
		return nil, fmt.Errorf("unexpected ready member type %T", results[0].Member)
	}
	task, err := q.tasks.Get(ctx, id)
	if err != nil {
		// Record vanished (e.g. flushed) after we popped its ID — nothing to run.
		if err == store.ErrTaskNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("hydrate task %s: %w", id, err)
	}
	return &task, nil
}

// Size returns the number of task IDs in the ready set.
func (q *PriorityQueue) Size(ctx context.Context) (int64, error) {
	return q.client.ZCard(ctx, store.KeyReady).Result()
}
