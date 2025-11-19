# Build stage
FROM golang:1.24-alpine AS builder

# Install required packages
RUN apk add --no-cache git make protobuf protobuf-dev

# Set working directory
WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Install protoc plugins
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@latest && \
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Copy source code
COPY . .

# Generate protobuf code and build
RUN export PATH=$PATH:$(go env GOPATH)/bin && \
    make proto && \
    CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o file-transfer-server ./server

# Runtime stage
FROM alpine:latest

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/file-transfer-server .

# Create default directories
RUN mkdir -p /data

# Expose ports
EXPOSE 8080 50051

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD ["/bin/sh", "-c", "test -n \"$FILE_TRANSFER_MODE\""]

# Run the binary
ENTRYPOINT ["/app/file-transfer-server"]
