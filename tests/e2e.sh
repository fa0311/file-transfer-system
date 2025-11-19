#!/bin/bash

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Test configuration
TEST_DIR="/tmp/test-transfer-$$"
SENDER_DIR="${TEST_DIR}/sender"
RECEIVER_DIR="${TEST_DIR}/receiver"
RECEIVER_PORT=50051
SENDER_PORT=8080

# Cleanup function
cleanup() {
    echo -e "${YELLOW}Cleaning up...${NC}"
    if [ ! -z "$RECEIVER_PID" ]; then
        kill $RECEIVER_PID 2>/dev/null || true
    fi
    if [ ! -z "$SENDER_PID" ]; then
        kill $SENDER_PID 2>/dev/null || true
    fi
    rm -rf "${TEST_DIR}"
}

trap cleanup EXIT

# Print test header
print_test_header() {
    echo ""
    echo "=============================================="
    echo "$1"
    echo "=============================================="
}

# Print test result
print_result() {
    if [ $1 -eq 0 ]; then
        echo -e "${GREEN}✓ PASS${NC}: $2"
    else
        echo -e "${RED}✗ FAIL${NC}: $2"
        exit 1
    fi
}

# Setup test environment
print_test_header "Setting up test environment"
mkdir -p "${SENDER_DIR}"
mkdir -p "${RECEIVER_DIR}"

# Create test files
echo "Creating test files..."
echo "Hello, World!" > "${SENDER_DIR}/small.txt"
dd if=/dev/urandom of="${SENDER_DIR}/medium.bin" bs=1M count=10 2>/dev/null
dd if=/dev/urandom of="${SENDER_DIR}/large.bin" bs=1M count=100 2>/dev/null

# Calculate checksums
SMALL_MD5=$(md5sum "${SENDER_DIR}/small.txt" | awk '{print $1}')
MEDIUM_MD5=$(md5sum "${SENDER_DIR}/medium.bin" | awk '{print $1}')
LARGE_MD5=$(md5sum "${SENDER_DIR}/large.bin" | awk '{print $1}')

echo "Test files created:"
echo "  - small.txt: $(stat -f%z "${SENDER_DIR}/small.txt" 2>/dev/null || stat -c%s "${SENDER_DIR}/small.txt") bytes"
echo "  - medium.bin: $(stat -f%z "${SENDER_DIR}/medium.bin" 2>/dev/null || stat -c%s "${SENDER_DIR}/medium.bin") bytes"
echo "  - large.bin: $(stat -f%z "${SENDER_DIR}/large.bin" 2>/dev/null || stat -c%s "${SENDER_DIR}/large.bin") bytes"

# Start receiver server
print_test_header "Starting receiver server"
PEER_SERVER_ADDR="localhost:${SENDER_PORT}" \
ROOT_DIR="${RECEIVER_DIR}" \
GRPC_PORT=${RECEIVER_PORT} \
HTTP_PORT=8081 \
./bin/file-transfer-server > "${TEST_DIR}/receiver.log" 2>&1 &
RECEIVER_PID=$!

echo "Receiver PID: $RECEIVER_PID"
echo "Waiting for receiver to start..."
sleep 2

# Check if receiver is running
if ! kill -0 $RECEIVER_PID 2>/dev/null; then
    echo -e "${RED}Failed to start receiver server${NC}"
    cat "${TEST_DIR}/receiver.log"
    exit 1
fi
print_result 0 "Receiver server started"

# Start sender server
print_test_header "Starting sender server"
PEER_SERVER_ADDR="localhost:${RECEIVER_PORT}" \
ROOT_DIR="${SENDER_DIR}" \
HTTP_PORT=${SENDER_PORT} \
GRPC_PORT=50052 \
./bin/file-transfer-server > "${TEST_DIR}/sender.log" 2>&1 &
SENDER_PID=$!

echo "Sender PID: $SENDER_PID"
echo "Waiting for sender to start..."
sleep 2

# Check if sender is running
if ! kill -0 $SENDER_PID 2>/dev/null; then
    echo -e "${RED}Failed to start sender server${NC}"
    cat "${TEST_DIR}/sender.log"
    exit 1
fi
print_result 0 "Sender server started"

# Test 1: Transfer small file
print_test_header "Test 1: Transfer small text file"
curl -X POST http://localhost:${SENDER_PORT}/transfer \
    -H "Content-Type: application/json" \
    -d '{"source":"small.txt","target":"small.txt"}' \
    > "${TEST_DIR}/transfer1.log" 2>&1

sleep 1

