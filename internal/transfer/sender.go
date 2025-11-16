package transfer

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"

	pb "github.com/fileserver/transfer/api/proto"
	"github.com/fileserver/transfer/internal/progress"
	"github.com/google/uuid"
)

const (
	// ChunkSize is the size of each chunk (1MB)
	ChunkSize = 1024 * 1024
)

// Sender handles file sending operations
type Sender struct {
	validator *Validator
	tracker   *progress.Tracker
}

// NewSender creates a new file sender
func NewSender(allowedDir string, tracker *progress.Tracker) *Sender {
	return &Sender{
		validator: NewValidator(allowedDir),
		tracker:   tracker,
	}
}

// PrepareFiles validates and prepares files for transfer
func (s *Sender) PrepareFiles(sourcePath string) ([]string, error) {
	return s.validator.ValidateSourcePath(sourcePath)
}

// SendFile sends a file in chunks
func (s *Sender) SendFile(filePath, destPath string, stream pb.FileTransfer_TransferFileClient) error {
	// Open the file
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Get file info
	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}

	totalSize := info.Size()
	fileID := uuid.New().String()

	// Start tracking progress
	s.tracker.StartTransfer(fileID, filePath, totalSize)

	// Calculate relative path for destination
	relativePath := filepath.Base(filePath)
	if destPath != "" {
		// If destPath is a directory, append filename
		relativePath = filepath.Join(destPath, filepath.Base(filePath))
	}

	// Send file in chunks
	buffer := make([]byte, ChunkSize)
	var offset int64 = 0

	for {
		n, err := file.Read(buffer)
		if err != nil && err != io.EOF {
			s.tracker.FailTransfer(fileID, err.Error())
			return fmt.Errorf("failed to read file: %w", err)
		}

		if n == 0 {
			break
		}

		// Calculate checksum for this chunk
		checksum := sha256.Sum256(buffer[:n])

		// Create chunk message
		chunk := &pb.FileChunk{
			FileId:    fileID,
			FilePath:  relativePath,
			TotalSize: totalSize,
			Offset:    offset,
			Data:      buffer[:n],
			Checksum:  checksum[:],
			IsLast:    err == io.EOF || offset+int64(n) >= totalSize,
		}

		// Send chunk
		if err := stream.Send(chunk); err != nil {
			s.tracker.FailTransfer(fileID, err.Error())
			return fmt.Errorf("failed to send chunk: %w", err)
		}

		// Update progress
		offset += int64(n)
		s.tracker.UpdateProgress(fileID, offset)

		if chunk.IsLast {
			break
		}
	}

	// Receive status updates
	for {
		status, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			s.tracker.FailTransfer(fileID, err.Error())
			return fmt.Errorf("failed to receive status: %w", err)
		}

		if status.Status == "error" {
			s.tracker.FailTransfer(fileID, status.ErrorMessage)
			return fmt.Errorf("transfer failed: %s", status.ErrorMessage)
		}

		if status.Status == "completed" {
			s.tracker.CompleteTransfer(fileID)
			break
		}
	}

	return nil
}

// GetProgress returns the progress for a file transfer
func (s *Sender) GetProgress(fileID string) *progress.FileProgress {
	return s.tracker.GetProgress(fileID)
}
