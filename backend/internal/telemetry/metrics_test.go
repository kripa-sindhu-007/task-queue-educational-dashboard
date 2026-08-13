package telemetry_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	dto "github.com/prometheus/client_model/go"
	"github.com/redis/go-redis/v9"

	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/store"
	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/telemetry"
)

func TestNew_RegistersCollectors(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := telemetry.New(reg)

	// Touch each collector once so the label-less Vecs materialize a family
	// (a fresh CounterVec/HistogramVec emits nothing until a series is observed).
	m.IncTasksProcessed("sleep", "completed")
	m.ObserveTaskDuration("sleep", time.Millisecond)
	m.ObserveEnqueueToStart(time.Millisecond)
	m.IncReaperReclaims()

	mfs, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	got := map[string]bool{}
	for _, mf := range mfs {
		got[mf.GetName()] = true
	}
	for _, name := range []string{
		"tasks_processed_total",
		"task_duration_seconds",
		"enqueue_to_start_seconds",
		"reaper_reclaims_total",
	} {
		if !got[name] {
			t.Errorf("expected metric %q registered, gathered: %v", name, got)
		}
	}
}

func TestMetrics_HelpersRecordSamples(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := telemetry.New(reg)

	m.IncTasksProcessed("sleep", "completed")
	m.IncTasksProcessed("sleep", "completed")
	m.IncTasksProcessed("hash", "failed")
	m.ObserveTaskDuration("sleep", 250*time.Millisecond)
	m.ObserveEnqueueToStart(1200 * time.Millisecond)
	m.IncReaperReclaims()
	m.IncReaperReclaims()
	m.IncReaperReclaims()

	if got := counterValue(t, reg, "tasks_processed_total", map[string]string{"type": "sleep", "status": "completed"}); got != 2 {
		t.Errorf("tasks_processed_total{sleep,completed} = %v, want 2", got)
	}
	if got := counterValue(t, reg, "tasks_processed_total", map[string]string{"type": "hash", "status": "failed"}); got != 1 {
		t.Errorf("tasks_processed_total{hash,failed} = %v, want 1", got)
	}
	if got := counterValue(t, reg, "reaper_reclaims_total", nil); got != 3 {
		t.Errorf("reaper_reclaims_total = %v, want 3", got)
	}
	if got := histogramCount(t, reg, "task_duration_seconds", map[string]string{"type": "sleep"}); got != 1 {
		t.Errorf("task_duration_seconds{sleep} sample count = %d, want 1", got)
	}
	if got := histogramCount(t, reg, "enqueue_to_start_seconds", nil); got != 1 {
		t.Errorf("enqueue_to_start_seconds sample count = %d, want 1", got)
	}
}

// TestMetrics_NilIsNoOp asserts the helpers are safe on a nil receiver, so the
// nil-tolerant injection contract holds.
func TestMetrics_NilIsNoOp(t *testing.T) {
	var m *telemetry.Metrics // nil
	m.IncTasksProcessed("x", "completed")
	m.ObserveTaskDuration("x", time.Second)
	m.ObserveEnqueueToStart(time.Second)
	m.IncReaperReclaims()
	// Reaching here without a panic is the assertion.
}

func TestMetrics_EmptyTypeNormalizesToUnknown(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := telemetry.New(reg)
	m.IncTasksProcessed("", "completed")

	if got := counterValue(t, reg, "tasks_processed_total", map[string]string{"type": "unknown", "status": "completed"}); got != 1 {
		t.Errorf("expected empty type normalized to 'unknown', got value %v", got)
	}
}

func TestRegisterQueueDepth_ReadsReadySet(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { client.Close() })

	// Seed the ready set with three task IDs.
	ctx := context.Background()
	for i, id := range []string{"a", "b", "c"} {
		client.ZAdd(ctx, store.KeyReady, redis.Z{Score: float64(i), Member: id})
	}

	reg := prometheus.NewRegistry()
	telemetry.RegisterQueueDepth(reg, store.NewQueuePeekStore(client, store.NewTaskStore(client)))

	expected := `
# HELP queue_depth Number of tasks currently in the ready queue.
# TYPE queue_depth gauge
queue_depth 3
`
	if err := testutil.GatherAndCompare(reg, strings.NewReader(expected), "queue_depth"); err != nil {
		t.Fatalf("queue_depth mismatch: %v", err)
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
