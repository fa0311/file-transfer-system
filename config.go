package main

import (
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	GRPCListenAddr string
	HTTPListenAddr string
	TargetServer   string
	AllowedDir     string
}

func LoadConfig() (*Config, error) {
	grpcAddr := os.Getenv("GRPC_LISTEN_ADDR")
	if grpcAddr == "" {
		grpcAddr = "0.0.0.0:50051"
	}

	httpAddr := os.Getenv("HTTP_LISTEN_ADDR")
	if httpAddr == "" {
		httpAddr = "0.0.0.0:8080"
	}

	targetServer := os.Getenv("TARGET_SERVER")
	if targetServer == "" {
		return nil, fmt.Errorf("TARGET_SERVER environment variable is required")
	}

	allowedDir := os.Getenv("ALLOWED_DIR")
	if allowedDir == "" {
		return nil, fmt.Errorf("ALLOWED_DIR environment variable is required")
	}

	// Clean and resolve the allowed directory path
	allowedDir, err := filepath.Abs(allowedDir)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve ALLOWED_DIR: %w", err)
	}

	// Check if directory exists and is writable
	info, err := os.Stat(allowedDir)
	if err != nil {
		return nil, fmt.Errorf("ALLOWED_DIR does not exist: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("ALLOWED_DIR is not a directory")
	}

	// Test write permission
	testFile := filepath.Join(allowedDir, ".write_test")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		return nil, fmt.Errorf("ALLOWED_DIR is not writable: %w", err)
	}
	os.Remove(testFile)

	return &Config{
		GRPCListenAddr: grpcAddr,
		HTTPListenAddr: httpAddr,
		TargetServer:   targetServer,
		AllowedDir:     allowedDir,
	}, nil
}
