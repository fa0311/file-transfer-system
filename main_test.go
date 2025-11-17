package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParsePathPrefix(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantPrefix string
		wantPath   string
	}{
		{
			name:       "local prefix",
			input:      "local:/test.txt",
			wantPrefix: "local",
			wantPath:   "/test.txt",
		},
		{
			name:       "peer prefix",
			input:      "peer:/data/file.bin",
			wantPrefix: "peer",
			wantPath:   "/data/file.bin",
		},
		{
			name:       "no prefix",
			input:      "/test.txt",
			wantPrefix: "",
			wantPath:   "",
		},
		{
			name:       "invalid format",
			input:      "invalidformat",
			wantPrefix: "",
			wantPath:   "",
		},
		{
			name:       "multiple colons",
			input:      "local:/path:with:colons",
			wantPrefix: "local",
			wantPath:   "/path:with:colons",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPrefix, gotPath := parsePathPrefix(tt.input)
			if gotPrefix != tt.wantPrefix {
				t.Errorf("parsePathPrefix() prefix = %v, want %v", gotPrefix, tt.wantPrefix)
			}
			if gotPath != tt.wantPath {
				t.Errorf("parsePathPrefix() path = %v, want %v", gotPath, tt.wantPath)
			}
		})
	}
}

func TestCreateDirIfNotExists(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name    string
		dir     string
		wantErr bool
	}{
		{
			name:    "create new directory",
			dir:     filepath.Join(tmpDir, "newdir"),
			wantErr: false,
		},
		{
			name:    "existing directory",
			dir:     tmpDir,
			wantErr: false,
		},
		{
			name:    "nested directory creation",
			dir:     filepath.Join(tmpDir, "a", "b", "c"),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := createDirIfNotExists(tt.dir)
			if (err != nil) != tt.wantErr {
				t.Errorf("createDirIfNotExists() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				// Verify directory exists
				if _, err := os.Stat(tt.dir); os.IsNotExist(err) {
					t.Errorf("createDirIfNotExists() did not create directory: %v", tt.dir)
				}
			}
		})
	}
}

func TestCopyFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create source file
	srcFile := filepath.Join(tmpDir, "source.txt")
	content := []byte("test content for copy")
	if err := os.WriteFile(srcFile, content, 0644); err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	tests := []struct {
		name    string
		src     string
		dst     string
		wantErr bool
	}{
		{
			name:    "successful copy",
			src:     srcFile,
			dst:     filepath.Join(tmpDir, "dest.txt"),
			wantErr: false,
		},
		{
			name:    "non-existent source",
			src:     filepath.Join(tmpDir, "nonexistent.txt"),
			dst:     filepath.Join(tmpDir, "dest2.txt"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := copyFile(tt.src, tt.dst)
			if (err != nil) != tt.wantErr {
				t.Errorf("copyFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				// Verify destination file exists and has same content
				dstContent, err := os.ReadFile(tt.dst)
				if err != nil {
					t.Errorf("Failed to read destination file: %v", err)
					return
				}
				if string(dstContent) != string(content) {
					t.Errorf("copyFile() content mismatch, got %v, want %v", string(dstContent), string(content))
				}
			}
		})
	}
}

func TestProgressMessage(t *testing.T) {
	pm := ProgressMessage{
		Type:    "info",
		Message: "test message",
		Time:    "2024-01-01T00:00:00Z",
	}

	if pm.Type != "info" {
		t.Errorf("ProgressMessage.Type = %v, want %v", pm.Type, "info")
	}
	if pm.Message != "test message" {
		t.Errorf("ProgressMessage.Message = %v, want %v", pm.Message, "test message")
	}
	if pm.Time != "2024-01-01T00:00:00Z" {
		t.Errorf("ProgressMessage.Time = %v, want %v", pm.Time, "2024-01-01T00:00:00Z")
	}
}
