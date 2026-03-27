package plugins

import (
	"testing"
)

func TestFormatCountdown(t *testing.T) {
	tests := []struct {
		name     string
		minutes  int
		expected string
	}{
		{name: "hours and minutes", minutes: 158, expected: "2h38m"},
		{name: "exactly one hour", minutes: 60, expected: "1h0m"},
		{name: "just under an hour", minutes: 59, expected: "59m"},
		{name: "one minute", minutes: 1, expected: "1m"},
		{name: "zero minutes", minutes: 0, expected: "<1m"},
		{name: "negative minutes", minutes: -5, expected: "<1m"},
		{name: "large value", minutes: 600, expected: "10h0m"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatCountdown(tt.minutes)
			if result != tt.expected {
				t.Errorf("formatCountdown(%d) = %q, want %q", tt.minutes, result, tt.expected)
			}
		})
	}
}
