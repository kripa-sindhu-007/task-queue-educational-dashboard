package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/model"
	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/queue"
	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/store"
)

func newTestServer(t *testing.T) (http.Handler, *store.TaskStore, *queue.PriorityQueue) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	tasks := store.NewTaskStore(client)
	q := queue.NewPriorityQueue(client, tasks)
	delayed := queue.NewDelayedScheduler(client, q, tasks, store.NewEventStore(client))

	h := NewHandler(HandlerDeps{
		Queue:   q,
		Delayed: delayed,
		Metrics: store.NewMetricsStore(client),
		Events:  store.NewEventStore(client),
		Tasks:   tasks,
		Redis:   client,
	})
	return NewRouter(h), tasks, q
}

func submit(t *testing.T, srv http.Handler, body string) model.Task {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/tasks", strings.NewReader(body))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("submit: status %d, body %s", rec.Code, rec.Body.String())
	}
	var task model.Task
	if err := json.Unmarshal(rec.Body.Bytes(), &task); err != nil {
		t.Fatalf("decode submit response: %v", err)
	}
	return task
}

// Two submissions with identical fields (and no client-supplied ID) must become
// two distinct queue entries — the Phase 0 acceptance criterion at the API layer.
func TestSubmitIdenticalFieldsCreatesTwoEntries(t *testing.T) {
	srv, _, q := newTestServer(t)

	a := submit(t, srv, `{"priority":5,"max_retries":2}`)
	b := submit(t, srv, `{"priority":5,"max_retries":2}`)
	if a.ID == b.ID {
		t.Fatalf("expected distinct generated IDs, both were %q", a.ID)
	}

	size, err := q.Size(context.Background())
	if err != nil {
		t.Fatalf("size: %v", err)
	}
	if size != 2 {
		t.Fatalf("expected 2 ready entries, got %d", size)
	}
}

// GET /api/tasks/{id} returns the canonical record; unknown IDs return 404 and
// must not be shadowed by the /api/tasks/failed route.
func TestGetTaskByID(t *testing.T) {
	srv, _, _ := newTestServer(t)

	created := submit(t, srv, `{"id":"task-xyz","type":"sleep","priority":3}`)

	req := httptest.NewRequest(http.MethodGet, "/api/tasks/task-xyz", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("get task: status %d, body %s", rec.Code, rec.Body.String())
	}
	var got model.Task
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.ID != created.ID || got.Type != "sleep" || got.Priority != 3 {
		t.Fatalf("canonical record mismatch: %+v", got)
	}

	// Missing task → 404.
	miss := httptest.NewRequest(http.MethodGet, "/api/tasks/does-not-exist", nil)
	missRec := httptest.NewRecorder()
	srv.ServeHTTP(missRec, miss)
	if missRec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for missing task, got %d", missRec.Code)
	}
}

// GET /api/nodes returns registered cluster nodes with liveness. When no
// NodeStore is wired (or none registered) it returns an empty array, never null.
func TestGetNodes(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	nodes := store.NewNodeStore(client, 10*time.Second)
	h := NewHandler(HandlerDeps{
		Tasks: store.NewTaskStore(client),
		Redis: client,
		Nodes: nodes,
	})
	srv := NewRouter(h)

	// Empty cluster → [].
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/nodes", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("get nodes: status %d", rec.Code)
	}
	if strings.TrimSpace(rec.Body.String()) != "[]" {
		t.Fatalf("expected [] for empty cluster, got %s", rec.Body.String())
	}

	// Register a node and verify it appears alive.
	if err := nodes.Register(context.Background(), model.Node{
		ID: "node-1", Hostname: "host-1", Capacity: 5, StartedAt: time.Now(),
	}); err != nil {
		t.Fatalf("register: %v", err)
	}
	rec2 := httptest.NewRecorder()
	srv.ServeHTTP(rec2, httptest.NewRequest(http.MethodGet, "/api/nodes", nil))
	var got []model.Node
	if err := json.Unmarshal(rec2.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode nodes: %v", err)
	}
	if len(got) != 1 || got[0].ID != "node-1" || !got[0].Alive {
		t.Fatalf("expected 1 alive node, got %+v", got)
	}
}
