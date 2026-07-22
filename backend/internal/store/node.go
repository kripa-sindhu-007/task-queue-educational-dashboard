package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/model"
)

// NodeStore manages cluster membership (P2.2/P2.6). Each node is tracked by
// three keys:
//
//   - taskqueue:nodes            — a SET of all known node IDs (the registry)
//   - taskqueue:node:{id}:hb     — a string key with TTL; its existence == alive
//   - taskqueue:node:{id}:tasks  — a SET of task IDs the node currently leases
//
// A node is considered alive while it refreshes its heartbeat key before the
// TTL expires. When the key expires (process died or hung), the node stays in
// the registry set until the reaper detects the missing heartbeat, reclaims its
// in-flight tasks, and removes it. This replaces the old fixed 0..N worker hash.
type NodeStore struct {
	client *redis.Client
	ttl    time.Duration
}

// NewNodeStore creates a NodeStore. heartbeatTTL is how long a heartbeat key
// lives before expiring (e.g. 10s for a 3s beat interval).
func NewNodeStore(client *redis.Client, heartbeatTTL time.Duration) *NodeStore {
	return &NodeStore{client: client, ttl: heartbeatTTL}
}

// Register records a new node in the registry and writes its first heartbeat.
// The full record is stored as the heartbeat key's value so ListNodes can
// hydrate it without a second key.
func (s *NodeStore) Register(ctx context.Context, node model.Node) error {
	data, err := json.Marshal(node)
	if err != nil {
		return fmt.Errorf("marshal node: %w", err)
	}
	pipe := s.client.Pipeline()
	pipe.SAdd(ctx, KeyNodes, node.ID)
	pipe.Set(ctx, NodeHeartbeatKey(node.ID), string(data), s.ttl)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("register node %s: %w", node.ID, err)
	}
	return nil
}

// Heartbeat refreshes a node's heartbeat key TTL. If the key was already gone
// (e.g. the node was slow and the reaper cleaned it up), it is recreated with
// the given record so a briefly-stalled node can rejoin.
func (s *NodeStore) Heartbeat(ctx context.Context, node model.Node) error {
	data, err := json.Marshal(node)
	if err != nil {
		return fmt.Errorf("marshal node: %w", err)
	}
	pipe := s.client.Pipeline()
	pipe.SAdd(ctx, KeyNodes, node.ID)
	pipe.Set(ctx, NodeHeartbeatKey(node.ID), string(data), s.ttl)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("heartbeat node %s: %w", node.ID, err)
	}
	return nil
}

// Deregister removes a node cleanly on graceful shutdown: drop it from the
// registry and delete its heartbeat and (now-empty) tasks set.
func (s *NodeStore) Deregister(ctx context.Context, nodeID string) error {
	pipe := s.client.Pipeline()
	pipe.SRem(ctx, KeyNodes, nodeID)
	pipe.Del(ctx, NodeHeartbeatKey(nodeID))
	pipe.Del(ctx, NodeTasksKey(nodeID))
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("deregister node %s: %w", nodeID, err)
	}
	return nil
}

// RegisteredIDs returns every node ID in the registry set (alive or not).
func (s *NodeStore) RegisteredIDs(ctx context.Context) ([]string, error) {
	ids, err := s.client.SMembers(ctx, KeyNodes).Result()
	if err != nil {
		return nil, fmt.Errorf("smembers nodes: %w", err)
	}
	return ids, nil
}

// IsAlive reports whether a node's heartbeat key still exists.
func (s *NodeStore) IsAlive(ctx context.Context, nodeID string) (bool, error) {
	n, err := s.client.Exists(ctx, NodeHeartbeatKey(nodeID)).Result()
	if err != nil {
		return false, fmt.Errorf("exists heartbeat %s: %w", nodeID, err)
	}
	return n > 0, nil
}

// ListNodes returns every registered node with liveness and in-flight task
// count populated. Dead nodes (heartbeat expired but still in the registry)
// are included with Alive=false so the dashboard can show them graying out
// until the reaper cleans them up.
func (s *NodeStore) ListNodes(ctx context.Context) ([]model.Node, error) {
	ids, err := s.RegisteredIDs(ctx)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, nil
	}

	pipe := s.client.Pipeline()
	hbCmds := make(map[string]*redis.StringCmd, len(ids))
	countCmds := make(map[string]*redis.IntCmd, len(ids))
	for _, id := range ids {
		hbCmds[id] = pipe.Get(ctx, NodeHeartbeatKey(id))
		countCmds[id] = pipe.SCard(ctx, NodeTasksKey(id))
	}
	// Pipeline returns redis.Nil if any GET missed; that's expected for dead nodes.
	if _, err := pipe.Exec(ctx); err != nil && err != redis.Nil {
		return nil, fmt.Errorf("list nodes pipeline: %w", err)
	}

	nodes := make([]model.Node, 0, len(ids))
	for _, id := range ids {
		var node model.Node
		data, err := hbCmds[id].Result()
		if err == redis.Nil {
			// Heartbeat expired — node is dead. We still know its ID.
			node = model.Node{ID: id, Alive: false}
		} else if err != nil {
			continue
		} else {
			if uerr := json.Unmarshal([]byte(data), &node); uerr != nil {
				node = model.Node{ID: id}
			}
			node.Alive = true
		}
		node.InFlightTasks, _ = countCmds[id].Result()
		nodes = append(nodes, node)
	}
	return nodes, nil
}

// DeadNodeIDs returns registered node IDs whose heartbeat key has expired.
// These are the nodes the reaper must reclaim work from.
func (s *NodeStore) DeadNodeIDs(ctx context.Context) ([]string, error) {
	ids, err := s.RegisteredIDs(ctx)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, nil
	}
	pipe := s.client.Pipeline()
	cmds := make(map[string]*redis.IntCmd, len(ids))
	for _, id := range ids {
		cmds[id] = pipe.Exists(ctx, NodeHeartbeatKey(id))
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return nil, fmt.Errorf("dead node scan: %w", err)
	}
	var dead []string
	for _, id := range ids {
		if n, _ := cmds[id].Result(); n == 0 {
			dead = append(dead, id)
		}
	}
	return dead, nil
}

// OwnedTaskIDs returns the task IDs a node currently holds in flight.
func (s *NodeStore) OwnedTaskIDs(ctx context.Context, nodeID string) ([]string, error) {
	ids, err := s.client.SMembers(ctx, NodeTasksKey(nodeID)).Result()
	if err != nil {
		return nil, fmt.Errorf("smembers node tasks %s: %w", nodeID, err)
	}
	return ids, nil
}

// Remove drops a node from the registry and deletes its (already-drained) tasks
// set. Called by the reaper after a dead node's tasks have been reclaimed.
func (s *NodeStore) Remove(ctx context.Context, nodeID string) error {
	pipe := s.client.Pipeline()
	pipe.SRem(ctx, KeyNodes, nodeID)
	pipe.Del(ctx, NodeHeartbeatKey(nodeID))
	pipe.Del(ctx, NodeTasksKey(nodeID))
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("remove node %s: %w", nodeID, err)
	}
	return nil
}
