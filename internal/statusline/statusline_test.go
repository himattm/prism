package statusline

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/himattm/prism/internal/burnrate"
	"github.com/himattm/prism/internal/colors"
	"github.com/himattm/prism/internal/config"
	"github.com/himattm/prism/internal/tokens"
)

// TestRenderLinesChanged_NeverUsesClaudeStats verifies that linesChanged
// ALWAYS uses git diff stats and NEVER falls back to Claude's session stats.
// This is critical - we've had bugs where Claude's stats were shown instead.
func TestRenderLinesChanged_NeverUsesClaudeStats(t *testing.T) {
	// Create a temp git repo for testing
	tmpDir := setupTestGitRepo(t)
	defer os.RemoveAll(tmpDir)

	// Create a StatusLine with Claude's stats set to non-zero values
	// If the implementation incorrectly uses these, the test will fail
	sl := &StatusLine{
		input: Input{
			Workspace: WorkspaceInfo{
				ProjectDir: tmpDir,
				CurrentDir: tmpDir,
			},
			Cost: CostInfo{
				TotalLinesAdded:   999, // These should NEVER appear in output
				TotalLinesRemoved: 888, // These should NEVER appear in output
			},
		},
		isIdle: false, // Even when not idle, should use git stats
	}

	result := sl.renderLinesChanged()

	// Should show +0 -0 (clean repo), NOT +999 -888 (Claude's stats)
	if strings.Contains(result, "999") || strings.Contains(result, "888") {
		t.Errorf("renderLinesChanged used Claude's stats instead of git stats: %s", result)
	}
	if !strings.Contains(result, "+0") || !strings.Contains(result, "-0") {
		t.Errorf("expected +0 -0 for clean repo, got: %s", result)
	}
}

// TestRenderLinesChanged_WithUncommittedChanges verifies git stats are shown
func TestRenderLinesChanged_WithUncommittedChanges(t *testing.T) {
	tmpDir := setupTestGitRepo(t)
	defer os.RemoveAll(tmpDir)

	// Create and stage a new file (git diff HEAD shows staged changes)
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("line1\nline2\nline3\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Stage the file so it shows in git diff HEAD
	cmd := exec.Command("git", "add", "test.txt")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to git add: %v", err)
	}

	sl := &StatusLine{
		input: Input{
			Workspace: WorkspaceInfo{
				ProjectDir: tmpDir,
			},
			Cost: CostInfo{
				TotalLinesAdded:   0, // Even if Claude says 0
				TotalLinesRemoved: 0, // Git should show the real changes
			},
		},
	}

	result := sl.renderLinesChanged()

	// Should show +3 -0 (3 lines added)
	if !strings.Contains(result, "+3") {
		t.Errorf("expected +3 for 3 added lines, got: %s", result)
	}
}

