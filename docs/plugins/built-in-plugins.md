# Built-in Plugins

Prism ships with 14 native plugins written in Go. Each plugin integrates directly into the status line, with automatic caching and color-coded output.

## Overview

| Plugin | Description | Requirements |
|--------|-------------|--------------|
| `git` | Branch, dirty status, upstream tracking | `git` |
| `usage` | Auto-detect billing type, show cost or plan limits | OAuth token |
| `usage_text` | Max/Pro limits in text format | OAuth token |
| `usage_bars` | Max/Pro limits as sparkline bars | OAuth token |
| `spotify` | Currently playing track | macOS: osascript, Linux: playerctl |
| `android_devices` | Connected Android devices | `adb` |
| `agent_task_queue` | Build queue status | `tq` CLI (auto-installs via `uv`) |
| `supabase` | Local dev stack and migration status | `supabase` CLI |
| `vercel` | Latest deployment status | Vercel project linked |
| `cpu` | CPU usage sparkline | None |
| `memory` | Memory usage sparkline | None |
| `battery` | Battery level sparkline | Laptop with battery |
| `stack` | Auto-detected tech stack | None |
| `update` | Update available indicator | None |

---

## git

Shows the current git branch, dirty state, stash count, and upstream tracking status.

**Example output:**

```
main*+2 ⇣3 ⇡1
```

| Component | Meaning |
|-----------|---------|
| `main` | Current branch name (yellow) |
| `*` | Staged or unstaged changes |
| `+` | Untracked files present |
| `⇣3` | 3 commits behind upstream |
| `⇡1` | 1 commit ahead of upstream |

When HEAD is detached, the short commit hash is shown instead of a branch name.

**Configuration:** No config options. The plugin name `git` is used directly in sections.

**Cache:** 2-second TTL. Invalidated automatically when Claude becomes idle (via the `idle` hook), ensuring fresh data after tool calls that may modify the repository.

**Requirements:** `git` must be in PATH.

---

## usage

A unified plugin that auto-detects your billing type and renders the appropriate information.

- **API billing users** see cost, cache efficiency, and burn rate
- **Max/Pro plan users** see session and weekly limits (text or bars)

Detection works by checking for an OAuth token in `~/.claude/.credentials.json` or the macOS Keychain.

**Example output (API billing):**

```
$1.23 ⌁78% ~$4.50/h
```

| Component | Meaning |
|-----------|---------|
| `$1.23` | Session cost |
| `⌁78%` | Cache efficiency (cache read tokens as % of input) |
| `~$4.50/h` | Burn rate extrapolated from session data |

**Example output (Max/Pro text):**

```
4h30m:78% 5d:40% 4d:25%
```

| Segment | Meaning |
|---------|---------|
| `4h30m:78%` | 5-hour session limit: time remaining and utilization |
| `5d:40%` | 7-day weekly limit |
| `4d:25%` | Opus-specific weekly limit |

Color urgency:

| Utilization | Color |
|-------------|-------|
| Under 70% | White |
| 70--89% | Yellow |
| 90% and above | Red |

**Configuration:**

```json
{
  "plugins": {
    "usage": {
      "usage_plan": {
        "style": "text",
        "show_hours": true,
        "show_minutes": true,
        "show_days": true,
        "show_opus": true
      },
      "api_billing": {
        "decimals": 2,
        "color": "gray",
        "show_cache": true,
        "show_burn_rate": true
      }
    }
  }
}
```

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `usage_plan.style` | string | `"text"` | `"text"` or `"bars"` for Max/Pro display |
| `usage_plan.show_hours` | bool | `true` | Show 5-hour session limit |
| `usage_plan.show_minutes` | bool | `true` | Include minutes in hour time display |
| `usage_plan.show_days` | bool | `true` | Show 7-day weekly limit |
| `usage_plan.show_opus` | bool | `true` | Show Opus-specific limit |
| `api_billing.decimals` | int | `2` | Decimal places for cost display (0--10) |
| `api_billing.color` | string | `"gray"` | Color key for cost text |
| `api_billing.show_cache` | bool | `true` | Show cache efficiency indicator |
| `api_billing.show_burn_rate` | bool | `true` | Show burn rate (~$/h) |

