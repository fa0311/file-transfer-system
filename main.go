package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	pb "github.com/fa0311/file-transfer-system/api/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const ChunkSize = 1024 * 1024 // 1MB

type Server struct {
	pb.UnimplementedFileTransferServer
	config    *Config
	validator *PathValidator
}

type ProgressMessage struct {
	Type    string `json:"type"`
	Message string `json:"message"`
	Time    string `json:"time"`
}

func main() {
	config, err := LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	validator := NewPathValidator(config.AllowedDir)
	server := &Server{
		config:    config,
		validator: validator,
	}

	// Start gRPC server
	go func() {
		if err := startGRPCServer(server, config.GRPCListenAddr); err != nil {
			log.Fatalf("Failed to start gRPC server: %v", err)
		}
	}()

	// Start HTTP server
	if err := startHTTPServer(server, config.HTTPListenAddr); err != nil {
		log.Fatalf("Failed to start HTTP server: %v", err)
	}
}

func startGRPCServer(server *Server, addr string) error {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterFileTransferServer(grpcServer, server)

	log.Printf("gRPC server listening on %s", addr)
	return grpcServer.Serve(lis)
}

func startHTTPServer(server *Server, addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", handleHealth)
	mux.HandleFunc("/transfer", server.handleTransfer)
	mux.HandleFunc("/delete", server.handleDelete)

	log.Printf("HTTP server listening on %s", addr)
	return http.ListenAndServe(addr, mux)
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

func (s *Server) handleTransfer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		SourcePath string `json:"source_path"`
		DestPath   string `json:"dest_path"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}

	// Set response headers for streaming
	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("Transfer-Encoding", "chunked")
	
	encoder := json.NewEncoder(w)
	
	writeProgress := func(msgType, message string) {
		_ = encoder.Encode(ProgressMessage{
			Type:    msgType,
			Message: message,
			Time:    time.Now().Format(time.RFC3339),
		})
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}

	writeProgress("info", "Transfer started")

	// Parse source and destination
	sourcePrefix, sourcePath := parsePathPrefix(req.SourcePath)
	destPrefix, destPath := parsePathPrefix(req.DestPath)

	if sourcePrefix == "" || destPrefix == "" {
		writeProgress("error", "Invalid path prefix. Use 'local:' or 'peer:'")
		return
	}

	// Handle different transfer scenarios
	if sourcePrefix == "local" && destPrefix == "peer" {
		// Transfer from local to peer
		if err := s.transferLocalToPeer(sourcePath, destPath, writeProgress); err != nil {
			writeProgress("error", fmt.Sprintf("Transfer failed: %v", err))
			return
		}
	} else if sourcePrefix == "peer" && destPrefix == "local" {
		// Transfer from peer to local
		if err := s.transferPeerToLocal(sourcePath, destPath, writeProgress); err != nil {
			writeProgress("error", fmt.Sprintf("Transfer failed: %v", err))
			return
		}
	} else if sourcePrefix == "local" && destPrefix == "local" {
		// Local copy
		if err := s.copyLocal(sourcePath, destPath, writeProgress); err != nil {
			writeProgress("error", fmt.Sprintf("Copy failed: %v", err))
			return
		}
	} else {
		writeProgress("error", "Invalid transfer direction")
		return
	}

	writeProgress("completed", "Transfer completed successfully")
}

func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		FilePath string `json:"file_path"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}

	prefix, path := parsePathPrefix(req.FilePath)
	if prefix == "" {
		respondJSON(w, false, "Invalid path prefix. Use 'local:' or 'peer:'", "")
		return
	}

	if prefix == "local" {
		validPath, err := s.validator.ValidatePath(path)
		if err != nil {
			respondJSON(w, false, fmt.Sprintf("Path validation failed: %v", err), "local")
			return
		}

		if err := os.Remove(validPath); err != nil {
			respondJSON(w, false, fmt.Sprintf("Failed to delete file: %v", err), "local")
			return
		}

		respondJSON(w, true, "File deleted successfully", "local")
	} else {
		conn, err := s.connectToPeer()
		if err != nil {
			respondJSON(w, false, fmt.Sprintf("Failed to connect to peer: %v", err), "peer")
			return
		}
		defer func() { _ = conn.Close() }()

		client := pb.NewFileTransferClient(conn)
		resp, err := client.DeleteFile(context.Background(), &pb.DeleteRequest{FilePath: path})
		if err != nil {
			respondJSON(w, false, fmt.Sprintf("Failed to delete file on peer: %v", err), "peer")
			return
		}

		respondJSON(w, resp.Success, resp.Message, "peer")
	}
}

