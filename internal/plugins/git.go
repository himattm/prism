package plugins

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/himattm/prism/internal/cache"
	"github.com/himattm/prism/internal/git"
	"github.com/himattm/prism/internal/plugin"
)

// GitPlugin shows git branch and status
type GitPlugin struct {
	cache *cache.Cache
}

func (p *GitPlugin) Name() string {
	return "git"
}

func (p *GitPlugin) SetCache(c *cache.Cache) {
	p.cache = c
}

// OnHook invalidates git cache when Claude becomes idle (fresh data on next render)
func (p *GitPlugin) OnHook(ctx context.Context, hookType HookType, hookCtx HookContext) (string, error) {
	if hookType == HookIdle && p.cache != nil {
		p.cache.DeleteByPrefix("git:")
	}
	return "", nil
}

func (p *GitPlugin) Execute(ctx context.Context, input plugin.Input) (string, error) {
	gitDir := p.getEffectiveGitDir(ctx, input)
	if gitDir == "" {
		return "", nil
	}

	cacheKey := fmt.Sprintf("git:%s", gitDir)

	// Check cache first
	if p.cache != nil {
		if cached, ok := p.cache.Get(cacheKey); ok {
			return cached, nil
		}
	}

	// Get branch name
	branch := getGitBranch(ctx, gitDir)
	if branch == "" {
		return "", nil
	}

	// Get dirty status
	dirty := getGitDirty(ctx, gitDir)

	// Get upstream status
	behind, ahead := getUpstreamStatus(ctx, gitDir)

	// Format output
	yellow := input.Colors["yellow"]
	reset := input.Colors["reset"]

	var result strings.Builder
	result.WriteString(yellow)
	result.WriteString(branch)

	if dirty != "" {
		result.WriteString(dirty)
	}

	if behind > 0 {
		result.WriteString(fmt.Sprintf(" ⇣%d", behind))
	}
	if ahead > 0 {
		result.WriteString(fmt.Sprintf(" ⇡%d", ahead))
	}

	result.WriteString(reset)
	output := result.String()

	// Cache for 2 seconds
	if p.cache != nil {
		p.cache.Set(cacheKey, output, cache.GitTTL)
	}

	return output, nil
}

// getEffectiveGitDir returns the best directory to use for git operations.
// Tries ProjectDir first (fast path), falls back to finding git root from CurrentDir.
func (p *GitPlugin) getEffectiveGitDir(ctx context.Context, input plugin.Input) string {
	return git.EffectiveDir(ctx, input.Prism.ProjectDir, input.Prism.CurrentDir, p.cache)
}

func getGitBranch(ctx context.Context, dir string) string {
	// Try to get current branch
	cmd := exec.CommandContext(ctx, "git", "--no-optional-locks", "branch", "--show-current")
	cmd.Dir = dir
	var out bytes.Buffer
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		return ""
	}

	branch := strings.TrimSpace(out.String())
	if branch != "" {
		return branch
	}

	// Detached HEAD - get short commit
	cmd = exec.CommandContext(ctx, "git", "--no-optional-locks", "rev-parse", "--short", "HEAD")
	cmd.Dir = dir
	out.Reset()
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		return ""
	}

	return strings.TrimSpace(out.String())
}

func getGitDirty(ctx context.Context, dir string) string {
	cmd := exec.CommandContext(ctx, "git", "--no-optional-locks", "status", "--porcelain")
	cmd.Dir = dir
	var out bytes.Buffer
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		return ""
	}

	output := out.String()
	if output == "" {
		return ""
	}

	var dirty strings.Builder
	hasStaged := false
	hasUnstaged := false
	hasUntracked := false

	lines := strings.Split(strings.TrimSpace(output), "\n")
	for _, line := range lines {
		if len(line) < 2 {
			continue
		}

		index := line[0]
		worktree := line[1]

		// Check for staged changes (index not empty and not '?')
		if index != ' ' && index != '?' {
			hasStaged = true
		}

		// Check for unstaged changes (worktree modified)
		if worktree != ' ' && worktree != '?' {
			hasUnstaged = true
		}

		// Check for untracked files
		if index == '?' {
			hasUntracked = true
		}
	}

	if hasStaged {
		dirty.WriteString("*")
	}
	if hasUnstaged {
		dirty.WriteString("*")
	}
	if hasUntracked {
		dirty.WriteString("+")
	}

	return dirty.String()
}

func getUpstreamStatus(ctx context.Context, dir string) (behind, ahead int) {
	// Single command to get both ahead and behind counts
	// Output format: "<ahead>\t<behind>\n"
	cmd := exec.CommandContext(ctx, "git", "--no-optional-locks", "rev-list", "--left-right", "--count", "@{upstream}...HEAD")
	cmd.Dir = dir
	var out bytes.Buffer
	cmd.Stdout = &out

	if cmd.Run() != nil {
		return 0, 0
	}

	parts := strings.Fields(strings.TrimSpace(out.String()))
	if len(parts) != 2 {
		return 0, 0
	}

	behind, _ = strconv.Atoi(parts[0])
	ahead, _ = strconv.Atoi(parts[1])
	return behind, ahead
}
