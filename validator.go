package main

import (
	"fmt"
	"path/filepath"
	"strings"
)

type PathValidator struct {
	allowedDir string
}

func NewPathValidator(allowedDir string) *PathValidator {
	return &PathValidator{
		allowedDir: allowedDir,
	}
}

// ValidatePath checks if the given path is within the allowed directory
func (v *PathValidator) ValidatePath(path string) (string, error) {
	// Remove any potential path traversal attempts
	if strings.Contains(path, "..") {
		return "", fmt.Errorf("path contains invalid characters: ..")
	}

	// Clean the path
	cleanPath := filepath.Clean(path)
	
	// Remove leading slash to treat as relative path
	cleanPath = strings.TrimPrefix(cleanPath, "/")
	
	// Always treat paths relative to allowed directory
	absPath := filepath.Join(v.allowedDir, cleanPath)

	// Resolve any symlinks
	resolvedPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		// If file doesn't exist yet, just use the cleaned path
		resolvedPath = absPath
	}

	// Ensure the path is within allowed directory
	relPath, err := filepath.Rel(v.allowedDir, resolvedPath)
	if err != nil {
		return "", fmt.Errorf("failed to determine relative path: %w", err)
	}

	// Check if path tries to escape allowed directory
	if strings.HasPrefix(relPath, "..") {
		return "", fmt.Errorf("path is outside allowed directory")
	}

	return resolvedPath, nil
}

// ValidateAndEnsureDir validates the path and creates parent directories if needed
func (v *PathValidator) ValidateAndEnsureDir(path string) (string, error) {
	validPath, err := v.ValidatePath(path)
	if err != nil {
		return "", err
	}

	// Create parent directory if it doesn't exist
	dir := filepath.Dir(validPath)
	if err := createDirIfNotExists(dir); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	return validPath, nil
}
