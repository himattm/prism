package plugins

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/himattm/prism/internal/cache"
	"github.com/himattm/prism/internal/plugin"
)

func TestUsagePlugin_Name(t *testing.T) {
	p := &UsagePlugin{}
	if p.Name() != "usage" {
		t.Errorf("expected name 'usage', got '%s'", p.Name())
	}
}

func TestUsageBarsPlugin_Name(t *testing.T) {
	p := &UsageBarsPlugin{}
	if p.Name() != "usage_bars" {
		t.Errorf("expected name 'usage_bars', got '%s'", p.Name())
	}
}

func TestUsageTextPlugin_Name(t *testing.T) {
	p := &UsageTextPlugin{}
	if p.Name() != "usage_text" {
		t.Errorf("expected name 'usage_text', got '%s'", p.Name())
	}
}

func TestParseUsageConfig(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]any
		expected usageConfig
	}{
		{
			name:  "empty config uses defaults",
			input: map[string]any{},
			expected: usageConfig{
				style:        "text",
				showHours:    true,
				showDays:     true,
				showOpus:     true,
				costDecimals: 2,
				costColor:    "gray",
			},
		},
		{
			name: "usage_plan style option",
			input: map[string]any{
				"usage": map[string]any{
					"usage_plan": map[string]any{
						"style": "bars",
					},
				},
			},
			expected: usageConfig{
				style:        "bars",
				showHours:    true,
				showDays:     true,
				showOpus:     true,
				costDecimals: 2,
				costColor:    "gray",
			},
		},
		{
			name: "usage_plan hide options",
			input: map[string]any{
				"usage": map[string]any{
					"usage_plan": map[string]any{
						"show_hours": false,
						"show_days":  false,
						"show_opus":  false,
					},
				},
			},
			expected: usageConfig{
				style:        "text",
				showHours:    false,
				showDays:     false,
				showOpus:     false,
				costDecimals: 2,
				costColor:    "gray",
			},
		},
		{
			name: "api_billing options",
			input: map[string]any{
				"usage": map[string]any{
					"api_billing": map[string]any{
						"decimals": float64(4),
						"color":    "cyan",
					},
				},
			},
			expected: usageConfig{
				style:        "text",
				showHours:    true,
				showDays:     true,
				showOpus:     true,
				costDecimals: 4,
				costColor:    "cyan",
			},
		},
		{
			name: "both subsections",
			input: map[string]any{
				"usage": map[string]any{
					"usage_plan": map[string]any{
						"style":     "bars",
						"show_opus": false,
					},
					"api_billing": map[string]any{
						"decimals": float64(3),
					},
				},
			},
			expected: usageConfig{
				style:        "bars",
				showHours:    true,
				showDays:     true,
				showOpus:     false,
				costDecimals: 3,
				costColor:    "gray",
			},
		},
	}

	p := &UsagePlugin{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := plugin.Input{Config: tt.input}
			result := p.parseConfig(input)

			if result.style != tt.expected.style {
				t.Errorf("style: expected %s, got %s", tt.expected.style, result.style)
			}
			if result.showHours != tt.expected.showHours {
				t.Errorf("showHours: expected %v, got %v", tt.expected.showHours, result.showHours)
			}
			if result.showDays != tt.expected.showDays {
				t.Errorf("showDays: expected %v, got %v", tt.expected.showDays, result.showDays)
			}
			if result.showOpus != tt.expected.showOpus {
				t.Errorf("showOpus: expected %v, got %v", tt.expected.showOpus, result.showOpus)
			}
			if result.costDecimals != tt.expected.costDecimals {
				t.Errorf("costDecimals: expected %d, got %d", tt.expected.costDecimals, result.costDecimals)
			}
			if result.costColor != tt.expected.costColor {
				t.Errorf("costColor: expected %s, got %s", tt.expected.costColor, result.costColor)
			}
		})
	}
}

func TestUtilizationToBarLevel(t *testing.T) {
	tests := []struct {
		utilization float64
		expected    int
	}{
		{0, 0},
		{12.5, 1},
		{25, 2},
		{37.5, 3},
		{50, 4},
		{62.5, 5},
		{75, 6},
		{87.5, 7},
		{100, 7},
		{-10, 0}, // negative clamped to 0
		{150, 7}, // over 100 clamped to 7
	}

	for _, tt := range tests {
		result := UtilizationToBarLevel(tt.utilization)
		if result != tt.expected {
			t.Errorf("UtilizationToBarLevel(%v): expected %d, got %d", tt.utilization, tt.expected, result)
		}
	}
}

