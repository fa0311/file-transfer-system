package transfer

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"

	pb "github.com/fileserver/transfer/api/proto"
	"github.com/fileserver/transfer/internal/progress"
)

// Receiver handles file receiving operations
type Receiver struct {
	validator  *Validator
	tracker    *progress.Tracker
	allowedDir string
}

// NewReceiver creates a new file receiver
func NewReceiver(allowedDir string, tracker *progress.Tracker) *Receiver {
	return &Receiver{
		validator:  NewValidator(allowedDir),
		tracker:    tracker,
		allowedDir: allowedDir,
	}
}

// ReceiveFile receives a file from a stream
func (r *Receiver) ReceiveFile(stream pb.FileTransfer_TransferFileServer) error {
	var (
		currentFile *os.File
		fileID      string
		filePath    string
		totalSize   int64
		received    int64
	)

	defer func() {
		if currentFile != nil {
			currentFile.Close()
		}
	}()

	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			// End of stream
			if currentFile != nil {
				if err := r.finalizeFile(stream, fileID, filePath, totalSize, received); err != nil {
					return err
				}
			}
			return nil
		}
		if err != nil {
			r.sendErrorStatus(stream, fileID, fmt.Sprintf("failed to receive chunk: %v", err))
			return err
		}

		// First chunk of a new file
		if fileID == "" || fileID != chunk.FileId {
			// Close previous file if exists
			if currentFile != nil {
				if err := r.finalizeFile(stream, fileID, filePath, totalSize, received); err != nil {
					return err
				}
				currentFile = nil
			}

			// Initialize new file transfer
			fileID = chunk.FileId
			totalSize = chunk.TotalSize

			// Validate and prepare destination path
			validatedPath, err := r.validator.ValidateDestPath(chunk.FilePath)
			if err != nil {
				r.sendErrorStatus(stream, fileID, fmt.Sprintf("invalid destination path: %v", err))
				return err
			}
			filePath = validatedPath

			// Create directory if needed
			dir := filepath.Dir(filePath)
			if err := os.MkdirAll(dir, 0755); err != nil {
				r.sendErrorStatus(stream, fileID, fmt.Sprintf("failed to create directory: %v", err))
				return err
			}

			// Open file for writing
			currentFile, err = os.Create(filePath)
			if err != nil {
				r.sendErrorStatus(stream, fileID, fmt.Sprintf("failed to create file: %v", err))
				return err
			}

			// Start tracking progress
			r.tracker.StartTransfer(fileID, filePath, totalSize)
			received = 0
		}

		// Verify checksum
		checksum := sha256.Sum256(chunk.Data)
		if string(checksum[:]) != string(chunk.Checksum) {
			err := fmt.Errorf("checksum mismatch at offset %d", chunk.Offset)
			r.sendErrorStatus(stream, fileID, err.Error())
			r.tracker.FailTransfer(fileID, err.Error())
			return err
		}

		// Write chunk to file
		n, err := currentFile.WriteAt(chunk.Data, chunk.Offset)
		if err != nil {
			r.sendErrorStatus(stream, fileID, fmt.Sprintf("failed to write chunk: %v", err))
			r.tracker.FailTransfer(fileID, err.Error())
			return err
		}

		received += int64(n)
		r.tracker.UpdateProgress(fileID, received)

		// Send progress status
		progress := r.tracker.GetProgress(fileID)
		status := &pb.TransferStatus{
			FileId:            fileID,
			FilePath:          filePath,
			BytesTransferred:  received,
			TotalSize:         totalSize,
			ProgressPercent:   progress.GetProgressPercent(),
			Status:            "in_progress",
			TransferSpeedMbps: progress.GetTransferSpeed(),
		}

		if err := stream.Send(status); err != nil {
			r.tracker.FailTransfer(fileID, err.Error())
			return err
		}

		// Check if this is the last chunk
		if chunk.IsLast {
			if err := r.finalizeFile(stream, fileID, filePath, totalSize, received); err != nil {
				return err
			}
			currentFile.Close()
			currentFile = nil
			fileID = ""
		}
	}
}

func (r *Receiver) finalizeFile(stream pb.FileTransfer_TransferFileServer, fileID, filePath string, totalSize, received int64) error {
	// Verify that we received all bytes
	if received != totalSize {
		err := fmt.Errorf("incomplete transfer: received %d bytes, expected %d", received, totalSize)
		r.sendErrorStatus(stream, fileID, err.Error())
		r.tracker.FailTransfer(fileID, err.Error())
		return err
	}

	// Mark as completed
	r.tracker.CompleteTransfer(fileID)

	// Send completion status
	progress := r.tracker.GetProgress(fileID)
	status := &pb.TransferStatus{
		FileId:            fileID,
		FilePath:          filePath,
		BytesTransferred:  received,
		TotalSize:         totalSize,
		ProgressPercent:   100.0,
		Status:            "completed",
		TransferSpeedMbps: progress.GetTransferSpeed(),
	}

	return stream.Send(status)
}

func (r *Receiver) sendErrorStatus(stream pb.FileTransfer_TransferFileServer, fileID, errorMsg string) {
	status := &pb.TransferStatus{
		FileId:       fileID,
		Status:       "error",
		ErrorMessage: errorMsg,
	}
	stream.Send(status)
}
