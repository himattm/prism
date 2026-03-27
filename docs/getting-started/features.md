# Features

An overview of everything Prism can do.

## Smart Billing Detection

Prism automatically detects whether you are authenticated via an API key or a Max/Pro OAuth token and renders usage information accordingly.

**API billing** shows:

- Session cost in dollars (`$1.23`)
- Cache efficiency with the `⌁` symbol (`⌁78%`) — higher is better
- Estimated burn rate per hour (`~$4.50/h`)

**Max/Pro plans** show:

- Time remaining in the session limit with usage percentage (`4h30m:78%`)
- Weekly limit with usage percentage (`5d:40%`)
- Opus weekly limit when applicable

Color urgency applies to both modes:

| Usage | Color |
|---|---|
| Under 70% | White |
| 70–89% | Yellow |
| 90% and above | Red |

## Context Window Visualization

The context section renders a visual progress bar using Unicode block characters, giving you an at-a-glance view of how much of the context window you have consumed.

- Supports standard context windows and extended 1M-token context windows
- Uses the same white / yellow / red urgency color system as usage

## Git Integration

The git section surfaces the state of your repository without leaving Claude Code:

- Branch name
- Dirty status (`*`) when there are uncommitted changes
- Untracked files indicator (`+`) when untracked files are present
- Upstream tracking: commits behind (`⇣n`) and commits ahead (`⇡n`)

Worktree sessions are indicated with a `⎇` prefix in the directory section, making it easy to identify which worktree you are working in.

## Plugin Ecosystem

Prism ships with 14 built-in native Go plugins covering common development workflows. Additional plugins can be added from the community or written yourself.

| Plugin type | Description |
|---|---|
| Built-in | Compiled into Prism; zero external dependencies, maximum performance |
| Script plugins | Shell scripts that print a single line of output |
| Binary plugins | Compiled executables that follow the Prism plugin protocol |

The plugin manager CLI makes it easy to manage third-party plugins:

```bash
prism plugin add <source>
prism plugin list
prism plugin update
prism plugin remove <name>
```

Plugins execute in parallel so a slow plugin does not block the rest of the status line from rendering.

## Multi-line Layouts

Status information can be organized across multiple lines by using nested arrays in the `sections` config:

```json
{
  "sections": [
    ["dir", "model", "context", "usage", "git"],
    ["spotify"]
  ]
}
```

This keeps the primary information on the first line and secondary or auxiliary plugins on subsequent lines.

## Hook System

Prism registers handlers for all 11 Claude Code lifecycle events:

| Hook | Async | Purpose |
|---|---|---|
| `UserPromptSubmit` | No | Mark status as busy |
| `Stop` | Yes | Mark status as idle |
| `SessionStart` | Yes | Initialize session state |
| `SessionEnd` | Yes | Flush session state |
| `PreCompact` | No | Pre-compact housekeeping |
| `Setup` | No | One-time setup on session init |
| `PreToolUse` | No | React before a tool runs |
| `PostToolUse` | Yes | Invalidate caches after tool use |
| `PermissionRequest` | No | Handle permission prompts |
| `Notification` | Yes | Process Claude notifications |
| `SubagentStop` | Yes | React when a subagent finishes |

Hooks drive cache invalidation, state transitions, and any plugin behavior that needs to respond to Claude Code events.

## Auto-Updates

Prism keeps itself up to date without requiring manual intervention:

- Checks for new releases every 8 hours (configurable)
- Downloads and installs updates in the background while Claude is idle
- Does not interrupt active sessions

The update interval and auto-update behavior are configurable. See [Configuration](../configuration/config-system.md) for details.

## Caching

Prism uses an intelligent caching layer to avoid redundant work during rapid status line refreshes:

- Each plugin has its own configurable TTL
- Cache entries are invalidated by relevant hook events (for example, `PostToolUse` invalidates git state)
- Cached results are served instantly, keeping the status line responsive even under heavy tool use
