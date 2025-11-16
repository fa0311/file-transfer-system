package progress

import (
	"sync"
	"time"
)

// FileProgress tracks the progress of a file transfer
type FileProgress struct {
	FileID            string
	FilePath          string
	TotalSize         int64
	BytesTransferred  int64
	StartTime         time.Time
	LastUpdateTime    time.Time
	Status            string
	ErrorMessage      string
	mu                sync.RWMutex
}

// Tracker manages progress for multiple file transfers
type Tracker struct {
	transfers map[string]*FileProgress
	mu        sync.RWMutex
}

// NewTracker creates a new progress tracker
func NewTracker() *Tracker {
	return &Tracker{
		transfers: make(map[string]*FileProgress),
	}
}

// StartTransfer initializes a new file transfer
func (t *Tracker) StartTransfer(fileID, filePath string, totalSize int64) *FileProgress {
	t.mu.Lock()
	defer t.mu.Unlock()

	progress := &FileProgress{
		FileID:           fileID,
		FilePath:         filePath,
		TotalSize:        totalSize,
		BytesTransferred: 0,
		StartTime:        time.Now(),
		LastUpdateTime:   time.Now(),
		Status:           "in_progress",
	}

	t.transfers[fileID] = progress
	return progress
}

// UpdateProgress updates the bytes transferred for a file
func (t *Tracker) UpdateProgress(fileID string, bytesTransferred int64) {
	t.mu.RLock()
	progress, exists := t.transfers[fileID]
	t.mu.RUnlock()

	if !exists {
		return
	}

	progress.mu.Lock()
	progress.BytesTransferred = bytesTransferred
	progress.LastUpdateTime = time.Now()
	progress.mu.Unlock()
}

// CompleteTransfer marks a transfer as completed
func (t *Tracker) CompleteTransfer(fileID string) {
	t.mu.RLock()
	progress, exists := t.transfers[fileID]
	t.mu.RUnlock()

	if !exists {
		return
	}

	progress.mu.Lock()
	progress.Status = "completed"
	progress.LastUpdateTime = time.Now()
	progress.mu.Unlock()
}

// FailTransfer marks a transfer as failed
func (t *Tracker) FailTransfer(fileID string, errorMsg string) {
	t.mu.RLock()
	progress, exists := t.transfers[fileID]
	t.mu.RUnlock()

	if !exists {
		return
	}

	progress.mu.Lock()
	progress.Status = "error"
	progress.ErrorMessage = errorMsg
	progress.LastUpdateTime = time.Now()
	progress.mu.Unlock()
}

// GetProgress returns the current progress of a transfer
func (t *Tracker) GetProgress(fileID string) *FileProgress {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if progress, exists := t.transfers[fileID]; exists {
		return progress
	}
	return nil
}

// GetProgressPercent returns the progress percentage
func (fp *FileProgress) GetProgressPercent() float32 {
	fp.mu.RLock()
	defer fp.mu.RUnlock()

	if fp.TotalSize == 0 {
		return 0
	}
	return float32(fp.BytesTransferred) / float32(fp.TotalSize) * 100
}

// GetTransferSpeed returns the transfer speed in MB/s
func (fp *FileProgress) GetTransferSpeed() float64 {
	fp.mu.RLock()
	defer fp.mu.RUnlock()

	elapsed := time.Since(fp.StartTime).Seconds()
	if elapsed == 0 {
		return 0
	}

	bytesPerSecond := float64(fp.BytesTransferred) / elapsed
	return bytesPerSecond / (1024 * 1024) // Convert to MB/s
}

// RemoveTransfer removes a completed transfer from tracking
func (t *Tracker) RemoveTransfer(fileID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.transfers, fileID)
}
