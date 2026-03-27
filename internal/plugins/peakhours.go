package plugins

import (
	"fmt"
)

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