// TestRenderLinesChanged_IdleStateDoesNotAffectBehavior ensures idle state
// has no impact on which stats are used (always git)
func TestRenderLinesChanged_IdleStateDoesNotAffectBehavior(t *testing.T) {
	tmpDir := setupTestGitRepo(t)
	defer os.RemoveAll(tmpDir)

	// Create an uncommitted change
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("hello\n"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	claudeStats := CostInfo{
		TotalLinesAdded:   100,
		TotalLinesRemoved: 50,
	}

	// Test with isIdle = true
	slIdle := &StatusLine{
		input:  Input{Workspace: WorkspaceInfo{ProjectDir: tmpDir}, Cost: claudeStats},
		isIdle: true,
	}
	resultIdle := slIdle.renderLinesChanged()

	// Test with isIdle = false
	slBusy := &StatusLine{
		input:  Input{Workspace: WorkspaceInfo{ProjectDir: tmpDir}, Cost: claudeStats},
		isIdle: false,
	}
	resultBusy := slBusy.renderLinesChanged()

	// Both should show the same git-based stats (+1 -0), not Claude's stats
	if resultIdle != resultBusy {
		t.Errorf("idle state affected linesChanged output:\nidle=%s\nbusy=%s", resultIdle, resultBusy)
	}
	if strings.Contains(resultIdle, "100") || strings.Contains(resultIdle, "50") {
		t.Errorf("Claude's stats were used instead of git stats: %s", resultIdle)
	}
}

// TestGetGitDiffStats_EmptyDir returns 0,0 for empty project dir
func TestGetGitDiffStats_EmptyDir(t *testing.T) {
	added, removed := getGitDiffStats("")
	if added != 0 || removed != 0 {
		t.Errorf("expected 0,0 for empty dir, got %d,%d", added, removed)
	}
}

// TestGetGitDiffStats_NotGitRepo returns 0,0 for non-git directory
func TestGetGitDiffStats_NotGitRepo(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "prism-test-nogit-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	added, removed := getGitDiffStats(tmpDir)
	if added != 0 || removed != 0 {
		t.Errorf("expected 0,0 for non-git dir, got %d,%d", added, removed)
	}
}

// TestGetGitDiffStats_CleanRepo returns 0,0 for clean working tree
func TestGetGitDiffStats_CleanRepo(t *testing.T) {
	tmpDir := setupTestGitRepo(t)
	defer os.RemoveAll(tmpDir)

	added, removed := getGitDiffStats(tmpDir)
	if added != 0 || removed != 0 {
		t.Errorf("expected 0,0 for clean repo, got %d,%d", added, removed)
	}
}

// TestGetGitDiffStats_WithChanges correctly counts added/removed lines
func TestGetGitDiffStats_WithChanges(t *testing.T) {
	tmpDir := setupTestGitRepo(t)
	defer os.RemoveAll(tmpDir)

	// Modify the existing file (adds lines, removes lines)
	readmeFile := filepath.Join(tmpDir, "README.md")
	if err := os.WriteFile(readmeFile, []byte("new content\nline 2\nline 3\n"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	added, removed := getGitDiffStats(tmpDir)

	// Original had 1 line ("# Test"), new has 3 lines
	// So we should see additions and the original line removed
	if added == 0 && removed == 0 {
		t.Errorf("expected non-zero changes after modifying file, got +%d -%d", added, removed)
	}
}

// TestGetGitDiffStats_NewUntrackedFile does not count untracked files
func TestGetGitDiffStats_NewUntrackedFile(t *testing.T) {
	tmpDir := setupTestGitRepo(t)
	defer os.RemoveAll(tmpDir)

	// Create a new untracked file (not staged)
	newFile := filepath.Join(tmpDir, "untracked.txt")
	if err := os.WriteFile(newFile, []byte("untracked content\n"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	added, removed := getGitDiffStats(tmpDir)

	// git diff HEAD doesn't show untracked files
	if added != 0 || removed != 0 {
		t.Errorf("untracked files should not affect diff stats, got +%d -%d", added, removed)
	}
}

// TestGetGitDiffStats_StagedChanges counts staged changes
func TestGetGitDiffStats_StagedChanges(t *testing.T) {
	tmpDir := setupTestGitRepo(t)
	defer os.RemoveAll(tmpDir)

	// Create and stage a new file
	newFile := filepath.Join(tmpDir, "staged.txt")
	if err := os.WriteFile(newFile, []byte("line1\nline2\n"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	cmd := exec.Command("git", "add", "staged.txt")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to git add: %v", err)
	}

	added, removed := getGitDiffStats(tmpDir)

	// git diff HEAD shows staged changes
	if added != 2 {
		t.Errorf("expected 2 added lines for staged file, got +%d -%d", added, removed)
	}
}

// TestRenderLinesChanged_OutputFormat verifies the output format
func TestRenderLinesChanged_OutputFormat(t *testing.T) {
	tmpDir := setupTestGitRepo(t)
	defer os.RemoveAll(tmpDir)

	sl := &StatusLine{
		input: Input{Workspace: WorkspaceInfo{ProjectDir: tmpDir}},
	}

	result := sl.renderLinesChanged()

	// Should contain ANSI color codes and +/- format
	if !strings.Contains(result, "\033[32m+") { // Green for additions
		t.Errorf("missing green color for additions: %s", result)
	}
	if !strings.Contains(result, "\033[31m-") { // Red for removals
		t.Errorf("missing red color for removals: %s", result)
	}
}

// setupTestGitRepoAt initializes a git repository at the given directory.
// The directory must already exist. Returns the directory path.
func setupTestGitRepoAt(t *testing.T, dir string) string {
	t.Helper()

	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if err := cmd.Run(); err != nil {
			t.Fatalf("failed to run %v: %v", args, err)
		}
	}

	// Create initial commit
	readmeFile := filepath.Join(dir, "README.md")
	if err := os.WriteFile(readmeFile, []byte("# Test\n"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	cmd := exec.Command("git", "add", "README.md")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to git add: %v", err)
	}

	cmd = exec.Command("git", "commit", "-m", "Initial commit")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to create initial commit: %v", err)
	}

	return dir
}

// setupTestGitRepo creates a temporary git repository for testing
func setupTestGitRepo(t *testing.T) string {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "prism-test-git-*")
	if err != nil {
		t.Fatal(err)
	}

	return setupTestGitRepoAt(t, tmpDir)
}

// TestNew_CreatesStatusLine verifies the constructor works
func TestNew_CreatesStatusLine(t *testing.T) {
	input := Input{
		SessionID: "test-session",
		Model:     ModelInfo{DisplayName: "Test Model"},
	}
	cfg := config.Config{}

	sl := New(input, cfg)

	if sl == nil {
		t.Fatal("New returned nil")
	}
	if sl.input.SessionID != "test-session" {
		t.Errorf("session ID not set correctly")
	}
}

// TestRenderContextBar_NoBrackets verifies brackets were removed
func TestRenderContextBar_NoBrackets(t *testing.T) {
	result := renderContextBar(50, 50, false)

	if strings.Contains(result, "[") || strings.Contains(result, "]") {
		t.Errorf("context bar should not contain brackets: %s", result)
	}
	if !strings.Contains(result, "█") {
		t.Errorf("context bar should contain filled blocks: %s", result)
	}
	if !strings.Contains(result, "50%") {
		t.Errorf("context bar should contain percentage: %s", result)
	}
}

// TestRenderContextBar_Percentages verifies bar fills correctly at different percentages
func TestRenderContextBar_Percentages(t *testing.T) {
	tests := []struct {
		pct          int
		expectedFill int // number of █ characters
	}{
		{0, 0},
		{10, 1},
		{50, 5},
		{100, 10},
	}

	for _, tt := range tests {
		result := renderContextBar(tt.pct, tt.pct, false)
		fillCount := strings.Count(result, "█")
		if fillCount != tt.expectedFill {
			t.Errorf("at %d%%, expected %d filled blocks, got %d: %s",
				tt.pct, tt.expectedFill, fillCount, result)
		}
	}
}

// TestRenderContextBar_BufferZone verifies buffer zone rendering
func TestRenderContextBar_BufferZone(t *testing.T) {
	// With buffer enabled, should have ▒ characters at the end
	withBuffer := renderContextBar(50, 50, true)
	if !strings.Contains(withBuffer, "▒") {
		t.Errorf("buffer zone should show ▒ when enabled: %s", withBuffer)
	}

	// Without buffer, should not have ▒ characters
	withoutBuffer := renderContextBar(50, 50, false)
	if strings.Contains(withoutBuffer, "▒") {
		t.Errorf("buffer zone should not show ▒ when disabled: %s", withoutBuffer)
	}
}

// TestRenderContext_UsesNewUsedPercentage verifies Claude Code 2.1.6+ field is used
func TestRenderContext_UsesNewUsedPercentage(t *testing.T) {
	sl := &StatusLine{
		input: Input{
			Context: ContextInfo{
				UsedPercentage: 42.0,
				// Legacy fields would calculate differently
				CurrentUsage: ContextUsage{
					InputTokens:  10000,
					OutputTokens: 10000,
				},
				ContextWindow: 200000,
			},
		},
		config: config.Config{},
	}

	result := sl.renderContext()

	// Should use the new UsedPercentage (42%), not calculated from tokens
	if !strings.Contains(result, "42%") {
		t.Errorf("should use UsedPercentage field, got: %s", result)
	}
}

// TestRenderContext_UsesRemainingPercentage verifies fallback to remaining_percentage
func TestRenderContext_UsesRemainingPercentage(t *testing.T) {
	sl := &StatusLine{
		input: Input{
			Context: ContextInfo{
				UsedPercentage:      0,    // Not provided
				RemainingPercentage: 70.0, // 100 - 70 = 30% used
				CurrentUsage: ContextUsage{
					InputTokens: 50000, // Would calculate to 25%
				},
				ContextWindow: 200000,
			},
		},
		config: config.Config{},
	}

	result := sl.renderContext()

	// Should calculate 100 - 70 = 30%
	if !strings.Contains(result, "30%") {
		t.Errorf("should calculate used from remaining (30%%), got: %s", result)
	}
}

// TestRenderContext_FallsBackToLegacy verifies legacy calculation when no new fields
func TestRenderContext_FallsBackToLegacy(t *testing.T) {
	sl := &StatusLine{
		input: Input{
			Context: ContextInfo{
				UsedPercentage:      0, // Not provided
				RemainingPercentage: 0, // Not provided
				CurrentUsage: ContextUsage{
					InputTokens:  50000,
					OutputTokens: 0,
				},
				ContextWindow: 200000,
			},
		},
		config: config.Config{},
	}

	result := sl.renderContext()

	// Should fall back to legacy: 50000 / (200000 * 0.775) = 32% (with 22.5% autocompact buffer)
	if !strings.Contains(result, "32%") {
		t.Errorf("should fall back to legacy calculation (32%%), got: %s", result)
	}
}

// TestCalculateContextPct_PrefersNewFields verifies plugin context pct uses new fields
func TestCalculateContextPct_PrefersNewFields(t *testing.T) {
	sl := &StatusLine{
		input: Input{
			Context: ContextInfo{
				UsedPercentage: 75.0,
				CurrentUsage: ContextUsage{
					InputTokens: 10000, // Would calculate to 5%
				},
				ContextWindow: 200000,
			},
		},
		config: config.Config{},
	}

	pct := sl.calculateContextPct()

	if pct != 75 {
		t.Errorf("calculateContextPct should prefer UsedPercentage, got: %d", pct)
	}
}

// TestCalculateContextPct_FallsBackToLegacy verifies legacy fallback for plugins
func TestCalculateContextPct_FallsBackToLegacy(t *testing.T) {
	sl := &StatusLine{
		input: Input{
			Context: ContextInfo{
				UsedPercentage:      0,
				RemainingPercentage: 0,
				CurrentUsage: ContextUsage{
					InputTokens: 40000,
				},
				ContextWindow: 200000,
			},
		},
		config: config.Config{},
	}

	pct := sl.calculateContextPct()

	// 40000 / (200000 * 0.775) = 25% (with 22.5% autocompact buffer)
	if pct != 25 {
		t.Errorf("calculateContextPct should fall back to legacy (25%%), got: %d", pct)
	}
}

// TestCalculateContextPct_OnlyRemainingPercentage verifies correct calculation
// when only RemainingPercentage is set (UsedPercentage is 0).
func TestCalculateContextPct_OnlyRemainingPercentage(t *testing.T) {
	sl := &StatusLine{
		input: Input{
			Context: ContextInfo{
				UsedPercentage:      0,
				RemainingPercentage: 60.0,
			},
		},
		config: config.Config{},
	}

	pct := sl.calculateContextPct()
	if pct != 40 {
		t.Errorf("calculateContextPct with RemainingPercentage=60 should return 40, got: %d", pct)
	}
}

// TestCalculateContextPct_BothSet verifies UsedPercentage takes priority
// when both UsedPercentage and RemainingPercentage are set.
func TestCalculateContextPct_BothSet(t *testing.T) {
	sl := &StatusLine{
		input: Input{
			Context: ContextInfo{
				UsedPercentage:      75.0,
				RemainingPercentage: 30.0,
			},
		},
		config: config.Config{},
	}

	pct := sl.calculateContextPct()
	if pct != 75 {
		t.Errorf("calculateContextPct with UsedPercentage=75 should return 75, got: %d", pct)
	}
}

// TestCalculateContextPct_NeitherSet verifies fallback when both are zero.
func TestCalculateContextPct_NeitherSet(t *testing.T) {
	sl := &StatusLine{
		input: Input{
			Context: ContextInfo{
				UsedPercentage:      0,
				RemainingPercentage: 0,
				ContextWindow:       0, // No legacy data either
			},
		},
		config: config.Config{},
	}

	pct := sl.calculateContextPct()
	if pct != 0 {
		t.Errorf("calculateContextPct with neither set should return 0, got: %d", pct)
	}
}

// TestIsWorktree_MainRepo returns false for main repository
func TestIsWorktree_MainRepo(t *testing.T) {
	tmpDir := setupTestGitRepo(t)
	defer os.RemoveAll(tmpDir)

	sl := &StatusLine{
		input: Input{
			Workspace: WorkspaceInfo{
				ProjectDir: tmpDir,
			},
		},
	}

	if sl.isWorktree() {
		t.Error("isWorktree should return false for main repo")
	}
}

// TestIsWorktree_Worktree returns true for a git worktree
func TestIsWorktree_Worktree(t *testing.T) {
	tmpDir := setupTestGitRepo(t)
	defer os.RemoveAll(tmpDir)

	// Create a worktree
	worktreeDir := filepath.Join(os.TempDir(), "prism-test-worktree")
	defer os.RemoveAll(worktreeDir)

	cmd := exec.Command("git", "worktree", "add", worktreeDir, "HEAD")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to create worktree: %v", err)
	}

	sl := &StatusLine{
		input: Input{
			Workspace: WorkspaceInfo{
				ProjectDir: worktreeDir,
			},
		},
	}

	if !sl.isWorktree() {
		t.Error("isWorktree should return true for worktree")
	}
}

// TestIsWorktree_NonGitDir returns false for non-git directory
func TestIsWorktree_NonGitDir(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "prism-test-nogit-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	sl := &StatusLine{
		input: Input{
			Workspace: WorkspaceInfo{
				ProjectDir: tmpDir,
			},
		},
	}

	if sl.isWorktree() {
		t.Error("isWorktree should return false for non-git directory")
	}
}

// TestIsWorktree_EmptyProjectDir returns false for empty project dir
func TestIsWorktree_EmptyProjectDir(t *testing.T) {
	sl := &StatusLine{
		input: Input{
			Workspace: WorkspaceInfo{
				ProjectDir: "",
			},
		},
	}

	if sl.isWorktree() {
		t.Error("isWorktree should return false for empty project dir")
	}
}

// TestRenderDir_WorktreeIndicator shows ⎇ for worktrees
func TestRenderDir_WorktreeIndicator(t *testing.T) {
	tmpDir := setupTestGitRepo(t)
	defer os.RemoveAll(tmpDir)

	// Create a worktree
	worktreeDir := filepath.Join(os.TempDir(), "prism-test-worktree-render")
	defer os.RemoveAll(worktreeDir)

	cmd := exec.Command("git", "worktree", "add", worktreeDir, "HEAD")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to create worktree: %v", err)
	}

	sl := &StatusLine{
		input: Input{
			Workspace: WorkspaceInfo{
				ProjectDir: worktreeDir,
				CurrentDir: worktreeDir,
			},
		},
		config: config.Config{Icon: "💎"},
	}

	result := sl.renderDir()

	if !strings.Contains(result, "⎇") {
		t.Errorf("renderDir should include ⎇ indicator for worktree, got: %s", result)
	}

	// Should show mainRepoName/worktreeName
	mainRepoName := filepath.Base(tmpDir)
	worktreeName := filepath.Base(worktreeDir)
	expected := mainRepoName + "/" + worktreeName
	if !strings.Contains(result, expected) {
		t.Errorf("renderDir should show '%s', got: %s", expected, result)
	}
}

// TestRenderDir_NoIndicatorForMainRepo does not show ⎇ for main repo
func TestRenderDir_NoIndicatorForMainRepo(t *testing.T) {
	tmpDir := setupTestGitRepo(t)
	defer os.RemoveAll(tmpDir)

	sl := &StatusLine{
		input: Input{
			Workspace: WorkspaceInfo{
				ProjectDir: tmpDir,
				CurrentDir: tmpDir,
			},
		},
		config: config.Config{Icon: "💎"},
	}

	result := sl.renderDir()

	if strings.Contains(result, "⎇") {
		t.Errorf("renderDir should not include ⎇ indicator for main repo, got: %s", result)
	}
}

// TestFindWorktreeRoot_MainRepo returns false for main repository
func TestFindWorktreeRoot_MainRepo(t *testing.T) {
	tmpDir := setupTestGitRepo(t)
	defer os.RemoveAll(tmpDir)

	root, found := findWorktreeRoot(tmpDir)
	if found {
		t.Errorf("findWorktreeRoot should return false for main repo, got root: %s", root)
	}
}

// TestFindWorktreeRoot_Worktree returns worktree root
func TestFindWorktreeRoot_Worktree(t *testing.T) {
	tmpDir := setupTestGitRepo(t)
	defer os.RemoveAll(tmpDir)

	// Create a worktree
	worktreeDir := filepath.Join(os.TempDir(), "prism-test-worktree-find")
	defer os.RemoveAll(worktreeDir)

	cmd := exec.Command("git", "worktree", "add", worktreeDir, "HEAD")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to create worktree: %v", err)
	}

	root, found := findWorktreeRoot(worktreeDir)
	if !found {
		t.Error("findWorktreeRoot should return true for worktree")
	}
	if root != worktreeDir {
		t.Errorf("expected root %s, got %s", worktreeDir, root)
	}
}

