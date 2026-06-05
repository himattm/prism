package fsutil

import (
	"os"
	"path/filepath"
)

// SecureWriteFile atomically writes data to a file by creating a temporary
// file in the target directory, writing the data, and renaming it to the target
// path. This prevents symlink attacks because os.Rename replaces the target,
// including symlinks, instead of following them.
func SecureWriteFile(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)

	// Create temporary file in target directory to ensure they're on same mount point
	tmpFile, err := os.CreateTemp(dir, filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()

	// Ensure temporary file is cleaned up if we return early
	defer os.Remove(tmpPath)

	if err := tmpFile.Chmod(perm); err != nil {
		tmpFile.Close()
		return err
	}

	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		return err
	}

	if err := tmpFile.Close(); err != nil {
		return err
	}

	// os.Rename atomically replaces the target, protecting against symlink attacks
	return os.Rename(tmpPath, path)
}
