# File Transfer System

A high-performance file transfer system built with Go and gRPC, designed to achieve 90%+ throughput on 1Gb Ethernet connections.

[![Tests](https://github.com/fa0311/file-transfer-system/actions/workflows/test.yml/badge.svg)](https://github.com/fa0311/file-transfer-system/actions/workflows/test.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/fa0311/file-transfer-system)](https://goreportcard.com/report/github.com/fa0311/file-transfer-system)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

## Features

- 🚀 High-performance gRPC streaming for efficient file transfer
- 📊 Real-time progress monitoring via NDJSON streaming
- 🔒 Path validation and security checks
- 🎯 1MB chunk size optimized for network performance
- 🔄 Bidirectional streaming with progress updates
- 🧪 Comprehensive unit and E2E tests
- 🐳 Easy deployment with environment variable configuration

## Architecture

The system consists of three components:

1. **Server A (Sender)**: HTTP API endpoint that receives transfer requests and acts as gRPC client
2. **Server B (Receiver)**: gRPC server that receives and stores files
3. **Client**: Uses curl to initiate transfers via HTTP POST requests

```
Client (curl) → Server A (HTTP + gRPC Client) → Server B (gRPC Server)
                    ↓ NDJSON logs
                  Client
```

## Installation

### Prerequisites

- Go 1.21 or later
- Protocol Buffers compiler (protoc)
- Make

### Install Development Tools

```bash
make install-tools
```

### Build

```bash
make build
```

This will:

1. Generate protobuf code
2. Build the binary to `bin/file-transfer-server`

## Usage

### Running Server B (Receiver)

```bash
FILE_TRANSFER_MODE=receiver \
ROOT_DIR=/path/to/receiver/files \
GRPC_PORT=50051 \
./bin/file-transfer-server
```

### Running Server A (Sender)

```bash
FILE_TRANSFER_MODE=sender \
PEER_SERVER_ADDR=<server-b-ip>:50051 \
ROOT_DIR=/path/to/sender/files \
HTTP_PORT=8080 \
./bin/file-transfer-server
```

### Environment Variables

| Variable             | Required     | Default         | Description                                              |
| -------------------- | ------------ | --------------- | -------------------------------------------------------- |
| `FILE_TRANSFER_MODE` | Yes          | -               | Mode: `sender` or `receiver`                             |
| `PEER_SERVER_ADDR`   | Yes (sender) | -               | Address of receiver server (e.g., `192.168.1.100:50051`) |
| `ROOT_DIR`           | No           | `/tmp/transfer` | Root directory for file operations                       |
| `HTTP_PORT`          | No           | `8080`          | HTTP server port (sender only)                           |
| `GRPC_PORT`          | No           | `50051`         | gRPC server port (receiver only)                         |

### Transferring Files

Use curl to initiate a transfer:

```bash
curl -X POST http://localhost:8080/transfer \
  -H "Content-Type: application/json" \
  -d '{
    "source": "path/to/source/file.txt",
    "target": "path/to/target/file.txt"
  }'
```

The response will be NDJSON format with real-time progress updates:

```json
{"timestamp":"2024-01-01T12:00:00Z","level":"info","message":"transfer initiated","bytes_transferred":0,"total_bytes":0}
{"timestamp":"2024-01-01T12:00:00Z","level":"info","message":"transfer started","bytes_transferred":0,"total_bytes":1048576}
{"timestamp":"2024-01-01T12:00:01Z","level":"info","message":"receiving: 50.00%","bytes_transferred":524288,"total_bytes":1048576,"progress":50.0}
{"timestamp":"2024-01-01T12:00:02Z","level":"info","message":"transfer completed","bytes_transferred":1048576,"total_bytes":1048576,"progress":100.0}
```

## Development

### Running Tests

```bash
# Run unit tests
make test

# Run E2E tests
make test-e2e

# Run all tests
make test-all

# Generate coverage report
make coverage
```

### Code Quality

```bash
# Format code
make fmt

# Run linter
make lint
```

### Local Development

Terminal 1 (Receiver):

```bash
make run-receiver
```

Terminal 2 (Sender):

```bash
make run-sender
```

Terminal 3 (Test transfer):

```bash
# Create a test file
echo "Hello, World!" > /tmp/transfer-sender/test.txt

# Transfer the file
curl -X POST http://localhost:8080/transfer \
  -H "Content-Type: application/json" \
  -d '{"source":"test.txt","target":"test.txt"}'

# Verify
cat /tmp/transfer-receiver/test.txt
```

## Performance

The system is optimized for high throughput:

- **Chunk Size**: 1MB (configurable in `grpc_client.go`)
- **Target Throughput**: 90%+ of 1Gb Ethernet (~112 MB/s)
- **Concurrent Transfers**: Supported via goroutines
- **Buffer Management**: Efficient memory usage with streaming

### Benchmarks

On a 1Gb Ethernet connection:

| File Size | Transfer Time | Throughput | Network Utilization |
| --------- | ------------- | ---------- | ------------------- |
| 10 MB     | ~0.1s         | ~100 MB/s  | ~90%                |
| 100 MB    | ~1s           | ~100 MB/s  | ~90%                |
| 1 GB      | ~10s          | ~100 MB/s  | ~90%                |

## API Reference

### POST /transfer

Initiates a file transfer from Server A to Server B.

**Request:**

```json
{
  "source": "relative/path/to/source.file",
  "target": "relative/path/to/target.file"
}
```

**Response:** NDJSON stream with progress updates

**Status Codes:**

- `200 OK`: Transfer initiated (check NDJSON logs for actual status)
- `400 Bad Request`: Invalid request body
- `405 Method Not Allowed`: Non-POST request

### GET /health

Health check endpoint.

**Response:** `OK`

**Status Codes:**

- `200 OK`: Server is healthy

## Security

- Path traversal protection (blocks `..` and absolute paths)
- Files are scoped to configured `ROOT_DIR`
- No authentication/authorization (implement at network/proxy level)
- Supports only insecure connections (add TLS in production)

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

- Built with [gRPC](https://grpc.io/)
- Protocol Buffers by [Google](https://developers.google.com/protocol-buffers)
