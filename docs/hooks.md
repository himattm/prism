# Hooks

Hooks let Prism react to Claude Code lifecycle events. When Claude starts a session, submits a prompt, stops responding, or compacts context, Prism receives a notification and can update state, invalidate caches, or display information.

## How Hooks Work

Claude Code sends events to Prism via `prism hook <type>` commands configured in `settings.json`. Prism dispatches the event to all registered plugins that implement the `Hookable` interface.

Each hook call is a lightweight process invocation. Claude Code passes context as JSON on stdin, and Prism reads it, dispatches to plugins, and exits.

## Available Events

| Hook Type | Claude Code Event | Mode | Purpose |
|-----------|------------------|------|---------|
| `busy` | UserPromptSubmit | sync | User submitted a prompt — mark as busy |
| `idle` | Stop | async | Claude stopped responding — mark as idle, invalidate caches |
| `session-start` | SessionStart | async | New session started — initialize state |
| `session-end` | SessionEnd | async | Session ending — cleanup |
| `pre-compact` | PreCompact | sync | Context about to be compacted — may warn user |
| `setup` | Setup | sync | Repo init or maintenance |
| `pre-tool-use` | PreToolUse | sync | Before tool execution — can block |
| `post-tool-use` | PostToolUse | async | After tool execution — logging |
| `permission-request` | PermissionRequest | sync | Permission dialog shown |
| `notification` | Notification | async | Notification sent |
| `subagent-stop` | SubagentStop | async | Subagent task completed |

## Sync vs Async

Claude Code 2.1.0+ supports two hook execution modes:

- **Sync hooks** block Claude Code until the hook process exits. Use for hooks that produce output (written to stdout and displayed by Claude Code) or that need to prevent an action from proceeding.
- **Async hooks** run in the background without blocking Claude Code. Use for cleanup tasks, cache invalidation, and logging where the result does not need to be seen immediately.

The table above shows the recommended mode for each hook type.

## Hook Input

Each hook invocation receives a JSON payload on stdin:

```json
{
  "session_id": "abc123",
  "agent_type": "main"
}
```

The `agent_type` field is available in Claude Code 2.1.14+. It distinguishes between the main agent and subagents spawned during a session.

## Configuration in settings.json

The installer configures all hooks automatically. You only need to set them manually if performing a manual installation.

The complete hook configuration block for `~/.claude/settings.json`:

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

## How Plugins Use Hooks

Plugins implement the `Hookable` interface to react to events:

```go
type Hookable interface {
    OnHook(ctx context.Context, hookType HookType, hookCtx HookContext) (string, error)
}
```

When Prism receives a hook event, it calls `OnHook` on every plugin that implements this interface. Plugins that do not implement `Hookable` are skipped silently.

Common patterns used by built-in plugins:

| Pattern | Hook | Plugins |
|---------|------|---------|
| Cache invalidation | `idle` | `git`, `spotify`, `android_devices`, `supabase`, `vercel` |
| State initialization | `session-start` | `stack` |
| Busy/idle marker file management | `busy`, `idle` | Internal state tracking |

## Autocompact Buffer

The `autocompactBuffer` config option (default: `22.5`) works with the `pre-compact` hook. When Claude Code is about to compact the context window, Prism checks whether the remaining context falls within the configured buffer percentage. If it does, Prism can surface a warning before compaction occurs.

Set `autocompactBuffer` to `0` to disable this behavior entirely.

```json
{
  "autocompactBuffer": 22.5
}
```

See [Config System](configuration/config-system.md) for details on where to set this option.
