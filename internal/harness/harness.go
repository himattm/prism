package harness

import (
	"path/filepath"

	"github.com/himattm/prism/internal/statusline"
)

// Harness defines how Prism integrates with an agent CLI.
// Each supported CLI (Claude Code, Codex, etc.) implements this interface.
type Harness interface {
	// ID returns the short identifier (e.g., "claude", "codex").
	ID() string

	// Name returns the human-readable name (e.g., "Claude Code", "Codex CLI").
	Name() string

	// HomeDir returns the harness config directory (e.g., ~/.claude, ~/.codex).
	HomeDir() string

	// ParseInput maps the harness's stdin JSON to Prism's internal Input type.
	ParseInput(raw []byte) (statusline.Input, error)

	// DefaultSections returns the default section layout for this harness.
	DefaultSections() [][]string

	// Hooks returns the hook event mappings for settings migration.
	Hooks() []HookDef

	// SetupInstructions returns text telling the user how to configure this harness.
	SetupInstructions() string
}

// HookDef describes a single hook event mapping from a harness to a Prism command.
type HookDef struct {
	Event   string // The harness's event name (e.g., "UserPromptSubmit", "Stop")
	Command string // The prism command to run (e.g., "$HOME/.claude/prism hook busy")
	Async   bool   // Whether the hook should run asynchronously
}

// PathProvider is optionally implemented by harnesses that expose derived paths.
// All harnesses embedding Base get this for free.
type PathProvider interface {
	BinaryPath() string
	GlobalConfigPath() string
	PluginDir() string
	LogFile() string
}

// LogFileFor returns the hook log file path for a harness.
// If the harness implements PathProvider, it uses that; otherwise derives from HomeDir().
func LogFileFor(h Harness) string {
	if pp, ok := h.(PathProvider); ok {
		return pp.LogFile()
	}
	return filepath.Join(h.HomeDir(), "prism-hooks.log")
}

// PluginDirFor returns the plugin directory for a harness.
// If the harness implements PathProvider, it uses that; otherwise derives from HomeDir().
func PluginDirFor(h Harness) string {
	if pp, ok := h.(PathProvider); ok {
		return pp.PluginDir()
	}
	return filepath.Join(h.HomeDir(), "prism-plugins")
}

// Base provides default path derivation logic. Embed in concrete harnesses
// to avoid repeating boilerplate — only override what's unique per harness.
type Base struct {
	home string
}

// NewBase creates a Base with the given home directory.
func NewBase(home string) Base {
	return Base{home: home}
}

// HomeDir returns the harness home directory.
func (b Base) HomeDir() string { return b.home }

// BinaryPath returns the expected path to the Prism binary for this harness.
func (b Base) BinaryPath() string { return filepath.Join(b.home, "prism") }

// GlobalConfigPath returns the path to the global Prism config for this harness.
func (b Base) GlobalConfigPath() string { return filepath.Join(b.home, "prism-config.json") }

// PluginDir returns the path to the plugin directory for this harness.
func (b Base) PluginDir() string { return filepath.Join(b.home, "prism-plugins") }

// LogFile returns the path to the hook log file for this harness.
func (b Base) LogFile() string { return filepath.Join(b.home, "prism-hooks.log") }
