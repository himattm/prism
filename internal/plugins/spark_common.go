package plugins

import (
	"fmt"

	"github.com/himattm/prism/internal/plugin"
	"github.com/himattm/prism/internal/sparkline"
)

// globalSessionID is used for machine-wide metrics (CPU, memory, battery)
// so all Claude Code sessions share one sparkline buffer.
const globalSessionID = "global"

// formatSparkOutput renders label + sparkline + pct with a pre-computed color.
func formatSparkOutput(input plugin.Input, label string, buf *sparkline.Buffer, pct int, color string) string {
	reset := input.Colors["reset"]
	spark := buf.Render()
	return fmt.Sprintf("%s%s %s %d%%%s", color, label, spark, pct, reset)
}

// formatSparkMetric renders a label + sparkline + percentage with color
// e.g. "CPU ▁▃▅▇█▅▃▂ 34%"
func formatSparkMetric(input plugin.Input, label string, buf *sparkline.Buffer, pct int) string {
	return formatSparkOutput(input, label, buf, pct, sparkColor(input, pct))
}

// formatSparkMetricMuted renders a muted variant (dimmed)
func formatSparkMetricMuted(input plugin.Input, label string, buf *sparkline.Buffer, pct int) string {
	color := input.Colors["dim"] + sparkColor(input, pct)
	return formatSparkOutput(input, label, buf, pct, color)
}

// sparkColor returns the appropriate ANSI color based on the percentage:
//   - <70%  → gray (normal)
//   - 70-89% → yellow (warning)
//   - >=90% → crimson (critical)
func sparkColor(input plugin.Input, pct int) string {
	switch {
	case pct >= 90:
		if c, ok := input.Colors["crimson"]; ok {
			return c
		}
		return input.Colors["red"]
	case pct >= 70:
		return input.Colors["yellow"]
	default:
		return input.Colors["gray"]
	}
}
