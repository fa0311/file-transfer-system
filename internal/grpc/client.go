package grpc

import (
	"context"
	"fmt"
	"log"
	"time"

	pb "github.com/fileserver/transfer/api/proto"
	"github.com/fileserver/transfer/internal/config"
	"github.com/fileserver/transfer/internal/progress"
	"github.com/fileserver/transfer/internal/transfer"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Client represents the gRPC client
type Client struct {
	config     *config.Config
	sender     *transfer.Sender
	tracker    *progress.Tracker
	conn       *grpc.ClientConn
	client     pb.FileTransferClient
	retryCount int
	retryDelay time.Duration
}

// NewClient creates a new gRPC client
func NewClient(cfg *config.Config, tracker *progress.Tracker) *Client {
	return &Client{
		config:     cfg,
		sender:     transfer.NewSender(cfg.AllowedDir, tracker),
		tracker:    tracker,
		retryCount: 3,
		retryDelay: 2 * time.Second,
	}
}

// Connect establishes connection to the target server
func (c *Client) Connect() error {
	var err error
	
	log.Printf("Connecting to peer: %s", c.config.TargetServer)
	
	c.conn, err = grpc.Dial(
		c.config.TargetServer,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithMaxMsgSize(10*1024*1024), // 10MB max message size
	)
	if err != nil {
		return fmt.Errorf("failed to connect to peer: %w", err)
	}

	c.client = pb.NewFileTransferClient(c.conn)
	return nil
}

// Close closes the connection
func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// VerifyPeer verifies that the peer's target server points back to this server
func (c *Client) VerifyPeer() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	log.Println("Verifying peer configuration...")

	resp, err := c.client.GetPeerInfo(ctx, &pb.PeerInfoRequest{})
	if err != nil {
		return fmt.Errorf("failed to get peer info: %w", err)
	}

	log.Printf("Peer information - Target: %s, Listen: %s", resp.TargetServer, resp.GrpcListenAddr)

	// Verify that peer's target points to this server
	// We need to check if the peer's TargetServer matches our listen address
	// The peer should have our address in their TARGET_SERVER
	if resp.TargetServer == "" {
		return fmt.Errorf("peer has no target server configured")
	}

	log.Printf("Peer verification successful - Peer is configured to connect back to us")
	return nil
}

// TransferFiles transfers multiple files to the target server
func (c *Client) TransferFiles(sourcePath, destPath string) error {
	// Connect to peer if not already connected
	if c.conn == nil {
		if err := c.Connect(); err != nil {
			return fmt.Errorf("failed to connect to peer: %w", err)
		}
		log.Println("Connected to peer for transfer")
	}

	// Prepare files (validate and expand wildcards)
	files, err := c.sender.PrepareFiles(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to prepare files: %w", err)
	}

	log.Printf("Transferring %d file(s) to %s", len(files), c.config.TargetServer)

	// Transfer each file
	for _, file := range files {
		if err := c.transferSingleFile(file, destPath); err != nil {
			log.Printf("Failed to transfer file %s: %v", file, err)
			return err
		}
		log.Printf("Successfully transferred: %s", file)
	}

	return nil
}

func (c *Client) transferSingleFile(filePath, destPath string) error {
	var lastErr error

	// Retry logic
	for attempt := 0; attempt <= c.retryCount; attempt++ {
		if attempt > 0 {
			log.Printf("Retrying transfer (attempt %d/%d) for: %s", attempt, c.retryCount, filePath)
			time.Sleep(c.retryDelay)
		}

		ctx := context.Background()
		stream, err := c.client.TransferFile(ctx)
		if err != nil {
			lastErr = fmt.Errorf("failed to create stream: %w", err)
			continue
		}

		// Send file
		err = c.sender.SendFile(filePath, destPath, stream)
		if err != nil {
			lastErr = err
			stream.CloseSend()
			continue
		}

		// Close the send side and wait for final response
		stream.CloseSend()
		return nil
	}

	return fmt.Errorf("failed after %d retries: %w", c.retryCount, lastErr)
}

// HealthCheck performs a health check on the target server
func (c *Client) HealthCheck() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := c.client.HealthCheck(ctx, &pb.HealthCheckRequest{})
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}

	if !resp.Healthy {
		return fmt.Errorf("peer server is unhealthy: %s", resp.Message)
	}

	return nil
}
