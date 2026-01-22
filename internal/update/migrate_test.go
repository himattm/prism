package update

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestMigrateSettings(t *testing.T) {
	tests := []struct {
		name           string
		input          map[string]any
		wantHooksAdded int
		validate       func(t *testing.T, settings map[string]any)
	}{
		{
			name:           "empty settings - adds all hooks",
			input:          map[string]any{},
			wantHooksAdded: 6,
			validate: func(t *testing.T, settings map[string]any) {
				assertAllPrismHooks(t, settings)
			},
		},
		{
			name: "no hooks key - creates hooks and adds all",
			input: map[string]any{
				"theme":  "dark",
				"apiKey": "sk-xxx",
			},
			wantHooksAdded: 6,
			validate: func(t *testing.T, settings map[string]any) {
				assertAllPrismHooks(t, settings)
				// Verify other keys preserved
				if settings["theme"] != "dark" {
					t.Error("theme key not preserved")
				}
				if settings["apiKey"] != "sk-xxx" {
					t.Error("apiKey key not preserved")
				}
			},
		},
		{
			name: "empty hooks object - adds all",
			input: map[string]any{
				"hooks": map[string]any{},
			},
			wantHooksAdded: 6,
			validate: func(t *testing.T, settings map[string]any) {
				assertAllPrismHooks(t, settings)
			},
		},
		{
			name: "partial prism hooks - adds missing",
			input: map[string]any{
				"hooks": map[string]any{
					"Stop": []any{
						map[string]any{
							"hooks": []any{
								map[string]any{
									"type":    "command",
									"command": "$HOME/.claude/prism hook idle",
								},
							},
						},
					},
					"UserPromptSubmit": []any{
						map[string]any{
							"hooks": []any{
								map[string]any{
									"type":    "command",
									"command": "$HOME/.claude/prism hook busy",
								},
							},
						},
					},
				},
			},
			wantHooksAdded: 4, // SessionStart, SessionEnd, PreCompact, Setup
			validate: func(t *testing.T, settings map[string]any) {
				assertAllPrismHooks(t, settings)
				// Verify no duplicates
				hooks := settings["hooks"].(map[string]any)
				if len(hooks["Stop"].([]any)) != 1 {
					t.Error("Stop hook duplicated")
				}
			},
		},
		{
			name: "all prism hooks present - idempotent",
			input: map[string]any{
				"hooks": map[string]any{
					"UserPromptSubmit": []any{map[string]any{"hooks": []any{map[string]any{"type": "command", "command": "$HOME/.claude/prism hook busy"}}}},
					"Stop":             []any{map[string]any{"hooks": []any{map[string]any{"type": "command", "command": "$HOME/.claude/prism hook idle"}}}},
					"SessionStart":     []any{map[string]any{"hooks": []any{map[string]any{"type": "command", "command": "$HOME/.claude/prism hook session-start"}}}},
					"SessionEnd":       []any{map[string]any{"hooks": []any{map[string]any{"type": "command", "command": "$HOME/.claude/prism hook session-end"}}}},
					"PreCompact":       []any{map[string]any{"hooks": []any{map[string]any{"type": "command", "command": "$HOME/.claude/prism hook pre-compact"}}}},
					"Setup":            []any{map[string]any{"hooks": []any{map[string]any{"type": "command", "command": "$HOME/.claude/prism hook setup"}}}},
				},
			},
			wantHooksAdded: 0,
			validate: func(t *testing.T, settings map[string]any) {
				assertAllPrismHooks(t, settings)
			},
		},
		{
			name: "custom hooks only - preserved, prism added",
			input: map[string]any{
				"hooks": map[string]any{
					"Stop": []any{
						map[string]any{
							"hooks": []any{
								map[string]any{
									"type":    "command",
									"command": "notify-send 'Claude stopped'",
								},
							},
						},
					},
				},
			},
			wantHooksAdded: 6,
			validate: func(t *testing.T, settings map[string]any) {
				assertAllPrismHooks(t, settings)
				// Verify custom hook preserved
				if !hasHookCommand(settings, "Stop", "notify-send 'Claude stopped'") {
					t.Error("custom Stop hook not preserved")
				}
				// Should have 2 hook groups for Stop
				hooks := settings["hooks"].(map[string]any)
				if len(hooks["Stop"].([]any)) != 2 {
					t.Error("expected 2 hook groups for Stop")
				}
			},
		},
		{
			name: "mixed custom and prism - no duplicates",
			input: map[string]any{
				"hooks": map[string]any{
					"Stop": []any{
						map[string]any{
							"hooks": []any{
								map[string]any{"type": "command", "command": "custom-hook.sh"},
							},
						},
						map[string]any{
							"hooks": []any{
								map[string]any{"type": "command", "command": "$HOME/.claude/prism hook idle"},
							},
						},
					},
				},
			},
			wantHooksAdded: 5, // All except Stop (already has prism hook)
			validate: func(t *testing.T, settings map[string]any) {
				assertAllPrismHooks(t, settings)
				hooks := settings["hooks"].(map[string]any)
				// Stop should still have exactly 2 (custom + prism, no duplicate)
				if len(hooks["Stop"].([]any)) != 2 {
					t.Errorf("Stop should have 2 hook groups, got %d", len(hooks["Stop"].([]any)))
				}
			},
		},
		{
			name: "preserves other hook events",
			input: map[string]any{
				"hooks": map[string]any{
					"PreToolUse": []any{
						map[string]any{
							"hooks": []any{
								map[string]any{"type": "command", "command": "audit.sh"},
							},
						},
					},
				},
			},
			wantHooksAdded: 6,
			validate: func(t *testing.T, settings map[string]any) {
				assertAllPrismHooks(t, settings)
				// Verify PreToolUse preserved
				if !hasHookCommand(settings, "PreToolUse", "audit.sh") {
					t.Error("PreToolUse hook not preserved")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp directory with .claude subdirectory
			tmpHome := t.TempDir()
			claudeDir := filepath.Join(tmpHome, ".claude")
			if err := os.MkdirAll(claudeDir, 0755); err != nil {
				t.Fatal(err)
			}

			// Write input settings
			settingsPath := filepath.Join(claudeDir, "settings.json")
			data, err := json.MarshalIndent(tt.input, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(settingsPath, data, 0644); err != nil {
				t.Fatal(err)
			}

			// Override home directory for test
			origHome := os.Getenv("HOME")
			os.Setenv("HOME", tmpHome)
			defer os.Setenv("HOME", origHome)

			// Run migration
			added, err := MigrateSettings()
			if err != nil {
				t.Fatalf("MigrateSettings failed: %v", err)
			}

			if added != tt.wantHooksAdded {
				t.Errorf("hooks added = %d, want %d", added, tt.wantHooksAdded)
			}

			// Read back and validate
			resultData, err := os.ReadFile(settingsPath)
			if err != nil {
				t.Fatal(err)
			}

			var result map[string]any
			if err := json.Unmarshal(resultData, &result); err != nil {
				t.Fatal(err)
			}

			tt.validate(t, result)
		})
	}
}

func TestMigrateSettings_NoSettingsFile(t *testing.T) {
	tmpHome := t.TempDir()
	claudeDir := filepath.Join(tmpHome, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatal(err)
	}

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	// No settings.json exists
	added, err := MigrateSettings()
	if err != nil {
		t.Errorf("expected no error for missing file, got: %v", err)
	}
	if added != 0 {
		t.Errorf("expected 0 hooks added for missing file, got: %d", added)
	}
}

// TestMigrateSettings_SharedFixtures tests against the same fixtures used by install.sh tests
func TestMigrateSettings_SharedFixtures(t *testing.T) {
	fixtures := []struct {
		file           string
		wantHooksAdded int
		extraChecks    func(t *testing.T, settings map[string]any)
	}{
		{"01-empty.json", 6, nil},
		{"02-no-hooks.json", 6, func(t *testing.T, s map[string]any) {
			if s["theme"] != "dark" {
				t.Error("theme not preserved")
			}
		}},
		{"03-empty-hooks.json", 6, nil},
		{"04-partial-prism-hooks.json", 4, nil}, // Stop + UserPromptSubmit already present
		{"05-all-prism-hooks.json", 0, nil},     // All present, idempotent
		{"06-custom-hooks-only.json", 6, func(t *testing.T, s map[string]any) {
			if !hasHookCommand(s, "Stop", "notify-send 'Claude stopped'") {
				t.Error("custom Stop hook not preserved")
			}
		}},
		{"07-custom-and-prism-hooks.json", 4, func(t *testing.T, s map[string]any) {
			// Stop has custom + prism, UserPromptSubmit has prism
			if !hasHookCommand(s, "Stop", "notify-send 'Claude stopped'") {
				t.Error("custom Stop hook not preserved")
			}
		}},
		{"08-other-statusline.json", 6, func(t *testing.T, s map[string]any) {
			if !hasHookCommand(s, "Stop", "echo 'stopped'") {
				t.Error("custom Stop hook not preserved")
			}
		}},
		{"09-complex-existing.json", 6, func(t *testing.T, s map[string]any) {
			if s["theme"] != "dark" {
				t.Error("theme not preserved")
			}
			if s["apiKey"] != "sk-xxx" {
				t.Error("apiKey not preserved")
			}
			if !hasHookCommand(s, "PreToolUse", "audit-tool-use.sh") {
				t.Error("PreToolUse hook not preserved")
			}
			if !hasHookCommand(s, "Stop", "python-linter.sh") {
				t.Error("custom Stop hook not preserved")
			}
		}},
		{"10-multiple-hooks-per-event.json", 6, func(t *testing.T, s map[string]any) {
			if !hasHookCommand(s, "Stop", "first-hook.sh") {
				t.Error("first-hook.sh not preserved")
			}
			if !hasHookCommand(s, "Stop", "second-hook.sh") {
				t.Error("second-hook.sh not preserved")
			}
			if !hasHookCommand(s, "Stop", "third-hook.sh") {
				t.Error("third-hook.sh not preserved")
			}
		}},
	}

	for _, tt := range fixtures {
		t.Run(tt.file, func(t *testing.T) {
			input := loadFixture(t, tt.file)

			// Create temp directory
			tmpHome := t.TempDir()
			claudeDir := filepath.Join(tmpHome, ".claude")
			os.MkdirAll(claudeDir, 0755)

			// Write fixture to temp settings
			settingsPath := filepath.Join(claudeDir, "settings.json")
			data, _ := json.MarshalIndent(input, "", "  ")
			os.WriteFile(settingsPath, data, 0644)

			// Override HOME
			origHome := os.Getenv("HOME")
			os.Setenv("HOME", tmpHome)
			defer os.Setenv("HOME", origHome)

			// Run migration
			added, err := MigrateSettings()
			if err != nil {
				t.Fatalf("MigrateSettings failed: %v", err)
			}

			if added != tt.wantHooksAdded {
				t.Errorf("hooks added = %d, want %d", added, tt.wantHooksAdded)
			}

			// Read result
			resultData, _ := os.ReadFile(settingsPath)
			var result map[string]any
			json.Unmarshal(resultData, &result)

			// All prism hooks should be present
			assertAllPrismHooks(t, result)

			// Run extra checks if any
			if tt.extraChecks != nil {
				tt.extraChecks(t, result)
			}
		})
	}
}

func TestMigrateSettings_Idempotent(t *testing.T) {
	tmpHome := t.TempDir()
	claudeDir := filepath.Join(tmpHome, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatal(err)
	}

	settingsPath := filepath.Join(claudeDir, "settings.json")
	input := map[string]any{
		"hooks": map[string]any{
			"Stop": []any{
				map[string]any{
					"hooks": []any{
						map[string]any{"type": "command", "command": "custom.sh"},
					},
				},
			},
		},
	}
	data, _ := json.MarshalIndent(input, "", "  ")
	os.WriteFile(settingsPath, data, 0644)

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)
	defer os.Setenv("HOME", origHome)

	// First run
	added1, _ := MigrateSettings()
	if added1 != 6 {
		t.Errorf("first run: expected 6 hooks added, got %d", added1)
	}

	// Second run - should add nothing
	added2, _ := MigrateSettings()
	if added2 != 0 {
		t.Errorf("second run: expected 0 hooks added, got %d", added2)
	}

	// Third run - still nothing
	added3, _ := MigrateSettings()
	if added3 != 0 {
		t.Errorf("third run: expected 0 hooks added, got %d", added3)
	}
}

// Helper functions

func getFixturesDir() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "..", "..", "test", "installer", "fixtures")
}

func loadFixture(t *testing.T, name string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(getFixturesDir(), name))
	if err != nil {
		t.Fatalf("failed to load fixture %s: %v", name, err)
	}
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("failed to parse fixture %s: %v", name, err)
	}
	return result
}

func assertAllPrismHooks(t *testing.T, settings map[string]any) {
	t.Helper()
	for _, h := range PrismHooks {
		if !hasHookCommand(settings, h.Event, h.Command) {
			t.Errorf("missing prism hook: %s -> %s", h.Event, h.Command)
		}
	}
}

func hasHookCommand(settings map[string]any, event, command string) bool {
	hooks, ok := settings["hooks"].(map[string]any)
	if !ok {
		return false
	}

	eventHooks, ok := hooks[event].([]any)
	if !ok {
		return false
	}

	for _, hookGroup := range eventHooks {
		group, ok := hookGroup.(map[string]any)
		if !ok {
			continue
		}

		hookList, ok := group["hooks"].([]any)
		if !ok {
			continue
		}

		for _, hook := range hookList {
			h, ok := hook.(map[string]any)
			if !ok {
				continue
			}
			if h["command"] == command {
				return true
			}
		}
	}

	return false
}