**Cache:** 60-second in-memory TTL. On-disk cache (in temp directory) persists across process invocations with a 5-minute staleness threshold -- stale data is rendered with dimmed colors. Fresh data is only fetched when Claude is idle.

**Requirements:** OAuth token (for Max/Pro). API billing mode requires no credentials (reads cost from the session context).

!!! note "Mutual exclusion"
    Only one usage plugin renders per status line refresh. If `usage`, `usage_text`, and `usage_bars` are all in your sections config, only the first one encountered will render.

---

## usage_text

Forces Max/Pro text format regardless of the unified `usage` plugin's style setting. Useful when you want explicit control over the format.

**Example output:**

```
4h30m:78% 5d:40% 4d:25%
```

**Configuration:**

```json
{
  "plugins": {
    "usage_text": {
      "enabled": true,
      "show_hours": true,
      "show_minutes": true,
      "show_days": true,
      "show_opus": true
    }
  }
}
```

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `enabled` | bool | `true` | Enable or disable the plugin |
| `show_hours` | bool | `true` | Show 5-hour session limit |
| `show_minutes` | bool | `true` | Include minutes in hour time display |
| `show_days` | bool | `true` | Show 7-day weekly limit |
| `show_opus` | bool | `true` | Show Opus-specific limit |

**Cache:** Same as `usage` -- 60-second in-memory, 5-minute on-disk staleness.

---

## usage_bars

Forces Max/Pro bar format. Each limit is shown as a pair of colored bar characters -- one for time remaining, one for utilization level.

**Example output:**

```
▅█ ▃▆ ▂▅
```

Each pair uses distinct colors:

| Pair | Time color | Usage color |
|------|-----------|-------------|
| 5-hour session | Teal | Sky blue |
| 7-day weekly | Dark violet | Lavender |
| Opus weekly | Tangerine | Peach |

Bar characters use 8 levels of Unicode block elements (`▁▂▃▄▅▆▇█`).

**Configuration:**

```json
{
  "plugins": {
    "usage_bars": {
      "enabled": true,
      "show_hours": true,
      "show_days": true,
      "show_opus": true
    }
  }
}
```

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `enabled` | bool | `true` | Enable or disable the plugin |
| `show_hours` | bool | `true` | Show 5-hour session bars |
| `show_days` | bool | `true` | Show 7-day weekly bars |
| `show_opus` | bool | `true` | Show Opus weekly bars |

**Cache:** Same as `usage`.

---

## spotify

Displays the currently playing Spotify track.

**Example output:**

```
♫ Artist - Track Title
⏸ Artist - Track Title
```

Playing tracks use emerald color; paused tracks use gray.

**Configuration:**

```json
{
  "plugins": {
    "spotify": {
      "show_icon": true,
      "max_length": 40,
      "format": "artist_track",
      "show_when_paused": false
    }
  }
}
```

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `show_icon` | bool | `true` | Show play/pause icon prefix |
| `max_length` | int | `40` | Maximum character length for track text (0 disables truncation) |
| `format` | string | `"artist_track"` | Display format: `artist_track`, `track_artist`, or `track_only` |
| `show_when_paused` | bool | `false` | Show track info when playback is paused |

**Cache:** 3-second TTL. Invalidated when Claude becomes idle.

**Platform support:**

- **macOS:** Uses AppleScript (`osascript`) to query the Spotify application
- **Linux:** Uses `playerctl -p spotify` to query track metadata

---

## android_devices

Shows connected Android devices with configurable display fields.

**Example output:**

```
⬡ Pixel 6 (14)
```

Multiple devices are shown side by side, each prefixed with the hexagon icon.

**Configuration:**

```json
{
  "plugins": {
    "android_devices": {
      "display": "model:version",
      "packages": ["com.example.app"]
    }
  }
}
```

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `display` | string | `"serial"` | Display fields, joined with `:` |
| `packages` | array | `[]` | Package names for app version lookup (supports wildcards) |

Available display fields:

