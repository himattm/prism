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
	if p.cache != nil {
		if cached, ok := p.cache.Get(memCacheKey); ok {
			return cached, nil
		}
	}

	pct := getMemPercent()
	if pct < 0 {
		return "", nil
	}

	buf := sparkline.PushAndSave(globalSessionID, "mem", pct)
	output := formatSparkMetricMuted(input, "MEM", buf, pct)

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

// getMemPercentDarwin parses vm_stat and sysctl to compute memory usage on macOS.
// vm_stat reports pages; we multiply by page size and compare to total physical memory.
func getMemPercentDarwin() int {
	ctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
	defer cancel()

	// Get total physical memory
	totalCmd := exec.CommandContext(ctx, "sysctl", "-n", "hw.memsize")
	var totalOut bytes.Buffer
	totalCmd.Stdout = &totalOut
	if err := totalCmd.Run(); err != nil {
		return -1
	}
	totalBytes, err := strconv.ParseInt(strings.TrimSpace(totalOut.String()), 10, 64)
	if err != nil || totalBytes == 0 {
		return -1
	}

	// Get vm_stat output
	vmCmd := exec.CommandContext(ctx, "vm_stat")
	var vmOut bytes.Buffer
	vmCmd.Stdout = &vmOut
	if err := vmCmd.Run(); err != nil {
		return -1
	}

	// Parse page size from first line: "Mach Virtual Memory Statistics: (page size of 16384 bytes)"
	lines := strings.Split(vmOut.String(), "\n")
	pageSize := int64(16384) // default for Apple Silicon
	if len(lines) > 0 {
		if idx := strings.Index(lines[0], "page size of "); idx >= 0 {
			rest := lines[0][idx+len("page size of "):]
			if sp := strings.Index(rest, " "); sp > 0 {
				if ps, err := strconv.ParseInt(rest[:sp], 10, 64); err == nil {
					pageSize = ps
				}
			}
		}
	}

	// Parse page counts: "Pages free: 12345."
	pages := make(map[string]int64)
	for _, line := range lines[1:] {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		valStr := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(parts[1]), "."))
		if v, err := strconv.ParseInt(valStr, 10, 64); err == nil {
			pages[key] = v
		}
	}

	// Free + inactive + speculative + purgeable ≈ available
	available := (pages["Pages free"] + pages["Pages inactive"] +
		pages["Pages speculative"] + pages["Pages purgeable"]) * pageSize

	used := totalBytes - available
	if used < 0 {
		used = 0
	}

	return clamp(int(used*100/totalBytes), 0, 100)
}
