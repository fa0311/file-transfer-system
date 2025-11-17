#!/bin/bash

set -e

echo "Checking if protobuf files are up to date..."

# Generate proto files
protoc --go_out=. --go_opt=paths=source_relative \
       --go-grpc_out=. --go-grpc_opt=paths=source_relative \
       api/proto/transfer.proto

# Check if there are any changes
if ! git diff --exit-code api/proto/transfer.pb.go api/proto/transfer_grpc.pb.go; then
    echo "Error: Generated protobuf files are out of date!"
    echo "Please run 'make proto' or the following command:"
    echo "protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative api/proto/transfer.proto"
    exit 1
fi

echo "✓ Protobuf files are up to date"
