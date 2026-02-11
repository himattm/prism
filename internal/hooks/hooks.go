package hooks

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/himattm/prism/internal/config"
	"github.com/himattm/prism/internal/plugins"
)

// Input represents the JSON input from Claude Code hooks
type Input struct {
	SessionID string `json:"session_id"`
	AgentType string `json:"agent_type"` // Populated if --agent was specified (Claude Code 2.1.14+)
}

// Manager handles hook execution
type Manager struct {
	registry *plugins.Registry
	logFile  string
}

// NewManager creates a new hook manager
func NewManager() *Manager {
	homeDir, _ := os.UserHomeDir()
	return &Manager{
		registry: plugins.NewRegistry(),
		logFile:  filepath.Join(homeDir, ".claude", "prism-hooks.log"),
	}
}

// logHook writes hook invocation details to the log file
func (m *Manager) logHook(hookType string, input Input, rawInput []byte) {
	const maxLogSize = 10 * 1024 * 1024 // 10 MB

	// Rotate log if it exceeds max size
	if info, err := os.Stat(m.logFile); err == nil && info.Size() > maxLogSize {
		os.Rename(m.logFile, m.logFile+".old")
	}

	f, err := os.OpenFile(m.logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return // Silently fail - don't break hooks for logging
	}
	defer f.Close()

	timestamp := time.Now().Format("2006-01-02 15:04:05.000")

	// Format the raw JSON input nicely if we have it
	var rawJSON string
	if len(rawInput) > 0 {
		var prettyJSON map[string]any
		if json.Unmarshal(rawInput, &prettyJSON) == nil {
			if pretty, err := json.MarshalIndent(prettyJSON, "  ", "  "); err == nil {
				rawJSON = string(pretty)
			}
		}
		if rawJSON == "" {
			rawJSON = string(rawInput)
		}
	}

	fmt.Fprintf(f, "[%s] HOOK: %s\n", timestamp, hookType)
	fmt.Fprintf(f, "  session_id: %s\n", input.SessionID)
	if input.AgentType != "" {
		fmt.Fprintf(f, "  agent_type: %s\n", input.AgentType)
	}
	if rawJSON != "" {
		fmt.Fprintf(f, "  raw_input:\n  %s\n", rawJSON)
	}
	fmt.Fprintln(f, "")
}

// HandleIdle processes the idle hook (called when Claude stops responding)
func (m *Manager) HandleIdle(input Input, rawInput []byte) error {
	m.logHook("idle", input, rawInput)

	// 1. Create idle marker file
	if input.SessionID != "" {
		idleFile := filepath.Join(os.TempDir(), fmt.Sprintf("prism-idle-%s", input.SessionID))
		if err := os.WriteFile(idleFile, []byte{}, 0644); err != nil {
			return err
		}
	}

	// 2. Load config for plugins
	cfg := config.Load("")
	pluginConfig := make(map[string]any)
	if cfg.Plugins != nil {
		pluginConfig = cfg.Plugins
	}

	// 3. Run hooks on all hookable plugins
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	hookCtx := plugins.HookContext{
		SessionID: input.SessionID,
		AgentType: input.AgentType,
		Config:    pluginConfig,
	}

	outputs := m.registry.RunHooks(ctx, plugins.HookIdle, hookCtx)

	// 4. Print any outputs (for Claude Code to display)
	if len(outputs) > 0 {
		fmt.Print(strings.Join(outputs, "\n"))
	}

	return nil
}

// HandleBusy processes the busy hook (called when user submits prompt)
func (m *Manager) HandleBusy(input Input, rawInput []byte) error {
	m.logHook("busy", input, rawInput)

	// 1. Remove idle marker file
	if input.SessionID != "" {
		idleFile := filepath.Join(os.TempDir(), fmt.Sprintf("prism-idle-%s", input.SessionID))
		os.Remove(idleFile) // Ignore error if doesn't exist
	}

	// 2. Load config for plugins
	cfg := config.Load("")
	pluginConfig := make(map[string]any)
	if cfg.Plugins != nil {
		pluginConfig = cfg.Plugins
	}

	// 3. Run hooks on all hookable plugins
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	hookCtx := plugins.HookContext{
		SessionID: input.SessionID,
		AgentType: input.AgentType,
		Config:    pluginConfig,
	}

	outputs := m.registry.RunHooks(ctx, plugins.HookBusy, hookCtx)

	// 4. Print any outputs (for notifications)
	if len(outputs) > 0 {
		fmt.Print(strings.Join(outputs, "\n"))
	}

	return nil
}

