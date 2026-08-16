package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/cron"
)

type cronRequest struct {
	ID       string            `json:"id"`
	Schedule string            `json:"schedule"`
	Task     cron.TaskTemplate `json:"task"`
	Enabled  *bool             `json:"enabled"` // pointer so an omitted field defaults to true
}

// genCronID returns a unique cron-job ID when the client does not supply one.
func genCronID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("cron-%d", time.Now().UnixNano())
	}
	return "cron-" + hex.EncodeToString(b[:])
}

// CreateCron registers a cron job (P4.4). The schedule is validated with the same
// parser the materializer uses (5- or 6-field); an invalid spec returns 400. A
// missing id is generated; enabled defaults to true.
func (h *Handler) CreateCron(w http.ResponseWriter, r *http.Request) {
	if h.deps.Cron == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "cron store not configured"})
		return
	}

	var req cronRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	if _, err := cron.Parse(req.Schedule); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": fmt.Sprintf("invalid schedule %q: %v", req.Schedule, err),
		})
		return
	}

	id := req.ID
	if id == "" {
		id = genCronID()
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	job := cron.CronJob{
		ID:        id,
		Schedule:  req.Schedule,
		Task:      req.Task,
		Enabled:   enabled,
		CreatedAt: time.Now(),
	}
	if err := h.deps.Cron.Save(r.Context(), job); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, job)
}

// ListCron returns all registered cron jobs (P4.4).
func (h *Handler) ListCron(w http.ResponseWriter, r *http.Request) {
	if h.deps.Cron == nil {
		writeJSON(w, http.StatusOK, []cron.CronJob{})
		return
	}
	jobs, err := h.deps.Cron.List(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if jobs == nil {
		jobs = []cron.CronJob{}
	}
	writeJSON(w, http.StatusOK, jobs)
}

// DeleteCron removes a cron job by id (P4.4). Deleting a missing id is a no-op.
func (h *Handler) DeleteCron(w http.ResponseWriter, r *http.Request) {
	if h.deps.Cron == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "cron store not configured"})
		return
	}
	id := r.PathValue("id")
	if err := h.deps.Cron.Delete(r.Context(), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
