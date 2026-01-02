package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type TransferRequest struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

type LogEntry struct {
	Timestamp        string  `json:"timestamp"`
	Level            string  `json:"level"`
	Message          string  `json:"message"`
	BytesTransferred int64   `json:"bytes_transferred"`
	TotalBytes       int64   `json:"total_bytes"`
	Progress         float64 `json:"progress,omitempty"`
	Error            string  `json:"error,omitempty"`
}

func handleTransfer(peerAddr, rootDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request
	var req TransferRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("invalid request: %v", err), http.StatusBadRequest)
		return
	}

	// Set headers for NDJSON streaming
	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("Transfer-Encoding", "chunked")
	w.Header().Set("X-Content-Type-Options", "nosniff")

	// Flush headers immediately
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}

	// Create progress channel
	progressChan := make(chan TransferProgress, 100)
	errChan := make(chan error, 1)

	// Start transfer in goroutine
	ctx := r.Context()
	go func() {
		err := TransferFile(ctx, peerAddr, req.Source, req.Target, rootDir, progressChan)
		if err != nil {
			errChan <- err
		}
		close(progressChan)
		close(errChan)
	}()

	encoder := json.NewEncoder(w)
	flusher, _ := w.(http.Flusher)

	// Send initial log
	logEntry := LogEntry{
		Timestamp:        time.Now().Format(time.RFC3339),
		Level:            "info",
		Message:          "transfer initiated",
		BytesTransferred: 0,
		TotalBytes:       0,
	}
	if err := encoder.Encode(logEntry); err != nil {
		return
	}
	flusher.Flush()

	// Stream progress updates
	for {
		select {
		case progress, ok := <-progressChan:
			if !ok {
				// Channel closed, check for errors
				if err := <-errChan; err != nil {
					logEntry := LogEntry{
						Timestamp:        time.Now().Format(time.RFC3339),
						Level:            "error",
						Message:          "transfer failed",
						BytesTransferred: 0,
						TotalBytes:       0,
						Error:            err.Error(),
					}
					_ = encoder.Encode(logEntry)
					flusher.Flush()
					
					// Force close TCP connection to signal error to curl
					if hijacker, ok := w.(http.Hijacker); ok {
						conn, _, _ := hijacker.Hijack()
						conn.Close()
					}
				}
				return
			}

			var progressPercent float64
			if progress.TotalBytes > 0 {
				progressPercent = float64(progress.BytesTransferred) / float64(progress.TotalBytes) * 100
			}

			logEntry := LogEntry{
				Timestamp:        progress.Timestamp.Format(time.RFC3339),
				Level:            "info",
				Message:          progress.Message,
				BytesTransferred: progress.BytesTransferred,
				TotalBytes:       progress.TotalBytes,
				Progress:         progressPercent,
			}
			if err := encoder.Encode(logEntry); err != nil {
				return
			}
			flusher.Flush()

		case <-ctx.Done():
			logEntry := LogEntry{
				Timestamp: time.Now().Format(time.RFC3339),
				Level:     "error",
				Message:   "transfer cancelled",
				Error:     ctx.Err().Error(),
			}
			_ = encoder.Encode(logEntry)
			flusher.Flush()
			
			// Force close TCP connection to signal error to curl
			if hijacker, ok := w.(http.Hijacker); ok {
				conn, _, _ := hijacker.Hijack()
				conn.Close()
			}
			return
		}
	}
	}
}

func StartHTTPServer(ctx context.Context, port, peerAddr, rootDir string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/transfer", handleTransfer(peerAddr, rootDir))
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	httpServer := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}()

	fmt.Printf("Starting HTTP server: port=%s, peerAddr=%s, rootDir=%s\n", port, peerAddr, rootDir)
	return httpServer.ListenAndServe()
}
