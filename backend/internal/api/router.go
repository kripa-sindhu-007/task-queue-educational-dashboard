package api

import (
	"log/slog"
	"net/http"
)

// NewRouter builds the API mux. logger drives the request/panic middleware (nil
// falls back to slog.Default()). metrics, when non-nil, is mounted at GET
// /metrics for Prometheus to scrape (outside the /api prefix, per convention);
// pass nil to omit it (e.g. in tests).
func NewRouter(h *Handler, logger *slog.Logger, metrics http.Handler) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	mux := http.NewServeMux()

	mux.HandleFunc("POST /api/tasks", h.SubmitTask)
	mux.HandleFunc("GET /api/metrics", h.GetMetrics)
	mux.HandleFunc("GET /api/tasks/failed", h.GetFailedTasks)
	mux.HandleFunc("POST /api/tasks/failed/redrive", h.RedriveFailed)
	mux.HandleFunc("GET /api/tasks/{id}", h.GetTask)
	mux.HandleFunc("GET /api/health", h.HealthCheck)
	mux.HandleFunc("GET /api/events", h.GetEvents)
	mux.HandleFunc("GET /api/events/cluster", h.GetClusterEvents)
	mux.HandleFunc("GET /api/workers", h.GetWorkers)
	mux.HandleFunc("GET /api/nodes", h.GetNodes)
	mux.HandleFunc("GET /api/leader", h.GetLeader)
	mux.HandleFunc("GET /api/queues", h.GetQueues)
	mux.HandleFunc("GET /api/metrics/enhanced", h.GetEnhancedMetrics)
	mux.HandleFunc("POST /api/cron", h.CreateCron)
	mux.HandleFunc("GET /api/cron", h.ListCron)
	mux.HandleFunc("DELETE /api/cron/{id}", h.DeleteCron)
	mux.HandleFunc("DELETE /api/flush", h.FlushData)

	if metrics != nil {
		mux.Handle("GET /metrics", metrics)
	}

	var handler http.Handler = mux
	handler = CORS(handler)
	handler = Logging(handler, logger)
	handler = Recovery(handler, logger)

	return handler
}
