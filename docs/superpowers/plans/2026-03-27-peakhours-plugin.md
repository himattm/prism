# Peak Hours Plugin Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a native Prism plugin that displays Claude's peak/off-peak status with a smart countdown in the status line.

**Architecture:** Single-file plugin (`peakhours.go`) that fetches status from the promoclock.co API, caches with a dynamic TTL derived from the API's `minutesUntilChange` field, and falls back to local timezone calculation when the API is unreachable. Returns empty string on weekends.

**Tech Stack:** Go standard library (`net/http`, `encoding/json`, `time`), Prism plugin interface, Prism cache package.

**Spec:** `docs/superpowers/specs/2026-03-27-peakhours-plugin-design.md`

---

### Task 1: Add PeakHoursTTL Cache Constant

**Files:**
- Modify: `internal/cache/cache.go` (TTL constants block at end of file)

- [ ] **Step 1: Add the constant**

Add `PeakHoursTTL` to the existing TTL constants block in `internal/cache/cache.go`:

```go
PeakHoursTTL = 30 * time.Minute // Fallback TTL when API doesn't return minutesUntilChange
```

- [ ] **Step 2: Verify build**

Run: `go build ./...`
Expected: Clean build, no errors.

- [ ] **Step 3: Commit**

```bash
git add internal/cache/cache.go
git commit -m "Add PeakHoursTTL cache constant"
```

---

### Task 2: Countdown Formatting

**Files:**
- Create: `internal/plugins/peakhours_test.go`
- Create: `internal/plugins/peakhours.go`

- [ ] **Step 1: Write failing test for formatCountdown**

Create `internal/plugins/peakhours_test.go`:

```go
package plugins

import (
	"testing"
)

func TestFormatCountdown(t *testing.T) {
	tests := []struct {
		name     string
		minutes  int
		expected string
	}{
		{name: "hours and minutes", minutes: 158, expected: "2h38m"},
		{name: "exactly one hour", minutes: 60, expected: "1h0m"},
		{name: "just under an hour", minutes: 59, expected: "59m"},
		{name: "one minute", minutes: 1, expected: "1m"},
		{name: "zero minutes", minutes: 0, expected: "<1m"},
		{name: "negative minutes", minutes: -5, expected: "<1m"},
		{name: "large value", minutes: 600, expected: "10h0m"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatCountdown(tt.minutes)
			if result != tt.expected {
				t.Errorf("formatCountdown(%d) = %q, want %q", tt.minutes, result, tt.expected)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/plugins/ -run TestFormatCountdown -v`
Expected: FAIL — `formatCountdown` not defined.

- [ ] **Step 3: Write minimal implementation**

Create `internal/plugins/peakhours.go`:

```go
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
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/plugins/ -run TestFormatCountdown -v`
Expected: PASS — all 7 cases.

- [ ] **Step 5: Commit**

```bash
git add internal/plugins/peakhours.go internal/plugins/peakhours_test.go
git commit -m "Add formatCountdown for peak hours plugin"
```

---

### Task 3: Local Timezone Fallback Logic

**Files:**
- Modify: `internal/plugins/peakhours_test.go`
- Modify: `internal/plugins/peakhours.go`

- [ ] **Step 1: Write failing tests for localPeakStatus**

Append to `internal/plugins/peakhours_test.go`:

```go
import (
	"time"
)

func TestLocalPeakStatus(t *testing.T) {
	loc, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		t.Fatalf("failed to load timezone: %v", err)
	}

	tests := []struct {
		name            string
		time            time.Time
		expectPeak      bool
		expectWeekend   bool
		expectMinutes   int // approximate — we check within a range
	}{
		{
			name:          "weekday peak start",
			time:          time.Date(2026, 3, 27, 5, 0, 0, 0, loc), // Friday 5:00 AM PT
			expectPeak:    true,
			expectWeekend: false,
			expectMinutes: 360, // 6 hours until 11 AM
		},
		{
			name:          "weekday mid-peak",
			time:          time.Date(2026, 3, 27, 8, 0, 0, 0, loc), // Friday 8:00 AM PT
			expectPeak:    true,
			expectWeekend: false,
			expectMinutes: 180, // 3 hours until 11 AM
		},
		{
			name:          "weekday peak end boundary",
			time:          time.Date(2026, 3, 27, 10, 59, 0, 0, loc), // Friday 10:59 AM PT
			expectPeak:    true,
			expectWeekend: false,
			expectMinutes: 1,
		},
		{
			name:          "weekday just after peak",
			time:          time.Date(2026, 3, 27, 11, 0, 0, 0, loc), // Friday 11:00 AM PT
			expectPeak:    false,
			expectWeekend: false,
			expectMinutes: 3960, // 66 hours until Monday 5 AM (skips weekend)
		},
		{
			name:          "weekday evening",
			time:          time.Date(2026, 3, 26, 20, 0, 0, 0, loc), // Thursday 8:00 PM PT
			expectPeak:    false,
			expectWeekend: false,
			expectMinutes: 540, // 9 hours until 5 AM
		},
		{
			name:          "weekday before peak",
			time:          time.Date(2026, 3, 27, 3, 0, 0, 0, loc), // Friday 3:00 AM PT
			expectPeak:    false,
			expectWeekend: false,
			expectMinutes: 120, // 2 hours until 5 AM
		},
		{
			name:          "saturday",
			time:          time.Date(2026, 3, 28, 8, 0, 0, 0, loc), // Saturday
			expectPeak:    false,
			expectWeekend: true,
			expectMinutes: 0, // don't care for weekend
		},
		{
			name:          "sunday",
			time:          time.Date(2026, 3, 29, 14, 0, 0, 0, loc), // Sunday
			expectPeak:    false,
			expectWeekend: true,
			expectMinutes: 0,
		},
		{
			name:          "friday after peak",
			time:          time.Date(2026, 3, 27, 15, 0, 0, 0, loc), // Friday 3 PM PT
			expectPeak:    false,
			expectWeekend: false,
			expectMinutes: 3720, // 62 hours until Monday 5 AM (skips weekend)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isPeak, isWeekend, minutesUntil := localPeakStatus(tt.time)
			if isPeak != tt.expectPeak {
				t.Errorf("isPeak: got %v, want %v", isPeak, tt.expectPeak)
			}
			if isWeekend != tt.expectWeekend {
				t.Errorf("isWeekend: got %v, want %v", isWeekend, tt.expectWeekend)
			}
			if !tt.expectWeekend && minutesUntil != tt.expectMinutes {
				t.Errorf("minutesUntil: got %d, want %d", minutesUntil, tt.expectMinutes)
			}
		})
	}
}
```

Update the last test case `"friday after peak"` — the expected minutes for Friday 3 PM to Monday 5 AM:
- Friday 3 PM to Saturday 12 AM = 9h
- Saturday = 24h
- Sunday = 24h
- Monday 12 AM to 5 AM = 5h
- Total = 62h = 3720m

So set `expectMinutes: 3720`.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/plugins/ -run TestLocalPeakStatus -v`
Expected: FAIL — `localPeakStatus` not defined.

- [ ] **Step 3: Write implementation**

Add to `internal/plugins/peakhours.go`:

```go
import (
	"time"
)

const (
	peakStartHour = 5  // 5 AM PT
	peakEndHour   = 11 // 11 AM PT
)

