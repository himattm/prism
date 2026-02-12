package git

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
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
// If currentDir is inside a different worktree than projectDir, returns the
// worktree root so that git commands reflect the worktree's branch.
// Otherwise tries projectDir first, then falls back to finding git root from currentDir.
// Results are cached using the provided cache (if non-nil).
func EffectiveDir(ctx context.Context, projectDir, currentDir string, c *cache.Cache) string {
	// Check if currentDir is in a worktree that differs from projectDir.
	// This handles the case where Claude Code was launched from the main repo
	// but the user cd'd into a worktree directory.
	if currentDir != "" && projectDir != "" {
		cacheKey := "git:effective:wt:" + currentDir
		if c != nil {
			if cached, ok := c.Get(cacheKey); ok {
				if cached != "" {
					return cached
				}
				// cached == "" means not a worktree, fall through
			}
		}

		if wtRoot, ok := findWorktreeRoot(currentDir); ok {
			// currentDir is in a worktree — use it if it differs from projectDir
			resolved := resolveSymlinks(projectDir)
			if resolved != wtRoot {
				if c != nil {
					c.Set(cacheKey, wtRoot, cache.GitTTL)
				}
				return wtRoot
			}
		}

		if c != nil {
			c.Set(cacheKey, "", cache.GitTTL)
		}
	}

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

// findWorktreeRoot walks up from dir looking for a .git file (not directory),
// which indicates the directory is a git worktree root.
func findWorktreeRoot(dir string) (string, bool) {
	current := dir
	for {
		gitPath := filepath.Join(current, ".git")
		info, err := os.Stat(gitPath)
		if err == nil {
			if !info.IsDir() {
				return current, true // .git is a file → worktree
			}
			return "", false // .git is a directory → main repo, stop
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", false
		}
		current = parent
	}
}

// resolveSymlinks resolves symlinks in a path for reliable comparison.
func resolveSymlinks(path string) string {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return path
	}
	return resolved
}
