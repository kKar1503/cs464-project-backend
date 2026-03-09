#!/bin/bash

# Generate secure random passwords for game backend

echo "=== Game Backend Password Generator ==="
echo ""

echo "MySQL Root Password:"
MYSQL_ROOT=$(openssl rand -base64 24)
echo "$MYSQL_ROOT"
echo ""

echo "MySQL User Password:"
MYSQL_USER=$(openssl rand -base64 24)
echo "$MYSQL_USER"
echo ""

echo "Redis Password:"
REDIS_PASS=$(openssl rand -base64 32)
echo "$REDIS_PASS"
echo ""

echo "=== Copy these to your .env file ==="
echo ""
echo "MYSQL_ROOT_PASSWORD=$MYSQL_ROOT"
echo "MYSQL_PASSWORD=$MYSQL_USER"
echo "REDIS_PASSWORD=$REDIS_PASS"
echo ""

echo "=== Or update .env automatically ==="
echo "Run: ./scripts/update-env-passwords.sh"
