package plugins

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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
	sessionID := input.Prism.SessionID

	if p.cache != nil {
		if cached, ok := p.cache.Get(batCacheKey); ok {
			return cached, nil
		}
	}

	pct, charging := getBatteryStatus()
	if pct < 0 {
		return "", nil // no battery (desktop, server, unsupported)
	}

	buf := sparkline.PushAndSave(sessionID, "bat", pct)

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

// batteryPctRe matches "42%" in pmset output
var batteryPctRe = regexp.MustCompile(`(\d+)%`)

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

	// Find percentage
	match := batteryPctRe.FindStringSubmatch(output)
	if match == nil {
		return -1, false
	}
	pct, err := strconv.Atoi(match[1])
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

// formatBatteryMetric formats battery with sparkline and charging-aware colors
func formatBatteryMetric(input plugin.Input, label string, buf *sparkline.Buffer, pct int, charging bool) string {
	reset := input.Colors["reset"]

	var color string
	if charging {
		color = input.Colors["emerald"]
	} else {
		color = sparkColor(input, pct)
	}

	spark := buf.Render()
	return fmt.Sprintf("%s%s %s %d%%%s", color, label, spark, pct, reset)
}
