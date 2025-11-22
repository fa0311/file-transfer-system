#!/bin/bash

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Configuration
PEER_SERVER="192.168.70.3:50051"
LOCAL_DIR="/tmp/transfer-test"
FILE_SIZE_GB=1
HTTP_PORT=8080
GRPC_PORT=50052

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}  10GB File Transfer Test${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""
echo "Target Server: ${PEER_SERVER}"
echo "File Size: ${FILE_SIZE_GB}GB"
echo ""

# Cleanup function
cleanup() {
    echo -e "${YELLOW}Cleaning up...${NC}"
    if [ ! -z "$SERVER_PID" ]; then
        kill $SERVER_PID 2>/dev/null || true
        wait $SERVER_PID 2>/dev/null || true
    fi
}
trap cleanup EXIT

# Step 1: Create test directory
echo -e "${GREEN}[1/5]${NC} Creating test directory..."
mkdir -p "${LOCAL_DIR}"

# Step 2: Create test file
TEST_FILE="${LOCAL_DIR}/test-${FILE_SIZE_GB}gb.bin"
if [ -f "$TEST_FILE" ]; then
    echo -e "${YELLOW}Test file already exists, skipping creation${NC}"
else
    echo -e "${GREEN}[2/5]${NC} Creating ${FILE_SIZE_GB}GB test file..."
    echo "This may take a few seconds..."
    dd if=/dev/zero of="${TEST_FILE}" bs=1M count=$((FILE_SIZE_GB * 1024)) 2>&1 | tail -3
fi

FILE_SIZE=$(stat -c%s "${TEST_FILE}" 2>/dev/null || stat -f%z "${TEST_FILE}")
echo "File size: $(numfmt --to=iec-i --suffix=B $FILE_SIZE 2>/dev/null || echo "${FILE_SIZE} bytes")"
echo ""

# Step 3: Build server
echo -e "${GREEN}[3/5]${NC} Building server..."
export PATH=$PATH:$(go env GOPATH)/bin
make build > /dev/null 2>&1

# Step 4: Start local server
echo -e "${GREEN}[4/5]${NC} Starting local server..."
PEER_SERVER_ADDR="${PEER_SERVER}" \
ROOT_DIR="${LOCAL_DIR}" \
HTTP_PORT=${HTTP_PORT} \
GRPC_PORT=${GRPC_PORT} \
./bin/file-transfer-server > /tmp/transfer-server.log 2>&1 &
SERVER_PID=$!

echo "Server PID: $SERVER_PID"
echo "Waiting for server to start..."
sleep 3

# Check if server is running
if ! kill -0 $SERVER_PID 2>/dev/null; then
    echo -e "${RED}Failed to start server${NC}"
    cat /tmp/transfer-server.log
    exit 1
fi
echo -e "${GREEN}Server started successfully${NC}"
echo ""

# Step 5: Transfer file
RESPONSE_LOG="${LOCAL_DIR}/transfer_response.ndjson"
REQUEST_BODY="{\"source\":\"test-${FILE_SIZE_GB}gb.bin\",\"target\":\"test-${FILE_SIZE_GB}gb.bin\"}"

echo -e "${GREEN}[5/5]${NC} Transferring ${FILE_SIZE_GB}GB file..."
echo "This will take a while..."
echo "Saving response to: ${RESPONSE_LOG}"
echo ""
echo "Request URL: http://localhost:${HTTP_PORT}/transfer"
echo "Request Method: POST"
echo "Request Headers: Content-Type: application/json"
echo "Request Body:"
echo "$REQUEST_BODY" | jq . 2>/dev/null || echo "$REQUEST_BODY"
echo ""

START_TIME=$(date +%s)


curl -X POST "http://localhost:${HTTP_PORT}/transfer" \
    -H "Content-Type: application/json" \
    -d "$REQUEST_BODY"