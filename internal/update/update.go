package update

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/himattm/prism/internal/fsutil"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/himattm/prism/internal/version"
)

// PrismHooks defines all hooks that Prism needs wired up
var PrismHooks = []struct {
	Event   string
	Command string
}{
	{"UserPromptSubmit", "$HOME/.claude/prism hook busy"},
	{"Stop", "$HOME/.claude/prism hook idle"},
	{"SessionStart", "$HOME/.claude/prism hook session-start"},
	{"SessionEnd", "$HOME/.claude/prism hook session-end"},
	{"PreCompact", "$HOME/.claude/prism hook pre-compact"},
	{"Setup", "$HOME/.claude/prism hook setup"},
}

const (
	releasesURL = "https://api.github.com/repos/himattm/prism/releases/latest"
)

// Info contains update check results
type Info struct {
	CurrentVersion  string
	LatestVersion   string
	UpdateAvailable bool
}

// Check fetches the latest version and compares with current
func Check(ctx context.Context) (*Info, error) {
	latest, err := fetchLatestVersion(ctx)
	if err != nil {
		return nil, err
	}

	return &Info{
		CurrentVersion:  version.Version,
		LatestVersion:   latest,
		UpdateAvailable: compareVersions(version.Version, latest) < 0,
	}, nil
}

// Download fetches and installs the latest binary
func Download(ctx context.Context) error {
	// Determine binary URL
	osName := runtime.GOOS
	arch := runtime.GOARCH

	binaryURL := fmt.Sprintf("https://github.com/himattm/prism/releases/latest/download/prism-%s-%s", osName, arch)

	// Get the path to current binary
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}
	binaryPath := filepath.Join(homeDir, ".claude", "prism")

	// Download to temp file
	req, err := http.NewRequestWithContext(ctx, "GET", binaryURL, nil)
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("binary not found for %s/%s (release may not include this platform)", osName, arch)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}

	// Write to temp file
	out, err := os.CreateTemp(filepath.Dir(binaryPath), filepath.Base(binaryPath)+".*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tempPath := out.Name()

	_, err = io.Copy(out, resp.Body)
	out.Close()
	if err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("failed to write file: %w", err)
	}

	// Make executable
	if err := os.Chmod(tempPath, 0755); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("failed to chmod: %w", err)
	}

	// Atomic replace
	if err := os.Rename(tempPath, binaryPath); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("failed to install: %w", err)
	}

	return nil
}

func fetchLatestVersion(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", releasesURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return "", fmt.Errorf("no releases found (releases not yet published)")
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", err
	}

	// Strip leading 'v' if present
	ver := strings.TrimPrefix(release.TagName, "v")
	if ver == "" {
		return "", fmt.Errorf("version not found in release")
	}

	return ver, nil
}

func compareVersions(a, b string) int {
	partsA := strings.Split(a, ".")
	partsB := strings.Split(b, ".")

	maxLen := len(partsA)
	if len(partsB) > maxLen {
		maxLen = len(partsB)
	}

	for i := 0; i < maxLen; i++ {
		var numA, numB int
		if i < len(partsA) {
			numA, _ = strconv.Atoi(partsA[i])
		}
		if i < len(partsB) {
			numB, _ = strconv.Atoi(partsB[i])
		}

		if numA < numB {
			return -1
		}
		if numA > numB {
			return 1
		}
	}

	return 0
}

// MigrateSettings ensures all Prism hooks are wired up in settings.json
// Returns the number of hooks added (0 if already up to date)
func MigrateSettings() (int, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return 0, fmt.Errorf("failed to get home directory: %w", err)
	}

	settingsPath := filepath.Join(homeDir, ".claude", "settings.json")

	// Read existing settings
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		if os.IsNotExist(err) {
			// No settings file - nothing to migrate
			return 0, nil
		}
		return 0, fmt.Errorf("failed to read settings: %w", err)
	}

	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		return 0, fmt.Errorf("failed to parse settings: %w", err)
	}

	// Ensure hooks object exists
	hooks, ok := settings["hooks"].(map[string]any)
	if !ok {
		hooks = make(map[string]any)
		settings["hooks"] = hooks
	}

	// Check and add missing Prism hooks
	added := 0
	for _, h := range PrismHooks {
		if !hasPrismHook(hooks, h.Event, h.Command) {
			addPrismHook(hooks, h.Event, h.Command)
			added++
		}
	}

	if added == 0 {
		return 0, nil
	}

	// Write back
	output, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return 0, fmt.Errorf("failed to marshal settings: %w", err)
	}

	if err := fsutil.SecureWriteFile(settingsPath, output, 0644); err != nil {
		return 0, fmt.Errorf("failed to write settings: %w", err)
	}

	return added, nil
}

// hasPrismHook checks if a specific Prism hook command exists for an event
func hasPrismHook(hooks map[string]any, event, command string) bool {
	eventHooks, ok := hooks[event].([]any)
	if !ok {
		return false
	}

	for _, hookGroup := range eventHooks {
		group, ok := hookGroup.(map[string]any)
		if !ok {
			continue
		}

		hookList, ok := group["hooks"].([]any)
		if !ok {
			continue
		}

		for _, hook := range hookList {
			h, ok := hook.(map[string]any)
			if !ok {
				continue
			}
			if h["command"] == command {
				return true
			}
		}
	}

	return false
}

// addPrismHook adds a Prism hook command to an event
func addPrismHook(hooks map[string]any, event, command string) {
	newHook := map[string]any{
		"hooks": []any{
			map[string]any{
				"type":    "command",
				"command": command,
			},
		},
	}

	eventHooks, ok := hooks[event].([]any)
	if !ok {
		eventHooks = []any{}
	}

	hooks[event] = append(eventHooks, newHook)
}
