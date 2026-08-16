// Package election provides a Redis-lease leader election so that exactly one
// leader-eligible replica runs the singleton background loops (delayed scheduler
// + reaper). The lease is a coordination optimization, NOT a safety mechanism:
// during a handover two nodes may briefly overlap, and correctness in that window
// still rests on the atomic Lua (promote.lua/reclaim.lua) plus idempotency, not
// on the lease. The Elector prefers ZERO leaders to two — it steps down
// aggressively on any renew error.
package election

import (
	"context"
	_ "embed"
	"log/slog"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/store"
)

//go:embed scripts/renew.lua
var renewScript string

//go:embed scripts/release.lua
var releaseScript string

// Config tunes an Elector.
type Config struct {
	// TTL is the lease lifetime (PX on the leader key). The renew interval derives
	// as TTL/3 (no separate knob). Defaults to 10s when non-positive.
	TTL time.Duration
	// Eligible gates participation. When false, RunWhenLeader never acquires the
	// lease or runs onElected — the node is API-only and simply parks on ctx.
	Eligible bool
	// Logger is optional; nil falls back to slog.Default().
	Logger *slog.Logger
}

// Elector acquires and holds a Redis lease to coordinate single-leader work.
type Elector struct {
	client     *redis.Client
	nodeID     string
	ttl        time.Duration
	renewInt   time.Duration
	eligible   bool
	logger     *slog.Logger
	renewCmd   *redis.Script
	releaseCmd *redis.Script

	mu       sync.RWMutex
	isLeader bool
}

// New builds an Elector for nodeID against client.
func New(client *redis.Client, nodeID string, cfg Config) *Elector {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	ttl := cfg.TTL
	if ttl <= 0 {
		ttl = 10 * time.Second
	}
	renewInt := ttl / 3
	if renewInt <= 0 {
		renewInt = ttl
	}
	return &Elector{
		client:     client,
		nodeID:     nodeID,
		ttl:        ttl,
		renewInt:   renewInt,
		eligible:   cfg.Eligible,
		logger:     logger.With("node_id", nodeID),
		renewCmd:   redis.NewScript(renewScript),
		releaseCmd: redis.NewScript(releaseScript),
	}
}

// RunWhenLeader blocks until ctx is done. While this node holds the lease it runs
// onElected under a leader-scoped context (a child of ctx) that is cancelled the
// instant leadership is lost or ctx is cancelled. onElected is expected to start
// the singleton loops bound to leaderCtx and return; those loops stop when
// leaderCtx is cancelled. A leader-ineligible node never acquires and never runs
// onElected — it just parks until ctx is done.
func (e *Elector) RunWhenLeader(ctx context.Context, onElected func(leaderCtx context.Context)) error {
	if !e.eligible {
		e.logger.Info("leader election disabled (LEADER_ELIGIBLE=false); node is API-only")
		<-ctx.Done()
		return ctx.Err()
	}

	// Retry acquisition on the same cadence as renewal (~TTL/3).
	ticker := time.NewTicker(e.renewInt)
	defer ticker.Stop()

	for {
		acquired, err := e.acquire(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			e.logger.Error("leader acquire error", "error", err)
		}
		if acquired {
			e.setLeader(true)
			e.logger.Info("leadership acquired")
			released := e.runAsLeader(ctx, onElected)
			e.setLeader(false)
			e.logger.Info("leadership lost")
			if released {
				// Graceful shutdown: ctx was cancelled and the lease released.
				return ctx.Err()
			}
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

// runAsLeader runs onElected under a leader-scoped context and renews the lease
// on a ticker until leadership is lost (renew CAS fails or errors) or ctx is
// cancelled. It returns true when it stepped down because ctx was cancelled (a
// graceful shutdown, lease released), false when it stepped down on lease loss.
func (e *Elector) runAsLeader(ctx context.Context, onElected func(context.Context)) (gracefulRelease bool) {
	leaderCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// onElected starts the singleton loops bound to leaderCtx and returns; run it
	// in a goroutine so a slow start never stalls renewal, and wait for it to
	// return before we step down.
	done := make(chan struct{})
	go func() {
		defer close(done)
		onElected(leaderCtx)
	}()

	ticker := time.NewTicker(e.renewInt)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Graceful shutdown: stop the leader loops, then release the lease with a
			// fresh context so a clean restart fails over in ms, not a full TTL.
			cancel()
			<-done
			releaseCtx, releaseCancel := context.WithTimeout(context.Background(), 2*time.Second)
			e.release(releaseCtx)
			releaseCancel()
			return true
		case <-ticker.C:
			ok, err := e.renew(ctx)
			if err != nil {
				if ctx.Err() != nil {
					// Shutdown races the renew; the ctx.Done branch handles release.
					continue
				}
				// Prefer zero leaders to two: any renew error means step down now.
				e.logger.Warn("leader renew error — stepping down", "error", err)
				cancel()
				<-done
				return false
			}
			if !ok {
				e.logger.Warn("leader lease lost (renew CAS failed) — stepping down")
				cancel()
				<-done
				return false
			}
		}
	}
}

// acquire attempts SET taskqueue:leader {nodeID} NX PX {ttl}.
func (e *Elector) acquire(ctx context.Context) (bool, error) {
	return e.client.SetNX(ctx, store.KeyLeader, e.nodeID, e.ttl).Result()
}

// renew extends the lease via the owner-only CAS script. Returns false when this
// node no longer owns the lease.
func (e *Elector) renew(ctx context.Context) (bool, error) {
	res, err := e.renewCmd.Run(ctx, e.client, []string{store.KeyLeader}, e.nodeID, e.ttl.Milliseconds()).Int64()
	if err != nil {
		return false, err
	}
	return res == 1, nil
}

// release deletes the lease via the owner-only CAS script (no-op if not owned).
func (e *Elector) release(ctx context.Context) {
	if _, err := e.releaseCmd.Run(ctx, e.client, []string{store.KeyLeader}, e.nodeID).Result(); err != nil && err != redis.Nil {
		e.logger.Warn("leader release error", "error", err)
	}
}

func (e *Elector) setLeader(v bool) {
	e.mu.Lock()
	e.isLeader = v
	e.mu.Unlock()
}

// IsLeader reports whether this node currently believes it holds the lease.
func (e *Elector) IsLeader() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.isLeader
}

// ReadLeader returns the current leader's node ID (empty string when no leader
// currently holds the lease).
func (e *Elector) ReadLeader(ctx context.Context) (string, error) {
	return ReadLeader(ctx, e.client)
}

// ReadLeader returns the node ID currently holding the leader lease, or "" when
// none is held. Exported as a package func so an API-only node (which has no
// Elector) can still serve /api/leader from the shared Redis client.
func ReadLeader(ctx context.Context, client *redis.Client) (string, error) {
	v, err := client.Get(ctx, store.KeyLeader).Result()
	if err == redis.Nil {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return v, nil
}
