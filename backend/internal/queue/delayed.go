package queue

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/model"
	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/store"
)

// DelayedScheduler holds the "delayed" set. Members are task IDs; score = the
// unix timestamp at which the task should become ready.
type DelayedScheduler struct {
	client *redis.Client
	queue  *PriorityQueue
	tasks  *store.TaskStore
	events *store.EventStore
}

func NewDelayedScheduler(client *redis.Client, queue *PriorityQueue, tasks *store.TaskStore, events *store.EventStore) *DelayedScheduler {
	return &DelayedScheduler{client: client, queue: queue, tasks: tasks, events: events}
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

// Start polls the delayed set every second, moving due task IDs to the ready set.
func (d *DelayedScheduler) Start(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Delayed scheduler stopped")
			return
		case <-ticker.C:
			d.promoteDueTasks(ctx)
		}
	}
}

func (d *DelayedScheduler) promoteDueTasks(ctx context.Context) {
	now := float64(time.Now().Unix())
	ids, err := d.client.ZRangeByScore(ctx, store.KeyDelayed, &redis.ZRangeBy{
		Min:   "-inf",
		Max:   fmt.Sprintf("%f", now),
		Count: 100,
	}).Result()
	if err != nil {
		log.Printf("Error fetching delayed tasks: %v", err)
		return
	}

	for _, id := range ids {
		// ZREM acts as the claim: whoever removes it owns the promotion.
		removed, err := d.client.ZRem(ctx, store.KeyDelayed, id).Result()
		if err != nil || removed == 0 {
			continue // another instance grabbed it
		}
		task, err := d.tasks.Get(ctx, id)
		if err != nil {
			if err != store.ErrTaskNotFound {
				log.Printf("Error loading delayed task %s: %v", id, err)
			}
			continue
		}
		if err := d.queue.Enqueue(ctx, task); err != nil {
			log.Printf("Error promoting delayed task %s: %v", id, err)
			continue
		}
		log.Printf("Promoted delayed task %s to ready queue", id)
		if d.events != nil {
			event := model.TaskEvent{
				ID:        fmt.Sprintf("evt-%d", time.Now().UnixNano()),
				TaskID:    id,
				Type:      "promoted",
				WorkerID:  -1,
				Detail:    "Moved from delayed to ready queue",
				Timestamp: time.Now(),
			}
			if err := d.events.Push(ctx, event); err != nil {
				log.Printf("Error pushing promoted event: %v", err)
			}
		}
	}
}
