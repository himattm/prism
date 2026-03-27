package plugins

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/himattm/prism/internal/cache"
	"github.com/himattm/prism/internal/plugin"
)

const (
	peakStartHour = 5  // 5 AM PT
	peakEndHour   = 11 // 11 AM PT

	peakHoursAPIURL     = "https://promoclock.co/api/status"
	peakHoursAPITimeout = 2 * time.Second
)

// peakHoursResponse represents the JSON from promoclock.co/api/status
type peakHoursResponse struct {
	Status             string `json:"status"`
	IsPeak             bool   `json:"isPeak"`
	IsOffPeak          bool   `json:"isOffPeak"`
	IsWeekend          bool   `json:"isWeekend"`
	MinutesUntilChange int    `json:"minutesUntilChange"`
}

// parsePeakHoursResponse parses the promoclock API JSON response.
func parsePeakHoursResponse(body []byte) (*peakHoursResponse, error) {
	var resp peakHoursResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// localPeakStatus computes peak/off-peak status from local timezone math.
func localPeakStatus(now time.Time) (bool, bool, int) {
	loc, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		return false, false, 30
	}

	pt := now.In(loc)
	weekday := pt.Weekday()

	if weekday == time.Saturday || weekday == time.Sunday {
		return false, true, 0
	}

	hour := pt.Hour()
	minute := pt.Minute()

	if hour >= peakStartHour && hour < peakEndHour {
		minutesUntil := (peakEndHour-hour)*60 - minute
		return true, false, minutesUntil
	}

	if hour < peakStartHour {
		minutesUntil := (peakStartHour-hour)*60 - minute
		return false, false, minutesUntil
	}

	nextPeak := time.Date(pt.Year(), pt.Month(), pt.Day()+1, peakStartHour, 0, 0, 0, loc)
	for nextPeak.Weekday() == time.Saturday || nextPeak.Weekday() == time.Sunday {
		nextPeak = nextPeak.AddDate(0, 0, 1)
	}
	minutesUntil := int(nextPeak.Sub(pt).Minutes())
	return false, false, minutesUntil
}

// formatCountdown formats minutes remaining into a compact string.
// >60 min: "2h38m", 1-60 min: "38m", <1 min: "<1m"
func formatCountdown(minutes int) string {
	if minutes <= 0 {
		return "<1m"
	}
	if minutes >= 60 {
		return fmt.Sprintf("%dh%dm", minutes/60, minutes%60)
	}
	return fmt.Sprintf("%dm", minutes)
}

// formatPeakHoursOutput builds the colored status line segment.
func formatPeakHoursOutput(colors map[string]string, isPeak, isWeekend bool, minutesUntilChange int) string {
	if isWeekend {
		return ""
	}

	reset := colors["reset"]
	var b strings.Builder

	if isPeak {
		b.WriteString(colors["red"])
		b.WriteString("▲ Peak ")
	} else {
		b.WriteString(colors["green"])
		b.WriteString("▼ Off-Peak ")
	}

	b.WriteString(formatCountdown(minutesUntilChange))
	b.WriteString(reset)

	return b.String()
}

// PeakHoursPlugin displays Claude's current peak/off-peak status
type PeakHoursPlugin struct {
	cache *cache.Cache
}

func (p *PeakHoursPlugin) Name() string {
	return "peakhours"
}

func (p *PeakHoursPlugin) SetCache(c *cache.Cache) {
	p.cache = c
}

func (p *PeakHoursPlugin) Execute(ctx context.Context, input plugin.Input) (string, error) {
	cacheKey := "peakhours:status"

	// Check cache — may contain empty string for weekends
	if p.cache != nil {
		if cached, ok := p.cache.Get(cacheKey); ok {
			return cached, nil
		}
	}

	// Try API first
	output, ttl := p.fetchFromAPI(ctx, input.Colors)

	// Fallback to local timezone calculation if API failed
	if output == nil {
		result, cacheTTL := p.localFallback(input.Colors)
		output = &result
		ttl = cacheTTL
	}

	// Cache the result (including empty string for weekends)
	if p.cache != nil && ttl > 0 {
		p.cache.Set(cacheKey, *output, ttl)
	}

	return *output, nil
}

// fetchFromAPI calls the promoclock API and returns formatted output + TTL.
// Returns nil output if the API call fails.
func (p *PeakHoursPlugin) fetchFromAPI(ctx context.Context, colors map[string]string) (*string, time.Duration) {
	reqCtx, cancel := context.WithTimeout(ctx, peakHoursAPITimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, peakHoursAPIURL, nil)
	if err != nil {
		return nil, 0
	}

	client := &http.Client{Timeout: peakHoursAPITimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, 0
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, 0
	}

	apiResp, err := parsePeakHoursResponse(body)
	if err != nil {
		return nil, 0
	}

	output := formatPeakHoursOutput(colors, apiResp.IsPeak, apiResp.IsWeekend, apiResp.MinutesUntilChange)

	ttl := time.Duration(apiResp.MinutesUntilChange) * time.Minute
	if ttl <= 0 {
		ttl = cache.PeakHoursTTL
	}

	return &output, ttl
}

// localFallback computes peak status from timezone math when the API is unreachable.
func (p *PeakHoursPlugin) localFallback(colors map[string]string) (string, time.Duration) {
	isPeak, isWeekend, minutesUntil := localPeakStatus(time.Now())
	output := formatPeakHoursOutput(colors, isPeak, isWeekend, minutesUntil)

	ttl := time.Duration(minutesUntil) * time.Minute
	if ttl <= 0 {
		ttl = cache.PeakHoursTTL
	}

	return output, ttl
}
