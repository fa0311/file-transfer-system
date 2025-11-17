package main

import (
	"os"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	// Save original env vars
	originalGRPC := os.Getenv("GRPC_LISTEN_ADDR")
	originalHTTP := os.Getenv("HTTP_LISTEN_ADDR")
	originalTarget := os.Getenv("TARGET_SERVER")
	originalAllowed := os.Getenv("ALLOWED_DIR")

	// Restore env vars after test
	defer func() {
		_ = os.Setenv("GRPC_LISTEN_ADDR", originalGRPC)
		_ = os.Setenv("HTTP_LISTEN_ADDR", originalHTTP)
		_ = os.Setenv("TARGET_SERVER", originalTarget)
		_ = os.Setenv("ALLOWED_DIR", originalAllowed)
	}()

	t.Run("with all env vars set", func(t *testing.T) {
		tmpDir := t.TempDir()

		_ = os.Setenv("GRPC_LISTEN_ADDR", "0.0.0.0:50051")
		_ = os.Setenv("HTTP_LISTEN_ADDR", "0.0.0.0:8080")
		_ = os.Setenv("TARGET_SERVER", "localhost:50052")
		_ = os.Setenv("ALLOWED_DIR", tmpDir)

		config, err := LoadConfig()
		if err != nil {
			t.Fatalf("LoadConfig() error = %v", err)
		}

		if config.GRPCListenAddr != "0.0.0.0:50051" {
			t.Errorf("GRPCListenAddr = %v, want %v", config.GRPCListenAddr, "0.0.0.0:50051")
		}
		if config.HTTPListenAddr != "0.0.0.0:8080" {
			t.Errorf("HTTPListenAddr = %v, want %v", config.HTTPListenAddr, "0.0.0.0:8080")
		}
		if config.TargetServer != "localhost:50052" {
			t.Errorf("TargetServer = %v, want %v", config.TargetServer, "localhost:50052")
		}
	})

	t.Run("with default values", func(t *testing.T) {
		tmpDir := t.TempDir()

		_ = os.Unsetenv("GRPC_LISTEN_ADDR")
		_ = os.Unsetenv("HTTP_LISTEN_ADDR")
		_ = os.Setenv("TARGET_SERVER", "localhost:50052")
		_ = os.Setenv("ALLOWED_DIR", tmpDir)

		config, err := LoadConfig()
		if err != nil {
			t.Fatalf("LoadConfig() error = %v", err)
		}

		if config.GRPCListenAddr != "0.0.0.0:50051" {
			t.Errorf("GRPCListenAddr = %v, want default %v", config.GRPCListenAddr, "0.0.0.0:50051")
		}
		if config.HTTPListenAddr != "0.0.0.0:8080" {
			t.Errorf("HTTPListenAddr = %v, want default %v", config.HTTPListenAddr, "0.0.0.0:8080")
		}
	})

	t.Run("missing TARGET_SERVER", func(t *testing.T) {
		tmpDir := t.TempDir()

		_ = os.Unsetenv("TARGET_SERVER")
		_ = os.Setenv("ALLOWED_DIR", tmpDir)

		_, err := LoadConfig()
		if err == nil {
			t.Error("LoadConfig() expected error for missing TARGET_SERVER, got nil")
		}
	})

	t.Run("missing ALLOWED_DIR", func(t *testing.T) {
		_ = os.Setenv("TARGET_SERVER", "localhost:50052")
		_ = os.Unsetenv("ALLOWED_DIR")

		_, err := LoadConfig()
		if err == nil {
			t.Error("LoadConfig() expected error for missing ALLOWED_DIR, got nil")
		}
	})

	t.Run("non-existent ALLOWED_DIR", func(t *testing.T) {
		_ = os.Setenv("TARGET_SERVER", "localhost:50052")
		_ = os.Setenv("ALLOWED_DIR", "/nonexistent/path/that/does/not/exist")

		_, err := LoadConfig()
		if err == nil {
			t.Error("LoadConfig() expected error for non-existent ALLOWED_DIR, got nil")
		}
	})
}
