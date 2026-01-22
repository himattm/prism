package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/himattm/prism/internal/colors"
	"github.com/himattm/prism/internal/config"
	"github.com/himattm/prism/internal/hooks"
	"github.com/himattm/prism/internal/plugin"
	"github.com/himattm/prism/internal/plugins"
	"github.com/himattm/prism/internal/statusline"
	"github.com/himattm/prism/internal/update"
	"github.com/himattm/prism/internal/version"
)

func main() {
	if len(os.Args) < 2 {
		// No args = status line mode (read JSON from stdin)
		runStatusLine()
		return
	}

	// CLI mode
	switch os.Args[1] {
	case "version", "--version", "-v":
		fmt.Printf("Prism %s (Go)\n", version.Version)

	case "help", "--help", "-h":
		printHelp()

	case "plugin", "plugins":
		handlePluginCommand(os.Args[2:])

	case "update":
		autoMode := len(os.Args) > 2 && os.Args[2] == "--auto"
		handleUpdate(autoMode)

	case "check-update":
		handleCheckUpdate()

	case "init":
		handleInit()

	case "init-global":
		handleInitGlobal()

	case "hook":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "Usage: prism hook <type>")
			fmt.Fprintln(os.Stderr, "Types: idle, busy, session-start, session-end, pre-compact, setup,")
			fmt.Fprintln(os.Stderr, "       pre-tool-use, post-tool-use, permission-request, notification, subagent-stop")
			os.Exit(1)
		}
		handleHook(os.Args[2])

	case "refract":
		handleRefract()

	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
		fmt.Fprintln(os.Stderr, "Run 'prism help' for usage")
		os.Exit(1)
	}
}

func runStatusLine() {
	// Read JSON input from stdin
	var input statusline.Input
	decoder := json.NewDecoder(os.Stdin)
	if err := decoder.Decode(&input); err != nil {
		fmt.Fprintf(os.Stderr, "Error reading input: %v\n", err)
		os.Exit(1)
	}

	// Load config
	cfg := config.Load(input.Workspace.ProjectDir)

	// Build and render status line
	sl := statusline.New(input, cfg)
	output := sl.Render()

	fmt.Print(output)
}

func printHelp() {
	fmt.Printf(`Prism %s - A fast, customizable status line for Claude Code

Usage:
  prism init                  Create .claude/prism.json in current directory
  prism init-global           Create ~/.claude/prism-config.json
  prism update                Check for Prism updates and install
  prism check-update          Check for Prism updates (no install)
  prism version               Show version
  prism refract               Show available colors with prism animation
  prism help                  Show this help

Plugin commands:
  prism plugin list           List installed plugins with versions
  prism plugin add <url>      Install plugin from GitHub/URL
  prism plugin check-updates  Check plugins for updates
  prism plugin update <name>  Update a plugin (or --all)
  prism plugin remove <name>  Remove a plugin

Config precedence (highest to lowest):
  1. .claude/prism.local.json    Your personal overrides (gitignored)
  2. .claude/prism.json          Repo config (commit for your team)
  3. ~/.claude/prism-config.json Global defaults
`, version.Version)
}

func handlePluginCommand(args []string) {
	if len(args) == 0 {
		args = []string{"list"}
	}

	pm := plugin.NewManager()

	switch args[0] {
	case "list", "ls":
		// Get native plugins from registry
		registry := plugins.NewRegistry()
		nativeNames := registry.List()
		nativePlugins := make([]plugin.NativePluginInfo, len(nativeNames))
		for i, name := range nativeNames {
			nativePlugins[i] = plugin.NativePluginInfo{
				Name:    name,
				Version: version.Version,
			}
		}
		pm.List(nativePlugins)

	case "add", "install":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: prism plugin add <url>")
			os.Exit(1)
		}
		if err := pm.Add(args[1]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "check-updates", "check":
		pm.CheckUpdates()

	case "update", "upgrade":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: prism plugin update <name|--all>")
			os.Exit(1)
		}
		if err := pm.Update(args[1]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	case "remove", "uninstall", "rm":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: prism plugin remove <name>")
			os.Exit(1)
		}
		if err := pm.Remove(args[1]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

	default:
		fmt.Printf("Unknown plugin command: %s\n", args[0])
		fmt.Println("Run 'prism plugin' for usage")
		os.Exit(1)
	}
}

