# Pi coding agent

Prism works in the [Pi coding agent](https://pi.dev/) as well as Claude Code,
from the same binary and the same config files.

## How it works

Claude Code and Pi render status lines very differently:

| | Claude Code | Pi |
|---|---|---|
| Status line is… | an **external command** | an **in-process extension** |
| Mechanism | runs your binary, pipes JSON to stdin, prints stdout | loads a TypeScript module and calls `ctx.ui.setStatus(...)` |

Pi has no "run this command for my status line" hook, so a Go binary can't be
wired in directly. Instead, Prism ships a tiny TypeScript extension **embedded
inside the `prism` binary**. When you install it, Prism writes that extension
into Pi's extension directory. At runtime the extension collects Pi's live
session data (model, context usage, working directory, session id), hands it to
the `prism` binary using the **same JSON contract Prism already speaks**, and
pushes the rendered output into Pi's footer.

The Go renderer is unchanged — there's no separate "Pi mode" to maintain. Your
`prism.json` / `prism-config.json` files apply in Pi exactly as they do in
Claude Code.

## Installation

If Pi (`~/.pi`) is present when you run the Prism installer, the Pi extension is
set up automatically. Otherwise, install it any time with:

```bash
~/.claude/prism install-pi
```

Then restart Pi (or start a new session). To remove it:

```bash
~/.claude/prism uninstall-pi
```

The extension is written to `~/.pi/agent/extensions/prism/`, alongside a
`prism-bin.txt` recording which `prism` binary to invoke. Set the `PRISM_BIN`
environment variable to override that path.

## Default layout in Pi

Pi doesn't expose Anthropic-specific data — plan usage limits, per-message API
cost, or Anthropic peak-hour windows — so Prism's **default** Pi layout omits the
`usage`, `cost`, and `peakhours` sections:

```
dir · model · context · git
supabase · vercel · android
spotify
```

Everything else (git status, the context bar, project-tooling plugins) works the
same as in Claude Code. If you set `"sections"` explicitly in your config, your
layout is used as-is in both agents.

## What carries over

- **Idle/busy indicator** — the extension maps Pi's `agent_start` / `agent_end`
  events to Prism's existing idle/busy markers, so idle-gated behavior (like
  auto-update) works the same.
- **Plugins & config** — the 3-tier config system and all plugins behave
  identically; only the Claude-only sections above are dropped from the Pi
  default.

## Limitations

- Cost is not available from Pi, so cost-based sections show nothing there.
- Token detail beyond the total context usage Pi reports is not exposed, so the
  context bar uses Pi's reported percentage.
