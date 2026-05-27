package fsutil

import (
	"fmt"
	"os"
	"path/filepath"
)

// SecureWriteFile atomically writes data to a file by creating a temporary file
// in the same directory and renaming it to the target path.
// This prevents symlink attacks and race conditions.
func SecureWriteFile(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)

	// Create a temporary file in the same directory as the target
	f, err := os.CreateTemp(dir, "prism-tmp-*")
	if err != nil {
		return fmt.Errorf("failed to create temporary file: %w", err)
	}
	tmpName := f.Name()

	// Ensure temp file is cleaned up if we fail before rename
	defer func() {
		f.Close()
		if _, err := os.Stat(tmpName); err == nil {
			os.Remove(tmpName)
		}
	}()

	// Write data
	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("failed to write to temporary file: %w", err)
	}

	// Set permissions
	if err := f.Chmod(perm); err != nil {
		return fmt.Errorf("failed to set permissions on temporary file: %w", err)
	}

	// Close file before renaming
	if err := f.Close(); err != nil {
		return fmt.Errorf("failed to close temporary file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("failed to rename temporary file to target: %w", err)
	}

	return nil
}
