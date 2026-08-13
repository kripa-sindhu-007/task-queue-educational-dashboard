package worker_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/redis/go-redis/v9"

	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/broker"
	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/handler"
	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/model"
	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/queue"
	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/store"
	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/telemetry"
	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/worker"
)

// execEnv bundles the collaborators an executor test needs, all backed by
// miniredis. The handler registry maps type "ok" to a success and "boom" to a
// failure so tests can drive both terminal paths.
type execEnv struct {
	client *redis.Client
	broker *broker.RedisBroker
	deps   worker.ExecutorDeps
	reg    *prometheus.Registry
}

func newExecEnv(t *testing.T, logger *slog.Logger, metrics *telemetry.Metrics) execEnv {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { client.Close() })

	tasks := store.NewTaskStore(client)
	pq := queue.NewPriorityQueue(client, tasks)
	delayed := queue.NewDelayedScheduler(client, pq, tasks, nil, logger)
	b := broker.NewRedisBroker(client, tasks, pq, delayed, 30*time.Second, "test-node")

	reg := handler.NewRegistry()
	reg.Register("ok", func(context.Context, model.Task) (handler.Result, error) {
		return handler.Result{Detail: "done"}, nil
	})
	reg.Register("boom", func(context.Context, model.Task) (handler.Result, error) {
		return handler.Result{}, errors.New("boom")
	})

	deps := worker.ExecutorDeps{
		Broker:     b,
		Handlers:   reg,
		Tasks:      tasks,
		Delayed:    delayed,
		DeadLetter: store.NewDeadLetterStore(client),
		Metrics:    store.NewMetricsStore(client),
		Events:     store.NewEventStore(client),
		Logger:     logger,
		Telemetry:  metrics,
	}
	return execEnv{client: client, broker: b, deps: deps}
}

// leaseTask enqueues a task and immediately dequeues it so it holds a lease
// (sits in the processing set), returning the leased record.
func leaseTask(t *testing.T, b *broker.RedisBroker, task model.Task) *model.Task {
	t.Helper()
	ctx := context.Background()
	if err := b.Enqueue(ctx, task); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	leased, err := b.Dequeue(ctx)
	if err != nil {
		t.Fatalf("dequeue: %v", err)
	}
	if leased == nil {
		t.Fatal("dequeue returned nil")
	}
	return leased
}

func TestExecutor_Success_RecordsCompletedMetrics(t *testing.T) {
	reg := prometheus.NewRegistry()
	metrics := telemetry.New(reg)
	env := newExecEnv(t, slog.Default(), metrics)
	exec := worker.NewExecutor(env.deps)

	task := model.Task{ID: "ok-1", Type: "ok", MaxRetries: 3, CreatedAt: time.Now().Add(-500 * time.Millisecond)}
	leased := leaseTask(t, env.broker, task)
	exec.Execute(leased, 0)

	if got := counterValue(t, reg, "tasks_processed_total", map[string]string{"type": "ok", "status": "completed"}); got != 1 {
		t.Errorf("tasks_processed_total{ok,completed} = %v, want 1", got)
	}
	if got := histogramCount(t, reg, "task_duration_seconds", map[string]string{"type": "ok"}); got != 1 {
		t.Errorf("task_duration_seconds{ok} count = %d, want 1", got)
	}
	if got := histogramCount(t, reg, "enqueue_to_start_seconds", nil); got != 1 {
		t.Errorf("enqueue_to_start_seconds count = %d, want 1", got)
	}
}

