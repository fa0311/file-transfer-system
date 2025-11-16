# File Transfer System

A server-to-server file transfer system using Go and gRPC.

## Overview

This system provides the following features:

- Bidirectional file transfer between Server A and Server B
- Transfer initiation via HTTP from Client C
- Wildcard support for multiple file transfers
- Streaming transfer for large files (30GB+)
- Real-time progress reporting in JSONL format
- Data integrity verification with SHA256 checksums

## Architecture

```
Client C --[HTTP POST]--> Server A --[gRPC Stream]--> Server B
Client C --[HTTP POST]--> Server B --[gRPC Stream]--> Server A
```

## Required Environment Variables

### Server A

```bash
GRPC_LISTEN_ADDR=0.0.0.0:50051
HTTP_LISTEN_ADDR=0.0.0.0:8080
TARGET_SERVER=server-b:50051
ALLOWED_DIR=/data
```

### Server B

```bash
GRPC_LISTEN_ADDR=0.0.0.0:50051
HTTP_LISTEN_ADDR=0.0.0.0:8080
TARGET_SERVER=server-a:50051
ALLOWED_DIR=/data
```

## Build

### Local Build

```bash
# Install dependencies
go mod tidy

# Generate Protocol Buffers
protoc --go_out=. --go_opt=paths=source_relative \
       --go-grpc_out=. --go-grpc_opt=paths=source_relative \
       api/proto/transfer.proto

# Build
go build -o server ./cmd/server
```

### Docker Build

```bash
docker build -t file-transfer-server .
```

## Running

### Local Execution

#### Starting Server A

```bash
export GRPC_LISTEN_ADDR=0.0.0.0:50051
export HTTP_LISTEN_ADDR=0.0.0.0:8080
export TARGET_SERVER=localhost:50052
export ALLOWED_DIR=/data

./server
```

#### Starting Server B

```bash
export GRPC_LISTEN_ADDR=0.0.0.0:50052
export HTTP_LISTEN_ADDR=0.0.0.0:8081
export TARGET_SERVER=localhost:50051
export ALLOWED_DIR=/data

./server
```

### Docker Compose Execution

Create `docker-compose.yml`:

```yaml
version: "3.8"

services:
  server-a:
    image: ghcr.io/your-org/file-transfer-server:latest
    container_name: file-transfer-server-a
    environment:
      - GRPC_LISTEN_ADDR=0.0.0.0:50051
      - HTTP_LISTEN_ADDR=0.0.0.0:8080
      - TARGET_SERVER=server-b:50051
      - ALLOWED_DIR=/data
    volumes:
      - ./data-a:/data
    ports:
      - "8080:8080"
      - "50051:50051"
    networks:
      - transfer-network
    restart: unless-stopped

  server-b:
    image: ghcr.io/your-org/file-transfer-server:latest
    container_name: file-transfer-server-b
    environment:
      - GRPC_LISTEN_ADDR=0.0.0.0:50051
      - HTTP_LISTEN_ADDR=0.0.0.0:8080
      - TARGET_SERVER=server-a:50051
      - ALLOWED_DIR=/data
    volumes:
      - ./data-b:/data
    ports:
      - "8081:8080"
      - "50052:50051"
    networks:
      - transfer-network
    restart: unless-stopped

networks:
  transfer-network:
```

Start services:

```bash
# Start services
docker-compose up -d

# View logs
docker-compose logs -f

# Stop services
docker-compose down
```

## API Usage

### File Transfer Request

**Endpoint:** `POST /transfer`

**Path Prefix (Required):**

- `local:` - Refers to the local server (the server receiving the HTTP request)
- `peer:` - Refers to the peer server (the target server)

**Request Examples:**

```bash
# Transfer a single file from local to peer (explicit)
curl -X POST http://server-a:8080/transfer \
  -H "Content-Type: application/json" \
  -d '{
    "source_path": "local:/data/video.mp4",
    "dest_path": "peer:/data/received/"
  }'

# Transfer multiple files with wildcards
curl -X POST http://server-a:8080/transfer \
  -H "Content-Type: application/json" \
  -d '{
    "source_path": "local:/data/videos/*.mp4",
    "dest_path": "peer:/data/received/"
  }'
```

