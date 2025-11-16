#!/bin/bash

set -e

echo "=== File Transfer System Test ==="
echo ""

# Cleanup function
cleanup() {
    echo ""
    echo "Cleaning up..."
    kill $SERVER_A_PID 2>/dev/null || true
    kill $SERVER_B_PID 2>/dev/null || true
    wait $SERVER_A_PID 2>/dev/null || true
    wait $SERVER_B_PID 2>/dev/null || true
    echo "Test completed."
}

trap cleanup EXIT

# Start Server A
echo "Starting Server A..."
cd /home/file-transfer-system
GRPC_LISTEN_ADDR=0.0.0.0:50051 \
HTTP_LISTEN_ADDR=0.0.0.0:8080 \
TARGET_SERVER=localhost:50052 \
ALLOWED_DIR=/home/data-a \
./server > /tmp/server-a.log 2>&1 &
SERVER_A_PID=$!
echo "Server A started (PID: $SERVER_A_PID)"

# Start Server B
echo "Starting Server B..."
GRPC_LISTEN_ADDR=0.0.0.0:50052 \
HTTP_LISTEN_ADDR=0.0.0.0:8081 \
TARGET_SERVER=localhost:50051 \
ALLOWED_DIR=/home/data-b \
./server > /tmp/server-b.log 2>&1 &
SERVER_B_PID=$!
echo "Server B started (PID: $SERVER_B_PID)"

# Wait for servers to start
echo ""
echo "Waiting for servers to start..."
sleep 3

# Health check for both servers
echo ""
echo "=== Health Check ==="
echo "Server A:"
curl -s http://localhost:8080/health
echo ""
echo "Server B:"
curl -s http://localhost:8081/health

# Test 1: Transfer 500MB file from Server A to Server B
echo ""
echo "=== Test 1: Transfer 500MB file from data-a to data-b (via Server A) ==="
echo "Source: /home/data-a/test-500mb.bin"
echo "Destination: /home/data-b/received-500mb.bin"
echo ""

START_TIME=$(date +%s)
curl -X POST http://localhost:8080/transfer \
  -H "Content-Type: application/json" \
  -d '{
    "source_path": "local:/test-500mb.bin",
    "dest_path": "peer:/received-500mb.bin"
  }' 2>/dev/null
END_TIME=$(date +%s)
ELAPSED=$((END_TIME - START_TIME))

echo ""
echo "Transfer completed in $ELAPSED seconds"

# Verify file was transferred
if [ -f /home/data-b/received-500mb.bin ]; then
    SIZE_A=$(stat -f%z /home/data-a/test-500mb.bin 2>/dev/null || stat -c%s /home/data-a/test-500mb.bin)
    SIZE_B=$(stat -f%z /home/data-b/received-500mb.bin 2>/dev/null || stat -c%s /home/data-b/received-500mb.bin)
    echo "✓ File transferred successfully"
    echo "  Source size: $SIZE_A bytes"
    echo "  Destination size: $SIZE_B bytes"
    
    if [ "$SIZE_A" = "$SIZE_B" ]; then
        echo "  ✓ File sizes match!"
    else
        echo "  ✗ File sizes do not match!"
        exit 1
    fi
else
    echo "✗ File transfer failed - destination file not found"
    exit 1
fi

# Create a test file in data-b for reverse transfer
echo ""
echo "=== Creating test file in data-b for reverse transfer ==="
dd if=/dev/urandom of=/home/data-b/test-reverse.bin bs=1M count=100 2>&1 | tail -n 1

# Test 2: Transfer file from Server B to Server A
echo ""
echo "=== Test 2: Transfer 100MB file from data-b to data-a (via Server B) ==="
echo "Source: /home/data-b/test-reverse.bin"
echo "Destination: /home/data-a/received-reverse.bin"
echo ""

START_TIME=$(date +%s)あ
curl -X POST http://localhost:8081/transfer \
  -H "Content-Type: application/json" \
  -d '{
    "source_path": "local:/test-reverse.bin",
    "dest_path": "peer:/received-reverse.bin"
  }' 2>/dev/null
END_TIME=$(date +%s)
ELAPSED=$((END_TIME - START_TIME))

echo ""
echo "Transfer completed in $ELAPSED seconds"

# Verify file was transferred
if [ -f /home/data-a/received-reverse.bin ]; then
    SIZE_A=$(stat -f%z /home/data-b/test-reverse.bin 2>/dev/null || stat -c%s /home/data-b/test-reverse.bin)
    SIZE_B=$(stat -f%z /home/data-a/received-reverse.bin 2>/dev/null || stat -c%s /home/data-a/received-reverse.bin)
    echo "✓ File transferred successfully"
    echo "  Source size: $SIZE_A bytes"
    echo "  Destination size: $SIZE_B bytes"
    
    if [ "$SIZE_A" = "$SIZE_B" ]; then
        echo "  ✓ File sizes match!"
    else
        echo "  ✗ File sizes do not match!"
        exit 1
    fi
else
    echo "✗ File transfer failed - destination file not found"
    exit 1
fi

# Summary
echo ""
echo "=== Test Summary ==="
echo "✓ All tests passed!"
echo "✓ Server A and Server B are working correctly"
echo "✓ Bidirectional file transfer working"
echo "✓ Large file (500MB) transfer successful"
echo ""
echo "Files in data-a:"
ls -lh /home/data-a/
echo ""
echo "Files in data-b:"
ls -lh /home/data-b/
