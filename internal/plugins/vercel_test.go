package plugins

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
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
				ShowTeam:     false,
			},
		},
		{
			name: "all options",
			input: map[string]any{
				"vercel": map[string]any{
					"show_url":       true,
					"max_url_length": float64(40),
					"show_team":      true,
				},
			},
			expected: vercelConfig{
				ShowURL:      true,
				MaxURLLength: 40,
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
			name:     "with URL",
			deploy:   &vercelDeployment{State: "READY", URL: "my-app-abc.vercel.app"},
			cfg:      vercelConfig{ShowURL: true, MaxURLLength: 30},
			expected: "[emerald]▲ ready my-app-abc.vercel.app[reset]",
		},
		{
			name:     "with long URL truncated",
			deploy:   &vercelDeployment{State: "READY", URL: "my-very-long-app-name-abc123def456.vercel.app"},
			cfg:      vercelConfig{ShowURL: true, MaxURLLength: 20},
			expected: "[emerald]▲ ready my-very-long-app-na…[reset]",
		},
		{
			name:     "with team",
			deploy:   &vercelDeployment{State: "READY"},
			cfg:      vercelConfig{ShowTeam: true},
			team:     "my-team",
			expected: "[emerald]▲ my-team: ready[reset]",
		},
		{
			name:     "all options",
			deploy:   &vercelDeployment{State: "BUILDING", URL: "app.vercel.app"},
			cfg:      vercelConfig{ShowURL: true, MaxURLLength: 30, ShowTeam: true},
			team:     "acme",
			expected: "[yellow]▲ acme: building app.vercel.app[reset]",
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

// --- CLI interaction tests using mock scripts ---

// createMockScript creates a temporary executable script that prints the given
// stdout content and exits with the given code. Returns the path to the script.
func createMockScript(t *testing.T, stdout string, exitCode int) string {
	t.Helper()
	tmpDir := t.TempDir()
	var scriptPath string
	if runtime.GOOS == "windows" {
		scriptPath = filepath.Join(tmpDir, "mock-vercel.bat")
		content := fmt.Sprintf("@echo off\necho %s\nexit /b %d", stdout, exitCode)
		if err := os.WriteFile(scriptPath, []byte(content), 0755); err != nil {
			t.Fatal(err)
		}
	} else {
		scriptPath = filepath.Join(tmpDir, "mock-vercel")
		content := fmt.Sprintf("#!/bin/sh\nprintf '%%s' '%s'\nexit %d", stdout, exitCode)
		if err := os.WriteFile(scriptPath, []byte(content), 0755); err != nil {
			t.Fatal(err)
		}
	}
	return scriptPath
}

// createMockStderrScript creates a script that writes to both stdout and stderr.
func createMockStderrScript(t *testing.T, stdout, stderr string, exitCode int) string {
	t.Helper()
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "mock-vercel")
	content := fmt.Sprintf("#!/bin/sh\nprintf '%%s' '%s'\nprintf '%%s' '%s' >&2\nexit %d", stdout, stderr, exitCode)
	if err := os.WriteFile(scriptPath, []byte(content), 0755); err != nil {
		t.Fatal(err)
	}
	return scriptPath
}

func TestGetLatestVercelDeployment_Success(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping shell script test on Windows")
	}

	jsonResponse := `{"deployments":[{"uid":"dpl_abc123","name":"my-app","state":"READY","url":"my-app-abc.vercel.app"}]}`
	mockPath := createMockScript(t, jsonResponse, 0)

	ctx := context.Background()
	deployment, err := getLatestVercelDeployment(ctx, mockPath, "prj_test123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deployment == nil {
		t.Fatal("expected non-nil deployment")
	}
	if deployment.UID != "dpl_abc123" {
		t.Errorf("expected UID 'dpl_abc123', got '%s'", deployment.UID)
	}
	if deployment.Name != "my-app" {
		t.Errorf("expected Name 'my-app', got '%s'", deployment.Name)
	}
	if deployment.State != "READY" {
		t.Errorf("expected State 'READY', got '%s'", deployment.State)
	}
	if deployment.URL != "my-app-abc.vercel.app" {
		t.Errorf("expected URL 'my-app-abc.vercel.app', got '%s'", deployment.URL)
	}
}

func TestGetLatestVercelDeployment_EmptyDeployments(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping shell script test on Windows")
	}

	jsonResponse := `{"deployments":[]}`
	mockPath := createMockScript(t, jsonResponse, 0)

	ctx := context.Background()
	deployment, err := getLatestVercelDeployment(ctx, mockPath, "prj_test123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deployment != nil {
		t.Errorf("expected nil deployment for empty list, got %+v", deployment)
	}
}

func TestGetLatestVercelDeployment_CLIError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping shell script test on Windows")
	}

	mockPath := createMockStderrScript(t, "", "authentication required", 1)

	ctx := context.Background()
	deployment, err := getLatestVercelDeployment(ctx, mockPath, "prj_test123")
	if err == nil {
		t.Fatal("expected error when CLI fails")
	}
	if deployment != nil {
		t.Errorf("expected nil deployment on error, got %+v", deployment)
	}
	// Verify stderr content is included in error message
	if !strings.Contains(err.Error(), "authentication required") {
		t.Errorf("expected error to contain stderr output, got: %v", err)
	}
}

