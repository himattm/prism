package statusline

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/himattm/prism/internal/cache"
	"github.com/himattm/prism/internal/colors"
	"github.com/himattm/prism/internal/config"
	"github.com/himattm/prism/internal/plugin"
	"github.com/himattm/prism/internal/plugins"
	"github.com/himattm/prism/internal/version"
)

// Package-level cache for statusline operations
var statusCache = cache.New()

// StatusLine handles rendering the status line
type StatusLine struct {
	input           Input
	config          config.Config
	pluginManager   *plugin.Manager
	nativePlugins   *plugins.Registry
	isIdle          bool
	bashPlugins     []plugin.Plugin // Cached discovered bash plugins
	bashPluginsOnce sync.Once
}

// New creates a new StatusLine renderer
func New(input Input, cfg config.Config) *StatusLine {
	return &StatusLine{
		input:         input,
		config:        cfg,
		pluginManager: plugin.NewManager(),
		nativePlugins: plugins.NewRegistry(),
		isIdle:        checkIsIdle(input.SessionID),
	}
}

// discoverBashPlugins discovers bash plugins once and caches them
func (sl *StatusLine) discoverBashPlugins() []plugin.Plugin {
	sl.bashPluginsOnce.Do(func() {
		discovered, err := sl.pluginManager.Discover()
		if err == nil {
			sl.bashPlugins = discovered
		}
	})
	return sl.bashPlugins
}

func checkIsIdle(sessionID string) bool {
	idleFile := filepath.Join(os.TempDir(), fmt.Sprintf("prism-idle-%s", sessionID))
	if _, err := os.Stat(idleFile); err == nil {
		return true
	}
	// Check if any idle files exist (hooks are active)
	matches, _ := filepath.Glob(filepath.Join(os.TempDir(), "prism-idle-*"))
	if len(matches) > 0 {
		return false
	}
	// No idle files = hooks not set up, assume idle
	return true
}

// Render generates the status line output
func (sl *StatusLine) Render() string {
	lines := sl.config.GetAllSectionLines()
	var output []string

	for i, sections := range lines {
		line := sl.renderLine(sections)
		if line != "" {
			// Prepend update indicator to first line only
			if i == 0 {
				updateOutput := sl.runUpdatePlugin()
				if updateOutput != "" {
					line = updateOutput + colors.Separator() + line
				}
			}
			output = append(output, line)
		}
	}

	return strings.Join(output, "\n")
}

func (sl *StatusLine) renderLine(sections []string) string {
	// Run all sections in parallel
	type result struct {
		index  int
		output string
	}

	results := make([]string, len(sections))
	var wg sync.WaitGroup

	for i, section := range sections {
		wg.Add(1)
		go func(idx int, sec string) {
			defer wg.Done()
			results[idx] = sl.renderSection(sec)
		}(i, section)
	}

	wg.Wait()

	// Filter empty and join (preserving order)
	var parts []string
	for _, out := range results {
		if out != "" {
			parts = append(parts, out)
		}
	}

	return strings.Join(parts, colors.Separator())
}

func (sl *StatusLine) renderSection(section string) string {
	switch section {
	case "dir":
		return sl.renderDir()
	case "model":
		return sl.renderModel()
	case "context":
		return sl.renderContext()
	case "linesChanged":
		return sl.renderLinesChanged()
	case "cost":
		return sl.renderCost()
	case "git":
		return sl.runPlugin("git")
	case "android_devices":
		return sl.runPlugin("android_devices")
	case "devices":
		return sl.runPlugin("devices") // legacy alias
	default:
		// Try to run as plugin
		return sl.runPlugin(section)
	}
}

func (sl *StatusLine) renderDir() string {
	projectDir := sl.input.Workspace.ProjectDir
	currentDir := sl.input.Workspace.CurrentDir

	// Determine display base - prioritize worktree detection on currentDir
	displayBase := projectDir
	inWorktree := false

	// Check if currentDir is in a worktree (even if projectDir isn't)
	if currentDir != "" {
		if worktreeRoot, ok := sl.findWorktreeRootCached(currentDir); ok {
			displayBase = worktreeRoot
			inWorktree = true
		}
	}

	// Fall back to checking projectDir
	if !inWorktree {
		inWorktree = sl.isWorktree()
	}

	displayName := filepath.Base(displayBase)
	icon := sl.config.Icon
	if icon != "" {
		icon += " "
	}

	// Calculate subdir relative to display base
	subdir := ""
	if currentDir != "" && displayBase != "" {
		if strings.HasPrefix(currentDir, displayBase) {
			subdir = strings.TrimPrefix(currentDir, displayBase)
		}
	}

	worktreeIndicator := ""
	if inWorktree {
		worktreeIndicator = fmt.Sprintf("%s⎇%s ", colors.Cyan, colors.Reset)
	}

	if subdir != "" {
		return fmt.Sprintf("%s%s%s%s%s%s%s",
			icon, worktreeIndicator, colors.Dim, colors.Cyan, displayName, colors.Reset,
			colors.Wrap(colors.Cyan, subdir))
	}

	return fmt.Sprintf("%s%s%s", icon, worktreeIndicator, colors.Wrap(colors.Cyan, displayName))
}

