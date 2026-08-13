-- signal.lua
-- Cap-guarded doorbell push (P3.4). Rings the ready doorbell by RPUSHing a single
-- fungible wake-up token onto the signal list, but only while the list is below
-- its cap. The token carries no task identity — it just wakes one blocked worker,
-- which then runs the unchanged atomic claim (dequeue.lua). Bounding the list at
-- the cap keeps it from growing to queue depth when all workers are busy; any
-- token beyond the number of blocked workers is redundant because the fallback
-- poll re-claims anyway.
--
-- KEYS[1] = signal list (taskqueue:ready:signal)
-- ARGV[1] = cap (max tokens retained)
--
-- Returns: 1 if a token was pushed, 0 if the list was already at the cap.

local sig = KEYS[1]
local cap = tonumber(ARGV[1])

if redis.call('LLEN', sig) < cap then
    redis.call('RPUSH', sig, '1')
    return 1
end
return 0
