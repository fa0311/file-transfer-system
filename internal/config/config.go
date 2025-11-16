package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// Config holds the application configuration
type Config struct {
	GRPCListenAddr  string
	HTTPListenAddr  string
	TargetServer    string
	AllowedDir      string
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	cfg := &Config{
		GRPCListenAddr: getEnv("GRPC_LISTEN_ADDR", "0.0.0.0:50051"),
		HTTPListenAddr: getEnv("HTTP_LISTEN_ADDR", "0.0.0.0:8080"),
		TargetServer:   getEnv("TARGET_SERVER", ""),
		AllowedDir:     getEnv("ALLOWED_DIR", ""),
	}

	// Validate required fields
	if cfg.TargetServer == "" {
		return nil, fmt.Errorf("TARGET_SERVER environment variable is required")
	}
	if cfg.AllowedDir == "" {
		return nil, fmt.Errorf("ALLOWED_DIR environment variable is required")
	}

	// Validate allowed directory exists and is accessible
	absPath, err := filepath.Abs(cfg.AllowedDir)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve ALLOWED_DIR path: %w", err)
	}
	cfg.AllowedDir = absPath

	info, err := os.Stat(cfg.AllowedDir)
	if err != nil {
		return nil, fmt.Errorf("ALLOWED_DIR does not exist or is not accessible: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("ALLOWED_DIR is not a directory: %s", cfg.AllowedDir)
	}

	// Check write permissions by attempting to create a test file
	testFile := filepath.Join(cfg.AllowedDir, ".write_test")
	f, err := os.Create(testFile)
	if err != nil {
		return nil, fmt.Errorf("ALLOWED_DIR is not writable: %w", err)
	}
	f.Close()
	os.Remove(testFile)

	return cfg, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
