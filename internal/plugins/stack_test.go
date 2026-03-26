package plugins

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectStack_GoProject(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test"), 0644)

	cfg := stackConfig{maxItems: 4}
	items := detectStack(dir, cfg)

	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d: %v", len(items), items)
	}
	if items[0].Name != "Go" {
		t.Errorf("expected Go, got %s", items[0].Name)
	}
}

func TestDetectStack_NextjsProject(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "next.config.js"), []byte("module.exports = {}"), 0644)
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{
		"dependencies": {"react": "^18", "next": "^14"},
		"devDependencies": {}
	}`), 0644)

	cfg := stackConfig{maxItems: 10}
	items := detectStack(dir, cfg)

	names := itemNames(items)
	if !containsItem(names, "Next.js") {
		t.Errorf("expected Next.js, got %v", names)
	}
	// React should be suppressed by Next.js
	if containsItem(names, "React") {
		t.Errorf("React should be suppressed when Next.js is detected, got %v", names)
	}
	// Node should still be detected
	if !containsItem(names, "Node") {
		t.Errorf("expected Node (from package.json), got %v", names)
	}
}

func TestDetectStack_SupabaseProject(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "supabase"), 0755)
	os.WriteFile(filepath.Join(dir, "supabase", "config.toml"), []byte("[project]"), 0644)

	cfg := stackConfig{maxItems: 4}
	items := detectStack(dir, cfg)

	names := itemNames(items)
	if !containsItem(names, "Supabase") {
		t.Errorf("expected Supabase, got %v", names)
	}
}

func TestDetectStack_VercelProject(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "vercel.json"), []byte("{}"), 0644)

	cfg := stackConfig{maxItems: 4}
	items := detectStack(dir, cfg)

	names := itemNames(items)
	if !containsItem(names, "Vercel") {
		t.Errorf("expected Vercel, got %v", names)
	}
}

func TestDetectStack_DockerProject(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte("FROM alpine"), 0644)
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test"), 0644)

	cfg := stackConfig{maxItems: 4}
	items := detectStack(dir, cfg)

	names := itemNames(items)
	if !containsItem(names, "Docker") {
		t.Errorf("expected Docker, got %v", names)
	}
	if !containsItem(names, "Go") {
		t.Errorf("expected Go, got %v", names)
	}
}

func TestDetectStack_FullStack(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "next.config.ts"), []byte(""), 0644)
	os.WriteFile(filepath.Join(dir, "vercel.json"), []byte("{}"), 0644)
	os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte("FROM node"), 0644)
	os.MkdirAll(filepath.Join(dir, "supabase"), 0755)
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{
		"dependencies": {"next": "^14", "react": "^18", "@supabase/supabase-js": "^2"},
		"devDependencies": {"prisma": "^5"}
	}`), 0644)
	os.MkdirAll(filepath.Join(dir, "prisma"), 0755)
	os.WriteFile(filepath.Join(dir, "prisma", "schema.prisma"), []byte(""), 0644)

	cfg := stackConfig{maxItems: 10}
	items := detectStack(dir, cfg)

	names := itemNames(items)
	if !containsItem(names, "Next.js") {
		t.Errorf("expected Next.js, got %v", names)
	}
	if !containsItem(names, "Supabase") {
		t.Errorf("expected Supabase, got %v", names)
	}
	if !containsItem(names, "Vercel") {
		t.Errorf("expected Vercel, got %v", names)
	}
	if !containsItem(names, "Prisma") {
		t.Errorf("expected Prisma, got %v", names)
	}
	if !containsItem(names, "Docker") {
		t.Errorf("expected Docker, got %v", names)
	}
	if containsItem(names, "React") {
		t.Errorf("React should be suppressed by Next.js, got %v", names)
	}
}

func TestDetectStack_MaxItems(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "next.config.js"), []byte(""), 0644)
	os.WriteFile(filepath.Join(dir, "vercel.json"), []byte("{}"), 0644)
	os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte("FROM node"), 0644)
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{
		"dependencies": {"next": "^14", "react": "^18"},
		"devDependencies": {}
	}`), 0644)

	cfg := stackConfig{maxItems: 2}
	items := detectStack(dir, cfg)

	if len(items) > 2 {
		t.Errorf("expected at most 2 items, got %d: %v", len(items), itemNames(items))
	}
}

func TestDetectStack_HideItems(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test"), 0644)
	os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte("FROM alpine"), 0644)

	cfg := stackConfig{maxItems: 4, hide: []string{"docker"}}
	items := detectStack(dir, cfg)

	names := itemNames(items)
	if containsItem(names, "Docker") {
		t.Errorf("Docker should be hidden, got %v", names)
	}
	if !containsItem(names, "Go") {
		t.Errorf("expected Go, got %v", names)
	}
}

func TestDetectStack_Empty(t *testing.T) {
	dir := t.TempDir()

	cfg := stackConfig{maxItems: 4}
	items := detectStack(dir, cfg)

	if len(items) != 0 {
		t.Errorf("expected 0 items for empty dir, got %d: %v", len(items), itemNames(items))
	}
}

func TestDetectStack_PkgJSONDeps(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{
		"dependencies": {"vue": "^3"},
		"devDependencies": {"vite": "^5"}
	}`), 0644)

	cfg := stackConfig{maxItems: 10}
	items := detectStack(dir, cfg)

	names := itemNames(items)
	if !containsItem(names, "Vue") {
		t.Errorf("expected Vue from deps, got %v", names)
	}
	if !containsItem(names, "Vite") {
		t.Errorf("expected Vite from devDeps, got %v", names)
	}
}

