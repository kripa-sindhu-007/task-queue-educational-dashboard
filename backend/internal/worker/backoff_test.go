package worker

import (
	"testing"
	"time"
)

func TestBackoffDelay(t *testing.T) {
	cases := []struct {
		retries int
		want    time.Duration
	}{
		{1, 2 * time.Second},
		{2, 4 * time.Second},
		{3, 8 * time.Second},
		{4, 16 * time.Second},
		{5, 32 * time.Second},
		{6, 60 * time.Second},  // 64 capped to 60
		{10, 60 * time.Second}, // stays capped
	}
	for _, c := range cases {
		if got := BackoffDelay(c.retries); got != c.want {
			t.Errorf("BackoffDelay(%d) = %v, want %v", c.retries, got, c.want)
		}
	}
}
