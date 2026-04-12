package harness

import (
	"testing"
)

func TestCodexParseInput(t *testing.T) {
	raw := `{
		"session_id": "codex-session-1",
		"cwd": "/home/user/project",
		"model": "o3-pro",
		"context_window_percent": 45.0,
		"context_window_used_tokens": 90000,
		"context_window_size": 200000,
		"lines_added": 200,
		"lines_removed": 75,
		"rate_limits": {
			"5h_limit_percent": 60.0,
			"7d_limit_percent": 30.0
		}
	}`

	h := Get("codex")
	input, err := h.ParseInput([]byte(raw))
	if err != nil {
		t.Fatalf("ParseInput failed: %v", err)
	}

	if input.SessionID != "codex-session-1" {
		t.Errorf("SessionID: expected 'codex-session-1', got %q", input.SessionID)
	}
	if input.Model.DisplayName != "o3-pro" {
		t.Errorf("Model.DisplayName: expected 'o3-pro', got %q", input.Model.DisplayName)
	}
	if input.Workspace.ProjectDir != "/home/user/project" {
		t.Errorf("Workspace.ProjectDir: expected '/home/user/project', got %q", input.Workspace.ProjectDir)
	}
	if input.Workspace.CurrentDir != "/home/user/project" {
		t.Errorf("Workspace.CurrentDir: expected '/home/user/project', got %q", input.Workspace.CurrentDir)
	}
	if input.Cost.TotalLinesAdded != 200 {
		t.Errorf("Cost.TotalLinesAdded: expected 200, got %d", input.Cost.TotalLinesAdded)
	}
	if input.Cost.TotalLinesRemoved != 75 {
		t.Errorf("Cost.TotalLinesRemoved: expected 75, got %d", input.Cost.TotalLinesRemoved)
	}
	if input.Context.UsedPercentage != 45.0 {
		t.Errorf("Context.UsedPercentage: expected 45.0, got %f", input.Context.UsedPercentage)
	}
	if input.Context.RemainingPercentage != 55.0 {
		t.Errorf("Context.RemainingPercentage: expected 55.0, got %f", input.Context.RemainingPercentage)
	}
	if input.Context.ContextWindow != 200000 {
		t.Errorf("Context.ContextWindow: expected 200000, got %d", input.Context.ContextWindow)
	}
	if input.Context.CurrentUsage.InputTokens != 90000 {
		t.Errorf("Context.CurrentUsage.InputTokens: expected 90000, got %d", input.Context.CurrentUsage.InputTokens)
	}
}

func TestCodexParseInputDerivePercentFromTokens(t *testing.T) {
	// When context_window_percent is 0 but tokens are provided, derive percentage
	raw := `{
		"session_id": "test",
		"cwd": "/tmp",
		"model": "o3",
		"context_window_percent": 0,
		"context_window_used_tokens": 100000,
		"context_window_size": 200000
	}`

	h := Get("codex")
	input, err := h.ParseInput([]byte(raw))
	if err != nil {
		t.Fatalf("ParseInput failed: %v", err)
	}

	if input.Context.UsedPercentage != 50.0 {
		t.Errorf("expected derived UsedPercentage of 50.0, got %f", input.Context.UsedPercentage)
	}
}

func TestCodexParseInputMinimalFields(t *testing.T) {
	// Only session_id and model — everything else should default gracefully
	raw := `{
		"session_id": "minimal",
		"model": "gpt-4.1"
	}`

	h := Get("codex")
	input, err := h.ParseInput([]byte(raw))
	if err != nil {
		t.Fatalf("ParseInput failed: %v", err)
	}

	if input.SessionID != "minimal" {
		t.Errorf("SessionID: expected 'minimal', got %q", input.SessionID)
	}
	if input.Model.DisplayName != "gpt-4.1" {
		t.Errorf("Model: expected 'gpt-4.1', got %q", input.Model.DisplayName)
	}
	if input.Context.ContextWindow != 200000 {
		t.Errorf("expected default ContextWindow 200000, got %d", input.Context.ContextWindow)
	}
	if input.Cost.TotalCostUSD != 0 {
		t.Errorf("expected zero cost for Codex, got %f", input.Cost.TotalCostUSD)
	}
}

func TestCodexParseInputIgnoresUnknownFields(t *testing.T) {
	// Forward compatibility: unknown fields should be silently ignored
	raw := `{
		"session_id": "test",
		"model": "o3",
		"cwd": "/tmp",
		"some_future_field": "value",
		"another_field": 42
	}`

	h := Get("codex")
	input, err := h.ParseInput([]byte(raw))
	if err != nil {
		t.Fatalf("ParseInput should not fail on unknown fields: %v", err)
	}

	if input.SessionID != "test" {
		t.Errorf("SessionID: expected 'test', got %q", input.SessionID)
	}
}

func TestCodexParseInputInvalid(t *testing.T) {
	h := Get("codex")
	_, err := h.ParseInput([]byte("not json"))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestCodexDefaultSectionsNoCost(t *testing.T) {
	h := Get("codex")
	sections := h.DefaultSections()

	for _, line := range sections {
		for _, section := range line {
			if section == "cost" || section == "usage" {
				t.Errorf("Codex default sections should not include %q (subscription-based)", section)
			}
		}
	}
}