func TestGetLatestVercelDeployment_InvalidJSON(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping shell script test on Windows")
	}

	mockPath := createMockScript(t, "not valid json at all", 0)

	ctx := context.Background()
	deployment, err := getLatestVercelDeployment(ctx, mockPath, "prj_test123")
	if err == nil {
		t.Fatal("expected error for invalid JSON response")
	}
	if deployment != nil {
		t.Errorf("expected nil deployment for invalid JSON, got %+v", deployment)
	}
}

func TestGetLatestVercelDeployment_BuildingState(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping shell script test on Windows")
	}

	jsonResponse := `{"deployments":[{"uid":"dpl_build1","name":"my-app","state":"BUILDING","url":""}]}`
	mockPath := createMockScript(t, jsonResponse, 0)

	ctx := context.Background()
	deployment, err := getLatestVercelDeployment(ctx, mockPath, "prj_test123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deployment == nil {
		t.Fatal("expected non-nil deployment")
	}
	if deployment.State != "BUILDING" {
		t.Errorf("expected State 'BUILDING', got '%s'", deployment.State)
	}
}

func TestGetLatestVercelDeployment_ContextCanceled(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping shell script test on Windows")
	}

	// Create a script that sleeps for a long time
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "mock-vercel-slow")
	if err := os.WriteFile(scriptPath, []byte("#!/bin/sh\nsleep 30"), 0755); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	deployment, err := getLatestVercelDeployment(ctx, scriptPath, "prj_test123")
	if err == nil {
		t.Fatal("expected error when context is canceled")
	}
	if deployment != nil {
		t.Errorf("expected nil deployment on canceled context, got %+v", deployment)
	}
}

func TestGetVercelTeam_Success(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping shell script test on Windows")
	}

	mockPath := createMockScript(t, "my-team-name", 0)

	p := &VercelPlugin{}
	c := cache.New()
	p.SetCache(c)

	ctx := context.Background()
	team := p.getVercelTeam(ctx, mockPath)
	if team != "my-team-name" {
		t.Errorf("expected team 'my-team-name', got '%s'", team)
	}

	// Verify it was cached
	if cached, ok := c.Get("vercel:team"); !ok || cached != "my-team-name" {
		t.Errorf("expected team to be cached, got cached=%s, ok=%v", cached, ok)
	}
}

func TestGetVercelTeam_CLIError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping shell script test on Windows")
	}

	mockPath := createMockStderrScript(t, "", "not logged in", 1)

	p := &VercelPlugin{}
	c := cache.New()
	p.SetCache(c)

	ctx := context.Background()
	team := p.getVercelTeam(ctx, mockPath)
	if team != "" {
		t.Errorf("expected empty team on error, got '%s'", team)
	}

	// Verify nothing was cached
	if _, ok := c.Get("vercel:team"); ok {
		t.Error("team should not be cached on error")
	}
}

func TestGetVercelTeam_UsesCache(t *testing.T) {
	p := &VercelPlugin{}
	c := cache.New()
	p.SetCache(c)

	// Pre-populate cache
	c.Set("vercel:team", "cached-team", time.Minute)

	ctx := context.Background()
	// Pass a non-existent path; if cache works, it won't even try to run it
	team := p.getVercelTeam(ctx, "/nonexistent/vercel")
	if team != "cached-team" {
		t.Errorf("expected cached team 'cached-team', got '%s'", team)
	}
}

func TestGetVercelTeam_TrimsWhitespace(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping shell script test on Windows")
	}

	// Create script that outputs team name with trailing whitespace/newlines
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "mock-vercel")
	if err := os.WriteFile(scriptPath, []byte("#!/bin/sh\nprintf 'my-team\\n  '"), 0755); err != nil {
		t.Fatal(err)
	}

	p := &VercelPlugin{}
	c := cache.New()
	p.SetCache(c)

	ctx := context.Background()
	team := p.getVercelTeam(ctx, scriptPath)
	if team != "my-team" {
		t.Errorf("expected trimmed team 'my-team', got '%s'", team)
	}
}

func TestGetLatestVercelDeployment_CommandArgs(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping shell script test on Windows")
	}

	// Create a script that echoes the arguments it receives, so we can verify
	// the correct args are passed to the vercel CLI
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "mock-vercel")
	// Script writes args to a file, then outputs valid JSON
	argsFile := filepath.Join(tmpDir, "args.txt")
	scriptContent := fmt.Sprintf(`#!/bin/sh
echo "$@" > %s
printf '{"deployments":[]}'
`, argsFile)
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	_, err := getLatestVercelDeployment(ctx, scriptPath, "prj_myproject")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Read the captured args
	argsData, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("failed to read args file: %v", err)
	}
	args := strings.TrimSpace(string(argsData))
	expectedArgs := "api /v6/deployments?limit=1&projectId=prj_myproject"
	if args != expectedArgs {
		t.Errorf("expected args '%s', got '%s'", expectedArgs, args)
	}
}

// verifyVercelCLIExists checks if the vercel CLI is available for integration tests
func verifyVercelCLIExists(t *testing.T) string {
	t.Helper()
	path, err := exec.LookPath("vercel")
	if err != nil {
		t.Skip("vercel CLI not available, skipping integration test")
	}
	return path
}
