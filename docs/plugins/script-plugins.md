# Script Plugins

Script plugins extend Prism without modifying Go source code. Write a shell script, drop it in the plugins directory, and reference it by name in your sections config.

## Overview

Script plugins receive JSON on stdin with session context, configuration, and color codes. They print a single line of formatted text to stdout. Exit 0 with output to show the section; exit 0 with no output to hide it; exit non-zero on error (section hidden, error logged).

## Plugin Directory

```
~/.claude/prism-plugins/
```

Prism scans this directory on startup. The directory is created automatically if it does not exist.

## Naming Convention

```
prism-plugin-<name>.sh
```

The `<name>` portion becomes the plugin identifier used in your sections config. For example, `prism-plugin-weather.sh` is referenced as `weather` in sections.

## Input Format

Plugins receive a JSON object on stdin with the following structure:

```json
{
  "prism": {
    "version": "0.10.1",
    "project_dir": "/path/to/project",
    "current_dir": "/path/to/project/src",
    "session_id": "abc123",
    "is_idle": false
  },
  "session": {
    "model": "Opus 4.6",
    "context_pct": 56,
    "cost_usd": 1.23,
    "lines_added": 100,
    "lines_removed": 45,
    "input_tokens": 50000,
    "output_tokens": 12000,
    "cache_creation_tokens": 8000,
    "cache_read_tokens": 35000,
    "context_window_size": 200000
  },
  "config": {},
  "colors": {
    "red": "\u001b[31m",
    "green": "\u001b[32m",
    "yellow": "\u001b[33m",
    "reset": "\u001b[0m"
  }
}
```

| Field | Description |
|-------|-------------|
| `prism.version` | Prism version string |
| `prism.project_dir` | Root project directory |
| `prism.current_dir` | Current working directory |
| `prism.session_id` | Unique session identifier |
| `prism.is_idle` | `true` when Claude is idle (safe for expensive operations) |
| `session.model` | Active Claude model |
| `session.context_pct` | Context window usage (0--100) |
| `session.cost_usd` | Session cost in USD |
| `config` | Plugin-specific configuration from `prism.json` |
| `colors` | Map of ANSI color escape codes |

## Output

Print a single line to stdout. Use the ANSI color codes from the `colors` input for consistent styling across terminals. Always reset colors at the end of your output.

## Metadata Headers

Add metadata as `@`-prefixed comments in the first 20 lines of your script. These are parsed by the plugin manager for listing, update checking, and installation.

```bash
#!/bin/bash
# @name weather
# @version 1.0.0
# @description Shows current weather
# @author Your Name
# @source https://github.com/user/prism-plugin-weather
# @update-url https://raw.githubusercontent.com/user/prism-plugin-weather/main/prism-plugin-weather.sh
```

| Header | Purpose |
|--------|---------|
| `@name` | Plugin identifier (should match filename) |
| `@version` | Semver version for update checking |
| `@description` | Short description shown in `prism plugin list` |
| `@author` | Plugin author |
| `@source` | Repository or homepage URL |
| `@update-url` | Direct URL to the latest version of the script (used by `prism plugin update`) |

## Example: Weather Plugin

This example demonstrates the full plugin interface -- reading input, parsing config, caching, and formatted output:

```bash
#!/bin/bash
# Prism Plugin: Weather (Example)
# Shows current temperature for a configured location
#
# This is an example plugin demonstrating the Prism plugin interface.
# Copy to ~/.claude/prism-plugins/ and customize for your needs.
#
# Config in .claude/prism.json:
#   {
#     "sections": ["dir", "model", "weather", "git"],
#     "plugins": {
#       "weather": {
#         "location": "San Francisco",
#         "units": "imperial"
#       }
#     }
#   }
#
# Plugin Interface:
# - INPUT:  JSON on stdin with prism context, session info, config, and colors
# - OUTPUT: Formatted text with ANSI codes on stdout
# - Exit 0 with output to show section
# - Exit 0 with no output to hide section
# - Exit non-zero on error (section hidden, error logged)

set -e

# Read full input JSON from stdin
INPUT=$(cat)

# Parse plugin config (with defaults)
LOCATION=$(echo "$INPUT" | jq -r '.config.weather.location // "New York"')
UNITS=$(echo "$INPUT" | jq -r '.config.weather.units // "imperial"')

# Parse colors for consistent styling
CYAN=$(echo "$INPUT" | jq -r '.colors.cyan')
GRAY=$(echo "$INPUT" | jq -r '.colors.gray')
RESET=$(echo "$INPUT" | jq -r '.colors.reset')

# Check if session is idle (safe to run expensive operations)
IS_IDLE=$(echo "$INPUT" | jq -r '.prism.is_idle')

# Cache file for weather data (avoid hitting API on every status update)
CACHE="/tmp/prism-weather-cache"
CACHE_MAX_AGE=300  # 5 minutes

# Check cache first
if [ -f "$CACHE" ]; then
    cache_age=$(($(date +%s) - $(stat -f %m "$CACHE" 2>/dev/null || echo 0)))
    if [ "$cache_age" -lt "$CACHE_MAX_AGE" ]; then
        cat "$CACHE"
        exit 0
    fi
fi

# Only fetch new data when session is idle
if [ "$IS_IDLE" != "true" ]; then
    # Return cached value or nothing when busy
    [ -f "$CACHE" ] && cat "$CACHE"
    exit 0
fi

# Fetch weather (using wttr.in for simplicity)
# In a real plugin, you might use a proper weather API
TEMP=$(timeout 2 curl -sf "wttr.in/${LOCATION}?format=%t" 2>/dev/null || echo "")

# Exit silently if fetch failed (section will be hidden)
[ -z "$TEMP" ] && exit 0

# Format and cache the output
OUTPUT="${CYAN}${TEMP}${RESET}"
echo "$OUTPUT" > "$CACHE"

# Output the formatted section
echo -e "$OUTPUT"
```

