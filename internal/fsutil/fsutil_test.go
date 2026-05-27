package fsutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSecureWriteFile(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")

	data := []byte("hello secure world")
	err := SecureWriteFile(filePath, data, 0644)
	if err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	readData, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	if string(readData) != string(data) {
		t.Errorf("expected %s, got %s", string(data), string(readData))
	}

	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("failed to stat file: %v", err)
	}

	if info.Mode().Perm() != 0644 {
		t.Errorf("expected permission 0644, got %v", info.Mode().Perm())
	}
}
