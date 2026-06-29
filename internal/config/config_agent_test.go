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
