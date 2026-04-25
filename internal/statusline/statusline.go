package statusline

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/himattm/prism/internal/burnrate"
	"github.com/himattm/prism/internal/cache"
	"github.com/himattm/prism/internal/colors"
	"github.com/himattm/prism/internal/config"
	"github.com/himattm/prism/internal/git"
	"github.com/himattm/prism/internal/hooks"
	"github.com/himattm/prism/internal/plugin"
	"github.com/himattm/prism/internal/plugins"
	"github.com/himattm/prism/internal/tokens"
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
	colorsMap       map[string]string // Cached to avoid per-plugin allocations
}

// New creates a new StatusLine renderer
func New(input Input, cfg config.Config) *StatusLine {
	return &StatusLine{
		input:         input,
		config:        cfg,
		pluginManager: plugin.NewManager(),
		nativePlugins: plugins.NewRegistry(),
		isIdle:        checkIsIdle(input.SessionID),
		colorsMap:     colors.ColorMap(),
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
	idleFile := hooks.IdleFilePath(sessionID)
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
	if inWorktree {
		if mainRepo := getMainRepoName(displayBase); mainRepo != "" {
			displayName = mainRepo + "/" + displayName
		}
	}
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

// parseMainRepoName reads the .git file in a worktree root to determine the
// main repository name. In a git worktree, .git is a file containing:
//
//	gitdir: /path/to/main/repo/.git/worktrees/worktree-name
//
// Returns "" on any failure (not a worktree, can't read, can't parse).
func parseMainRepoName(worktreeRoot string) string {
	data, err := os.ReadFile(filepath.Join(worktreeRoot, ".git"))
	if err != nil {
		return ""
	}

	content := strings.TrimSpace(string(data))
	if !strings.HasPrefix(content, "gitdir: ") {
		return ""
	}

	gitdir := strings.TrimPrefix(content, "gitdir: ")
	if !filepath.IsAbs(gitdir) {
		gitdir = filepath.Clean(filepath.Join(worktreeRoot, gitdir))
	}

	// Find the ".git" segment in the path to locate the main repo root.
	// e.g. /path/to/main-repo/.git/worktrees/name → /path/to/main-repo
	sep := string(filepath.Separator)
	marker := sep + ".git" + sep
	idx := strings.LastIndex(gitdir, marker)
	if idx < 0 {
		return ""
	}

	return filepath.Base(gitdir[:idx])
}

// getMainRepoName returns the main repository name for a worktree, with caching.
func getMainRepoName(worktreeRoot string) string {
	cacheKey := "main-repo:" + worktreeRoot
	if cached, ok := statusCache.Get(cacheKey); ok {
		return cached
	}

	name := parseMainRepoName(worktreeRoot)
	statusCache.Set(cacheKey, name, cache.WorktreeTTL)
	return name
}

func (sl *StatusLine) renderModel() string {
	name := trimContextSuffix(sl.input.Model.DisplayName)
	windowSize := sl.input.Context.ContextWindow
	if windowSize == 0 {
		windowSize = 200000
	}
	suffix := formatContextWindowSize(windowSize)
	return colors.Wrap(colors.Magenta, name+" "+suffix)
}

// trimContextSuffix removes trailing context-window text that Claude Code may
// include in the display name, e.g. "Opus 4.6 (1M context)" → "Opus 4.6".
// This prevents duplication when we append our own "(1M)" suffix.
func trimContextSuffix(name string) string {
	// Match patterns like " (1M context)", " (200k context)"
	if idx := strings.LastIndex(name, " ("); idx >= 0 {
		tail := name[idx:]
		// To avoid false positives, also check that the suffix contains a digit.
		if strings.HasSuffix(tail, " context)") && strings.ContainsAny(tail, "0123456789") {
			return name[:idx]
		}
	}
	return name
}

// formatContextWindowSize formats a token count as a compact label like "(1M)" or "(200k)".
func formatContextWindowSize(tokens int) string {
	if tokens >= 1000000 {
		if tokens%1000000 == 0 {
			return fmt.Sprintf("(%dM)", tokens/1000000)
		}
		// e.g. 1500000 -> "(1.5M)"
		val := float64(tokens) / 1000000
		s := strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.1f", val), "0"), ".")
		return fmt.Sprintf("(%sM)", s)
	}
	if tokens >= 1000 {
		if tokens%1000 == 0 {
			return fmt.Sprintf("(%dk)", tokens/1000)
		}
		val := float64(tokens) / 1000
		s := strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.1f", val), "0"), ".")
		return fmt.Sprintf("(%sk)", s)
	}
	return fmt.Sprintf("(%d)", tokens)
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
		colorPct := compactionProximity(pct, bufferPct)
		return renderContextBar(pct, colorPct, bufferPct > 0)
	}

	// Fall back to legacy calculation for older Claude Code versions
	pct := sl.calculateContextPctLegacy()
	return renderContextBar(pct, pct, bufferPct > 0)
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
		usage.CacheCreationTokens
	pct := (totalTokens * 100) / usableCapacity
	if pct > 100 {
		pct = 100
	}
	return pct
}

// compactionProximity scales raw usage percentage to reflect how close it is
// to the autocompact trigger point. When bufferPct is 0 (disabled), returns rawPct unchanged.
func compactionProximity(rawPct int, bufferPct float64) int {
	if bufferPct <= 0 {
		return rawPct
	}
	if bufferPct >= 100 {
		return 100
	}
	proximity := float64(rawPct) * 100.0 / (100.0 - bufferPct)
	if proximity > 100 {
		return 100
	}
	return int(proximity)
}

func renderContextBar(pct int, colorPct int, showBuffer bool) string {
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
	case colorPct >= 90:
		barColor = colors.Red
	case colorPct >= 70:
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
	gitDir := sl.getEffectiveGitDir()
	added, removed := getGitDiffStats(gitDir)

	return fmt.Sprintf("%s+%d%s %s-%d%s",
		colors.Green, added, colors.Reset,
		colors.Red, removed, colors.Reset)
}

// getEffectiveGitDir returns the best directory for git operations.
// Tries ProjectDir first, falls back to finding git root from CurrentDir.
func (sl *StatusLine) getEffectiveGitDir() string {
	return git.EffectiveDir(
		context.Background(),
		sl.input.Workspace.ProjectDir,
		sl.input.Workspace.CurrentDir,
		statusCache,
	)
}

func getGitDiffStats(projectDir string) (int, int) {
	if projectDir == "" {
		return 0, 0
	}

	// Check cache first
	cacheKey := "diffstats:" + projectDir
	if cached, ok := statusCache.Get(cacheKey); ok {
		var added, removed int
		parts := strings.SplitN(cached, ",", 2)
		if len(parts) == 2 {
			added, _ = strconv.Atoi(parts[0])
			removed, _ = strconv.Atoi(parts[1])
		}
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
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			a, _ := strconv.Atoi(parts[0])
			r, _ := strconv.Atoi(parts[1])
			added += a
			removed += r
		}
	}

	// Cache the result
	statusCache.Set(cacheKey, fmt.Sprintf("%d,%d", added, removed), cache.GitTTL)
	return added, removed
}

