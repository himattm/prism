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
	"unicode/utf8"

	"github.com/himattm/prism/internal/cache"
	"github.com/himattm/prism/internal/plugin"
)

// VercelPlugin displays the latest Vercel deployment status
type VercelPlugin struct {
	cache *cache.Cache
}

// vercelProject represents the .vercel/project.json file
type vercelProject struct {
	ProjectID string `json:"projectId"`
	OrgID     string `json:"orgId"`
}

// vercelDeployment represents a single deployment from the Vercel API
type vercelDeployment struct {
	UID   string `json:"uid"`
	Name  string `json:"name"`
	State string `json:"state"` // BUILDING, READY, ERROR, QUEUED, CANCELED
	URL   string `json:"url"`
}

// vercelDeploymentsResponse is the response from /v6/deployments
type vercelDeploymentsResponse struct {
	Deployments []vercelDeployment `json:"deployments"`
}

// vercelConfig holds plugin configuration
type vercelConfig struct {
	ShowURL      bool
	MaxURLLength int
	ShowTeam     bool
}

func (p *VercelPlugin) Name() string {
	return "vercel"
}

func (p *VercelPlugin) SetCache(c *cache.Cache) {
	p.cache = c
}

// OnHook invalidates Vercel deployment cache when Claude becomes idle
func (p *VercelPlugin) OnHook(ctx context.Context, hookType HookType, hookCtx HookContext) (string, error) {
	if hookType == HookIdle && p.cache != nil {
		p.cache.DeleteByPrefix("vercel:deploy:")
	}
	return "", nil
}

func (p *VercelPlugin) Execute(ctx context.Context, input plugin.Input) (string, error) {
	cfg := parseVercelConfig(input.Config)

	cacheKey := "vercel:deploy:" + input.Prism.ProjectDir

	// Check cache first
	if p.cache != nil {
		if cached, ok := p.cache.Get(cacheKey); ok {
			return cached, nil
		}
	}

	// Check if vercel CLI is installed
	vercelPath, err := exec.LookPath("vercel")
	if err != nil {
		return "", nil
	}

	// Read .vercel/project.json from project directory
	project, err := readVercelProject(input.Prism.ProjectDir)
	if err != nil {
		return "", nil // Not a Vercel project
	}

	// Get latest deployment
	deployment, err := getLatestVercelDeployment(ctx, vercelPath, project.ProjectID)
	if err != nil || deployment == nil {
		return "", nil
	}

	// Get team name if configured
	var teamName string
	if cfg.ShowTeam {
		teamName = p.getVercelTeam(ctx, vercelPath)
	}

	// Format output
	output := formatVercelOutput(input, deployment, cfg, teamName)

	// Cache with state-dependent TTL
	if p.cache != nil {
		ttl := cache.VercelDeployTTL
		if deployment.State == "BUILDING" || deployment.State == "QUEUED" {
			ttl = cache.VercelBuildTTL
		}
		p.cache.Set(cacheKey, output, ttl)
	}

	return output, nil
}

func readVercelProject(projectDir string) (*vercelProject, error) {
	path := filepath.Join(projectDir, ".vercel", "project.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var project vercelProject
	if err := json.Unmarshal(data, &project); err != nil {
		return nil, err
	}

	if project.ProjectID == "" {
		return nil, os.ErrNotExist
	}

	return &project, nil
}

func getLatestVercelDeployment(ctx context.Context, vercelPath, projectID string) (*vercelDeployment, error) {
	cmd := exec.CommandContext(ctx, vercelPath, "api", "/v6/deployments?limit=1&projectId="+projectID)
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
	}

	var response vercelDeploymentsResponse
	if err := json.Unmarshal(out.Bytes(), &response); err != nil {
		return nil, err
	}

	if len(response.Deployments) == 0 {
		return nil, nil
	}

	return &response.Deployments[0], nil
}

func (p *VercelPlugin) getVercelTeam(ctx context.Context, vercelPath string) string {
	teamCacheKey := "vercel:team"

	if p.cache != nil {
		if cached, ok := p.cache.Get(teamCacheKey); ok {
			return cached
		}
	}

	cmd := exec.CommandContext(ctx, vercelPath, "whoami")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &bytes.Buffer{} // Capture stderr for debuggability

	if err := cmd.Run(); err != nil {
		return ""
	}

	team := strings.TrimSpace(out.String())

	if p.cache != nil {
		p.cache.Set(teamCacheKey, team, cache.VercelTeamTTL)
	}

	return team
}

func parseVercelConfig(config map[string]any) vercelConfig {
	cfg := vercelConfig{
		ShowURL:      false,
		MaxURLLength: 30,
		ShowTeam:     false,
	}

	vercelCfg, ok := config["vercel"].(map[string]any)
	if !ok {
		return cfg
	}

	if v, ok := vercelCfg["show_url"].(bool); ok {
		cfg.ShowURL = v
	}
	if v, ok := vercelCfg["max_url_length"].(float64); ok {
		cfg.MaxURLLength = int(v)
	}
	if v, ok := vercelCfg["show_team"].(bool); ok {
		cfg.ShowTeam = v
	}

	return cfg
}

func formatVercelOutput(input plugin.Input, deploy *vercelDeployment, cfg vercelConfig, teamName string) string {
	var result strings.Builder

	reset := input.Colors["reset"]

	// Choose color based on deployment state
	var color string
	var stateText string
	switch deploy.State {
	case "READY":
		color = input.Colors["emerald"]
		stateText = "ready"
	case "BUILDING":
		color = input.Colors["yellow"]
		stateText = "building"
	case "ERROR":
		color = input.Colors["red"]
		stateText = "error"
	case "QUEUED":
		color = input.Colors["gray"]
		stateText = "queued"
	case "CANCELED":
		color = input.Colors["gray"]
		stateText = "canceled"
	default:
		color = input.Colors["gray"]
		stateText = strings.ToLower(deploy.State)
	}

	result.WriteString(color)
	result.WriteString("▲ ")

	// Optional team prefix
	if cfg.ShowTeam && teamName != "" {
		result.WriteString(teamName)
		result.WriteString(": ")
	}

	result.WriteString(stateText)

	// Optional URL
	if cfg.ShowURL && deploy.URL != "" {
		url := deploy.URL
		if cfg.MaxURLLength > 0 && utf8.RuneCountInString(url) > cfg.MaxURLLength {
			runes := []rune(url)
			url = string(runes[:cfg.MaxURLLength-1]) + "…"
		}
		result.WriteString(" ")
		result.WriteString(url)
	}

	result.WriteString(reset)

	return result.String()
}
