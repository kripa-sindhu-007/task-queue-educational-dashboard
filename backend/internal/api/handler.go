package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/model"
	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/queue"
	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/store"
	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/worker"
)

// HandlerDeps bundles the collaborators the API handler needs, replacing a long
// positional constructor (P0.8).
type HandlerDeps struct {
	Queue       *queue.PriorityQueue
	Delayed     *queue.DelayedScheduler
	DeadLetter  *store.DeadLetterStore
	Metrics     *store.MetricsStore
	Pool        *worker.Pool
	Redis       *redis.Client
	Events      *store.EventStore
	WorkerState *store.WorkerStateStore
	QueuePeek   *store.QueuePeekStore
	Tasks       *store.TaskStore
	Nodes       *store.NodeStore
}

type Handler struct {
	deps HandlerDeps
}

func NewHandler(deps HandlerDeps) *Handler {
	return &Handler{deps: deps}
}

type submitRequest struct {
	ID         string          `json:"id"`
	Type       string          `json:"type"`
	Payload    json.RawMessage `json:"payload"`
	Priority   int             `json:"priority"`
	Delay      int             `json:"delay"`
	MaxRetries int             `json:"max_retries"`
}

// genID returns a unique task ID. Every submission gets its own identity so that
// two tasks with otherwise-identical fields are distinct entries (the identity
// bug fix). A client-supplied ID is honored and becomes an idempotency key in P1.6.
func genID() string {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("task-%d", time.Now().UnixNano())
	}
	return "task-" + hex.EncodeToString(b[:])
}

func (h *Handler) SubmitTask(w http.ResponseWriter, r *http.Request) {
	var req submitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	id := req.ID
	if id == "" {
		id = genID()
	} else {
		// Client-supplied ID: check for duplicates (idempotent enqueue, P1.6).
		// If a record already exists, reject to prevent double-submission from
		// network retries. Server-generated IDs are always unique so they skip this.
		exists, err := h.deps.Tasks.Exists(r.Context(), id)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		if exists {
			writeJSON(w, http.StatusConflict, map[string]string{
				"error": fmt.Sprintf("task with id %q already exists", id),
			})
			return
		}
	}

	task := model.Task{
		ID:         id,
		Type:       req.Type,
		Payload:    req.Payload,
		Priority:   req.Priority,
		Delay:      req.Delay,
		MaxRetries: req.MaxRetries,
		Status:     model.StatusPending,
		CreatedAt:  time.Now(),
	}

	ctx := r.Context()

	// Persist the canonical record before referencing its ID from any queue.
	if err := h.deps.Tasks.Save(ctx, task); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// Emit submitted event.
	event := model.TaskEvent{
		ID:        fmt.Sprintf("evt-%d", time.Now().UnixNano()),
		TaskID:    task.ID,
		Type:      "submitted",
		WorkerID:  -1,
		Detail:    fmt.Sprintf("Priority=%d, Delay=%ds, MaxRetries=%d", task.Priority, task.Delay, task.MaxRetries),
		Timestamp: time.Now(),
	}
	h.deps.Events.Push(ctx, event)
	h.deps.Metrics.IncrSubmitted(ctx)

	if req.Delay > 0 {
		delay := time.Duration(req.Delay) * time.Second
		if err := h.deps.Delayed.Schedule(ctx, task, delay); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
	} else {
		if err := h.deps.Queue.Enqueue(ctx, task); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
	}

	writeJSON(w, http.StatusCreated, task)
}

// GetTask returns the canonical task record for a given ID (P0.7b / P1.7).
func (h *Handler) GetTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	task, err := h.deps.Tasks.Get(r.Context(), id)
	if err == store.ErrTaskNotFound {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "task not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, task)
}

func (h *Handler) GetMetrics(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	queueSize, _ := h.deps.Queue.Size(ctx)
	activeWorkers := h.deps.Pool.ActiveWorkers()

	m, err := h.deps.Metrics.Get(ctx, queueSize, activeWorkers)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, m)
}

