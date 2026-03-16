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

func TestVercelPlugin_Name(t *testing.T) {
	p := &VercelPlugin{}
	if p.Name() != "vercel" {
		t.Errorf("expected name 'vercel', got '%s'", p.Name())
	}
}

func TestVercelPlugin_SetCache(t *testing.T) {
	p := &VercelPlugin{}
	c := cache.New()
	p.SetCache(c)

	if p.cache != c {
		t.Error("cache was not set correctly")
	}
}

func TestParseVercelConfig(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]any
		expected vercelConfig
	}{
		{
			name:  "empty config uses defaults",
			input: map[string]any{},
			expected: vercelConfig{
				ShowURL:      false,
				MaxURLLength: 30,
				ShowBranch:   false,
				ShowTeam:     false,
			},
		},
		{
			name: "show_url true",
			input: map[string]any{
				"vercel": map[string]any{
					"show_url": true,
				},
			},
			expected: vercelConfig{
				ShowURL:      true,
				MaxURLLength: 30,
				ShowBranch:   false,
				ShowTeam:     false,
			},
		},
		{
			name: "custom max_url_length",
			input: map[string]any{
				"vercel": map[string]any{
					"max_url_length": float64(50),
				},
			},
			expected: vercelConfig{
				ShowURL:      false,
				MaxURLLength: 50,
				ShowBranch:   false,
				ShowTeam:     false,
			},
		},
		{
			name: "all options",
			input: map[string]any{
				"vercel": map[string]any{
					"show_url":       true,
					"max_url_length": float64(40),
					"show_branch":    true,
					"show_team":      true,
				},
			},
			expected: vercelConfig{
				ShowURL:      true,
				MaxURLLength: 40,
				ShowBranch:   true,
				ShowTeam:     true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseVercelConfig(tt.input)
			if result.ShowURL != tt.expected.ShowURL {
				t.Errorf("ShowURL: expected %v, got %v", tt.expected.ShowURL, result.ShowURL)
			}
			if result.MaxURLLength != tt.expected.MaxURLLength {
				t.Errorf("MaxURLLength: expected %d, got %d", tt.expected.MaxURLLength, result.MaxURLLength)
			}
			if result.ShowBranch != tt.expected.ShowBranch {
				t.Errorf("ShowBranch: expected %v, got %v", tt.expected.ShowBranch, result.ShowBranch)
			}
			if result.ShowTeam != tt.expected.ShowTeam {
				t.Errorf("ShowTeam: expected %v, got %v", tt.expected.ShowTeam, result.ShowTeam)
			}
		})
	}
}

