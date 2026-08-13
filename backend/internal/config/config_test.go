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
