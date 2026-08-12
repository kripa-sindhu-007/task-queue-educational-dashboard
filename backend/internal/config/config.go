package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	RedisAddr  string
	RedisPass  string
	ServerPort string

	WorkerCount  int
	PollInterval time.Duration // how long a worker sleeps when the ready queue is empty
	DrainTimeout time.Duration // budget for post-cancellation Redis writes on shutdown

	// Phase 1: lease-based delivery
	VisibilityTimeout time.Duration // how long a worker has to Ack before the reaper reclaims the task
	ReaperInterval    time.Duration // how often the reaper scans for expired leases

	// Phase 2: distribution / cluster membership
	RunWorkers        bool          // run the worker pool in-process (single-binary/dev mode)
	HeartbeatInterval time.Duration // how often a node refreshes its heartbeat key
	HeartbeatTTL      time.Duration // node heartbeat key expiry (must exceed HeartbeatInterval)
	NodeGraceWindow   time.Duration // how long a dead node stays visible (alive:false) before the reaper prunes it

	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration
}

// Load reads configuration from the environment, applies defaults, and validates
// the result. It returns an error rather than panicking so main can decide.
func Load() (*Config, error) {
	cfg := &Config{
		RedisAddr:  getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPass:  getEnv("REDIS_PASSWORD", ""),
		ServerPort: getEnv("SERVER_PORT", "8080"),

		WorkerCount:  getEnvInt("WORKER_COUNT", 5),
		PollInterval: getEnvMillis("POLL_INTERVAL_MS", 500),
		DrainTimeout: getEnvMillis("DRAIN_TIMEOUT_MS", 5000),

		VisibilityTimeout: getEnvMillis("VISIBILITY_TIMEOUT_MS", 30000), // 30s default
		ReaperInterval:    getEnvMillis("REAPER_INTERVAL_MS", 5000),     // 5s default

		RunWorkers:        getEnvBool("RUN_WORKERS", true),             // in-process workers on by default (single-binary)
		HeartbeatInterval: getEnvMillis("HEARTBEAT_INTERVAL_MS", 3000), // 3s beat
		HeartbeatTTL:      getEnvMillis("HEARTBEAT_TTL_MS", 10000),     // 10s expiry (tolerates missed beats)
		NodeGraceWindow:   getEnvMillis("DEAD_NODE_GRACE_MS", 30000),   // 30s dead-card visibility before prune

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
	if c.WorkerCount <= 0 {
		return fmt.Errorf("config: WORKER_COUNT must be > 0, got %d", c.WorkerCount)
	}
	if c.PollInterval <= 0 {
		return fmt.Errorf("config: POLL_INTERVAL_MS must be > 0")
	}
	if c.DrainTimeout <= 0 {
		return fmt.Errorf("config: DRAIN_TIMEOUT_MS must be > 0")
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
