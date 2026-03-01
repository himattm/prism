package plugins

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/himattm/prism/internal/burnrate"
	"github.com/himattm/prism/internal/cache"
	"github.com/himattm/prism/internal/plugin"
)

// UsagePlugin is a unified plugin that auto-detects billing type and renders
// the appropriate information:
// - For Max/Pro users (OAuth): shows usage limits (text or bars format)
// - For API billing users: shows cost
type UsagePlugin struct {
	cache *cache.Cache
}

func (p *UsagePlugin) Name() string {
	return "usage"
}

func (p *UsagePlugin) SetCache(c *cache.Cache) {
	p.cache = c
}

// usageConfig holds all configuration options for the usage plugin
type usageConfig struct {
	// Max/Pro plan options (usage_plan subsection)
	style       string // "text" or "bars"
	showHours   bool   // 5-hour session limit
	showMinutes bool   // include minutes in hour time display
	showDays    bool   // 7-day weekly limit
	showOpus    bool   // Opus-specific limit

	// API billing options (api_billing subsection)
	costDecimals int    // decimal places for cost (default 2)
	costColor    string // color key for cost (default "gray")
	showBurnRate bool   // show burn rate ~$X.XX/h (default true)
	showCache    bool   // show cache efficiency ⌁XX% (default true)
}

func (p *UsagePlugin) parseConfig(input plugin.Input) usageConfig {
	cfg := usageConfig{
		// usage_plan defaults
		style:       "text",
		showHours:   true,
		showMinutes: true,
		showDays:    true,
		showOpus:    true,
		// api_billing defaults
		costDecimals: 2,
		costColor:    "gray",
		showBurnRate: true,
		showCache:    true,
	}

	if c, ok := input.Config["usage"].(map[string]any); ok {
		// Parse usage_plan subsection
		if plan, ok := c["usage_plan"].(map[string]any); ok {
			if v, ok := plan["style"].(string); ok {
				cfg.style = v
			}
			if v, ok := plan["show_hours"].(bool); ok {
				cfg.showHours = v
			}
			if v, ok := plan["show_minutes"].(bool); ok {
				cfg.showMinutes = v
			}
			if v, ok := plan["show_days"].(bool); ok {
				cfg.showDays = v
			}
			if v, ok := plan["show_opus"].(bool); ok {
				cfg.showOpus = v
			}
		}
		// Parse api_billing subsection
		if billing, ok := c["api_billing"].(map[string]any); ok {
			if v, ok := billing["decimals"].(float64); ok {
				cfg.costDecimals = int(v)
			}
			if v, ok := billing["color"].(string); ok {
				cfg.costColor = v
			}
			if v, ok := billing["show_burn_rate"].(bool); ok {
				cfg.showBurnRate = v
			}
			if v, ok := billing["show_cache"].(bool); ok {
				cfg.showCache = v
			}
		}
	}

	return cfg
}

func (p *UsagePlugin) Execute(ctx context.Context, input plugin.Input) (string, error) {
	// Check if another usage plugin already rendered this cycle
	if _, ok := p.cache.Get(usageRenderedKey); ok {
		return "", nil
	}

	cfg := p.parseConfig(input)

	// Try to detect if user has OAuth credentials (Max/Pro plan)
	hasOAuth := p.hasOAuthCredentials()

	var result string
	var err error

	if hasOAuth {
		// Max/Pro user - show usage limits
		result, err = p.renderUsageLimits(ctx, input, cfg)
	} else {
		// API billing user - show cost
		result = p.renderCost(input, cfg)
	}

	// Mark that we rendered, so other usage plugins skip
	if result != "" {
		p.cache.Set(usageRenderedKey, "1", usageRenderedTTL)
	}

	return result, err
}

// hasOAuthCredentials checks if OAuth credentials exist
// Uses cached token lookup to avoid repeated keychain/filesystem access
func (p *UsagePlugin) hasOAuthCredentials() bool {
	// Check cache first to avoid repeated credential checks
	if cached, ok := p.cache.Get("has_oauth"); ok {
		return cached == "true"
	}

	// Try to get the token (uses token cache internally)
	token, err := GetCachedOAuthToken(p.cache)
	hasToken := err == nil && token != ""

	// Cache the detection result for 5 minutes
	if hasToken {
		p.cache.Set("has_oauth", "true", 5*time.Minute)
	} else {
		p.cache.Set("has_oauth", "false", 5*time.Minute)
	}

	return hasToken
}

