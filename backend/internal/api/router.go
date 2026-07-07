package api

import "net/http"

func NewRouter(h *Handler) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /api/tasks", h.SubmitTask)
	mux.HandleFunc("GET /api/metrics", h.GetMetrics)
	mux.HandleFunc("GET /api/tasks/failed", h.GetFailedTasks)
	mux.HandleFunc("GET /api/tasks/{id}", h.GetTask)
	mux.HandleFunc("GET /api/health", h.HealthCheck)

	// New educational endpoints
	mux.HandleFunc("GET /api/events", h.GetEvents)
	mux.HandleFunc("GET /api/workers", h.GetWorkers)
	mux.HandleFunc("GET /api/queues", h.GetQueues)
	mux.HandleFunc("GET /api/metrics/enhanced", h.GetEnhancedMetrics)
	mux.HandleFunc("DELETE /api/flush", h.FlushData)

	// Apply middleware: Recovery -> Logging -> CORS -> routes
	var handler http.Handler = mux
	handler = CORS(handler)
	handler = Logging(handler)
	handler = Recovery(handler)

	return handler
}
