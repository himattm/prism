package plugins

import (
	"testing"
	"time"
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

func TestLocalPeakStatus(t *testing.T) {
	loc, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		t.Fatalf("failed to load timezone: %v", err)
	}

	tests := []struct {
		name          string
		time          time.Time
		expectPeak    bool
		expectWeekend bool
		expectMinutes int
	}{
		{
			name:          "weekday peak start",
			time:          time.Date(2026, 3, 27, 5, 0, 0, 0, loc),
			expectPeak:    true,
			expectWeekend: false,
			expectMinutes: 360,
		},
		{
			name:          "weekday mid-peak",
			time:          time.Date(2026, 3, 27, 8, 0, 0, 0, loc),
			expectPeak:    true,
			expectWeekend: false,
			expectMinutes: 180,
		},
		{
			name:          "weekday peak end boundary",
			time:          time.Date(2026, 3, 27, 10, 59, 0, 0, loc),
			expectPeak:    true,
			expectWeekend: false,
			expectMinutes: 1,
		},
		{
			name:          "weekday just after peak",
			time:          time.Date(2026, 3, 27, 11, 0, 0, 0, loc),
			expectPeak:    false,
			expectWeekend: false,
			expectMinutes: 3960,
		},
		{
			name:          "weekday evening",
			time:          time.Date(2026, 3, 26, 20, 0, 0, 0, loc),
			expectPeak:    false,
			expectWeekend: false,
			expectMinutes: 540,
		},
		{
			name:          "weekday before peak",
			time:          time.Date(2026, 3, 27, 3, 0, 0, 0, loc),
			expectPeak:    false,
			expectWeekend: false,
			expectMinutes: 120,
		},
		{
			name:          "saturday",
			time:          time.Date(2026, 3, 28, 8, 0, 0, 0, loc),
			expectPeak:    false,
			expectWeekend: true,
			expectMinutes: 0,
		},
		{
			name:          "sunday",
			time:          time.Date(2026, 3, 29, 14, 0, 0, 0, loc),
			expectPeak:    false,
			expectWeekend: true,
			expectMinutes: 0,
		},
		{
			name:          "friday after peak",
			time:          time.Date(2026, 3, 27, 15, 0, 0, 0, loc),
			expectPeak:    false,
			expectWeekend: false,
			expectMinutes: 3720,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isPeak, isWeekend, minutesUntil := localPeakStatus(tt.time)
			if isPeak != tt.expectPeak {
				t.Errorf("isPeak: got %v, want %v", isPeak, tt.expectPeak)
			}
			if isWeekend != tt.expectWeekend {
				t.Errorf("isWeekend: got %v, want %v", isWeekend, tt.expectWeekend)
			}
			if !tt.expectWeekend && minutesUntil != tt.expectMinutes {
				t.Errorf("minutesUntil: got %d, want %d", minutesUntil, tt.expectMinutes)
			}
		})
	}
}
