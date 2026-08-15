// Command loadgen is a hand-rolled load generator for the task queue (P3.5).
//
// It submits tasks to POST /api/tasks at a controlled rate — constant (-rate)
// or a linear ramp (-ramp start:end over -duration) — with a configurable
// task-type mix (-mix). Its purpose is to drive the observability stack: as the
// submit rate outpaces worker throughput, queue_depth climbs and
// enqueue_to_start p99 rises on the Grafana dashboard; kill a worker mid-run to
// see the p99 spike and the reaper-reclaim recovery.
//
// No external load-testing deps (no k6/vegeta) — stdlib only, matching the
// repo's stdlib-first bias.
//
// Examples:
//
//	go run ./cmd/loadgen -ramp 100:2000 -duration 90s
//	go run ./cmd/loadgen -rate 500 -duration 30s -mix hash:70,sleep:30
//	docker compose run --rm loadgen -ramp 100:2000 -duration 90s
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// Built-in task types (mirrors internal/handler builtin identifiers).
const (
	typeSleep     = "sleep"
	typeHash      = "hash"
	typeHTTPFetch = "http_fetch"
)

func main() {
	var (
		urlFlag     = flag.String("url", envOr("LOADGEN_URL", "http://localhost:8080"), "task queue API base URL")
		rate        = flag.Float64("rate", 200, "constant submissions per second (ignored when -ramp is set)")
		ramp        = flag.String("ramp", "", "linear ramp start:end submissions/sec over -duration, e.g. 100:2000")
		duration    = flag.Duration("duration", 60*time.Second, "how long to run; 0 = until Ctrl-C")
		mix         = flag.String("mix", "hash:50,sleep:50", "weighted task-type mix, e.g. hash:60,sleep:30,http_fetch:10")
		concurrency = flag.Int("concurrency", 50, "max in-flight HTTP submitters")
		priority    = flag.Int("priority", -1, "task priority 0-9; -1 = random per task")
		maxRetries  = flag.Int("max-retries", 3, "max retries per task")
		sleepMS     = flag.Int("sleep-ms", 0, "sleep task duration_ms; 0 = handler default (random 200-800ms)")
		failRate    = flag.Float64("fail-rate", 0, "sleep task fail_rate 0..1; 0 = handler default (0.3)")
		hashRounds  = flag.Int("hash-rounds", 0, "hash task rounds; 0 = handler default (100000)")
		fetchURL    = flag.String("fetch-url", "", "URL for http_fetch tasks (required if http_fetch is in the mix)")
		seed        = flag.Int64("seed", time.Now().UnixNano(), "PRNG seed for reproducible runs")
	)
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	entries, total, err := parseMix(*mix)
	if err != nil {
		logger.Error("invalid -mix", "error", err)
		os.Exit(2)
	}
	for _, e := range entries {
		if e.typ == typeHTTPFetch && *fetchURL == "" {
			logger.Error("http_fetch is in the mix but -fetch-url is empty")
			os.Exit(2)
		}
	}

	rampEnabled := *ramp != ""
	rateStart, rateEnd := *rate, *rate
	if rampEnabled {
		rateStart, rateEnd, err = parseRamp(*ramp)
		if err != nil {
			logger.Error("invalid -ramp", "error", err)
			os.Exit(2)
		}
		if *duration <= 0 {
			logger.Error("-ramp requires a positive -duration")
			os.Exit(2)
		}
	}
	if rateStart <= 0 && rateEnd <= 0 {
		logger.Error("rate must be positive")
		os.Exit(2)
	}

	submitURL := strings.TrimRight(*urlFlag, "/") + "/api/tasks"
	client := &http.Client{
		Timeout: 15 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        *concurrency * 2,
			MaxIdleConnsPerHost: *concurrency * 2,
			IdleConnTimeout:     30 * time.Second,
		},
	}

	// Signal-cancellable context for the run.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	logger.Info("loadgen starting",
		"url", submitURL,
		"mode", modeString(rampEnabled, rateStart, rateEnd),
		"duration", duration.String(),
		"mix", *mix,
		"concurrency", *concurrency,
	)

	// Counters (addressed atomically).
	var attempted, ok, tooMany, failed, inflight, behind int64
	lr := newLatencyRecorder(200000, rand.New(rand.NewSource(*seed)))

	// Token channel: one token == "submit one task now". Senders build the task.
	jobs := make(chan struct{}, *concurrency*4)
	runStart := time.Now()

	// Rate scheduler: emits tokens at the (possibly ramping) target rate using a
	// fractional accumulator so fractional per-tick rates are honored over time.
	go func() {
		defer close(jobs)
		const tick = 5 * time.Millisecond
		t := time.NewTicker(tick)
		defer t.Stop()
		last := runStart
		var due float64
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-t.C:
				elapsed := now.Sub(runStart)
				if *duration > 0 && elapsed >= *duration {
					return
				}
				tr := targetRate(elapsed, *duration, rateStart, rateEnd, rampEnabled)
				due += tr * now.Sub(last).Seconds()
				last = now
				// Cap carry-over to ~1s of target so a stall can't unleash a burst.
				if due > tr+1 {
					due = tr + 1
				}
			emit:
				for due >= 1 {
					select {
					case jobs <- struct{}{}:
						due--
					default:
						// Senders saturated — can't hit target rate this tick.
						atomic.AddInt64(&behind, 1)
						break emit
					}
				}
			}
		}
	}()

	// Submitter pool.
	var wg sync.WaitGroup
	for i := 0; i < *concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			r := rand.New(rand.NewSource(*seed + int64(id) + 1))
			for range jobs {
				body := buildTask(r, entries, total, *priority, *maxRetries, *sleepMS, *failRate, *hashRounds, *fetchURL)
				atomic.AddInt64(&inflight, 1)
				lat, status, err := submit(ctx, client, submitURL, body)
				atomic.AddInt64(&inflight, -1)
				atomic.AddInt64(&attempted, 1)
				switch {
				case err != nil:
					atomic.AddInt64(&failed, 1)
				case status == http.StatusTooManyRequests:
					atomic.AddInt64(&tooMany, 1)
				case status >= 200 && status < 300:
					atomic.AddInt64(&ok, 1)
					lr.record(lat)
				default:
					atomic.AddInt64(&failed, 1)
				}
			}
		}(i)
	}

	// Per-second progress printer.
	printerCtx, stopPrinter := context.WithCancel(context.Background())
	go func() {
		tk := time.NewTicker(time.Second)
		defer tk.Stop()
		var lastAttempted int64
		for {
			select {
			case <-printerCtx.Done():
				return
			case <-tk.C:
				el := time.Since(runStart)
				a := atomic.LoadInt64(&attempted)
				logger.Info("progress",
					"elapsed", el.Round(time.Second).String(),
					"target_rate", int(targetRate(el, *duration, rateStart, rateEnd, rampEnabled)),
					"actual_rate", a-lastAttempted,
					"submitted", a,
					"ok", atomic.LoadInt64(&ok),
					"http_429", atomic.LoadInt64(&tooMany),
					"errors", atomic.LoadInt64(&failed),
					"inflight", atomic.LoadInt64(&inflight),
					"behind_ticks", atomic.LoadInt64(&behind),
				)
				lastAttempted = a
			}
		}
	}()

	// Wait for the scheduler (duration/signal) to close jobs and the pool to drain.
	wg.Wait()
	stopPrinter()

	el := time.Since(runStart)
	p := lr.percentiles(50, 95, 99)
	logger.Info("loadgen complete",
		"elapsed", el.Round(10*time.Millisecond).String(),
		"submitted", atomic.LoadInt64(&attempted),
		"ok", atomic.LoadInt64(&ok),
		"http_429", atomic.LoadInt64(&tooMany),
		"errors", atomic.LoadInt64(&failed),
		"effective_rate", int(float64(atomic.LoadInt64(&attempted))/el.Seconds()),
		"submit_p50", roundMS(p[0]),
		"submit_p95", roundMS(p[1]),
		"submit_p99", roundMS(p[2]),
		"submit_max", roundMS(lr.max()),
		"behind_ticks", atomic.LoadInt64(&behind),
	)
}

