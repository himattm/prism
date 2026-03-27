# Config System

Prism uses a three-tier configuration system that merges settings from global, repository, and local files. This lets you share sensible defaults across all projects while keeping personal or machine-specific overrides out of version control.

## Configuration Tiers

| Priority | File | Purpose |
|----------|------|---------|
| Highest | `.claude/prism.local.json` | Personal overrides (gitignored) |
| Medium | `.claude/prism.json` | Repo-level config (committed) |
| Lowest | `~/.claude/prism-config.json` | Global defaults |

Settings are merged from lowest to highest priority. Higher-priority values override lower-priority ones. Plugin configs under the `plugins` key are merged at the top level — each plugin's block is replaced wholesale if the same plugin key appears in a higher-priority file.

## Creating Config Files

```bash
prism init          # Creates .claude/prism.json in the current directory
prism init-global   # Creates ~/.claude/prism-config.json
```

## Config Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `configVersion` | string | — | Tracks applied migrations; managed by the installer |
| `icon` | string | `"💎"` | Icon displayed in the status line |
| `sections` | `string[]` or `string[][]` | See below | Status line sections to display |
| `autocompactBuffer` | number | `22.5` | Context percentage buffer before autocompact warning; set to `0` to disable |
| `plugins` | object | `{}` | Plugin-specific configuration keyed by plugin name |

### Default Configuration

When no `sections` field is present, Prism uses the following layout:

```json
{
  "sections": [
    ["dir", "model", "context", "usage", "git"],
    ["supabase", "vercel"],
    ["spotify"]
  ]
}
```

## Plugin Configuration

Plugin-specific settings live under the `plugins` key, keyed by plugin name:

```json
{
  "plugins": {
    "spotify": {
      "max_length": 50,
      "format": "track_artist"
    },
    "update": {
      "check_interval_hours": 12
    }
  }
}
```

See [Built-in Plugins](../plugins/built-in-plugins.md) and [Custom Plugins](../plugins/custom-plugins.md) for each plugin's available options.

## Example Configurations

### Global config (`~/.claude/prism-config.json`)

A minimal global config that sets default sections for all projects:

```json
{
  "sections": ["dir", "model", "context", "cost", "git"]
}
```

### Repo config (`.claude/prism.json`)

A full repo-level config with a multi-line layout and project-specific settings:

```json
{
  "icon": "🚀",
  "sections": [
    ["dir", "model", "context", "linesChanged", "cost", "git"],
    ["devices"]
  ],
  "android": {
    "packages": ["com.example.app.debug", "com.example.app"]
  },
  "ios": {
    "bundleIds": ["com.example.app"]
  }
}
```

### Local override (`.claude/prism.local.json`)

A personal override that changes only the icon for the local machine:

```json
{
  "icon": "🔧"
}
```

!!! tip
    Add `.claude/prism.local.json` to your global `.gitignore` so personal overrides are never accidentally committed.