func TestTimeToBarLevel(t *testing.T) {
	tests := []struct {
		duration    time.Duration
		maxDuration time.Duration
		expected    int
	}{
		{0, 5 * time.Hour, 0},
		{1 * time.Hour, 5 * time.Hour, 1},
		{2 * time.Hour, 5 * time.Hour, 3},
		{3 * time.Hour, 5 * time.Hour, 4},
		{5 * time.Hour, 5 * time.Hour, 7},
		{10 * time.Hour, 5 * time.Hour, 7},          // over max clamped
		{-1 * time.Hour, 5 * time.Hour, 0},          // negative clamped
		{3 * 24 * time.Hour, 7 * 24 * time.Hour, 3}, // 3 days of 7
	}

	for _, tt := range tests {
		result := TimeToBarLevel(tt.duration, tt.maxDuration)
		if result != tt.expected {
			t.Errorf("TimeToBarLevel(%v, %v): expected %d, got %d", tt.duration, tt.maxDuration, tt.expected, result)
		}
	}
}

func TestLevelToBarChar(t *testing.T) {
	expected := []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

	for level := 0; level <= 7; level++ {
		result := LevelToBarChar(level)
		if result != expected[level] {
			t.Errorf("LevelToBarChar(%d): expected %c, got %c", level, expected[level], result)
		}
	}

	// Test clamping
	if LevelToBarChar(-1) != '▁' {
		t.Error("LevelToBarChar(-1) should clamp to level 0")
	}
	if LevelToBarChar(10) != '█' {
		t.Error("LevelToBarChar(10) should clamp to level 7")
	}
}

func TestFormatTimeRemaining(t *testing.T) {
	tests := []struct {
		duration    time.Duration
		useDays     bool
		showMinutes bool
		expected    string
	}{
		// Hours format with minutes enabled
		{4*time.Hour + 30*time.Minute, false, true, "4h30m"},
		{2*time.Hour + 10*time.Minute, false, true, "2h10m"},
		{45 * time.Minute, false, true, "0h45m"},
		{10 * time.Minute, false, true, "0h10m"},
		{6 * time.Hour, false, true, "5h"},  // capped at 5h, no remaining minutes
		{0, false, true, "0h"},              // zero duration
		{-1 * time.Hour, false, true, "0h"}, // negative treated as 0

		// Hours format with minutes disabled
		{4*time.Hour + 30*time.Minute, false, false, "4h"},
		{2*time.Hour + 10*time.Minute, false, false, "2h"},
		{45 * time.Minute, false, false, "0h"},
		{6 * time.Hour, false, false, "5h"}, // capped at 5h

		// Days format (showMinutes is ignored)
		{6*24*time.Hour + 12*time.Hour, true, false, "7d"},
		{3*24*time.Hour + 1*time.Hour, true, false, "4d"},
		{20 * time.Hour, true, false, "1d"},
		{8 * 24 * time.Hour, true, false, "7d"}, // capped at 7d
		{0, true, false, "1d"},
	}

	for _, tt := range tests {
		result := FormatTimeRemaining(tt.duration, tt.useDays, tt.showMinutes)
		if result != tt.expected {
			t.Errorf("FormatTimeRemaining(%v, useDays=%v, showMinutes=%v): expected %s, got %s",
				tt.duration, tt.useDays, tt.showMinutes, tt.expected, result)
		}
	}
}

func TestGetUsageColor(t *testing.T) {
	white := "WHITE"
	yellow := "YELLOW"
	red := "RED"

	tests := []struct {
		utilization float64
		expected    string
	}{
		{0, white},
		{50, white},
		{69, white},
		{70, yellow},
		{80, yellow},
		{89, yellow},
		{90, red},
		{95, red},
		{100, red},
	}

	for _, tt := range tests {
		result := getUsageColor(tt.utilization, white, yellow, red)
		if result != tt.expected {
			t.Errorf("getUsageColor(%v): expected %s, got %s", tt.utilization, tt.expected, result)
		}
	}
}

