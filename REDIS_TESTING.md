# Redis CLI Testing Guide

Complete guide for testing Redis with password authentication.

## Quick Start

### Get Your Password

```bash
cat .env | grep REDIS_PASSWORD
# REDIS_PASSWORD=vMIpkx6YQtbzXj8IUFlwZ/DIZuov6J4g38EXkEUwgPE=
```

### Test Connection

```bash
# Simple ping test
docker exec -it game-redis redis-cli -a "vMIpkx6YQtbzXj8IUFlwZ/DIZuov6J4g38EXkEUwgPE=" ping
# Output: PONG

# Using environment variable (easier)
docker exec -it game-redis redis-cli -a "$(grep REDIS_PASSWORD .env | cut -d'=' -f2)" ping
```

## Connection Methods

### Method 1: One-line Commands

```bash
# Set password as environment variable for easier use
export REDIS_PASS=$(grep REDIS_PASSWORD .env | cut -d'=' -f2)

# Now use it in commands
docker exec -it game-redis redis-cli -a "$REDIS_PASS" ping
docker exec -it game-redis redis-cli -a "$REDIS_PASS" SET mykey "value"
docker exec -it game-redis redis-cli -a "$REDIS_PASS" GET mykey
```

### Method 2: Interactive CLI

```bash
# Enter Redis CLI
docker exec -it game-redis redis-cli

# Authenticate first
127.0.0.1:6379> AUTH vMIpkx6YQtbzXj8IUFlwZ/DIZuov6J4g38EXkEUwgPE=
OK

# Now run commands
127.0.0.1:6379> PING
PONG

# Exit when done
127.0.0.1:6379> EXIT
```

### Method 3: From Host (if redis-cli installed)

```bash
# Install redis-cli on host
brew install redis  # macOS
apt-get install redis-tools  # Ubuntu

# Connect to Docker Redis
redis-cli -h 127.0.0.1 -p 6379 -a "vMIpkx6YQtbzXj8IUFlwZ/DIZuov6J4g38EXkEUwgPE="
```

## Basic Commands

### Strings (Simple Key-Value)

```bash
# Set a value
SET mykey "Hello World"

# Get a value
GET mykey

# Set with expiration (seconds)
SET session:123 "user_data" EX 3600

# Check time to live
TTL session:123

# Set if not exists
SETNX lock:game:1 "locked"

# Increment counter
INCR visitor:count

# Multiple set/get
MSET key1 "val1" key2 "val2" key3 "val3"
MGET key1 key2 key3
```

### Lists (Ordered Collections)

```bash
# Push to list (left/right)
LPUSH queue:matchmaking user:1
RPUSH queue:matchmaking user:2

# Pop from list
LPOP queue:matchmaking
RPOP queue:matchmaking

# Get range
LRANGE queue:matchmaking 0 -1

# Get length
LLEN queue:matchmaking

# Blocking pop (wait for item)
BLPOP queue:matchmaking 5
```

### Sets (Unique Collections)

```bash
# Add to set
SADD online:users user:1 user:2 user:3

# Check membership
SISMEMBER online:users user:1

# Get all members
SMEMBERS online:users

# Remove from set
SREM online:users user:1

# Set operations
SINTER online:users premium:users  # Intersection
SUNION online:users offline:users  # Union
SDIFF all:users banned:users       # Difference
```

### Sorted Sets (Scored Collections)

```bash
# Add with score (MMR rating)
ZADD matchmaking:queue 1200 user:1
ZADD matchmaking:queue 1150 user:2
ZADD matchmaking:queue 1180 user:3

# Get by rank
ZRANGE matchmaking:queue 0 -1
ZRANGE matchmaking:queue 0 -1 WITHSCORES

# Get by score range (MMR between 1100-1300)
ZRANGEBYSCORE matchmaking:queue 1100 1300

# Get rank
ZRANK matchmaking:queue user:1

# Get score
ZSCORE matchmaking:queue user:1

# Remove member
ZREM matchmaking:queue user:1

# Increment score
ZINCRBY matchmaking:queue 50 user:1
```

### Hashes (Object Storage)

```bash
# Set hash fields
HSET user:1 name "John" mmr 1200 level 10

# Get hash field
HGET user:1 name

# Get all hash
HGETALL user:1

# Set multiple fields
HMSET user:2 name "Jane" mmr 1300 level 15

# Increment field
HINCRBY user:1 mmr 50
```

## Game-Specific Examples

### User Sessions

