# Quick Start

## Your First Status Line

After installation, start a new Claude Code session. Prism renders a status line at the bottom of the terminal that updates as you work. With the default configuration you will see something like:

```
myproject  Opus 4.6  ████░░░░▒▒ 56%  $1.23 ⌁78% ~$4.50/h  main*+ ⇣3 ⇡1
```

Each section is described below.

| Section | Example | Description |
|---|---|---|
| Directory | `myproject` | Name of the current project directory |
| Model | `Opus 4.6` | Active Claude model |
| Context | `████░░░░▒▒ 56%` | Context window usage, visualized as a progress bar. Color shifts from white to yellow to red as usage rises. |
| Usage (API) | `$1.23 ⌁78% ~$4.50/h` | Session cost, cache efficiency, and estimated burn rate per hour |
| Usage (Max/Pro) | `4h30m:78% 5d:40% 4d:25%` | Time remaining in session limit, weekly limit, and Opus weekly limit |
| Git | `main*+ ⇣3 ⇡1` | Branch name, dirty flag (*), untracked files (+), commits behind (⇣n) and ahead (⇡n) |

Prism automatically detects whether you are on an API billing plan or a Max/Pro subscription and displays the appropriate usage format.

## Customizing Your Layout

### Project Config

To customize the sections shown in a specific project, run:

```bash
prism init
```

This creates `.claude/prism.json` in your current directory. Open it and edit the `sections` array to control which sections appear and in what order:

```json
{
  "sections": ["dir", "model", "context", "usage", "git"]
}
```

Available section names correspond to the built-in plugins. See [Built-in Plugins](../plugins/built-in-plugins.md) for the full list.

### Multi-line Layout

You can spread sections across multiple lines by nesting arrays:

```json
{
  "sections": [
    ["dir", "model", "context", "usage", "git"],
    ["spotify"]
  ]
}
```

Each inner array becomes one line in the status display.

## Exploring Colors

To see the available ANSI colors with a visual demo, run:

```bash
prism refract
```

This prints all supported colors so you can reference them when configuring plugin color settings.

## Next Steps

- [Configuration](../configuration/config-system.md) — full reference for global and project config options
- [Plugins](../plugins/built-in-plugins.md) — all available built-in plugins and their options
- [Hooks](../hooks.md) — how Prism reacts to Claude Code lifecycle events
