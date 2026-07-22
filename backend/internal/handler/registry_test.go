package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/handler"
	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/model"
)

func TestDispatch_UnknownType(t *testing.T) {
	r := handler.NewRegistry()
	_, err := r.Dispatch(context.Background(), model.Task{ID: "t1", Type: "does-not-exist"})
	if !errors.Is(err, handler.ErrNoHandler) {
		t.Fatalf("expected ErrNoHandler, got %v", err)
	}
}

func TestDispatch_EmptyTypeDefaultsToSleep(t *testing.T) {
	r := handler.NewDefaultRegistry()
	// A short, guaranteed-success sleep so the test is fast and deterministic.
	payload, _ := json.Marshal(map[string]any{"duration_ms": 1, "fail_rate": 0.0001})
	res, err := r.Dispatch(context.Background(), model.Task{ID: "t1", Type: "", Payload: payload})
	if err != nil {
		t.Fatalf("expected empty type to default to sleep and succeed, got %v", err)
	}
	if res.Detail == "" {
		t.Fatal("expected a result detail from sleep handler")
	}
}

func TestRegister_Overrides(t *testing.T) {
	r := handler.NewRegistry()
	called := ""
	r.Register("x", func(ctx context.Context, task model.Task) (handler.Result, error) {
		called = "first"
		return handler.Result{}, nil
	})
	r.Register("x", func(ctx context.Context, task model.Task) (handler.Result, error) {
		called = "second"
		return handler.Result{}, nil
	})
	_, _ = r.Dispatch(context.Background(), model.Task{Type: "x"})
	if called != "second" {
		t.Fatalf("expected second registration to win, got %q", called)
	}
}

func TestSleepHandler_RespectsFailRate(t *testing.T) {
	// fail_rate=1 forces a failure deterministically.
	payload, _ := json.Marshal(map[string]any{"duration_ms": 1, "fail_rate": 1})
	_, err := handler.SleepHandler(context.Background(), model.Task{ID: "t1", Payload: payload})
	if err == nil {
		t.Fatal("expected failure with fail_rate=1")
	}
}

func TestHashHandler_Deterministic(t *testing.T) {
	payload, _ := json.Marshal(map[string]any{"input": "hello", "rounds": 10})
	r1, err := handler.HashHandler(context.Background(), model.Task{Payload: payload})
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	r2, _ := handler.HashHandler(context.Background(), model.Task{Payload: payload})
	if r1.Detail != r2.Detail {
		t.Fatalf("hash not deterministic: %q vs %q", r1.Detail, r2.Detail)
	}
}

func TestHTTPFetchHandler_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	payload, _ := json.Marshal(map[string]any{"url": srv.URL})
	res, err := handler.HTTPFetchHandler(context.Background(), model.Task{Payload: payload})
	if err != nil {
		t.Fatalf("http_fetch: %v", err)
	}
	if res.Detail == "" {
		t.Fatal("expected detail with status/latency")
	}
}

func TestHTTPFetchHandler_Non2xxIsFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	payload, _ := json.Marshal(map[string]any{"url": srv.URL})
	_, err := handler.HTTPFetchHandler(context.Background(), model.Task{Payload: payload})
	if err == nil {
		t.Fatal("expected non-2xx to be treated as failure")
	}
}

func TestHTTPFetchHandler_MissingURL(t *testing.T) {
	_, err := handler.HTTPFetchHandler(context.Background(), model.Task{Payload: json.RawMessage(`{}`)})
	if err == nil {
		t.Fatal("expected error when url is missing")
	}
}
