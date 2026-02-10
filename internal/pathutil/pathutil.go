package pathutil

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ValidateWithinBase checks that the resolved path stays within the base directory
// to prevent path traversal attacks. Returns nil if baseDir is empty.
func ValidateWithinBase(path, baseDir string) error {
	if baseDir == "" {
		return nil
	}

	cleanBase, err := filepath.Abs(baseDir)
	if err != nil {
		return fmt.Errorf("failed to resolve base directory: %v", err)
	}

	cleanPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %v", err)
	}

	if !strings.HasPrefix(cleanPath, cleanBase+string(filepath.Separator)) && cleanPath != cleanBase {
		return fmt.Errorf("path traversal detected: %s is outside allowed directory %s", path, baseDir)
	}

	return nil
}