// renderCost renders the cost for API billing users
func (p *UsagePlugin) renderCost(input plugin.Input, cfg usageConfig) string {
	cost := input.Session.CostUSD
	color := input.Colors[cfg.costColor]
	if color == "" {
		color = input.Colors["gray"] // fallback
	}
	reset := input.Colors["reset"]
	format := fmt.Sprintf("$%%.%df", cfg.costDecimals)
	costStr := fmt.Sprintf(format, cost)

	// Append cache efficiency indicator if enabled
	if cfg.showCache {
		if ratio, ok := cacheRatio(input.Session.InputTokens, input.Session.CacheCreationTokens, input.Session.CacheReadTokens); ok {
			costStr += fmt.Sprintf(" ⌁%d%%", ratio)
		}
	}

	// Append burn rate if enabled and session ID is available
	if cfg.showBurnRate && input.Prism.SessionID != "" {
		costStr += p.getBurnRateSuffix(input.Prism.SessionID, cost)
	}

	return fmt.Sprintf("%s%s%s", color, costStr, reset)
}

// cacheRatio calculates the cache read hit percentage.
// Returns (ratio, true) if there are cache reads, or (0, false) if not applicable.
func cacheRatio(inputTokens, cacheCreationTokens, cacheReadTokens int) (int, bool) {
	if cacheReadTokens <= 0 {
		return 0, false
	}
	denominator := inputTokens + cacheCreationTokens + cacheReadTokens
	if denominator <= 0 {
		return 0, false
	}
	return cacheReadTokens * 100 / denominator, true
}

// getBurnRateSuffix returns the burn rate suffix string (e.g., " ~$1.23/h") or empty string.
func (p *UsagePlugin) getBurnRateSuffix(sessionID string, currentCost float64) string {
	snap, existed, err := burnrate.LoadOrCreateSnapshot(sessionID, currentCost)
	if err != nil || !existed {
		return ""
	}

	rate, show := burnrate.CalculateRate(snap, currentCost, time.Now())
	if !show {
		return ""
	}

	return " " + burnrate.FormatRate(rate)
}

// renderUsageLimits renders usage limits for Max/Pro users
func (p *UsagePlugin) renderUsageLimits(ctx context.Context, input plugin.Input, cfg usageConfig) (string, error) {
	// Get usage data (with caching)
	usage, err := p.getUsageData(ctx, input.Prism.IsIdle)
	if err != nil || usage == nil {
		// Fall back to cost if we can't get usage data
		return p.renderCost(input, cfg), nil
	}

	if cfg.style == "bars" {
		return p.renderBars(input, usage, cfg), nil
	}
	return p.renderText(input, usage, cfg), nil
}

// renderText renders usage as text with countdown labels
func (p *UsagePlugin) renderText(input plugin.Input, usage *UsageResponse, cfg usageConfig) string {
	white := input.Colors["white"]
	yellow := input.Colors["yellow"]
	red := input.Colors["red"]
	reset := input.Colors["reset"]

	var result string

	// 5-hour session
	if cfg.showHours && usage.FiveHour != nil {
		timeRemaining, _ := TimeUntilReset(usage.FiveHour.ResetsAt)
		timeStr := FormatTimeRemaining(timeRemaining, false, cfg.showMinutes)
		color := getUsageColor(usage.FiveHour.Utilization, white, yellow, red)
		result += fmt.Sprintf("%s%s:%.0f%%%s", color, timeStr, usage.FiveHour.Utilization, reset)
	}

	// 7-day weekly
	if cfg.showDays && usage.SevenDay != nil {
		if result != "" {
			result += " "
		}
		timeRemaining, _ := TimeUntilReset(usage.SevenDay.ResetsAt)
		timeStr := FormatTimeRemaining(timeRemaining, true, false)
		color := getUsageColor(usage.SevenDay.Utilization, white, yellow, red)
		result += fmt.Sprintf("%s%s:%.0f%%%s", color, timeStr, usage.SevenDay.Utilization, reset)
	}

	// Opus weekly
	if cfg.showOpus && usage.SevenDayOpus != nil {
		if result != "" {
			result += " "
		}
		timeRemaining, _ := TimeUntilReset(usage.SevenDayOpus.ResetsAt)
		timeStr := FormatTimeRemaining(timeRemaining, true, false)
		color := getUsageColor(usage.SevenDayOpus.Utilization, white, yellow, red)
		result += fmt.Sprintf("%s%s:%.0f%%%s", color, timeStr, usage.SevenDayOpus.Utilization, reset)
	}

	return result
}

