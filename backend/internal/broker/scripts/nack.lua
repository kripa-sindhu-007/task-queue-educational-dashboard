-- nack.lua
-- Atomically releases a task's lease IFF this node still owns it. Like ack.lua,
-- ownership is verified first so a worker whose lease already expired cannot
-- release a lease that now belongs to another node. The caller (executor)
-- handles retry/DLQ routing after this confirms the release.
--
-- KEYS[1] = processing set (taskqueue:processing)
-- KEYS[2] = task record key (taskqueue:task:{id})
-- KEYS[3] = this node's task SET (taskqueue:node:{id}:tasks)
--
-- ARGV[1] = task ID
-- ARGV[2] = expected owner (this node's ID)
--
-- Returns: 1 if released, 0 if the lease is no longer held by this node.

if redis.call('ZSCORE', KEYS[1], ARGV[1]) == false then
    return 0
end

if redis.call('HGET', KEYS[2], 'owner') ~= ARGV[2] then
    return 0
end

redis.call('ZREM', KEYS[1], ARGV[1])
redis.call('SREM', KEYS[3], ARGV[1])
return 1