// TestFindWorktreeRoot_WorktreeSubdir finds root from subdir
func TestFindWorktreeRoot_WorktreeSubdir(t *testing.T) {
	tmpDir := setupTestGitRepo(t)
	defer os.RemoveAll(tmpDir)

	// Create a worktree
	worktreeDir := filepath.Join(os.TempDir(), "prism-test-worktree-subdir")
	defer os.RemoveAll(worktreeDir)

	cmd := exec.Command("git", "worktree", "add", worktreeDir, "HEAD")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to create worktree: %v", err)
	}

	// Create a subdirectory in the worktree
	subDir := filepath.Join(worktreeDir, "src", "components")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}

	root, found := findWorktreeRoot(subDir)
	if !found {
		t.Error("findWorktreeRoot should return true for worktree subdir")
	}
	if root != worktreeDir {
		t.Errorf("expected root %s, got %s", worktreeDir, root)
	}
}

// TestFindWorktreeRoot_EmptyDir returns false for empty string
func TestFindWorktreeRoot_EmptyDir(t *testing.T) {
	root, found := findWorktreeRoot("")
	if found {
		t.Errorf("findWorktreeRoot should return false for empty dir, got root: %s", root)
	}
}

// TestRenderDir_CurrentDirInWorktree - main scenario: start in main repo, cd into worktree
func TestRenderDir_CurrentDirInWorktree(t *testing.T) {
	tmpDir := setupTestGitRepo(t)
	defer os.RemoveAll(tmpDir)

	// Create a worktree
	worktreeDir := filepath.Join(os.TempDir(), "prism-test-worktree-cd")
	defer os.RemoveAll(worktreeDir)

	cmd := exec.Command("git", "worktree", "add", worktreeDir, "HEAD")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to create worktree: %v", err)
	}

	// Simulate: Claude started in main repo (projectDir) but cd'd into worktree (currentDir)
	sl := &StatusLine{
		input: Input{
			Workspace: WorkspaceInfo{
				ProjectDir: tmpDir,      // Where Claude started
				CurrentDir: worktreeDir, // Where Claude cd'd to
			},
		},
		config: config.Config{},
	}

	result := sl.renderDir()

	// Should show worktree name with ⎇ indicator
	worktreeName := filepath.Base(worktreeDir)
	if !strings.Contains(result, "⎇") {
		t.Errorf("should show ⎇ indicator when currentDir is in worktree, got: %s", result)
	}
	if !strings.Contains(result, worktreeName) {
		t.Errorf("should show worktree name '%s', got: %s", worktreeName, result)
	}
	// Should show the main repo name as prefix
	mainRepoName := filepath.Base(tmpDir)
	expected := mainRepoName + "/" + worktreeName
	if !strings.Contains(result, expected) {
		t.Errorf("should show '%s' (mainRepo/worktree), got: %s", expected, result)
	}
}