func handleUpdate(autoMode bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if !autoMode {
		fmt.Println("Checking for updates...")
	}

	info, err := update.Check(ctx)
	if err != nil {
		if !autoMode {
			fmt.Printf("Current version: %s\n", version.Version)
			fmt.Fprintf(os.Stderr, "\nCannot update: %v\n", err)
		}
		os.Exit(1)
	}

	if !autoMode {
		fmt.Printf("Current version: %s\n", info.CurrentVersion)
		fmt.Printf("Latest version:  %s\n", info.LatestVersion)
	}

	if !info.UpdateAvailable {
		if !autoMode {
			fmt.Println("\nYou're already on the latest version!")
		}
		return
	}

	if !autoMode {
		fmt.Println("\nDownloading update...")
	}

	if err := update.Download(ctx); err != nil {
		if !autoMode {
			fmt.Fprintf(os.Stderr, "Error downloading update: %v\n", err)
		}
		os.Exit(1)
	}

	// Migrate settings.json to add any new hooks
	hooksAdded, err := update.MigrateSettings()
	if err != nil && !autoMode {
		fmt.Fprintf(os.Stderr, "Warning: failed to migrate settings: %v\n", err)
	}

	// Clear the update cache so indicator disappears
	cacheFile := filepath.Join(os.TempDir(), "prism-update-check")
	os.Remove(cacheFile)

	// Clean up lock file (for both auto and manual modes)
	lockFile := filepath.Join(os.TempDir(), "prism-update-lock")
	os.Remove(lockFile)

	// Write marker file for auto-update tracking
	if autoMode {
		markerFile := filepath.Join(os.TempDir(), "prism-auto-installed")
		os.WriteFile(markerFile, []byte(info.LatestVersion), 0644)
	} else {
		fmt.Printf("\nUpdated to %s!\n", info.LatestVersion)
		if hooksAdded > 0 {
			fmt.Printf("Added %d new hook(s) to settings.json\n", hooksAdded)
		}
	}
}

func handleCheckUpdate() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	fmt.Println("Checking for updates...")

	info, err := update.Check(ctx)
	if err != nil {
		fmt.Printf("Current version: %s\n", version.Version)
		fmt.Printf("\nCould not check for updates: %v\n", err)
		fmt.Println("You may be running a development build.")
		return
	}

	fmt.Printf("Current version: %s\n", info.CurrentVersion)
	fmt.Printf("Latest version:  %s\n", info.LatestVersion)

	if info.UpdateAvailable {
		fmt.Println("\nUpdate available! Run 'prism update' to install.")
	} else {
		fmt.Println("\nYou're on the latest version.")
	}
}

func handleInit() {
	if err := config.Init("."); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Created .claude/prism.json")
}

