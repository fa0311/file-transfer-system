package transfer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Validator validates file paths against allowed directory
type Validator struct {
	allowedDir string
}

// NewValidator creates a new path validator
func NewValidator(allowedDir string) *Validator {
	return &Validator{
		allowedDir: allowedDir,
	}
}

// ValidatePath validates that a path is within the allowed directory
// and doesn't contain path traversal attempts
func (v *Validator) ValidatePath(path string) (string, error) {
	// Clean the path to resolve any .. or . components
	cleanPath := filepath.Clean(path)

	// Make it absolute if it's not already
	var absPath string
	if filepath.IsAbs(cleanPath) {
		absPath = cleanPath
	} else {
		absPath = filepath.Join(v.allowedDir, cleanPath)
	}

	// Resolve symlinks
	resolvedPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		// If file doesn't exist yet, check the directory
		dir := filepath.Dir(absPath)
		resolvedDir, err := filepath.EvalSymlinks(dir)
		if err != nil {
			// Directory doesn't exist, validate the path structure
			resolvedPath = absPath
		} else {
			resolvedPath = filepath.Join(resolvedDir, filepath.Base(absPath))
		}
	}

	// Check if the resolved path is within allowed directory
	if !strings.HasPrefix(resolvedPath, v.allowedDir) {
		return "", fmt.Errorf("path '%s' is outside allowed directory '%s'", path, v.allowedDir)
	}

	// Check for suspicious patterns
	if strings.Contains(path, "..") {
		return "", fmt.Errorf("path traversal detected in path: %s", path)
	}

	return resolvedPath, nil
}

// ValidateSourcePath validates a source path (can use wildcards)
func (v *Validator) ValidateSourcePath(path string) ([]string, error) {
	// First validate the pattern itself
	cleanPath := filepath.Clean(path)
	
	// Check if it's absolute or relative
	var pattern string
	if filepath.IsAbs(cleanPath) {
		pattern = cleanPath
	} else {
		pattern = filepath.Join(v.allowedDir, cleanPath)
	}

	// Check for path traversal before globbing
	if strings.Contains(path, "..") {
		return nil, fmt.Errorf("path traversal detected in path: %s", path)
	}

	// Expand wildcards
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid glob pattern: %w", err)
	}

	if len(matches) == 0 {
		return nil, fmt.Errorf("no files match pattern: %s", path)
	}

	// Validate each match
	validPaths := make([]string, 0, len(matches))
	for _, match := range matches {
		// Ensure the match is within allowed directory
		if !strings.HasPrefix(match, v.allowedDir) {
			continue // Skip files outside allowed directory
		}

		// Only include regular files
		info, err := os.Lstat(match)
		if err != nil {
			continue
		}
		if info.Mode().IsRegular() {
			validPaths = append(validPaths, match)
		}
	}

	if len(validPaths) == 0 {
		return nil, fmt.Errorf("no valid files found matching pattern: %s", path)
	}

	return validPaths, nil
}

// ValidateDestPath validates a destination path
func (v *Validator) ValidateDestPath(path string) (string, error) {
	return v.ValidatePath(path)
}
