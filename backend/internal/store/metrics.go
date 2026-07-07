package store

import (
	"context"
	"fmt"
	"strconv"

	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/model"
	"github.com/redis/go-redis/v9"
)

type MetricsStore struct {
	client *redis.Client
}

func NewMetricsStore(client *redis.Client) *MetricsStore {
	return &MetricsStore{client: client}
}

func (m *MetricsStore) IncrProcessed(ctx context.Context) error {
	return m.client.HIncrBy(ctx, KeyMetrics, "processed", 1).Err()
}

func (m *MetricsStore) IncrFailed(ctx context.Context) error {
	return m.client.HIncrBy(ctx, KeyMetrics, "failed", 1).Err()
}

func (m *MetricsStore) IncrRetries(ctx context.Context) error {
	return m.client.HIncrBy(ctx, KeyMetrics, "retries", 1).Err()
}

func (m *MetricsStore) IncrSubmitted(ctx context.Context) error {
	return m.client.HIncrBy(ctx, KeyMetrics, "submitted", 1).Err()
}

func (m *MetricsStore) Get(ctx context.Context, queueSize, activeWorkers int64) (*model.Metrics, error) {
	vals, err := m.client.HGetAll(ctx, KeyMetrics).Result()
	if err != nil {
		return nil, fmt.Errorf("hgetall metrics: %w", err)
	}
	return &model.Metrics{
		TotalProcessed: parseI64(vals["processed"]),
		TotalFailed:    parseI64(vals["failed"]),
		TotalRetries:   parseI64(vals["retries"]),
		QueueSize:      queueSize,
		ActiveWorkers:  activeWorkers,
	}, nil
}

func (m *MetricsStore) GetEnhanced(ctx context.Context, queueSize, activeWorkers, delayedSize, deadLetterSize int64) (*model.EnhancedMetrics, error) {
	vals, err := m.client.HGetAll(ctx, KeyMetrics).Result()
	if err != nil {
		return nil, fmt.Errorf("hgetall metrics: %w", err)
	}

	processed := parseI64(vals["processed"])
	failed := parseI64(vals["failed"])
	submitted := parseI64(vals["submitted"])

	var successRate float64
	total := processed + failed
	if total > 0 {
		successRate = float64(processed) / float64(total) * 100
	}

	return &model.EnhancedMetrics{
		Metrics: model.Metrics{
			TotalProcessed: processed,
			TotalFailed:    failed,
			TotalRetries:   parseI64(vals["retries"]),
			QueueSize:      queueSize,
			ActiveWorkers:  activeWorkers,
		},
		SuccessRate:      successRate,
		DelayedQueueSize: delayedSize,
		DeadLetterSize:   deadLetterSize,
		TotalSubmitted:   submitted,
	}, nil
}

func parseI64(s string) int64 {
	n, _ := strconv.ParseInt(s, 10, 64)
	return n
}