func handleInitGlobal() {
	if err := config.InitGlobal(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Created ~/.claude/prism-config.json")
}

func handleHook(hookType string) {
	// Read raw JSON from stdin (Claude Code provides session info)
	rawInput, _ := io.ReadAll(os.Stdin)

	var input hooks.Input
	if len(rawInput) > 0 {
		if err := json.Unmarshal(rawInput, &input); err != nil {
			// Silent fail for hooks - don't break Claude Code
			input = hooks.Input{}
		}
	}

	manager := hooks.NewManager()

	switch hookType {
	case "idle":
		if err := manager.HandleIdle(input, rawInput); err != nil {
			os.Exit(1)
		}
	case "busy":
		if err := manager.HandleBusy(input, rawInput); err != nil {
			os.Exit(1)
		}
	case "session-start":
		if err := manager.HandleSessionStart(input, rawInput); err != nil {
			os.Exit(1)
		}
	case "session-end":
		if err := manager.HandleSessionEnd(input, rawInput); err != nil {
			os.Exit(1)
		}
	case "pre-compact":
		if err := manager.HandlePreCompact(input, rawInput); err != nil {
			os.Exit(1)
		}
	case "setup":
		if err := manager.HandleSetup(input, rawInput); err != nil {
			os.Exit(1)
		}
	case "pre-tool-use":
		if err := manager.HandlePreToolUse(input, rawInput); err != nil {
			os.Exit(1)
		}
	case "post-tool-use":
		if err := manager.HandlePostToolUse(input, rawInput); err != nil {
			os.Exit(1)
		}
	case "permission-request":
		if err := manager.HandlePermissionRequest(input, rawInput); err != nil {
			os.Exit(1)
		}
	case "notification":
		if err := manager.HandleNotification(input, rawInput); err != nil {
			os.Exit(1)
		}
	case "subagent-stop":
		if err := manager.HandleSubagentStop(input, rawInput); err != nil {
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "Unknown hook type: %s\n", hookType)
		fmt.Fprintln(os.Stderr, "Available: idle, busy, session-start, session-end, pre-compact, setup,")
		fmt.Fprintln(os.Stderr, "           pre-tool-use, post-tool-use, permission-request, notification, subagent-stop")
		os.Exit(1)
	}
}

func handleRefract() {
	colorMap := colors.ColorMap()
	reset := colorMap["reset"]

	// Clear screen
	fmt.Print("\033[2J\033[H")

	// Animate the prism refraction
	animatePrism(colorMap)

	// Group colors by category
	categories := map[string][]string{
		"Reds":        {"maroon", "dark_red", "crimson", "coral", "salmon", "rose", "hot_pink", "deep_pink", "pink", "light_pink"},
		"Oranges":     {"tangerine", "dark_orange", "orange", "light_orange", "peach"},
		"Yellows":     {"olive", "gold", "khaki", "light_yellow"},
		"Greens":      {"dark_green", "forest_green", "sea_green", "emerald", "lime_green", "lime", "spring_green", "mint", "light_green", "pale_green"},
		"Teals/Cyans": {"teal", "dark_cyan", "turquoise", "aqua", "sky_blue", "light_cyan", "powder_blue"},
		"Blues":       {"navy", "dark_blue", "medium_blue", "royal_blue", "dodger_blue", "cornflower_blue", "steel_blue", "slate_blue", "light_blue"},
		"Purples":     {"indigo", "dark_violet", "purple", "violet", "orchid", "plum", "lavender", "mauve"},
		"Grays":       {"black", "dark_gray", "dim_gray", "gray", "light_gray", "silver", "white"},
	}

	categoryOrder := []string{"Reds", "Oranges", "Yellows", "Greens", "Teals/Cyans", "Blues", "Purples", "Grays"}

	// Animate colors appearing
	for i, category := range categoryOrder {
		colorNames := categories[category]

		// Category header with matching color
		headerColor := getHeaderColor(category, colorMap)
		fmt.Printf("%s%s%s\n", headerColor, category, reset)

		// Print colors in rows of 5
		for j, name := range colorNames {
			if code, ok := colorMap[name]; ok {
				fmt.Printf("  %s██%s %-18s", code, reset, name)
				if (j+1)%5 == 0 {
					fmt.Println()
				}
			}
			time.Sleep(15 * time.Millisecond)
		}
		if len(colorNames)%5 != 0 {
			fmt.Println()
		}

		if i < len(categoryOrder)-1 {
			fmt.Println()
		}
	}

	fmt.Println()

	// Print total count with rainbow effect
	countText := fmt.Sprintf("✨ %d colors available", len(colorMap)-1)
	rainbow := []string{colorMap["red"], colorMap["orange"], colorMap["yellow"], colorMap["green"], colorMap["cyan"], colorMap["blue"], colorMap["purple"]}
	for i, c := range countText {
		fmt.Printf("%s%c%s", rainbow[i%len(rainbow)], c, reset)
		time.Sleep(20 * time.Millisecond)
	}
	fmt.Println()
}

func animatePrism(colorMap map[string]string) {
	reset := colorMap["reset"]
	white := colorMap["bright_white"]
	dim := colorMap["dim"]

	// Spectrum colors for the rainbow beam (7 colors)
	spectrum := []string{
		colorMap["red"],
		colorMap["orange"],
		colorMap["yellow"],
		colorMap["green"],
		colorMap["cyan"],
		colorMap["blue"],
		colorMap["purple"],
	}

	// Simple animation - just draw the final frame with slight delays
	// All 7 rays emerge from the prism edges (red at top, purple at base)
	fmt.Printf("                                   %s◇%s%s━━━━━━━▶%s\n", dim, reset, spectrum[0], reset)
	time.Sleep(50 * time.Millisecond)
	fmt.Printf("                                  %s/ \\%s%s━━━━━━▶%s\n", dim, reset, spectrum[1], reset)
	time.Sleep(50 * time.Millisecond)
	fmt.Printf("                                 %s/   \\%s%s━━━━━▶%s\n", dim, reset, spectrum[2], reset)
	time.Sleep(50 * time.Millisecond)
	fmt.Printf("%s━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━▶%s/     \\%s%s━━━━▶%s\n", white, dim, reset, spectrum[3], reset)
	time.Sleep(50 * time.Millisecond)
	fmt.Printf("                               %s/       \\%s%s━━━▶%s\n", dim, reset, spectrum[4], reset)
	time.Sleep(50 * time.Millisecond)
	fmt.Printf("                              %s/         \\%s%s━━▶%s\n", dim, reset, spectrum[5], reset)
	time.Sleep(50 * time.Millisecond)
	fmt.Printf("                             %s/───────────\\%s%s━▶%s\n", dim, reset, spectrum[6], reset)

	time.Sleep(500 * time.Millisecond)

	// Title with rainbow gradient
	fmt.Println()
	title := "P R I S M   C O L O R S"
	for i, c := range title {
		if c == ' ' {
			fmt.Print(" ")
		} else {
			fmt.Printf("%s%c%s", spectrum[i%len(spectrum)], c, reset)
		}
		time.Sleep(30 * time.Millisecond)
	}
	fmt.Println()
	fmt.Println()

	time.Sleep(300 * time.Millisecond)
}

func getHeaderColor(category string, colorMap map[string]string) string {
	switch category {
	case "Reds":
		return colorMap["crimson"]
	case "Oranges":
		return colorMap["orange"]
	case "Yellows":
		return colorMap["gold"]
	case "Greens":
		return colorMap["emerald"]
	case "Teals/Cyans":
		return colorMap["turquoise"]
	case "Blues":
		return colorMap["dodger_blue"]
	case "Purples":
		return colorMap["purple"]
	case "Grays":
		return colorMap["light_gray"]
	default:
		return colorMap["white"]
	}
}
