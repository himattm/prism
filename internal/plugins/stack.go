package plugins

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/himattm/prism/internal/cache"
	"github.com/himattm/prism/internal/plugin"
)

const (
	stackCacheTTL = 30 * time.Second // Stack rarely changes mid-session
	stackCacheKey = "stack:detected"
)

// StackPlugin auto-detects the project's tech stack by scanning for marker files
type StackPlugin struct {
	cache *cache.Cache
}

func (p *StackPlugin) Name() string { return "stack" }

func (p *StackPlugin) SetCache(c *cache.Cache) { p.cache = c }

// OnHook invalidates stack cache on session start (fresh detection)
func (p *StackPlugin) OnHook(ctx context.Context, hookType HookType, hookCtx HookContext) (string, error) {
	if hookType == HookSessionStart && p.cache != nil {
		p.cache.DeleteByPrefix("stack:")
	}
	return "", nil
}

func (p *StackPlugin) Execute(ctx context.Context, input plugin.Input) (string, error) {
	dir := input.Prism.ProjectDir
	if dir == "" {
		dir = input.Prism.CurrentDir
	}
	if dir == "" {
		return "", nil
	}

	cacheKey := stackCacheKey + ":" + dir
	if p.cache != nil {
		if cached, ok := p.cache.Get(cacheKey); ok {
			return cached, nil
		}
	}

	cfg := parseStackConfig(input.Config)
	detected := detectStack(dir, cfg)
	if len(detected) == 0 {
		return "", nil
	}

	output := formatStack(input, detected)

	if p.cache != nil {
		p.cache.Set(cacheKey, output, stackCacheTTL)
	}

	return output, nil
}

// stackConfig holds plugin configuration
type stackConfig struct {
	maxItems int      // max items to show (default 4)
	hide     []string // items to hide
}

func parseStackConfig(config map[string]any) stackConfig {
	cfg := stackConfig{
		maxItems: 4,
	}

	stackCfg, ok := config["stack"].(map[string]any)
	if !ok {
		return cfg
	}

	if v, ok := stackCfg["max_items"].(float64); ok {
		cfg.maxItems = int(v)
	}
	if v, ok := stackCfg["hide"].([]any); ok {
		for _, item := range v {
			if s, ok := item.(string); ok {
				cfg.hide = append(cfg.hide, s)
			}
		}
	}

	return cfg
}

// stackItem represents a detected technology
type stackItem struct {
	Name     string // Display name (e.g. "Next.js")
	Category string // Category for sorting: "runtime", "framework", "service", "infra"
	Color    string // Color key from the color map
}

// detectionRule defines how to detect a technology
type detectionRule struct {
	Item       stackItem
	Files      []string // Check if any of these files exist
	Dirs       []string // Check if any of these dirs exist
	PkgJSON    []string // Check if any of these appear in package.json dependencies
	HasPkgJSON bool     // Only check if package.json exists (for Node runtime detection)
}

