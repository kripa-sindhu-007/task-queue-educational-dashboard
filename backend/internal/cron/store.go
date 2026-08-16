package cron

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/redis/go-redis/v9"

	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/store"
)

// ErrCronNotFound is returned by Get when no job exists for the given id.
var ErrCronNotFound = errors.New("cron job not found")

// CronStore persists cron jobs in a single Redis hash (taskqueue:cron), keyed by
// job id → JSON(CronJob). It follows the store/*.go conventions: a plain redis
// client, context-first methods, and wrapped errors.
type CronStore struct {
	client *redis.Client
}

func NewCronStore(client *redis.Client) *CronStore {
	return &CronStore{client: client}
}

// Save creates or fully overwrites a cron job's record.
func (s *CronStore) Save(ctx context.Context, job CronJob) error {
	b, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("marshal cron %s: %w", job.ID, err)
	}
	if err := s.client.HSet(ctx, store.KeyCron, job.ID, b).Err(); err != nil {
		return fmt.Errorf("save cron %s: %w", job.ID, err)
	}
	return nil
}

// List returns all stored cron jobs (order unspecified).
func (s *CronStore) List(ctx context.Context) ([]CronJob, error) {
	vals, err := s.client.HGetAll(ctx, store.KeyCron).Result()
	if err != nil {
		return nil, fmt.Errorf("list cron: %w", err)
	}
	jobs := make([]CronJob, 0, len(vals))
	for _, v := range vals {
		var job CronJob
		if err := json.Unmarshal([]byte(v), &job); err != nil {
			return nil, fmt.Errorf("unmarshal cron: %w", err)
		}
		jobs = append(jobs, job)
	}
	return jobs, nil
}

// Get loads a single cron job by id, returning ErrCronNotFound if it is absent.
func (s *CronStore) Get(ctx context.Context, id string) (CronJob, error) {
	v, err := s.client.HGet(ctx, store.KeyCron, id).Result()
	if err == redis.Nil {
		return CronJob{}, ErrCronNotFound
	}
	if err != nil {
		return CronJob{}, fmt.Errorf("get cron %s: %w", id, err)
	}
	var job CronJob
	if err := json.Unmarshal([]byte(v), &job); err != nil {
		return CronJob{}, fmt.Errorf("unmarshal cron %s: %w", id, err)
	}
	return job, nil
}

// Delete removes a cron job. Deleting a missing id is a no-op (no error).
func (s *CronStore) Delete(ctx context.Context, id string) error {
	if err := s.client.HDel(ctx, store.KeyCron, id).Err(); err != nil {
		return fmt.Errorf("delete cron %s: %w", id, err)
	}
	return nil
}

// UpdateCursor advances a job's LastScheduledUnix cursor to unix, leaving the
// rest of the record intact. Only the leader-gated materializer calls this, and
// only after a slot's task has been enqueued.
func (s *CronStore) UpdateCursor(ctx context.Context, id string, unix int64) error {
	job, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	job.LastScheduledUnix = unix
	return s.Save(ctx, job)
}
