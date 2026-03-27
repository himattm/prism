# Prism

**A fast, customizable, and colorful status line for Claude Code**

Prism is a native Go binary that replaces Claude Code's default status bar with rich, real-time information about your development environment. It provides actionable context about Claude's session state (token usage, model, context window fill %), project and development tooling information (git status, Vercel deployments, Supabase status, Android devices), and ambient information (currently playing Spotify track, system metrics). It automatically detects billing type (API vs Max/Pro plans) and manages auto-updates seamlessly in the background.

---

## Features

**Smart Billing Detection**
Auto-detects API vs Max/Pro billing and shows the metrics most relevant to your plan.

**Context Window Visualization**
Color-coded progress bar that shifts from white to yellow to red as your context window fills, giving you urgency at a glance.

**Plugin Ecosystem**
14 built-in plugins covering git, cloud services, system metrics, and more — plus support for community script and binary plugins.

**Multi-line Layouts**
Organize plugins across multiple status lines to keep your most important information front and center.

**Hook System**
React to 11 Claude Code lifecycle events (session start, stop, tool use, errors, and more) with custom plugin output.

**Auto-updates**
Background update checking and installation keeps Prism current without interrupting your workflow.

---

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/himattm/prism/main/install.sh | bash
```

[Get Started](getting-started/installation.md){ .md-button .md-button--primary }
