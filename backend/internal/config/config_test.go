package config

import (
	"testing"
	"time"
)

// TestLoad_SignalDefaults covers the P3.4 doorbell knobs: with no env override
// SignalBlock defaults to 1s and SignalCap to 1024.
func TestLoad_SignalDefaults(t *testing.T) {
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.SignalBlock != time.Second {
		t.Errorf("SignalBlock = %v, want 1s", cfg.SignalBlock)
	}
	if cfg.SignalCap != 1024 {
		t.Errorf("SignalCap = %d, want 1024", cfg.SignalCap)
	}
}

func TestLoad_RejectsNonPositiveSignalBlock(t *testing.T) {
	t.Setenv("SIGNAL_BLOCK_MS", "0")
	if _, err := Load(); err == nil {
		t.Fatal("expected error for SIGNAL_BLOCK_MS=0, got nil")
	}
}

func TestLoad_RejectsNonPositiveSignalCap(t *testing.T) {
	t.Setenv("SIGNAL_CAP", "0")
	if _, err := Load(); err == nil {
		t.Fatal("expected error for SIGNAL_CAP=0, got nil")
	}
	t.Setenv("SIGNAL_CAP", "-5")
	if _, err := Load(); err == nil {
		t.Fatal("expected error for SIGNAL_CAP=-5, got nil")
	}
}

// TestLoad_BackpressureDefaults covers the P3.6 knobs: backpressure disabled
// (MaxQueueDepth 0) by default, Retry-After 5s.
func TestLoad_BackpressureDefaults(t *testing.T) {
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.MaxQueueDepth != 0 {
		t.Errorf("MaxQueueDepth = %d, want 0 (disabled)", cfg.MaxQueueDepth)
	}
	if cfg.RetryAfterSeconds != 5 {
		t.Errorf("RetryAfterSeconds = %d, want 5", cfg.RetryAfterSeconds)
	}
}

func TestLoad_BackpressureOverrides(t *testing.T) {
	t.Setenv("MAX_QUEUE_DEPTH", "5000")
	t.Setenv("RETRY_AFTER_SECONDS", "2")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.MaxQueueDepth != 5000 || cfg.RetryAfterSeconds != 2 {
		t.Fatalf("got MaxQueueDepth=%d RetryAfterSeconds=%d, want 5000/2", cfg.MaxQueueDepth, cfg.RetryAfterSeconds)
	}
}

func TestLoad_RejectsBadBackpressure(t *testing.T) {
	t.Setenv("MAX_QUEUE_DEPTH", "-1")
	if _, err := Load(); err == nil {
		t.Fatal("expected error for MAX_QUEUE_DEPTH=-1, got nil")
	}
	t.Setenv("MAX_QUEUE_DEPTH", "0") // reset to valid
	t.Setenv("RETRY_AFTER_SECONDS", "0")
	if _, err := Load(); err == nil {
		t.Fatal("expected error for RETRY_AFTER_SECONDS=0, got nil")
	}
}
