package cron

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func newTestCronStore(t *testing.T) *CronStore {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { client.Close() })
	return NewCronStore(client)
}

func TestCronStoreCRUD(t *testing.T) {
	ctx := context.Background()
	s := newTestCronStore(t)

	job := CronJob{
		ID:        "cron-1",
		Schedule:  "*/10 * * * * *",
		Task:      TaskTemplate{Type: "sleep", Payload: json.RawMessage(`{"ms":1}`), Priority: 3, MaxRetries: 2},
		Enabled:   true,
		CreatedAt: time.Now().Truncate(time.Second),
	}

	// Get on missing → ErrCronNotFound.
	if _, err := s.Get(ctx, "cron-1"); err != ErrCronNotFound {
		t.Fatalf("expected ErrCronNotFound, got %v", err)
	}

	// Save + Get round-trips every field.
	if err := s.Save(ctx, job); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := s.Get(ctx, "cron-1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.ID != job.ID || got.Schedule != job.Schedule || got.Task.Type != "sleep" ||
		got.Task.Priority != 3 || got.Task.MaxRetries != 2 || !got.Enabled {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
	if string(got.Task.Payload) != `{"ms":1}` {
		t.Fatalf("payload mismatch: %s", got.Task.Payload)
	}

	// List returns it.
	jobs, err := s.List(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(jobs) != 1 || jobs[0].ID != "cron-1" {
		t.Fatalf("expected 1 job, got %+v", jobs)
	}

	// UpdateCursor advances only the cursor.
	if err := s.UpdateCursor(ctx, "cron-1", 12345); err != nil {
		t.Fatalf("update cursor: %v", err)
	}
	got, err = s.Get(ctx, "cron-1")
	if err != nil {
		t.Fatalf("get after cursor: %v", err)
	}
	if got.LastScheduledUnix != 12345 {
		t.Fatalf("cursor = %d, want 12345", got.LastScheduledUnix)
	}
	if got.Schedule != job.Schedule || !got.Enabled {
		t.Fatalf("cursor update clobbered other fields: %+v", got)
	}

	// Delete removes it; deleting again is a no-op.
	if err := s.Delete(ctx, "cron-1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := s.Get(ctx, "cron-1"); err != ErrCronNotFound {
		t.Fatalf("expected ErrCronNotFound after delete, got %v", err)
	}
	if err := s.Delete(ctx, "cron-1"); err != nil {
		t.Fatalf("delete missing should be no-op, got %v", err)
	}
}
