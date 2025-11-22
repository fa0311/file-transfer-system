package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	// Read environment variables
	peerAddr := os.Getenv("PEER_SERVER_ADDR")
	rootDir := os.Getenv("ROOT_DIR")
	httpPort := getEnv("HTTP_PORT", "8080")
	grpcPort := getEnv("GRPC_PORT", "50051")

	if peerAddr == "" {
		log.Fatal("PEER_SERVER_ADDR environment variable is required")
	}

	if rootDir == "" {
		log.Fatal("ROOT_DIR environment variable is required")
	}

	// Create root directory if it doesn't exist
	if err := os.MkdirAll(rootDir, 0755); err != nil {
		log.Fatalf("Failed to create root directory: %v", err)
	}

	// Setup signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		log.Printf("Received signal: %v, shutting down...", sig)
		cancel()
	}()

	log.Printf("Starting file transfer server")
	log.Printf("Configuration: httpPort=%s, grpcPort=%s, peerAddr=%s, rootDir=%s", httpPort, grpcPort, peerAddr, rootDir)

	// Start both servers concurrently
	errChan := make(chan error, 2)

	// Start gRPC server (for receiving files)
	go func() {
		if err := StartGRPCServer(ctx, grpcPort, rootDir); err != nil {
			errChan <- fmt.Errorf("gRPC server error: %v", err)
		}
	}()

	// Start HTTP server (for sending files)
	go func() {
		if err := StartHTTPServer(ctx, httpPort, peerAddr, rootDir); err != nil {
			errChan <- fmt.Errorf("HTTP server error: %v", err)
		}
	}()

	// Wait for error or context cancellation
	select {
	case err := <-errChan:
		log.Fatal(err)
	case <-ctx.Done():
		log.Println("Shutting down...")
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