```bash
# Store session (expires in 1 hour)
SET session:abc123 '{"user_id":"1","username":"player1"}' EX 3600

# Check session exists
EXISTS session:abc123

# Get session
GET session:abc123

# Delete session (logout)
DEL session:abc123

# Extend session TTL
EXPIRE session:abc123 3600
```

### Matchmaking Queue

```bash
# Add players to queue with MMR
ZADD matchmaking:queue 1200 user:1
ZADD matchmaking:queue 1180 user:2
ZADD matchmaking:queue 1220 user:3

# Find players in MMR range (±50)
ZRANGEBYSCORE matchmaking:queue 1150 1250

# Remove from queue
ZREM matchmaking:queue user:1

# Get queue size
ZCARD matchmaking:queue

# Get top players
ZREVRANGE matchmaking:queue 0 9  # Top 10
```

### Online Users

```bash
# Add user online
SADD online:users user:1

# Remove user (disconnect)
SREM online:users user:1

# Check if online
SISMEMBER online:users user:1

# Get all online users
SMEMBERS online:users

# Count online users
SCARD online:users
```

### Friend List Cache

```bash
# Store friend list
SADD friends:user:1 user:2 user:3 user:4

# Get friends
SMEMBERS friends:user:1

# Add friend
SADD friends:user:1 user:5

# Remove friend
SREM friends:user:1 user:2

# Check if friends
SISMEMBER friends:user:1 user:3

# Get mutual friends
SINTER friends:user:1 friends:user:2
```

### Game State

```bash
# Store game state as JSON
SET game:session:xyz '{"players":["user:1","user:2"],"status":"in_progress"}' EX 7200

# Store individual fields
HSET game:session:xyz player1 "user:1" player2 "user:2" status "in_progress" created_at 1234567890

# Get game state
HGETALL game:session:xyz

# Update status
HSET game:session:xyz status "completed"
```

### Leaderboard

```bash
# Add scores
ZADD leaderboard 1500 player1
ZADD leaderboard 1450 player2
ZADD leaderboard 1600 player3

# Get top 10
ZREVRANGE leaderboard 0 9 WITHSCORES

# Get player rank (0-based)
ZREVRANK leaderboard player1

# Get players around a specific player
ZREVRANK leaderboard player1  # Get rank
ZREVRANGE leaderboard rank-5 rank+5  # Get context
```

### Pub/Sub (Real-time Events)

```bash
# Subscribe to channel (Terminal 1)
SUBSCRIBE game:events
SUBSCRIBE matchmaking:found
SUBSCRIBE game:session:xyz

# Publish event (Terminal 2)
PUBLISH game:events "player_joined"
PUBLISH matchmaking:found '{"match_id":"123","players":["user:1","user:2"]}'

# Pattern subscribe (subscribes to all game channels)
PSUBSCRIBE game:*

# Unsubscribe
UNSUBSCRIBE game:events
```

## Monitoring & Debugging

### Server Info

```bash
# General info
INFO

# Memory info
INFO memory

# Stats
INFO stats

# Replication info
INFO replication
```

### Performance

```bash
# Monitor all commands (real-time)
MONITOR

# Slow log
SLOWLOG GET 10

# Client list
CLIENT LIST

# Memory usage of key
MEMORY USAGE mykey
```

### Database Management

```bash
# List all keys (use with caution in production)
KEYS *

# Count all keys
DBSIZE

# Find keys by pattern
KEYS user:*
KEYS session:*

# Scan keys (safer than KEYS)
SCAN 0 MATCH user:* COUNT 100

# Delete all keys (DANGEROUS!)
FLUSHDB

# Delete all databases (VERY DANGEROUS!)
FLUSHALL

# Select database (default is 0)
SELECT 0
```

## Testing Script

Save this as `test-redis.sh`:

```bash
#!/bin/bash

REDIS_PASS=$(grep REDIS_PASSWORD .env | cut -d'=' -f2)

echo "=== Testing Redis Connection ==="

# Test 1: Ping
echo "Test 1: Ping"
docker exec -it game-redis redis-cli -a "$REDIS_PASS" ping

# Test 2: Set and Get
echo -e "\nTest 2: Set and Get"
docker exec -it game-redis redis-cli -a "$REDIS_PASS" SET test:key "Hello Redis"
docker exec -it game-redis redis-cli -a "$REDIS_PASS" GET test:key

# Test 3: Expiring Key
echo -e "\nTest 3: Expiring Key"
docker exec -it game-redis redis-cli -a "$REDIS_PASS" SET test:expiring "expires soon" EX 10
docker exec -it game-redis redis-cli -a "$REDIS_PASS" TTL test:expiring

# Test 4: List Operations
echo -e "\nTest 4: List Operations"
docker exec -it game-redis redis-cli -a "$REDIS_PASS" RPUSH test:list item1 item2 item3
docker exec -it game-redis redis-cli -a "$REDIS_PASS" LRANGE test:list 0 -1

# Test 5: Set Operations
echo -e "\nTest 5: Set Operations"
docker exec -it game-redis redis-cli -a "$REDIS_PASS" SADD test:set member1 member2 member3
docker exec -it game-redis redis-cli -a "$REDIS_PASS" SMEMBERS test:set

# Test 6: Sorted Set (Matchmaking simulation)
echo -e "\nTest 6: Sorted Set (Matchmaking)"
docker exec -it game-redis redis-cli -a "$REDIS_PASS" ZADD test:mmr 1200 player1 1150 player2 1180 player3
docker exec -it game-redis redis-cli -a "$REDIS_PASS" ZRANGE test:mmr 0 -1 WITHSCORES

# Test 7: Hash (User profile)
echo -e "\nTest 7: Hash (User Profile)"
docker exec -it game-redis redis-cli -a "$REDIS_PASS" HSET test:user name "TestUser" level 10 score 1200
docker exec -it game-redis redis-cli -a "$REDIS_PASS" HGETALL test:user

# Cleanup
echo -e "\nCleaning up test keys..."
docker exec -it game-redis redis-cli -a "$REDIS_PASS" DEL test:key test:expiring test:list test:set test:mmr test:user

echo -e "\n=== All tests completed! ==="
```

Make it executable:
```bash
chmod +x test-redis.sh
./test-redis.sh
```

## Helper Aliases

Add these to your `~/.zshrc` or `~/.bashrc`:

```bash
# Redis aliases
alias redis-cli-docker='docker exec -it game-redis redis-cli -a "$(grep REDIS_PASSWORD .env | cut -d=\"'\"'\"' -f2)"'
alias redis-ping='docker exec -it game-redis redis-cli -a "$(grep REDIS_PASSWORD .env | cut -d=\"'\"'\"' -f2)" ping'
alias redis-info='docker exec -it game-redis redis-cli -a "$(grep REDIS_PASSWORD .env | cut -d=\"'\"'\"' -f2)" INFO'
alias redis-keys='docker exec -it game-redis redis-cli -a "$(grep REDIS_PASSWORD .env | cut -d=\"'\"'\"' -f2)" KEYS "*"'
```

Then use:
```bash
redis-cli-docker  # Enter interactive mode
redis-ping        # Quick ping test
redis-info        # Server info
redis-keys        # List all keys
```

## Common Patterns

### Cache Pattern

```bash
# Check cache
GET cache:user:1

# If not exists, set from database
SET cache:user:1 '{"id":1,"name":"John"}' EX 300
```

### Lock Pattern

```bash
# Acquire lock
SET lock:resource:1 "token123" NX EX 10

# Release lock (only if you own it)
# Use Lua script in production for atomic check-and-delete
```

### Rate Limiting

```bash
# Simple counter
INCR ratelimit:user:1:minute
EXPIRE ratelimit:user:1:minute 60

# Check limit
GET ratelimit:user:1:minute
```

## Security Notes

- ⚠️ **Never use `KEYS *` in production** - Use `SCAN` instead
- ⚠️ **Never use `FLUSHALL`/`FLUSHDB` in production**
- ✅ Always use password authentication
- ✅ Use TLS in production (AWS ElastiCache)
- ✅ Limit `MONITOR` usage (performance impact)

## Troubleshooting

### Authentication Failed

```bash
# Check password
cat .env | grep REDIS_PASSWORD

# Test without password (should fail)
docker exec -it game-redis redis-cli ping
# Error: NOAUTH Authentication required

# Test with password
docker exec -it game-redis redis-cli -a "your-password" ping
# PONG
```

### Connection Refused

```bash
# Check Redis is running
docker ps | grep redis

# Check logs
docker logs game-redis
```

### Out of Memory

```bash
# Check memory usage
docker exec -it game-redis redis-cli -a "$REDIS_PASS" INFO memory

# Check eviction policy
docker exec -it game-redis redis-cli -a "$REDIS_PASS" CONFIG GET maxmemory-policy
```

## Resources

- Redis Commands: https://redis.io/commands
- Redis Data Types: https://redis.io/docs/data-types/
- Best Practices: https://redis.io/docs/manual/patterns/
