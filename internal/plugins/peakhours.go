package plugins

import (
	"encoding/json"
	"fmt"
	"time"
)

const (
	peakStartHour = 5  // 5 AM PT
	peakEndHour   = 11 // 11 AM PT
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
