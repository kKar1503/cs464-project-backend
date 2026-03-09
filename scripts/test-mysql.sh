#!/bin/bash

# Simple MySQL connection test script

echo "=== MySQL Connection Test ==="
echo ""

# Get password from .env
MYSQL_PASS=$(grep MYSQL_PASSWORD .env | cut -d'=' -f2)
MYSQL_USER=$(grep MYSQL_USER .env | cut -d'=' -f2)
MYSQL_DB=$(grep MYSQL_DATABASE .env | cut -d'=' -f2)

echo "Testing MySQL with password..."
echo ""

# Test 1: Connection
echo "1. Connection test:"
docker exec game-mysql mysql -u "$MYSQL_USER" -p"$MYSQL_PASS" -e "SELECT 'Connected!' as Status;" 2>&1 | grep -v Warning

# Test 2: Show databases
echo ""
echo "2. Available databases:"
docker exec game-mysql mysql -u "$MYSQL_USER" -p"$MYSQL_PASS" -e "SHOW DATABASES;" 2>&1 | grep -v Warning

# Test 3: Show tables in gamedb
echo ""
echo "3. Tables in $MYSQL_DB:"
docker exec game-mysql mysql -u "$MYSQL_USER" -p"$MYSQL_PASS" "$MYSQL_DB" -e "SHOW TABLES;" 2>&1 | grep -v Warning

# Test 4: Count users
echo ""
echo "4. User count:"
docker exec game-mysql mysql -u "$MYSQL_USER" -p"$MYSQL_PASS" "$MYSQL_DB" -e "SELECT COUNT(*) as user_count FROM users;" 2>&1 | grep -v Warning

echo ""
echo "=== Test complete! ==="