// --- rate & mix helpers (pure, unit-tested) --------------------------------

type mixEntry struct {
	typ    string
	weight int
}

// parseMix parses "hash:60,sleep:30,http_fetch:10" into weighted entries and
// their total weight.
func parseMix(s string) ([]mixEntry, int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, 0, errors.New("empty mix")
	}
	var entries []mixEntry
	total := 0
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		kv := strings.SplitN(part, ":", 2)
		if len(kv) != 2 {
			return nil, 0, fmt.Errorf("bad mix entry %q (want type:weight)", part)
		}
		typ := strings.TrimSpace(kv[0])
		switch typ {
		case typeSleep, typeHash, typeHTTPFetch:
		default:
			return nil, 0, fmt.Errorf("unknown task type %q (want sleep|hash|http_fetch)", typ)
		}
		w, err := strconv.Atoi(strings.TrimSpace(kv[1]))
		if err != nil || w <= 0 {
			return nil, 0, fmt.Errorf("bad weight in %q (want a positive integer)", part)
		}
		entries = append(entries, mixEntry{typ: typ, weight: w})
		total += w
	}
	if total == 0 {
		return nil, 0, errors.New("mix has zero total weight")
	}
	return entries, total, nil
}

// parseRamp parses "start:end" into two non-negative rates.
func parseRamp(s string) (float64, float64, error) {
	kv := strings.SplitN(strings.TrimSpace(s), ":", 2)
	if len(kv) != 2 {
		return 0, 0, fmt.Errorf("bad -ramp %q (want start:end)", s)
	}
	start, err1 := strconv.ParseFloat(strings.TrimSpace(kv[0]), 64)
	end, err2 := strconv.ParseFloat(strings.TrimSpace(kv[1]), 64)
	if err1 != nil || err2 != nil || start < 0 || end < 0 {
		return 0, 0, fmt.Errorf("bad -ramp %q (want non-negative start:end)", s)
	}
	return start, end, nil
}

