package plugins

import (
	"context"
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
	memCacheTTL = 2 * time.Second
	memCacheKey = "mem:pct"
)

// MemoryPlugin displays memory usage as a sparkline
type MemoryPlugin struct {
	cache *cache.Cache
}

func (p *MemoryPlugin) Name() string { return "memory" }

func (p *MemoryPlugin) SetCache(c *cache.Cache) { p.cache = c }

func (p *MemoryPlugin) Execute(ctx context.Context, input plugin.Input) (string, error) {
	sessionID := input.Prism.SessionID

	if p.cache != nil {
		if cached, ok := p.cache.Get(memCacheKey); ok {
			return cached, nil
		}
	}

	pct := getMemPercent()
	if pct < 0 {
		return "", nil
	}

	buf := sparkline.PushAndSave(sessionID, "mem", pct)
	output := formatSparkMetric(input, "MEM", buf, pct)

	if p.cache != nil {
		p.cache.Set(memCacheKey, output, memCacheTTL)
	}

	return output, nil
}

// getMemPercent returns memory usage percentage (0-100)
func getMemPercent() int {
	switch runtime.GOOS {
	case "linux":
		return getMemPercentLinux()
	case "darwin":
		return getMemPercentDarwin()
	default:
		return -1
	}
}

// getMemPercentLinux reads /proc/meminfo to calculate used memory percentage
func getMemPercentLinux() int {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return -1
	}

	var memTotal, memAvailable int64
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		val, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil {
			continue
		}
		switch fields[0] {
		case "MemTotal:":
			memTotal = val
		case "MemAvailable:":
			memAvailable = val
		}
	}

	if memTotal == 0 {
		return -1
	}

	used := memTotal - memAvailable
	return clamp(int(used*100/memTotal), 0, 100)
}

// getMemPercentDarwin uses vm_stat output (would need subprocess)
// For now, falls back to reading cached value or returning -1
func getMemPercentDarwin() int {
	// On macOS we'd need to call vm_stat or use syscall
	// For cross-platform simplicity, try /tmp cache file first
	data, err := os.ReadFile("/tmp/prism-mem-darwin.cache")
	if err != nil {
		return -1
	}
	pct, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return -1
	}
	return clamp(pct, 0, 100)
}
