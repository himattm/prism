package harness

import (
	"encoding/json"
	"testing"

	"github.com/himattm/prism/internal/statusline"
)

func TestClaudeParseInput(t *testing.T) {
	raw := `{
		"session_id": "abc123",
		"model": {"display_name": "Opus 4.6 (1M context)"},
		"workspace": {"project_dir": "/home/user/project", "current_dir": "/home/user/project/src"},
		"cost": {"total_cost_usd": 1.23, "total_lines_added": 100, "total_lines_removed": 50},
		"context_window": {
			"current_usage": {
				"input_tokens": 50000,
				"output_tokens": 10000,
				"cache_creation_input_tokens": 5000,
				"cache_read_input_tokens": 20000
			},
			"context_window_size": 200000,
			"used_percentage": 42.5,
			"remaining_percentage": 57.5
		}
	}`

	h := Get("claude")
	input, err := h.ParseInput([]byte(raw))
	if err != nil {
		t.Fatalf("ParseInput failed: %v", err)
	}

	if input.SessionID != "abc123" {
		t.Errorf("SessionID: expected 'abc123', got %q", input.SessionID)
	}
	if input.Model.DisplayName != "Opus 4.6 (1M context)" {
		t.Errorf("Model.DisplayName: expected 'Opus 4.6 (1M context)', got %q", input.Model.DisplayName)
	}
	if input.Workspace.ProjectDir != "/home/user/project" {
		t.Errorf("Workspace.ProjectDir: expected '/home/user/project', got %q", input.Workspace.ProjectDir)
	}
	if input.Workspace.CurrentDir != "/home/user/project/src" {
		t.Errorf("Workspace.CurrentDir: expected '/home/user/project/src', got %q", input.Workspace.CurrentDir)
	}
	if input.Cost.TotalCostUSD != 1.23 {
		t.Errorf("Cost.TotalCostUSD: expected 1.23, got %f", input.Cost.TotalCostUSD)
	}
	if input.Cost.TotalLinesAdded != 100 {
		t.Errorf("Cost.TotalLinesAdded: expected 100, got %d", input.Cost.TotalLinesAdded)
	}
	if input.Context.ContextWindow != 200000 {
		t.Errorf("Context.ContextWindow: expected 200000, got %d", input.Context.ContextWindow)
	}
	if input.Context.UsedPercentage != 42.5 {
		t.Errorf("Context.UsedPercentage: expected 42.5, got %f", input.Context.UsedPercentage)
	}
	if input.Context.CurrentUsage.InputTokens != 50000 {
		t.Errorf("Context.CurrentUsage.InputTokens: expected 50000, got %d", input.Context.CurrentUsage.InputTokens)
	}
}

func TestClaudeParseInputRoundTrip(t *testing.T) {
	// Verify that parsing then re-marshaling produces the same structure
	original := statusline.Input{
		SessionID: "test-session",
		Model:     statusline.ModelInfo{DisplayName: "Sonnet 4.6"},
		Workspace: statusline.WorkspaceInfo{ProjectDir: "/tmp", CurrentDir: "/tmp/sub"},
		Cost:      statusline.CostInfo{TotalCostUSD: 0.50},
		Context: statusline.ContextInfo{
			ContextWindow:  200000,
			UsedPercentage: 30.0,
		},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	h := Get("claude")
	parsed, err := h.ParseInput(data)
	if err != nil {
		t.Fatalf("ParseInput failed: %v", err)
	}

	if parsed.SessionID != original.SessionID {
		t.Errorf("SessionID mismatch: %q vs %q", parsed.SessionID, original.SessionID)
	}
	if parsed.Model.DisplayName != original.Model.DisplayName {
		t.Errorf("Model mismatch: %q vs %q", parsed.Model.DisplayName, original.Model.DisplayName)
	}
	if parsed.Cost.TotalCostUSD != original.Cost.TotalCostUSD {
		t.Errorf("Cost mismatch: %f vs %f", parsed.Cost.TotalCostUSD, original.Cost.TotalCostUSD)
	}
}

func TestClaudeParseInputInvalid(t *testing.T) {
	h := Get("claude")
	_, err := h.ParseInput([]byte("not json"))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}
