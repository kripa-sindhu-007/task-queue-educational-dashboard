package store

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/model"
)

func newTestClient(t *testing.T) *redis.Client {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	return redis.NewClient(&redis.Options{Addr: mr.Addr()})
}

func TestTaskStoreSaveGetRoundTrip(t *testing.T) {
	ctx := context.Background()
	tasks := NewTaskStore(newTestClient(t))

	want := model.Task{
		ID:         "task-1",
		Type:       "http_fetch",
		Payload:    json.RawMessage(`{"url":"https://example.com"}`),
		Priority:   7,
		Delay:      3,
		MaxRetries: 2,
		Retries:    1,
		Status:     model.StatusProcessing,
		CreatedAt:  time.Now().UTC().Truncate(time.Second),
		Error:      "boom",
	}
	if err := tasks.Save(ctx, want); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := tasks.Get(ctx, "task-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ID != want.ID || got.Type != want.Type || got.Priority != want.Priority ||
		got.MaxRetries != want.MaxRetries || got.Retries != want.Retries ||
		got.Status != want.Status || got.Error != want.Error {
		t.Fatalf("round-trip mismatch:\n got=%+v\nwant=%+v", got, want)
	}
	if string(got.Payload) != string(want.Payload) {
		t.Fatalf("payload mismatch: got %q want %q", got.Payload, want.Payload)
	}
	if !got.CreatedAt.Equal(want.CreatedAt) {
		t.Fatalf("created_at mismatch: got %v want %v", got.CreatedAt, want.CreatedAt)
	}
}

func TestTaskStoreGetMissing(t *testing.T) {
	tasks := NewTaskStore(newTestClient(t))
	if _, err := tasks.Get(context.Background(), "nope"); err != ErrTaskNotFound {
		t.Fatalf("expected ErrTaskNotFound, got %v", err)
	}
}

func TestTaskStoreGetManySkipsMissing(t *testing.T) {
	ctx := context.Background()
	tasks := NewTaskStore(newTestClient(t))
	for _, id := range []string{"a", "c"} {
		if err := tasks.Save(ctx, model.Task{ID: id, Status: model.StatusPending}); err != nil {
			t.Fatalf("Save %s: %v", id, err)
		}
	}
	got, err := tasks.GetMany(ctx, []string{"a", "b", "c"}) // "b" never saved
	if err != nil {
		t.Fatalf("GetMany: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 tasks (b skipped), got %d", len(got))
	}
}