if [ -f "${RECEIVER_DIR}/small.txt" ]; then
    RECEIVED_MD5=$(md5sum "${RECEIVER_DIR}/small.txt" | awk '{print $1}')
    if [ "$SMALL_MD5" = "$RECEIVED_MD5" ]; then
        print_result 0 "Small file transferred successfully and checksum matches"
    else
        print_result 1 "Small file checksum mismatch"
    fi
else
    print_result 1 "Small file not found on receiver"
fi

# Test 2: Transfer medium file
print_test_header "Test 2: Transfer medium binary file (10MB)"
curl -X POST http://localhost:${SENDER_PORT}/transfer \
    -H "Content-Type: application/json" \
    -d '{"source":"medium.bin","target":"medium.bin"}' \
    > "${TEST_DIR}/transfer2.log" 2>&1

sleep 2

if [ -f "${RECEIVER_DIR}/medium.bin" ]; then
    RECEIVED_MD5=$(md5sum "${RECEIVER_DIR}/medium.bin" | awk '{print $1}')
    if [ "$MEDIUM_MD5" = "$RECEIVED_MD5" ]; then
        print_result 0 "Medium file transferred successfully and checksum matches"
    else
        print_result 1 "Medium file checksum mismatch"
    fi
else
    print_result 1 "Medium file not found on receiver"
fi

# Test 3: Transfer large file
print_test_header "Test 3: Transfer large binary file (100MB)"
echo "This may take a few seconds..."
START_TIME=$(date +%s)

curl -X POST http://localhost:${SENDER_PORT}/transfer \
    -H "Content-Type: application/json" \
    -d '{"source":"large.bin","target":"large.bin"}' \
    > "${TEST_DIR}/transfer3.log" 2>&1

END_TIME=$(date +%s)
DURATION=$((END_TIME - START_TIME))

sleep 2

if [ -f "${RECEIVER_DIR}/large.bin" ]; then
    RECEIVED_MD5=$(md5sum "${RECEIVER_DIR}/large.bin" | awk '{print $1}')
    if [ "$LARGE_MD5" = "$RECEIVED_MD5" ]; then
        if [ $DURATION -gt 0 ]; then
            THROUGHPUT=$((100 / DURATION))
            echo "Transfer time: ${DURATION}s, Throughput: ~${THROUGHPUT}MB/s"
        else
            echo "Transfer time: <1s, Throughput: >100MB/s"
        fi
        print_result 0 "Large file transferred successfully and checksum matches"
    else
        print_result 1 "Large file checksum mismatch"
    fi
else
    print_result 1 "Large file not found on receiver"
fi

# Test 4: Transfer to subdirectory
print_test_header "Test 4: Transfer to subdirectory"
curl -X POST http://localhost:${SENDER_PORT}/transfer \
    -H "Content-Type: application/json" \
    -d '{"source":"small.txt","target":"subdir/nested/small.txt"}' \
    > "${TEST_DIR}/transfer4.log" 2>&1

sleep 1

if [ -f "${RECEIVER_DIR}/subdir/nested/small.txt" ]; then
    RECEIVED_MD5=$(md5sum "${RECEIVER_DIR}/subdir/nested/small.txt" | awk '{print $1}')
    if [ "$SMALL_MD5" = "$RECEIVED_MD5" ]; then
        print_result 0 "File transferred to subdirectory successfully"
    else
        print_result 1 "Subdirectory file checksum mismatch"
    fi
else
    print_result 1 "File not found in subdirectory"
fi

# Test 5: Check NDJSON log format
print_test_header "Test 5: Verify NDJSON log format"
if grep -q '"timestamp"' "${TEST_DIR}/transfer1.log" && \
   grep -q '"level"' "${TEST_DIR}/transfer1.log" && \
   grep -q '"message"' "${TEST_DIR}/transfer1.log" && \
   grep -q '"bytes_transferred"' "${TEST_DIR}/transfer1.log"; then
    print_result 0 "NDJSON log format is correct"
else
    print_result 1 "NDJSON log format is incorrect"
fi

# Test 6: Check health endpoint
print_test_header "Test 6: Health check endpoint"
HEALTH_STATUS=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:${SENDER_PORT}/health)
if [ "$HEALTH_STATUS" = "200" ]; then
    print_result 0 "Health endpoint is working"
else
    print_result 1 "Health endpoint returned status $HEALTH_STATUS"
fi

# Print summary
print_test_header "Test Summary"
echo -e "${GREEN}All tests passed!${NC}"
echo ""
echo "Receiver log:"
tail -n 20 "${TEST_DIR}/receiver.log"
echo ""
echo "Sender log:"
tail -n 20 "${TEST_DIR}/sender.log"

exit 0
