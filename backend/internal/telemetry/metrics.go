// Package telemetry holds the process-local Prometheus collectors for the task
// queue. They are additive to the Redis-backed store.MetricsStore (which still
// feeds the dashboard's /api/metrics) — these are the numbers Prometheus scrapes
// off /metrics.
//
// All collectors register on a caller-supplied *prometheus.Registry rather than
// the global default, so tests can use a fresh registry and the two binaries
// never trip a duplicate-registration panic. A nil *Metrics is a safe no-op, so
// components that don't emit metrics (and the ~40 existing tests) can leave the
// field unset.
package telemetry

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Metrics bundles the imperatively-updated collectors. Construct it with New and
// inject it through the same Deps structs as the logger; a nil *Metrics no-ops.
type Metrics struct {
	tasksProcessed *prometheus.CounterVec   // tasks_processed_total{type,status}
	taskDuration   *prometheus.HistogramVec // task_duration_seconds{type}
	enqueueToStart prometheus.Histogram     // enqueue_to_start_seconds
	reaperReclaims prometheus.Counter       // reaper_reclaims_total
}

// New builds the collectors and registers them on reg. It panics (via
// MustRegister) if reg already holds a collision — callers pass a fresh
// registry, so that only fires on a genuine programming error.
func New(reg *prometheus.Registry) *Metrics {
	m := &Metrics{
		tasksProcessed: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "tasks_processed_total",
			Help: "Total tasks processed, partitioned by task type and terminal status (completed/retried/failed).",
		}, []string{"type", "status"}),
		taskDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "task_duration_seconds",
			Help:    "Handler execution time per task, partitioned by task type.",
			Buckets: prometheus.DefBuckets,
		}, []string{"type"}),
		enqueueToStart: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "enqueue_to_start_seconds",
			Help:    "Time from task creation to the start of execution.",
			Buckets: prometheus.DefBuckets,
		}),
		reaperReclaims: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "reaper_reclaims_total",
			Help: "Total leases reclaimed by the reaper (expired or dead-node).",
		}),
	}
	reg.MustRegister(m.tasksProcessed, m.taskDuration, m.enqueueToStart, m.reaperReclaims)
	return m
}

// normalizeType keeps the type label bounded: an empty task type becomes
// "unknown" rather than an empty-string series.
func normalizeType(taskType string) string {
	if taskType == "" {
		return "unknown"
	}
	return taskType
}

// IncTasksProcessed records one processed task with the given terminal status
// (completed/retried/failed).
func (m *Metrics) IncTasksProcessed(taskType, status string) {
	if m == nil {
		return
	}
	m.tasksProcessed.WithLabelValues(normalizeType(taskType), status).Inc()
}

// ObserveTaskDuration records the handler execution time for a task type.
func (m *Metrics) ObserveTaskDuration(taskType string, d time.Duration) {
	if m == nil {
		return
	}
	m.taskDuration.WithLabelValues(normalizeType(taskType)).Observe(d.Seconds())
}

// ObserveEnqueueToStart records the delay between task creation and execution.
func (m *Metrics) ObserveEnqueueToStart(d time.Duration) {
	if m == nil {
		return
	}
	m.enqueueToStart.Observe(d.Seconds())
}

// IncReaperReclaims records one lease reclaimed by the reaper.
func (m *Metrics) IncReaperReclaims() {
	if m == nil {
		return
	}
	m.reaperReclaims.Inc()
}
