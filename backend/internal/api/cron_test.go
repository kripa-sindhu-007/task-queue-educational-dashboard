package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/cron"
	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/store"
)

func newCronServer(t *testing.T) http.Handler {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { client.Close() })

	h := NewHandler(HandlerDeps{
		Tasks: store.NewTaskStore(client),
		Cron:  cron.NewCronStore(client),
		Redis: client,
	})
	return NewRouter(h, nil, nil)
}

func TestCronCRUDEndpoints(t *testing.T) {
	srv := newCronServer(t)

	// POST valid schedule → 201 with a generated id and enabled defaulting true.
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/cron",
		strings.NewReader(`{"schedule":"*/10 * * * * *","task":{"type":"sleep","priority":2}}`)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: status %d, body %s", rec.Code, rec.Body.String())
	}
	var created cron.CronJob
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create: %v", err)
	}
	if created.ID == "" || !strings.HasPrefix(created.ID, "cron-") {
		t.Fatalf("expected generated cron id, got %q", created.ID)
	}
	if !created.Enabled {
		t.Fatalf("expected enabled to default true")
	}
	if created.Task.Type != "sleep" || created.Task.Priority != 2 {
		t.Fatalf("task template mismatch: %+v", created.Task)
	}

	// POST bad schedule → 400.
	bad := httptest.NewRecorder()
	srv.ServeHTTP(bad, httptest.NewRequest(http.MethodPost, "/api/cron",
		strings.NewReader(`{"schedule":"not a cron","task":{"type":"sleep"}}`)))
	if bad.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for bad schedule, got %d (body %s)", bad.Code, bad.Body.String())
	}

	// GET → the created job appears.
	list := httptest.NewRecorder()
	srv.ServeHTTP(list, httptest.NewRequest(http.MethodGet, "/api/cron", nil))
	if list.Code != http.StatusOK {
		t.Fatalf("list: status %d", list.Code)
	}
	var jobs []cron.CronJob
	if err := json.Unmarshal(list.Body.Bytes(), &jobs); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(jobs) != 1 || jobs[0].ID != created.ID {
		t.Fatalf("expected the created job in list, got %+v", jobs)
	}

	// DELETE → 200 and the job is gone.
	del := httptest.NewRecorder()
	srv.ServeHTTP(del, httptest.NewRequest(http.MethodDelete, "/api/cron/"+created.ID, nil))
	if del.Code != http.StatusOK {
		t.Fatalf("delete: status %d, body %s", del.Code, del.Body.String())
	}

	list2 := httptest.NewRecorder()
	srv.ServeHTTP(list2, httptest.NewRequest(http.MethodGet, "/api/cron", nil))
	if strings.TrimSpace(list2.Body.String()) != "[]" {
		t.Fatalf("expected [] after delete, got %s", list2.Body.String())
	}
}