// TestRenderDir_CurrentDirInWorktreeSubdir shows worktree name + subdir
func TestRenderDir_CurrentDirInWorktreeSubdir(t *testing.T) {
	tmpDir := setupTestGitRepo(t)
	defer os.RemoveAll(tmpDir)

	// Create a worktree
	worktreeDir := filepath.Join(os.TempDir(), "prism-test-worktree-subdir-render")
	defer os.RemoveAll(worktreeDir)

	cmd := exec.Command("git", "worktree", "add", worktreeDir, "HEAD")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to create worktree: %v", err)
	}

	// Create a subdirectory in the worktree
	subDir := filepath.Join(worktreeDir, "src")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}

	// Simulate: Claude started in main repo but cd'd into worktree/src
	sl := &StatusLine{
		input: Input{
			Workspace: WorkspaceInfo{
				ProjectDir: tmpDir, // Where Claude started
				CurrentDir: subDir, // Worktree subdirectory
			},
		},
		config: config.Config{},
	}

	result := sl.renderDir()

	// Should show mainRepo/worktreeName + /src with ⎇ indicator
	worktreeName := filepath.Base(worktreeDir)
	mainRepoName := filepath.Base(tmpDir)
	if !strings.Contains(result, "⎇") {
		t.Errorf("should show ⎇ indicator, got: %s", result)
	}
	expected := mainRepoName + "/" + worktreeName
	if !strings.Contains(result, expected) {
		t.Errorf("should show '%s', got: %s", expected, result)
	}
	if !strings.Contains(result, "/src") {
		t.Errorf("should show subdir '/src', got: %s", result)
	}
}

