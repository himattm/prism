package harness

import (
	"os"
	"testing"
)

func TestRegistryContainsClaude(t *testing.T) {
	h := Get("claude")
	if h == nil {
		t.Fatal("expected claude harness to be registered")
	}
	if h.ID() != "claude" {
		t.Errorf("expected ID 'claude', got %q", h.ID())
	}
	if h.Name() != "Claude Code" {
		t.Errorf("expected Name 'Claude Code', got %q", h.Name())
	}
}

func TestRegistryContainsCodex(t *testing.T) {
	h := Get("codex")
	if h == nil {
		t.Fatal("expected codex harness to be registered")
	}
	if h.ID() != "codex" {
		t.Errorf("expected ID 'codex', got %q", h.ID())
	}
	if h.Name() != "Codex CLI" {
		t.Errorf("expected Name 'Codex CLI', got %q", h.Name())
	}
}

func TestGetUnknownHarness(t *testing.T) {
	h := Get("nonexistent")
	if h != nil {
		t.Errorf("expected nil for unknown harness, got %v", h)
	}
}

func TestAllReturnsRegisteredHarnesses(t *testing.T) {
	ids := All()
	if len(ids) < 2 {
		t.Fatalf("expected at least 2 registered harnesses, got %d", len(ids))
	}

	found := map[string]bool{}
	for _, id := range ids {
		found[id] = true
	}
	if !found["claude"] {
		t.Error("expected 'claude' in All()")
	}
	if !found["codex"] {
		t.Error("expected 'codex' in All()")
	}
}

func TestAllReturnsSorted(t *testing.T) {
	ids := All()
	for i := 1; i < len(ids); i++ {
		if ids[i] < ids[i-1] {
			t.Errorf("All() not sorted: %v", ids)
			break
		}
	}
}

func TestDetectDefaultsToClaude(t *testing.T) {
	os.Unsetenv("PRISM_HARNESS")
	h := Detect()
	if h == nil {
		t.Fatal("Detect() returned nil")
	}
	if h.ID() != "claude" {
		t.Errorf("expected default detect to be 'claude', got %q", h.ID())
	}
}

func TestDetectWithEnvVar(t *testing.T) {
	os.Setenv("PRISM_HARNESS", "codex")
	defer os.Unsetenv("PRISM_HARNESS")

	h := Detect()
	if h == nil {
		t.Fatal("Detect() returned nil")
	}
	if h.ID() != "codex" {
		t.Errorf("expected detect to be 'codex', got %q", h.ID())
	}
}

func TestDetectUnknownFallsToClaude(t *testing.T) {
	os.Setenv("PRISM_HARNESS", "unknown_harness")
	defer os.Unsetenv("PRISM_HARNESS")

	h := Detect()
	if h == nil {
		t.Fatal("Detect() returned nil")
	}
	if h.ID() != "claude" {
		t.Errorf("expected fallback to 'claude', got %q", h.ID())
	}
}

func TestBasePathDerivation(t *testing.T) {
	b := NewBase("/home/test/.myharness")

	if b.HomeDir() != "/home/test/.myharness" {
		t.Errorf("unexpected HomeDir: %q", b.HomeDir())
	}
	if b.BinaryPath() != "/home/test/.myharness/prism" {
		t.Errorf("unexpected BinaryPath: %q", b.BinaryPath())
	}
	if b.GlobalConfigPath() != "/home/test/.myharness/prism-config.json" {
		t.Errorf("unexpected GlobalConfigPath: %q", b.GlobalConfigPath())
	}
	if b.PluginDir() != "/home/test/.myharness/prism-plugins" {
		t.Errorf("unexpected PluginDir: %q", b.PluginDir())
	}
	if b.LogFile() != "/home/test/.myharness/prism-hooks.log" {
		t.Errorf("unexpected LogFile: %q", b.LogFile())
	}
}

func TestHarnessHomeDirs(t *testing.T) {
	homeDir, _ := os.UserHomeDir()

	claude := Get("claude")
	if claude.HomeDir() != homeDir+"/.claude" {
		t.Errorf("Claude HomeDir: expected %s/.claude, got %s", homeDir, claude.HomeDir())
	}

	codex := Get("codex")
	if codex.HomeDir() != homeDir+"/.codex" {
		t.Errorf("Codex HomeDir: expected %s/.codex, got %s", homeDir, codex.HomeDir())
	}
}

func TestHarnessesHaveHooks(t *testing.T) {
	for _, id := range All() {
		h := Get(id)
		hooks := h.Hooks()
		if len(hooks) == 0 {
			t.Errorf("harness %q has no hooks defined", id)
		}
	}
}

func TestHarnessesHaveDefaultSections(t *testing.T) {
	for _, id := range All() {
		h := Get(id)
		sections := h.DefaultSections()
		if len(sections) == 0 {
			t.Errorf("harness %q has no default sections", id)
		}
		for i, line := range sections {
			if len(line) == 0 {
				t.Errorf("harness %q section line %d is empty", id, i)
			}
		}
	}
}

func TestHarnessesHaveSetupInstructions(t *testing.T) {
	for _, id := range All() {
		h := Get(id)
		if h.SetupInstructions() == "" {
			t.Errorf("harness %q has empty SetupInstructions", id)
		}
	}
}

func TestLogFileFor(t *testing.T) {
	h := Get("claude")
	logFile := LogFileFor(h)
	homeDir, _ := os.UserHomeDir()
	expected := homeDir + "/.claude/prism-hooks.log"
	if logFile != expected {
		t.Errorf("expected %q, got %q", expected, logFile)
	}
}

func TestPluginDirFor(t *testing.T) {
	h := Get("codex")
	pluginDir := PluginDirFor(h)
	homeDir, _ := os.UserHomeDir()
	expected := homeDir + "/.codex/prism-plugins"
	if pluginDir != expected {
		t.Errorf("expected %q, got %q", expected, pluginDir)
	}
}
