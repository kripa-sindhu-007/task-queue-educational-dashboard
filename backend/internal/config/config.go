package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	RedisAddr   string
	RedisPass   string
	ServerPort  string
	MetricsPort string // port the standalone worker serves /metrics on (server uses ServerPort)

	WorkerCount  int
	PollInterval time.Duration // P3.4: now only the short back-off sleep after a Dequeue *error* (the empty-queue wait is the doorbell)
	DrainTimeout time.Duration // budget for post-cancellation Redis writes on shutdown

	// P3.4: blocking task pickup (doorbell). SignalBlock is the single knob that
	// is simultaneously the BLPOP block timeout, the shutdown-check granularity
	// and the fallback-poll backstop; SignalCap bounds the doorbell list.
	SignalBlock time.Duration // how long a worker blocks on the doorbell before looping to re-check ctx and re-poll
	SignalCap   int           // max wake-up tokens retained in the doorbell list

	// Phase 1: lease-based delivery
	VisibilityTimeout time.Duration // how long a worker has to Ack before the reaper reclaims the task
	ReaperInterval    time.Duration // how often the reaper scans for expired leases

	// Phase 2: distribution / cluster membership
	RunWorkers        bool          // run the worker pool in-process (single-binary/dev mode)
	HeartbeatInterval time.Duration // how often a node refreshes its heartbeat key
	HeartbeatTTL      time.Duration // node heartbeat key expiry (must exceed HeartbeatInterval)
	NodeGraceWindow   time.Duration // how long a dead node stays visible (alive:false) before the reaper prunes it

	// Phase 3: backpressure (P3.6). When MaxQueueDepth > 0, submissions are shed
	// with HTTP 429 + Retry-After once the ready queue reaches that depth. 0
	// disables backpressure (unbounded intake — the pre-P3.6 behavior).
	MaxQueueDepth     int // ready-queue depth at/above which new submits are rejected; 0 = disabled
	RetryAfterSeconds int // value for the Retry-After header on a 429

	// Phase 4: leader election (P4.1). Exactly one leader-eligible node holds the
	// Redis lease at a time and runs the singleton loops (delayed scheduler +
	// reaper). LeaderEligible=false makes a pure API node that never competes.
	LeaderEligible bool          // whether this node competes for leadership and runs the singleton loops
	LeaderTTL      time.Duration // leader lease TTL; renew interval derives as TTL/3

	// Phase 4: cron jobs (P4.4). The leader-gated cron materializer scans stored
	// schedule specs on this tick and enqueues the latest due slot as an ordinary
	// task on the existing enqueue path.
	CronTick time.Duration // how often the cron materializer scans for due schedules

	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration
}

