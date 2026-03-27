# Sections

Sections are the individual units of information that make up the status line. Each section has a name that you use in the `sections` config field. Built-in sections are rendered directly by Prism; plugin sections are executed via the plugin system.

## Built-in Sections

### `dir` — Project Directory

Shows the project name derived from the root directory.

- When inside a subdirectory, shows `project/subdir`
- In git worktrees, prefixes the display with the `⎇` symbol

### `model` — Claude Model

Displays the active Claude model name (for example, `Opus 4.6` or `Sonnet 4.5`).

### `context` — Context Window Usage

Renders a visual progress bar using Unicode block characters:

```
████░░░░▒▒ 56%
```

- Block characters represent the percentage of the context window consumed
- Supports both standard and extended (1M-token) context windows

Color urgency matches the usage section:

| Usage | Color |
|-------|-------|
| Under 70% | White |
| 70–89% | Yellow |
| 90% and above | Red |

### `usage` — Billing and Usage

Automatically detects your billing type via the OAuth token and renders accordingly.

**API billing** shows cost, cache efficiency, and burn rate:

```
$1.23 ⌁78% ~$4.50/h
```

**Max/Pro plans** show session, weekly, and Opus limits:

```
4h30m:78% 5d:40% 4d:25%
```

See [Built-in Plugins](../plugins/built-in-plugins.md) for full configuration options.

### `linesChanged` — Code Changes

Shows uncommitted line changes since the last commit:

```
+123 -45
```

Additions are shown in green; deletions are shown in red.

### `git` — Git Status

Surfaces repository state without leaving Claude Code. See [Built-in Plugins](../plugins/built-in-plugins.md) for full details.

## Plugin Sections

Any registered plugin name can be used as a section. Plugins that return an empty string are silently omitted from the status line.

See [Built-in Plugins](../plugins/built-in-plugins.md), [Script Plugins](../plugins/script-plugins.md), and [Custom Plugins](../plugins/custom-plugins.md) for available plugins and how to add your own.

## Legacy Aliases

!!! warning
    `cost` is a legacy alias for `usage` that only displays API billing information. Use `usage` instead so Prism can auto-detect your billing type and render the appropriate output.