func respondJSON(w http.ResponseWriter, success bool, message, target string) {
	response := map[string]interface{}{
		"success": success,
		"message": message,
	}
	if target != "" {
		response["target"] = target
	}
	_ = json.NewEncoder(w).Encode(response)
}

// gRPC server implementation
func (s *Server) Transfer(stream pb.FileTransfer_TransferServer) error {
	var currentFile *os.File
	var currentPath string
	var currentRequestedPath string
	var receivedBytes int64

	defer func() {
		if currentFile != nil {
			_ = currentFile.Close()
			log.Printf("Transfer complete: received %d bytes, saved to %s", receivedBytes, currentPath)
		}
	}()

	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("receive error: %w", err)
		}
		
		// Validate and open file if first chunk or if file path changed
		if currentFile == nil || currentRequestedPath != chunk.FilePath {
			if currentFile != nil {
				_ = currentFile.Close()
			}

			validPath, err := s.validator.ValidateAndEnsureDir(chunk.FilePath)
			if err != nil {
				_ = stream.Send(&pb.TransferResponse{
					Success: false,
					Message: fmt.Sprintf("Path validation failed: %v", err),
				})
				return err
			}

			currentRequestedPath = chunk.FilePath
			currentPath = validPath
			currentFile, err = os.Create(validPath)
			if err != nil {
				_ = stream.Send(&pb.TransferResponse{
					Success: false,
					Message: fmt.Sprintf("Failed to create file: %v", err),
				})
				return err
			}
			receivedBytes = 0
		}

		// Verify checksum
		hash := sha256.Sum256(chunk.Data)
		checksum := hex.EncodeToString(hash[:])
		if checksum != chunk.Checksum {
			_ = stream.Send(&pb.TransferResponse{
				Success: false,
				Message: "Checksum mismatch",
			})
			return fmt.Errorf("checksum mismatch")
		}

		// Write data
		n, err := currentFile.Write(chunk.Data)
		if err != nil {
			_ = stream.Send(&pb.TransferResponse{
				Success: false,
				Message: fmt.Sprintf("Failed to write file: %v", err),
			})
			return err
		}
		receivedBytes += int64(n)

		// Send progress
		_ = stream.Send(&pb.TransferResponse{
			Success:          true,
			Message:          "Chunk received",
			BytesTransferred: receivedBytes,
		})

		if chunk.IsLast {
			break
		}
	}

	return nil
}

func (s *Server) DeleteFile(ctx context.Context, req *pb.DeleteRequest) (*pb.DeleteResponse, error) {
	validPath, err := s.validator.ValidatePath(req.FilePath)
	if err != nil {
		return &pb.DeleteResponse{
			Success: false,
			Message: fmt.Sprintf("Path validation failed: %v", err),
		}, nil
	}

	if err := os.Remove(validPath); err != nil {
		return &pb.DeleteResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to delete file: %v", err),
		}, nil
	}

	return &pb.DeleteResponse{
		Success: true,
		Message: "File deleted successfully",
	}, nil
}

// Helper functions
func parsePathPrefix(path string) (string, string) {
	parts := strings.SplitN(path, ":", 2)
	if len(parts) != 2 {
		return "", ""
	}
	return parts[0], parts[1]
}

func (s *Server) connectToPeer() (*grpc.ClientConn, error) {
	conn, err := grpc.NewClient(s.config.TargetServer,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to peer: %w", err)
	}
	return conn, nil
}

func (s *Server) transferLocalToPeer(sourcePath, destPath string, writeProgress func(string, string)) error {
	// Expand wildcards
	matches, err := s.expandWildcard(sourcePath)
	if err != nil {
		return err
	}

	conn, err := s.connectToPeer()
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	client := pb.NewFileTransferClient(conn)

	for _, match := range matches {
		writeProgress("progress", fmt.Sprintf("Transferring %s...", match))
		
		// Determine destination path
		finalDestPath := destPath
		if strings.HasSuffix(destPath, "/") {
			finalDestPath = filepath.Join(destPath, filepath.Base(match))
		}

		if err := s.sendFile(client, match, finalDestPath); err != nil {
			return fmt.Errorf("failed to send %s: %w", match, err)
		}
	}

	return nil
}

func (s *Server) transferPeerToLocal(sourcePath, destPath string, writeProgress func(string, string)) error {
	// For peer to local, we need to request the peer to send files to us
	// This is a simplified implementation - in production, you'd need a more sophisticated approach
	return fmt.Errorf("peer to local transfer not yet fully implemented")
}

