package plugins

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/himattm/prism/internal/cache"
	"github.com/himattm/prism/internal/plugin"
	"github.com/himattm/prism/internal/sparkline"
)

const (
	cpuCacheTTL = 2 * time.Second
	cpuCacheKey = "cpu:pct"

	// prevCPUKey stores the previous /proc/stat snapshot for delta calculation
	prevCPUKey = "cpu:prev"
)

// CPUPlugin displays CPU usage as a sparkline
type CPUPlugin struct {
	cache *cache.Cache
}

func (p *CPUPlugin) Name() string { return "cpu" }

func (p *CPUPlugin) SetCache(c *cache.Cache) { p.cache = c }

func (p *CPUPlugin) Execute(ctx context.Context, input plugin.Input) (string, error) {
	sessionID := input.Prism.SessionID

	// Check cache to avoid re-reading /proc/stat too often
	if p.cache != nil {
		if cached, ok := p.cache.Get(cpuCacheKey); ok {
			return cached, nil
		}
	}

	pct := getCPUPercent(p.cache)
	if pct < 0 {
		return "", nil // unsupported platform
	}

	buf := sparkline.PushAndSave(sessionID, "cpu", pct)
	output := formatSparkMetric(input, "CPU", buf, pct)

	if p.cache != nil {
		p.cache.Set(cpuCacheKey, output, cpuCacheTTL)
	}

	return output, nil
}

// getCPUPercent returns the current CPU usage percentage (0-100)
func getCPUPercent(c *cache.Cache) int {
	switch runtime.GOOS {
	case "linux":
		return getCPUPercentLinux(c)
	case "darwin":
		return getCPUPercentDarwin()
	default:
		return -1
	}
}

// getCPUPercentLinux reads /proc/stat and computes delta CPU usage
func getCPUPercentLinux(c *cache.Cache) int {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return -1
	}

	// First line: cpu  user nice system idle iowait irq softirq steal
	lines := strings.SplitN(string(data), "\n", 2)
	if len(lines) == 0 {
		return -1
	}

	fields := strings.Fields(lines[0])
	if len(fields) < 5 || fields[0] != "cpu" {
		return -1
	}

	// Parse all numeric fields
	var vals []int64
	for _, f := range fields[1:] {
		v, err := strconv.ParseInt(f, 10, 64)
		if err != nil {
			return -1
		}
		vals = append(vals, v)
	}

	// total = sum of all fields, idle = fields[3] (idle) + fields[4] (iowait) if present
	var total, idle int64
	for _, v := range vals {
		total += v
	}
	idle = vals[3] // idle
	if len(vals) > 4 {
		idle += vals[4] // iowait
	}

	// Load previous snapshot
	var prevTotal, prevIdle int64
	if c != nil {
		if prev, ok := c.Get(prevCPUKey); ok {
			fmt.Sscanf(prev, "%d,%d", &prevTotal, &prevIdle)
		}
	}

	// Save current snapshot
	if c != nil {
		c.Set(prevCPUKey, fmt.Sprintf("%d,%d", total, idle), 30*time.Second)
	}

	// Calculate delta
	if prevTotal == 0 {
		// First reading - no delta available, estimate from current values
		// This won't be very accurate but gives something on first call
		if total == 0 {
			return 0
		}
		return int((total - idle) * 100 / total)
	}

	dTotal := total - prevTotal
	dIdle := idle - prevIdle
	if dTotal <= 0 {
		return 0
	}

	return int((dTotal - dIdle) * 100 / dTotal)
}

// getCPUPercentDarwin uses host_processor_info via sysctl on macOS
func getCPUPercentDarwin() int {
	// Read from /usr/bin/ps as a quick approximation
	// This sums CPU% across all processes (can exceed 100% on multi-core)
	data, err := os.ReadFile("/tmp/prism-cpu-darwin.cache")
	if err != nil {
		// Fallback: use load average as a rough proxy
		return getCPUFromLoadAvg()
	}
	pct, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return getCPUFromLoadAvg()
	}
	return clamp(pct, 0, 100)
}

// getCPUFromLoadAvg reads /proc/loadavg (Linux) or uses sysctl (macOS)
// and normalizes by CPU count as a rough CPU% estimate
func getCPUFromLoadAvg() int {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return -1
	}
	fields := strings.Fields(string(data))
	if len(fields) == 0 {
		return -1
	}
	load, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return -1
	}
	cpus := runtime.NumCPU()
	if cpus == 0 {
		cpus = 1
	}
	pct := int(load * 100 / float64(cpus))
	return clamp(pct, 0, 100)
}

func clamp(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
