package hooks

import (
	"fmt"
	"os"
	"path/filepath"
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
