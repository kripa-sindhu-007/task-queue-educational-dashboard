-- release.lua
-- Owner-only lease release (CAS). Deletes the leader lease IFF this node still
-- holds it, so a graceful step-down never deletes a lease that another node has
-- since acquired. Frees the crown immediately on clean shutdown so a restart
-- fails over in milliseconds rather than after a full TTL.
--
-- KEYS[1] = leader key (taskqueue:leader)
--
-- ARGV[1] = this node's ID (expected lease holder)
--
-- Returns: 1 if the lease was released (this node owned it), 0 otherwise.

if redis.call('GET', KEYS[1]) ~= ARGV[1] then
    return 0
end

redis.call('DEL', KEYS[1])
return 1