func TestDetectStack_NuxtSuppressesVue(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "nuxt.config.ts"), []byte(""), 0644)
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{
		"dependencies": {"nuxt": "^3", "vue": "^3"},
		"devDependencies": {}
	}`), 0644)

	cfg := stackConfig{maxItems: 10}
	items := detectStack(dir, cfg)

	names := itemNames(items)
	if !containsItem(names, "Nuxt") {
		t.Errorf("expected Nuxt, got %v", names)
	}
	if containsItem(names, "Vue") {
		t.Errorf("Vue should be suppressed by Nuxt, got %v", names)
	}
}

func TestDetectStack_PythonProject(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte("[project]"), 0644)

	cfg := stackConfig{maxItems: 4}
	items := detectStack(dir, cfg)

	names := itemNames(items)
	if !containsItem(names, "Python") {
		t.Errorf("expected Python, got %v", names)
	}
}

func TestDetectStack_DjangoSuppressesPython(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "config"), 0755)
	os.WriteFile(filepath.Join(dir, "config/urls.py"), []byte(""), 0644)
	os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte("[project]"), 0644)

	cfg := stackConfig{maxItems: 10}
	items := detectStack(dir, cfg)

	names := itemNames(items)
	if !containsItem(names, "Django") {
		t.Errorf("expected Django, got %v", names)
	}
	if containsItem(names, "Python") {
		t.Errorf("Python should be suppressed by Django, got %v", names)
	}
}

func TestDetectStack_RustProject(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "Cargo.toml"), []byte("[package]"), 0644)

	cfg := stackConfig{maxItems: 4}
	items := detectStack(dir, cfg)

	if len(items) != 1 || items[0].Name != "Rust" {
		t.Errorf("expected [Rust], got %v", itemNames(items))
	}
}

func TestDetectStack_TerraformProject(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "main.tf"), []byte(""), 0644)

	cfg := stackConfig{maxItems: 4}
	items := detectStack(dir, cfg)

	names := itemNames(items)
	if !containsItem(names, "Terraform") {
		t.Errorf("expected Terraform, got %v", names)
	}
}

func TestFormatStack(t *testing.T) {
	input := testInput("test-format-stack")
	input.Colors["turquoise"] = "\033[38;5;45m"
	input.Colors["dim"] = "\033[2m"

	items := []stackItem{
		{Name: "Go", Color: "turquoise"},
		{Name: "Docker", Color: "dodger_blue"},
	}

	output := formatStack(input, items)
	if output == "" {
		t.Error("expected non-empty output")
	}
	// Should contain both names
	if !containsSubstring(output, "Go") {
		t.Error("output should contain 'Go'")
	}
	if !containsSubstring(output, "Docker") {
		t.Error("output should contain 'Docker'")
	}
}

func TestParseStackConfig(t *testing.T) {
	cfg := parseStackConfig(map[string]any{
		"stack": map[string]any{
			"max_items": float64(3),
			"hide":      []any{"docker", "node"},
		},
	})

	if cfg.maxItems != 3 {
		t.Errorf("expected maxItems 3, got %d", cfg.maxItems)
	}
	if len(cfg.hide) != 2 {
		t.Errorf("expected 2 hide items, got %d", len(cfg.hide))
	}
}

func TestReadPkgJSONDeps(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{
		"dependencies": {"react": "^18", "next": "^14"},
		"devDependencies": {"typescript": "^5"}
	}`), 0644)

	deps := readPkgJSONDeps(dir)
	if deps == nil {
		t.Fatal("expected non-nil deps")
	}
	if !deps["react"] {
		t.Error("expected react in deps")
	}
	if !deps["next"] {
		t.Error("expected next in deps")
	}
	if !deps["typescript"] {
		t.Error("expected typescript in devDeps")
	}
	if deps["nonexistent"] {
		t.Error("unexpected dep found")
	}
}

func TestReadPkgJSONDeps_Missing(t *testing.T) {
	dir := t.TempDir()
	deps := readPkgJSONDeps(dir)
	if deps != nil {
		t.Error("expected nil for missing package.json")
	}
}

// helpers

func itemNames(items []stackItem) []string {
	names := make([]string, len(items))
	for i, item := range items {
		names[i] = item.Name
	}
	return names
}

func containsItem(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}
