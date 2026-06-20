package plugins

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/himattm/prism/internal/cache"
	"github.com/himattm/prism/internal/plugin"
	"github.com/himattm/prism/internal/sparkline"
)

const (
	batCacheTTL = 10 * time.Second // Battery changes slowly
	batCacheKey = "bat:pct"
)

// BatteryPlugin displays battery level as a sparkline
type BatteryPlugin struct {
	cache *cache.Cache
}

func (p *BatteryPlugin) Name() string { return "battery" }

func (p *BatteryPlugin) SetCache(c *cache.Cache) { p.cache = c }

func (p *BatteryPlugin) Execute(ctx context.Context, input plugin.Input) (string, error) {
	if p.cache != nil {
		if cached, ok := p.cache.Get(batCacheKey); ok {
			return cached, nil
		}
	}

	pct, charging := getBatteryStatus()
	if pct < 0 {
		return "", nil // no battery (desktop, server, unsupported)
	}

	buf := sparkline.PushAndSave(globalSessionID, "bat", pct)

	label := "BAT"
	if charging {
		label = "CHG"
	}

	output := formatBatteryMetric(input, label, buf, pct, charging)

	if p.cache != nil {
		p.cache.Set(batCacheKey, output, batCacheTTL)
	}

	return output, nil
}

// getBatteryStatus returns (percentage, charging) or (-1, false) if no battery
func getBatteryStatus() (int, bool) {
	switch runtime.GOOS {
	case "linux":
		return getBatteryLinux()
	case "darwin":
		return getBatteryDarwin()
	default:
		return -1, false
	}
}

// getBatteryLinux reads from /sys/class/power_supply/
func getBatteryLinux() (int, bool) {
	// Find battery device (usually BAT0 or BAT1)
	psDir := "/sys/class/power_supply"
	entries, err := os.ReadDir(psDir)
	if err != nil {
		return -1, false
	}

	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, "BAT") {
			continue
		}

		base := filepath.Join(psDir, name)

		// Read capacity
		capData, err := os.ReadFile(filepath.Join(base, "capacity"))
		if err != nil {
			continue
		}
		pct, err := strconv.Atoi(strings.TrimSpace(string(capData)))
		if err != nil {
			continue
		}

		// Read charging status
		statusData, err := os.ReadFile(filepath.Join(base, "status"))
		charging := false
		if err == nil {
			status := strings.TrimSpace(string(statusData))
			charging = status == "Charging" || status == "Full"
		}

		return clamp(pct, 0, 100), charging
	}

	return -1, false
}

// getBatteryDarwin parses `pmset -g batt` output on macOS.
// Example output:
//
//	Now drawing from 'Battery Power'
//	 -InternalBattery-0 (id=...)	85%; discharging; 3:42 remaining
func getBatteryDarwin() (int, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
	defer cancel()

	cmd := exec.CommandContext(ctx, "pmset", "-g", "batt")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return -1, false
	}

	output := out.String()

	// Find percentage without regex
	idx := strings.IndexByte(output, '%')
	if idx <= 0 {
		return -1, false
	}

	start := idx - 1
	for start >= 0 && output[start] >= '0' && output[start] <= '9' {
		start--
	}

	if start == idx-1 {
		// no digits found
		return -1, false
	}

	pctStr := output[start+1 : idx]
	pct, err := strconv.Atoi(pctStr)
	if err != nil {
		return -1, false
	}

	// Detect charging: look for "charging" but not "discharging"
	charging := strings.Contains(output, "charging") && !strings.Contains(output, "discharging")
	// Also treat "AC Power" with "charged" as charging/full
	if strings.Contains(output, "charged") {
		charging = true
	}

	return clamp(pct, 0, 100), charging
}

// formatBatteryMetric formats battery with charging-aware colors, always dimmed.
func formatBatteryMetric(input plugin.Input, label string, buf *sparkline.Buffer, pct int, charging bool) string {
	var color string
	if charging {
		color = input.Colors["emerald"]
	} else {
		// Invert: low battery is critical, high is healthy
		color = sparkColor(input, 100-pct)
	}
	return formatSparkOutput(input, label, buf, pct, input.Colors["dim"]+color)
}
