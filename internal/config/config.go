package config

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/himattm/prism/internal/fsutil"
	"github.com/himattm/prism/internal/version"
)

// Config represents the Prism configuration
type Config struct {
	ConfigVersion     string         `json:"configVersion,omitempty"` // Tracks which migrations have been applied
	Icon              string         `json:"icon,omitempty"`
	Sections          any            `json:"sections,omitempty"` // Can be []string or [][]string
	Plugins           map[string]any `json:"plugins,omitempty"`
	AutocompactBuffer *float64       `json:"autocompactBuffer,omitempty"` // Buffer percentage (default 22.5, set to 0 if disabled)
}

// GetAutocompactBuffer returns the autocompact buffer percentage (default 22.5)
func (c Config) GetAutocompactBuffer() float64 {
	if c.AutocompactBuffer == nil {
		return 22.5 // Default Claude Code buffer
	}
	return *c.AutocompactBuffer
}

// DefaultSectionLines returns the default multi-line section layout.
// This is the single source of truth for default sections.
//
// Lines are organized by concern:
//   - Line 1: Agent harness — dir, model, context window, token usage, and git branch
//   - Line 2: Project tooling — supabase, vercel, android, and other dev stack plugins
//   - Line 3: Auxiliary — ambient info like now-playing music
func DefaultSectionLines() [][]string {
	return [][]string{
		{"dir", "model", "context", "usage", "peakhours", "git"},
		{"supabase", "vercel", "android"},
		{"spotify"},
	}
}

// DefaultSectionLinesForAgent returns the default multi-line layout tailored to
// the host agent. Pi does not expose Anthropic-specific data (plan usage limits,
// API cost, or Anthropic peak-hour windows), so those Claude-only sections are
// omitted from Pi's default layout. An explicit "sections" config always wins
// over these defaults regardless of agent.
func DefaultSectionLinesForAgent(agent string) [][]string {
	if agent == "pi" {
		return [][]string{
			{"dir", "model", "context", "git"},
			{"supabase", "vercel", "android"},
			{"spotify"},
		}
	}
	return DefaultSectionLines()
}

// DefaultSections returns the default sections as a flat list (for backwards compatibility)
func DefaultSections() []string {
	lines := DefaultSectionLines()
	var result []string
	for _, line := range lines {
		result = append(result, line...)
	}
	return result
}

// Load reads and merges configuration from all config files
func Load(projectDir string) Config {
	cfg := Config{}

	// Load global config first
	if globalCfg, err := loadFile(globalConfigPath()); err == nil {
		cfg = mergeCfg(cfg, globalCfg)
	}

	// Load project config
	if projectDir != "" {
		projectCfgPath := filepath.Join(projectDir, ".claude", "prism.json")
		if projectCfg, err := loadFile(projectCfgPath); err == nil {
			cfg = mergeCfg(cfg, projectCfg)
		}

		// Load local overrides
		localCfgPath := filepath.Join(projectDir, ".claude", "prism.local.json")
		if localCfg, err := loadFile(localCfgPath); err == nil {
			cfg = mergeCfg(cfg, localCfg)
		}
	}

	return cfg
}

func globalConfigPath() string {
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".claude", "prism-config.json")
}

// PluginsDir returns the path to the plugins directory
func PluginsDir() string {
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, ".claude", "prism-plugins")
}

// LoadPluginConfig loads a plugin's own config.json and merges with prism.json overrides
func (c Config) LoadPluginConfig(name string) map[string]any {
	result := make(map[string]any)

	// First load plugin's own config.json
	pluginConfigPath := filepath.Join(PluginsDir(), name, "config.json")
	if data, err := os.ReadFile(pluginConfigPath); err == nil {
		json.Unmarshal(data, &result)
	}

	// Then overlay with prism.json plugin config
	if c.Plugins != nil {
		if override, ok := c.Plugins[name].(map[string]any); ok {
			for k, v := range override {
				result[k] = v
			}
		}
	}

	return result
}

func loadFile(path string) (Config, error) {
	var cfg Config
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	err = json.Unmarshal(data, &cfg)
	return cfg, err
}

func mergeCfg(base, overlay Config) Config {
	if overlay.Icon != "" {
		base.Icon = overlay.Icon
	}
	if overlay.Sections != nil {
		base.Sections = overlay.Sections
	}
	if overlay.Plugins != nil {
		if base.Plugins == nil {
			base.Plugins = make(map[string]any)
		}
		for k, v := range overlay.Plugins {
			base.Plugins[k] = v
		}
	}
	if overlay.AutocompactBuffer != nil {
		base.AutocompactBuffer = overlay.AutocompactBuffer
	}
	return base
}

// GetSections returns the configured sections as a flat list
func (c Config) GetSections() []string {
	if c.Sections == nil {
		return DefaultSections()
	}

	// Handle both flat and nested section arrays
	switch v := c.Sections.(type) {
	case []any:
		if len(v) == 0 {
			return DefaultSections()
		}
		// Check if first element is a string (flat) or array (nested)
		if _, ok := v[0].(string); ok {
			// Flat array
			sections := make([]string, len(v))
			for i, s := range v {
				sections[i] = s.(string)
			}
			return sections
		}
		// Nested array - flatten first line for now
		if arr, ok := v[0].([]any); ok {
			sections := make([]string, len(arr))
			for i, s := range arr {
				sections[i] = s.(string)
			}
			return sections
		}
	}

	return DefaultSections()
}

// IsMultiline returns true if sections are configured as multi-line
func (c Config) IsMultiline() bool {
	if c.Sections == nil {
		return false
	}
	if arr, ok := c.Sections.([]any); ok && len(arr) > 0 {
		_, isNested := arr[0].([]any)
		return isNested
	}
	return false
}

// GetAllSectionLines returns sections as lines (for multi-line support)
func (c Config) GetAllSectionLines() [][]string {
	if c.Sections == nil {
		return DefaultSectionLines()
	}

	switch v := c.Sections.(type) {
	case []any:
		if len(v) == 0 {
			return DefaultSectionLines()
		}
		// Check if nested
		if _, ok := v[0].([]any); ok {
			lines := make([][]string, len(v))
			for i, line := range v {
				if arr, ok := line.([]any); ok {
					sections := make([]string, len(arr))
					for j, s := range arr {
						sections[j] = s.(string)
					}
					lines[i] = sections
				}
			}
			return lines
		}
		// Flat array
		sections := make([]string, len(v))
		for i, s := range v {
			sections[i] = s.(string)
		}
		return [][]string{sections}
	}

	return DefaultSectionLines()
}

// Init creates a new project config file
func Init(dir string) error {
	configDir := filepath.Join(dir, ".claude")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return err
	}

	configPath := filepath.Join(configDir, "prism.json")
	if _, err := os.Stat(configPath); err == nil {
		return os.ErrExist
	}

	cfg := Config{
		ConfigVersion: version.Version,
		Icon:          "💎",
		Sections:      DefaultSectionLines(),
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return fsutil.SecureWriteFile(configPath, data, 0644)
}

// InitGlobal creates a new global config file
func InitGlobal() error {
	homeDir, _ := os.UserHomeDir()
	configDir := filepath.Join(homeDir, ".claude")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return err
	}

	configPath := filepath.Join(configDir, "prism-config.json")
	if _, err := os.Stat(configPath); err == nil {
		return os.ErrExist
	}

	cfg := Config{
		ConfigVersion: version.Version,
		Sections:      DefaultSectionLines(),
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return fsutil.SecureWriteFile(configPath, data, 0644)
}
