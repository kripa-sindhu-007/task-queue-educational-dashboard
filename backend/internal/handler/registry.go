// Package handler provides a registry that maps a task's Type to the function
// that executes it. This replaces the executor's hardcoded work simulation
// (P2.4) and lets workers run heterogeneous task types.
package handler

import (
	"context"
	"errors"
	"fmt"

	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/model"
)

// ErrNoHandler is returned by Dispatch when no handler is registered for a
// task's Type. The caller should treat this as a task failure (route to retry
// or DLQ) rather than a panic — a bad task type must not crash the worker.
var ErrNoHandler = errors.New("handler: no handler registered for task type")

// Result is the outcome of a successful handler run. Detail is a short
// human-readable summary surfaced in events (e.g. "200 in 143ms").
type Result struct {
	Detail string
}

// Handler executes a single task. It must respect ctx cancellation and return
// an error to signal failure (which triggers retry/DLQ routing).
type Handler func(ctx context.Context, task model.Task) (Result, error)

// Registry maps task types to handlers. It is read-only after construction/
// registration at startup, so no locking is needed for concurrent Dispatch.
type Registry struct {
	handlers map[string]Handler
}

// NewRegistry returns an empty registry. Use Register to add handlers, or
// NewDefaultRegistry for the built-in set.
func NewRegistry() *Registry {
	return &Registry{handlers: make(map[string]Handler)}
}

// Register associates a handler with a task type. A second registration for the
// same type overwrites the first.
func (r *Registry) Register(taskType string, h Handler) {
	r.handlers[taskType] = h
}

// Get returns the handler for a task type and whether one exists.
func (r *Registry) Get(taskType string) (Handler, bool) {
	h, ok := r.handlers[taskType]
	return h, ok
}

// Types returns the registered task types (for logging/introspection).
func (r *Registry) Types() []string {
	types := make([]string, 0, len(r.handlers))
	for t := range r.handlers {
		types = append(types, t)
	}
	return types
}

// Dispatch runs the handler for the task's Type. If Type is empty it defaults
// to "sleep" so tasks submitted before P2.4 (no Type) still run. Returns
// ErrNoHandler (wrapped) if the type is unknown.
func (r *Registry) Dispatch(ctx context.Context, task model.Task) (Result, error) {
	taskType := task.Type
	if taskType == "" {
		taskType = TypeSleep
	}
	h, ok := r.handlers[taskType]
	if !ok {
		return Result{}, fmt.Errorf("%w: %q", ErrNoHandler, taskType)
	}
	return h(ctx, task)
}

// NewDefaultRegistry returns a registry with the built-in handlers registered:
// sleep (the simulator), http_fetch (real HTTP GET), and hash (CPU-bound).
func NewDefaultRegistry() *Registry {
	r := NewRegistry()
	r.Register(TypeSleep, SleepHandler)
	r.Register(TypeHTTPFetch, HTTPFetchHandler)
	r.Register(TypeHash, HashHandler)
	return r
}