func (h *Handler) GetFailedTasks(w http.ResponseWriter, r *http.Request) {
	offset, _ := strconv.ParseInt(r.URL.Query().Get("offset"), 10, 64)
	limit, _ := strconv.ParseInt(r.URL.Query().Get("limit"), 10, 64)
	if limit <= 0 {
		limit = 20
	}

	tasks, err := h.deps.DeadLetter.List(r.Context(), offset, limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, tasks)
}

func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	if err := h.deps.Redis.Ping(r.Context()).Err(); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unhealthy", "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "healthy"})
}

// --- Educational endpoints ---

func (h *Handler) GetEvents(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.ParseInt(r.URL.Query().Get("limit"), 10, 64)
	if limit <= 0 {
		limit = 50
	}
	events, err := h.deps.Events.List(r.Context(), limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, events)
}

func (h *Handler) GetWorkers(w http.ResponseWriter, r *http.Request) {
	states, err := h.deps.WorkerState.GetAll(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, states)
}

// GetNodes returns the live cluster membership (P2.7): every registered node with
// its liveness and in-flight task count. Dead nodes (heartbeat expired but not yet
// reaped) appear with alive=false so the dashboard can show them graying out.
func (h *Handler) GetNodes(w http.ResponseWriter, r *http.Request) {
	if h.deps.Nodes == nil {
		writeJSON(w, http.StatusOK, []any{})
		return
	}
	nodes, err := h.deps.Nodes.ListNodes(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if nodes == nil {
		nodes = []model.Node{}
	}
	writeJSON(w, http.StatusOK, nodes)
}

func (h *Handler) GetQueues(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ready, err := h.deps.QueuePeek.PeekReady(ctx, 20)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	delayed, err := h.deps.QueuePeek.PeekDelayed(ctx, 20)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ready":   ready,
		"delayed": delayed,
	})
}

func (h *Handler) GetEnhancedMetrics(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	queueSize, _ := h.deps.Queue.Size(ctx)
	activeWorkers := h.deps.Pool.ActiveWorkers()
	delayedSize, _ := h.deps.QueuePeek.DelayedSize(ctx)
	deadLetterSize, _ := h.deps.QueuePeek.DeadLetterSize(ctx)

	m, err := h.deps.Metrics.GetEnhanced(ctx, queueSize, activeWorkers, delayedSize, deadLetterSize)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, m)
}

func (h *Handler) FlushData(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	keys := []string{
		store.KeyReady, store.KeyDelayed, store.KeyProcessing, store.KeyDeadLetter,
		store.KeyMetrics, store.KeyEvents, store.KeyWorkers,
	}
	h.deps.Redis.Del(ctx, keys...)

	// Delete all canonical task records (taskqueue:task:*).
	iter := h.deps.Redis.Scan(ctx, 0, store.KeyTaskPrefix+"*", 200).Iterator()
	var taskKeys []string
	for iter.Next(ctx) {
		taskKeys = append(taskKeys, iter.Val())
	}
	if len(taskKeys) > 0 {
		h.deps.Redis.Del(ctx, taskKeys...)
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "flushed"})
}

// RedriveFailed pops all tasks from the dead-letter queue, resets their retries
// and status, updates the canonical record, and re-enqueues them to ready (P1.7).
func (h *Handler) RedriveFailed(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	failed, err := h.deps.DeadLetter.DrainAll(ctx)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	if len(failed) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{
			"redriven": 0,
			"message":  "dead-letter queue is empty",
		})
		return
	}

	redriven := 0
	for _, ft := range failed {
		task := ft.Task
		task.Retries = 0
		task.Status = model.StatusPending
		task.Error = ""

		// Update canonical record
		if err := h.deps.Tasks.Save(ctx, task); err != nil {
			continue
		}
		// Enqueue to ready
		if err := h.deps.Queue.Enqueue(ctx, task); err != nil {
			continue
		}
		redriven++

		// Emit event
		event := model.TaskEvent{
			ID:        fmt.Sprintf("evt-%d", time.Now().UnixNano()),
			TaskID:    task.ID,
			Type:      "redriven",
			WorkerID:  -1,
			Detail:    "Moved from dead-letter queue back to ready",
			Timestamp: time.Now(),
		}
		h.deps.Events.Push(ctx, event)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"redriven": redriven,
		"total":    len(failed),
	})
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
