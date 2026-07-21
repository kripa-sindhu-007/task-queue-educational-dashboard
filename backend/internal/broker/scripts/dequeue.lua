-- dequeue.lua
-- Atomically pops the highest-priority task from the ready ZSET and inserts
-- it into the processing ZSET with a lease deadline score.
--
-- KEYS[1] = ready set (taskqueue:ready)
-- KEYS[2] = processing set (taskqueue:processing)
-- KEYS[3] = task record key prefix (taskqueue:task:) — we append the popped ID
--
-- ARGV[1] = lease deadline in milliseconds (unix timestamp ms)
--
-- Returns: task ID (string) if a task was claimed, or nil if ready is empty.

-- Pop the member with the lowest score (highest priority, since score = -priority)
local result = redis.call('ZPOPMIN', KEYS[1], 1)
if #result == 0 then
    return nil
end

local taskID = result[1]
-- result[2] is the score (we don't need it after popping)

-- Insert into processing ZSET with lease deadline as score
redis.call('ZADD', KEYS[2], ARGV[1], taskID)

-- Update the canonical task record's status field
local taskKey = KEYS[3] .. taskID
redis.call('HSET', taskKey, 'status', 'processing')

return taskID
