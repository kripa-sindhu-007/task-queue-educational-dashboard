package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/model"
)

// KeyTaskPrefix is the prefix for canonical per-task record hashes.
const KeyTaskPrefix = "taskqueue:task:"

// TaskKey returns the Redis key holding a task's canonical record hash.
func TaskKey(id string) string { return KeyTaskPrefix + id }

// ErrTaskNotFound is returned by Get when no record exists for the given id.
var ErrTaskNotFound = errors.New("task not found")

// TaskStore is the source of truth for task state. Each task lives in a Redis
// hash at taskqueue:task:{id}; the ready/delayed/processing sets hold only IDs,
// which fixes the old identity bug where two distinct tasks that happened to
// serialize identically collided as the same ZSET member.
type TaskStore struct {
	client *redis.Client
}

func NewTaskStore(client *redis.Client) *TaskStore {
	return &TaskStore{client: client}
}

func taskToHash(t model.Task) map[string]any {
	return map[string]any{
		"id":          t.ID,
		"type":        t.Type,
		"payload":     string(t.Payload),
		"priority":    t.Priority,
		"delay":       t.Delay,
		"max_retries": t.MaxRetries,
		"retries":     t.Retries,
		"status":      string(t.Status),
		"created_at":  t.CreatedAt.UTC().Format(time.RFC3339Nano),
		"error":       t.Error,
		"owner":       t.Owner,
	}
}

func hashToTask(v map[string]string) model.Task {
	priority, _ := strconv.Atoi(v["priority"])
	delay, _ := strconv.Atoi(v["delay"])
	maxRetries, _ := strconv.Atoi(v["max_retries"])
	retries, _ := strconv.Atoi(v["retries"])
	createdAt, _ := time.Parse(time.RFC3339Nano, v["created_at"])

	var payload json.RawMessage
	if p := v["payload"]; p != "" {
		payload = json.RawMessage(p)
	}

	return model.Task{
		ID:         v["id"],
		Type:       v["type"],
		Payload:    payload,
		Priority:   priority,
		Delay:      delay,
		MaxRetries: maxRetries,
		Retries:    retries,
		Status:     model.TaskStatus(v["status"]),
		CreatedAt:  createdAt,
		Error:      v["error"],
		Owner:      v["owner"],
	}
}

// Save writes (creates or fully overwrites) a task's canonical record.
func (s *TaskStore) Save(ctx context.Context, t model.Task) error {
	if err := s.client.HSet(ctx, TaskKey(t.ID), taskToHash(t)).Err(); err != nil {
		return fmt.Errorf("save task %s: %w", t.ID, err)
	}
	return nil
}

// Get loads a task record by id, returning ErrTaskNotFound if it does not exist.
func (s *TaskStore) Get(ctx context.Context, id string) (model.Task, error) {
	vals, err := s.client.HGetAll(ctx, TaskKey(id)).Result()
	if err != nil {
		return model.Task{}, fmt.Errorf("get task %s: %w", id, err)
	}
	if len(vals) == 0 {
		return model.Task{}, ErrTaskNotFound
	}
	return hashToTask(vals), nil
}

// Exists returns true if a task record with the given ID already exists.
// Used for idempotent enqueue: reject duplicate client-supplied IDs (P1.6).
func (s *TaskStore) Exists(ctx context.Context, id string) (bool, error) {
	n, err := s.client.Exists(ctx, TaskKey(id)).Result()
	if err != nil {
		return false, fmt.Errorf("exists task %s: %w", id, err)
	}
	return n > 0, nil
}

// GetMany loads several task records in a single pipeline, skipping any that are
// missing (e.g. flushed or expired between listing IDs and hydrating them).
func (s *TaskStore) GetMany(ctx context.Context, ids []string) ([]model.Task, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	pipe := s.client.Pipeline()
	cmds := make([]*redis.MapStringStringCmd, len(ids))
	for i, id := range ids {
		cmds[i] = pipe.HGetAll(ctx, TaskKey(id))
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return nil, fmt.Errorf("getmany tasks: %w", err)
	}
	tasks := make([]model.Task, 0, len(ids))
	for _, cmd := range cmds {
		vals := cmd.Val()
		if len(vals) == 0 {
			continue
		}
		tasks = append(tasks, hashToTask(vals))
	}
	return tasks, nil
}
