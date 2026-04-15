package plugins

import (
	"context"
	"fmt"

	"github.com/himattm/prism/internal/cache"
	"github.com/himattm/prism/internal/plugin"
)

// UsageTextPlugin displays usage limits as text with countdown labels
type UsageTextPlugin struct {
	cache *cache.Cache
}

func (p *UsageTextPlugin) Name() string {
	return "usage_text"
}

func (p *UsageTextPlugin) SetCache(c *cache.Cache) {
	p.cache = c
}

func (p *UsageTextPlugin) Execute(ctx context.Context, input plugin.Input) (string, error) {
	// Check if another usage plugin already rendered this cycle
	if _, ok := p.cache.Get(usageRenderedKey); ok {
		return "", nil
	}

	// Check if enabled (default: true)
	showHours := true
	showMinutes := true
	showDays := true
	showOpus := true

	if cfg, ok := input.Config["usage_text"].(map[string]any); ok {
		if enabled, ok := cfg["enabled"].(bool); ok && !enabled {
			return "", nil
		}
		if sh, ok := cfg["show_hours"].(bool); ok {
			showHours = sh
		}
		if sm, ok := cfg["show_minutes"].(bool); ok {
			showMinutes = sm
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

	// Get colors for urgency levels
	white := input.Colors["white"]
	yellow := input.Colors["yellow"]
	red := input.Colors["red"]
	reset := input.Colors["reset"]

	var result string

	// 5-hour session text
	if showHours && usage.FiveHour != nil {
		timeRemaining, _ := TimeUntilReset(usage.FiveHour.ResetsAt)
		timeStr := FormatTimeRemaining(timeRemaining, false, showMinutes)
		color := GetUsageColor(usage.FiveHour.Utilization, white, yellow, red)

		result += fmt.Sprintf("%s%s:%.0f%%%s",
			color, timeStr, usage.FiveHour.Utilization, reset)
	}

	// 7-day weekly text
	if showDays && usage.SevenDay != nil {
		if result != "" {
			result += " "
		}
		timeRemaining, _ := TimeUntilReset(usage.SevenDay.ResetsAt)
		timeStr := FormatTimeRemaining(timeRemaining, true, false)
		color := GetUsageColor(usage.SevenDay.Utilization, white, yellow, red)

		result += fmt.Sprintf("%s%s:%.0f%%%s",
			color, timeStr, usage.SevenDay.Utilization, reset)
	}

	// Opus weekly text
	if showOpus && usage.SevenDayOpus != nil {
		if result != "" {
			result += " "
		}
		timeRemaining, _ := TimeUntilReset(usage.SevenDayOpus.ResetsAt)
		timeStr := FormatTimeRemaining(timeRemaining, true, false)
		color := GetUsageColor(usage.SevenDayOpus.Utilization, white, yellow, red)

		result += fmt.Sprintf("%s%s:%.0f%%%s",
			color, timeStr, usage.SevenDayOpus.Utilization, reset)
	}

	// Mark that we rendered, so other usage plugins skip
	if result != "" {
		p.cache.Set(usageRenderedKey, "1", usageRenderedTTL)
	}

	return result, nil
}

func (p *UsageTextPlugin) getUsageData(ctx context.Context, isIdle bool) (*UsageResponse, error) {
	usage, _, err := GetUsageData(p.cache, ctx, isIdle)
	return usage, err
}
