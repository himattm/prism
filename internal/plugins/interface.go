package plugins

import (
	"context"

	"github.com/himattm/prism/internal/cache"
	"github.com/himattm/prism/internal/plugin"
)

// NativePlugin defines the interface for built-in Go plugins
type NativePlugin interface {
	Name() string
	Execute(ctx context.Context, input plugin.Input) (string, error)
	SetCache(c *cache.Cache)
}

// Registry holds all available native plugins
type Registry struct {
	plugins map[string]NativePlugin
	cache   *cache.Cache
}

// NewRegistry creates a new plugin registry with all native plugins
func NewRegistry() *Registry {
	c := cache.New()
	r := &Registry{
		plugins: make(map[string]NativePlugin),
		cache:   c,
	}

	// Register native plugins with shared cache
	r.registerWithCache(&AndroidPlugin{})
	r.registerWithCache(&GitPlugin{})
	r.registerWithCache(&SpotifyPlugin{})
	r.registerWithCache(&TaskQueuePlugin{})
	r.registerWithCache(&UpdatePlugin{})
	r.registerWithCache(&UsageBarsPlugin{})
	r.registerWithCache(&UsageTextPlugin{})
	r.registerWithCache(&UsagePlugin{})
	r.registerWithCache(&SupabasePlugin{})
	r.registerWithCache(&VercelPlugin{})

	return r
}

func (r *Registry) registerWithCache(p NativePlugin) {
	p.SetCache(r.cache)
	r.plugins[p.Name()] = p
}

// Register adds a plugin to the registry
func (r *Registry) Register(p NativePlugin) {
	r.plugins[p.Name()] = p
}

// Get returns a native plugin by name, or nil if not found
func (r *Registry) Get(name string) NativePlugin {
	return r.plugins[name]
}

// Has returns true if a native plugin exists for the given name
func (r *Registry) Has(name string) bool {
	_, ok := r.plugins[name]
	return ok
}

// List returns all native plugin names
func (r *Registry) List() []string {
	names := make([]string, 0, len(r.plugins))
	for name := range r.plugins {
		names = append(names, name)
	}
	return names
}

// HookType represents the type of hook event
type HookType string

const (
	// Session lifecycle hooks
	HookSessionStart HookType = "session_start" // SessionStart - Session started/resumed
	HookSessionEnd   HookType = "session_end"   // SessionEnd - Session ending

	// User interaction hooks
	HookBusy              HookType = "busy"               // UserPromptSubmit - User submitted prompt
	HookIdle              HookType = "idle"               // Stop - Claude finished responding
	HookNotification      HookType = "notification"       // Notification - Claude Code sends a notification
	HookPermissionRequest HookType = "permission_request" // PermissionRequest - Permission dialog shown

	// Tool hooks
	HookPreToolUse  HookType = "pre_tool_use"  // PreToolUse - Before tool calls (can block)
	HookPostToolUse HookType = "post_tool_use" // PostToolUse - After tool calls complete

	// Agent hooks
	HookSubagentStop HookType = "subagent_stop" // SubagentStop - Subagent task completed

	// Context management hooks
	HookPreCompact HookType = "pre_compact" // PreCompact - Before context compaction
	HookSetup      HookType = "setup"       // Setup - Repository init/maintenance (--init, --init-only, --maintenance)
)

// HookContext provides context for hook handlers
type HookContext struct {
	SessionID string
	AgentType string         // Agent type if --agent was specified (e.g., "coder", "researcher")
	Config    map[string]any // Plugin configuration
}

// Hookable is an optional interface for plugins that want to respond to state changes
type Hookable interface {
	// OnHook is called when a hook event occurs
	// Return value is optional output to display (e.g., notifications)
	OnHook(ctx context.Context, hookType HookType, hookCtx HookContext) (string, error)
}

// GetHookablePlugins returns all plugins implementing Hookable
func (r *Registry) GetHookablePlugins() []Hookable {
	var hookable []Hookable
	for _, p := range r.plugins {
		if h, ok := p.(Hookable); ok {
			hookable = append(hookable, h)
		}
	}
	return hookable
}

// RunHooks executes hooks on all hookable plugins sequentially
func (r *Registry) RunHooks(ctx context.Context, hookType HookType, hookCtx HookContext) []string {
	var outputs []string
	for _, h := range r.GetHookablePlugins() {
		if output, err := h.OnHook(ctx, hookType, hookCtx); err == nil && output != "" {
			outputs = append(outputs, output)
		}
	}
	return outputs
}