// HandleSessionStart processes the session start hook
func (m *Manager) HandleSessionStart(input Input, rawInput []byte) error {
	m.logHook("session-start", input, rawInput)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	hookCtx := plugins.HookContext{
		SessionID: input.SessionID,
		AgentType: input.AgentType,
	}

	outputs := m.registry.RunHooks(ctx, plugins.HookSessionStart, hookCtx)

	if len(outputs) > 0 {
		fmt.Print(strings.Join(outputs, "\n"))
	}

	return nil
}

// HandleSessionEnd processes the session end hook
func (m *Manager) HandleSessionEnd(input Input, rawInput []byte) error {
	m.logHook("session-end", input, rawInput)

	// Clean up idle marker file
	if input.SessionID != "" {
		idleFile := filepath.Join(os.TempDir(), fmt.Sprintf("prism-idle-%s", input.SessionID))
		os.Remove(idleFile)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	hookCtx := plugins.HookContext{
		SessionID: input.SessionID,
		AgentType: input.AgentType,
	}

	outputs := m.registry.RunHooks(ctx, plugins.HookSessionEnd, hookCtx)

	if len(outputs) > 0 {
		fmt.Print(strings.Join(outputs, "\n"))
	}

	return nil
}

// HandlePreCompact processes the pre-compact hook
func (m *Manager) HandlePreCompact(input Input, rawInput []byte) error {
	m.logHook("pre-compact", input, rawInput)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	hookCtx := plugins.HookContext{
		SessionID: input.SessionID,
		AgentType: input.AgentType,
	}

	outputs := m.registry.RunHooks(ctx, plugins.HookPreCompact, hookCtx)

	if len(outputs) > 0 {
		fmt.Print(strings.Join(outputs, "\n"))
	}

	return nil
}

// HandleSetup processes the setup hook (triggered by --init, --init-only, --maintenance)
func (m *Manager) HandleSetup(input Input, rawInput []byte) error {
	return m.runSimpleHook(plugins.HookSetup, "setup", input, rawInput)
}

// HandlePreToolUse processes the pre-tool-use hook (before tool calls)
func (m *Manager) HandlePreToolUse(input Input, rawInput []byte) error {
	return m.runSimpleHook(plugins.HookPreToolUse, "pre-tool-use", input, rawInput)
}

// HandlePostToolUse processes the post-tool-use hook (after tool calls)
func (m *Manager) HandlePostToolUse(input Input, rawInput []byte) error {
	return m.runSimpleHook(plugins.HookPostToolUse, "post-tool-use", input, rawInput)
}

// HandlePermissionRequest processes the permission-request hook
func (m *Manager) HandlePermissionRequest(input Input, rawInput []byte) error {
	return m.runSimpleHook(plugins.HookPermissionRequest, "permission-request", input, rawInput)
}

// HandleNotification processes the notification hook
func (m *Manager) HandleNotification(input Input, rawInput []byte) error {
	return m.runSimpleHook(plugins.HookNotification, "notification", input, rawInput)
}

// HandleSubagentStop processes the subagent-stop hook
func (m *Manager) HandleSubagentStop(input Input, rawInput []byte) error {
	return m.runSimpleHook(plugins.HookSubagentStop, "subagent-stop", input, rawInput)
}

// runSimpleHook is a helper for hooks that just dispatch to plugins without special logic
func (m *Manager) runSimpleHook(hookType plugins.HookType, hookName string, input Input, rawInput []byte) error {
	m.logHook(hookName, input, rawInput)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	hookCtx := plugins.HookContext{
		SessionID: input.SessionID,
		AgentType: input.AgentType,
	}

	outputs := m.registry.RunHooks(ctx, hookType, hookCtx)

	if len(outputs) > 0 {
		fmt.Print(strings.Join(outputs, "\n"))
	}

	return nil
}
