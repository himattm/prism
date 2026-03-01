package burnrate

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

func TestFilePath(t *testing.T) {
	path := FilePath("test-session-123")
	if path == "" {
		t.Error("FilePath returned empty string")
	}
	if !strings.Contains(path, "prism-burn-test-session-123") {
		t.Errorf("expected path to contain 'prism-burn-test-session-123', got: %s", path)
	}
}

func TestFilePath_PathTraversal(t *testing.T) {
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
			path := FilePath(tt.sessionID)
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

func TestLoadOrCreateSnapshotAt_CreatesNew(t *testing.T) {
	sessionID := "test-burn-create-" + time.Now().Format("20060102150405")
	defer os.Remove(FilePath(sessionID))

	now := time.Now()
	snap, existed, err := LoadOrCreateSnapshotAt(sessionID, 1.50, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if existed {
		t.Error("expected existed=false for new snapshot")
	}
	if snap.CostUSD != 1.50 {
		t.Errorf("expected cost 1.50, got %f", snap.CostUSD)
	}

	// Verify file was created
	data, err := os.ReadFile(FilePath(sessionID))
	if err != nil {
		t.Fatalf("snapshot file should exist: %v", err)
	}

	var saved Snapshot
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatalf("failed to parse snapshot: %v", err)
	}
	if saved.CostUSD != 1.50 {
		t.Errorf("saved cost should be 1.50, got %f", saved.CostUSD)
	}
}

func TestLoadOrCreateSnapshotAt_LoadsExisting(t *testing.T) {
	sessionID := "test-burn-load-" + time.Now().Format("20060102150405")
	path := FilePath(sessionID)
	defer os.Remove(path)

	// Create a snapshot file manually
	original := Snapshot{
		Timestamp: time.Now().Add(-5 * time.Minute),
		CostUSD:   0.75,
	}
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("failed to marshal snapshot: %v", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("failed to write snapshot file: %v", err)
	}

	snap, existed, err := LoadOrCreateSnapshotAt(sessionID, 2.00, time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !existed {
		t.Error("expected existed=true for existing snapshot")
	}
	if snap.CostUSD != 0.75 {
		t.Errorf("should load original cost 0.75, got %f", snap.CostUSD)
	}
}

func TestLoadOrCreateSnapshotAt_EmptySessionID(t *testing.T) {
	_, _, err := LoadOrCreateSnapshotAt("", 1.0, time.Now())
	if err == nil {
		t.Error("expected error for empty session ID")
	}
}

func TestLoadSnapshot_Existing(t *testing.T) {
	sessionID := "test-burn-loadonly-" + time.Now().Format("20060102150405")
	path := FilePath(sessionID)
	defer os.Remove(path)

	original := Snapshot{
		Timestamp: time.Now(),
		CostUSD:   3.50,
	}
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("failed to marshal snapshot: %v", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("failed to write snapshot file: %v", err)
	}

	snap := LoadSnapshot(sessionID)
	if snap == nil {
		t.Fatal("expected non-nil snapshot")
	}
	if snap.CostUSD != 3.50 {
		t.Errorf("expected cost 3.50, got %f", snap.CostUSD)
	}
}

func TestLoadSnapshot_NonExistent(t *testing.T) {
	snap := LoadSnapshot("nonexistent-session")
	if snap != nil {
		t.Error("expected nil for non-existent snapshot")
	}
}

func TestLoadSnapshot_EmptySessionID(t *testing.T) {
	snap := LoadSnapshot("")
	if snap != nil {
		t.Error("expected nil for empty session ID")
	}
}

func TestCalculateRate_TooEarly(t *testing.T) {
	now := time.Now()
	snap := &Snapshot{
		Timestamp: now.Add(-30 * time.Second),
		CostUSD:   0.50,
	}

	_, show := CalculateRate(snap, 1.50, now)
	if show {
		t.Error("should not show burn rate before 60 seconds")
	}
}

func TestCalculateRate_AfterMinimumTime(t *testing.T) {
	now := time.Now()
	snap := &Snapshot{
		Timestamp: now.Add(-2 * time.Hour),
		CostUSD:   1.00,
	}

	rate, show := CalculateRate(snap, 5.00, now)
	if !show {
		t.Error("should show burn rate after 2 hours")
	}
	// Rate should be (5.00 - 1.00) / 2.0 = $2.00/h
	if rate < 1.99 || rate > 2.01 {
		t.Errorf("expected rate ~$2.00/h, got $%.2f/h", rate)
	}
}

func TestCalculateRate_NegativeCostDiff(t *testing.T) {
	now := time.Now()
	snap := &Snapshot{
		Timestamp: now.Add(-1 * time.Hour),
		CostUSD:   5.00,
	}

	_, show := CalculateRate(snap, 3.00, now)
	if show {
		t.Error("should not show burn rate with negative cost difference")
	}
}

func TestCalculateRate_VeryLowRate(t *testing.T) {
	now := time.Now()
	snap := &Snapshot{
		Timestamp: now.Add(-1 * time.Hour),
		CostUSD:   1.000,
	}

	_, show := CalculateRate(snap, 1.005, now)
	if show {
		t.Error("should not show burn rate below $0.01/h")
	}
}

func TestCalculateRate_ExactlyMinRate(t *testing.T) {
	now := time.Now()
	snap := &Snapshot{
		Timestamp: now.Add(-1 * time.Hour),
		CostUSD:   1.00,
	}

	rate, show := CalculateRate(snap, 1.01, now)
	if !show {
		t.Error("should show burn rate at exactly $0.01/h")
	}
	if rate < 0.009 || rate > 0.011 {
		t.Errorf("expected rate ~$0.01/h, got $%.4f/h", rate)
	}
}

func TestFormatRate_SmallRate(t *testing.T) {
	result := FormatRate(1.23)
	if result != "~$1.23/h" {
		t.Errorf("expected '~$1.23/h', got '%s'", result)
	}
}

func TestFormatRate_LargeRate(t *testing.T) {
	result := FormatRate(15.50)
	if result != "~$16/h" {
		t.Errorf("expected '~$16/h', got '%s'", result)
	}
}

func TestFormatRate_TenExact(t *testing.T) {
	result := FormatRate(10.0)
	if result != "~$10/h" {
		t.Errorf("expected '~$10/h', got '%s'", result)
	}
}

func TestFormatRate_SubDollar(t *testing.T) {
	result := FormatRate(0.45)
	if result != "~$0.45/h" {
		t.Errorf("expected '~$0.45/h', got '%s'", result)
	}
}

func TestCleanup_RemovesFile(t *testing.T) {
	sessionID := "test-burn-cleanup-" + time.Now().Format("20060102150405")
	path := FilePath(sessionID)

	if err := os.WriteFile(path, []byte("{}"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("test setup: file should exist")
	}

	Cleanup(sessionID)

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("file should have been removed")
	}
}

func TestCleanup_EmptySessionID(t *testing.T) {
	Cleanup("")
}

func TestCleanup_NonExistentFile(t *testing.T) {
	Cleanup("nonexistent-session-id")
}
