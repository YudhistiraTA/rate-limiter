local key = KEYS[1] -- token bucket key
local capacity = tonumber(ARGV[1]) -- bucket capacity
local rate = tonumber(ARGV[2]) -- refill rate (tokens per second)
local ttl = tonumber(ARGV[3]) -- time to live in seconds
local now = math.floor(tonumber(redis.call("TIME")[1])) -- current time in seconds

local tokens = capacity -- default to full capacity
local tokens_str = redis.call("HGET", key, "tokens") -- get current tokens
if tokens_str then
    tokens = tonumber(tokens_str) -- if exists, parse it
end

local last_time = now -- default to now
local last_time_str = redis.call("HGET", key, "last_time") -- get last refill time
if last_time_str then
    last_time = tonumber(last_time_str) -- if exists, parse it
end

-- Refill tokens based on elapsed time
local delta = math.max(0, now - last_time)
local added = delta * rate
tokens = math.min(tokens + added, capacity)

-- Attempt to consume a token
local allowed = 0
if tokens >= 1 then
    tokens = tokens - 1
    allowed = 1
end

-- Update the token bucket state
redis.call("HSET", key, "tokens", tokens)
redis.call("HSET", key, "last_time", now)
redis.call("EXPIRE", key, ttl)

return allowed
