package plugins

import (
	"bytes"
	"context"

	"os"
	"os/exec"
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

	buf := sparkline.PushAndSave(globalSessionID, "cpu", pct)
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
			// OPTIMIZATION: Avoid slow fmt.Sscanf reflection by using string indexing and ParseInt
			idx := strings.IndexByte(prev, ',')
			if idx != -1 {
				prevTotal, _ = strconv.ParseInt(prev[:idx], 10, 64)
				prevIdle, _ = strconv.ParseInt(prev[idx+1:], 10, 64)
			}
		}
	}

	// Save current snapshot
	if c != nil {
		// OPTIMIZATION: Avoid fmt.Sprintf by using fast string concatenation
		c.Set(prevCPUKey, strconv.FormatInt(total, 10)+","+strconv.FormatInt(idle, 10), 30*time.Second)
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

// getCPUPercentDarwin sums per-process CPU% via ps and normalizes by core count.
// This is instantaneous (no multi-sample delay) and stays within the 500ms plugin timeout.
func getCPUPercentDarwin() int {
	ctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
	defer cancel()

	cmd := exec.CommandContext(ctx, "ps", "-A", "-o", "%cpu")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return getCPUFromLoadAvgDarwin()
	}

	var total float64
	for _, line := range strings.Split(out.String(), "\n") {
		line = strings.TrimSpace(line)
		if v, err := strconv.ParseFloat(line, 64); err == nil {
			total += v
		}
	}

	cpus := runtime.NumCPU()
	if cpus == 0 {
		cpus = 1
	}
	return clamp(int(total/float64(cpus)), 0, 100)
}

// getCPUFromLoadAvgDarwin uses sysctl to read load average on macOS
func getCPUFromLoadAvgDarwin() int {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sysctl", "-n", "vm.loadavg")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return -1
	}

	// Output: "{ 1.23 1.45 1.67 }"
	result := strings.Trim(strings.TrimSpace(out.String()), "{ }")
	fields := strings.Fields(result)
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
	return clamp(int(load*100/float64(cpus)), 0, 100)
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
