package plugins

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/himattm/prism/internal/cache"
	"github.com/himattm/prism/internal/plugin"
)

// setupGitRepo initializes a git repo in the given directory with a single commit.
func setupGitRepo(t *testing.T, dir string) {
	t.Helper()

	commands := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
		{"git", "checkout", "-B", "main"},
		{"git", "commit", "--allow-empty", "-m", "initial"},
	}

	for _, args := range commands {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("setup command %v failed: %v\n%s", args, err, out)
		}
	}
}

func TestGitPlugin_Execute_NonGitProjectDir_GitCurrentDir(t *testing.T) {
	// Parent directory is NOT a git repo
	parentDir := t.TempDir()

	// Create a subdirectory that IS a git repo
	gitRepoDir := filepath.Join(parentDir, "repo")
	if err := os.Mkdir(gitRepoDir, 0755); err != nil {
		t.Fatal(err)
	}
	setupGitRepo(t, gitRepoDir)

	p := &GitPlugin{}
	p.SetCache(cache.New())

	ctx := context.Background()
	input := plugin.Input{
		Prism: plugin.PrismContext{
			ProjectDir: parentDir,
			CurrentDir: gitRepoDir,
		},
		Colors: map[string]string{
			"yellow": "",
			"reset":  "",
		},
	}

	result, err := p.Execute(ctx, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should fall back to CurrentDir and find the branch
	if result == "" {
		t.Fatal("expected non-empty result when CurrentDir is a git repo, got empty")
	}

	if !strings.Contains(result, "main") && !strings.Contains(result, "master") {
		t.Errorf("expected result to contain branch name (main or master), got %q", result)
	}
}

func TestGitPlugin_Execute_NonGitProjectDir_NonGitCurrentDir(t *testing.T) {
	// Neither directory is a git repo
	tempDir := t.TempDir()

	p := &GitPlugin{}
	p.SetCache(cache.New())

	ctx := context.Background()
	input := plugin.Input{
		Prism: plugin.PrismContext{
			ProjectDir: tempDir,
			CurrentDir: tempDir,
		},
		Colors: map[string]string{
			"yellow": "",
			"reset":  "",
		},
	}

	result, err := p.Execute(ctx, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != "" {
		t.Errorf("expected empty result when neither dir is a git repo, got %q", result)
	}
}

func TestGitPlugin_Execute_GitProjectDir(t *testing.T) {
	// ProjectDir itself is a git repo
	gitRepoDir := t.TempDir()
	setupGitRepo(t, gitRepoDir)

	p := &GitPlugin{}
	p.SetCache(cache.New())

	ctx := context.Background()
	input := plugin.Input{
		Prism: plugin.PrismContext{
			ProjectDir: gitRepoDir,
			CurrentDir: gitRepoDir,
		},
		Colors: map[string]string{
			"yellow": "",
			"reset":  "",
		},
	}

	result, err := p.Execute(ctx, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == "" {
		t.Fatal("expected non-empty result when ProjectDir is a git repo, got empty")
	}

	if !strings.Contains(result, "main") && !strings.Contains(result, "master") {
		t.Errorf("expected result to contain branch name (main or master), got %q", result)
	}
}

func TestGitPlugin_Execute_SubdirOfGitRepo(t *testing.T) {
	// Parent is NOT a git repo, but CurrentDir is a subdirectory of one
	parentDir := t.TempDir()

	gitRepoDir := filepath.Join(parentDir, "repo")
	if err := os.Mkdir(gitRepoDir, 0755); err != nil {
		t.Fatal(err)
	}
	setupGitRepo(t, gitRepoDir)

	subDir := filepath.Join(gitRepoDir, "subdir")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatal(err)
	}

	p := &GitPlugin{}
	p.SetCache(cache.New())

	ctx := context.Background()
	input := plugin.Input{
		Prism: plugin.PrismContext{
			ProjectDir: parentDir,
			CurrentDir: subDir,
		},
		Colors: map[string]string{
			"yellow": "",
			"reset":  "",
		},
	}

	result, err := p.Execute(ctx, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should find the git repo via rev-parse from the subdirectory
	if result == "" {
		t.Fatal("expected non-empty result when CurrentDir is inside a git repo, got empty")
	}

	if !strings.Contains(result, "main") && !strings.Contains(result, "master") {
		t.Errorf("expected result to contain branch name (main or master), got %q", result)
	}
}

func TestGitPlugin_Execute_WorktreeBranch(t *testing.T) {
	// Setup: main repo on "main", worktree on "feature-branch".
	// ProjectDir points to the main repo, CurrentDir points to the worktree.
	// The git plugin should show the worktree's branch, not the main repo's.
	mainRepoDir := t.TempDir()
	setupGitRepo(t, mainRepoDir)

	// Create a branch for the worktree
	cmd := exec.Command("git", "branch", "feature-branch")
	cmd.Dir = mainRepoDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git branch failed: %v\n%s", err, out)
	}

	// Create the worktree
	worktreeDir := filepath.Join(t.TempDir(), "worktree")
	cmd = exec.Command("git", "worktree", "add", worktreeDir, "feature-branch")
	cmd.Dir = mainRepoDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git worktree add failed: %v\n%s", err, out)
	}
	t.Cleanup(func() {
		cmd := exec.Command("git", "worktree", "remove", worktreeDir)
		cmd.Dir = mainRepoDir
		cmd.Run()
	})

	p := &GitPlugin{}
	p.SetCache(cache.New())

	ctx := context.Background()
	input := plugin.Input{
		Prism: plugin.PrismContext{
			ProjectDir: mainRepoDir,
			CurrentDir: worktreeDir,
		},
		Colors: map[string]string{
			"yellow": "",
			"reset":  "",
		},
	}

	result, err := p.Execute(ctx, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result, "feature-branch") {
		t.Errorf("expected worktree branch 'feature-branch', got %q", result)
	}

	if strings.Contains(result, "main") {
		t.Errorf("should NOT show main repo branch 'main', got %q", result)
	}
}
