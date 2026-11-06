#!/bin/bash

# Test script for timeout functionality

echo "=== Testing Timeout Functionality ==="
echo

# Test 1: Basic timeout test (10 seconds)
echo "Test 1: Basic timeout - server should stop accepting connections after 10s"
echo "Starting server with 10s timeout..."
./stf -timeout 10s -p :31000 &
SERVER_PID=$!
echo "Server PID: $SERVER_PID"

# Wait 5 seconds and test (should work)
sleep 5
echo "After 5s - Testing connection (should succeed):"
curl -s -o /dev/null -w "HTTP Status: %{http_code}\n" http://localhost:31000/ || echo "Connection failed"

# Wait another 6 seconds (total 11s) and test (should fail)
sleep 6
echo "After 11s total - Testing connection (should fail with 503):"
curl -s -o /dev/null -w "HTTP Status: %{http_code}\n" http://localhost:31000/ || echo "Connection refused"

# Kill the server if still running
kill $SERVER_PID 2>/dev/null
wait $SERVER_PID 2>/dev/null

echo
echo "---"
echo

# Test 2: No timeout test
echo "Test 2: No timeout mode - server should run indefinitely"
echo "Starting server with -notimeout flag..."
./stf -notimeout -p :31001 &
SERVER_PID=$!
echo "Server PID: $SERVER_PID"

# Wait 2 seconds and test
sleep 2
echo "After 2s - Testing connection (should succeed):"
curl -s -o /dev/null -w "HTTP Status: %{http_code}\n" http://localhost:31001/ || echo "Connection failed"

# Wait another 5 seconds and test again (should still work)
sleep 5
echo "After 7s total - Testing connection (should still succeed):"
curl -s -o /dev/null -w "HTTP Status: %{http_code}\n" http://localhost:31001/ || echo "Connection failed"

# Kill the server
echo "Killing server..."
kill $SERVER_PID
wait $SERVER_PID 2>/dev/null

echo
echo "---"
echo

# Test 3: Long connection handling
echo "Test 3: Long connection handling - existing connections should complete"
echo "Starting server with 10s timeout..."
./stf -timeout 10s -p :31002 &
SERVER_PID=$!
echo "Server PID: $SERVER_PID"

# Create a test file for download
dd if=/dev/zero of=testfile_large.bin bs=1M count=10 2>/dev/null

# Start a slow download (will take more than 10s)
sleep 2
echo "Starting slow download (rate limited to 100KB/s)..."
curl -s --limit-rate 100k http://localhost:31002/testfile_large.bin -o /dev/null &
DOWNLOAD_PID=$!

# Wait for timeout to trigger
sleep 9
echo "After 11s - Timeout should have triggered, checking if download continues..."

# Check if download is still running
if ps -p $DOWNLOAD_PID > /dev/null; then
    echo "Download still running (good - existing connections continue)"
else
    echo "Download stopped (unexpected)"
fi

# Try new connection (should fail)
echo "Trying new connection (should fail with 503):"
curl -s -o /dev/null -w "HTTP Status: %{http_code}\n" http://localhost:31002/ || echo "Connection refused"

# Clean up
kill $DOWNLOAD_PID 2>/dev/null
kill $SERVER_PID 2>/dev/null
wait $SERVER_PID 2>/dev/null
wait $DOWNLOAD_PID 2>/dev/null
rm -f testfile_large.bin

echo
echo "=== All Tests Completed ==="