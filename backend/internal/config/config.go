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

// getEnvMillis reads an integer number of milliseconds into a time.Duration.
func getEnvMillis(key string, fallbackMs int) time.Duration {
	return time.Duration(getEnvInt(key, fallbackMs)) * time.Millisecond
}