func TestCompactionProximity(t *testing.T) {
	tests := []struct {
		name      string
		rawPct    int
		bufferPct float64
		expected  int
	}{
		{"81% raw with 22.5% buffer", 81, 22.5, 100},   // 81 * 100 / 77.5 = 104.5 → capped at 100
		{"60% raw with 22.5% buffer", 60, 22.5, 77},    // 60 * 100 / 77.5 = 77.4
		{"30% raw with 22.5% buffer", 30, 22.5, 38},    // 30 * 100 / 77.5 = 38.7
		{"81% raw with 0% buffer", 81, 0, 81},          // disabled, returns raw
		{"95% raw with 0% buffer", 95, 0, 95},          // disabled, returns raw
		{"0% raw with 22.5% buffer", 0, 22.5, 0},       // 0 stays 0
		{"100% raw with 22.5% buffer", 100, 22.5, 100}, // capped at 100
		{"bufferPct exactly 100", 50, 100, 100},
		{"bufferPct over 100", 50, 150, 100},
		{"negative bufferPct", 50, -10, 50},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := compactionProximity(tt.rawPct, tt.bufferPct)
			if result != tt.expected {
				t.Errorf("compactionProximity(%d, %.1f) = %d, want %d",
					tt.rawPct, tt.bufferPct, result, tt.expected)
			}
		})
	}
}

func TestRenderContext_BugScenario_81PctShowsRedColor(t *testing.T) {
	// The bug: 81% raw with 22.5% buffer should show RED (proximity ~104%)
	// Previously showed yellow because color was based on raw 81%
	sl := &StatusLine{
		input: Input{
			Context: ContextInfo{
				UsedPercentage: 81.0,
			},
		},
		config: config.Config{}, // Default 22.5% buffer
	}

	result := sl.renderContext()

	if !strings.Contains(result, "81%") {
		t.Errorf("should display raw 81%%, got: %s", result)
	}
	// Should be red (proximity = 81 * 100 / 77.5 = 104.5, capped at 100, >= 90)
	if !strings.Contains(result, colors.Red) {
		t.Errorf("81%% with 22.5%% buffer should be RED (proximity ~104%%), got: %s", result)
	}
}

func TestRenderContext_BufferDisabled_YellowAt81(t *testing.T) {
	// With buffer disabled (0%), 81% should be yellow (70 <= 81 < 90)
	zero := 0.0
	sl := &StatusLine{
		input: Input{
			Context: ContextInfo{
				UsedPercentage: 81.0,
			},
		},
		config: config.Config{
			AutocompactBuffer: &zero,
		},
	}

	result := sl.renderContext()

	if !strings.Contains(result, "81%") {
		t.Errorf("should display raw 81%%, got: %s", result)
	}
	// Should be yellow, not red (buffer disabled, raw 81% >= 70 but < 90)
	if !strings.Contains(result, colors.Yellow) {
		t.Errorf("81%% with buffer disabled should be YELLOW, got: %s", result)
	}
	if strings.Contains(result, colors.Red) {
		t.Errorf("81%% with buffer disabled should NOT be red, got: %s", result)
	}
}

func TestRenderContext_60PctShowsYellowWithBuffer(t *testing.T) {
	// 60% raw with 22.5% buffer → proximity ~77% → yellow
	sl := &StatusLine{
		input: Input{
			Context: ContextInfo{
				UsedPercentage: 60.0,
			},
		},
		config: config.Config{}, // Default 22.5% buffer
	}

	result := sl.renderContext()

	if !strings.Contains(result, "60%") {
		t.Errorf("should display raw 60%%, got: %s", result)
	}
	if !strings.Contains(result, colors.Yellow) {
		t.Errorf("60%% with 22.5%% buffer should be YELLOW (proximity ~77%%), got: %s", result)
	}
}

