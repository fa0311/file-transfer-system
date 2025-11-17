#!/bin/bash

set -e

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Test results tracking
TESTS_PASSED=0
TESTS_FAILED=0
TESTS_TOTAL=0

echo "=========================================="
echo "  File Transfer System E2E Test Suite"
echo "=========================================="
echo ""

# Cleanup function
cleanup() {
    echo ""
    echo "Cleaning up..."
    kill $SERVER_A_PID 2>/dev/null || true
    kill $SERVER_B_PID 2>/dev/null || true
    wait $SERVER_A_PID 2>/dev/null || true
    wait $SERVER_B_PID 2>/dev/null || true
    
    # Clean up test files
    rm -f /home/data-a/test-*.bin 2>/dev/null || true
    rm -f /home/data-b/test-*.bin 2>/dev/null || true
    rm -f /home/data-a/received-*.bin 2>/dev/null || true
    rm -f /home/data-b/received-*.bin 2>/dev/null || true
    rm -f /home/data-a/empty.txt 2>/dev/null || true
    rm -f /home/data-b/empty.txt 2>/dev/null || true
    
    echo ""
    echo "=========================================="
    echo "  Test Results Summary"
    echo "=========================================="
    echo -e "${GREEN}Passed:${NC} $TESTS_PASSED"
    echo -e "${RED}Failed:${NC} $TESTS_FAILED"
    echo -e "Total:  $TESTS_TOTAL"
    echo ""
    
    if [ $TESTS_FAILED -eq 0 ]; then
        echo -e "${GREEN}✓ All tests passed!${NC}"
        exit 0
    else
        echo -e "${RED}✗ Some tests failed!${NC}"
        exit 1
    fi
}

trap cleanup EXIT

# Helper function to run a test
run_test() {
    local test_name=$1
    local test_func=$2
    
    TESTS_TOTAL=$((TESTS_TOTAL + 1))
    echo ""
    echo -e "${BLUE}[TEST $TESTS_TOTAL]${NC} $test_name"
    echo "----------------------------------------"
    
    if $test_func; then
        TESTS_PASSED=$((TESTS_PASSED + 1))
        echo -e "${GREEN}✓ PASSED${NC}"
    else
        TESTS_FAILED=$((TESTS_FAILED + 1))
        echo -e "${RED}✗ FAILED${NC}"
    fi
}

# Helper function to check file size
check_file_size() {
    local file=$1
    local expected_size=$2
    
    if [ ! -f "$file" ]; then
        echo "File not found: $file"
        return 1
    fi
    
    actual_size=$(stat -f%z "$file" 2>/dev/null || stat -c%s "$file")
    
    if [ "$actual_size" -eq "$expected_size" ]; then
        echo "✓ File size matches: $actual_size bytes"
        return 0
    else
        echo "✗ File size mismatch: expected $expected_size, got $actual_size"
        return 1
    fi
}

# Helper function to check file exists
check_file_exists() {
    local file=$1
    
    if [ -f "$file" ]; then
        echo "✓ File exists: $file"
        return 0
    else
        echo "✗ File not found: $file"
        return 1
    fi
}

# Helper function to check file does not exist
check_file_not_exists() {
    local file=$1
    
    if [ ! -f "$file" ]; then
        echo "✓ File correctly does not exist: $file"
        return 0
    else
        echo "✗ File unexpectedly exists: $file"
        return 1
    fi
}

# Helper function to calculate SHA256
calculate_sha256() {
    local file=$1
    sha256sum "$file" 2>/dev/null | awk '{print $1}' || shasum -a 256 "$file" | awk '{print $1}'
}

# Start servers
echo "Setting up test environment..."
echo "----------------------------------------"

cd /home/file-transfer-system

# Start Server A
echo "Starting Server A..."
GRPC_LISTEN_ADDR=0.0.0.0:50051 \
HTTP_LISTEN_ADDR=0.0.0.0:8080 \
TARGET_SERVER=localhost:50052 \
ALLOWED_DIR=/home/data-a \
./server > /tmp/server-a.log 2>&1 &
SERVER_A_PID=$!
echo "  PID: $SERVER_A_PID"

