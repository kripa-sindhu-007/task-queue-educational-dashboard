package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/model"
	"github.com/kripa-sindhu-007/task-queue-educational-dashboard/backend/internal/store"
)

func nodeSetup(t *testing.T) (*miniredis.Miniredis, *redis.Client, *store.NodeStore) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { client.Close() })
	return mr, client, store.NewNodeStore(client, 10*time.Second)
}

func TestNodeStore_RegisterAndList(t *testing.T) {
	_, _, ns := nodeSetup(t)
	ctx := context.Background()

	node := model.Node{ID: "node-a", Hostname: "host-a", Capacity: 5, StartedAt: time.Now()}
	if err := ns.Register(ctx, node); err != nil {
		t.Fatalf("register: %v", err)
	}

	nodes, err := ns.ListNodes(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	if nodes[0].ID != "node-a" || nodes[0].Hostname != "host-a" || nodes[0].Capacity != 5 {
		t.Fatalf("node record not hydrated: %+v", nodes[0])
	}
	if !nodes[0].Alive {
		t.Fatal("expected freshly registered node to be alive")
	}
}

func TestNodeStore_HeartbeatExpiryMakesNodeDead(t *testing.T) {
	mr, _, ns := nodeSetup(t)
	ctx := context.Background()

	node := model.Node{ID: "node-b", Hostname: "host-b", Capacity: 3, StartedAt: time.Now()}
	if err := ns.Register(ctx, node); err != nil {
		t.Fatalf("register: %v", err)
	}

	// Advance miniredis clock past the 10s TTL so the heartbeat key expires.
	mr.FastForward(11 * time.Second)

	alive, err := ns.IsAlive(ctx, "node-b")
	if err != nil {
		t.Fatalf("isalive: %v", err)
	}
	if alive {
		t.Fatal("expected node to be dead after TTL expiry")
	}

	dead, err := ns.DeadNodeIDs(ctx)
	if err != nil {
		t.Fatalf("deadnodeids: %v", err)
	}
	if len(dead) != 1 || dead[0] != "node-b" {
		t.Fatalf("expected [node-b] dead, got %v", dead)
	}

	// ListNodes should still show it (in registry) but Alive=false.
	nodes, _ := ns.ListNodes(ctx)
	if len(nodes) != 1 || nodes[0].Alive {
		t.Fatalf("expected 1 dead node in list, got %+v", nodes)
	}
}

func TestNodeStore_HeartbeatKeepsAlive(t *testing.T) {
	mr, _, ns := nodeSetup(t)
	ctx := context.Background()

	node := model.Node{ID: "node-c", Hostname: "host-c", Capacity: 1, StartedAt: time.Now()}
	ns.Register(ctx, node)

	// Advance almost to expiry, then heartbeat to refresh the TTL.
	mr.FastForward(8 * time.Second)
	if err := ns.Heartbeat(ctx, node); err != nil {
		t.Fatalf("heartbeat: %v", err)
	}
	mr.FastForward(8 * time.Second) // 16s total, but refreshed at 8s

	alive, _ := ns.IsAlive(ctx, "node-c")
	if !alive {
		t.Fatal("expected node kept alive by heartbeat refresh")
	}
}

func TestNodeStore_Deregister(t *testing.T) {
	_, client, ns := nodeSetup(t)
	ctx := context.Background()

	node := model.Node{ID: "node-d", Hostname: "host-d", Capacity: 2, StartedAt: time.Now()}
	ns.Register(ctx, node)
	// Simulate an in-flight task owned by the node.
	client.SAdd(ctx, store.NodeTasksKey("node-d"), "task-1")

	if err := ns.Deregister(ctx, "node-d"); err != nil {
		t.Fatalf("deregister: %v", err)
	}

	ids, _ := ns.RegisteredIDs(ctx)
	if len(ids) != 0 {
		t.Fatalf("expected registry empty after deregister, got %v", ids)
	}
	if n, _ := client.Exists(ctx, store.NodeTasksKey("node-d")).Result(); n != 0 {
		t.Fatal("expected node tasks set deleted on deregister")
	}
}

func TestNodeStore_OwnedTaskIDsAndInFlightCount(t *testing.T) {
	_, client, ns := nodeSetup(t)
	ctx := context.Background()

	node := model.Node{ID: "node-e", Hostname: "host-e", Capacity: 4, StartedAt: time.Now()}
	ns.Register(ctx, node)
	client.SAdd(ctx, store.NodeTasksKey("node-e"), "t1", "t2", "t3")

	owned, err := ns.OwnedTaskIDs(ctx, "node-e")
	if err != nil {
		t.Fatalf("owned: %v", err)
	}
	if len(owned) != 3 {
		t.Fatalf("expected 3 owned tasks, got %d", len(owned))
	}

	nodes, _ := ns.ListNodes(ctx)
	if len(nodes) != 1 || nodes[0].InFlightTasks != 3 {
		t.Fatalf("expected in-flight count 3, got %+v", nodes)
	}
}
