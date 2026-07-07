package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/model"
	"github.com/redis/go-redis/v9"
)

type WorkerStateStore struct {
	client *redis.Client
}

func NewWorkerStateStore(client *redis.Client) *WorkerStateStore {
	return &WorkerStateStore{client: client}
}

// Set updates a single worker's state in the hash.
func (w *WorkerStateStore) Set(ctx context.Context, state model.WorkerState) error {
	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("marshal worker state: %w", err)
	}
	return w.client.HSet(ctx, KeyWorkers, strconv.Itoa(state.ID), string(data)).Err()
}

// WorkerIdleState creates an idle WorkerState for initialization.
func WorkerIdleState(id int) model.WorkerState {
	return model.WorkerState{ID: id, Status: "idle"}
}

// GetAll returns all worker states.
func (w *WorkerStateStore) GetAll(ctx context.Context) ([]model.WorkerState, error) {
	vals, err := w.client.HGetAll(ctx, KeyWorkers).Result()
	if err != nil {
		return nil, fmt.Errorf("hgetall workers: %w", err)
	}
	states := make([]model.WorkerState, 0, len(vals))
	for _, v := range vals {
		var ws model.WorkerState
		if err := json.Unmarshal([]byte(v), &ws); err != nil {
			continue
		}
		states = append(states, ws)
	}
	return states, nil
}
