package queue

import (
	"context"
	_ "embed"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/model"
	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/store"
)

//go:embed scripts/signal.lua
var signalScript string

// DefaultSignalCap is the fallback doorbell-list bound used when a PriorityQueue
// is constructed with a non-positive cap (e.g. in tests). Production wires
// cfg.SignalCap.
const DefaultSignalCap = 1024

// PriorityQueue is the "ready" set. Members are task IDs (not full JSON);
// the canonical record lives in the TaskStore. Score = -priority so that
// higher-priority tasks pop first.
//
// P3.4: PriorityQueue also owns the ready doorbell — the signal list that idle
// workers block on (WaitReady) and that ready-producers ring (Signal). The
// doorbell is a pure wake-up optimization layered over the poll-correct design:
// tokens carry no task identity and are never read for correctness.
type PriorityQueue struct {
	client    *redis.Client
	tasks     *store.TaskStore
	signalCap int
	signalCmd *redis.Script
}

func NewPriorityQueue(client *redis.Client, tasks *store.TaskStore, signalCap int) *PriorityQueue {
	if signalCap <= 0 {
		signalCap = DefaultSignalCap
	}
	return &PriorityQueue{
		client:    client,
		tasks:     tasks,
		signalCap: signalCap,
		signalCmd: redis.NewScript(signalScript),
	}
}

// Enqueue adds a task ID to the ready set, then rings the doorbell so a blocked
// worker wakes and runs the atomic claim. The caller is responsible for having
// persisted the canonical record via the TaskStore first. A failure to ring the
// doorbell is non-fatal — the fallback poll (WaitReady's timeout) still picks the
// task up within SignalBlock — so a Signal error must not fail the Enqueue.
func (q *PriorityQueue) Enqueue(ctx context.Context, task model.Task) error {
	if err := q.client.ZAdd(ctx, store.KeyReady, redis.Z{
		Score:  float64(-task.Priority),
		Member: task.ID,
	}).Err(); err != nil {
		return err
	}
	// Best-effort doorbell: correctness never depends on the token arriving.
	_ = q.Signal(ctx)
	return nil
}

// Signal rings the ready doorbell via the shared cap-guarded signal.lua: it
// pushes at most one wake-up token, and only while the list is below SignalCap.
// Reused by the Go ready-producers (Enqueue / RedriveFailed); the batch Lua
// scripts (promote.lua / reclaim.lua) inline the same two lines to stay atomic
// with their ZADD ready.
func (q *PriorityQueue) Signal(ctx context.Context) error {
	return q.signalCmd.Run(ctx, q.client, []string{store.KeyReadySignal}, q.signalCap).Err()
}

// WaitReady blocks on the doorbell for up to timeout, returning true when a
// wake-up token arrived (a worker should immediately attempt the atomic claim)
// and false when the block timed out with no token (the fallback-poll backstop —
// the worker loops and re-claims anyway). A redis.Nil (BLPOP timeout) is the
// normal false path; any other error (including a cancelled context) is returned
// so the caller can distinguish shutdown from a real Redis fault.
func (q *PriorityQueue) WaitReady(ctx context.Context, timeout time.Duration) (bool, error) {
	res, err := q.client.BLPop(ctx, timeout, store.KeyReadySignal).Result()
	if err == redis.Nil {
		return false, nil // block timed out, no token — fall back to a poll
	}
	if err != nil {
		return false, err
	}
	return len(res) > 0, nil
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
