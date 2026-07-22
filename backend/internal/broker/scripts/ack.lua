-- ack.lua
-- Atomically completes a task IFF this node still owns the lease. Ownership is
-- verified before mutating anything so a worker whose lease already expired
-- (reclaimed by the reaper and possibly re-leased to another node) cannot clobber
-- the new owner's lease. This is lightweight fencing by owner equality; true
-- monotonic fencing tokens are a later concern (P4).
--
-- KEYS[1] = processing set (taskqueue:processing)
-- KEYS[2] = task record key (taskqueue:task:{id})
-- KEYS[3] = this node's task SET (taskqueue:node:{id}:tasks)
--
-- ARGV[1] = task ID
-- ARGV[2] = expected owner (this node's ID)
--
-- Returns: 1 if acked, 0 if the lease is no longer held by this node
--          (expired/reclaimed/re-leased) — treated as ErrLeaseNotHeld by caller.

-- Not in processing at all → nothing to ack.
if redis.call('ZSCORE', KEYS[1], ARGV[1]) == false then
    return 0
end

-- Owned by someone else now (reclaimed then re-leased) → do not touch it.
if redis.call('HGET', KEYS[2], 'owner') ~= ARGV[2] then
    return 0
end

redis.call('ZREM', KEYS[1], ARGV[1])
redis.call('SREM', KEYS[3], ARGV[1])
redis.call('HSET', KEYS[2], 'status', 'completed', 'error', '', 'owner', '')

return 1
