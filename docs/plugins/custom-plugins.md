# Custom Plugins

Native plugins are written in Go and compiled directly into Prism. They have full access to the cache system, hook lifecycle, and ANSI color map.

## Plugin Interface

Every native plugin must implement the `NativePlugin` interface defined in `internal/plugins/interface.go`:

```go
type NativePlugin interface {
    Name() string
    Execute(ctx context.Context, input plugin.Input) (string, error)
    SetCache(c *cache.Cache)
}
```

| Method | Purpose |
|--------|---------|
| `Name()` | Returns the plugin's identifier used in sections config |
| `Execute()` | Runs on each status line refresh; returns the formatted string to display |
| `SetCache()` | Receives a shared cache instance at registration time |

## Hookable Interface

Plugins that need to respond to lifecycle events can optionally implement the `Hookable` interface:

```go
type Hookable interface {
    OnHook(ctx context.Context, hookType HookType, hookCtx HookContext) (string, error)
}
```

Hook types include:

| Hook | When it fires |
|------|---------------|
| `session_start` | Session started or resumed |
| `session_end` | Session ending |
| `busy` | User submitted a prompt |
| `idle` | Claude finished responding |
| `notification` | Claude Code sends a notification |
| `permission_request` | Permission dialog shown |
| `pre_tool_use` | Before tool calls |
| `post_tool_use` | After tool calls complete |
| `subagent_stop` | Subagent task completed |
| `pre_compact` | Before context compaction |
| `setup` | Repository init/maintenance |

The `HookContext` struct provides:

```go
type HookContext struct {
    SessionID string
    AgentType string         // e.g., "coder", "researcher"
    Config    map[string]any // Plugin configuration
}
```

## Plugin Input

The `Execute` method receives a `plugin.Input` struct containing all context needed for rendering. The struct is defined in `internal/plugin/types.go`:

```go
type Input struct {
    Prism   PrismContext      `json:"prism"`
    Session SessionContext    `json:"session"`
    Config  map[string]any    `json:"config"`
    Colors  map[string]string `json:"colors"`
}
```

### PrismContext

| Field | Type | Description |
|-------|------|-------------|
| `Version` | `string` | Prism version (e.g., `"0.10.1"`) |
| `ProjectDir` | `string` | Root project directory |
| `CurrentDir` | `string` | Current working directory |
| `SessionID` | `string` | Unique session identifier |
| `IsIdle` | `bool` | Whether Claude is idle (safe for expensive operations) |

### SessionContext

| Field | Type | Description |
|-------|------|-------------|
| `Model` | `string` | Active model name (e.g., `"Opus 4.6"`) |
| `ContextPct` | `int` | Context window usage percentage (0--100) |
| `CostUSD` | `float64` | Session cost in USD |
| `LinesAdded` | `int` | Lines added in the session |
| `LinesRemoved` | `int` | Lines removed in the session |
| `InputTokens` | `int` | Total input tokens consumed |
| `OutputTokens` | `int` | Total output tokens generated |
| `CacheCreationTokens` | `int` | Tokens used to create cache entries |
| `CacheReadTokens` | `int` | Tokens read from cache |
| `ContextWindowSize` | `int` | Total context window size in tokens |

### Config

`Config` is a `map[string]any` containing the plugin's configuration from `prism.json`. Access your plugin's config by key:

```go
if cfg, ok := input.Config["my_plugin"].(map[string]any); ok {
    if v, ok := cfg["some_option"].(bool); ok {
        // use v
    }
}
```

### Colors

`Colors` is a `map[string]string` of ANSI escape codes. Always use these instead of hardcoding escape sequences -- Prism manages color availability and terminal compatibility.

Common color keys include: `red`, `green`, `yellow`, `white`, `gray`, `reset`, `dim`, `emerald`, `teal`, `sky_blue`, `crimson`, `coral`, `violet`, and many more.

Always append `reset` after colored text to avoid color bleed:

```go
result := input.Colors["emerald"] + "my output" + input.Colors["reset"]
```

## Creating a Plugin

### 1. Create the source file

Add a new file in `internal/plugins/`. The file should define a struct, implement the three interface methods, and handle caching.

### 2. Register the plugin

In `internal/plugins/interface.go`, add your plugin to the `NewRegistry()` function:

```go
func NewRegistry() *Registry {
    c := cache.New()
    r := &Registry{
        plugins: make(map[string]NativePlugin),
        cache:   c,
    }

    // ... existing plugins ...
    r.registerWithCache(&MyPlugin{})

    return r
}
```

### 3. Minimal example

Here is a complete minimal plugin that shows a greeting:

```go
package plugins

import (
    "context"
    "fmt"

    "github.com/himattm/prism/internal/cache"
    "github.com/himattm/prism/internal/plugin"
)

// HelloPlugin displays a simple greeting
type HelloPlugin struct {
    cache *cache.Cache
}

func (p *HelloPlugin) Name() string { return "hello" }

func (p *HelloPlugin) SetCache(c *cache.Cache) { p.cache = c }

func (p *HelloPlugin) Execute(ctx context.Context, input plugin.Input) (string, error) {
    cacheKey := "hello:output"

    // Check cache
    if p.cache != nil {
        if cached, ok := p.cache.Get(cacheKey); ok {
            return cached, nil
        }
    }

    // Build output
    green := input.Colors["emerald"]
    reset := input.Colors["reset"]
    output := fmt.Sprintf("%sHello from Prism%s", green, reset)

    // Cache for 10 seconds
    if p.cache != nil {
        p.cache.Set(cacheKey, output, cache.ConfigTTL)
    }

    return output, nil
}
```

### 4. Adding hook support

To respond to lifecycle events, implement the `Hookable` interface on the same struct:

```go
func (p *HelloPlugin) OnHook(ctx context.Context, hookType HookType, hookCtx HookContext) (string, error) {
    if hookType == HookIdle && p.cache != nil {
        p.cache.DeleteByPrefix("hello:")
    }
    return "", nil
}
```

## Best Practices

**Use caching for expensive operations.** Any plugin that shells out to a command, reads files, or makes network calls should cache results. Use the shared cache with an appropriate TTL:

- `cache.GitTTL` (2s) -- for fast-changing process data
- `cache.ConfigTTL` (10s) -- for configuration-like data that changes infrequently
- Custom durations via `time.Duration` for specific needs

**Return empty string when nothing to display.** If your plugin has nothing relevant to show (tool not installed, no data available, wrong platform), return `""` with a `nil` error. The section will be hidden from the status line.

**Use `input.Colors` for all ANSI colors.** Never hardcode escape sequences. The color map handles terminal capability detection.

**Check `input.Prism.IsIdle` before expensive operations.** Network calls, slow shell commands, and file system scans should only run when idle. Serve cached or stale data while Claude is busy to keep the status line responsive.

**Handle context cancellation.** The `ctx` parameter carries a timeout. Pass it through to `exec.CommandContext` and check `ctx.Err()` in long-running operations.

**Keep execution under the timeout.** Plugins that take too long will be killed. Use `context.WithTimeout` to set explicit deadlines on subprocesses and network calls.

## Testing

Plugin tests follow standard Go testing patterns. Here is a test skeleton based on the existing test conventions:

```go
package plugins

import (
    "context"
    "testing"
    "time"

    "github.com/himattm/prism/internal/cache"
    "github.com/himattm/prism/internal/plugin"
)

func TestHelloPlugin_Name(t *testing.T) {
    p := &HelloPlugin{}
    if p.Name() != "hello" {
        t.Errorf("expected name 'hello', got '%s'", p.Name())
    }
}

func TestHelloPlugin_Execute_Caching(t *testing.T) {
    p := &HelloPlugin{}
    c := cache.New()
    p.SetCache(c)

    // Pre-populate cache
    expected := "[emerald]Hello from Prism[reset]"
    c.Set("hello:output", expected, time.Minute)

    input := plugin.Input{
        Colors: map[string]string{
            "emerald": "[emerald]",
            "reset":   "[reset]",
        },
    }

    result, err := p.Execute(context.Background(), input)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if result != expected {
        t.Errorf("expected '%s', got '%s'", expected, result)
    }
}

func TestHelloPlugin_OnHook_InvalidatesCache(t *testing.T) {
    p := &HelloPlugin{}
    c := cache.New()
    p.SetCache(c)

    c.Set("hello:output", "cached", time.Minute)

    _, err := p.OnHook(context.Background(), HookIdle, HookContext{})
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }

    if _, ok := c.Get("hello:output"); ok {
        t.Error("cache should be invalidated after idle hook")
    }
}
```

Run tests with:

```bash
go test ./internal/plugins/ -run TestHello -v
```