func TestReadVercelProject(t *testing.T) {
	t.Run("valid project.json", func(t *testing.T) {
		tmpDir := t.TempDir()
		vercelDir := filepath.Join(tmpDir, ".vercel")
		os.MkdirAll(vercelDir, 0755)
		os.WriteFile(filepath.Join(vercelDir, "project.json"), []byte(`{"projectId":"prj_abc123","orgId":"team_xyz"}`), 0644)

		project, err := readVercelProject(tmpDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if project.ProjectID != "prj_abc123" {
			t.Errorf("expected projectId 'prj_abc123', got '%s'", project.ProjectID)
		}
		if project.OrgID != "team_xyz" {
			t.Errorf("expected orgId 'team_xyz', got '%s'", project.OrgID)
		}
	})

	t.Run("missing project.json", func(t *testing.T) {
		tmpDir := t.TempDir()

		project, err := readVercelProject(tmpDir)
		if err == nil {
			t.Error("expected error for missing project.json")
		}
		if project != nil {
			t.Error("expected nil project")
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		tmpDir := t.TempDir()
		vercelDir := filepath.Join(tmpDir, ".vercel")
		os.MkdirAll(vercelDir, 0755)
		os.WriteFile(filepath.Join(vercelDir, "project.json"), []byte(`not json`), 0644)

		project, err := readVercelProject(tmpDir)
		if err == nil {
			t.Error("expected error for invalid JSON")
		}
		if project != nil {
			t.Error("expected nil project")
		}
	})

	t.Run("empty projectId", func(t *testing.T) {
		tmpDir := t.TempDir()
		vercelDir := filepath.Join(tmpDir, ".vercel")
		os.MkdirAll(vercelDir, 0755)
		os.WriteFile(filepath.Join(vercelDir, "project.json"), []byte(`{"projectId":"","orgId":"team_xyz"}`), 0644)

		project, err := readVercelProject(tmpDir)
		if err == nil {
			t.Error("expected error for empty projectId")
		}
		if project != nil {
			t.Error("expected nil project")
		}
	})
}

func TestFormatVercelOutput(t *testing.T) {
	colors := map[string]string{
		"emerald": "[emerald]",
		"yellow":  "[yellow]",
		"red":     "[red]",
		"gray":    "[gray]",
		"reset":   "[reset]",
	}

	input := plugin.Input{
		Colors: colors,
	}

	tests := []struct {
		name     string
		deploy   *vercelDeployment
		cfg      vercelConfig
		team     string
		expected string
	}{
		{
			name:     "ready state",
			deploy:   &vercelDeployment{State: "READY", URL: "my-app-abc.vercel.app"},
			cfg:      vercelConfig{},
			expected: "[emerald]▲ ready[reset]",
		},
		{
			name:     "building state",
			deploy:   &vercelDeployment{State: "BUILDING"},
			cfg:      vercelConfig{},
			expected: "[yellow]▲ building[reset]",
		},
		{
			name:     "error state",
			deploy:   &vercelDeployment{State: "ERROR"},
			cfg:      vercelConfig{},
			expected: "[red]▲ error[reset]",
		},
		{
			name:     "queued state",
			deploy:   &vercelDeployment{State: "QUEUED"},
			cfg:      vercelConfig{},
			expected: "[gray]▲ queued[reset]",
		},
		{
			name:     "canceled state",
			deploy:   &vercelDeployment{State: "CANCELED"},
			cfg:      vercelConfig{},
			expected: "[gray]▲ canceled[reset]",
		},
		{
			name:     "unknown state",
			deploy:   &vercelDeployment{State: "INITIALIZING"},
			cfg:      vercelConfig{},
			expected: "[gray]▲ initializing[reset]",
		},
		{
			name:   "with URL",
			deploy: &vercelDeployment{State: "READY", URL: "my-app-abc.vercel.app"},
			cfg:    vercelConfig{ShowURL: true, MaxURLLength: 30},
			expected: "[emerald]▲ ready my-app-abc.vercel.app[reset]",
		},
		{
			name:   "with long URL truncated",
			deploy: &vercelDeployment{State: "READY", URL: "my-very-long-app-name-abc123def456.vercel.app"},
			cfg:    vercelConfig{ShowURL: true, MaxURLLength: 20},
			expected: "[emerald]▲ ready my-very-long-app-na…[reset]",
		},
		{
			name: "with branch",
			deploy: func() *vercelDeployment {
				d := &vercelDeployment{State: "READY"}
				d.Meta.GithubCommitRef = "main"
				return d
			}(),
			cfg:      vercelConfig{ShowBranch: true},
			expected: "[emerald]▲ ready (main)[reset]",
		},
		{
			name:     "with team",
			deploy:   &vercelDeployment{State: "READY"},
			cfg:      vercelConfig{ShowTeam: true},
			team:     "my-team",
			expected: "[emerald]▲ my-team: ready[reset]",
		},
		{
			name: "all options",
			deploy: func() *vercelDeployment {
				d := &vercelDeployment{State: "BUILDING", URL: "app.vercel.app"}
				d.Meta.GithubCommitRef = "feat/new"
				return d
			}(),
			cfg:      vercelConfig{ShowURL: true, MaxURLLength: 30, ShowBranch: true, ShowTeam: true},
			team:     "acme",
			expected: "[yellow]▲ acme: building app.vercel.app (feat/new)[reset]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatVercelOutput(input, tt.deploy, tt.cfg, tt.team)
			if result != tt.expected {
				t.Errorf("expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestVercelPlugin_OnHook_InvalidatesCache(t *testing.T) {
	p := &VercelPlugin{}
	c := cache.New()
	p.SetCache(c)

	// Add items to cache
	c.Set("vercel:deploy:/some/project", "test value", time.Minute)
	c.Set("vercel:team", "my-team", time.Minute)

	// Verify deploy cache is there
	if _, ok := c.Get("vercel:deploy:/some/project"); !ok {
		t.Fatal("cache item not found before hook")
	}

	// Fire idle hook
	_, err := p.OnHook(context.Background(), HookIdle, HookContext{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Deploy cache should be invalidated
	if _, ok := c.Get("vercel:deploy:/some/project"); ok {
		t.Error("deploy cache should be invalidated after idle hook")
	}

	// Team cache should NOT be invalidated (different prefix)
	if _, ok := c.Get("vercel:team"); !ok {
		t.Error("team cache should not be invalidated by idle hook")
	}
}

func TestVercelPlugin_OnHook_OtherHooksIgnored(t *testing.T) {
	p := &VercelPlugin{}
	c := cache.New()
	p.SetCache(c)

	c.Set("vercel:deploy:/some/project", "test value", time.Minute)

	_, err := p.OnHook(context.Background(), HookBusy, HookContext{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if _, ok := c.Get("vercel:deploy:/some/project"); !ok {
		t.Error("cache should not be invalidated by busy hook")
	}
}

func TestVercelPlugin_OnHook_NilCache(t *testing.T) {
	p := &VercelPlugin{} // No cache set

	_, err := p.OnHook(context.Background(), HookIdle, HookContext{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestVercelPlugin_Execute_Caching(t *testing.T) {
	p := &VercelPlugin{}
	c := cache.New()
	p.SetCache(c)

	expectedOutput := "[emerald]▲ ready[reset]"
	c.Set("vercel:deploy:/my/project", expectedOutput, time.Minute)

	ctx := context.Background()
	input := plugin.Input{
		Prism: plugin.PrismContext{
			ProjectDir: "/my/project",
		},
		Config: map[string]any{},
		Colors: map[string]string{
			"emerald": "[emerald]",
			"reset":   "[reset]",
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

func TestVercelPlugin_Execute_NoVercelCLI(t *testing.T) {
	p := &VercelPlugin{}
	c := cache.New()
	p.SetCache(c)

	// Set PATH to empty so vercel won't be found
	originalPath := os.Getenv("PATH")
	os.Setenv("PATH", "")
	defer os.Setenv("PATH", originalPath)

	ctx := context.Background()
	input := plugin.Input{
		Prism: plugin.PrismContext{
			ProjectDir: "/some/project",
		},
		Config: map[string]any{},
		Colors: map[string]string{},
	}

	result, err := p.Execute(ctx, input)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result != "" {
		t.Errorf("expected empty result when vercel not installed, got '%s'", result)
	}
}