// targetRate returns the instantaneous target submit rate. In constant mode (or
// with a non-positive duration) it returns start; in ramp mode it linearly
// interpolates start→end across the run, clamped to [start,end] at the edges.
func targetRate(elapsed, duration time.Duration, start, end float64, ramp bool) float64 {
	if !ramp || duration <= 0 {
		return start
	}
	frac := float64(elapsed) / float64(duration)
	if frac < 0 {
		frac = 0
	}
	if frac > 1 {
		frac = 1
	}
	return start + (end-start)*frac
}

// --- task construction -----------------------------------------------------

func pickType(r *rand.Rand, entries []mixEntry, total int) string {
	n := r.Intn(total)
	for _, e := range entries {
		if n < e.weight {
			return e.typ
		}
		n -= e.weight
	}
	return entries[len(entries)-1].typ
}

func buildTask(r *rand.Rand, entries []mixEntry, total, prio, maxRetries, sleepMS int, failRate float64, hashRounds int, fetchURL string) []byte {
	typ := pickType(r, entries, total)

	payload := map[string]any{}
	switch typ {
	case typeSleep:
		if sleepMS > 0 {
			payload["duration_ms"] = sleepMS
		}
		if failRate > 0 {
			payload["fail_rate"] = failRate
		}
	case typeHash:
		payload["input"] = strconv.FormatInt(r.Int63(), 36)
		if hashRounds > 0 {
			payload["rounds"] = hashRounds
		}
	case typeHTTPFetch:
		payload["url"] = fetchURL
	}

	p := prio
	if p < 0 {
		p = r.Intn(10)
	}
	payloadJSON, _ := json.Marshal(payload)
	body, _ := json.Marshal(map[string]any{
		"type":        typ,
		"priority":    p,
		"payload":     json.RawMessage(payloadJSON),
		"max_retries": maxRetries,
	})
	return body
}

func submit(ctx context.Context, client *http.Client, url string, body []byte) (time.Duration, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return 0, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	start := time.Now()
	resp, err := client.Do(req)
	lat := time.Since(start)
	if err != nil {
		return lat, 0, err
	}
	// Drain (bounded) so the connection is reusable.
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))
	_ = resp.Body.Close()
	return lat, resp.StatusCode, nil
}

// --- latency recorder (reservoir-sampled, bounded memory) ------------------

type latencyRecorder struct {
	mu   sync.Mutex
	buf  []float64 // seconds
	cap  int
	seen int64
	r    *rand.Rand
	maxV float64
}

func newLatencyRecorder(capN int, r *rand.Rand) *latencyRecorder {
	return &latencyRecorder{buf: make([]float64, 0, capN), cap: capN, r: r}
}

func (l *latencyRecorder) record(d time.Duration) {
	sec := d.Seconds()
	l.mu.Lock()
	l.seen++
	if sec > l.maxV {
		l.maxV = sec
	}
	if len(l.buf) < l.cap {
		l.buf = append(l.buf, sec)
	} else if j := l.r.Int63n(l.seen); j < int64(l.cap) {
		l.buf[j] = sec // reservoir replacement
	}
	l.mu.Unlock()
}

func (l *latencyRecorder) max() float64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.maxV
}

func (l *latencyRecorder) percentiles(ps ...float64) []float64 {
	l.mu.Lock()
	cp := append([]float64(nil), l.buf...)
	l.mu.Unlock()
	out := make([]float64, len(ps))
	if len(cp) == 0 {
		return out
	}
	sort.Float64s(cp)
	for i, p := range ps {
		idx := int(p/100*float64(len(cp)-1) + 0.5)
		if idx < 0 {
			idx = 0
		}
		if idx >= len(cp) {
			idx = len(cp) - 1
		}
		out[i] = cp[idx]
	}
	return out
}

// --- misc ------------------------------------------------------------------

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func modeString(ramp bool, start, end float64) string {
	if ramp {
		return fmt.Sprintf("ramp %g->%g/s", start, end)
	}
	return fmt.Sprintf("constant %g/s", start)
}

func roundMS(sec float64) string {
	return (time.Duration(sec * float64(time.Second))).Round(time.Millisecond).String()
}