// Load reads configuration from the environment, applies defaults, and validates
// the result. It returns an error rather than panicking so main can decide.
func Load() (*Config, error) {
	cfg := &Config{
		RedisAddr:   getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPass:   getEnv("REDIS_PASSWORD", ""),
		ServerPort:  getEnv("SERVER_PORT", "8080"),
		MetricsPort: getEnv("METRICS_PORT", "9100"),

		WorkerCount:  getEnvInt("WORKER_COUNT", 5),
		PollInterval: getEnvMillis("POLL_INTERVAL_MS", 500),
		DrainTimeout: getEnvMillis("DRAIN_TIMEOUT_MS", 5000),

		SignalBlock: getEnvMillis("SIGNAL_BLOCK_MS", 1000), // 1s: doorbell block = shutdown-check = fallback-poll interval
		SignalCap:   getEnvInt("SIGNAL_CAP", 1024),         // doorbell list bound

		VisibilityTimeout: getEnvMillis("VISIBILITY_TIMEOUT_MS", 30000), // 30s default
		ReaperInterval:    getEnvMillis("REAPER_INTERVAL_MS", 5000),     // 5s default

		RunWorkers:        getEnvBool("RUN_WORKERS", true),             // in-process workers on by default (single-binary)
		HeartbeatInterval: getEnvMillis("HEARTBEAT_INTERVAL_MS", 3000), // 3s beat
		HeartbeatTTL:      getEnvMillis("HEARTBEAT_TTL_MS", 10000),     // 10s expiry (tolerates missed beats)
		NodeGraceWindow:   getEnvMillis("DEAD_NODE_GRACE_MS", 30000),   // 30s dead-card visibility before prune

		MaxQueueDepth:     getEnvInt("MAX_QUEUE_DEPTH", 0),     // 0 = backpressure disabled
		RetryAfterSeconds: getEnvInt("RETRY_AFTER_SECONDS", 5), // Retry-After header on a 429

		LeaderEligible: getEnvBool("LEADER_ELIGIBLE", true),  // compete for leadership by default (single-binary wins trivially)
		LeaderTTL:      getEnvMillis("LEADER_TTL_MS", 10000), // 10s lease; renewed every ~3.3s (TTL/3)

		CronTick: getEnvMillis("CRON_TICK_MS", 1000), // 1s default cron scan cadence

		ReadTimeout:     getEnvMillis("HTTP_READ_TIMEOUT_MS", 10000),
		WriteTimeout:    getEnvMillis("HTTP_WRITE_TIMEOUT_MS", 15000),
		IdleTimeout:     getEnvMillis("HTTP_IDLE_TIMEOUT_MS", 60000),
		ShutdownTimeout: getEnvMillis("HTTP_SHUTDOWN_TIMEOUT_MS", 10000),
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) validate() error {
	if c.RedisAddr == "" {
		return fmt.Errorf("config: REDIS_ADDR must not be empty")
	}
	if c.ServerPort == "" {
		return fmt.Errorf("config: SERVER_PORT must not be empty")
	}
	if c.MetricsPort == "" {
		return fmt.Errorf("config: METRICS_PORT must not be empty")
	}
	if c.WorkerCount <= 0 {
		return fmt.Errorf("config: WORKER_COUNT must be > 0, got %d", c.WorkerCount)
	}
	if c.PollInterval <= 0 {
		return fmt.Errorf("config: POLL_INTERVAL_MS must be > 0")
	}
	if c.DrainTimeout <= 0 {
		return fmt.Errorf("config: DRAIN_TIMEOUT_MS must be > 0")
	}
	if c.SignalBlock <= 0 {
		return fmt.Errorf("config: SIGNAL_BLOCK_MS must be > 0")
	}
	if c.SignalCap <= 0 {
		return fmt.Errorf("config: SIGNAL_CAP must be > 0, got %d", c.SignalCap)
	}
	if c.VisibilityTimeout <= 0 {
		return fmt.Errorf("config: VISIBILITY_TIMEOUT_MS must be > 0")
	}
	if c.ReaperInterval <= 0 {
		return fmt.Errorf("config: REAPER_INTERVAL_MS must be > 0")
	}
	if c.HeartbeatInterval <= 0 {
		return fmt.Errorf("config: HEARTBEAT_INTERVAL_MS must be > 0")
	}
	if c.HeartbeatTTL <= c.HeartbeatInterval {
		return fmt.Errorf("config: HEARTBEAT_TTL_MS (%v) must exceed HEARTBEAT_INTERVAL_MS (%v) to tolerate a missed beat",
			c.HeartbeatTTL, c.HeartbeatInterval)
	}
	if c.NodeGraceWindow <= 0 {
		return fmt.Errorf("config: DEAD_NODE_GRACE_MS must be > 0")
	}
	if c.MaxQueueDepth < 0 {
		return fmt.Errorf("config: MAX_QUEUE_DEPTH must be >= 0, got %d", c.MaxQueueDepth)
	}
	if c.RetryAfterSeconds <= 0 {
		return fmt.Errorf("config: RETRY_AFTER_SECONDS must be > 0, got %d", c.RetryAfterSeconds)
	}
	if c.LeaderTTL <= 0 {
		return fmt.Errorf("config: LEADER_TTL_MS must be > 0")
	}
	if c.CronTick <= 0 {
		return fmt.Errorf("config: CRON_TICK_MS must be > 0")
	}
	return nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

// getEnvBool reads a boolean env var (accepts 1/0, true/false, etc. per
// strconv.ParseBool), falling back when unset or unparseable.
func getEnvBool(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return fallback
}

// getEnvMillis reads an integer number of milliseconds into a time.Duration.
func getEnvMillis(key string, fallbackMs int) time.Duration {
	return time.Duration(getEnvInt(key, fallbackMs)) * time.Millisecond
}
