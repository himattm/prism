package hooks

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestHandleSessionEnd_CleansBurnRateFile(t *testing.T) {
	sessionID := "test-session-end-burn-" + time.Now().Format("20060102150405")

	// Create the burn rate file
	burnRateFile := filepath.Join(os.TempDir(), fmt.Sprintf("prism-burn-%s", sessionID))
	if err := os.WriteFile(burnRateFile, []byte(`{"timestamp":"2025-01-01T00:00:00Z","cost_usd":1.0}`), 0644); err != nil {
		t.Fatalf("failed to create burn rate file: %v", err)
	}
	defer os.Remove(burnRateFile)

	// Also create idle file (normal cleanup)
	idleFile := filepath.Join(os.TempDir(), fmt.Sprintf("prism-idle-%s", sessionID))
	if err := os.WriteFile(idleFile, []byte{}, 0644); err != nil {
		t.Fatalf("failed to create idle file: %v", err)
	}
	defer os.Remove(idleFile)

	m := NewManager()
	input := Input{SessionID: sessionID}

	err := m.HandleSessionEnd(input, nil)
	if err != nil {
		t.Fatalf("HandleSessionEnd returned error: %v", err)
	}

	// Verify burn rate file was removed
	if _, err := os.Stat(burnRateFile); !os.IsNotExist(err) {
		t.Error("burn rate file should have been removed by HandleSessionEnd")
	}

	// Verify idle file was also removed
	if _, err := os.Stat(idleFile); !os.IsNotExist(err) {
		t.Error("idle file should have been removed by HandleSessionEnd")
	}
}

func TestHandleSessionEnd_NoErrorWithoutFiles(t *testing.T) {
	m := NewManager()
	input := Input{SessionID: "nonexistent-session-" + time.Now().Format("20060102150405")}

	err := m.HandleSessionEnd(input, nil)
	if err != nil {
		t.Fatalf("HandleSessionEnd should not error when files don't exist: %v", err)
	}
}

func TestHandleSessionEnd_EmptySessionID(t *testing.T) {
	m := NewManager()
	input := Input{SessionID: ""}

	err := m.HandleSessionEnd(input, nil)
	if err != nil {
		t.Fatalf("HandleSessionEnd should not error with empty session ID: %v", err)
	}
}

func TestIdleFilePath(t *testing.T) {
	path := IdleFilePath("test-session-123")
	if path == "" {
		t.Error("IdleFilePath returned empty string")
	}
	if !strings.Contains(path, "prism-idle-test-session-123") {
		t.Errorf("expected path to contain 'prism-idle-test-session-123', got: %s", path)
	}
}

func TestIdleFilePath_PathTraversal(t *testing.T) {
	tests := []struct {
		name      string
		sessionID string
	}{
		{"dot-dot-slash", "../../../etc/passwd"},
		{"forward-slash", "foo/bar/baz"},
		{"backslash", `foo\bar\baz`},
		{"mixed-separators", `../foo\bar/../baz`},
	}

	tmpDir := os.TempDir()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := IdleFilePath(tt.sessionID)
			if !strings.HasPrefix(path, tmpDir) {
				t.Errorf("path %q escapes temp dir %q", path, tmpDir)
			}
			// Ensure no path separators in the filename portion
			filename := strings.TrimPrefix(path, tmpDir)
			filename = strings.TrimPrefix(filename, string(os.PathSeparator))
			if strings.ContainsAny(filename, `/\`) {
				t.Errorf("filename %q contains path separators", filename)
			}
		})
	}
}
