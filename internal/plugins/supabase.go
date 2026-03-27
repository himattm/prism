package plugins

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/himattm/prism/internal/cache"
	"github.com/himattm/prism/internal/plugin"
)

// SupabasePlugin displays Supabase local dev stack status and pending migrations
type SupabasePlugin struct {
	cache *cache.Cache
}

// supabaseConfig holds plugin configuration
type supabaseConfig struct {
	showMigrations  bool
	showWhenStopped bool
}

func (p *SupabasePlugin) Name() string {
	return "supabase"
}

func (p *SupabasePlugin) SetCache(c *cache.Cache) {
	p.cache = c
}

// OnHook invalidates Supabase cache when Claude becomes idle
func (p *SupabasePlugin) OnHook(ctx context.Context, hookType HookType, hookCtx HookContext) (string, error) {
	if hookType == HookIdle && p.cache != nil {
		p.cache.DeleteByPrefix("supabase:")
	}
	return "", nil
}

func (p *SupabasePlugin) Execute(ctx context.Context, input plugin.Input) (string, error) {
	projectDir := input.Prism.ProjectDir
	if projectDir == "" {
		return "", nil
	}

	// Check cache for full output
	cacheKey := "supabase:output:" + projectDir
	if p.cache != nil {
		if cached, ok := p.cache.Get(cacheKey); ok {
			return cached, nil
		}
	}

	// Detect Supabase project
	if !p.isSupabaseProject(projectDir) {
		return "", nil
	}

	// Check CLI availability
	supabasePath, err := exec.LookPath("supabase")
	if err != nil {
		return "", nil
	}

	cfg := parseSupabaseConfig(input.Config)

	// Check local stack status
	running := p.checkLocalStatus(ctx, supabasePath, projectDir)

	if !running && !cfg.showWhenStopped {
		return "", nil
	}

	// Build output
	green := input.Colors["supabase_green"]
	gray := input.Colors["gray"]
	reset := input.Colors["reset"]

	var result strings.Builder
	if running {
		result.WriteString(green)
		result.WriteString("ϟ Supabase: up")
	} else {
		result.WriteString(gray)
		result.WriteString("ϟ Supabase: stopped")
	}

	// Fetch migrations (idle-only for fresh data, use cache otherwise)
	if running && cfg.showMigrations {
		migrationKey := "supabase:migrations:" + projectDir
		if input.Prism.IsIdle {
			pending := countPendingMigrations(ctx, supabasePath, projectDir)
			migrationFragment := ""
			if pending > 0 {
				migrationFragment = fmt.Sprintf(" ↑%d", pending)
			}
			if p.cache != nil {
				p.cache.Set(migrationKey, migrationFragment, cache.ConfigTTL)
			}
			result.WriteString(migrationFragment)
		} else if p.cache != nil {
			if cached, ok := p.cache.Get(migrationKey); ok {
				result.WriteString(cached)
			}
		}
	}

	result.WriteString(reset)
	output := result.String()

	if p.cache != nil {
		p.cache.Set(cacheKey, output, cache.SupabaseTTL)
	}

	return output, nil
}

// isSupabaseProject checks for supabase/config.toml in the project directory
func (p *SupabasePlugin) isSupabaseProject(projectDir string) bool {
	detectKey := "supabase:detect:" + projectDir
	if p.cache != nil {
		if cached, ok := p.cache.Get(detectKey); ok {
			return cached == "true"
		}
	}

	configPath := filepath.Join(projectDir, "supabase", "config.toml")
	_, err := os.Stat(configPath)
	exists := err == nil

	if p.cache != nil {
		val := "false"
		if exists {
			val = "true"
		}
		p.cache.Set(detectKey, val, cache.ConfigTTL)
	}

	return exists
}

// checkLocalStatus runs `supabase status --output json` to determine if the local stack is running
func (p *SupabasePlugin) checkLocalStatus(ctx context.Context, supabasePath, projectDir string) bool {
	cmd := exec.CommandContext(ctx, supabasePath, "status", "--output", "json")
	cmd.Dir = projectDir
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &bytes.Buffer{}

	if err := cmd.Run(); err != nil {
		return false
	}

	return parseStatusOutput(out.String())
}

// parseStatusOutput determines if the Supabase local stack is running based on
// the JSON output from `supabase status --output json`. Returns false for empty
// strings, "null", empty JSON objects, and invalid JSON. Returns true only when
// the output contains a valid JSON object with at least one meaningful key
// (e.g., "API_URL", "DB_URL").
func parseStatusOutput(output string) bool {
	trimmed := strings.TrimSpace(output)
	if trimmed == "" || trimmed == "null" {
		return false
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(trimmed), &parsed); err != nil {
		return false
	}

	// An empty object means no services are running
	return len(parsed) > 0
}

// countPendingMigrations runs `supabase migration list` and counts unapplied migrations
func countPendingMigrations(ctx context.Context, supabasePath, projectDir string) int {
	cmd := exec.CommandContext(ctx, supabasePath, "migration", "list")
	cmd.Dir = projectDir
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &bytes.Buffer{}

	if err := cmd.Run(); err != nil {
		return 0
	}

	return parseMigrationOutput(out.String())
}

// parseMigrationOutput counts lines containing "Not Applied" in migration list output
func parseMigrationOutput(output string) int {
	count := 0
	for _, line := range strings.Split(output, "\n") {
		if strings.Contains(strings.ToLower(line), "not applied") {
			count++
		}
	}
	return count
}

func parseSupabaseConfig(config map[string]any) supabaseConfig {
	cfg := supabaseConfig{
		showMigrations:  true,
		showWhenStopped: false,
	}

	supabaseCfg, ok := config["supabase"].(map[string]any)
	if !ok {
		return cfg
	}

	if v, ok := supabaseCfg["show_migrations"].(bool); ok {
		cfg.showMigrations = v
	}
	if v, ok := supabaseCfg["show_when_stopped"].(bool); ok {
		cfg.showWhenStopped = v
	}

	return cfg
}