# Start Server B
echo "Starting Server B..."
GRPC_LISTEN_ADDR=0.0.0.0:50052 \
HTTP_LISTEN_ADDR=0.0.0.0:8081 \
TARGET_SERVER=localhost:50051 \
ALLOWED_DIR=/home/data-b \
./server > /tmp/server-b.log 2>&1 &
SERVER_B_PID=$!
echo "  PID: $SERVER_B_PID"

# Wait for servers to start
echo ""
echo "Waiting for servers to initialize..."
sleep 3

# ==========================================
# Test Suite
# ==========================================

# Test 1: Health Check
test_health_check() {
    echo "Checking Server A health..."
    response=$(curl -s http://localhost:8080/health)
    if echo "$response" | grep -q "healthy"; then
        echo "✓ Server A is healthy"
    else
        echo "✗ Server A health check failed"
        return 1
    fi
    
    echo "Checking Server B health..."
    response=$(curl -s http://localhost:8081/health)
    if echo "$response" | grep -q "healthy"; then
        echo "✓ Server B is healthy"
        return 0
    else
        echo "✗ Server B health check failed"
        return 1
    fi
}

# Test 2: Empty File Transfer
test_empty_file_transfer() {
    echo "Creating empty file..."
    touch /home/data-a/empty.txt
    
    echo "Transferring empty file from A to B..."
    response=$(curl -s -X POST http://localhost:8080/transfer \
        -H "Content-Type: application/json" \
        -d '{"source_path": "local:/empty.txt", "dest_path": "peer:/empty.txt"}')
    
    if echo "$response" | grep -q "completed"; then
        check_file_exists "/home/data-b/empty.txt" && \
        check_file_size "/home/data-b/empty.txt" 0
        return $?
    else
        echo "✗ Transfer response did not indicate completion"
        return 1
    fi
}

# Test 3: Small File Transfer (< 1MB)
test_small_file_transfer() {
    echo "Creating 100KB test file..."
    dd if=/dev/urandom of=/home/data-a/test-small.bin bs=1K count=100 2>&1 | tail -n 1
    
    echo "Transferring small file from A to B..."
    start_time=$(date +%s)
    response=$(curl -s -X POST http://localhost:8080/transfer \
        -H "Content-Type: application/json" \
        -d '{"source_path": "local:/test-small.bin", "dest_path": "peer:/test-small.bin"}')
    end_time=$(date +%s)
    elapsed=$((end_time - start_time))
    
    echo "Transfer completed in $elapsed seconds"
    
    if echo "$response" | grep -q "completed"; then
        src_size=$(stat -f%z /home/data-a/test-small.bin 2>/dev/null || stat -c%s /home/data-a/test-small.bin)
        check_file_exists "/home/data-b/test-small.bin" && \
        check_file_size "/home/data-b/test-small.bin" "$src_size"
        return $?
    else
        echo "✗ Transfer failed"
        return 1
    fi
}

# Test 4: Large File Transfer (10MB)
test_large_file_transfer() {
    echo "Creating 10MB test file..."
    dd if=/dev/urandom of=/home/data-a/test-large.bin bs=1M count=10 2>&1 | tail -n 1
    
    echo "Transferring large file from A to B..."
    start_time=$(date +%s)
    response=$(curl -s -X POST http://localhost:8080/transfer \
        -H "Content-Type: application/json" \
        -d '{"source_path": "local:/test-large.bin", "dest_path": "peer:/test-large.bin"}')
    end_time=$(date +%s)
    elapsed=$((end_time - start_time))
    
    echo "Transfer completed in $elapsed seconds"
    
    if echo "$response" | grep -q "completed"; then
        src_size=$(stat -f%z /home/data-a/test-large.bin 2>/dev/null || stat -c%s /home/data-a/test-large.bin)
        check_file_exists "/home/data-b/test-large.bin" && \
        check_file_size "/home/data-b/test-large.bin" "$src_size"
        return $?
    else
        echo "✗ Transfer failed"
        return 1
    fi
}

# Test 5: Bidirectional Transfer
test_bidirectional_transfer() {
    echo "Creating test file in data-b..."
    dd if=/dev/urandom of=/home/data-b/test-reverse.bin bs=1M count=5 2>&1 | tail -n 1
    
    echo "Transferring file from B to A..."
    response=$(curl -s -X POST http://localhost:8081/transfer \
        -H "Content-Type: application/json" \
        -d '{"source_path": "local:/test-reverse.bin", "dest_path": "peer:/test-reverse.bin"}')
    
    if echo "$response" | grep -q "completed"; then
        src_size=$(stat -f%z /home/data-b/test-reverse.bin 2>/dev/null || stat -c%s /home/data-b/test-reverse.bin)
        check_file_exists "/home/data-a/test-reverse.bin" && \
        check_file_size "/home/data-a/test-reverse.bin" "$src_size"
        return $?
    else
        echo "✗ Transfer failed"
        return 1
    fi
}

# Test 6: File Overwrite
test_file_overwrite() {
    echo "Creating initial file..."
    echo "first version" > /home/data-a/test-overwrite.txt
    
    echo "Transferring initial file..."
    curl -s -X POST http://localhost:8080/transfer \
        -H "Content-Type: application/json" \
        -d '{"source_path": "local:/test-overwrite.txt", "dest_path": "peer:/test-overwrite.txt"}' > /dev/null
    
    echo "Creating updated file..."
    echo "second version updated" > /home/data-a/test-overwrite.txt
    
    echo "Transferring updated file (overwrite)..."
    response=$(curl -s -X POST http://localhost:8080/transfer \
        -H "Content-Type: application/json" \
        -d '{"source_path": "local:/test-overwrite.txt", "dest_path": "peer:/test-overwrite.txt"}')
    
    if echo "$response" | grep -q "completed"; then
        content=$(cat /home/data-b/test-overwrite.txt)
        if echo "$content" | grep -q "second version updated"; then
            echo "✓ File was correctly overwritten"
            return 0
        else
            echo "✗ File content does not match expected"
            return 1
        fi
    else
        echo "✗ Transfer failed"
        return 1
    fi
}

# Test 7: Data Integrity (SHA256 Checksum)
test_data_integrity() {
    echo "Creating test file for integrity check..."
    dd if=/dev/urandom of=/home/data-a/test-integrity.bin bs=1M count=3 2>&1 | tail -n 1
    
    echo "Calculating source file checksum..."
    src_checksum=$(calculate_sha256 /home/data-a/test-integrity.bin)
    echo "  Source SHA256: $src_checksum"
    
    echo "Transferring file..."
    response=$(curl -s -X POST http://localhost:8080/transfer \
        -H "Content-Type: application/json" \
        -d '{"source_path": "local:/test-integrity.bin", "dest_path": "peer:/test-integrity.bin"}')
    
    if echo "$response" | grep -q "completed"; then
        echo "Calculating destination file checksum..."
        dst_checksum=$(calculate_sha256 /home/data-b/test-integrity.bin)
        echo "  Destination SHA256: $dst_checksum"
        
        if [ "$src_checksum" = "$dst_checksum" ]; then
            echo "✓ Checksums match - data integrity verified"
            return 0
        else
            echo "✗ Checksums do not match - data corruption detected"
            return 1
        fi
    else
        echo "✗ Transfer failed"
        return 1
    fi
}

# Test 8: Delete Local File
test_delete_local_file() {
    echo "Creating file to delete..."
    echo "delete me" > /home/data-a/test-delete.txt
    check_file_exists "/home/data-a/test-delete.txt" || return 1
    
    echo "Deleting local file..."
    response=$(curl -s -X POST http://localhost:8080/delete \
        -H "Content-Type: application/json" \
        -d '{"file_path": "local:/test-delete.txt"}')
    
    if echo "$response" | grep -q '"success":true'; then
        check_file_not_exists "/home/data-a/test-delete.txt"
        return $?
    else
        echo "✗ Delete response indicated failure"
        return 1
    fi
}

# Test 9: Delete Peer File
test_delete_peer_file() {
    echo "Creating file on peer to delete..."
    echo "delete me on peer" > /home/data-b/test-delete-peer.txt
    check_file_exists "/home/data-b/test-delete-peer.txt" || return 1
    
    echo "Deleting peer file from Server A..."
    response=$(curl -s -X POST http://localhost:8080/delete \
        -H "Content-Type: application/json" \
        -d '{"file_path": "peer:/test-delete-peer.txt"}')
    
    if echo "$response" | grep -q '"success":true'; then
        check_file_not_exists "/home/data-b/test-delete-peer.txt"
        return $?
    else
        echo "✗ Delete response indicated failure"
        return 1
    fi
}

# Test 10: Error - Non-existent File
test_error_nonexistent_file() {
    echo "Attempting to transfer non-existent file..."
    response=$(curl -s -X POST http://localhost:8080/transfer \
        -H "Content-Type: application/json" \
        -d '{"source_path": "local:/nonexistent.bin", "dest_path": "peer:/should-not-exist.bin"}')
    
    if echo "$response" | grep -q "error"; then
        echo "✓ Error correctly reported for non-existent file"
        check_file_not_exists "/home/data-b/should-not-exist.bin"
        return $?
    else
        echo "✗ Expected error but got success"
        return 1
    fi
}

# Test 11: Error - Invalid Path Prefix
test_error_invalid_prefix() {
    echo "Attempting transfer with invalid prefix..."
    response=$(curl -s -X POST http://localhost:8080/transfer \
        -H "Content-Type: application/json" \
        -d '{"source_path": "invalid:/test.bin", "dest_path": "peer:/test.bin"}')
    
    if echo "$response" | grep -q "error"; then
        echo "✓ Error correctly reported for invalid prefix"
        return 0
    else
        echo "✗ Expected error but got success"
        return 1
    fi
}

# Test 12: Error - Path Traversal Attack
test_error_path_traversal() {
    echo "Attempting path traversal attack..."
    response=$(curl -s -X POST http://localhost:8080/transfer \
        -H "Content-Type: application/json" \
        -d '{"source_path": "local:/../etc/passwd", "dest_path": "peer:/passwd"}')
    
    if echo "$response" | grep -q "error"; then
        echo "✓ Path traversal attack correctly blocked"
        check_file_not_exists "/home/data-b/passwd"
        return $?
    else
        echo "✗ Security vulnerability - path traversal was allowed"
        return 1
    fi
}

# Test 13: Error - Relative Path
test_error_relative_path() {
    echo "Attempting to use relative path..."
    response=$(curl -s -X POST http://localhost:8080/transfer \
        -H "Content-Type: application/json" \
        -d '{"source_path": "local:relative/path.txt", "dest_path": "peer:/path.txt"}')
    
    if echo "$response" | grep -q "error"; then
        echo "✓ Relative path correctly rejected"
        return 0
    else
        echo "✗ Expected error but got success"
        return 1
    fi
}

# Test 14: Error - Delete Non-existent File
test_error_delete_nonexistent() {
    echo "Attempting to delete non-existent file..."
    response=$(curl -s -X POST http://localhost:8080/delete \
        -H "Content-Type: application/json" \
        -d '{"file_path": "local:/does-not-exist.txt"}')
    
    if echo "$response" | grep -q '"success":false'; then
        echo "✓ Delete correctly failed for non-existent file"
        return 0
    else
        echo "✗ Expected failure but got success"
        return 1
    fi
}

# Test 15: HTTP Method Validation
test_http_method_validation() {
    echo "Testing GET method on /transfer (should fail)..."
    status_code=$(curl -s -o /dev/null -w "%{http_code}" -X GET http://localhost:8080/transfer)
    
    if [ "$status_code" = "405" ]; then
        echo "✓ GET method correctly rejected (405)"
        return 0
    else
        echo "✗ Expected 405 but got $status_code"
        return 1
    fi
}

# Test 16: Concurrent Transfers
test_concurrent_transfers() {
    echo "Creating test files for concurrent transfer..."
    dd if=/dev/urandom of=/home/data-a/test-concurrent-1.bin bs=1M count=2 2>&1 | tail -n 1
    dd if=/dev/urandom of=/home/data-a/test-concurrent-2.bin bs=1M count=2 2>&1 | tail -n 1
    dd if=/dev/urandom of=/home/data-a/test-concurrent-3.bin bs=1M count=2 2>&1 | tail -n 1
    
    echo "Starting concurrent transfers..."
    curl -s -X POST http://localhost:8080/transfer \
        -H "Content-Type: application/json" \
        -d '{"source_path": "local:/test-concurrent-1.bin", "dest_path": "peer:/test-concurrent-1.bin"}' > /tmp/transfer1.log &
    PID1=$!
    
    curl -s -X POST http://localhost:8080/transfer \
        -H "Content-Type: application/json" \
        -d '{"source_path": "local:/test-concurrent-2.bin", "dest_path": "peer:/test-concurrent-2.bin"}' > /tmp/transfer2.log &
    PID2=$!
    
    curl -s -X POST http://localhost:8080/transfer \
        -H "Content-Type: application/json" \
        -d '{"source_path": "local:/test-concurrent-3.bin", "dest_path": "peer:/test-concurrent-3.bin"}' > /tmp/transfer3.log &
    PID3=$!
    
    echo "Waiting for all transfers to complete..."
    wait $PID1 $PID2 $PID3
    
    echo "Verifying all files were transferred..."
    check_file_exists "/home/data-b/test-concurrent-1.bin" && \
    check_file_exists "/home/data-b/test-concurrent-2.bin" && \
    check_file_exists "/home/data-b/test-concurrent-3.bin"
    return $?
}

# Test 17: Performance - Transfer Rate
test_transfer_performance() {
    echo "Creating 50MB test file for performance test..."
    dd if=/dev/urandom of=/home/data-a/test-performance.bin bs=1M count=50 2>&1 | tail -n 1
    
    echo "Measuring transfer performance..."
    start_time=$(date +%s)
    response=$(curl -s -X POST http://localhost:8080/transfer \
        -H "Content-Type: application/json" \
        -d '{"source_path": "local:/test-performance.bin", "dest_path": "peer:/test-performance.bin"}')
    end_time=$(date +%s)
    elapsed=$((end_time - start_time))
    
    if [ $elapsed -eq 0 ]; then
        elapsed=1
    fi
    
    size_mb=50
    rate=$((size_mb / elapsed))
    
    echo "  File size: 50 MB"
    echo "  Transfer time: $elapsed seconds"
    echo "  Transfer rate: ~$rate MB/s"
    
    if echo "$response" | grep -q "completed"; then
        src_size=$(stat -f%z /home/data-a/test-performance.bin 2>/dev/null || stat -c%s /home/data-a/test-performance.bin)
        check_file_size "/home/data-b/test-performance.bin" "$src_size"
        return $?
    else
        echo "✗ Transfer failed"
        return 1
    fi
}

# ==========================================
# Run all tests
# ==========================================

run_test "Health Check" test_health_check
run_test "Empty File Transfer" test_empty_file_transfer
run_test "Small File Transfer (<1MB)" test_small_file_transfer
run_test "Large File Transfer (10MB)" test_large_file_transfer
run_test "Bidirectional Transfer" test_bidirectional_transfer
run_test "File Overwrite" test_file_overwrite
run_test "Data Integrity (SHA256)" test_data_integrity
run_test "Delete Local File" test_delete_local_file
run_test "Delete Peer File" test_delete_peer_file
run_test "Error: Non-existent File" test_error_nonexistent_file
run_test "Error: Invalid Path Prefix" test_error_invalid_prefix
run_test "Error: Path Traversal Attack" test_error_path_traversal
run_test "Error: Relative Path" test_error_relative_path
run_test "Error: Delete Non-existent File" test_error_delete_nonexistent
run_test "HTTP Method Validation" test_http_method_validation
run_test "Concurrent Transfers" test_concurrent_transfers
run_test "Transfer Performance (50MB)" test_transfer_performance

# Cleanup will be called by trap