func (s *Server) copyLocal(sourcePath, destPath string, writeProgress func(string, string)) error {
	matches, err := s.expandWildcard(sourcePath)
	if err != nil {
		return err
	}

	for _, match := range matches {
		writeProgress("progress", fmt.Sprintf("Copying %s...", match))
		
		validSrc, err := s.validator.ValidatePath(match)
		if err != nil {
			return err
		}

		finalDestPath := destPath
		if strings.HasSuffix(destPath, "/") {
			finalDestPath = filepath.Join(destPath, filepath.Base(match))
		}

		validDest, err := s.validator.ValidateAndEnsureDir(finalDestPath)
		if err != nil {
			return err
		}

		if err := copyFile(validSrc, validDest); err != nil {
			return err
		}
	}

	return nil
}

func (s *Server) expandWildcard(pattern string) ([]string, error) {
	validPattern, err := s.validator.ValidatePath(pattern)
	if err != nil {
		return nil, err
	}

	// Handle directory with /. (all contents including hidden files)
	if strings.HasSuffix(pattern, "/.") {
		dirPath := strings.TrimSuffix(validPattern, "/.")
		return s.listDirContents(dirPath)
	}

	// Handle directory with /* (all files in directory)
	if strings.HasSuffix(pattern, "/*") {
		return filepath.Glob(validPattern)
	}

	matches, err := filepath.Glob(validPattern)
	if err != nil {
		return nil, err
	}

	if len(matches) == 0 {
		// If no matches and no wildcard, treat as single file or directory
		if !strings.Contains(pattern, "*") {
			info, err := os.Stat(validPattern)
			if err == nil && info.IsDir() {
				// If it's a directory, get all files in it
				return s.listDirContents(validPattern)
			}
			return []string{validPattern}, nil
		}
		return nil, fmt.Errorf("no files match pattern: %s", pattern)
	}

	return matches, nil
}

func (s *Server) listDirContents(dirPath string) ([]string, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}

	var files []string
	for _, entry := range entries {
		fullPath := filepath.Join(dirPath, entry.Name())
		if entry.IsDir() {
			// Recursively get files from subdirectories
			subFiles, err := s.listDirContents(fullPath)
			if err != nil {
				return nil, err
			}
			files = append(files, subFiles...)
		} else {
			files = append(files, fullPath)
		}
	}

	return files, nil
}

func (s *Server) sendFile(client pb.FileTransferClient, srcPath, destPath string) error {
	file, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	stat, err := file.Stat()
	if err != nil {
		return err
	}

	stream, err := client.Transfer(context.Background())
	if err != nil {
		return err
	}

	// Handle empty files (0 bytes)
	if stat.Size() == 0 {
		// Send a single empty chunk to create the file
		hash := sha256.Sum256([]byte{})
		checksum := hex.EncodeToString(hash[:])

		chunk := &pb.FileChunk{
			FilePath:  destPath,
			Data:      []byte{},
			Offset:    0,
			TotalSize: 0,
			Checksum:  checksum,
			IsLast:    true,
		}

		if err := stream.Send(chunk); err != nil {
			return err
		}

		if _, recvErr := stream.Recv(); recvErr != nil && recvErr != io.EOF {
			return recvErr
		}

		if err := stream.CloseSend(); err != nil {
			return err
		}

		// Wait for final response
		for {
			_, err := stream.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				return err
			}
		}

		return nil
	}

	buffer := make([]byte, ChunkSize)
	var offset int64

	for {
		n, err := file.Read(buffer)
		if err != nil && err != io.EOF {
			return err
		}
		
		if n == 0 {
			break
		}

		data := buffer[:n]
		hash := sha256.Sum256(data)
		checksum := hex.EncodeToString(hash[:])

		chunk := &pb.FileChunk{
			FilePath:  destPath,
			Data:      data,
			Offset:    offset,
			TotalSize: stat.Size(),
			Checksum:  checksum,
			IsLast:    err == io.EOF,
		}

		if err := stream.Send(chunk); err != nil {
			return err
		}

		if _, recvErr := stream.Recv(); recvErr != nil && recvErr != io.EOF {
			return recvErr
		}

		offset += int64(n)

		if err == io.EOF {
			break
		}
	}

	if err := stream.CloseSend(); err != nil {
		return err
	}

	// Wait for final response
	for {
		_, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}

	return nil
}

func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = sourceFile.Close() }()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() { _ = destFile.Close() }()

	_, err = io.Copy(destFile, sourceFile)
	return err
}

func createDirIfNotExists(dir string) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return os.MkdirAll(dir, 0755)
	}
	return nil
}
