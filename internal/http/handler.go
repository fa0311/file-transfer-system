package http

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/fileserver/transfer/internal/config"
	grpcclient "github.com/fileserver/transfer/internal/grpc"
	"github.com/fileserver/transfer/internal/progress"
)

// TransferRequest represents the HTTP request for file transfer
type TransferRequest struct {
	SourcePath string `json:"source_path"`
	DestPath   string `json:"dest_path"`
}

// Handler handles HTTP requests
type Handler struct {
	config  *config.Config
	client  *grpcclient.Client
	tracker *progress.Tracker
}

// NewHandler creates a new HTTP handler
func NewHandler(cfg *config.Config, client *grpcclient.Client, tracker *progress.Tracker) *Handler {
	return &Handler{
		config:  cfg,
		client:  client,
		tracker: tracker,
	}
}

// Start starts the HTTP server
func (h *Handler) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/transfer", h.handleTransfer)
	mux.HandleFunc("/health", h.handleHealth)

	server := &http.Server{
		Addr:    h.config.HTTPListenAddr,
		Handler: mux,
	}

	log.Printf("Starting HTTP server on %s", h.config.HTTPListenAddr)

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	return nil
}

// handleTransfer handles file transfer requests
func (h *Handler) handleTransfer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request
	var req TransferRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}

	if req.SourcePath == "" {
		http.Error(w, "source_path is required", http.StatusBadRequest)
		return
	}

	log.Printf("Transfer request received: source=%s, dest=%s", req.SourcePath, req.DestPath)

	// Set headers for JSONL (JSON Lines) response
	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Flush headers
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}

	// Create a context that can be canceled
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// Send initial status
	h.sendEvent(w, "info", "Transfer started")

	// Perform transfer in a goroutine
	errChan := make(chan error, 1)
	go func() {
		errChan <- h.client.TransferFiles(req.SourcePath, req.DestPath)
	}()

	// Stream progress updates
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			h.sendEvent(w, "error", "Client disconnected")
			return

		case err := <-errChan:
			if err != nil {
				h.sendEvent(w, "error", err.Error())
				log.Printf("Transfer failed: %v", err)
			} else {
				h.sendEvent(w, "completed", "Transfer completed successfully")
				log.Println("Transfer completed successfully")
			}
			return

		case <-ticker.C:
			// Send progress update
			// Note: In a real implementation, you'd track progress per request
			h.sendEvent(w, "progress", "Transferring...")
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
	}
}

// handleHealth handles health check requests
func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// sendEvent sends a JSONL (JSON Lines) event
func (h *Handler) sendEvent(w http.ResponseWriter, eventType, message string) {
	event := map[string]string{
		"type":    eventType,
		"message": message,
		"time":    time.Now().Format(time.RFC3339),
	}

	data, _ := json.Marshal(event)
	fmt.Fprintf(w, "%s\n", data)

	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}
