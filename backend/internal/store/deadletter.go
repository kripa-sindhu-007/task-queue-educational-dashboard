package store

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/model"
	"github.com/redis/go-redis/v9"
)

type DeadLetterStore struct {
	client *redis.Client
}

func NewDeadLetterStore(client *redis.Client) *DeadLetterStore {
	return &DeadLetterStore{client: client}
}

// Push adds a failed task to the dead-letter list.
func (d *DeadLetterStore) Push(ctx context.Context, ft model.FailedTask) error {
	data, err := json.Marshal(ft)
	if err != nil {
		return fmt.Errorf("marshal failed task: %w", err)
	}
	return d.client.LPush(ctx, KeyDeadLetter, string(data)).Err()
}

// List returns a paginated slice of failed tasks (newest first).
func (d *DeadLetterStore) List(ctx context.Context, offset, limit int64) ([]model.FailedTask, error) {
	results, err := d.client.LRange(ctx, KeyDeadLetter, offset, offset+limit-1).Result()
	if err != nil {
		return nil, fmt.Errorf("lrange deadletter: %w", err)
	}
	tasks := make([]model.FailedTask, 0, len(results))
	for _, r := range results {
		var ft model.FailedTask
		if err := json.Unmarshal([]byte(r), &ft); err != nil {
			continue
		}
		tasks = append(tasks, ft)
	}
	return tasks, nil
}