**Response:**

Returns real-time progress in JSONL (JSON Lines) format:

```
{"type":"info","message":"Transfer started","time":"2024-11-16T18:30:00Z"}
{"type":"progress","message":"Transferring...","time":"2024-11-16T18:30:01Z"}
{"type":"completed","message":"Transfer completed successfully","time":"2024-11-16T18:30:10Z"}
```

### File Deletion Request

**Endpoint:** `POST /delete` or `DELETE /delete`

**Path Prefix (Required):**

- `local:` - Delete file on the local server
- `peer:` - Delete file on the peer server

**Request Examples:**

```bash
# Delete a file on the local server
curl -X POST http://server-a:8080/delete \
  -H "Content-Type: application/json" \
  -d '{
    "file_path": "local:/data/old-file.mp4"
  }'

# Delete a file on the peer server (explicit)
curl -X POST http://server-a:8080/delete \
  -H "Content-Type: application/json" \
  -d '{
    "file_path": "peer:/data/old-file.mp4"
  }'

```

**Response:**

```json
{
  "success": true,
  "message": "File deleted successfully",
  "target": "local"
}
```

**Error Response:**

```json
{
  "success": false,
  "message": "Failed to delete file: file does not exist",
  "target": "peer"
}
```

### Health Check

**Endpoint:** `GET /health`

```bash
curl http://server-a:8080/health
```

**Response:**

```json
{
  "status": "healthy",
  "timestamp": "2024-11-16T18:30:00Z"
}
```

## Security Features

1. **Path Traversal Attack Prevention**

   - All file paths are restricted within `ALLOWED_DIR`
   - Rejects malicious paths containing `..`

2. **Checksum Verification**
   - Verifies SHA256 checksum for each chunk transfer
   - Ensures data integrity

## Technical Specifications

- **Chunk Size:** 1MB
- **Transfer Protocol:** gRPC bidirectional streaming
- **Progress Reporting:** JSONL (JSON Lines)
- **Retry:** Maximum 3 attempts (automatic retry)

## Troubleshooting

### Peer Connection Error

```
Failed to connect to peer after 10 attempts
```

**Solution:**

1. Verify `TARGET_SERVER` configuration
2. Check network connectivity
3. Ensure peer server is running

### Path Validation Error

```
path is outside allowed directory
```

**Solution:**

1. Verify `ALLOWED_DIR` is correctly configured
2. Ensure specified path is within `ALLOWED_DIR`

### Directory Permission Error

```
ALLOWED_DIR is not writable
```

**Solution:**

1. Check `ALLOWED_DIR` permissions: `ls -la /data`
2. Modify permissions if needed: `chmod 755 /data`

## Directory Structure

```
file-transfer-system/
├── cmd/
│   └── server/
│       └── main.go              # Entry point
├── internal/
│   ├── config/
│   │   └── config.go            # Configuration management
│   ├── grpc/
│   │   ├── server.go            # gRPC server
│   │   └── client.go            # gRPC client
│   ├── http/
│   │   └── handler.go           # HTTP handler
│   ├── transfer/
│   │   ├── sender.go            # File sending
│   │   ├── receiver.go          # File receiving
│   │   └── validator.go         # Path validation
│   └── progress/
│       └── tracker.go           # Progress tracking
├── api/
│   └── proto/
│       ├── transfer.proto       # Protocol Buffers definition
│       ├── transfer.pb.go       # Generated Go code
│       └── transfer_grpc.pb.go  # Generated gRPC code
├── .github/
│   └── workflows/
│       └── docker-build.yml     # GitHub Actions workflow
├── Dockerfile                   # Docker build configuration
├── docker-compose.yml           # Docker Compose configuration
├── go.mod
├── go.sum
└── README.md
```

## License

MIT License