// allRules defines all detection rules, ordered by priority within each category
var allRules = []detectionRule{
	// === Frameworks (most specific first) ===
	{
		Item:    stackItem{Name: "Next.js", Category: "framework", Color: "white"},
		Files:   []string{"next.config.js", "next.config.mjs", "next.config.ts"},
		PkgJSON: []string{"next"},
	},
	{
		Item:    stackItem{Name: "Nuxt", Category: "framework", Color: "emerald"},
		Files:   []string{"nuxt.config.js", "nuxt.config.ts"},
		PkgJSON: []string{"nuxt"},
	},
	{
		Item:    stackItem{Name: "SvelteKit", Category: "framework", Color: "coral"},
		Files:   []string{"svelte.config.js", "svelte.config.ts"},
		PkgJSON: []string{"@sveltejs/kit"},
	},
	{
		Item:    stackItem{Name: "Remix", Category: "framework", Color: "violet"},
		PkgJSON: []string{"@remix-run/node", "@remix-run/react"},
	},
	{
		Item:    stackItem{Name: "Astro", Category: "framework", Color: "orchid"},
		Files:   []string{"astro.config.mjs", "astro.config.ts"},
		PkgJSON: []string{"astro"},
	},
	{
		Item:    stackItem{Name: "Vite", Category: "framework", Color: "violet"},
		Files:   []string{"vite.config.js", "vite.config.ts", "vite.config.mjs"},
		PkgJSON: []string{"vite"},
	},
	{
		Item:    stackItem{Name: "React", Category: "framework", Color: "sky_blue"},
		PkgJSON: []string{"react"},
	},
	{
		Item:    stackItem{Name: "Vue", Category: "framework", Color: "emerald"},
		PkgJSON: []string{"vue"},
	},
	{
		Item:    stackItem{Name: "Svelte", Category: "framework", Color: "coral"},
		PkgJSON: []string{"svelte"},
	},
	{
		Item:    stackItem{Name: "Angular", Category: "framework", Color: "red"},
		Files:   []string{"angular.json"},
		PkgJSON: []string{"@angular/core"},
	},
	{
		// manage.py alone is too broad (any Python script can be named this).
		// Use Django-specific project structure markers instead.
		Item:  stackItem{Name: "Django", Category: "framework", Color: "dark_green"},
		Files: []string{"config/urls.py", "config/wsgi.py", "config/settings.py"},
	},
	{
		// wsgi.py alone is generic WSGI, not Flask-specific.
		// .flaskenv is a Flask-specific configuration file.
		Item:  stackItem{Name: "Flask", Category: "framework", Color: "white"},
		Files: []string{".flaskenv"},
	},
	{
		Item:  stackItem{Name: "Rails", Category: "framework", Color: "red"},
		Files: []string{"config/routes.rb"},
	},
	{
		Item:  stackItem{Name: "Laravel", Category: "framework", Color: "red"},
		Files: []string{"artisan"},
	},

	// === Services & Platforms ===
	{
		Item:    stackItem{Name: "Supabase", Category: "service", Color: "emerald"},
		Dirs:    []string{"supabase", ".supabase"},
		Files:   []string{"supabase/config.toml"},
		PkgJSON: []string{"@supabase/supabase-js", "supabase"},
	},
	{
		Item:  stackItem{Name: "Vercel", Category: "service", Color: "white"},
		Files: []string{"vercel.json"},
		Dirs:  []string{".vercel"},
	},
	{
		Item:  stackItem{Name: "Netlify", Category: "service", Color: "teal"},
		Files: []string{"netlify.toml"},
		Dirs:  []string{".netlify"},
	},
	{
		Item:    stackItem{Name: "Firebase", Category: "service", Color: "orange"},
		Files:   []string{"firebase.json", ".firebaserc"},
		PkgJSON: []string{"firebase"},
	},
	{
		Item:    stackItem{Name: "Prisma", Category: "service", Color: "dark_violet"},
		Files:   []string{"prisma/schema.prisma"},
		PkgJSON: []string{"prisma", "@prisma/client"},
	},
	{
		Item:    stackItem{Name: "Drizzle", Category: "service", Color: "lime_green"},
		Files:   []string{"drizzle.config.ts", "drizzle.config.js"},
		PkgJSON: []string{"drizzle-orm"},
	},
	{
		Item:  stackItem{Name: "Terraform", Category: "service", Color: "violet"},
		Files: []string{"main.tf", "terraform.tfvars"},
		Dirs:  []string{".terraform"},
	},
	{
		Item:  stackItem{Name: "Pulumi", Category: "service", Color: "violet"},
		Files: []string{"Pulumi.yaml"},
	},
	{
		Item:  stackItem{Name: "AWS CDK", Category: "service", Color: "orange"},
		Files: []string{"cdk.json"},
	},

	// === Infrastructure ===
	{
		Item:  stackItem{Name: "Docker", Category: "infra", Color: "dodger_blue"},
		Files: []string{"Dockerfile", "docker-compose.yml", "docker-compose.yaml", "compose.yml", "compose.yaml"},
	},
	{
		Item:  stackItem{Name: "K8s", Category: "infra", Color: "dodger_blue"},
		Dirs:  []string{"k8s", "kubernetes"},
		Files: []string{"skaffold.yaml"},
	},
	{
		Item:    stackItem{Name: "Turborepo", Category: "infra", Color: "crimson"},
		Files:   []string{"turbo.json"},
		PkgJSON: []string{"turbo"},
	},
	{
		Item:  stackItem{Name: "Nx", Category: "infra", Color: "dodger_blue"},
		Files: []string{"nx.json"},
	},

	// === Runtimes (least specific, detected last) ===
	{
		Item:  stackItem{Name: "Go", Category: "runtime", Color: "turquoise"},
		Files: []string{"go.mod"},
	},
	{
		Item:  stackItem{Name: "Rust", Category: "runtime", Color: "coral"},
		Files: []string{"Cargo.toml"},
	},
	{
		Item:  stackItem{Name: "Python", Category: "runtime", Color: "yellow"},
		Files: []string{"pyproject.toml", "requirements.txt", "Pipfile", "setup.py", "setup.cfg"},
	},
	{
		Item:  stackItem{Name: "Java", Category: "runtime", Color: "red"},
		Files: []string{"pom.xml", "build.gradle", "build.gradle.kts"},
	},
	{
		Item:  stackItem{Name: "Ruby", Category: "runtime", Color: "red"},
		Files: []string{"Gemfile"},
	},
	{
		Item:  stackItem{Name: "Elixir", Category: "runtime", Color: "dark_violet"},
		Files: []string{"mix.exs"},
	},
	{
		Item:  stackItem{Name: "Zig", Category: "runtime", Color: "orange"},
		Files: []string{"build.zig"},
	},
	{
		Item:       stackItem{Name: "Node", Category: "runtime", Color: "emerald"},
		HasPkgJSON: true,
	},
	{
		Item:  stackItem{Name: "Deno", Category: "runtime", Color: "white"},
		Files: []string{"deno.json", "deno.jsonc"},
	},
	{
		Item:  stackItem{Name: "Bun", Category: "runtime", Color: "peach"},
		Files: []string{"bun.lockb", "bunfig.toml"},
	},
}

