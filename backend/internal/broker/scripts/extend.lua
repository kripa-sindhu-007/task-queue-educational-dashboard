-- extend.lua
-- Atomically extends the lease deadline for a task already in the processing
-- set. Existence check and score update happen in one script, so there is no
-- TOCTOU window between "is it still leased?" and "push back its deadline".
--
-- KEYS[1] = processing set (taskqueue:processing)
--
-- ARGV[1] = task ID
-- ARGV[2] = new lease deadline in milliseconds (unix timestamp ms)
--
-- Returns: 1 if the lease was extended, 0 if the task is not in processing
--          (already acked, reclaimed, or never leased).

if redis.call('ZSCORE', KEYS[1], ARGV[1]) == false then
    return 0
end

redis.call('ZADD', KEYS[1], ARGV[2], ARGV[1])
return 1
