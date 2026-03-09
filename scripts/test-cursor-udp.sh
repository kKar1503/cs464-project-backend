#!/bin/bash

# Simple test script for the cursor UDP service

echo "Testing Cursor UDP Service..."
echo "=============================="
echo ""

# Start the service in the background
echo "Starting cursor-udp service..."
./bin/cursor-udp &
SERVICE_PID=$!

# Give it time to start
sleep 1

# Send test data
echo ""
echo "Sending test cursor data..."
echo "cursor_move:100,200" | nc -u -w1 localhost 9001

# Wait a bit for the response
sleep 1

# Clean up
echo ""
echo "Stopping service..."
kill $SERVICE_PID 2>/dev/null

echo ""
echo "Test complete!"
