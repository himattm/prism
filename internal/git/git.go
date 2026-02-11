package git

import (
	"bytes"
	"context"
	"os/exec"
	"strings"
	"time"

	"github.com/himattm/prism/internal/cache"
)

const gitTimeout = 500 * time.Millisecond

// IsRepo checks if a directory is inside a git repository.
func IsRepo(ctx context.Context, dir string) bool {
	ctx, cancel := context.WithTimeout(ctx, gitTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "--no-optional-locks", "rev-parse", "--git-dir")
	cmd.Dir = dir
	return cmd.Run() == nil
}

// FindRoot runs git rev-parse --show-toplevel from the given directory
// to find the git root. Returns empty string if not in a git repo.
func FindRoot(ctx context.Context, dir string) string {
	ctx, cancel := context.WithTimeout(ctx, gitTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "--no-optional-locks", "rev-parse", "--show-toplevel")
	cmd.Dir = dir
	var out bytes.Buffer
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		return ""
	}

	return strings.TrimSpace(out.String())
}

// EffectiveDir returns the best directory for git operations.
// Tries projectDir first, falls back to finding git root from currentDir.
// Results are cached using the provided cache (if non-nil).
func EffectiveDir(ctx context.Context, projectDir, currentDir string, c *cache.Cache) string {
	// Fast path: ProjectDir is set and is a git repo
	if projectDir != "" {
		if IsRepo(ctx, projectDir) {
			return projectDir
		}
	}

	// Fallback: find git root from CurrentDir
	if currentDir == "" {
		return ""
	}

	cacheKey := "git:effective:" + currentDir
	if c != nil {
		if cached, ok := c.Get(cacheKey); ok {
			return cached
		}
	}

	gitRoot := FindRoot(ctx, currentDir)

	if c != nil {
		c.Set(cacheKey, gitRoot, cache.GitTTL)
	}

	return gitRoot
}
