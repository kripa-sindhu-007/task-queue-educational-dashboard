package handler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"time"

	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/model"
)

// Built-in task type identifiers.
const (
	TypeSleep     = "sleep"
	TypeHTTPFetch = "http_fetch"
	TypeHash      = "hash"
)

// --- sleep -----------------------------------------------------------------

// sleepPayload configures the sleep handler. All fields are optional.
type sleepPayload struct {
	DurationMS int     `json:"duration_ms"` // fixed sleep; 0 = random 200-800ms
	FailRate   float64 `json:"fail_rate"`   // 0..1 chance of simulated failure; 0 = use default 0.3
}

// SleepHandler is the original work simulator: it sleeps for a bounded duration
// and fails ~30% of the time. Preserving this keeps the educational demo
// (retries, backoff, dead-lettering) working exactly as before.
func SleepHandler(ctx context.Context, task model.Task) (Result, error) {
	var p sleepPayload
	if len(task.Payload) > 0 {
		_ = json.Unmarshal(task.Payload, &p) // best-effort; defaults on error
	}

	dur := time.Duration(p.DurationMS) * time.Millisecond
	if dur <= 0 {
		dur = time.Duration(200+rand.Intn(600)) * time.Millisecond
	}
	failRate := p.FailRate
	if failRate <= 0 {
		failRate = 0.3
	}

	select {
	case <-ctx.Done():
		return Result{}, ctx.Err()
	case <-time.After(dur):
	}

	if rand.Float64() < failRate {
		return Result{}, fmt.Errorf("simulated failure for task %s", task.ID)
	}
	return Result{Detail: fmt.Sprintf("slept %v", dur)}, nil
}

// --- http_fetch ------------------------------------------------------------

type httpFetchPayload struct {
	URL string `json:"url"`
}

// httpFetchClient bounds each fetch. A dedicated client avoids leaking the
// default client's unbounded timeout into task execution.
var httpFetchClient = &http.Client{Timeout: 10 * time.Second}

// HTTPFetchHandler performs a GET against the payload URL and records the status
// code and latency. A non-2xx response is treated as a failure so retry/DLQ
// logic exercises real outcomes.
//
// Security note: this fetches a URL supplied in the task payload. In a real
// deployment this is an SSRF vector (a caller could target internal services);
// it would need an allowlist/egress controls. It is acceptable here as an
// educational, operator-submitted workload, and requests are bounded by timeout.
func HTTPFetchHandler(ctx context.Context, task model.Task) (Result, error) {
	var p httpFetchPayload
	if err := json.Unmarshal(task.Payload, &p); err != nil {
		return Result{}, fmt.Errorf("http_fetch: invalid payload: %w", err)
	}
	if p.URL == "" {
		return Result{}, fmt.Errorf("http_fetch: payload.url is required")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.URL, nil)
	if err != nil {
		return Result{}, fmt.Errorf("http_fetch: build request: %w", err)
	}

	start := time.Now()
	resp, err := httpFetchClient.Do(req)
	if err != nil {
		return Result{}, fmt.Errorf("http_fetch: %w", err)
	}
	defer resp.Body.Close()
	// Drain (bounded) so the connection can be reused, and to measure full latency.
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
	latency := time.Since(start)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Result{}, fmt.Errorf("http_fetch: %s returned %d in %v", p.URL, resp.StatusCode, latency)
	}
	return Result{Detail: fmt.Sprintf("%d in %v", resp.StatusCode, latency.Round(time.Millisecond))}, nil
}

// --- hash ------------------------------------------------------------------

type hashPayload struct {
	Input  string `json:"input"`
	Rounds int    `json:"rounds"` // iterated SHA-256 rounds; 0 defaults to 100000
}

// HashHandler is a CPU-bound workload: it iterates SHA-256 over the input a
// configurable number of rounds. Useful for load-testing worker throughput and
// showing CPU-bound vs IO-bound task behavior.
func HashHandler(ctx context.Context, task model.Task) (Result, error) {
	var p hashPayload
	if len(task.Payload) > 0 {
		_ = json.Unmarshal(task.Payload, &p)
	}
	rounds := p.Rounds
	if rounds <= 0 {
		rounds = 100000
	}

	digest := []byte(p.Input)
	for i := 0; i < rounds; i++ {
		// Check for cancellation periodically without paying the cost every round.
		if i%4096 == 0 {
			select {
			case <-ctx.Done():
				return Result{}, ctx.Err()
			default:
			}
		}
		sum := sha256.Sum256(digest)
		digest = sum[:]
	}
	return Result{Detail: fmt.Sprintf("%d rounds -> %s", rounds, hex.EncodeToString(digest)[:16])}, nil
}
