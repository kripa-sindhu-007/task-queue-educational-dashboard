-- dequeue.lua
-- Atomically pops the highest-priority task from the ready ZSET, leases it into
-- the processing ZSET, and records which node now owns it (P2.2). Ownership is
-- tracked two ways so the reaper can reclaim a dead node's work efficiently:
--   * owner field on the task hash (authoritative)
--   * membership in the owning node's task SET (fast per-node reclaim)
--
-- KEYS[1] = ready set (taskqueue:ready)
-- KEYS[2] = processing set (taskqueue:processing)
-- KEYS[3] = task record key prefix (taskqueue:task:) — append the popped ID
-- KEYS[4] = owning node's task SET (taskqueue:node:{id}:tasks)
--
-- ARGV[1] = lease deadline in milliseconds (unix timestamp ms)
-- ARGV[2] = owner node ID
--
-- Returns: task ID (string) if a task was claimed, or nil if ready is empty.

-- Pop the member with the lowest score (highest priority, since score = -priority)
local result = redis.call('ZPOPMIN', KEYS[1], 1)
if #result == 0 then
    return nil
end

local taskID = result[1]

-- Lease into processing with the deadline as score.
redis.call('ZADD', KEYS[2], ARGV[1], taskID)

-- Record ownership on the canonical record and in the node's task set.
local taskKey = KEYS[3] .. taskID
redis.call('HSET', taskKey, 'status', 'processing', 'owner', ARGV[2])
redis.call('SADD', KEYS[4], taskID)

return taskID