func TestExecutor_DeadLetter_RecordsFailedMetric(t *testing.T) {
	reg := prometheus.NewRegistry()
	metrics := telemetry.New(reg)
	env := newExecEnv(t, slog.Default(), metrics)
	exec := worker.NewExecutor(env.deps)

	// MaxRetries=0 => first failure exhausts retries and dead-letters immediately.
	task := model.Task{ID: "boom-1", Type: "boom", MaxRetries: 0, CreatedAt: time.Now()}
	leased := leaseTask(t, env.broker, task)
	exec.Execute(leased, 0)

	if got := counterValue(t, reg, "tasks_processed_total", map[string]string{"type": "boom", "status": "failed"}); got != 1 {
		t.Errorf("tasks_processed_total{boom,failed} = %v, want 1", got)
	}
	// It must NOT have been counted as completed or retried.
	if got := counterValue(t, reg, "tasks_processed_total", map[string]string{"type": "boom", "status": "completed"}); got != -1 {
		t.Errorf("unexpected completed series for boom: %v", got)
	}
	dlSize, _ := env.client.LLen(context.Background(), store.KeyDeadLetter).Result()
	if dlSize != 1 {
		t.Errorf("expected DLQ size 1, got %d", dlSize)
	}
}

func TestExecutor_Retry_RecordsRetriedMetric(t *testing.T) {
	reg := prometheus.NewRegistry()
	metrics := telemetry.New(reg)
	env := newExecEnv(t, slog.Default(), metrics)
	exec := worker.NewExecutor(env.deps)

	// MaxRetries=2 => first failure routes to retry (delayed), not the DLQ.
	task := model.Task{ID: "boom-retry", Type: "boom", MaxRetries: 2, CreatedAt: time.Now()}
	leased := leaseTask(t, env.broker, task)
	exec.Execute(leased, 0)

	if got := counterValue(t, reg, "tasks_processed_total", map[string]string{"type": "boom", "status": "retried"}); got != 1 {
		t.Errorf("tasks_processed_total{boom,retried} = %v, want 1", got)
	}
}

// TestExecutor_LogsCarryTaskAndNodeID covers P3.1b: the executor derives a
// per-task child logger, and when the injected base logger already carries a
// node_id, task log lines carry both keys as structured JSON fields.
func TestExecutor_LogsCarryTaskAndNodeID(t *testing.T) {
	var buf bytes.Buffer
	base := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})).With("node_id", "node-xyz")

	env := newExecEnv(t, base, nil) // nil telemetry: metrics no-op, logging still exercised
	exec := worker.NewExecutor(env.deps)

	task := model.Task{ID: "log-task-1", Type: "ok", MaxRetries: 1, CreatedAt: time.Now()}
	leased := leaseTask(t, env.broker, task)
	exec.Execute(leased, 0)

	out := buf.String()
	if !strings.Contains(out, `"task_id":"log-task-1"`) {
		t.Errorf("expected a log line carrying task_id, got:\n%s", out)
	}
	if !strings.Contains(out, `"node_id":"node-xyz"`) {
		t.Errorf("expected log lines carrying node_id, got:\n%s", out)
	}
}

// --- gather helpers -------------------------------------------------------

func counterValue(t *testing.T, g prometheus.Gatherer, name string, labels map[string]string) float64 {
	t.Helper()
	mfs, err := g.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	for _, mf := range mfs {
		if mf.GetName() != name {
			continue
		}
		for _, m := range mf.GetMetric() {
			if labelsMatch(m.GetLabel(), labels) {
				return m.GetCounter().GetValue()
			}
		}
	}
	return -1
}

func histogramCount(t *testing.T, g prometheus.Gatherer, name string, labels map[string]string) uint64 {
	t.Helper()
	mfs, err := g.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	for _, mf := range mfs {
		if mf.GetName() != name {
			continue
		}
		for _, m := range mf.GetMetric() {
			if labelsMatch(m.GetLabel(), labels) {
				return m.GetHistogram().GetSampleCount()
			}
		}
	}
	return 0
}

func labelsMatch(pairs []*dto.LabelPair, want map[string]string) bool {
	got := map[string]string{}
	for _, p := range pairs {
		got[p.GetName()] = p.GetValue()
	}
	if len(got) != len(want) {
		return false
	}
	for k, v := range want {
		if got[k] != v {
			return false
		}
	}
	return true
}