func TestUsagePlugin_RenderCost(t *testing.T) {
	p := &UsagePlugin{}
	p.SetCache(cache.New())

	input := plugin.Input{
		Session: plugin.SessionContext{
			CostUSD: 1.2345,
		},
		Colors: map[string]string{
			"gray":  "\033[90m",
			"cyan":  "\033[36m",
			"reset": "\033[0m",
		},
	}

	tests := []struct {
		name     string
		cfg      usageConfig
		expected string
	}{
		{
			name: "default 2 decimals gray",
			cfg: usageConfig{
				costDecimals: 2,
				costColor:    "gray",
			},
			expected: "\033[90m$1.23\033[0m",
		},
		{
			name: "4 decimals",
			cfg: usageConfig{
				costDecimals: 4,
				costColor:    "gray",
			},
			expected: "\033[90m$1.2345\033[0m",
		},
		{
			name: "0 decimals",
			cfg: usageConfig{
				costDecimals: 0,
				costColor:    "gray",
			},
			expected: "\033[90m$1\033[0m",
		},
		{
			name: "cyan color",
			cfg: usageConfig{
				costDecimals: 2,
				costColor:    "cyan",
			},
			expected: "\033[36m$1.23\033[0m",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := p.renderCost(input, tt.cfg)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestUsagePlugin_Execute_APIBilling(t *testing.T) {
	// This test verifies that without OAuth, the plugin falls back to cost display
	p := &UsagePlugin{}
	p.SetCache(cache.New())

	// Pre-cache that we don't have OAuth (simulating API billing user)
	p.cache.Set("has_oauth", "false", 5*time.Minute)

	ctx := context.Background()
	input := plugin.Input{
		Prism: plugin.PrismContext{
			IsIdle: false,
		},
		Session: plugin.SessionContext{
			CostUSD: 2.50,
		},
		Config: map[string]any{},
		Colors: map[string]string{
			"gray":  "\033[90m",
			"reset": "\033[0m",
		},
	}

	result, err := p.Execute(ctx, input)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	expected := "\033[90m$2.50\033[0m"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestUsageDiskCache_RoundTrip(t *testing.T) {
	path := filepath.Join(os.TempDir(), usageDiskCacheFile)
	defer os.Remove(path)

	// Save usage data to disk
	usage := &UsageResponse{
		FiveHour: &UsageLimit{Utilization: 25.0, ResetsAt: "2026-01-01T05:00:00Z"},
		SevenDay: &UsageLimit{Utilization: 10.0, ResetsAt: "2026-01-07T00:00:00Z"},
	}
	saveUsageCache(usage)

	// Load it back
	loaded, ok := loadUsageCache()
	if !ok {
		t.Fatal("expected to load cached usage data")
	}
	if loaded.FiveHour == nil || loaded.FiveHour.Utilization != 25.0 {
		t.Errorf("expected FiveHour.Utilization=25, got %v", loaded.FiveHour)
	}
	if loaded.SevenDay == nil || loaded.SevenDay.Utilization != 10.0 {
		t.Errorf("expected SevenDay.Utilization=10, got %v", loaded.SevenDay)
	}
	if loaded.SevenDayOpus != nil {
		t.Errorf("expected SevenDayOpus=nil, got %v", loaded.SevenDayOpus)
	}
}

func TestUsageDiskCache_LoadMissing(t *testing.T) {
	// Remove any existing cache file
	os.Remove(filepath.Join(os.TempDir(), usageDiskCacheFile))

	_, ok := loadUsageCache()
	if ok {
		t.Error("expected load to fail when no cache file exists")
	}
}

func TestUsageDiskCache_SaveNil(t *testing.T) {
	// Remove any pre-existing cache file so we can verify nil doesn't create one
	path := filepath.Join(os.TempDir(), usageDiskCacheFile)
	os.Remove(path)

	// Should not panic or create a file
	saveUsageCache(nil)

	if _, err := os.Stat(path); err == nil {
		t.Error("saveUsageCache(nil) should not create a cache file")
	}
}

func TestUsageDiskCache_Expired(t *testing.T) {
	path := filepath.Join(os.TempDir(), usageDiskCacheFile)
	defer os.Remove(path)

	// Write valid cache data
	usage := &UsageResponse{
		FiveHour: &UsageLimit{Utilization: 50.0, ResetsAt: "2026-01-01T05:00:00Z"},
	}
	saveUsageCache(usage)

	// Backdate the file's modification time beyond the TTL
	old := time.Now().Add(-(usageDiskCacheTTL + time.Minute))
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatalf("failed to backdate cache file: %v", err)
	}

	// loadUsageCache should reject the stale file
	_, ok := loadUsageCache()
	if ok {
		t.Error("expected loadUsageCache to reject expired cache file")
	}
}

func TestTimeUntilReset(t *testing.T) {
	// Test with a future time
	futureTime := time.Now().Add(2 * time.Hour).Format(time.RFC3339)
	duration, err := TimeUntilReset(futureTime)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	// Should be approximately 2 hours (allow some slack for test execution)
	if duration < 1*time.Hour+59*time.Minute || duration > 2*time.Hour+1*time.Minute {
		t.Errorf("expected ~2h, got %v", duration)
	}

	// Test with invalid format
	_, err = TimeUntilReset("invalid")
	if err == nil {
		t.Error("expected error for invalid time format")
	}
}
