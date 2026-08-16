package worker

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/model"
	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/store"
)

// NodeConfig wires a Node's collaborators and timing.
type NodeConfig struct {
	Nodes *store.NodeStore
	// Pool executes tasks. A nil Pool makes a presence-only node (role "server"):
	// it registers, heartbeats and appears in /api/nodes for the leader crown, but
	// never starts the executor loop (P4.1 scheduler replicas run capacity 0).
	Pool              *Pool
	Events            *store.EventStore // optional: emits node_joined on successful registration
	NodeID            string
	Hostname          string
	Role              string        // "worker" or "server"; defaults to "worker" when empty
	Capacity          int           // executor goroutines (== pool worker count; 0 for a presence-only node)
	HeartbeatInterval time.Duration // how often to refresh the heartbeat key
	Logger            *slog.Logger  // optional: nil falls back to slog.Default()
}

// Node represents this process's membership in the cluster: it registers itself,
// keeps a heartbeat alive while it runs, executes tasks via its Pool, and
// deregisters on graceful shutdown. If the process dies without deregistering,
// the heartbeat key expires and the reaper reclaims this node's in-flight work.
type Node struct {
	cfg    NodeConfig
	logger *slog.Logger
}

func NewNode(cfg NodeConfig) *Node {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &Node{cfg: cfg, logger: logger.With("node_id", cfg.NodeID)}
}

// Run registers the node, starts the heartbeat loop and worker pool, and blocks
// until ctx is cancelled and in-flight work has drained. It then deregisters.
func (n *Node) Run(ctx context.Context) {
	role := n.cfg.Role
	if role == "" {
		role = "worker"
	}
	node := model.Node{
		ID:        n.cfg.NodeID,
		Hostname:  n.cfg.Hostname,
		Role:      role,
		Capacity:  n.cfg.Capacity,
		StartedAt: time.Now(),
	}

	if err := n.cfg.Nodes.Register(ctx, node); err != nil {
		n.logger.Error("node registration failed", "error", err)
	} else {
		n.logger.Info("node registered", "host", n.cfg.Hostname, "capacity", n.cfg.Capacity)
		if n.cfg.Events != nil {
			n.cfg.Events.Push(ctx, model.TaskEvent{
				ID:        fmt.Sprintf("evt-%d", time.Now().UnixNano()),
				Type:      "node_joined",
				WorkerID:  -1,
				Detail:    fmt.Sprintf("Node %s joined (host=%s, capacity=%d)", n.cfg.NodeID, n.cfg.Hostname, n.cfg.Capacity),
				Timestamp: time.Now(),
			})
		}
	}

	// Heartbeat loop runs independently of task execution so a busy node still
	// refreshes its presence. It stops when ctx is cancelled.
	hbDone := make(chan struct{})
	go func() {
		defer close(hbDone)
		n.heartbeatLoop(ctx, node)
	}()

	// Start executors and block until they drain after ctx cancellation. A
	// presence-only node (nil Pool) has no executors: it just holds its heartbeat
	// alive until ctx is cancelled, then deregisters like any other node.
	if n.cfg.Pool != nil {
		n.cfg.Pool.Start(ctx)
		n.cfg.Pool.Wait()
	} else {
		<-ctx.Done()
	}

	// Wait for the heartbeat loop to observe cancellation before deregistering,
	// so it can't re-create the heartbeat key after we delete it.
	<-hbDone

	// Deregister using a fresh context: ctx is already cancelled at this point.
	deregCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := n.cfg.Nodes.Deregister(deregCtx, n.cfg.NodeID); err != nil {
		n.logger.Error("node deregister failed", "error", err)
	} else {
		n.logger.Info("node deregistered")
	}
}

func (n *Node) heartbeatLoop(ctx context.Context, node model.Node) {
	ticker := time.NewTicker(n.cfg.HeartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := n.cfg.Nodes.Heartbeat(ctx, node); err != nil {
				n.logger.Error("node heartbeat error", "error", err)
			}
		}
	}
}
