package model

import (
	"encoding/json"
	"time"
)

type TaskStatus string

const (
	StatusPending    TaskStatus = "pending"
	StatusProcessing TaskStatus = "processing"
	StatusCompleted  TaskStatus = "completed"
	StatusFailed     TaskStatus = "failed"
)

type Task struct {
	ID         string          `json:"id"`
	Type       string          `json:"type,omitempty"`
	Payload    json.RawMessage `json:"payload,omitempty"`
	Priority   int             `json:"priority"`
	Delay      int             `json:"delay"`       // seconds before execution
	MaxRetries int             `json:"max_retries"` // 0 = no retries
	Retries    int             `json:"retries"`
	Status     TaskStatus      `json:"status"`
	CreatedAt  time.Time       `json:"created_at"`
	Error      string          `json:"error,omitempty"`
	Owner      string          `json:"owner,omitempty"` // node ID currently leasing the task (P2.2)
}

type Metrics struct {
	TotalProcessed int64 `json:"total_processed"`
	TotalFailed    int64 `json:"total_failed"`
	TotalRetries   int64 `json:"total_retries"`
	QueueSize      int64 `json:"queue_size"`
	ActiveWorkers  int64 `json:"active_workers"`
}

type FailedTask struct {
	Task     Task      `json:"task"`
	FailedAt time.Time `json:"failed_at"`
	Reason   string    `json:"reason"`
}

type TaskEvent struct {
	ID        string    `json:"id"`
	TaskID    string    `json:"task_id"`
	Type      string    `json:"type"` // submitted, started, completed, failed, retrying, dead_lettered, promoted
	WorkerID  int       `json:"worker_id"`
	Detail    string    `json:"detail"`
	Timestamp time.Time `json:"timestamp"`
}

type WorkerState struct {
	ID        int       `json:"id"`
	Status    string    `json:"status"` // idle, processing
	TaskID    string    `json:"task_id,omitempty"`
	StartedAt time.Time `json:"started_at,omitempty"`
}

// Node is a standalone worker process that joins the cluster (P2.2). Identity is
// {hostname}-{shortuuid}. Presence is tracked by a heartbeat TTL key; liveness
// is derived from whether that key still exists.
type Node struct {
	ID        string    `json:"id"`
	Hostname  string    `json:"hostname"`
	Role      string    `json:"role,omitempty"` // "worker" (executes tasks) or "server" (presence-only scheduler/API, capacity 0); P4.1
	Capacity  int       `json:"capacity"`       // number of executor goroutines
	StartedAt time.Time `json:"started_at"`
	// Runtime fields, populated by NodeStore.ListNodes (not stored in the record):
	Alive         bool  `json:"alive"`
	InFlightTasks int64 `json:"in_flight_tasks"`
}

type DelayedEntry struct {
	Task      Task    `json:"task"`
	ExecuteAt float64 `json:"execute_at"`
}

type EnhancedMetrics struct {
	Metrics
	SuccessRate      float64 `json:"success_rate"`
	DelayedQueueSize int64   `json:"delayed_queue_size"`
	DeadLetterSize   int64   `json:"dead_letter_size"`
	TotalSubmitted   int64   `json:"total_submitted"`
}
