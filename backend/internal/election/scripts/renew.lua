-- renew.lua
-- Owner-only lease renewal (CAS). Extends the leader lease IFF this node still
-- holds it, so a node that already lost the lease (expired, or acquired by
-- another) can never resurrect or steal it. This is the "prefer zero leaders to
-- two" guard: a failed CAS makes the caller step down rather than assume it is
-- still leader.
--
-- KEYS[1] = leader key (taskqueue:leader)
--
-- ARGV[1] = this node's ID (expected lease holder)
-- ARGV[2] = new TTL in milliseconds
--
-- Returns: 1 if the lease was renewed (this node owns it), 0 otherwise.

if redis.call('GET', KEYS[1]) ~= ARGV[1] then
    return 0
end

redis.call('PEXPIRE', KEYS[1], ARGV[2])
return 1