// TestRenderContext_LegacyExcludesCacheReadTokens verifies that cache read tokens
// do not inflate the legacy context percentage calculation. Cache read tokens represent
// tokens served from cache at the same logical position and don't consume additional space.
func TestRenderContext_LegacyExcludesCacheReadTokens(t *testing.T) {
	// Set up two StatusLines: one with cache read tokens, one without.
	// Both should produce the same percentage since cache reads shouldn't count.
	base := ContextUsage{
		InputTokens:         40000,
		OutputTokens:        0,
		CacheCreationTokens: 0,
		CacheReadTokens:     0,
	}
	withCache := ContextUsage{
		InputTokens:         40000,
		OutputTokens:        0,
		CacheCreationTokens: 0,
		CacheReadTokens:     50000, // Large cache read — should be ignored
	}

	slBase := &StatusLine{
		input: Input{
			Context: ContextInfo{
				UsedPercentage:      0,
				RemainingPercentage: 0,
				CurrentUsage:        base,
				ContextWindow:       200000,
			},
		},
		config: config.Config{},
	}
	slWithCache := &StatusLine{
		input: Input{
			Context: ContextInfo{
				UsedPercentage:      0,
				RemainingPercentage: 0,
				CurrentUsage:        withCache,
				ContextWindow:       200000,
			},
		},
		config: config.Config{},
	}

	pctBase := slBase.calculateContextPctLegacy()
	pctWithCache := slWithCache.calculateContextPctLegacy()

	if pctBase != pctWithCache {
		t.Errorf("cache read tokens should not affect percentage: without=%d%%, with=%d%%", pctBase, pctWithCache)
	}

	// Verify the actual value is correct: 40000 / (200000 * 0.775) = 25%
	if pctBase != 25 {
		t.Errorf("expected 25%% for 40000 tokens in 200000 window, got %d%%", pctBase)
	}
}

// TestGetEffectiveGitDir_ProjectDirIsGitRepo returns ProjectDir when it is a git repo
func TestGetEffectiveGitDir_ProjectDirIsGitRepo(t *testing.T) {
	tmpDir := setupTestGitRepo(t)
	defer os.RemoveAll(tmpDir)

	sl := &StatusLine{
		input: Input{
			Workspace: WorkspaceInfo{
				ProjectDir: tmpDir,
				CurrentDir: tmpDir,
			},
		},
	}

	result := sl.getEffectiveGitDir()
	if result != tmpDir {
		t.Errorf("expected ProjectDir %s, got %s", tmpDir, result)
	}
}

// TestGetEffectiveGitDir_NonGitProjectDir_GitCurrentDir falls back to CurrentDir's git root
func TestGetEffectiveGitDir_NonGitProjectDir_GitCurrentDir(t *testing.T) {
	parentDir, err := os.MkdirTemp("", "prism-test-parent-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(parentDir)

	gitRepoDir := filepath.Join(parentDir, "my-repo")
	if err := os.MkdirAll(gitRepoDir, 0755); err != nil {
		t.Fatal(err)
	}
	setupTestGitRepoAt(t, gitRepoDir)

	sl := &StatusLine{
		input: Input{
			Workspace: WorkspaceInfo{
				ProjectDir: parentDir,
				CurrentDir: gitRepoDir,
			},
		},
	}

	result := sl.getEffectiveGitDir()
	expectedDir, err := filepath.EvalSymlinks(gitRepoDir)
	if err != nil {
		t.Fatalf("failed to eval symlinks: %v", err)
	}
	if result != expectedDir {
		t.Errorf("expected git root %s, got %s", expectedDir, result)
	}
}

// TestGetEffectiveGitDir_NonGitProjectDir_GitCurrentDirSubdir finds git root from subdirectory
func TestGetEffectiveGitDir_NonGitProjectDir_GitCurrentDirSubdir(t *testing.T) {
	parentDir, err := os.MkdirTemp("", "prism-test-parent-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(parentDir)

	gitRepoDir := filepath.Join(parentDir, "my-repo")
	if err := os.MkdirAll(gitRepoDir, 0755); err != nil {
		t.Fatal(err)
	}
	setupTestGitRepoAt(t, gitRepoDir)

	// Create a subdirectory inside the git repo
	subDir := filepath.Join(gitRepoDir, "src", "components")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}

	sl := &StatusLine{
		input: Input{
			Workspace: WorkspaceInfo{
				ProjectDir: parentDir,
				CurrentDir: subDir,
			},
		},
	}

	result := sl.getEffectiveGitDir()
	expectedDir, err := filepath.EvalSymlinks(gitRepoDir)
	if err != nil {
		t.Fatalf("failed to eval symlinks: %v", err)
	}
	if result != expectedDir {
		t.Errorf("expected git root %s, got %s", expectedDir, result)
	}
}

// TestRenderLinesChanged_NonGitProjectDir_GitCurrentDir shows diff stats from CurrentDir's repo
func TestRenderLinesChanged_NonGitProjectDir_GitCurrentDir(t *testing.T) {
	parentDir, err := os.MkdirTemp("", "prism-test-parent-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(parentDir)

	gitRepoDir := filepath.Join(parentDir, "my-repo")
	if err := os.MkdirAll(gitRepoDir, 0755); err != nil {
		t.Fatal(err)
	}
	setupTestGitRepoAt(t, gitRepoDir)

	// Create a staged change so diff stats are non-zero
	newFile := filepath.Join(gitRepoDir, "new.txt")
	if err := os.WriteFile(newFile, []byte("line1\nline2\nline3\n"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	cmd := exec.Command("git", "add", "new.txt")
	cmd.Dir = gitRepoDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to git add: %v", err)
	}

	sl := &StatusLine{
		input: Input{
			Workspace: WorkspaceInfo{
				ProjectDir: parentDir,
				CurrentDir: gitRepoDir,
			},
		},
	}

	result := sl.renderLinesChanged()

	// Should show +3 -0 from the git repo, not +0 -0
	if !strings.Contains(result, "+3") {
		t.Errorf("expected +3 for staged lines in CurrentDir repo, got: %s", result)
	}
}

// TestCacheRatio tests the cache ratio calculation helper
func TestCacheRatio(t *testing.T) {
	tests := []struct {
		name                  string
		input, creation, read int
		expectedRatio         int
		expectedOK            bool
	}{
		{"normal cache hits", 5000, 2000, 3000, 30, true},     // 3000/(5000+2000+3000)=30%
		{"high cache ratio", 1000, 1000, 8000, 80, true},      // 8000/10000=80%
		{"all cache reads", 0, 0, 10000, 100, true},           // 10000/10000=100%
		{"zero cache reads", 10000, 5000, 0, 0, false},        // no cache reads
		{"zero denominator", 0, 0, 0, 0, false},               // all zero
		{"negative cache reads", 10000, 0, -1, 0, false},      // negative
		{"only input tokens no cache", 50000, 0, 0, 0, false}, // no cache at all
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ratio, ok := tokens.CacheEfficiency(tt.input, tt.creation, tt.read)
			if ok != tt.expectedOK {
				t.Errorf("CacheEfficiency(%d, %d, %d) ok = %v, want %v", tt.input, tt.creation, tt.read, ok, tt.expectedOK)
			}
			if ok && ratio != tt.expectedRatio {
				t.Errorf("CacheEfficiency(%d, %d, %d) = %d, want %d", tt.input, tt.creation, tt.read, ratio, tt.expectedRatio)
			}
		})
	}
}

// TestRenderCost_WithCacheIndicator verifies cache indicator appears in cost display
func TestRenderCost_WithCacheIndicator(t *testing.T) {
	sl := &StatusLine{
		input: Input{
			Cost: CostInfo{TotalCostUSD: 1.50},
			Context: ContextInfo{
				CurrentUsage: ContextUsage{
					InputTokens:         5000,
					CacheCreationTokens: 2000,
					CacheReadTokens:     3000,
				},
			},
		},
	}

	result := sl.renderCost()

	if !strings.Contains(result, "$1.50") {
		t.Errorf("expected cost $1.50, got: %s", result)
	}
	// 3000 / (5000+2000+3000) = 30%
	if !strings.Contains(result, "⌁30%") {
		t.Errorf("expected cache indicator ⌁30%%, got: %s", result)
	}
}

// TestRenderCost_CacheBeforeBurnRate verifies output order: cost → cache → burn rate
func TestRenderCost_CacheBeforeBurnRate(t *testing.T) {
	sessionID := "test-order-" + time.Now().Format("20060102150405")
	path := burnrate.FilePath(sessionID)
	defer os.Remove(path)

	// Create a snapshot from 2 hours ago
	snap := struct {
		Timestamp time.Time `json:"timestamp"`
		CostUSD   float64   `json:"cost_usd"`
	}{
		Timestamp: time.Now().Add(-2 * time.Hour),
		CostUSD:   1.00,
	}
	data, err := json.Marshal(snap)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("failed to write: %v", err)
	}

	sl := &StatusLine{
		input: Input{
			SessionID: sessionID,
			Cost:      CostInfo{TotalCostUSD: 5.00},
			Context: ContextInfo{
				CurrentUsage: ContextUsage{
					InputTokens:         5000,
					CacheCreationTokens: 2000,
					CacheReadTokens:     3000,
				},
			},
		},
	}

	result := sl.renderCost()

	cacheIdx := strings.Index(result, "⌁")
	burnIdx := strings.Index(result, "~$")
	if cacheIdx < 0 || burnIdx < 0 {
		t.Fatalf("expected both cache and burn rate in output, got: %s", result)
	}
	if cacheIdx > burnIdx {
		t.Errorf("cache indicator should appear before burn rate, got: %s", result)
	}
}

