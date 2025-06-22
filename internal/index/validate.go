package index

import (
	"fmt"
	"path/filepath"
	"strings"
)

// validateFilePath validates that a file path is safe to use
func validateFilePath(filename string) error {
	cleanPath := filepath.Clean(filename)
	if strings.Contains(cleanPath, "..") {
		return fmt.Errorf("invalid file path: contains directory traversal")
	}
	if filepath.IsAbs(cleanPath) {
		return fmt.Errorf("invalid file path: absolute paths not allowed")
	}
	return nil
}
