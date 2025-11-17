package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPathValidator_ValidatePath(t *testing.T) {
	// Setup test directory
	tmpDir := t.TempDir()
	validator := NewPathValidator(tmpDir)

	tests := []struct {
		name    string
		path    string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid absolute path",
			path:    "/test.txt",
			wantErr: false,
		},
		{
			name:    "valid absolute path with subdirectory",
			path:    "/subdir/test.txt",
			wantErr: false,
		},
		{
			name:    "relative path should fail",
			path:    "test.txt",
			wantErr: true,
			errMsg:  "relative paths are not allowed, use absolute paths starting with /",
		},
		{
			name:    "path with .. should fail",
			path:    "/../../etc/passwd",
			wantErr: true,
			errMsg:  "path contains invalid characters",
		},
		{
			name:    "path with .. in middle should fail",
			path:    "/test/../../../etc/passwd",
			wantErr: true,
			errMsg:  "path contains invalid characters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := validator.ValidatePath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errMsg != "" {
				if err.Error() != tt.errMsg {
					t.Errorf("ValidatePath() error message = %v, want %v", err.Error(), tt.errMsg)
				}
			}
		})
	}
}

func TestPathValidator_ValidateAndEnsureDir(t *testing.T) {
	tmpDir := t.TempDir()
	validator := NewPathValidator(tmpDir)

	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{
			name:    "create new directory structure",
			path:    "/new/deep/path/file.txt",
			wantErr: false,
		},
		{
			name:    "existing path",
			path:    "/existing.txt",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validPath, err := validator.ValidateAndEnsureDir(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateAndEnsureDir() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				// Check that parent directory was created
				dir := filepath.Dir(validPath)
				if _, err := os.Stat(dir); os.IsNotExist(err) {
					t.Errorf("ValidateAndEnsureDir() did not create directory: %v", dir)
				}
			}
		})
	}
}

func TestNewPathValidator(t *testing.T) {
	tmpDir := t.TempDir()
	validator := NewPathValidator(tmpDir)

	if validator.allowedDir != tmpDir {
		t.Errorf("NewPathValidator() allowedDir = %v, want %v", validator.allowedDir, tmpDir)
	}
}
