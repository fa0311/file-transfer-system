package main

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"

	pb "github.com/fa0311/file-transfer-system/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

const bufSize = 1024 * 1024

func setupTestServer(t *testing.T) (*grpc.Server, *bufconn.Listener, string) {
	lis := bufconn.Listen(bufSize)
	
	tmpDir, err := os.MkdirTemp("", "test-transfer-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	server := grpc.NewServer()
	pb.RegisterFileTransferServer(server, NewFileTransferServer(tmpDir))

	go func() {
		if err := server.Serve(lis); err != nil {
			t.Logf("Server exited with error: %v", err)
		}
	}()

	return server, lis, tmpDir
}

func createTestClient(ctx context.Context, lis *bufconn.Listener) (pb.FileTransferClient, *grpc.ClientConn, error) {
	conn, err := grpc.DialContext(ctx, "bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, nil, err
	}

	client := pb.NewFileTransferClient(conn)
	return client, conn, nil
}

func TestFileTransferServer_Transfer_Success(t *testing.T) {
	server, lis, tmpDir := setupTestServer(t)
	defer server.Stop()
	defer os.RemoveAll(tmpDir)

	ctx := context.Background()
	client, conn, err := createTestClient(ctx, lis)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer conn.Close()

	stream, err := client.Transfer(ctx)
	if err != nil {
		t.Fatalf("Failed to create stream: %v", err)
	}

	// Send metadata
	err = stream.Send(&pb.TransferRequest{
		Payload: &pb.TransferRequest_Metadata{
			Metadata: &pb.TransferMetadata{
				FilePath: "test.txt",
				FileSize: 13,
			},
		},
	})
	if err != nil {
		t.Fatalf("Failed to send metadata: %v", err)
	}

	// Receive metadata response
	resp, err := stream.Recv()
	if err != nil {
		t.Fatalf("Failed to receive metadata response: %v", err)
	}
	if !resp.Success {
		t.Fatalf("Metadata response unsuccessful: %s", resp.Message)
	}

	// Send chunk
	err = stream.Send(&pb.TransferRequest{
		Payload: &pb.TransferRequest_Chunk{
			Chunk: &pb.FileChunk{
				Data: []byte("Hello, World!"),
			},
		},
	})
	if err != nil {
		t.Fatalf("Failed to send chunk: %v", err)
	}

	// Receive chunk response
	resp, err = stream.Recv()
	if err != nil {
		t.Fatalf("Failed to receive chunk response: %v", err)
	}
	if !resp.Success {
		t.Fatalf("Chunk response unsuccessful: %s", resp.Message)
	}

	// Send completion
	err = stream.Send(&pb.TransferRequest{
		Payload: &pb.TransferRequest_Complete{
			Complete: &pb.TransferComplete{
				BytesTransferred: 13,
			},
		},
	})
	if err != nil {
		t.Fatalf("Failed to send completion: %v", err)
	}

	// Receive final response
	resp, err = stream.Recv()
	if err != nil {
		t.Fatalf("Failed to receive final response: %v", err)
	}
	if !resp.Success {
		t.Fatalf("Final response unsuccessful: %s", resp.Message)
	}

	stream.CloseSend()

	// Verify file was created
	targetPath := filepath.Join(tmpDir, "test.txt")
	content, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("Failed to read transferred file: %v", err)
	}

	if string(content) != "Hello, World!" {
		t.Fatalf("File content mismatch: got %s, want Hello, World!", string(content))
	}
}

func TestFileTransferServer_Transfer_InvalidPath(t *testing.T) {
	server, lis, tmpDir := setupTestServer(t)
	defer server.Stop()
	defer os.RemoveAll(tmpDir)

	ctx := context.Background()
	client, conn, err := createTestClient(ctx, lis)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer conn.Close()

	stream, err := client.Transfer(ctx)
	if err != nil {
		t.Fatalf("Failed to create stream: %v", err)
	}

	// Send metadata with invalid path (containing ..)
	err = stream.Send(&pb.TransferRequest{
		Payload: &pb.TransferRequest_Metadata{
			Metadata: &pb.TransferMetadata{
				FilePath: "../../../etc/passwd",
				FileSize: 10,
			},
		},
	})
	if err != nil {
		t.Fatalf("Failed to send metadata: %v", err)
	}

	// Should receive error
	_, err = stream.Recv()
	if err == nil {
		t.Fatal("Expected error for invalid path, got nil")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatal("Expected gRPC status error")
	}

	if st.Code() != codes.InvalidArgument {
		t.Fatalf("Expected InvalidArgument error, got %v", st.Code())
	}
}

func TestFileTransferServer_Transfer_MissingMetadata(t *testing.T) {
	server, lis, tmpDir := setupTestServer(t)
	defer server.Stop()
	defer os.RemoveAll(tmpDir)

	ctx := context.Background()
	client, conn, err := createTestClient(ctx, lis)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer conn.Close()

	stream, err := client.Transfer(ctx)
	if err != nil {
		t.Fatalf("Failed to create stream: %v", err)
	}

	// Send chunk without metadata first
	err = stream.Send(&pb.TransferRequest{
		Payload: &pb.TransferRequest_Chunk{
			Chunk: &pb.FileChunk{
				Data: []byte("Hello"),
			},
		},
	})
	if err != nil {
		t.Fatalf("Failed to send chunk: %v", err)
	}

	// Should receive error
	_, err = stream.Recv()
	if err == nil {
		t.Fatal("Expected error for missing metadata, got nil")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatal("Expected gRPC status error")
	}

	if st.Code() != codes.InvalidArgument {
		t.Fatalf("Expected InvalidArgument error, got %v", st.Code())
	}
}

func TestFileTransferServer_Transfer_ByteMismatch(t *testing.T) {
	server, lis, tmpDir := setupTestServer(t)
	defer server.Stop()
	defer os.RemoveAll(tmpDir)

	ctx := context.Background()
	client, conn, err := createTestClient(ctx, lis)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer conn.Close()

	stream, err := client.Transfer(ctx)
	if err != nil {
		t.Fatalf("Failed to create stream: %v", err)
	}

	// Send metadata
	err = stream.Send(&pb.TransferRequest{
		Payload: &pb.TransferRequest_Metadata{
			Metadata: &pb.TransferMetadata{
				FilePath: "test.txt",
				FileSize: 10,
			},
		},
	})
	if err != nil {
		t.Fatalf("Failed to send metadata: %v", err)
	}

	stream.Recv() // Receive metadata response

	// Send chunk
	err = stream.Send(&pb.TransferRequest{
		Payload: &pb.TransferRequest_Chunk{
			Chunk: &pb.FileChunk{
				Data: []byte("Hello"),
			},
		},
	})
	if err != nil {
		t.Fatalf("Failed to send chunk: %v", err)
	}

	stream.Recv() // Receive chunk response

	// Send completion with wrong byte count
	err = stream.Send(&pb.TransferRequest{
		Payload: &pb.TransferRequest_Complete{
			Complete: &pb.TransferComplete{
				BytesTransferred: 10, // Wrong, should be 5
			},
		},
	})
	if err != nil {
		t.Fatalf("Failed to send completion: %v", err)
	}

	// Should receive error
	_, err = stream.Recv()
	if err == nil {
		t.Fatal("Expected error for byte mismatch, got nil")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatal("Expected gRPC status error")
	}

	if st.Code() != codes.DataLoss {
		t.Fatalf("Expected DataLoss error, got %v", st.Code())
	}
}
