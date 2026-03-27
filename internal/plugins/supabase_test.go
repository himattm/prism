package plugins

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/himattm/prism/internal/cache"
	"github.com/himattm/prism/internal/plugin"
)

func TestSupabasePlugin_Name(t *testing.T) {
	p := &SupabasePlugin{}
	if p.Name() != "supabase" {
		t.Errorf("expected name 'supabase', got '%s'", p.Name())
	}
}

func TestSupabasePlugin_SetCache(t *testing.T) {
	p := &SupabasePlugin{}
	c := cache.New()
	p.SetCache(c)

	if p.cache != c {
		t.Error("cache was not set correctly")
	}
}

func TestParseSupabaseConfig(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]any
		expected supabaseConfig
	}{
		{
			name:  "empty config uses defaults",
			input: map[string]any{},
			expected: supabaseConfig{
				showMigrations:  true,
				showWhenStopped: false,
			},
		},
		{
			name: "show_migrations false",
			input: map[string]any{
				"supabase": map[string]any{
					"show_migrations": false,
				},
			},
			expected: supabaseConfig{
				showMigrations:  false,
				showWhenStopped: false,
			},
		},
		{
			name: "show_when_stopped true",
			input: map[string]any{
				"supabase": map[string]any{
					"show_when_stopped": true,
				},
			},
			expected: supabaseConfig{
				showMigrations:  true,
				showWhenStopped: true,
			},
		},
		{
			name: "all options",
			input: map[string]any{
				"supabase": map[string]any{
					"show_migrations":   false,
					"show_when_stopped": true,
				},
			},
			expected: supabaseConfig{
				showMigrations:  false,
				showWhenStopped: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseSupabaseConfig(tt.input)
			if result.showMigrations != tt.expected.showMigrations {
				t.Errorf("showMigrations: expected %v, got %v", tt.expected.showMigrations, result.showMigrations)
			}
			if result.showWhenStopped != tt.expected.showWhenStopped {
				t.Errorf("showWhenStopped: expected %v, got %v", tt.expected.showWhenStopped, result.showWhenStopped)
			}
		})
	}
}

func TestParseMigrationOutput(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		expected int
	}{
		{
			name:     "no output",
			output:   "",
			expected: 0,
		},
		{
			name: "all applied",
			output: `    LOCAL  │ REMOTE │     TIME
  ─────────┼────────┼──────────────
  20240101 │   ✓    │ Applied
  20240102 │   ✓    │ Applied`,
			expected: 0,
		},
		{
			name: "one pending",
			output: `    LOCAL  │ REMOTE │     TIME
  ─────────┼────────┼──────────────
  20240101 │   ✓    │ Applied
  20240102 │        │ Not Applied`,
			expected: 1,
		},
		{
			name: "multiple pending",
			output: `    LOCAL  │ REMOTE │     TIME
  ─────────┼────────┼──────────────
  20240101 │   ✓    │ Applied
  20240102 │        │ Not Applied
  20240103 │        │ Not Applied
  20240104 │        │ Not Applied`,
			expected: 3,
		},
		{
			name: "all pending",
			output: `    LOCAL  │ REMOTE │     TIME
  ─────────┼────────┼──────────────
  20240101 │        │ Not Applied
  20240102 │        │ Not Applied`,
			expected: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseMigrationOutput(tt.output)
			if result != tt.expected {
				t.Errorf("expected %d pending migrations, got %d", tt.expected, result)
			}
		})
	}
}

func TestParseStatusOutput(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		expected bool
	}{
		{
			name:     "empty string",
			output:   "",
			expected: false,
		},
		{
			name:     "whitespace only",
			output:   "   \n\t  ",
			expected: false,
		},
		{
			name:     "null literal",
			output:   "null",
			expected: false,
		},
		{
			name:     "null with whitespace",
			output:   "  null\n",
			expected: false,
		},
		{
			name:     "empty JSON object",
			output:   "{}",
			expected: false,
		},
		{
			name:     "invalid JSON",
			output:   "not json at all",
			expected: false,
		},
		{
			name:     "valid status with services",
			output:   `{"API_URL":"http://127.0.0.1:54321","DB_URL":"postgresql://postgres:postgres@127.0.0.1:54322/postgres","STUDIO_URL":"http://127.0.0.1:54323"}`,
			expected: true,
		},
		{
			name: "valid status with whitespace",
			output: `  {
				"API_URL": "http://127.0.0.1:54321",
				"DB_URL": "postgresql://postgres:postgres@127.0.0.1:54322/postgres"
			}  `,
			expected: true,
		},
		{
			name:     "JSON array instead of object",
			output:   `["something"]`,
			expected: false,
		},
		{
			name:     "JSON string instead of object",
			output:   `"running"`,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseStatusOutput(tt.output)
			if result != tt.expected {
				t.Errorf("parseStatusOutput(%q) = %v, want %v", tt.output, result, tt.expected)
			}
		})
	}
}

