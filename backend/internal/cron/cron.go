// Package cron implements scheduled (cron) tasks (P4.4). A CronJob stores a
// standard cron spec and a task template; a leader-gated CronMaterializer turns
// due schedule slots into ordinary tasks on the existing enqueue path. The
// instance task ID is deterministic ({cronID}-{slotUnix}) so the same
// TaskStore.Exists idempotency check SubmitTask uses makes a failover double-fire
// a no-op.
package cron

import (
	"encoding/json"
	"time"

	"github.com/robfig/cron/v3"
)

// TaskTemplate is the shape of the task a CronJob materializes on each due slot.
// It mirrors the fields SubmitTask accepts, minus Delay (a cron instance is
// enqueued directly to ready).
type TaskTemplate struct {
	Type       string          `json:"type"`
	Payload    json.RawMessage `json:"payload,omitempty"`
	Priority   int             `json:"priority"`
	MaxRetries int             `json:"max_retries"`
}

// CronJob is a stored schedule. LastScheduledUnix is the materializer's cursor:
// the unix second of the most recent slot already enqueued (0 = none yet). Only
// the leader writes the cursor.
type CronJob struct {
	ID                string       `json:"id"`
	Schedule          string       `json:"schedule"`
	Task              TaskTemplate `json:"task"`
	Enabled           bool         `json:"enabled"`
	CreatedAt         time.Time    `json:"created_at"`
	LastScheduledUnix int64        `json:"last_scheduled_unix"`
}

// parser accepts BOTH classic 5-field cron specs and 6-field specs with an
// optional leading seconds field (SecondOptional), plus @-descriptors like
// @hourly. It is shared so validation (Parse) and materialization use the exact
// same grammar.
var parser = cron.NewParser(
	cron.SecondOptional | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor,
)

// Parse validates a cron spec and returns its schedule. It accepts 5-field and
// 6-field-with-seconds specs; an invalid spec returns an error (the API maps
// this to 400).
func Parse(spec string) (cron.Schedule, error) {
	return parser.Parse(spec)
}
