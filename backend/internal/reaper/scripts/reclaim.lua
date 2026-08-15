-- reclaim.lua
-- Atomically reclaims a single task from the processing set. Used for both
-- lease-expiry reclaim and eager dead-node reclaim. ZREM is the concurrency
-- guard: only the caller that removes the member owns the reclaim, which safely
-- handles the reaper-vs-worker and reaper-vs-reaper races.
--
-- The script reads retries/max_retries/priority from the task hash itself
-- (rather than trusting caller-passed values) so its decision is always
-- self-consistent with the authoritative record.
--
-- KEYS[1] = processing set (taskqueue:processing)
-- KEYS[2] = ready set (taskqueue:ready)
-- KEYS[3] = task record key (taskqueue:task:{id})
-- KEYS[4] = owning node's task SET (taskqueue:node:{owner}:tasks)
-- KEYS[5] = ready doorbell signal list (taskqueue:ready:signal)
--
-- ARGV[1] = task ID
-- ARGV[2] = doorbell cap (max tokens retained in the signal list)
--
-- Returns:
--   "reclaimed" — task moved back to ready with retries incremented
--   "dead_lettered" — retries exhausted, task marked failed (caller pushes to DLQ list)
--   "orphan" — task had no canonical record; removed from processing, nothing else to do
--   "lost_race" — another reaper or the worker already handled it

-- Try to claim the task by removing from processing.
local removed = redis.call('ZREM', KEYS[1], ARGV[1])
if removed == 0 then
    return "lost_race"
end

-- The task is leaving processing regardless of outcome, so it leaves the owning
-- node's in-flight set too.
redis.call('SREM', KEYS[4], ARGV[1])

-- The record may have been flushed after we claimed it.
if redis.call('EXISTS', KEYS[3]) == 0 then
    return "orphan"
end

local maxRetries = tonumber(redis.call('HGET', KEYS[3], 'max_retries')) or 0
local retries = tonumber(redis.call('HGET', KEYS[3], 'retries')) or 0
local priority = tonumber(redis.call('HGET', KEYS[3], 'priority')) or 0

if retries >= maxRetries then
    -- Exhausted retries — mark as failed and clear ownership.
    redis.call('HSET', KEYS[3], 'status', 'failed', 'owner', '')
    return "dead_lettered"
end

-- Increment retries, reset status to pending, clear ownership, put back in ready.
redis.call('HSET', KEYS[3], 'retries', tostring(retries + 1), 'status', 'pending', 'owner', '')
redis.call('ZADD', KEYS[2], -priority, ARGV[1])

-- Ring the doorbell so a blocked worker re-claims the reclaimed task without
-- waiting for the fallback poll. Only the "reclaimed" branch makes a task ready,
-- so only it pushes a token; dead_lettered/orphan/lost_race push nothing. Atomic
-- with the ZADD ready above, and cap-guarded like every other producer.
local signal_cap = tonumber(ARGV[2])
if redis.call('LLEN', KEYS[5]) < signal_cap then
    redis.call('RPUSH', KEYS[5], '1')
end

return "reclaimed"
