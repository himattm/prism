package fsutil

import (
	"os"
	"path/filepath"
)

// SecureWriteFile securely writes data to a file by creating a temporary file
// in the target directory and atomically renaming it to the target path.
// This prevents symlink attacks and partial writes.
func SecureWriteFile(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)

	// Create a temporary file in the target directory
	tmpFile, err := os.CreateTemp(dir, filepath.Base(path)+".*")
	if err != nil {
		return err
	}

	tmpPath := tmpFile.Name()

	// Ensure temp file is cleaned up if rename fails (or panics)
	defer os.Remove(tmpPath)

	// Set permissions (CreateTemp creates with 0600)
	if err := tmpFile.Chmod(perm); err != nil {
		tmpFile.Close()
		return err
	}

	// Write data to temp file
	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		return err
	}

	// Sync to ensure data is written to disk
	if err := tmpFile.Sync(); err != nil {
		tmpFile.Close()
		return err
	}

	// Close file before renaming (required on Windows)
	if err := tmpFile.Close(); err != nil {
		return err
	}

	// Atomically rename temp file to target path
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}

	return nil
}
