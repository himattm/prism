package plugins

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/himattm/prism/internal/cache"
	"github.com/himattm/prism/internal/fsutil"
	"github.com/himattm/prism/internal/plugin"
)

const (
	updateCheckURL  = "https://api.github.com/repos/himattm/prism/releases/latest"
	updateCacheTTL  = 8 * time.Hour
	updateCacheFile = "prism-update-check"
)

// UpdatePlugin shows indicator when Prism update is available
type UpdatePlugin struct {
	cache *cache.Cache
}

type updateCache struct {
	CheckedAt     int64  `json:"checked_at"`
	LocalVersion  string `json:"local_version"`
	RemoteVersion string `json:"remote_version"`
	UpdateAvail   bool   `json:"update_available"`
}

func (p *UpdatePlugin) Name() string {
	return "update"
}

func (p *UpdatePlugin) SetCache(c *cache.Cache) {
	p.cache = c
}

func (p *UpdatePlugin) Execute(ctx context.Context, input plugin.Input) (string, error) {
	// Get config
	enabled := true
	checkInterval := updateCacheTTL
	if cfg, ok := input.Config["update"].(map[string]any); ok {
		if e, ok := cfg["enabled"].(bool); ok {
			enabled = e
		}
		if hours, ok := cfg["check_interval_hours"].(float64); ok {
			checkInterval = time.Duration(hours) * time.Hour
		}
	}

	if !enabled {
		return "", nil
	}

	// Check file-based cache first
	cacheData, cacheExists := loadUpdateCache()

	// Invalidate cache if local version changed (e.g., user manually updated)
	if cacheExists && cacheData.LocalVersion != input.Prism.Version {
		cacheExists = false
	}

	// If cache exists and is fresh, use it
	if cacheExists {
		age := time.Since(time.Unix(cacheData.CheckedAt, 0))
		if age < checkInterval {
			if cacheData.UpdateAvail {
				return formatUpdateIndicator(input.Colors), nil
			}
			return "", nil
		}
	}

	// Only refresh when idle (to avoid blocking during active use)
	if !input.Prism.IsIdle && cacheExists {
		// Return stale cache data while not idle
		if cacheData.UpdateAvail {
			return formatUpdateIndicator(input.Colors), nil
		}
		return "", nil
	}

	// Fetch latest version (with timeout)
	fetchCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	latestVersion, err := fetchLatestVersion(fetchCtx)
	if err != nil {
		// On error, use stale cache if available
		if cacheExists && cacheData.UpdateAvail {
			return formatUpdateIndicator(input.Colors), nil
		}
		return "", nil
	}

	// Compare versions
	currentVersion := input.Prism.Version
	updateAvail := compareVersions(currentVersion, latestVersion) < 0

	// Save to cache (format compatible with prism-update-hook.sh)
	saveUpdateCache(updateCache{
		CheckedAt:     time.Now().Unix(),
		LocalVersion:  currentVersion,
		RemoteVersion: latestVersion,
		UpdateAvail:   updateAvail,
	})

	if updateAvail {
		return formatUpdateIndicator(input.Colors), nil
	}
	return "", nil
}

func formatUpdateIndicator(colors map[string]string) string {
	yellow := colors["yellow"]
	reset := colors["reset"]
	return fmt.Sprintf("%s⬆%s", yellow, reset)
}

func fetchLatestVersion(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", updateCheckURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	// Parse GitHub releases API response
	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", err
	}

	// Strip leading 'v' if present (v0.2.0 -> 0.2.0)
	version := strings.TrimPrefix(release.TagName, "v")
	if version == "" {
		return "", fmt.Errorf("version not found")
	}

	return version, nil
}

func loadUpdateCache() (updateCache, bool) {
	path := filepath.Join(os.TempDir(), updateCacheFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return updateCache{}, false
	}

	var cache updateCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return updateCache{}, false
	}

	return cache, true
}

func saveUpdateCache(c updateCache) {
	path := filepath.Join(os.TempDir(), updateCacheFile)
	data, err := json.Marshal(c)
	if err != nil {
		return
	}
	fsutil.SecureWriteFile(path, data, 0644)
}

// compareVersions compares two semver strings
// Returns -1 if a < b, 0 if a == b, 1 if a > b
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

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

