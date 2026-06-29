package config

import "testing"

func TestDefaultSectionLinesForAgent(t *testing.T) {
	// Pi layout must not contain Claude-only sections.
	piExcluded := map[string]bool{"usage": true, "cost": true, "peakhours": true}
	pi := DefaultSectionLinesForAgent("pi")
	for _, line := range pi {
		for _, sec := range line {
			if piExcluded[sec] {
				t.Errorf("pi default layout contains Claude-only section %q", sec)
			}
		}
	}

	// Pi layout should still contain the core agent-agnostic sections.
	if !containsSection(pi, "dir") || !containsSection(pi, "model") ||
		!containsSection(pi, "context") || !containsSection(pi, "git") {
		t.Errorf("pi default layout missing a core section: %v", pi)
	}

	// The agent match is case-insensitive.
	for _, variant := range []string{"Pi", "PI"} {
		if containsSection(DefaultSectionLinesForAgent(variant), "usage") {
			t.Errorf("agent %q should map to the pi layout (no usage)", variant)
		}
	}

	// Empty / claude-code agents get the standard default.
	for _, agent := range []string{"", "claude-code"} {
		got := DefaultSectionLinesForAgent(agent)
		if !containsSection(got, "usage") {
			t.Errorf("agent %q should use the standard default (with usage), got %v", agent, got)
		}
	}
}

func TestFilterSectionsForAgent(t *testing.T) {
	configured := [][]string{
		{"dir", "model", "context", "usage", "peakhours", "git"},
		{"cost"}, // a whole line of Claude-only sections should be dropped
		{"spotify"},
	}

	// Claude Code (and unknown agents) get the layout unchanged.
	for _, agent := range []string{"", "claude-code"} {
		got := FilterSectionsForAgent(agent, configured)
		if len(got) != len(configured) || !containsSection(got, "usage") || !containsSection(got, "cost") {
			t.Errorf("agent %q should be unchanged, got %v", agent, got)
		}
	}

	// Pi drops Claude-only sections even from an explicit config, and removes the
	// now-empty line.
	pi := FilterSectionsForAgent("pi", configured)
	for _, sec := range []string{"usage", "cost", "peakhours"} {
		if containsSection(pi, sec) {
			t.Errorf("pi layout should not contain %q: %v", sec, pi)
		}
	}
	if len(pi) != 2 { // the {"cost"} line collapses away entirely
		t.Errorf("expected 2 lines after filtering, got %d: %v", len(pi), pi)
	}
	if !containsSection(pi, "git") || !containsSection(pi, "spotify") {
		t.Errorf("pi layout dropped a non-Claude section: %v", pi)
	}
}

func containsSection(lines [][]string, name string) bool {
	for _, line := range lines {
		for _, sec := range line {
			if sec == name {
				return true
			}
		}
	}
	return false
}