// isWorktree returns true if the project directory is a git worktree
func (sl *StatusLine) isWorktree() bool {
	projectDir := sl.input.Workspace.ProjectDir
	if projectDir == "" {
		return false
	}

	// Check cache first (worktree status rarely changes)
	cacheKey := "worktree:" + projectDir
	if cached, ok := statusCache.Get(cacheKey); ok {
		return cached == "true"
	}

	// In a worktree, .git is a file (not a directory)
	gitPath := filepath.Join(projectDir, ".git")
	info, err := os.Stat(gitPath)
	if err != nil {
		statusCache.Set(cacheKey, "false", cache.WorktreeTTL)
		return false
	}

	isWt := !info.IsDir()
	if isWt {
		statusCache.Set(cacheKey, "true", cache.WorktreeTTL)
	} else {
		statusCache.Set(cacheKey, "false", cache.WorktreeTTL)
	}
	return isWt
}

// findWorktreeRoot walks up from dir to find if it's within a git worktree.
// Returns (worktreeRoot, true) if found, ("", false) otherwise.
func findWorktreeRoot(dir string) (string, bool) {
	if dir == "" {
		return "", false
	}

	current := dir
	for {
		gitPath := filepath.Join(current, ".git")
		info, err := os.Stat(gitPath)
		if err == nil {
			if !info.IsDir() {
				return current, true // .git is file = worktree
			}
			return "", false // .git is dir = main repo, stop
		}

		parent := filepath.Dir(current)
		if parent == current {
			return "", false // reached root
		}
		current = parent
	}
}

// findWorktreeRootCached wraps findWorktreeRoot with caching
func (sl *StatusLine) findWorktreeRootCached(dir string) (string, bool) {
	if dir == "" {
		return "", false
	}

	cacheKey := "worktree-root:" + dir
	if cached, ok := statusCache.Get(cacheKey); ok {
		if cached == "" {
			return "", false
		}
		return cached, true
	}

	root, isWorktree := findWorktreeRoot(dir)
	if isWorktree {
		statusCache.Set(cacheKey, root, cache.WorktreeTTL)
	} else {
		statusCache.Set(cacheKey, "", cache.WorktreeTTL)
	}
	return root, isWorktree
}

func (sl *StatusLine) renderModel() string {
	return colors.Wrap(colors.Magenta, sl.input.Model.DisplayName)
}

func (sl *StatusLine) renderContext() string {
	// Get autocompact buffer from config (default 22.5%)
	bufferPct := sl.config.GetAutocompactBuffer()

	// Check if Claude Code provided the new percentage fields (2.1.6+)
	// Use used_percentage directly, or calculate from remaining_percentage
	if sl.input.Context.UsedPercentage > 0 || sl.input.Context.RemainingPercentage > 0 {
		var pct int
		if sl.input.Context.UsedPercentage > 0 {
			pct = int(sl.input.Context.UsedPercentage)
		} else {
			// Calculate used from remaining (they should sum to 100)
			pct = int(100 - sl.input.Context.RemainingPercentage)
		}
		if pct > 100 {
			pct = 100
		}
		if pct < 0 {
			pct = 0
		}
		return renderContextBar(pct, bufferPct > 0)
	}

	// Fall back to legacy calculation for older Claude Code versions
	pct := sl.calculateContextPctLegacy()
	return renderContextBar(pct, bufferPct > 0)
}

func (sl *StatusLine) calculateContextPctLegacy() int {
	usage := sl.input.Context.CurrentUsage
	windowSize := sl.input.Context.ContextWindow
	if windowSize == 0 {
		windowSize = 200000 // Default
	}

	// Get autocompact buffer from config (default 22.5%)
	bufferPct := sl.config.GetAutocompactBuffer()

	// Calculate usable capacity (total - buffer)
	usableCapacity := windowSize
	if bufferPct > 0 {
		usableCapacity = int(float64(windowSize) * (100.0 - bufferPct) / 100.0)
	}

	totalTokens := usage.InputTokens + usage.OutputTokens +
		usage.CacheCreationTokens + usage.CacheReadTokens
	pct := (totalTokens * 100) / usableCapacity
	if pct > 100 {
		pct = 100
	}
	return pct
}