// TestRenderCost_ShowCacheDisabled verifies show_cache=false suppresses cache indicator
func TestRenderCost_ShowCacheDisabled(t *testing.T) {
	sl := &StatusLine{
		input: Input{
			Cost: CostInfo{TotalCostUSD: 1.50},
			Context: ContextInfo{
				CurrentUsage: ContextUsage{
					InputTokens:         5000,
					CacheCreationTokens: 2000,
					CacheReadTokens:     3000,
				},
			},
		},
		config: config.Config{
			Plugins: map[string]any{
				"usage": map[string]any{
					"api_billing": map[string]any{
						"show_cache": false,
					},
				},
			},
		},
	}

	result := sl.renderCost()

	if strings.Contains(result, "⌁") {
		t.Errorf("show_cache=false should suppress cache indicator, got: %s", result)
	}
}

// TestRenderCost_ShowBurnRateDisabled verifies show_burn_rate=false suppresses burn rate
func TestRenderCost_ShowBurnRateDisabled(t *testing.T) {
	sessionID := "test-noburnrate-" + time.Now().Format("20060102150405")
	path := burnrate.FilePath(sessionID)
	defer os.Remove(path)

	// Create a snapshot from 2 hours ago
	snap := struct {
		Timestamp time.Time `json:"timestamp"`
		CostUSD   float64   `json:"cost_usd"`
	}{
		Timestamp: time.Now().Add(-2 * time.Hour),
		CostUSD:   1.00,
	}
	data, err := json.Marshal(snap)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("failed to write: %v", err)
	}

	sl := &StatusLine{
		input: Input{
			SessionID: sessionID,
			Cost:      CostInfo{TotalCostUSD: 5.00},
		},
		config: config.Config{
			Plugins: map[string]any{
				"usage": map[string]any{
					"api_billing": map[string]any{
						"show_burn_rate": false,
					},
				},
			},
		},
	}

	result := sl.renderCost()

	if strings.Contains(result, "/h") {
		t.Errorf("show_burn_rate=false should suppress burn rate, got: %s", result)
	}
}

// TestGetMainRepoName_ValidWorktree returns correct main repo name
func TestGetMainRepoName_ValidWorktree(t *testing.T) {
	tmpDir := setupTestGitRepo(t)
	defer os.RemoveAll(tmpDir)

	// Create a worktree
	worktreeDir := filepath.Join(os.TempDir(), "prism-test-mainrepo-valid")
	defer os.RemoveAll(worktreeDir)

	cmd := exec.Command("git", "worktree", "add", worktreeDir, "HEAD")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to create worktree: %v", err)
	}

	name := getMainRepoName(worktreeDir)
	expected := filepath.Base(tmpDir)
	if name != expected {
		t.Errorf("getMainRepoName should return '%s', got '%s'", expected, name)
	}
}

// TestGetMainRepoName_MainRepo returns empty for a regular (non-worktree) repo
func TestGetMainRepoName_MainRepo(t *testing.T) {
	tmpDir := setupTestGitRepo(t)
	defer os.RemoveAll(tmpDir)

	name := getMainRepoName(tmpDir)
	if name != "" {
		t.Errorf("getMainRepoName should return '' for main repo, got '%s'", name)
	}
}