Key patterns demonstrated:

- **Read config with defaults** using `jq` with the `//` fallback operator
- **Use provided colors** rather than hardcoding ANSI codes
- **Implement file-based caching** to avoid repeated API calls
- **Check `is_idle`** before expensive network operations
- **Exit silently** on failure (output nothing, exit 0)

## Plugin Manager

Prism includes a built-in plugin manager for installing, updating, and removing community plugins.

### Commands

```bash
prism plugin list              # List all plugins (native + community)
prism plugin add <url>         # Install from GitHub or direct URL
prism plugin check-updates     # Check for available updates
prism plugin update <name>     # Update a specific plugin
prism plugin update --all      # Update all plugins
prism plugin remove <name>     # Remove a community plugin
```

### Installing from GitHub

When you pass a GitHub repository URL, the plugin manager first tries to download a pre-built binary for your platform from the latest GitHub release. If no binary is found, it falls back to downloading the script from the repository's `main` branch.

```bash
prism plugin add https://github.com/user/prism-plugin-weather
```

Binary assets must follow the naming convention `prism-plugin-<name>-<os>-<arch>` (e.g., `prism-plugin-weather-darwin-arm64`).

### Installing from a direct URL

You can also install from any URL that serves a script or binary:

```bash
prism plugin add https://example.com/prism-plugin-mything.sh
```

## Binary Plugins

Compiled plugins work the same way as scripts -- they receive JSON on stdin and print output to stdout. The only difference is the metadata mechanism: instead of header comments, binary plugins use a sidecar JSON file.

Place the metadata file alongside the binary with a `.json` extension:

```
~/.claude/prism-plugins/prism-plugin-myplugin        # the binary
~/.claude/prism-plugins/prism-plugin-myplugin.json    # metadata
```

Sidecar metadata format:

```json
{
  "name": "myplugin",
  "version": "1.0.0",
  "description": "What it does",
  "author": "Your Name",
  "source": "https://github.com/user/prism-plugin-myplugin",
  "update_url": "https://api.github.com/repos/user/prism-plugin-myplugin/releases/latest"
}
```

The `update_url` for binary plugins should point to the GitHub releases API endpoint. The plugin manager will automatically download the correct platform-specific binary during updates.

## Using in Config

Reference your plugin by name in the `sections` array of `prism.json`:

```json
{
  "sections": [
    ["dir", "model", "context", "usage", "git"],
    ["weather"]
  ]
}
```

Plugin-specific configuration goes under the `plugins` key:

```json
{
  "sections": [
    ["dir", "model", "context", "usage", "git"],
    ["weather"]
  ],
  "plugins": {
    "weather": {
      "location": "San Francisco",
      "units": "imperial"
    }
  }
}
```

The `config` field in the plugin's JSON input will contain the value at `plugins.<name>` -- in this case, `{"location": "San Francisco", "units": "imperial"}`.

## Writing Tips

- **Use `jq` for JSON parsing.** It is the simplest way to extract fields from the input.
- **Implement caching.** Script plugins are executed on every status line refresh. Use file-based caching with a reasonable TTL.
- **Check `is_idle` before network calls.** Serve cached data when Claude is busy.
- **Keep execution fast.** Plugins that take too long will be killed by the execution timeout.
- **Exit cleanly.** Return exit code 0 even when you have nothing to display. Non-zero exits are logged as errors.
- **Test locally.** Pipe sample JSON into your script to verify it works:

```bash
echo '{"prism":{"is_idle":true},"session":{},"config":{},"colors":{"cyan":"\033[36m","reset":"\033[0m"}}' | bash ~/.claude/prism-plugins/prism-plugin-weather.sh
```