func renderContextBar(pct int, showBuffer bool) string {
	// 10-char bar: ████░░░░▒▒ (with buffer) or ████░░░░░░ (without)
	// No end caps for a cleaner look
	const barLen = 10
	filled := (pct * barLen) / 100
	if filled > barLen {
		filled = barLen
	}

	// Buffer zone is last 2-3 chars (representing ~22.5% of bar)
	// Only show if autocompact buffer is enabled
	bufferStart := 8 // Last 2 chars for buffer indicator

	// Choose color based on percentage: white -> yellow -> red
	// When colored, the entire bar is that color for uniformity
	var barColor string
	switch {
	case pct >= 90:
		barColor = colors.Red
	case pct >= 70:
		barColor = colors.Yellow
	default:
		barColor = "" // White/default
	}

	var bar strings.Builder

	// Apply color to entire bar when in warning/critical state
	if barColor != "" {
		bar.WriteString(barColor)
	}

	for i := 0; i < barLen; i++ {
		if i < filled {
			bar.WriteString("█")
		} else if showBuffer && i >= bufferStart {
			bar.WriteString("▒")
		} else {
			bar.WriteString("░")
		}
	}

	bar.WriteString(fmt.Sprintf(" %d%%", pct))

	if barColor != "" {
		bar.WriteString(colors.Reset)
	}

	return bar.String()
}

func (sl *StatusLine) renderLinesChanged() string {
	// ALWAYS use git diff stats - never use Claude's session stats
	// This shows actual uncommitted changes in the working tree
	added, removed := getGitDiffStats(sl.input.Workspace.ProjectDir)

	return fmt.Sprintf("%s+%d%s %s-%d%s",
		colors.Green, added, colors.Reset,
		colors.Red, removed, colors.Reset)
}

func getGitDiffStats(projectDir string) (int, int) {
	if projectDir == "" {
		return 0, 0
	}

	// Check cache first
	cacheKey := "diffstats:" + projectDir
	if cached, ok := statusCache.Get(cacheKey); ok {
		var added, removed int
		fmt.Sscanf(cached, "%d,%d", &added, &removed)
		return added, removed
	}

	cmd := exec.Command("git", "--no-optional-locks", "diff", "--numstat", "HEAD")
	cmd.Dir = projectDir
	output, err := cmd.Output()
	if err != nil {
		statusCache.Set(cacheKey, "0,0", cache.GitTTL)
		return 0, 0
	}

	var added, removed int
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		var a, r int
		fmt.Sscanf(line, "%d\t%d", &a, &r)
		added += a
		removed += r
	}

	// Cache the result
	statusCache.Set(cacheKey, fmt.Sprintf("%d,%d", added, removed), cache.GitTTL)
	return added, removed
}

func (sl *StatusLine) renderCost() string {
	cost := sl.input.Cost.TotalCostUSD
	return colors.Wrap(colors.Gray, fmt.Sprintf("$%.2f", cost))
}

func (sl *StatusLine) runPlugin(name string) string {
	// Build plugin input
	input := plugin.Input{
		Prism: plugin.PrismContext{
			Version:    version.Version,
			ProjectDir: sl.input.Workspace.ProjectDir,
			CurrentDir: sl.input.Workspace.CurrentDir,
			SessionID:  sl.input.SessionID,
			IsIdle:     sl.isIdle,
		},
		Session: plugin.SessionContext{
			Model:        sl.input.Model.DisplayName,
			ContextPct:   sl.calculateContextPct(),
			CostUSD:      sl.input.Cost.TotalCostUSD,
			LinesAdded:   sl.input.Cost.TotalLinesAdded,
			LinesRemoved: sl.input.Cost.TotalLinesRemoved,
		},
		Config: sl.getPluginConfig(name),
		Colors: colors.ColorMap(),
	}

	// Try native plugin first (much faster - no subprocess)
	if native := sl.nativePlugins.Get(name); native != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()

		output, err := native.Execute(ctx, input)
		if err == nil {
			return output
		}
		// Fall through to bash plugin on error
	}

	// Fall back to bash plugin
	return sl.runBashPlugin(name, input)
}

func (sl *StatusLine) runBashPlugin(name string, input plugin.Input) string {
	bashPlugins := sl.discoverBashPlugins()

	var targetPlugin *plugin.Plugin
	for _, p := range bashPlugins {
		if p.Name == name {
			targetPlugin = &p
			break
		}
	}

	if targetPlugin == nil {
		return ""
	}

	output, err := sl.pluginManager.Execute(*targetPlugin, input, 500*time.Millisecond)
	if err != nil {
		return ""
	}

	return output
}

func (sl *StatusLine) runUpdatePlugin() string {
	return sl.runPlugin("update")
}

func (sl *StatusLine) calculateContextPct() int {
	// Prefer new pre-calculated percentage from Claude Code 2.1.6+
	if sl.input.Context.UsedPercentage > 0 || sl.input.Context.RemainingPercentage > 0 {
		pct := int(sl.input.Context.UsedPercentage)
		if pct > 100 {
			pct = 100
		}
		return pct
	}

	// Fall back to legacy calculation
	return sl.calculateContextPctLegacy()
}

func (sl *StatusLine) getPluginConfig(name string) map[string]any {
	// Load from plugin's own config.json, then overlay prism.json overrides
	pluginCfg := sl.config.LoadPluginConfig(name)
	return map[string]any{name: pluginCfg}
}