// OnHook implements Hookable interface for auto-update and notifications
func (p *UpdatePlugin) OnHook(ctx context.Context, hookType HookType, hookCtx HookContext) (string, error) {
	// Handle auto-install on idle
	if hookType == HookIdle {
		return p.handleAutoInstall(hookCtx)
	}

	// Handle update notification on busy (user submitting prompt)
	if hookType == HookBusy {
		return p.handleUpdateNotification()
	}

	return "", nil
}

// handleAutoInstall checks for updates and auto-installs if enabled
func (p *UpdatePlugin) handleAutoInstall(hookCtx HookContext) (string, error) {
	// Check auto_install config (default: true)
	autoInstall := true
	if cfg, ok := hookCtx.Config["update"].(map[string]any); ok {
		if ai, ok := cfg["auto_install"].(bool); ok {
			autoInstall = ai
		}
	}
	if !autoInstall {
		return "", nil
	}

	// Check if update is available from cache
	cacheData, exists := loadUpdateCache()
	if !exists || !cacheData.UpdateAvail {
		return "", nil
	}

	// Check if we've already auto-installed this version
	markerFile := filepath.Join(os.TempDir(), "prism-auto-installed")
	if data, err := os.ReadFile(markerFile); err == nil {
		installedVersion := strings.TrimSpace(string(data))
		if installedVersion == cacheData.RemoteVersion {
			return "", nil // Already installed this version
		}
		// Different version - remove stale marker
		os.Remove(markerFile)
	}

	// Check if another instance is already updating (lock file)
	lockFile := filepath.Join(os.TempDir(), "prism-update-lock")
	if _, err := os.Stat(lockFile); err == nil {
		// Lock exists - check if it's stale (older than 2 minutes)
		if info, err := os.Stat(lockFile); err == nil {
			if time.Since(info.ModTime()) < 2*time.Minute {
				return "", nil // Another instance is updating
			}
			// Stale lock, remove it
			os.Remove(lockFile)
		}
	}

	// Create lock file
	if err := fsutil.SecureWriteFile(lockFile, []byte(fmt.Sprintf("%d", os.Getpid())), 0644); err != nil {
		return "", nil
	}

	// Get path to prism binary
	homeDir, err := os.UserHomeDir()
	if err != nil {
		os.Remove(lockFile)
		return "", nil
	}
	prismPath := filepath.Join(homeDir, ".claude", "prism")

	// Copy binary to temp location to avoid file handle conflicts with Claude Code
	// (Claude Code keeps the binary open, which can interfere with spawned processes)
	tempBinary := filepath.Join(os.TempDir(), "prism-updater")
	if err := copyFile(prismPath, tempBinary); err != nil {
		os.Remove(lockFile)
		return "", nil
	}
	if err := os.Chmod(tempBinary, 0755); err != nil {
		os.Remove(lockFile)
		return "", nil
	}

	// Spawn detached process to run "prism update"
	// This survives parent process exit (unlike goroutines)
	// The --auto flag will clean up the lock file when done
	cmd := exec.Command(tempBinary, "update", "--auto")
	cmd.Stderr = nil
	cmd.Stdout = nil
	cmd.Stdin = nil
	// Detach from parent process group
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
	if err := cmd.Start(); err != nil {
		os.Remove(lockFile)
		return "", nil
	}

	// Return notification that update is starting
	cyan := "\033[36m"
	reset := "\033[0m"
	return fmt.Sprintf("%sPrism auto-updating to %s...%s", cyan, cacheData.RemoteVersion, reset), nil
}

// handleUpdateNotification shows a one-per-day notification about available updates
func (p *UpdatePlugin) handleUpdateNotification() (string, error) {
	// Check if we've already prompted today
	promptedFile := filepath.Join(os.TempDir(), "prism-update-prompted")
	if info, err := os.Stat(promptedFile); err == nil {
		age := time.Since(info.ModTime())
		if age < 24*time.Hour {
			return "", nil // Already prompted today
		}
	}

	// Check if update is available from cache
	cacheData, exists := loadUpdateCache()
	if !exists || !cacheData.UpdateAvail {
		return "", nil
	}

	// Mark as prompted
	fsutil.SecureWriteFile(promptedFile, []byte{}, 0644)

	// Return notification message (ANSI colors for terminal)
	cyan := "\033[36m"
	yellow := "\033[33m"
	reset := "\033[0m"
	return fmt.Sprintf("%sPrism update available%s (%s → %s). Run %sprism update%s to upgrade.",
		cyan, reset, cacheData.LocalVersion, cacheData.RemoteVersion, yellow, reset), nil
}