func (sl *StatusLine) renderCost() string {
	cost := sl.input.Cost.TotalCostUSD
	costStr := fmt.Sprintf("$%.2f", cost)

	// Add cache efficiency indicator
	showCache := sl.getConfigBool("show_cache", true)
	if showCache {
		usage := sl.input.Context.CurrentUsage
		if ratio, ok := tokens.CacheEfficiency(usage.InputTokens, usage.CacheCreationTokens, usage.CacheReadTokens); ok {
			costStr += fmt.Sprintf(" ⌁%d%%", ratio)
		}
	}

	// Compute burn rate if session ID is available
	showBurnRate := sl.getConfigBool("show_burn_rate", true)
	if showBurnRate && sl.input.SessionID != "" {
		snap, existed, err := burnrate.LoadOrCreateSnapshot(sl.input.SessionID, cost)
		if err == nil && existed {
			if rate, show := burnrate.CalculateRate(snap, cost, time.Now()); show {
				costStr += " " + burnrate.FormatRate(rate)
			}
		}
	}

	return colors.Wrap(colors.Gray, costStr)
}

// getConfigBool reads a bool from config.Plugins["usage"]["api_billing"][key], defaulting to defVal.
func (sl *StatusLine) getConfigBool(key string, defVal bool) bool {
	if sl.config.Plugins == nil {
		return defVal
	}
	usage, ok := sl.config.Plugins["usage"].(map[string]any)
	if !ok {
		return defVal
	}
	billing, ok := usage["api_billing"].(map[string]any)
	if !ok {
		return defVal
	}
	v, ok := billing[key].(bool)
	if !ok {
		return defVal
	}
	return v
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
			Model:               sl.input.Model.DisplayName,
			ContextPct:          sl.calculateContextPct(),
			CostUSD:             sl.input.Cost.TotalCostUSD,
			LinesAdded:          sl.input.Cost.TotalLinesAdded,
			LinesRemoved:        sl.input.Cost.TotalLinesRemoved,
			InputTokens:         sl.input.Context.CurrentUsage.InputTokens,
			OutputTokens:        sl.input.Context.CurrentUsage.OutputTokens,
			CacheCreationTokens: sl.input.Context.CurrentUsage.CacheCreationTokens,
			CacheReadTokens:     sl.input.Context.CurrentUsage.CacheReadTokens,
			ContextWindowSize:   sl.input.Context.ContextWindow,
		},
		Config: sl.getPluginConfig(name),
		Colors: sl.colorsMap,
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
		var pct int
		if sl.input.Context.UsedPercentage > 0 {
			pct = int(sl.input.Context.UsedPercentage)
		} else {
			pct = int(100 - sl.input.Context.RemainingPercentage)
		}
		if pct > 100 {
			pct = 100
		}
		if pct < 0 {
			pct = 0
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
