package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	pb "github.com/fa0311/file-transfer-system/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type FileTransferServer struct {
	pb.UnimplementedFileTransferServer
	rootDir string
}

func NewFileTransferServer(rootDir string) *FileTransferServer {
	return &FileTransferServer{
		rootDir: rootDir,
	}
}

func (s *FileTransferServer) Transfer(stream pb.FileTransfer_TransferServer) error {
	// Step 1: Receive metadata
	req, err := stream.Recv()
	if err != nil {
		return status.Errorf(codes.Internal, "failed to receive metadata: %v", err)
	}

	metadata, ok := req.Payload.(*pb.TransferRequest_Metadata)
	if !ok {
		return status.Errorf(codes.InvalidArgument, "expected metadata as first message")
	}

	// Validate path
	cleanPath := filepath.Clean(metadata.Metadata.FilePath)
	if strings.HasPrefix(cleanPath, "..") || filepath.IsAbs(cleanPath) {
		return status.Errorf(codes.InvalidArgument, "invalid file path: %s", metadata.Metadata.FilePath)
	}

	targetPath := filepath.Join(s.rootDir, cleanPath)
	expectedSize := metadata.Metadata.FileSize

	// Create directory
	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return status.Errorf(codes.Internal, "failed to create directory: %v", err)
	}

	// Create file
	file, err := os.Create(targetPath)
	if err != nil {
		return status.Errorf(codes.Internal, "failed to create file: %v", err)
	}
	defer file.Close()

	// Send progress
	if err := stream.Send(&pb.TransferResponse{
		Success:       true,
		Message:       "file created",
		BytesReceived: 0,
	}); err != nil {
		return err
	}

	// Step 2: Receive chunks
	bytesReceived := int64(0)
	for {
		req, err := stream.Recv()
		if err != nil {
			return status.Errorf(codes.Internal, "failed to receive chunk: %v", err)
		}

		// Check if we received a chunk or complete message
		if chunk, ok := req.Payload.(*pb.TransferRequest_Chunk); ok {
			// Write chunk data
			n, err := file.Write(chunk.Chunk.Data)
			if err != nil {
				return status.Errorf(codes.Internal, "failed to write to file: %v", err)
			}

			bytesReceived += int64(n)

			// Send progress
			if err := stream.Send(&pb.TransferResponse{
				Success:       true,
				Message:       fmt.Sprintf("receiving: %.2f%%", float64(bytesReceived)/float64(expectedSize)*100),
				BytesReceived: bytesReceived,
			}); err != nil {
				return err
			}
		} else if complete, ok := req.Payload.(*pb.TransferRequest_Complete); ok {
			// Step 3: Verify completion
			if bytesReceived != complete.Complete.BytesTransferred {
				return status.Errorf(codes.DataLoss, "byte count mismatch: expected=%d, actual=%d", complete.Complete.BytesTransferred, bytesReceived)
			}

			// Sync file
			if err := file.Sync(); err != nil {
				return status.Errorf(codes.Internal, "failed to sync file: %v", err)
			}

			// Send final success response
			if err := stream.Send(&pb.TransferResponse{
				Success:       true,
				Message:       "transfer completed",
				BytesReceived: bytesReceived,
			}); err != nil {
				return err
			}

			return nil
		} else {
			return status.Errorf(codes.InvalidArgument, "unexpected message type")
		}
	}
}

func StartGRPCServer(ctx context.Context, port, rootDir string) error {
	lis, err := NewListener(port)
	if err != nil {
		return fmt.Errorf("failed to create listener: %v", err)
	}

	grpcServer := grpc.NewServer(
		grpc.MaxRecvMsgSize(10 * 1024 * 1024), // 10MB
		grpc.MaxSendMsgSize(10 * 1024 * 1024), // 10MB
	)

	pb.RegisterFileTransferServer(grpcServer, NewFileTransferServer(rootDir))

	go func() {
		<-ctx.Done()
		grpcServer.GracefulStop()
	}()

	fmt.Printf("Starting gRPC server: port=%s, rootDir=%s\n", port, rootDir)
	return grpcServer.Serve(lis)
}
