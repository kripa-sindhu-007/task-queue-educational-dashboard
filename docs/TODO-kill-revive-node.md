# TODO — UI-driven Kill / Revive Node (deferred to next session)

> Goal: from the dashboard, let a user "kill" a specific worker node and watch its
> workload shift to the other nodes, then "revive" it and watch it start receiving
> tasks again. Requires backend support (the API server and worker nodes are
> separate processes, so the paused state must live in Redis).

## Design decision (settled)

**Soft kill = pause (revivable), not a real crash.**

- **Killed/paused node** keeps heartbeating (so it stays visible in the registry and
  is revivable) but **stops dequeuing new tasks**. The shared `ready` queue then
  drains through the *other* live nodes — that's the visible redistribution.
  In-flight tasks it already holds are sub-second (sleep handler) and finish + Ack
  normally (graceful drain).
- **Revive** clears the flag → worker resumes dequeuing → receives tasks again.
- This is deliberately distinct from a **real crash** (`docker compose kill worker`),
  which stops the heartbeat → reaper reclaims in-flight work → node removed. Keep
  both; they teach different things.

**Three node states on the dashboard:**
- `alive` — green (working)
- `paused` / killed — amber (operator-killed, revivable)
- `dead` — coral (heartbeat expired / crashed; reaper will reap; no revive button)

Mechanism: a per-node control key in Redis: `taskqueue:node:{id}:paused` (string "1").
Worker polls it (~1s) and stops dequeuing while set. No TTL on the key; it is deleted
on resume, on graceful deregister, and by the reaper when it removes a dead node.

Not doing in v1 (note as future): forcibly releasing in-flight leases on pause for
instant reclaim. Skipped because it risks double-execution and needs per-task
cancellation; graceful drain is fine for the sub-second sleep handler.

## Implementation checklist

### 1. Backend store — paused flag
- [ ] `model.Node`: add `Paused bool `json:"paused"`` (runtime field, not persisted).
- [ ] `store/redis.go`: add `NodePausedKey(id) = "taskqueue:node:" + id + ":paused"`.
- [ ] `store/node.go`:
  - [ ] `SetPaused(ctx, id string, paused bool) error` — SET key "1" (no TTL) when
        paused, DEL when not.
  - [ ] `IsPaused(ctx, id string) (bool, error)` — EXISTS on the paused key.
  - [ ] `ListNodes`: also pipeline `EXISTS` on each node's paused key and set
        `node.Paused` on the result.
  - [ ] `Deregister` and `Remove`: also `DEL` the paused key (add to the pipelines).

### 2. Worker honors paused flag
- [ ] `worker/pool.go`: add `paused atomic.Bool` field + `SetPaused(bool)` method.
      In the `worker` loop, right after the `ctx.Done` check:
      `if p.paused.Load() { time.Sleep(p.pollInterval); continue }` (skip dequeue).
- [ ] `worker/node.go`: add a control-poll goroutine (ticker ~1s) that reads
      `nodeStore.IsPaused(ctx, nodeID)` and calls `pool.SetPaused(...)`. Stop it on
      `ctx.Done`; close its done channel before deregister (same pattern as the
      heartbeat loop). `Node` already has the `NodeStore` via `NodeConfig.Nodes`.

### 3. API — pause/resume endpoints
- [ ] `api/handler.go`: `PauseNode` and `ResumeNode` handlers using
      `h.deps.Nodes.SetPaused`. Return 404 if the id is not in the registry
      (`RegisteredIDs`/`IsAlive`), else 200 with `{ "id": ..., "paused": bool }`.
- [ ] `api/router.go`:
      `POST /api/nodes/{id}/pause` → `h.PauseNode`
      `POST /api/nodes/{id}/resume` → `h.ResumeNode`
      (`h.deps.Nodes` is already wired in `cmd/server/main.go`.)

### 4. Frontend — kill/revive buttons + paused state
- [ ] `lib/types.ts`: add `paused: boolean` to `Node`.
- [ ] `lib/api.ts`: `pauseNode(id)` / `resumeNode(id)` (POST, no body).
- [ ] `components/NodePanel.tsx`:
  - [ ] Card visual for paused: amber (`bg-sunny-soft` + `text-sunny-ink`), a Pause
        icon, and a `warning` badge reading "paused" (distinct from coral "dead").
        Card state precedence: `!alive` → dead (coral, no button); `alive && paused`
        → paused (amber, "Revive" button); `alive && !paused` → active (mint/green,
        "Kill" button).
  - [ ] Per-card clay button (`clay-btn`, small): "Kill" (Pause/Power icon, coral)
        when active → calls `pauseNode`; "Revive" (Play icon, mint) when paused →
        calls `resumeNode`. Optimistic UI is optional (polling refreshes at 1s).
  - [ ] `cursor-pointer`, `aria-label`, visible focus (clay-btn already has focus
        ring). Respect the a11y checklist.

### 5. Tests + verify
- [ ] `store/node_test.go`: SetPaused/IsPaused round-trip; ListNodes reflects
      paused=true after SetPaused, false after clear; Deregister/Remove delete the
      paused key.
- [ ] (optional) `api/handler_test.go`: pause/resume returns 200 for a registered
      node, 404 for unknown.
- [ ] Backend: `go vet ./...`, `go build ./...`, `go test ./... -count=1`.
- [ ] Frontend: `npx tsc --noEmit`, `npm run build`.

## Demo script (once built)
1. `docker compose up --build --scale worker=4`
2. Submit a batch (e.g. 5k) so the queue is busy.
3. Click **Kill** on one node → its busy dots drop to 0 within ~1s; the other three
   keep draining the queue (workload redistributed).
4. Click **Revive** → it resumes and its dots light up again as it receives tasks.

## Notes / edge cases
- Paused node still heartbeats → reaper never reclaims it (correct; it's not dead).
- If a paused node is then really killed (`docker compose kill`), its heartbeat
  expires → reaper reclaims in-flight + `Remove` deletes the paused key too.
- Control-poll adds one cheap Redis EXISTS/GET per node per ~1s — negligible.