// localPeakStatus computes peak/off-peak status from local timezone math.
// Returns: isPeak, isWeekend, minutesUntilNextTransition.
func localPeakStatus(now time.Time) (bool, bool, int) {
	loc, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		// Fallback: assume off-peak if timezone fails
		return false, false, 30
	}

	pt := now.In(loc)
	weekday := pt.Weekday()

	// Weekend check
	if weekday == time.Saturday || weekday == time.Sunday {
		return false, true, 0
	}

	hour := pt.Hour()
	minute := pt.Minute()

	if hour >= peakStartHour && hour < peakEndHour {
		// During peak — minutes until peakEndHour
		minutesUntil := (peakEndHour-hour)*60 - minute
		return true, false, minutesUntil
	}

	// Off-peak — minutes until next peak start
	if hour < peakStartHour {
		// Before peak today
		minutesUntil := (peakStartHour-hour)*60 - minute
		return false, false, minutesUntil
	}

	// After peak today — find next weekday 5 AM
	nextPeak := time.Date(pt.Year(), pt.Month(), pt.Day()+1, peakStartHour, 0, 0, 0, loc)
	for nextPeak.Weekday() == time.Saturday || nextPeak.Weekday() == time.Sunday {
		nextPeak = nextPeak.AddDate(0, 0, 1)
	}
	minutesUntil := int(nextPeak.Sub(pt).Minutes())
	return false, false, minutesUntil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/plugins/ -run TestLocalPeakStatus -v`
Expected: PASS — all cases.

- [ ] **Step 5: Commit**

```bash
git add internal/plugins/peakhours.go internal/plugins/peakhours_test.go
git commit -m "Add localPeakStatus timezone fallback logic"
```

---

### Task 4: API Response Parsing

**Files:**
- Modify: `internal/plugins/peakhours_test.go`
- Modify: `internal/plugins/peakhours.go`

- [ ] **Step 1: Write failing tests for parsePeakHoursResponse**

Append to `internal/plugins/peakhours_test.go`:

```go
func TestParsePeakHoursResponse(t *testing.T) {
	tests := []struct {
		name        string
		body        string
		expectErr   bool
		expectPeak  bool
		expectWknd  bool
		expectMin   int
	}{
		{
			name: "peak response",
			body: `{"status":"peak","isPeak":true,"isOffPeak":false,"isWeekend":false,"minutesUntilChange":190}`,
			expectPeak: true,
			expectWknd: false,
			expectMin:  190,
		},
		{
			name: "off-peak response",
			body: `{"status":"off-peak","isPeak":false,"isOffPeak":true,"isWeekend":false,"minutesUntilChange":252}`,
			expectPeak: false,
			expectWknd: false,
			expectMin:  252,
		},
		{
			name: "weekend response",
			body: `{"status":"off-peak","isPeak":false,"isOffPeak":true,"isWeekend":true,"minutesUntilChange":1440}`,
			expectPeak: false,
			expectWknd: true,
			expectMin:  1440,
		},
		{
			name:      "invalid json",
			body:      `not json`,
			expectErr: true,
		},
		{
			name:      "empty body",
			body:      ``,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := parsePeakHoursResponse([]byte(tt.body))
			if tt.expectErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if resp.IsPeak != tt.expectPeak {
				t.Errorf("IsPeak: got %v, want %v", resp.IsPeak, tt.expectPeak)
			}
			if resp.IsWeekend != tt.expectWknd {
				t.Errorf("IsWeekend: got %v, want %v", resp.IsWeekend, tt.expectWknd)
			}
			if resp.MinutesUntilChange != tt.expectMin {
				t.Errorf("MinutesUntilChange: got %d, want %d", resp.MinutesUntilChange, tt.expectMin)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/plugins/ -run TestParsePeakHoursResponse -v`
Expected: FAIL — `parsePeakHoursResponse` not defined.

- [ ] **Step 3: Write implementation**

Add to `internal/plugins/peakhours.go`:

```go
import (
	"encoding/json"
)

// peakHoursResponse represents the JSON from promoclock.co/api/status
type peakHoursResponse struct {
	Status             string `json:"status"`
	IsPeak             bool   `json:"isPeak"`
	IsOffPeak          bool   `json:"isOffPeak"`
	IsWeekend          bool   `json:"isWeekend"`
	MinutesUntilChange int    `json:"minutesUntilChange"`
}

// parsePeakHoursResponse parses the promoclock API JSON response.
func parsePeakHoursResponse(body []byte) (*peakHoursResponse, error) {
	var resp peakHoursResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/plugins/ -run TestParsePeakHoursResponse -v`
Expected: PASS — all 5 cases.

- [ ] **Step 5: Commit**

```bash
git add internal/plugins/peakhours.go internal/plugins/peakhours_test.go
git commit -m "Add peakHoursResponse struct and parser"
```

---

### Task 5: Output Formatting

**Files:**
- Modify: `internal/plugins/peakhours_test.go`
- Modify: `internal/plugins/peakhours.go`

- [ ] **Step 1: Write failing tests for formatPeakHoursOutput**

Append to `internal/plugins/peakhours_test.go`:

```go
func TestFormatPeakHoursOutput(t *testing.T) {
	colors := map[string]string{
		"red":   "[red]",
		"green": "[green]",
		"reset": "[reset]",
	}

	tests := []struct {
		name      string
		isPeak    bool
		isWeekend bool
		minutes   int
		expected  string
	}{
		{
			name:     "peak with hours and minutes",
			isPeak:   true,
			minutes:  158,
			expected: "[red]▲ Peak 2h38m[reset]",
		},
		{
			name:     "peak with minutes only",
			isPeak:   true,
			minutes:  38,
			expected: "[red]▲ Peak 38m[reset]",
		},
		{
			name:     "peak under one minute",
			isPeak:   true,
			minutes:  0,
			expected: "[red]▲ Peak <1m[reset]",
		},
		{
			name:     "off-peak with hours",
			isPeak:   false,
			minutes:  252,
			expected: "[green]▼ Off-Peak 4h12m[reset]",
		},
		{
			name:     "off-peak with minutes only",
			isPeak:   false,
			minutes:  45,
			expected: "[green]▼ Off-Peak 45m[reset]",
		},
		{
			name:      "weekend returns empty",
			isWeekend: true,
			expected:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatPeakHoursOutput(colors, tt.isPeak, tt.isWeekend, tt.minutes)
			if result != tt.expected {
				t.Errorf("got %q, want %q", result, tt.expected)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/plugins/ -run TestFormatPeakHoursOutput -v`
Expected: FAIL — `formatPeakHoursOutput` not defined.

- [ ] **Step 3: Write implementation**

Add to `internal/plugins/peakhours.go`:

```go
import (
	"strings"
)

// formatPeakHoursOutput builds the colored status line segment.
func formatPeakHoursOutput(colors map[string]string, isPeak, isWeekend bool, minutesUntilChange int) string {
	if isWeekend {
		return ""
	}

	reset := colors["reset"]
	var b strings.Builder

	if isPeak {
		b.WriteString(colors["red"])
		b.WriteString("▲ Peak ")
	} else {
		b.WriteString(colors["green"])
		b.WriteString("▼ Off-Peak ")
	}

	b.WriteString(formatCountdown(minutesUntilChange))
	b.WriteString(reset)

	return b.String()
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/plugins/ -run TestFormatPeakHoursOutput -v`
Expected: PASS — all 6 cases.

- [ ] **Step 5: Commit**

```bash
git add internal/plugins/peakhours.go internal/plugins/peakhours_test.go
git commit -m "Add formatPeakHoursOutput for peak hours plugin"
```

---

### Task 6: Plugin Struct, Execute, and Registration

**Files:**
- Modify: `internal/plugins/peakhours.go`
- Modify: `internal/plugins/peakhours_test.go`
- Modify: `internal/plugins/interface.go` (add to `NewRegistry`)

- [ ] **Step 1: Write failing tests for the plugin**

Append to `internal/plugins/peakhours_test.go`:

```go
import (
	"context"

	"github.com/himattm/prism/internal/cache"
	"github.com/himattm/prism/internal/plugin"
)

func TestPeakHoursPlugin_Name(t *testing.T) {
	p := &PeakHoursPlugin{}
	if p.Name() != "peakhours" {
		t.Errorf("expected name 'peakhours', got '%s'", p.Name())
	}
}

func TestPeakHoursPlugin_SetCache(t *testing.T) {
	p := &PeakHoursPlugin{}
	c := cache.New()
	p.SetCache(c)
	if p.cache != c {
		t.Error("cache was not set correctly")
	}
}

func TestPeakHoursPlugin_Execute_CacheHit(t *testing.T) {
	p := &PeakHoursPlugin{}
	c := cache.New()
	p.SetCache(c)

	expected := "[red]▲ Peak 2h38m[reset]"
	c.Set("peakhours:status", expected, time.Minute)

	input := plugin.Input{
		Colors: map[string]string{
			"red":   "[red]",
			"green": "[green]",
			"reset": "[reset]",
		},
	}

	result, err := p.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != expected {
		t.Errorf("got %q, want %q", result, expected)
	}
}

func TestPeakHoursPlugin_Execute_WeekendCacheHit(t *testing.T) {
	p := &PeakHoursPlugin{}
	c := cache.New()
	p.SetCache(c)

	// Weekend is cached as empty string
	c.Set("peakhours:status", "", time.Minute)

	input := plugin.Input{
		Colors: map[string]string{
			"red":   "[red]",
			"green": "[green]",
			"reset": "[reset]",
		},
	}

	result, err := p.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "" {
		t.Errorf("expected empty string for weekend, got %q", result)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/plugins/ -run TestPeakHoursPlugin -v`
Expected: FAIL — `PeakHoursPlugin` not defined.

- [ ] **Step 3: Write the plugin struct and Execute method**

Add to `internal/plugins/peakhours.go` (replacing the existing partial file — this is the complete final file):

```go
package plugins

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/himattm/prism/internal/cache"
	"github.com/himattm/prism/internal/plugin"
)

const (
	peakHoursAPIURL = "https://promoclock.co/api/status"
	peakHoursAPITimeout = 2 * time.Second
	peakStartHour   = 5  // 5 AM PT
	peakEndHour     = 11 // 11 AM PT
)

// PeakHoursPlugin displays Claude's current peak/off-peak status
type PeakHoursPlugin struct {
	cache *cache.Cache
}

// peakHoursResponse represents the JSON from promoclock.co/api/status
type peakHoursResponse struct {
	Status             string `json:"status"`
	IsPeak             bool   `json:"isPeak"`
	IsOffPeak          bool   `json:"isOffPeak"`
	IsWeekend          bool   `json:"isWeekend"`
	MinutesUntilChange int    `json:"minutesUntilChange"`
}

func (p *PeakHoursPlugin) Name() string {
	return "peakhours"
}

func (p *PeakHoursPlugin) SetCache(c *cache.Cache) {
	p.cache = c
}

func (p *PeakHoursPlugin) Execute(ctx context.Context, input plugin.Input) (string, error) {
	cacheKey := "peakhours:status"

	// Check cache — may contain empty string for weekends
	if p.cache != nil {
		if cached, ok := p.cache.Get(cacheKey); ok {
			return cached, nil
		}
	}

	// Try API first
	output, ttl := p.fetchFromAPI(ctx, input.Colors)

	// Fallback to local timezone calculation if API failed
	if output == nil {
		result, cacheTTL := p.localFallback(input.Colors)
		output = &result
		ttl = cacheTTL
	}

	// Cache the result (including empty string for weekends)
	if p.cache != nil && ttl > 0 {
		p.cache.Set(cacheKey, *output, ttl)
	}

	return *output, nil
}

// fetchFromAPI calls the promoclock API and returns formatted output + TTL.
// Returns nil output if the API call fails.
func (p *PeakHoursPlugin) fetchFromAPI(ctx context.Context, colors map[string]string) (*string, time.Duration) {
	reqCtx, cancel := context.WithTimeout(ctx, peakHoursAPITimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, peakHoursAPIURL, nil)
	if err != nil {
		return nil, 0
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, 0
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, 0
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0
	}

	apiResp, err := parsePeakHoursResponse(body)
	if err != nil {
		return nil, 0
	}

	output := formatPeakHoursOutput(colors, apiResp.IsPeak, apiResp.IsWeekend, apiResp.MinutesUntilChange)

	ttl := time.Duration(apiResp.MinutesUntilChange) * time.Minute
	if ttl <= 0 {
		ttl = cache.PeakHoursTTL
	}

	return &output, ttl
}

// localFallback computes peak status from timezone math when the API is unreachable.
func (p *PeakHoursPlugin) localFallback(colors map[string]string) (string, time.Duration) {
	isPeak, isWeekend, minutesUntil := localPeakStatus(time.Now())
	output := formatPeakHoursOutput(colors, isPeak, isWeekend, minutesUntil)

	ttl := time.Duration(minutesUntil) * time.Minute
	if ttl <= 0 {
		ttl = cache.PeakHoursTTL
	}

	return output, ttl
}

// parsePeakHoursResponse parses the promoclock API JSON response.
func parsePeakHoursResponse(body []byte) (*peakHoursResponse, error) {
	var resp peakHoursResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// localPeakStatus computes peak/off-peak status from local timezone math.
// Returns: isPeak, isWeekend, minutesUntilNextTransition.
func localPeakStatus(now time.Time) (bool, bool, int) {
	loc, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		return false, false, 30
	}

	pt := now.In(loc)
	weekday := pt.Weekday()

	if weekday == time.Saturday || weekday == time.Sunday {
		return false, true, 0
	}

	hour := pt.Hour()
	minute := pt.Minute()

	if hour >= peakStartHour && hour < peakEndHour {
		minutesUntil := (peakEndHour-hour)*60 - minute
		return true, false, minutesUntil
	}

	if hour < peakStartHour {
		minutesUntil := (peakStartHour-hour)*60 - minute
		return false, false, minutesUntil
	}

	// After peak today — find next weekday 5 AM
	nextPeak := time.Date(pt.Year(), pt.Month(), pt.Day()+1, peakStartHour, 0, 0, 0, loc)
	for nextPeak.Weekday() == time.Saturday || nextPeak.Weekday() == time.Sunday {
		nextPeak = nextPeak.AddDate(0, 0, 1)
	}
	minutesUntil := int(nextPeak.Sub(pt).Minutes())
	return false, false, minutesUntil
}

// formatPeakHoursOutput builds the colored status line segment.
func formatPeakHoursOutput(colors map[string]string, isPeak, isWeekend bool, minutesUntilChange int) string {
	if isWeekend {
		return ""
	}

	reset := colors["reset"]
	var b strings.Builder

	if isPeak {
		b.WriteString(colors["red"])
		b.WriteString("▲ Peak ")
	} else {
		b.WriteString(colors["green"])
		b.WriteString("▼ Off-Peak ")
	}

	b.WriteString(formatCountdown(minutesUntilChange))
	b.WriteString(reset)

	return b.String()
}

// formatCountdown formats minutes remaining into a compact string.
func formatCountdown(minutes int) string {
	if minutes <= 0 {
		return "<1m"
	}
	if minutes >= 60 {
		return fmt.Sprintf("%dh%dm", minutes/60, minutes%60)
	}
	return fmt.Sprintf("%dm", minutes)
}
```

- [ ] **Step 4: Register in NewRegistry**

Add to `internal/plugins/interface.go` in the `NewRegistry()` function, after the existing `r.registerWithCache(&StackPlugin{})` line:

```go
r.registerWithCache(&PeakHoursPlugin{})
```

- [ ] **Step 5: Run all tests**

Run: `go test ./internal/plugins/ -run TestPeakHours -v`
Expected: PASS — all tests.

Run: `go build ./...`
Expected: Clean build.

- [ ] **Step 6: Commit**

```bash
git add internal/plugins/peakhours.go internal/plugins/peakhours_test.go internal/plugins/interface.go
git commit -m "Add peak hours plugin with API fetch, local fallback, and smart caching"
```

---

### Task 7: Full Test Suite Verification

**Files:** None modified — verification only.

- [ ] **Step 1: Run entire test suite**

Run: `go test ./... -v`
Expected: All tests pass, including all existing plugin tests.

- [ ] **Step 2: Run build**

Run: `go build ./cmd/prism/`
Expected: Clean build, binary produced.

- [ ] **Step 3: Verify API check (optional, manual)**

Run from the project root: `go run ./cmd/prism/ --help` or similar to confirm the binary works.
