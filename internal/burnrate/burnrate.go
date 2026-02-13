package burnrate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Snapshot stores the initial cost and timestamp for rate calculation.
// Written to a temp file on first render, read on subsequent renders.
type Snapshot struct {
	Timestamp time.Time `json:"timestamp"`
	CostUSD   float64   `json:"cost_usd"`
}

// FilePath returns the path to the burn rate snapshot file for a session.
// Path separators are stripped from sessionID to prevent directory traversal.
func FilePath(sessionID string) string {
	safe := strings.ReplaceAll(strings.ReplaceAll(sessionID, "/", ""), "\\", "")
	return filepath.Join(os.TempDir(), fmt.Sprintf("prism-burn-%s", safe))
}

// LoadOrCreateSnapshot loads an existing snapshot or creates a new one.
// Returns the snapshot and whether it was pre-existing.
func LoadOrCreateSnapshot(sessionID string, currentCost float64) (*Snapshot, bool, error) {
	return LoadOrCreateSnapshotAt(sessionID, currentCost, time.Now())
}

// LoadOrCreateSnapshotAt is the testable version that accepts a timestamp.
func LoadOrCreateSnapshotAt(sessionID string, currentCost float64, now time.Time) (*Snapshot, bool, error) {
	if sessionID == "" {
		return nil, false, fmt.Errorf("empty session ID")
	}

	path := FilePath(sessionID)

	data, err := os.ReadFile(path)
	if err == nil {
		var snap Snapshot
		if err := json.Unmarshal(data, &snap); err == nil {
			return &snap, true, nil
		}
	}

	snap := &Snapshot{
		Timestamp: now,
		CostUSD:   currentCost,
	}

	data, err = json.Marshal(snap)
	if err != nil {
		return nil, false, err
	}

	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return nil, false, err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return nil, false, err
	}

	return snap, false, nil
}

// LoadSnapshot reads an existing snapshot without creating one.
// Returns nil if no snapshot exists.
func LoadSnapshot(sessionID string) *Snapshot {
	if sessionID == "" {
		return nil
	}
	data, err := os.ReadFile(FilePath(sessionID))
	if err != nil {
		return nil
	}
	var snap Snapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return nil
	}
	return &snap
}

// CalculateRate computes cost per hour between snapshot and current values.
// Returns the rate and whether it should be displayed.
func CalculateRate(snap *Snapshot, currentCost float64, now time.Time) (float64, bool) {
	elapsed := now.Sub(snap.Timestamp)

	if elapsed < 60*time.Second {
		return 0, false
	}

	costDiff := currentCost - snap.CostUSD
	if costDiff < 0 {
		return 0, false
	}

	hours := elapsed.Hours()
	if hours <= 0 {
		return 0, false
	}

	rate := costDiff / hours
	if rate < 0.01 {
		return 0, false
	}

	return rate, true
}

// FormatRate formats the burn rate for display.
func FormatRate(rate float64) string {
	if rate >= 10.0 {
		return fmt.Sprintf("~$%.0f/h", rate)
	}
	return fmt.Sprintf("~$%.2f/h", rate)
}

// Cleanup removes the burn rate snapshot file for a session.
func Cleanup(sessionID string) {
	if sessionID == "" {
		return
	}
	os.Remove(FilePath(sessionID))
}
