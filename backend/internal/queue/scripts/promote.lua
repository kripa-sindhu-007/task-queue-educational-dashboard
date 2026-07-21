-- promote.lua
-- Atomically promotes due tasks from the delayed ZSET to the ready ZSET.
-- Fetches up to ARGV[2] tasks whose score <= ARGV[1] (now), removes each from
-- delayed, and inserts into ready with score = -priority (read from the task hash).
--
-- KEYS[1] = delayed set (taskqueue:delayed)
-- KEYS[2] = ready set (taskqueue:ready)
-- KEYS[3] = task record key prefix (taskqueue:task:)
--
-- ARGV[1] = current unix timestamp (seconds, as string)
-- ARGV[2] = batch size limit
--
-- Returns: number of tasks promoted

local delayed_key = KEYS[1]
local ready_key = KEYS[2]
local prefix = KEYS[3]
local now = ARGV[1]
local limit = tonumber(ARGV[2])

-- Fetch due task IDs
local ids = redis.call('ZRANGEBYSCORE', delayed_key, '-inf', now, 'LIMIT', 0, limit)
if #ids == 0 then
    return 0
end

local promoted = 0
for _, id in ipairs(ids) do
    -- ZREM as concurrency guard: only one caller promotes each task
    local removed = redis.call('ZREM', delayed_key, id)
    if removed == 1 then
        -- Read priority from the task hash
        local priority = tonumber(redis.call('HGET', prefix .. id, 'priority')) or 0
        -- Insert into ready with score = -priority (higher priority = lower score = pops first)
        redis.call('ZADD', ready_key, -priority, id)
        promoted = promoted + 1
    end
end

return promoted
