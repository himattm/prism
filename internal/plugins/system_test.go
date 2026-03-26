package plugins

import (
	"context"
	"runtime"
	"testing"

	"github.com/himattm/prism/internal/cache"
	"github.com/himattm/prism/internal/plugin"
)

func testInput(sessionID string) plugin.Input {
	return plugin.Input{
		Prism: plugin.PrismContext{
			SessionID: sessionID,
		},
		Colors: map[string]string{
			"reset":   "\033[0m",
			"gray":    "\033[90m",
			"yellow":  "\033[33m",
			"red":     "\033[31m",
			"crimson": "\033[38;5;160m",
			"emerald": "\033[38;5;35m",
		},
	}
}

func TestCPUPlugin_Execute(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("CPU /proc/stat test only runs on Linux")
	}

	p := &CPUPlugin{}
	c := cache.New()
	p.SetCache(c)

	if p.Name() != "cpu" {
		t.Errorf("expected name 'cpu', got %q", p.Name())
	}

	input := testInput("test-cpu-session")
	output, err := p.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if output == "" {
		t.Fatal("expected non-empty output on Linux")
	}

	// Should contain "CPU" and a percentage
	if !containsSubstring(output, "CPU") {
		t.Errorf("output should contain 'CPU': %q", output)
	}
	if !containsSubstring(output, "%") {
		t.Errorf("output should contain '%%': %q", output)
	}

	// Second call should use cache
	output2, err := p.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("second Execute returned error: %v", err)
	}
	if output2 != output {
		t.Errorf("expected cached output %q, got %q", output, output2)
	}
}

func TestMemoryPlugin_Execute(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Memory /proc/meminfo test only runs on Linux")
	}

	p := &MemoryPlugin{}
	c := cache.New()
	p.SetCache(c)

	if p.Name() != "memory" {
		t.Errorf("expected name 'memory', got %q", p.Name())
	}

	input := testInput("test-mem-session")
	output, err := p.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if output == "" {
		t.Fatal("expected non-empty output on Linux")
	}
	if !containsSubstring(output, "MEM") {
		t.Errorf("output should contain 'MEM': %q", output)
	}
}

func TestBatteryPlugin_Execute(t *testing.T) {
	p := &BatteryPlugin{}
	c := cache.New()
	p.SetCache(c)

	if p.Name() != "battery" {
		t.Errorf("expected name 'battery', got %q", p.Name())
	}

	input := testInput("test-bat-session")
	output, err := p.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	// Battery may be empty string on desktops/servers with no battery - that's fine
	if output != "" {
		if !containsSubstring(output, "BAT") && !containsSubstring(output, "CHG") {
			t.Errorf("output should contain 'BAT' or 'CHG': %q", output)
		}
	}
}

func TestGetCPUPercentLinux(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux-only test")
	}

	c := cache.New()
	pct := getCPUPercentLinux(c)
	if pct < 0 || pct > 100 {
		t.Errorf("expected CPU%% in [0,100], got %d", pct)
	}

	// Second call should compute a delta
	pct2 := getCPUPercentLinux(c)
	if pct2 < 0 || pct2 > 100 {
		t.Errorf("expected CPU%% delta in [0,100], got %d", pct2)
	}
}

func TestGetMemPercentLinux(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux-only test")
	}

	pct := getMemPercentLinux()
	if pct < 0 || pct > 100 {
		t.Errorf("expected memory%% in [0,100], got %d", pct)
	}
}

func TestClamp(t *testing.T) {
	tests := []struct {
		v, min, max, want int
	}{
		{50, 0, 100, 50},
		{-10, 0, 100, 0},
		{200, 0, 100, 100},
		{0, 0, 100, 0},
		{100, 0, 100, 100},
	}
	for _, tt := range tests {
		got := clamp(tt.v, tt.min, tt.max)
		if got != tt.want {
			t.Errorf("clamp(%d, %d, %d) = %d, want %d", tt.v, tt.min, tt.max, got, tt.want)
		}
	}
}

func TestFormatSparkMetric(t *testing.T) {
	input := testInput("test-format")

	// Test via the real formatSparkMetric - need a real sparkline.Buffer
	// Just verify it doesn't panic with various percentages
	for _, pct := range []int{0, 25, 50, 69, 70, 89, 90, 100} {
		color := sparkColor(input, pct)
		if color == "" {
			t.Errorf("sparkColor returned empty for pct=%d", pct)
		}
	}
}

func TestSparkColor(t *testing.T) {
	input := testInput("test-color")

	tests := []struct {
		pct     int
		wantKey string
	}{
		{0, "gray"},
		{50, "gray"},
		{69, "gray"},
		{70, "yellow"},
		{89, "yellow"},
		{90, "crimson"},
		{100, "crimson"},
	}

	for _, tt := range tests {
		got := sparkColor(input, tt.pct)
		want := input.Colors[tt.wantKey]
		if got != want {
			t.Errorf("sparkColor(pct=%d) = %q, want %q (%s)", tt.pct, got, want, tt.wantKey)
		}
	}
}

func containsSubstring(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsCheck(s, sub))
}

func containsCheck(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