// detectStack scans the project directory and returns detected technologies
func detectStack(dir string, cfg stackConfig) []stackItem {
	// Build hide set
	hideSet := make(map[string]bool)
	for _, h := range cfg.hide {
		hideSet[strings.ToLower(h)] = true
	}

	// Read package.json deps once (if it exists)
	pkgDeps := readPkgJSONDeps(dir)
	hasPkgJSON := pkgDeps != nil

	var detected []stackItem
	seen := make(map[string]bool) // prevent duplicates

	// Track detected categories to suppress less-specific matches
	// e.g. if Next.js is detected, we might skip showing plain React
	suppressions := make(map[string]bool)

	for _, rule := range allRules {
		name := rule.Item.Name
		if seen[name] || hideSet[strings.ToLower(name)] {
			continue
		}
		if suppressions[name] {
			continue
		}

		matched := false

		// Check files
		for _, f := range rule.Files {
			if fileExists(filepath.Join(dir, f)) {
				matched = true
				break
			}
		}

		// Check dirs
		if !matched {
			for _, d := range rule.Dirs {
				if dirExists(filepath.Join(dir, d)) {
					matched = true
					break
				}
			}
		}

		// Check package.json deps
		if !matched && len(rule.PkgJSON) > 0 && pkgDeps != nil {
			for _, pkg := range rule.PkgJSON {
				if pkgDeps[pkg] {
					matched = true
					break
				}
			}
		}

		// Check HasPkgJSON
		if !matched && rule.HasPkgJSON && hasPkgJSON {
			matched = true
		}

		if matched {
			detected = append(detected, rule.Item)
			seen[name] = true

			// Apply suppressions
			switch name {
			case "Next.js", "Remix":
				suppressions["React"] = true
			case "Nuxt":
				suppressions["Vue"] = true
			case "SvelteKit":
				suppressions["Svelte"] = true
			case "Rails":
				suppressions["Ruby"] = true // Rails implies Ruby
			case "Django", "Flask":
				suppressions["Python"] = true
			case "Laravel":
				suppressions["PHP"] = true
			}
		}

		if len(detected) >= cfg.maxItems {
			break
		}
	}

	return detected
}

// readPkgJSONDeps reads package.json and returns a set of all dependency names
func readPkgJSONDeps(dir string) map[string]bool {
	data, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		return nil
	}

	var pkg struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil
	}

	deps := make(map[string]bool)
	for k := range pkg.Dependencies {
		deps[k] = true
	}
	for k := range pkg.DevDependencies {
		deps[k] = true
	}
	return deps
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// formatStack renders detected stack items with colors
func formatStack(input plugin.Input, items []stackItem) string {
	reset := input.Colors["reset"]
	sep := input.Colors["dim"] + "/" + reset

	var parts []string
	for _, item := range items {
		color := input.Colors[item.Color]
		if color == "" {
			color = input.Colors["gray"]
		}
		parts = append(parts, color+item.Name+reset)
	}

	return strings.Join(parts, sep)
}