| Field | Example | ADB Property |
|-------|---------|--------------|
| `serial` | `emulator-5560` | Device serial number |
| `model` | `Pixel 6 Pro` | `ro.product.model` |
| `version` | `14` | `ro.build.version.release` |
| `sdk` | `34` | `ro.build.version.sdk` |
| `manufacturer` | `Google` | `ro.product.manufacturer` |
| `device` | `cheetah` | `ro.product.device` |
| `build` | `userdebug` | `ro.build.type` |
| `arch` | `arm64-v8a` | `ro.product.cpu.abi` |

Compound display formats the first value as the main label and the rest in parentheses. For example, `model:version` renders as `Pixel 6 (14)`.

When `packages` is configured, matching app versions are appended to each device. Wildcards are supported (e.g., `com.example.*`).

**Cache:** 10-second TTL (5x the base process TTL). Invalidated when Claude becomes idle.

**Requirements:** `adb` must be in PATH.

---

## agent_task_queue

Displays the status of tasks in the agent task queue (the `tq` CLI).

**Example output:**

```
tq: ▸ gradlew:build
tq: ▸ pytest ⧗ 2
tq: ⚠ 3 waiting (run 'tq clear')
```

| State | Display |
|-------|---------|
| Running with command | `tq: ▸ <command>` (powder blue) |
| Running, no command | `tq: ▸ <count>` |
| Waiting tasks queued | `⧗ <count>` appended |
| Stalled (waiting, none running) | `tq: ⚠ N waiting (run 'tq clear')` (red) |
| No tasks | Hidden |

Gradle commands are automatically simplified: `./gradlew :app:assembleDebug` becomes `gradlew:assembleDebug`.

**Configuration:**

```json
{
  "plugins": {
    "agent_task_queue": {
      "max_command_length": 30,
      "show_queue_count": true
    }
  }
}
```

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `max_command_length` | int | `30` | Maximum characters for the command display |
| `show_queue_count` | bool | `true` | Show count of waiting tasks |

**Auto-install:** If `tq` is not found but `uv` is available, the plugin will automatically run `uv tool install agent-task-queue` once per session.

**Cache:** 2-second TTL. Invalidated when Claude becomes idle (data cache only; the install-attempted flag persists for the session).

---

## supabase

Shows Supabase local development stack status and pending migration count.

**Example output:**

```
⚡
⚡ ↑2
```

The lightning bolt is green when the local stack is running, gray when stopped. The `↑2` suffix indicates 2 pending (unapplied) migrations.

**Configuration:**

```json
{
  "plugins": {
    "supabase": {
      "show_migrations": true,
      "show_when_stopped": false
    }
  }
}
```

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `show_migrations` | bool | `true` | Show count of unapplied migrations |
| `show_when_stopped` | bool | `false` | Show the plugin even when the local stack is not running |

Detection: the plugin checks for `supabase/config.toml` in the project directory. Migration counts are only fetched when idle.

**Cache:** 3-second TTL for the main output. 10-second TTL for migration data. Project detection is cached for 10 seconds.

**Requirements:** `supabase` CLI must be in PATH. The project must contain `supabase/config.toml`.

---

## vercel

Shows the latest Vercel deployment status for the current project.

**Example output:**

```
▲ ready
▲ building
▲ myteam: error app-abc123.vercel.app
```

**Deployment state colors:**

| State | Color |
|-------|-------|
| `READY` | Emerald |
| `BUILDING` | Yellow |
| `ERROR` | Red |
| `QUEUED` | Gray |
| `CANCELED` | Gray |

**Configuration:**

```json
{
  "plugins": {
    "vercel": {
      "show_url": false,
      "max_url_length": 30,
      "show_team": false
    }
  }
}
```

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `show_url` | bool | `false` | Append the deployment URL |
| `max_url_length` | int | `30` | Maximum characters for the URL (truncated with ellipsis) |
| `show_team` | bool | `false` | Prefix output with the Vercel team/user name |

**Cache:** 10-second TTL for stable states (`READY`, `ERROR`, `CANCELED`). 3-second TTL during transient states (`BUILDING`, `QUEUED`) for faster polling. Team name is cached for 5 minutes.

**Requirements:** `vercel` CLI must be in PATH. The project must contain `.vercel/project.json` with a valid `projectId`.

---

## cpu

Displays CPU usage as a sparkline with per-session history.

