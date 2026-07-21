-- nack.lua
-- Atomically removes a task from the processing ZSET. The caller (executor)
-- handles retry routing (delayed set or DLQ) after this script confirms the
-- lease was released. This separation keeps the broker focused on delivery
-- semantics and the executor on retry policy.
--
-- KEYS[1] = processing set (taskqueue:processing)
--
-- ARGV[1] = task ID
--
-- Returns: 1 if the task was in processing and removed, 0 if not found
--          (already reclaimed by the reaper — the task may be re-delivered).

local removed = redis.call('ZREM', KEYS[1], ARGV[1])
return removed
