package plugins

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/himattm/prism/internal/cache"
	"github.com/himattm/prism/internal/plugin"
)

// UsageBarsPlugin displays usage limits as compact bar visualization
type UsageBarsPlugin struct {
	cache *cache.Cache
}

func (p *UsageBarsPlugin) Name() string {
	return "usage_bars"
}

func (p *UsageBarsPlugin) SetCache(c *cache.Cache) {
	p.cache = c
}

func (p *UsageBarsPlugin) Execute(ctx context.Context, input plugin.Input) (string, error) {
	// Check if another usage plugin already rendered this cycle
	if _, ok := p.cache.Get(usageRenderedKey); ok {
		return "", nil
	}

	// Check if enabled (default: true)
	showHours := true
	showDays := true
	showOpus := true

	if cfg, ok := input.Config["usage_bars"].(map[string]any); ok {
		if enabled, ok := cfg["enabled"].(bool); ok && !enabled {
			return "", nil
		}
		if sh, ok := cfg["show_hours"].(bool); ok {
			showHours = sh
		}
		if sd, ok := cfg["show_days"].(bool); ok {
			showDays = sd
		}
		if so, ok := cfg["show_opus"].(bool); ok {
			showOpus = so
		}
	}

	// Try to get cached usage data first
	usage, err := p.getUsageData(ctx, input.Prism.IsIdle)
	if err != nil || usage == nil {
		return "", nil // Silently fail - user may not have OAuth
	}

	// Get colors
	teal := input.Colors["teal"]
	skyBlue := input.Colors["sky_blue"]
	darkViolet := input.Colors["dark_violet"]
	lavender := input.Colors["lavender"]
	tangerine := input.Colors["tangerine"]
	peach := input.Colors["peach"]
	reset := input.Colors["reset"]

	var result string

	// 5-hour session bars
	if showHours && usage.FiveHour != nil {
		timeRemaining, _ := TimeUntilReset(usage.FiveHour.ResetsAt)
		timeLevel := TimeToBarLevel(timeRemaining, 5*time.Hour)
		usageLevel := UtilizationToBarLevel(usage.FiveHour.Utilization)

		result += fmt.Sprintf("%s%c%s%s%c%s",
			teal, LevelToBarChar(timeLevel), reset,
			skyBlue, LevelToBarChar(usageLevel), reset)
	}

	// 7-day weekly bars
	if showDays && usage.SevenDay != nil {
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
	if showOpus && usage.SevenDayOpus != nil {
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

	// Mark that we rendered, so other usage plugins skip
	if result != "" {
		p.cache.Set(usageRenderedKey, "1", usageRenderedTTL)
	}

	return result, nil
}

func (p *UsageBarsPlugin) getUsageData(ctx context.Context, isIdle bool) (*UsageResponse, error) {
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
		if usage, _, ok := loadUsageCache(); ok {
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