// renderBars renders usage as compact bar visualization
func (p *UsagePlugin) renderBars(input plugin.Input, usage *UsageResponse, cfg usageConfig) string {
	teal := input.Colors["teal"]
	skyBlue := input.Colors["sky_blue"]
	darkViolet := input.Colors["dark_violet"]
	lavender := input.Colors["lavender"]
	tangerine := input.Colors["tangerine"]
	peach := input.Colors["peach"]
	reset := input.Colors["reset"]

	var result string

	// 5-hour session bars
	if cfg.showHours && usage.FiveHour != nil {
		timeRemaining, _ := TimeUntilReset(usage.FiveHour.ResetsAt)
		timeLevel := TimeToBarLevel(timeRemaining, 5*time.Hour)
		usageLevel := UtilizationToBarLevel(usage.FiveHour.Utilization)
		result += fmt.Sprintf("%s%c%s%s%c%s",
			teal, LevelToBarChar(timeLevel), reset,
			skyBlue, LevelToBarChar(usageLevel), reset)
	}

	// 7-day weekly bars
	if cfg.showDays && usage.SevenDay != nil {
		if result != "" {
			result += " "
		}
		timeRemaining, _ := TimeUntilReset(usage.SevenDay.ResetsAt)
		timeLevel := TimeToBarLevel(timeRemaining, 7*24*time.Hour)
		usageLevel := UtilizationToBarLevel(usage.SevenDay.Utilization)
		result += fmt.Sprintf("%s%c%s%s%c%s",
			darkViolet, LevelToBarChar(timeLevel), reset,
			lavender, LevelToBarChar(usageLevel), reset)
	}

	// Opus weekly bars
	if cfg.showOpus && usage.SevenDayOpus != nil {
		if result != "" {
			result += " "
		}
		timeRemaining, _ := TimeUntilReset(usage.SevenDayOpus.ResetsAt)
		timeLevel := TimeToBarLevel(timeRemaining, 7*24*time.Hour)
		usageLevel := UtilizationToBarLevel(usage.SevenDayOpus.Utilization)
		result += fmt.Sprintf("%s%c%s%s%c%s",
			tangerine, LevelToBarChar(timeLevel), reset,
			peach, LevelToBarChar(usageLevel), reset)
	}

	return result
}

// getUsageColor returns the appropriate color based on utilization level
// Matches context bar thresholds: >= 90% red, >= 70% yellow, < 70% white
func getUsageColor(utilization float64, white, yellow, red string) string {
	switch {
	case utilization >= 90:
		return red
	case utilization >= 70:
		return yellow
	default:
		return white
	}
}

func (p *UsagePlugin) getUsageData(ctx context.Context, isIdle bool) (*UsageResponse, error) {
	// Check in-memory cache first
	if cached, ok := p.cache.Get(usageCacheKey); ok {
		var usage UsageResponse
		if err := json.Unmarshal([]byte(cached), &usage); err == nil {
			return &usage, nil
		}
	}

	// Only fetch fresh data when idle
	if !isIdle {
		// Return last-known data from disk while busy
		if usage, ok := loadUsageCache(); ok {
			return usage, nil
		}
		return nil, nil
	}

	// Get OAuth token (cached)
	token, err := GetCachedOAuthToken(p.cache)
	if err != nil {
		return nil, err
	}

	// Fetch usage data
	usage, err := FetchUsage(ctx, token)
	if err != nil {
		return nil, err
	}

	// Cache the result (in-memory and on disk)
	if data, err := json.Marshal(usage); err == nil {
		p.cache.Set(usageCacheKey, string(data), usageCacheTTL)
	}
	saveUsageCache(usage)

	return usage, nil
}
