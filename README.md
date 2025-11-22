# File Transfer System

gRPC-based file transfer system with streaming support.

[![Tests](https://github.com/fa0311/file-transfer-system/actions/workflows/test.yml/badge.svg)](https://github.com/fa0311/file-transfer-system/actions/workflows/test.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/fa0311/file-transfer-system)](https://goreportcard.com/report/github.com/fa0311/file-transfer-system)

## Quick Start

```bash
# Build
make build

# Run receiver
PEER_SERVER_ADDR=localhost:8080 ROOT_DIR=/tmp/receiver GRPC_PORT=50051 ./bin/file-transfer-server

# Run sender (in another terminal)
PEER_SERVER_ADDR=localhost:50051 ROOT_DIR=/tmp/sender HTTP_PORT=8080 ./bin/file-transfer-server

# Transfer file
echo "test" > /tmp/sender/test.txt
curl -X POST http://localhost:8080/transfer \
  -H "Content-Type: application/json" \
  -d '{"source":"test.txt","target":"test.txt"}'
```

## Architecture

```
Client (curl) → HTTP Server (Sender) → gRPC Client → gRPC Server (Receiver)
                       ↓ NDJSON progress stream
                   Client
```

## Implementation Details

```go
// Transfer settings
ChunkSize:        8 * 1024 * 1024  // 8MB chunks
WindowSize:       1 << 30           // 1GB gRPC window
MessageSizeLimit: 16 * 1024 * 1024 // 16MB max message size
ProgressInterval: 1 second          // Progress update frequency
```

**Transfer mechanism:**

- Asynchronous streaming (no per-chunk acknowledgments)
- Single final acknowledgment after transfer completion
- NDJSON progress updates every second

## Configuration

| Variable           | Description                 | Default  |
| ------------------ | --------------------------- | -------- |
| `PEER_SERVER_ADDR` | Peer server address         | Required |
| `ROOT_DIR`         | Root directory for files    | Required |
| `HTTP_PORT`        | HTTP server port (sender)   | 8080     |
| `GRPC_PORT`        | gRPC server port (receiver) | 50051    |

## API

```bash
# Transfer file
POST /transfer
Content-Type: application/json
{"source": "path/to/file", "target": "path/to/file"}

# Health check
GET /health
```

## Development

```bash
make test           # Unit tests
make test-e2e       # E2E tests
make lint           # Lint check
```

## License

MIT
