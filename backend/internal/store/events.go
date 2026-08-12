package store

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/model"
	"github.com/redis/go-redis/v9"
)

type EventStore struct {
	client *redis.Client
}

func NewEventStore(client *redis.Client) *EventStore {
	return &EventStore{client: client}
}

// clusterEventTypes are the rare lifecycle events mirrored into a second
// retained list so a burst of started/completed events can't evict them from the
// main 200-entry stream.
var clusterEventTypes = map[string]bool{
	"node_joined": true,
	"node_dead":   true,
	"reclaimed":   true,
}

// Push appends an event and trims the list to 200 entries. Lifecycle events are
// additionally mirrored into KeyEventsCluster (its own retained list) so they
// survive high-throughput bursts — this is the single choke point, so all
// existing callers get the mirror for free.
func (e *EventStore) Push(ctx context.Context, event model.TaskEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}
	pipe := e.client.Pipeline()
	pipe.LPush(ctx, KeyEvents, string(data))
	pipe.LTrim(ctx, KeyEvents, 0, 199)
	if clusterEventTypes[event.Type] {
		pipe.LPush(ctx, KeyEventsCluster, string(data))
		pipe.LTrim(ctx, KeyEventsCluster, 0, 199)
	}
	_, err = pipe.Exec(ctx)
	return err
}

// List returns the most recent events (newest first).
func (e *EventStore) List(ctx context.Context, limit int64) ([]model.TaskEvent, error) {
	return e.list(ctx, KeyEvents, limit)
}

// ListCluster returns the most recent cluster lifecycle events (newest first)
// from the retained cluster list.
func (e *EventStore) ListCluster(ctx context.Context, limit int64) ([]model.TaskEvent, error) {
	return e.list(ctx, KeyEventsCluster, limit)
}

func (e *EventStore) list(ctx context.Context, key string, limit int64) ([]model.TaskEvent, error) {
	if limit <= 0 {
		limit = 50
	}
	results, err := e.client.LRange(ctx, key, 0, limit-1).Result()
	if err != nil {
		return nil, fmt.Errorf("lrange %s: %w", key, err)
	}
	events := make([]model.TaskEvent, 0, len(results))
	for _, r := range results {
		var ev model.TaskEvent
		if err := json.Unmarshal([]byte(r), &ev); err != nil {
			continue
		}
		events = append(events, ev)
	}
	return events, nil
}