**Example output:**

```
CPU ▁▃▅▇█▅▃▂ 34%
```

Color thresholds:

| Usage | Color |
|-------|-------|
| Under 70% | Gray |
| 70--89% | Yellow |
| 90% and above | Crimson |

**Configuration:** No config options.

**Platform support:**

- **Linux:** Reads `/proc/stat` and computes delta CPU usage between samples
- **macOS:** Sums per-process CPU% via `ps -A -o %cpu` and normalizes by core count; falls back to `sysctl vm.loadavg` if `ps` fails

**Cache:** 2-second TTL. Sparkline history persists for the duration of the session.

---

## memory

Displays memory usage as a sparkline with per-session history.

**Example output:**

```
MEM ▁▃▅▇█▅▃▂ 62%
```

Color thresholds are the same as `cpu`.

**Configuration:** No config options.

**Platform support:**

- **Linux:** Reads `/proc/meminfo` (MemTotal minus MemAvailable)
- **macOS:** Uses `sysctl hw.memsize` for total memory and `vm_stat` to calculate available memory (free + inactive + speculative + purgeable pages)

**Cache:** 2-second TTL. Sparkline history persists for the session.

---

## battery

Displays battery level as a sparkline with charging state.

**Example output:**

```
BAT ▂▅▆█▃ 78%
CHG ▂▅▆█▃ 78%
```

| Label | Meaning |
|-------|---------|
| `BAT` | Running on battery |
| `CHG` | Charging or fully charged |

When charging, the output is colored emerald. When on battery, the color follows inverted thresholds (low battery = critical):

| Level | Color |
|-------|-------|
| Above 30% | Gray |
| 11--30% | Yellow |
| 10% and below | Crimson |

Hidden on desktops and servers without a battery.

**Configuration:** No config options.

**Platform support:**

- **Linux:** Reads `/sys/class/power_supply/BAT*/capacity` and `/sys/class/power_supply/BAT*/status`
- **macOS:** Parses `pmset -g batt` output for percentage and charging state

**Cache:** 10-second TTL. Sparkline history persists for the session.

---

## stack

Auto-detects the project's technology stack by scanning for marker files.

**Example output:**

```
Next.js/Supabase/Docker/Node
```

Technologies are colored individually and separated by a dimmed `/`. Detection is ordered by specificity -- frameworks are detected before runtimes, and suppressions prevent redundant entries (e.g., detecting Next.js suppresses plain React).

**Detected technologies include:**

- **Frameworks:** Next.js, Nuxt, SvelteKit, Remix, Astro, Vite, React, Vue, Svelte, Angular, Django, Flask, Rails, Laravel
- **Services:** Supabase, Vercel, Netlify, Firebase, Prisma, Drizzle, Terraform, Pulumi, AWS CDK
- **Infrastructure:** Docker, K8s, Turborepo, Nx
- **Runtimes:** Go, Rust, Python, Java, Ruby, Elixir, Zig, Node, Deno, Bun

**Configuration:**

```json
{
  "plugins": {
    "stack": {
      "max_items": 4,
      "hide": ["Docker", "Node"]
    }
  }
}
```

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `max_items` | int | `4` | Maximum number of technologies to display |
| `hide` | array | `[]` | Technology names to exclude from output |

**Cache:** 30-second TTL. Invalidated on session start (via the `session_start` hook) to ensure fresh detection when switching projects.

---

## update

Shows a yellow arrow indicator when a newer version of Prism is available.

**Example output:**

```
⬆
```

Hidden when no update is available. The plugin checks the GitHub releases API and compares semver versions.

**Configuration:**

```json
{
  "plugins": {
    "update": {
      "enabled": true,
      "check_interval_hours": 8
    }
  }
}
```

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `enabled` | bool | `true` | Enable or disable update checking |
| `check_interval_hours` | int | `8` | Hours between update checks |

**Cache:** File-based cache in the temp directory. The plugin only fetches from the network when idle, using stale cached data during active sessions. The cache is invalidated when the local version changes (e.g., after a manual update).

**Auto-update:** When enabled (default), the plugin will automatically download and install updates during idle periods via a detached background process. A one-per-day notification is shown when an update is available.
