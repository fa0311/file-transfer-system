package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	pb "github.com/fa0311/file-transfer-system/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	ChunkSize        = 8 * 1024 * 1024 // 8MB chunks for optimal network performance
	ProgressInterval = time.Second      // Progress update interval
)

type TransferProgress struct {
	BytesTransferred int64
	TotalBytes       int64
	Message          string
	Timestamp        time.Time
}

func TransferFile(ctx context.Context, peerAddr, sourcePath, targetPath, rootDir string, progressChan chan<- TransferProgress) error {
	// Validate source path
	cleanSourcePath := filepath.Clean(sourcePath)
	if strings.HasPrefix(cleanSourcePath, "..") || filepath.IsAbs(cleanSourcePath) {
		return fmt.Errorf("invalid source path: %s", sourcePath)
	}

	fullSourcePath := filepath.Join(rootDir, cleanSourcePath)

	// Check if file exists
	fileInfo, err := os.Stat(fullSourcePath)
	if err != nil {
		return fmt.Errorf("failed to stat source file: %v", err)
	}

	if fileInfo.IsDir() {
		return fmt.Errorf("source path is a directory, not a file")
	}

	fileSize := fileInfo.Size()

	// Connect to peer server
	conn, err := grpc.Dial(peerAddr, 
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(16 * 1024 * 1024),
			grpc.MaxCallSendMsgSize(16 * 1024 * 1024),
		),
		grpc.WithInitialWindowSize(1 << 30),     // 1GB initial window
		grpc.WithInitialConnWindowSize(1 << 30), // 1GB connection window
		grpc.WithWriteBufferSize(1 << 20),       // 1MB write buffer
		grpc.WithReadBufferSize(1 << 20),        // 1MB read buffer
	)
	if err != nil {
		return fmt.Errorf("failed to connect to peer server: %v", err)
	}
	defer conn.Close()

	client := pb.NewFileTransferClient(conn)
	stream, err := client.Transfer(ctx)
	if err != nil {
		return fmt.Errorf("failed to create transfer stream: %v", err)
	}

	// Step 1: Send metadata
	if err := stream.Send(&pb.TransferRequest{
		Payload: &pb.TransferRequest_Metadata{
			Metadata: &pb.TransferMetadata{
				FilePath: targetPath,
				FileSize: fileSize,
			},
		},
	}); err != nil {
		return fmt.Errorf("failed to send metadata: %v", err)
	}

	progressChan <- TransferProgress{
		BytesTransferred: 0,
		TotalBytes:       fileSize,
		Message:          "transfer started",
		Timestamp:        time.Now(),
	}

	// Step 2: Open and send file chunks
	file, err := os.Open(fullSourcePath)
	if err != nil {
		return fmt.Errorf("failed to open source file: %v", err)
	}
	defer file.Close()

	buffer := make([]byte, ChunkSize)
	bytesTransferred := int64(0)
	lastProgressTime := time.Now()

	for {
		n, err := file.Read(buffer)
		if err != nil && err != io.EOF {
			return fmt.Errorf("failed to read file: %v", err)
		}
		if n == 0 {
			break
		}

		// Send chunk without waiting for response
		if err := stream.Send(&pb.TransferRequest{
			Payload: &pb.TransferRequest_Chunk{
				Chunk: &pb.FileChunk{
					Data: buffer[:n],
				},
			},
		}); err != nil {
			return fmt.Errorf("failed to send chunk: %v", err)
		}

		bytesTransferred += int64(n)

		// Send local progress update
		if time.Since(lastProgressTime) >= ProgressInterval {
			progressChan <- TransferProgress{
				BytesTransferred: bytesTransferred,
				TotalBytes:       fileSize,
				Message:          fmt.Sprintf("sending: %.2f%%", float64(bytesTransferred)/float64(fileSize)*100),
				Timestamp:        time.Now(),
			}
			lastProgressTime = time.Now()
		}
	}

	// Step 3: Send completion message
	if err := stream.Send(&pb.TransferRequest{
		Payload: &pb.TransferRequest_Complete{
			Complete: &pb.TransferComplete{
				BytesTransferred: bytesTransferred,
			},
		},
	}); err != nil {
		return fmt.Errorf("failed to send completion: %v", err)
	}

	// Close send side
	if err := stream.CloseSend(); err != nil {
		return fmt.Errorf("failed to close send stream: %v", err)
	}

	// Wait for final response from server
	resp, err := stream.Recv()
	if err != nil {
		return fmt.Errorf("failed to receive final response: %v", err)
	}
	if !resp.Success {
		return fmt.Errorf("transfer failed: %s", resp.Message)
	}

	progressChan <- TransferProgress{
		BytesTransferred: bytesTransferred,
		TotalBytes:       fileSize,
		Message:          "transfer completed",
		Timestamp:        time.Now(),
	}

	return nil
}
