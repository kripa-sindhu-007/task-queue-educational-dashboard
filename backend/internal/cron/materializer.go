package cron

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	robfigcron "github.com/robfig/cron/v3"

	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/model"
	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/queue"
	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/store"
)

// CronMaterializer is the leader-gated loop that turns due cron slots into
// ordinary tasks. Like the delayed scheduler and reaper it takes concrete stores
// (not an HTTP client) and enqueues on the same path SubmitTask uses. The clock
// is injected (now) so tests drive materialize directly without real sleeps.
type CronMaterializer struct {
	crons   *CronStore
	tasks   *store.TaskStore
	queue   *queue.PriorityQueue
	events  *store.EventStore
	metrics *store.MetricsStore
	tick    time.Duration
	now     func() time.Time
	logger  *slog.Logger

	// parsed caches compiled schedules by spec string so a busy tick does not
	// reparse every job every second.
	parsed map[string]robfigcron.Schedule
}

// NewCronMaterializer builds a materializer. A nil now defaults to time.Now and a
// nil logger falls back to slog.Default(). tick is the scan cadence for Start.
func NewCronMaterializer(
	crons *CronStore,
	tasks *store.TaskStore,
	q *queue.PriorityQueue,
	events *store.EventStore,
	metrics *store.MetricsStore,
	tick time.Duration,
	now func() time.Time,
	logger *slog.Logger,
) *CronMaterializer {
	if now == nil {
		now = time.Now
	}
	if logger == nil {
		logger = slog.Default()
	}
	if tick <= 0 {
		tick = time.Second
	}
	return &CronMaterializer{
		crons:   crons,
		tasks:   tasks,
		queue:   q,
		events:  events,
		metrics: metrics,
		tick:    tick,
		now:     now,
		logger:  logger,
		parsed:  make(map[string]robfigcron.Schedule),
	}
}

// Start scans for due schedules every tick until ctx is cancelled. It mirrors
// delayed.go: the loop is split from the logic (materialize) so tests can drive
// materialize directly with a fake clock.
func (m *CronMaterializer) Start(ctx context.Context) {
	ticker := time.NewTicker(m.tick)
	defer ticker.Stop()

	m.logger.Info("cron materializer started", "tick", m.tick)

	for {
		select {
		case <-ctx.Done():
			m.logger.Info("cron materializer stopped")
			return
		case <-ticker.C:
			m.materialize(ctx, m.now())
		}
	}
}

// schedule returns the compiled schedule for spec, caching it. An invalid spec
// (which should never reach here — the API validates on write) returns an error.
func (m *CronMaterializer) schedule(spec string) (robfigcron.Schedule, error) {
	if s, ok := m.parsed[spec]; ok {
		return s, nil
	}
	s, err := Parse(spec)
	if err != nil {
		return nil, err
	}
	m.parsed[spec] = s
	return s, nil
}

// materialize enqueues the latest due slot for every enabled job as of now.
// Skip-to-latest: only the most recent slot in (cursor, now] is fired, so N
// missed slots after downtime collapse into a single fire and the cursor jumps
// past the gap.
func (m *CronMaterializer) materialize(ctx context.Context, now time.Time) {
	jobs, err := m.crons.List(ctx)
	if err != nil {
		m.logger.Error("cron: error listing jobs", "error", err)
		return
	}

	for _, job := range jobs {
		if !job.Enabled {
			continue
		}

		sched, err := m.schedule(job.Schedule)
		if err != nil {
			m.logger.Error("cron: invalid schedule", "id", job.ID, "schedule", job.Schedule, "error", err)
			continue
		}

		// Walk from the cursor (or CreatedAt on the first run) collecting the last
		// slot that is not after now. sched.Next returns the strictly-next fire
		// after its argument, so starting from the cursor never re-fires it.
		start := job.CreatedAt
		if job.LastScheduledUnix != 0 {
			start = time.Unix(job.LastScheduledUnix, 0)
		}
		var lastDue time.Time
		for next := sched.Next(start); !next.After(now); next = sched.Next(next) {
			lastDue = next
		}
		if lastDue.IsZero() {
			continue
		}

		id := fmt.Sprintf("%s-%d", job.ID, lastDue.Unix())

		// Idempotency (P1.6, same check as SubmitTask): if the instance record
		// already exists (e.g. a prior leader fired this slot before crashing, or a
		// re-tick at the same now), skip the enqueue but still advance the cursor.
		exists, err := m.tasks.Exists(ctx, id)
		if err != nil {
			m.logger.Error("cron: exists check failed", "id", id, "error", err)
			continue
		}
		if exists {
			if err := m.crons.UpdateCursor(ctx, job.ID, lastDue.Unix()); err != nil {
				m.logger.Error("cron: cursor update failed", "id", job.ID, "error", err)
			}
			continue
		}

		task := model.Task{
			ID:         id,
			Type:       job.Task.Type,
			Payload:    job.Task.Payload,
			Priority:   job.Task.Priority,
			MaxRetries: job.Task.MaxRetries,
			Status:     model.StatusPending,
			CreatedAt:  now,
		}

		// Persist the canonical record before referencing its ID from the ready set.
		if err := m.tasks.Save(ctx, task); err != nil {
			m.logger.Error("cron: save task failed", "id", id, "error", err)
			continue
		}
		if err := m.queue.Enqueue(ctx, task); err != nil {
			m.logger.Error("cron: enqueue failed", "id", id, "error", err)
			continue
		}

		if m.events != nil {
			event := model.TaskEvent{
				ID:        fmt.Sprintf("evt-%d", now.UnixNano()),
				TaskID:    task.ID,
				Type:      "submitted",
				WorkerID:  -1,
				Detail:    "cron " + job.ID,
				Timestamp: now,
			}
			m.events.Push(ctx, event)
		}
		if m.metrics != nil {
			m.metrics.IncrSubmitted(ctx)
		}

		// Advance the cursor LAST — only after the slot is live — so a crash before
		// this leaves the slot to be recomputed (and deduped by Exists), never
		// marked done before it was enqueued.
		if err := m.crons.UpdateCursor(ctx, job.ID, lastDue.Unix()); err != nil {
			m.logger.Error("cron: cursor update failed", "id", job.ID, "error", err)
		}
	}
}
