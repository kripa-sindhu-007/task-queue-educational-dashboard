package cron

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/model"
	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/queue"
	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/store"
)

type matDeps struct {
	crons  *CronStore
	tasks  *store.TaskStore
	queue  *queue.PriorityQueue
	client *redis.Client
}

func newMatDeps(t *testing.T) matDeps {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { client.Close() })

	tasks := store.NewTaskStore(client)
	return matDeps{
		crons:  NewCronStore(client),
		tasks:  tasks,
		queue:  queue.NewPriorityQueue(client, tasks, queue.DefaultSignalCap),
		client: client,
	}
}

// newMaterializer builds a materializer wired to the deps, with events + metrics
// stores exercised and a fixed injected clock supplied per-call to materialize.
func (d matDeps) newMaterializer() *CronMaterializer {
	return NewCronMaterializer(
		d.crons, d.tasks, d.queue,
		store.NewEventStore(d.client), store.NewMetricsStore(d.client),
		time.Second, nil, nil,
	)
}

// readyMembers returns the task IDs currently in the ready set.
func (d matDeps) readyMembers(t *testing.T) []string {
	t.Helper()
	ids, err := d.client.ZRange(context.Background(), store.KeyReady, 0, -1).Result()
	if err != nil {
		t.Fatalf("zrange ready: %v", err)
	}
	return ids
}

func contains(ids []string, want string) bool {
	for _, id := range ids {
		if id == want {
			return true
		}
	}
	return false
}

// (a) exactly-once per slot: a */10s job driven second-by-second across a 35s
// window fires exactly one instance per 10s slot, with deterministic IDs.
func TestMaterialize_ExactlyOncePerSlot(t *testing.T) {
	ctx := context.Background()
	d := newMatDeps(t)
	m := d.newMaterializer()

	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	job := CronJob{ID: "cron-a", Schedule: "*/10 * * * * *", Task: TaskTemplate{Type: "sleep"}, Enabled: true, CreatedAt: t0}
	if err := d.crons.Save(ctx, job); err != nil {
		t.Fatalf("save job: %v", err)
	}

	// Drive one tick per simulated second across (t0, t0+35s].
	for sec := 0; sec <= 35; sec++ {
		m.materialize(ctx, t0.Add(time.Duration(sec)*time.Second))
	}

	ready := d.readyMembers(t)
	// Slots strictly after CreatedAt and <= t0+35s: +10, +20, +30 → 3 instances.
	wantSlots := []int{10, 20, 30}
	if len(ready) != len(wantSlots) {
		t.Fatalf("expected %d instances, got %d: %v", len(wantSlots), len(ready), ready)
	}
	for _, s := range wantSlots {
		id := "cron-a-" + itoa(t0.Add(time.Duration(s)*time.Second).Unix())
		if !contains(ready, id) {
			t.Fatalf("missing expected instance %q in %v", id, ready)
		}
		exists, err := d.tasks.Exists(ctx, id)
		if err != nil || !exists {
			t.Fatalf("expected task record for %q (exists=%v err=%v)", id, exists, err)
		}
	}
	// The slot at t0 itself (CreatedAt boundary is exclusive) must NOT fire.
	if contains(ready, "cron-a-"+itoa(t0.Unix())) {
		t.Fatalf("boundary slot at CreatedAt should not fire: %v", ready)
	}
}

// (b) idempotent re-tick: calling materialize twice at the same now never
// double-enqueues a slot.
func TestMaterialize_IdempotentReTick(t *testing.T) {
	ctx := context.Background()
	d := newMatDeps(t)
	m := d.newMaterializer()

	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	job := CronJob{ID: "cron-b", Schedule: "*/10 * * * * *", Task: TaskTemplate{Type: "sleep"}, Enabled: true, CreatedAt: t0}
	if err := d.crons.Save(ctx, job); err != nil {
		t.Fatalf("save job: %v", err)
	}

	now := t0.Add(10 * time.Second)
	m.materialize(ctx, now)
	m.materialize(ctx, now) // re-tick at the same instant

	ready := d.readyMembers(t)
	if len(ready) != 1 {
		t.Fatalf("expected exactly 1 instance after a re-tick, got %d: %v", len(ready), ready)
	}
}

// (c) handover: materializer A fires a slot (Save+Enqueue) but crashes before
// UpdateCursor. A fresh materializer B on the same store (cursor unchanged)
// recomputes the same slot, sees the record via Exists, and does NOT re-enqueue.
func TestMaterialize_HandoverNoDoubleFire(t *testing.T) {
	ctx := context.Background()
	d := newMatDeps(t)

	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	job := CronJob{ID: "cron-c", Schedule: "*/10 * * * * *", Task: TaskTemplate{Type: "sleep"}, Enabled: true, CreatedAt: t0}
	if err := d.crons.Save(ctx, job); err != nil {
		t.Fatalf("save job: %v", err)
	}

	now := t0.Add(10 * time.Second)
	slotID := "cron-c-" + itoa(now.Unix())

	// Simulate leader A: fired the slot (Save + Enqueue) but died before advancing
	// the cursor — the cron record's LastScheduledUnix stays 0.
	task := model.Task{ID: slotID, Type: "sleep", Status: model.StatusPending, CreatedAt: now}
	if err := d.tasks.Save(ctx, task); err != nil {
		t.Fatalf("A save: %v", err)
	}
	if err := d.queue.Enqueue(ctx, task); err != nil {
		t.Fatalf("A enqueue: %v", err)
	}

	// Fresh leader B on the same store, cursor still 0.
	b := d.newMaterializer()
	b.materialize(ctx, now)

	ready := d.readyMembers(t)
	if len(ready) != 1 || ready[0] != slotID {
		t.Fatalf("handover re-enqueued the slot: ready=%v", ready)
	}
	// B should still have advanced the cursor past the deduped slot.
	got, err := d.crons.Get(ctx, "cron-c")
	if err != nil {
		t.Fatalf("get job: %v", err)
	}
	if got.LastScheduledUnix != now.Unix() {
		t.Fatalf("expected cursor advanced to %d, got %d", now.Unix(), got.LastScheduledUnix)
	}
}

// (d) catch-up = skip-to-latest: a cursor far in the past collapses to exactly
// ONE fire (the latest due slot), not one per missed slot.
func TestMaterialize_CatchUpSkipsToLatest(t *testing.T) {
	ctx := context.Background()
	d := newMatDeps(t)
	m := d.newMaterializer()

	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	// CreatedAt an hour ago: 360 */10s slots are "missed" between then and now.
	job := CronJob{ID: "cron-d", Schedule: "*/10 * * * * *", Task: TaskTemplate{Type: "sleep"}, Enabled: true, CreatedAt: t0.Add(-time.Hour)}
	if err := d.crons.Save(ctx, job); err != nil {
		t.Fatalf("save job: %v", err)
	}

	m.materialize(ctx, t0)

	ready := d.readyMembers(t)
	if len(ready) != 1 {
		t.Fatalf("catch-up should fire exactly once, got %d: %v", len(ready), ready)
	}
	// The single fire is the latest due slot (the slot at t0 itself).
	wantID := "cron-d-" + itoa(t0.Unix())
	if ready[0] != wantID {
		t.Fatalf("expected latest slot %q, got %q", wantID, ready[0])
	}
}

// itoa formats a unix second the same way fmt "%d" does in the materializer, so
// tests build the expected instance IDs identically.
func itoa(n int64) string {
	return strconv.FormatInt(n, 10)
}
