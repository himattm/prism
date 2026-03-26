package plugins

import (
	"fmt"

	"github.com/himattm/prism/internal/plugin"
	"github.com/himattm/prism/internal/sparkline"
)

// formatSparkMetric renders a label + sparkline + percentage with color
// e.g. "CPU ▁▃▅▇█▅▃▂ 34%"
func formatSparkMetric(input plugin.Input, label string, buf *sparkline.Buffer, pct int) string {
	reset := input.Colors["reset"]
	color := sparkColor(input, pct)
	spark := buf.Render()

	return fmt.Sprintf("%s%s %s %d%%%s", color, label, spark, pct, reset)
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
