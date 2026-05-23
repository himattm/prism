package fsutil

import (
	"os"
	"path/filepath"
)

// SecureWriteFile performs an atomic write to prevent symlink attacks and partial writes.
// It creates a temporary file in the same directory as the target file, writes the data,
// sets the permissions, and then atomically renames the temporary file to the target path.
func SecureWriteFile(name string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(name)

	// Ensure directory exists
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	f, err := os.CreateTemp(dir, "tmp-*")
	if err != nil {
		return err
	}
	tmpName := f.Name()

	// Ensure cleanup in case of failure
	defer os.Remove(tmpName)

	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}

	// Sync to ensure it's written to disk
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}

	if err := f.Close(); err != nil {
		return err
	}

	if err := os.Chmod(tmpName, perm); err != nil {
		return err
	}

	return os.Rename(tmpName, name)
}
