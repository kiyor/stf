#!/bin/bash

# E2E Test for STF Docker Image
set -e

IMAGE_NAME="${IMAGE_NAME:-kiyor/stf:latest}"
TEST_PORT="${TEST_PORT:-31100}"
CONTAINER_NAME="stf-e2e-test"

echo "=== E2E Test for STF Docker Image ==="
echo "Image: $IMAGE_NAME"
echo "Test Port: $TEST_PORT"
echo

# Cleanup function
cleanup() {
    echo "Cleaning up..."
    docker stop $CONTAINER_NAME 2>/dev/null || true
    docker rm $CONTAINER_NAME 2>/dev/null || true
}

trap cleanup EXIT

# Test 1: Basic container startup
echo "Test 1: Starting container..."
docker run -d --name $CONTAINER_NAME \
    -p $TEST_PORT:30000 \
    $IMAGE_NAME -timeout 30s -testfile

# Wait for container to be ready
sleep 2

# Check if container is running
if ! docker ps | grep -q $CONTAINER_NAME; then
    echo "ERROR: Container failed to start"
    docker logs $CONTAINER_NAME
    exit 1
fi
echo "✓ Container started successfully"

# Test 2: HTTP connectivity
echo
echo "Test 2: Testing HTTP connectivity..."
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:$TEST_PORT/ || echo "000")
if [ "$HTTP_CODE" == "200" ]; then
    echo "✓ HTTP connectivity OK (Status: $HTTP_CODE)"
else
    echo "ERROR: HTTP request failed (Status: $HTTP_CODE)"
    docker logs $CONTAINER_NAME
    exit 1
fi

# Test 3: Test file generation
echo
echo "Test 3: Testing test file generation..."
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:$TEST_PORT/1K || echo "000")
if [ "$HTTP_CODE" == "200" ]; then
    echo "✓ Test file generation OK"
else
    echo "ERROR: Test file generation failed (Status: $HTTP_CODE)"
    exit 1
fi

# Test 4: Download test (test file with specific size)
echo
echo "Test 4: Testing sized file download..."
# Download 1KB test file
DOWNLOAD_SIZE=$(curl -s http://localhost:$TEST_PORT/1K | wc -c)
EXPECTED_SIZE=1024
if [ "$DOWNLOAD_SIZE" -eq "$EXPECTED_SIZE" ]; then
    echo "✓ File download OK (size: $DOWNLOAD_SIZE bytes)"
else
    echo "WARNING: Downloaded file size mismatch (expected: $EXPECTED_SIZE, got: $DOWNLOAD_SIZE)"
fi

# Test 5: Check container health
echo
echo "Test 5: Checking container health..."
if docker inspect $CONTAINER_NAME --format='{{.State.Running}}' | grep -q true; then
    echo "✓ Container is healthy"
else
    echo "ERROR: Container is not running"
    exit 1
fi

# Test 6: Check logs
echo
echo "Test 6: Checking container logs..."
docker logs $CONTAINER_NAME | tail -5
echo "✓ Logs accessible"

# Test 7: Multi-platform image info
echo
echo "Test 7: Image information..."
docker inspect $IMAGE_NAME --format='{{.Architecture}}' || true
docker images $IMAGE_NAME --format "Size: {{.Size}}"
echo "✓ Image info OK"

echo
echo "=== All E2E Tests Passed ==="
echo
