#!/bin/bash

# Automatically update .env file with new random passwords

ENV_FILE=".env"

if [ ! -f "$ENV_FILE" ]; then
    echo "Error: .env file not found!"
    echo "Please copy .env.example to .env first:"
    echo "  cp .env.example .env"
    exit 1
fi

echo "=== Updating passwords in $ENV_FILE ==="
echo ""

# Generate new passwords
MYSQL_ROOT=$(openssl rand -base64 24)
MYSQL_USER=$(openssl rand -base64 24)
REDIS_PASS=$(openssl rand -base64 32)

# Backup original .env
cp "$ENV_FILE" "${ENV_FILE}.backup"
echo "✓ Backup created: ${ENV_FILE}.backup"

# Update passwords using sed (macOS compatible)
# Escape special characters for sed
MYSQL_ROOT_ESC=$(echo "$MYSQL_ROOT" | sed 's/[\/&]/\\&/g')
MYSQL_USER_ESC=$(echo "$MYSQL_USER" | sed 's/[\/&]/\\&/g')
REDIS_PASS_ESC=$(echo "$REDIS_PASS" | sed 's/[\/&]/\\&/g')

if [[ "$OSTYPE" == "darwin"* ]]; then
    # macOS
    sed -i '' "s/^MYSQL_ROOT_PASSWORD=.*/MYSQL_ROOT_PASSWORD=$MYSQL_ROOT_ESC/" "$ENV_FILE"
    sed -i '' "s/^MYSQL_PASSWORD=.*/MYSQL_PASSWORD=$MYSQL_USER_ESC/" "$ENV_FILE"
    sed -i '' "s/^REDIS_PASSWORD=.*/REDIS_PASSWORD=$REDIS_PASS_ESC/" "$ENV_FILE"
else
    # Linux
    sed -i "s/^MYSQL_ROOT_PASSWORD=.*/MYSQL_ROOT_PASSWORD=$MYSQL_ROOT_ESC/" "$ENV_FILE"
    sed -i "s/^MYSQL_PASSWORD=.*/MYSQL_PASSWORD=$MYSQL_USER_ESC/" "$ENV_FILE"
    sed -i "s/^REDIS_PASSWORD=.*/REDIS_PASSWORD=$REDIS_PASS_ESC/" "$ENV_FILE"
fi

echo "✓ Passwords updated in $ENV_FILE"
echo ""
echo "=== New Passwords ==="
echo "MYSQL_ROOT_PASSWORD=$MYSQL_ROOT"
echo "MYSQL_PASSWORD=$MYSQL_USER"
echo "REDIS_PASSWORD=$REDIS_PASS"
echo ""
echo "⚠️  IMPORTANT: Save these passwords securely!"
echo ""
echo "Next steps:"
echo "  1. Restart services: make docker-down && make docker-up"
echo "  2. Store backup in a safe place: ${ENV_FILE}.backup"
echo "  3. Never commit .env to git!"
