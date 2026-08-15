package main

import (
	"math"
	"math/rand"
	"testing"
	"time"
)

func TestParseMix(t *testing.T) {
	entries, total, err := parseMix("hash:60,sleep:30,http_fetch:10")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 100 {
		t.Fatalf("total = %d, want 100", total)
	}
	if len(entries) != 3 {
		t.Fatalf("entries = %d, want 3", len(entries))
	}
	if entries[0].typ != typeHash || entries[0].weight != 60 {
		t.Fatalf("entries[0] = %+v, want {hash 60}", entries[0])
	}

	// Whitespace and single-entry forms are tolerated.
	if _, total, err := parseMix(" sleep : 5 "); err != nil || total != 5 {
		t.Fatalf("whitespace mix: total=%d err=%v", total, err)
	}
}

func TestParseMixErrors(t *testing.T) {
	for _, s := range []string{
		"",               // empty
		"hash",           // missing weight
		"hash:0",         // non-positive
		"hash:-5",        // negative
		"hash:abc",       // non-numeric
		"bogus:10",       // unknown type
		"sleep:0,hash:0", // zero total
	} {
		if _, _, err := parseMix(s); err == nil {
			t.Errorf("parseMix(%q) = nil error, want error", s)
		}
	}
}

func TestParseRamp(t *testing.T) {
	start, end, err := parseRamp("100:2000")
	if err != nil || start != 100 || end != 2000 {
		t.Fatalf("parseRamp = (%g,%g,%v), want (100,2000,nil)", start, end, err)
	}
	for _, s := range []string{"100", "a:b", "-1:10", "100:-2"} {
		if _, _, err := parseRamp(s); err == nil {
			t.Errorf("parseRamp(%q) = nil error, want error", s)
		}
	}
}

func TestTargetRateRamp(t *testing.T) {
	dur := 100 * time.Second
	cases := []struct {
		elapsed time.Duration
		want    float64
	}{
		{0, 100},
		{50 * time.Second, 1050},
		{100 * time.Second, 2000},
		{200 * time.Second, 2000}, // clamped past the end
	}
	for _, c := range cases {
		got := targetRate(c.elapsed, dur, 100, 2000, true)
		if math.Abs(got-c.want) > 1e-9 {
			t.Errorf("targetRate(%v) = %g, want %g", c.elapsed, got, c.want)
		}
	}
}

func TestTargetRateConstant(t *testing.T) {
	// ramp=false always returns start, regardless of elapsed/duration.
	if got := targetRate(37*time.Second, time.Minute, 500, 9999, false); got != 500 {
		t.Errorf("constant targetRate = %g, want 500", got)
	}
	// non-positive duration is treated as constant.
	if got := targetRate(time.Second, 0, 250, 2000, true); got != 250 {
		t.Errorf("zero-duration targetRate = %g, want 250", got)
	}
}

func TestPickTypeRespectsWeights(t *testing.T) {
	entries, total, err := parseMix("hash:100")
	if err != nil {
		t.Fatal(err)
	}
	// A single-type mix must always pick that type.
	r := rand.New(rand.NewSource(1))
	for i := 0; i < 100; i++ {
		if got := pickType(r, entries, total); got != typeHash {
			t.Fatalf("pickType = %q, want hash", got)
		}
	}
}
