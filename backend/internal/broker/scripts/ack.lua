-- ack.lua
-- Atomically removes a task from the processing ZSET and marks it completed
-- in the canonical task record. The ZREM return value is the concurrency guard:
-- only the caller that successfully removes the member "owns" the completion.
--
-- KEYS[1] = processing set (taskqueue:processing)
-- KEYS[2] = task record key (taskqueue:task:{id})
--
-- ARGV[1] = task ID (for the ZREM member)
--
-- Returns: 1 if acked successfully, 0 if the task was not in processing
--          (already reclaimed by the reaper or never leased).

local removed = redis.call('ZREM', KEYS[1], ARGV[1])
if removed == 0 then
    return 0
end

-- Task was in processing and we own it — mark completed.
redis.call('HSET', KEYS[2], 'status', 'completed', 'error', '')

return 1
