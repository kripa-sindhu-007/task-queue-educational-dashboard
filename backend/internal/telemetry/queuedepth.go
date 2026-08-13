package telemetry

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// QueueDepthSource reads the current ready-queue depth. *store.QueuePeekStore
// satisfies it via ReadySize; the interface keeps telemetry free of a store
// import (and avoids an import cycle).
type QueueDepthSource interface {
	ReadySize(ctx context.Context) (int64, error)
}

// queueDepthCollector emits queue_depth by reading ZCARD(ready) at scrape time
// rather than maintaining a periodically-set gauge: no background goroutine and
// the value is never stale. It is registered only on the broker/server, where a
// QueuePeekStore exists.
type queueDepthCollector struct {
	source  QueueDepthSource
	desc    *prometheus.Desc
	timeout time.Duration
}

// RegisterQueueDepth wires the queue_depth collector onto reg, reading depth
// through source at scrape time.
func RegisterQueueDepth(reg *prometheus.Registry, source QueueDepthSource) {
	reg.MustRegister(&queueDepthCollector{
		source:  source,
		desc:    prometheus.NewDesc("queue_depth", "Number of tasks currently in the ready queue.", nil, nil),
		timeout: 2 * time.Second,
	})
}

func (c *queueDepthCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.desc
}

func (c *queueDepthCollector) Collect(ch chan<- prometheus.Metric) {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	n, err := c.source.ReadySize(ctx)
	if err != nil {
		// Skip this sample on a Redis error rather than emitting a misleading
		// zero; the scrape simply omits queue_depth for this tick.
		return
	}
	ch <- prometheus.MustNewConstMetric(c.desc, prometheus.GaugeValue, float64(n))
}
