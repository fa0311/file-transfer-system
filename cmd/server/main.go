package main

import (
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/fileserver/transfer/internal/config"
	grpcclient "github.com/fileserver/transfer/internal/grpc"
	grpcserver "github.com/fileserver/transfer/internal/grpc"
	httphandler "github.com/fileserver/transfer/internal/http"
	"github.com/fileserver/transfer/internal/progress"
)

func main() {
	// Set up JSON logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	slog.Info("Starting file transfer server")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		slog.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	slog.Info("Configuration loaded",
		"grpc_listen", cfg.GRPCListenAddr,
		"http_listen", cfg.HTTPListenAddr,
		"target_server", cfg.TargetServer,
		"allowed_dir", cfg.AllowedDir)

	// Create progress tracker
	tracker := progress.NewTracker()

	// Start gRPC server
	grpcSrv := grpcserver.NewServer(cfg, tracker)
	if err := grpcSrv.Start(); err != nil {
		slog.Error("Failed to start gRPC server", "error", err)
		os.Exit(1)
	}
	defer grpcSrv.Stop()

	// Give gRPC server time to start
	time.Sleep(1 * time.Second)

	// Create gRPC client for peer communication (connection will be established on-demand)
	grpcClient := grpcclient.NewClient(cfg, tracker)

	// Start HTTP server
	httpHandler := httphandler.NewHandler(cfg, grpcClient, tracker)
	if err := httpHandler.Start(); err != nil {
		slog.Error("Failed to start HTTP server", "error", err)
		os.Exit(1)
	}

	slog.Info("Server ready")

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	slog.Info("Shutting down server")
}
