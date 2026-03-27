# Peak Hours Plugin Design

## Overview

A native Prism plugin that shows Claude's current peak/off-peak status in the status line, helping users understand when their session limits drain faster (peak) vs normal rate (off-peak).

## Background

Anthropic adjusts Claude session limits during peak hours (weekdays 5-11 AM Pacific Time). During peak, the token cost per session is higher, so the 5-hour rolling session allowance depletes faster. Off-peak hours function normally. Weekends are entirely off-peak.

## Data Source

Primary: [promoclock.co/api/status](https://promoclock.co/api/status) — returns JSON:

```json
{
  "status": "peak",
  "isPeak": true,
  "isOffPeak": false,
  "isWeekend": false,
  "sessionLimitSpeed": "faster_than_normal",
  "emoji": "...",
  "label": "Peak Hours — Limits Drain Faster",
  "peakHours": "Weekdays 5am–11am PT / 1pm–7pm GMT",
  "nextChange": "2026-03-27T19:00:00.000Z",
  "minutesUntilChange": 190,
  "timestamp": "2026-03-27T15:49:21.359Z",
  "utcHour": 15,
  "utcDay": 5,
  "note": "No known end date for peak hours adjustment. Weekly limits unchanged."
}
```

Fallback: Local timezone calculation against `America/Los_Angeles` when the API is unreachable.

## Output Format

### Display States

| State | Output | Color |
|-------|--------|-------|
| Peak (weekday, 5-11 AM PT) | `▲ Peak 2h38m` | Red |
| Off-Peak (weekday, outside 5-11 AM PT) | `▼ Off-Peak 4h12m` | Green |
| Weekend | Empty string (plugin hidden) | N/A |

### Countdown Format

| Minutes Remaining | Display |
|-------------------|---------|
| > 60 | `2h38m` |
| 1–60 | `38m` |
| < 1 | `<1m` |

### Icons

- `▲` (U+25B2) — peak: up arrow = high demand, limits drain faster
- `▼` (U+25BC) — off-peak: down arrow = low demand, normal limits

## Architecture

### Plugin Struct

`PeakHoursPlugin` implements `NativePlugin` interface:
- `Name() string` — returns `"peakhours"`
- `Execute(ctx context.Context, input plugin.Input) (string, error)` — main logic
- `SetCache(c *cache.Cache)` — receives shared cache

No hooks needed. Simple Execute-only pattern.

### Execute Flow

1. Check cache for key `peakhours:status`
2. **Cache hit** — return cached output (may be empty string for weekends)
3. **Cache miss** — call `https://promoclock.co/api/status` with 2s timeout
4. **API success** — parse JSON response:
   - If `isWeekend` is true — cache empty string with TTL = `minutesUntilChange`, return `""`
   - Otherwise — format output, cache with TTL = `minutesUntilChange` from response
5. **API failure** — fall back to local timezone calculation, compute minutes until next boundary, cache with that TTL

### Smart Caching Strategy

The cache TTL is derived directly from the API's `minutesUntilChange` field. This means:
- If we're 3 hours from a transition, the cache lives for 3 hours
- API is only called at startup (cold cache) and at each peak/off-peak transition
- Typical day: 2-4 API calls total (one per transition)
- Weekend: 0 API calls (plugin returns empty immediately)

A fallback default TTL constant (`PeakHoursTTL`) is defined in `internal/cache/cache.go` for cases where the API response is missing `minutesUntilChange`. Default: 30 minutes.

### Local Fallback Logic

When the API is unreachable:
1. Load `America/Los_Angeles` timezone
2. Get current time in PT
3. Check: is it a weekday AND between 5:00 AM and 11:00 AM PT?
   - **Yes (peak)** — calculate minutes until 11:00 AM PT
   - **No, weekday off-peak** — calculate minutes until next 5:00 AM PT (same day if before 5 AM, next day if after 11 AM)
   - **Weekend** — return empty string
4. Format output identically to the API-driven path
5. Cache with TTL = calculated minutes until next transition

The fallback output is indistinguishable from the API-driven output to the user.

## Configuration

No configuration options. The plugin is opinionated with sensible defaults.

## File Structure

| File | Purpose |
|------|---------|
| `internal/plugins/peakhours.go` | Plugin implementation |
| `internal/plugins/peakhours_test.go` | Unit tests |
| `internal/cache/cache.go` | Add `PeakHoursTTL` constant |
| `internal/plugins/interface.go` | Register in `NewRegistry()` |

## Registration

Add to `NewRegistry()` in `internal/plugins/interface.go`:
```go
r.registerWithCache(&PeakHoursPlugin{})
```

## Testing

| Test | Purpose |
|------|---------|
| `TestPeakHoursName` | Name returns `"peakhours"` |
| `TestFormatCountdown` | Countdown formatting: hours+minutes, minutes only, <1m |
| `TestLocalFallback` | Timezone math produces correct peak/off-peak for known times |
| `TestWeekendReturnsEmpty` | Weekend returns `""` |
| `TestParseAPIResponse` | Correct parsing of promoclock JSON fields |
