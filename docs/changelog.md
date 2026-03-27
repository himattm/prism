# Changelog

## v0.10.1

- Add system metrics plugins: CPU usage, memory pressure, battery status, and stack detection
- Add Vercel deployment status plugin
- Add Supabase plugin for local dev stack and migration status
- Fix security and refactoring findings from code review
- Fix duplicate context window suffix in model display

## v0.10.0

- Support Opus 4.6 extended context window (1M tokens)

## v0.9.3

- Show parent project name in worktree status line

## v0.9.2

- Fix OAuth detection failure caused by Keychain truncation

## v0.9.1

- Show dimmed stale usage data instead of cost for Max/Pro users when live data is unavailable

## v0.9.0

- Add cache efficiency and burn rate metrics to usage display
- Persist usage data to disk for display during busy mode
- Fix context percentage calculation

## v0.8.1

- Add log rotation to prevent unbounded growth of `prism-hooks.log`

## v0.8.0

- Add git status detection when navigating into git repositories
- Fix context bar color to correctly reflect autocompact proximity

## v0.7.0

- Add Spotify now-playing plugin

## v0.6.0

- Add async hooks support for Claude Code v2.1.0+
- Add `agent_task_queue` plugin for build queue status display
- Add minutes option for remaining time display

## v0.5.0

- Detect worktrees when Claude navigates into them from the main repo
- Add support for all 11 Claude Code hooks
- Add Setup hook, AgentType detection, and hook logging
- Add installer improvements and auto-create global config on install

## v0.4.3

- Fix default sections to use `usage` instead of `cost`

## v0.4.2

- Fix auto-update by copying binary to a temp location before spawning

## v0.4.1

- Add usage plugins for Max/Pro plan limits
- Add worktree indicator to directory section
- Fix auto-update goroutine bug
- Change update check interval to 8 hours

## v0.4.0

- Add version-aware config migrations and remove deprecated plugins

## v0.3.1

- Clear update cache after a successful `prism update`

## v0.3.0

- Use context percentage fields from Claude Code 2.1.6

## v0.2.0

- Rewrite in Go with native plugins and parallel execution
- Add plugin architecture for extensible sections
- Add update notification system and plugin versioning
- Add context bar with colors, autocompact buffer, and actionable context percentage
- Add upstream tracking indicators to git plugin
- Add support for all Claude Code hooks built into the binary
- Add `prism update` and `prism check-update` commands
- Add `prism refract` command with expanded color palette
- Move config to `.claude/` directory with versioning
- Add comprehensive test suites for all plugins
- Fix git plugin to use hooks for cache invalidation

## v0.1.0

- Initial release with bash implementation
- 3-tier config system (global, project, local)
- Git plugin with caching, timeouts, and serialized access
- Multi-line sections, local overrides, and smarter device display
