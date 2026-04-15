package plugins

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/himattm/prism/internal/cache"
	"github.com/himattm/prism/internal/plugin"
	"github.com/himattm/prism/internal/tokens"
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
				showBurnRate: true,
				showCache:    true,
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
				showBurnRate: true,
				showCache:    true,
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
				showBurnRate: true,
				showCache:    true,
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
				showBurnRate: true,
				showCache:    true,
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
				showBurnRate: true,
				showCache:    true,
			},
		},
		{
			name: "show_burn_rate disabled",
			input: map[string]any{
				"usage": map[string]any{
					"api_billing": map[string]any{
						"show_burn_rate": false,
					},
				},
			},
			expected: usageConfig{
				style:        "text",
				showHours:    true,
				showDays:     true,
				showOpus:     true,
				costDecimals: 2,
				costColor:    "gray",
				showBurnRate: false,
				showCache:    true,
			},
		},
		{
			name: "negative decimals clamped to 0",
			input: map[string]any{
				"usage": map[string]any{
					"api_billing": map[string]any{
						"decimals": float64(-1),
					},
				},
			},
			expected: usageConfig{
				style:        "text",
				showHours:    true,
				showDays:     true,
				showOpus:     true,
				costDecimals: 0,
				costColor:    "gray",
				showBurnRate: true,
				showCache:    true,
			},
		},
		{
			name: "large decimals clamped to 10",
			input: map[string]any{
				"usage": map[string]any{
					"api_billing": map[string]any{
						"decimals": float64(99),
					},
				},
			},
			expected: usageConfig{
				style:        "text",
				showHours:    true,
				showDays:     true,
				showOpus:     true,
				costDecimals: 10,
				costColor:    "gray",
				showBurnRate: true,
				showCache:    true,
			},
		},
		{
			name: "show_cache disabled",
			input: map[string]any{
				"usage": map[string]any{
					"api_billing": map[string]any{
						"show_cache": false,
					},
				},
			},
			expected: usageConfig{
				style:        "text",
				showHours:    true,
				showDays:     true,
				showOpus:     true,
				costDecimals: 2,
				costColor:    "gray",
				showBurnRate: true,
				showCache:    false,
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
			if result.showBurnRate != tt.expected.showBurnRate {
				t.Errorf("showBurnRate: expected %v, got %v", tt.expected.showBurnRate, result.showBurnRate)
			}
			if result.showCache != tt.expected.showCache {
				t.Errorf("showCache: expected %v, got %v", tt.expected.showCache, result.showCache)
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
		{45 * time.Minute, false, true, "45m"},
		{10 * time.Minute, false, true, "10m"},
		{6 * time.Hour, false, true, "5h"},  // capped at 5h, no remaining minutes
		{0, false, true, "0m"},              // zero duration
		{-1 * time.Hour, false, true, "0m"}, // negative treated as 0

		// Hours format with minutes disabled
		{4*time.Hour + 30*time.Minute, false, false, "4h"},
		{2*time.Hour + 10*time.Minute, false, false, "2h"},
		{45 * time.Minute, false, false, "45m"},
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
		result := GetUsageColor(tt.utilization, white, yellow, red)
		if result != tt.expected {
			t.Errorf("GetUsageColor(%v): expected %s, got %s", tt.utilization, tt.expected, result)
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

func TestUsagePlugin_RenderCost_WithCacheIndicator(t *testing.T) {
	p := &UsagePlugin{}
	p.SetCache(cache.New())

	input := plugin.Input{
		Session: plugin.SessionContext{
			CostUSD:             1.23,
			InputTokens:         5000,
			CacheCreationTokens: 2000,
			CacheReadTokens:     3000,
		},
		Colors: map[string]string{
			"gray":  "\033[90m",
			"reset": "\033[0m",
		},
	}

	// showCache enabled (default) - should show indicator
	cfg := usageConfig{
		costDecimals: 2,
		costColor:    "gray",
		showCache:    true,
	}
	result := p.renderCost(input, cfg)
	// 3000 / (5000+2000+3000) = 30%
	expected := "\033[90m$1.23 ⌁30%\033[0m"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestUsagePlugin_RenderCost_CacheDisabledByConfig(t *testing.T) {
	p := &UsagePlugin{}
	p.SetCache(cache.New())

	input := plugin.Input{
		Session: plugin.SessionContext{
			CostUSD:             1.23,
			InputTokens:         5000,
			CacheCreationTokens: 2000,
			CacheReadTokens:     3000,
		},
		Colors: map[string]string{
			"gray":  "\033[90m",
			"reset": "\033[0m",
		},
	}

	// showCache disabled - should NOT show indicator
	cfg := usageConfig{
		costDecimals: 2,
		costColor:    "gray",
		showCache:    false,
	}
	result := p.renderCost(input, cfg)
	expected := "\033[90m$1.23\033[0m"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestUsagePlugin_RenderCost_NoCacheWhenZeroReads(t *testing.T) {
	p := &UsagePlugin{}
	p.SetCache(cache.New())

	input := plugin.Input{
		Session: plugin.SessionContext{
			CostUSD:             2.50,
			InputTokens:         10000,
			CacheCreationTokens: 5000,
			CacheReadTokens:     0,
		},
		Colors: map[string]string{
			"gray":  "\033[90m",
			"reset": "\033[0m",
		},
	}

	cfg := usageConfig{
		costDecimals: 2,
		costColor:    "gray",
		showCache:    true,
	}
	result := p.renderCost(input, cfg)
	expected := "\033[90m$2.50\033[0m"
	if result != expected {
		t.Errorf("expected %q (no cache indicator), got %q", expected, result)
	}
}

func TestCacheRatio(t *testing.T) {
	tests := []struct {
		name                  string
		input, creation, read int
		expectedRatio         int
		expectedOK            bool
	}{
		{"78% cache hits", 1000, 1200, 7800, 78, true},
		{"30% cache hits", 5000, 2000, 3000, 30, true},
		{"100% cache reads", 0, 0, 10000, 100, true},
		{"zero cache reads", 10000, 5000, 0, 0, false},
		{"all zeroes", 0, 0, 0, 0, false},
		{"negative cache reads", 10000, 0, -1, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ratio, ok := tokens.CacheEfficiency(tt.input, tt.creation, tt.read)
			if ok != tt.expectedOK {
				t.Errorf("CacheEfficiency(%d, %d, %d) ok = %v, want %v", tt.input, tt.creation, tt.read, ok, tt.expectedOK)
			}
			if ok && ratio != tt.expectedRatio {
				t.Errorf("CacheEfficiency(%d, %d, %d) = %d, want %d", tt.input, tt.creation, tt.read, ratio, tt.expectedRatio)
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

	// Load it back — freshly written file should not be stale
	loaded, stale, ok := loadUsageCache()
	if !ok {
		t.Fatal("expected to load cached usage data")
	}
	if stale {
		t.Error("expected freshly written cache to not be stale")
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

	data, stale, ok := loadUsageCache()
	if ok {
		t.Error("expected load to fail when no cache file exists")
	}
	if stale {
		t.Error("expected stale=false when file is missing")
	}
	if data != nil {
		t.Error("expected data=nil when file is missing")
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

	// loadUsageCache should return the data marked as stale
	loaded, stale, ok := loadUsageCache()
	if !ok {
		t.Fatal("expected loadUsageCache to return expired data with ok=true")
	}
	if !stale {
		t.Error("expected stale=true for expired cache file")
	}
	if loaded.FiveHour == nil || loaded.FiveHour.Utilization != 50.0 {
		t.Errorf("expected FiveHour.Utilization=50, got %v", loaded.FiveHour)
	}
}

func TestUsageDiskCache_Stale(t *testing.T) {
	path := filepath.Join(os.TempDir(), usageDiskCacheFile)
	defer os.Remove(path)

	usage := &UsageResponse{
		FiveHour:     &UsageLimit{Utilization: 30.0, ResetsAt: "2026-01-01T05:00:00Z"},
		SevenDay:     &UsageLimit{Utilization: 15.0, ResetsAt: "2026-01-07T00:00:00Z"},
		SevenDayOpus: &UsageLimit{Utilization: 5.0, ResetsAt: "2026-01-07T00:00:00Z"},
	}
	saveUsageCache(usage)

	// Backdate to just past the TTL
	old := time.Now().Add(-(usageDiskCacheTTL + 10*time.Second))
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatalf("failed to backdate cache file: %v", err)
	}

	loaded, stale, ok := loadUsageCache()
	if !ok {
		t.Fatal("expected ok=true for stale but readable file")
	}
	if !stale {
		t.Error("expected stale=true for expired file")
	}
	if loaded.FiveHour == nil || loaded.FiveHour.Utilization != 30.0 {
		t.Errorf("expected FiveHour.Utilization=30, got %v", loaded.FiveHour)
	}
	if loaded.SevenDay == nil || loaded.SevenDay.Utilization != 15.0 {
		t.Errorf("expected SevenDay.Utilization=15, got %v", loaded.SevenDay)
	}
	if loaded.SevenDayOpus == nil || loaded.SevenDayOpus.Utilization != 5.0 {
		t.Errorf("expected SevenDayOpus.Utilization=5, got %v", loaded.SevenDayOpus)
	}
}

func TestRenderStale(t *testing.T) {
	p := &UsagePlugin{}
	p.SetCache(cache.New())

	usage := &UsageResponse{
		FiveHour: &UsageLimit{Utilization: 45.0, ResetsAt: time.Now().Add(3 * time.Hour).Format(time.RFC3339)},
		SevenDay: &UsageLimit{Utilization: 20.0, ResetsAt: time.Now().Add(5 * 24 * time.Hour).Format(time.RFC3339)},
	}

	darkGray := "\033[90m"
	reset := "\033[0m"

	input := plugin.Input{
		Colors: map[string]string{
			"white":       "\033[37m",
			"yellow":      "\033[33m",
			"red":         "\033[31m",
			"teal":        "\033[36m",
			"sky_blue":    "\033[94m",
			"dark_violet": "\033[35m",
			"lavender":    "\033[95m",
			"tangerine":   "\033[38;5;208m",
			"peach":       "\033[38;5;217m",
			"dark_gray":   darkGray,
			"reset":       reset,
		},
	}

	// Test text mode: all non-reset colors should be dark_gray
	cfg := usageConfig{style: "text", showHours: true, showMinutes: true, showDays: true, showOpus: true}
	result := p.renderStale(input, usage, cfg)

	// The output should only contain dark_gray and reset escape codes (no white/yellow/red)
	for _, forbidden := range []string{"\033[37m", "\033[33m", "\033[31m"} {
		if contains(result, forbidden) {
			t.Errorf("stale text output should not contain color %q, got %q", forbidden, result)
		}
	}
	if !contains(result, darkGray) {
		t.Errorf("stale text output should contain dark_gray %q, got %q", darkGray, result)
	}

	// Test bars mode: all non-reset colors should be dark_gray
	cfg.style = "bars"
	result = p.renderStale(input, usage, cfg)
	for _, forbidden := range []string{"\033[36m", "\033[94m", "\033[35m", "\033[95m"} {
		if contains(result, forbidden) {
			t.Errorf("stale bars output should not contain color %q, got %q", forbidden, result)
		}
	}
	if !contains(result, darkGray) {
		t.Errorf("stale bars output should contain dark_gray %q, got %q", darkGray, result)
	}
}

// contains checks if substr exists in s (avoids importing strings for test).
func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
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