// TestParseMainRepoName_Fallbacks verifies graceful failures
func TestParseMainRepoName_Fallbacks(t *testing.T) {
	// Nonexistent directory
	if name := parseMainRepoName("/nonexistent/path"); name != "" {
		t.Errorf("expected '' for nonexistent dir, got '%s'", name)
	}

	// Directory without .git
	tmpDir, err := os.MkdirTemp("", "prism-test-nogit-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	if name := parseMainRepoName(tmpDir); name != "" {
		t.Errorf("expected '' for dir without .git, got '%s'", name)
	}

	// .git as directory (regular repo, not worktree)
	gitDir := filepath.Join(tmpDir, ".git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatal(err)
	}

	if name := parseMainRepoName(tmpDir); name != "" {
		t.Errorf("expected '' for .git directory, got '%s'", name)
	}
}

// TestRenderCost_NoCacheIndicatorWhenZero verifies no cache indicator when no cache reads
func TestRenderCost_NoCacheIndicatorWhenZero(t *testing.T) {
	sl := &StatusLine{
		input: Input{
			Cost: CostInfo{TotalCostUSD: 2.00},
			Context: ContextInfo{
				CurrentUsage: ContextUsage{
					InputTokens:         10000,
					CacheCreationTokens: 5000,
					CacheReadTokens:     0,
				},
			},
		},
	}

	result := sl.renderCost()

	if !strings.Contains(result, "$2.00") {
		t.Errorf("expected cost $2.00, got: %s", result)
	}
	if strings.Contains(result, "⌁") {
		t.Errorf("should not show cache indicator when no cache reads, got: %s", result)
	}
}

func TestFormatContextWindowSize(t *testing.T) {
	tests := []struct {
		tokens   int
		expected string
	}{
		{1000000, "(1M)"},
		{2000000, "(2M)"},
		{1500000, "(1.5M)"},
		{1100000, "(1.1M)"},
		{200000, "(200k)"},
		{128000, "(128k)"},
		{100000, "(100k)"},
		{1500, "(1.5k)"},
		{500, "(500)"},
		{0, "(0)"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			got := formatContextWindowSize(tt.tokens)
			if got != tt.expected {
				t.Errorf("formatContextWindowSize(%d) = %q, want %q", tt.tokens, got, tt.expected)
			}
		})
	}
}

func TestRenderModel_ShowsContextWindow(t *testing.T) {
	sl := &StatusLine{
		input: Input{
			Model: ModelInfo{DisplayName: "Opus 4.6"},
			Context: ContextInfo{
				ContextWindow: 1000000,
			},
		},
	}

	result := sl.renderModel()
	if !strings.Contains(result, "Opus 4.6") {
		t.Errorf("expected model name in output, got: %s", result)
	}
	if !strings.Contains(result, "(1M)") {
		t.Errorf("expected (1M) context window indicator, got: %s", result)
	}
}

func TestTrimContextSuffix(t *testing.T) {
	tests := []struct {
		input, expected string
	}{
		{"Opus 4.6 (1M context)", "Opus 4.6"},
		{"Sonnet 4.6 (200k context)", "Sonnet 4.6"},
		{"Haiku 4.5", "Haiku 4.5"},
		{"Opus 4.6", "Opus 4.6"},
		{"Claude (beta) (1M context)", "Claude (beta)"},
		{"My Model (for some context)", "My Model (for some context)"},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := trimContextSuffix(tt.input)
			if got != tt.expected {
				t.Errorf("trimContextSuffix(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestRenderModel_TrimsContextSuffixFromDisplayName(t *testing.T) {
	sl := &StatusLine{
		input: Input{
			Model:   ModelInfo{DisplayName: "Opus 4.6 (1M context)"},
			Context: ContextInfo{ContextWindow: 1000000},
		},
	}

	result := sl.renderModel()
	// Should NOT have duplicate: "Opus 4.6 (1M context) (1M)"
	if strings.Contains(result, "context)") {
		t.Errorf("should trim context suffix from display name, got: %s", result)
	}
	if !strings.Contains(result, "Opus 4.6") || !strings.Contains(result, "(1M)") {
		t.Errorf("expected 'Opus 4.6 (1M)', got: %s", result)
	}
}

func TestRenderModel_FallsBackTo200kWhenNotProvided(t *testing.T) {
	sl := &StatusLine{
		input: Input{
			Model: ModelInfo{DisplayName: "Opus 4.6"},
			Context: ContextInfo{
				ContextWindow: 0, // Not provided by Claude Code
			},
		},
	}

	result := sl.renderModel()
	if !strings.Contains(result, "(200k)") {
		t.Errorf("expected (200k) fallback when context_window_size not provided, got: %s", result)
	}
}

func TestCalculateContextPctLegacy_1MWindow(t *testing.T) {
	sl := &StatusLine{
		input: Input{
			Model: ModelInfo{DisplayName: "Opus 4.6"},
			Context: ContextInfo{
				UsedPercentage:      0,
				RemainingPercentage: 0,
				CurrentUsage: ContextUsage{
					InputTokens: 100000,
				},
				ContextWindow: 1000000, // Provided by Claude Code
			},
		},
		config: config.Config{},
	}

	pct := sl.calculateContextPctLegacy()

	// 100000 / (1000000 * 0.775) = 12% (with 22.5% autocompact buffer)
	if pct != 12 {
		t.Errorf("expected 12%% with 1M window, got: %d%%", pct)
	}
}

func TestFlattenSectionLines(t *testing.T) {
	got := flattenSectionLines([][]string{{"dir", "model"}, {"context"}, {"git"}})
	if len(got) != 1 {
		t.Fatalf("expected a single line, got %d: %v", len(got), got)
	}
	want := []string{"dir", "model", "context", "git"}
	if len(got[0]) != len(want) {
		t.Fatalf("flattened sections = %v, want %v", got[0], want)
	}
	for i, s := range want {
		if got[0][i] != s {
			t.Fatalf("flattened sections = %v, want %v", got[0], want)
		}
	}

	if flattenSectionLines(nil) != nil {
		t.Error("flattening empty layout should return nil")
	}
}
