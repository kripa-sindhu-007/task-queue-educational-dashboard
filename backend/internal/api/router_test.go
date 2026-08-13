package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/telemetry"
)

// TestNewRouter_ServesMetrics covers P3.2d: when a metrics handler is supplied,
// NewRouter mounts it at GET /metrics and returns Prometheus exposition text.
func TestNewRouter_ServesMetrics(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := telemetry.New(reg)
	// Materialize every family (the label-less Vecs emit nothing until observed).
	m.IncTasksProcessed("sleep", "completed")
	m.ObserveTaskDuration("sleep", 1)
	m.ObserveEnqueueToStart(1)
	m.IncReaperReclaims()

	router := NewRouter(&Handler{}, nil, promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
		t.Fatalf("expected text/plain content type, got %q", ct)
	}
	body := rec.Body.String()
	for _, name := range []string{
		"tasks_processed_total",
		"task_duration_seconds",
		"enqueue_to_start_seconds",
		"reaper_reclaims_total",
	} {
		if !strings.Contains(body, name) {
			t.Errorf("expected /metrics body to contain %q", name)
		}
	}
}

// TestNewRouter_NilMetricsOmitsEndpoint asserts that passing a nil metrics
// handler (the test/default path) leaves /metrics unmounted rather than panicking.
func TestNewRouter_NilMetricsOmitsEndpoint(t *testing.T) {
	router := NewRouter(&Handler{}, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 when metrics handler is nil, got %d", rec.Code)
	}
}