func TestSupabasePlugin_NoConfigToml(t *testing.T) {
	p := &SupabasePlugin{}
	c := cache.New()
	p.SetCache(c)

	// Use a temp dir with no supabase/config.toml
	tmpDir := t.TempDir()

	ctx := context.Background()
	input := plugin.Input{
		Prism: plugin.PrismContext{
			ProjectDir: tmpDir,
		},
		Config: map[string]any{},
		Colors: map[string]string{
			"supabase_green": "[supabase_green]",
			"gray":           "[gray]",
			"reset":          "[reset]",
		},
	}

	result, err := p.Execute(ctx, input)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result != "" {
		t.Errorf("expected empty string for non-supabase project, got '%s'", result)
	}
}

func TestSupabasePlugin_IsSupabaseProject(t *testing.T) {
	p := &SupabasePlugin{}
	c := cache.New()
	p.SetCache(c)

	// Create a temp dir with supabase/config.toml
	tmpDir := t.TempDir()
	supabaseDir := filepath.Join(tmpDir, "supabase")
	if err := os.MkdirAll(supabaseDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(supabaseDir, "config.toml"), []byte("[project]\nid = \"test\""), 0o644); err != nil {
		t.Fatal(err)
	}

	if !p.isSupabaseProject(tmpDir) {
		t.Error("expected isSupabaseProject to return true")
	}

	// Should be cached now
	if cached, ok := c.Get("supabase:detect:" + tmpDir); !ok || cached != "true" {
		t.Error("expected detection result to be cached")
	}

	// Non-supabase dir
	emptyDir := t.TempDir()
	if p.isSupabaseProject(emptyDir) {
		t.Error("expected isSupabaseProject to return false for empty dir")
	}
}

func TestSupabasePlugin_OnHook_InvalidatesCache(t *testing.T) {
	p := &SupabasePlugin{}
	c := cache.New()
	p.SetCache(c)

	// Add items to cache
	c.Set("supabase:output:/test", "test value", time.Minute)
	c.Set("supabase:detect:/test", "true", time.Minute)
	c.Set("supabase:migrations:/test", " ↑2", time.Minute)

	// Verify they're there
	if _, ok := c.Get("supabase:output:/test"); !ok {
		t.Fatal("cache item not found before hook")
	}

	// Fire idle hook
	_, err := p.OnHook(context.Background(), HookIdle, HookContext{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// All supabase cache should be invalidated
	if _, ok := c.Get("supabase:output:/test"); ok {
		t.Error("output cache should be invalidated after idle hook")
	}
	if _, ok := c.Get("supabase:detect:/test"); ok {
		t.Error("detect cache should be invalidated after idle hook")
	}
	if _, ok := c.Get("supabase:migrations:/test"); ok {
		t.Error("migrations cache should be invalidated after idle hook")
	}
}

func TestSupabasePlugin_OnHook_OtherHooksIgnored(t *testing.T) {
	p := &SupabasePlugin{}
	c := cache.New()
	p.SetCache(c)

	c.Set("supabase:output:/test", "test value", time.Minute)

	// Fire busy hook (not idle)
	_, err := p.OnHook(context.Background(), HookBusy, HookContext{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Cache should NOT be invalidated
	if _, ok := c.Get("supabase:output:/test"); !ok {
		t.Error("cache item should not be invalidated by busy hook")
	}
}

func TestSupabasePlugin_OnHook_NilCache(t *testing.T) {
	p := &SupabasePlugin{} // No cache set

	// Should not panic
	_, err := p.OnHook(context.Background(), HookIdle, HookContext{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSupabasePlugin_EmptyProjectDir(t *testing.T) {
	p := &SupabasePlugin{}
	c := cache.New()
	p.SetCache(c)

	ctx := context.Background()
	input := plugin.Input{
		Prism: plugin.PrismContext{
			ProjectDir: "",
		},
		Config: map[string]any{},
		Colors: map[string]string{},
	}

	result, err := p.Execute(ctx, input)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result != "" {
		t.Errorf("expected empty string for empty project dir, got '%s'", result)
	}
}

func TestSupabasePlugin_Execute_Caching(t *testing.T) {
	p := &SupabasePlugin{}
	c := cache.New()
	p.SetCache(c)

	// Pre-populate cache
	expectedOutput := "[supabase_green]⚡[reset]"
	c.Set("supabase:output:/test/dir", expectedOutput, time.Minute)

	ctx := context.Background()
	input := plugin.Input{
		Prism: plugin.PrismContext{
			ProjectDir: "/test/dir",
		},
		Config: map[string]any{},
		Colors: map[string]string{
			"supabase_green": "[supabase_green]",
			"gray":           "[gray]",
			"reset":          "[reset]",
		},
	}

	result, err := p.Execute(ctx, input)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result != expectedOutput {
		t.Errorf("expected cached output '%s', got '%s'", expectedOutput, result)
	}
}
