package statusline

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/himattm/prism/internal/burnrate"
)

func TestRenderCost_WithBurnRate(t *testing.T) {
	sessionID := "test-burn-render-" + time.Now().Format("20060102150405")
	path := burnrate.FilePath(sessionID)
	defer os.Remove(path)

	// Create a snapshot from 2 hours ago with $1.00
	snap := struct {
		Timestamp time.Time `json:"timestamp"`
		CostUSD   float64   `json:"cost_usd"`
	}{
		Timestamp: time.Now().Add(-2 * time.Hour),
		CostUSD:   1.00,
	}
	data, err := json.Marshal(snap)
	if err != nil {
		t.Fatalf("failed to marshal snapshot: %v", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("failed to write snapshot file: %v", err)
	}

	sl := &StatusLine{
		input: Input{
			SessionID: sessionID,
			Cost:      CostInfo{TotalCostUSD: 5.00},
		},
	}

	result := sl.renderCost()

	// Should contain the cost
	if !strings.Contains(result, "$5.00") {
		t.Errorf("expected cost '$5.00' in output, got: %s", result)
	}
	// Should contain burn rate: (5.00 - 1.00) / 2.0 = $2.00/h
	if !strings.Contains(result, "~$2.00/h") {
		t.Errorf("expected burn rate '~$2.00/h' in output, got: %s", result)
	}
}

func TestRenderCost_NoBurnRateBeforeMinTime(t *testing.T) {
	sessionID := "test-burn-noshow-" + time.Now().Format("20060102150405")
	path := burnrate.FilePath(sessionID)
	defer os.Remove(path)

	// Create a snapshot from just 10 seconds ago
	snap := struct {
		Timestamp time.Time `json:"timestamp"`
		CostUSD   float64   `json:"cost_usd"`
	}{
		Timestamp: time.Now().Add(-10 * time.Second),
		CostUSD:   1.00,
	}
	data, err := json.Marshal(snap)
	if err != nil {
		t.Fatalf("failed to marshal snapshot: %v", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("failed to write snapshot file: %v", err)
	}

	sl := &StatusLine{
		input: Input{
			SessionID: sessionID,
			Cost:      CostInfo{TotalCostUSD: 5.00},
		},
	}

	result := sl.renderCost()

	if !strings.Contains(result, "$5.00") {
		t.Errorf("expected cost '$5.00' in output, got: %s", result)
	}
	if strings.Contains(result, "/h") {
		t.Errorf("should not show burn rate before 60 seconds, got: %s", result)
	}
}

func TestRenderCost_NoBurnRateWithoutSessionID(t *testing.T) {
	sl := &StatusLine{
		input: Input{
			SessionID: "",
			Cost:      CostInfo{TotalCostUSD: 3.00},
		},
	}

	result := sl.renderCost()

	if !strings.Contains(result, "$3.00") {
		t.Errorf("expected cost '$3.00' in output, got: %s", result)
	}
	if strings.Contains(result, "/h") {
		t.Errorf("should not show burn rate without session ID, got: %s", result)
	}
}

func TestRenderCost_FirstCallCreatesSnapshot(t *testing.T) {
	sessionID := "test-burn-first-" + time.Now().Format("20060102150405")
	path := burnrate.FilePath(sessionID)
	defer os.Remove(path)

	sl := &StatusLine{
		input: Input{
			SessionID: sessionID,
			Cost:      CostInfo{TotalCostUSD: 2.00},
		},
	}

	result := sl.renderCost()

	// First call - should create snapshot but not show burn rate
	if !strings.Contains(result, "$2.00") {
		t.Errorf("expected cost '$2.00' in output, got: %s", result)
	}
	if strings.Contains(result, "/h") {
		t.Errorf("should not show burn rate on first call, got: %s", result)
	}

	// Verify snapshot was created
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("snapshot file should have been created")
	}
}
