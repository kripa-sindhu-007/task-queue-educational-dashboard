-- reclaim.lua
-- Atomically reclaims a single expired task from the processing set.
-- Uses ZREM as the concurrency guard: only the caller that removes the member
-- owns the reclaim. This handles the reaper-vs-worker race safely.
--
-- The script reads retries/max_retries/priority from the task hash itself
-- (rather than trusting caller-passed values) so its decision is always
-- self-consistent with the authoritative record, even if the caller's snapshot
-- was stale.
--
-- KEYS[1] = processing set (taskqueue:processing)
-- KEYS[2] = ready set (taskqueue:ready)
-- KEYS[3] = task record key (taskqueue:task:{id})
--
-- ARGV[1] = task ID
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

-- The record may have been flushed after we claimed it. We've already removed
-- it from processing above, so there is nothing left to reclaim.
if redis.call('EXISTS', KEYS[3]) == 0 then
    return "orphan"
end

local maxRetries = tonumber(redis.call('HGET', KEYS[3], 'max_retries')) or 0
local retries = tonumber(redis.call('HGET', KEYS[3], 'retries')) or 0
local priority = tonumber(redis.call('HGET', KEYS[3], 'priority')) or 0

if retries >= maxRetries then
    -- Exhausted retries — mark as failed in the record.
    redis.call('HSET', KEYS[3], 'status', 'failed')
    return "dead_lettered"
end

-- Increment retries, reset status to pending, put back in ready.
redis.call('HSET', KEYS[3], 'retries', tostring(retries + 1), 'status', 'pending')
redis.call('ZADD', KEYS[2], -priority, ARGV[1])

return "reclaimed"
