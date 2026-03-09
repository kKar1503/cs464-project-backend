#!/bin/bash

# Simple Redis connection test script

echo "=== Redis Connection Test ==="
echo ""

# Get password from .env
REDIS_PASS=$(grep REDIS_PASSWORD .env | cut -d'=' -f2)

echo "Testing Redis with password..."
echo ""

# Test 1: Ping
echo "1. PING test:"
docker exec game-redis redis-cli -a "$REDIS_PASS" ping

# Test 2: Set and Get
echo ""
echo "2. SET/GET test:"
docker exec game-redis redis-cli -a "$REDIS_PASS" SET test:hello "World"
docker exec game-redis redis-cli -a "$REDIS_PASS" GET test:hello

# Test 3: Info
echo ""
echo "3. Server info:"
docker exec game-redis redis-cli -a "$REDIS_PASS" INFO server | head -n 15

echo ""
echo "=== Test complete! ==="
