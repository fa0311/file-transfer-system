.PHONY: proto build test-e2e test-all clean run-sender run-receiver help

# Default target
help:
	@echo "Available targets:"
	@echo "  proto       - Generate Go code from Protocol Buffers"
	@echo "  build       - Build the binary"
	@echo "  test-e2e    - Run end-to-end tests"
	@echo "  test-all    - Run all tests"
	@echo "  clean       - Clean build artifacts"
	@echo "  run-server-a - Run server A"
	@echo "  run-server-b - Run server B"

# Generate protobuf code
proto:
	@echo "Generating protobuf code..."
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		proto/transfer.proto

# Build the binary
build: proto
	@echo "Building binary..."
	go build -o bin/file-transfer-server ./server

# Run end-to-end tests
test-e2e: build
	@echo "Running E2E tests..."
	./tests/e2e.sh

# Run all tests
test-all: test-e2e

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -rf bin/
	rm -f proto/*.pb.go
	rm -f coverage.out coverage.html
	rm -rf /tmp/test-transfer-*

# Run server A (for development)
run-server-a: build
	@echo "Running server A..."
	PEER_SERVER_ADDR=localhost:50052 \
	ROOT_DIR=/tmp/transfer-server-a \
	HTTP_PORT=8080 \
	GRPC_PORT=50051 \
	./bin/file-transfer-server

# Run server B (for development)
run-server-b: build
	@echo "Running server B..."
	PEER_SERVER_ADDR=localhost:50051 \
	ROOT_DIR=/tmp/transfer-server-b \
	HTTP_PORT=8081 \
	GRPC_PORT=50052 \
	./bin/file-transfer-server

# Install dependencies
deps:
	@echo "Installing dependencies..."
	go mod download
	go mod tidy

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...

# Lint code
lint:
	@echo "Linting code..."
	golangci-lint run ./...

# Install development tools
install-tools:
	@echo "Installing development tools..."
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
