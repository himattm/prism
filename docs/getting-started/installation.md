# Installation

## Requirements

- macOS (arm64 or amd64) or Linux (amd64 or arm64)
- [Claude Code](https://claude.ai/code) CLI installed

## Quick Install

The fastest way to install Prism is with the install script:

```bash
curl -fsSL https://raw.githubusercontent.com/himattm/prism/main/install.sh | bash
```

The installer automatically:

- Downloads the correct pre-built binary for your platform to `~/.claude/prism`
- Creates a default global config at `~/.claude/prism-config.json`
- Configures Claude Code hooks in `~/.claude/settings.json`
- Runs config migrations if you are upgrading from an older version

After running the installer, start a new Claude Code session to see the status line.

## Manual Installation

If you prefer to install manually, follow these steps.

### 1. Download the Binary

Download the binary for your platform from the [GitHub releases page](https://github.com/himattm/prism/releases). Place it at `~/.claude/prism` and make it executable:

```bash
chmod +x ~/.claude/prism
```

### 2. Configure the Status Line

Add the following block to `~/.claude/settings.json` to enable the status line:

```json
{
  "statusLine": {
    "type": "command",
    "command": "$HOME/.claude/prism"
  }
}
```

### 3. Add Hook Entries

Prism uses Claude Code's hook system to react to lifecycle events. Add the following to `~/.claude/settings.json`:

```json
{
  "hooks": {
    "UserPromptSubmit": [{"hooks": [
      {"type": "command", "command": "$HOME/.claude/prism hook busy"}
    ]}],
    "Stop": [{"hooks": [
      {"type": "command", "command": "$HOME/.claude/prism hook idle", "async": true}
    ]}],
    "SessionStart": [{"hooks": [
      {"type": "command", "command": "$HOME/.claude/prism hook session-start", "async": true}
    ]}],
    "SessionEnd": [{"hooks": [
      {"type": "command", "command": "$HOME/.claude/prism hook session-end", "async": true}
    ]}],
    "PreCompact": [{"hooks": [
      {"type": "command", "command": "$HOME/.claude/prism hook pre-compact"}
    ]}],
    "Setup": [{"hooks": [
      {"type": "command", "command": "$HOME/.claude/prism hook setup"}
    ]}],
    "PreToolUse": [{"hooks": [
      {"type": "command", "command": "$HOME/.claude/prism hook pre-tool-use"}
    ]}],
    "PostToolUse": [{"hooks": [
      {"type": "command", "command": "$HOME/.claude/prism hook post-tool-use", "async": true}
    ]}],
    "PermissionRequest": [{"hooks": [
      {"type": "command", "command": "$HOME/.claude/prism hook permission-request"}
    ]}],
    "Notification": [{"hooks": [
      {"type": "command", "command": "$HOME/.claude/prism hook notification", "async": true}
    ]}],
    "SubagentStop": [{"hooks": [
      {"type": "command", "command": "$HOME/.claude/prism hook subagent-stop", "async": true}
    ]}]
  }
}
```

!!! tip
    The `statusLine` and `hooks` blocks must be merged into your existing `settings.json` rather than replacing it. If you already have other settings, add these keys alongside them.

## Building from Source

Requires Go 1.21 or later.

```bash
git clone https://github.com/himattm/prism.git
cd prism
go build -o prism ./cmd/prism/
cp prism ~/.claude/prism
```

After building, follow the manual installation steps above to configure the status line and hooks.

## Auto-Updates

Prism checks for updates every 8 hours by default. Updates are downloaded and installed in the background while Claude is idle, so they do not interrupt your workflow.

To disable automatic updates, set the following in your config:

```json
{
  "update": {
    "enabled": false
  }
}
```

To trigger an update manually:

```bash
prism update
```

## Verifying Installation

```bash
prism version
```

This prints the installed version. If the command is not found, ensure `~/.claude/prism` exists and is executable, and that your `PATH` includes `~/.claude` or you are running the binary with its full path.
