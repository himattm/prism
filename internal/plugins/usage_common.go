package plugins

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

const (
	usageAPIURL     = "https://api.anthropic.com/api/oauth/usage"
	usageCacheTTL   = 60 * time.Second
	usageCacheKey   = "usage_data"
	tokenCacheKey   = "oauth_token"
	tokenCacheTTL   = 5 * time.Minute
	usageAPITimeout = 3 * time.Second

	// usageRenderedKey is used to coordinate between usage plugins
	// to prevent duplicate rendering in the same status line refresh
	usageRenderedKey = "usage_rendered"
	usageRenderedTTL = 100 * time.Millisecond

	// usageDiskCacheFile persists usage data across process invocations
	usageDiskCacheFile = "prism-usage-cache"

	// usageDiskCacheTTL is the freshness threshold for the on-disk usage cache.
	// Files newer than this are considered fresh; older files are returned
	// as stale so the caller can render them dimmed.
	usageDiskCacheTTL = 5 * time.Minute
)

// UsageResponse represents the API response from the usage endpoint
type UsageResponse struct {
	FiveHour     *UsageLimit `json:"five_hour"`
	SevenDay     *UsageLimit `json:"seven_day"`
	SevenDayOpus *UsageLimit `json:"seven_day_opus"`
}

// UsageLimit represents a single usage limit with utilization and reset time
type UsageLimit struct {
	Utilization float64 `json:"utilization"`
	ResetsAt    string  `json:"resets_at"`
}

// KeychainCredentials represents the structure stored in macOS Keychain
type KeychainCredentials struct {
	ClaudeAIOAuth *OAuthCredentials `json:"claudeAiOauth"`
}

// OAuthCredentials holds the OAuth token data
type OAuthCredentials struct {
	AccessToken  string   `json:"accessToken"`
	RefreshToken string   `json:"refreshToken"`
	ExpiresAt    int64    `json:"expiresAt"`
	Scopes       []string `json:"scopes"`
}

// GetOAuthToken retrieves the OAuth access token from the system credential store
// Supports macOS (Keychain) and Linux (~/.claude/.credentials.json)
// Note: This is uncached - prefer GetCachedOAuthToken() for repeated calls
func GetOAuthToken() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		return getOAuthTokenMacOS()
	case "linux":
		return getOAuthTokenLinux()
	default:
		return "", fmt.Errorf("OAuth token retrieval not supported on %s", runtime.GOOS)
	}
}

// GetCachedOAuthToken retrieves the OAuth token with caching to avoid
// repeated keychain/filesystem access. Cache TTL is 5 minutes.
func GetCachedOAuthToken(c cacheInterface) (string, error) {
	if c != nil {
		if cached, ok := c.Get(tokenCacheKey); ok {
			return cached, nil
		}
	}

	token, err := GetOAuthToken()
	if err != nil {
		return "", err
	}

	if c != nil {
		c.Set(tokenCacheKey, token, tokenCacheTTL)
	}

	return token, nil
}

// cacheInterface allows GetCachedOAuthToken to work with any cache implementation
type cacheInterface interface {
	Get(key string) (string, bool)
	Set(key string, value string, ttl time.Duration)
}

// getOAuthTokenMacOS retrieves the token from macOS Keychain
func getOAuthTokenMacOS() (string, error) {
	// Use a short timeout to avoid blocking if no credentials exist
	// or if Keychain prompts for user interaction
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "security", "find-generic-password", "-s", "Claude Code-credentials", "-w")
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to retrieve token from keychain: %w", err)
	}

	// Parse the JSON credentials
	var creds KeychainCredentials
	if err := json.Unmarshal(out.Bytes(), &creds); err != nil {
		return "", fmt.Errorf("failed to parse keychain credentials: %w", err)
	}

	if creds.ClaudeAIOAuth == nil || creds.ClaudeAIOAuth.AccessToken == "" {
		return "", fmt.Errorf("no OAuth token found in credentials")
	}

	return creds.ClaudeAIOAuth.AccessToken, nil
}

// getOAuthTokenLinux retrieves the token from ~/.claude/.credentials.json
func getOAuthTokenLinux() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	credPath := filepath.Join(homeDir, ".claude", ".credentials.json")
	data, err := os.ReadFile(credPath)
	if err != nil {
		return "", fmt.Errorf("failed to read credentials file: %w", err)
	}

	// Parse the JSON credentials (same structure as macOS)
	var creds KeychainCredentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return "", fmt.Errorf("failed to parse credentials file: %w", err)
	}

	if creds.ClaudeAIOAuth == nil || creds.ClaudeAIOAuth.AccessToken == "" {
		return "", fmt.Errorf("no OAuth token found in credentials")
	}

	return creds.ClaudeAIOAuth.AccessToken, nil
}

// FetchUsage calls the usage API and returns the current usage data
func FetchUsage(ctx context.Context, token string) (*UsageResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, usageAPITimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", usageAPIURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set required headers
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("anthropic-beta", "oauth-2025-04-20")

	client := &http.Client{Timeout: usageAPITimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch usage: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("usage API returned status %d", resp.StatusCode)
	}

	var usage UsageResponse
	if err := json.NewDecoder(resp.Body).Decode(&usage); err != nil {
		return nil, fmt.Errorf("failed to parse usage response: %w", err)
	}

	return &usage, nil
}

// TimeUntilReset calculates the duration until the reset time
func TimeUntilReset(resetsAt string) (time.Duration, error) {
	resetTime, err := time.Parse(time.RFC3339, resetsAt)
	if err != nil {
		return 0, fmt.Errorf("failed to parse reset time: %w", err)
	}
	return time.Until(resetTime), nil
}

// FormatTimeRemaining formats a duration as days or hours (with optional minutes)
func FormatTimeRemaining(d time.Duration, useDays bool, showMinutes bool) string {
	if d < 0 {
		d = 0
	}

	if useDays {
		// Round up to nearest day
		days := int(d.Hours()/24) + 1
		if d.Hours() <= 24 {
			days = 1
		}
		if days > 7 {
			days = 7
		}
		return fmt.Sprintf("%dd", days)
	}

	// Cap at 5 hours
	if d.Hours() > 5 {
		d = 5 * time.Hour
	}

	hours := int(d.Hours())
	minutes := int(d.Minutes()) - hours*60

	if showMinutes && minutes > 0 {
		return fmt.Sprintf("%dh%dm", hours, minutes)
	}
	if hours == 0 {
		return "0h"
	}
	return fmt.Sprintf("%dh", hours)
}

// UtilizationToBarLevel converts a utilization percentage (0-100) to a bar level (0-7)
func UtilizationToBarLevel(utilization float64) int {
	if utilization <= 0 {
		return 0
	}
	if utilization >= 100 {
		return 7
	}
	// Map 0-100 to 0-7 (8 levels)
	return int(utilization * 8 / 100)
}

// TimeToBarLevel converts time remaining to a bar level (0-7)
// For 5-hour window: 5h = 7, 0h = 0
// For 7-day window: 7d = 7, 0d = 0
func TimeToBarLevel(d time.Duration, maxDuration time.Duration) int {
	if d <= 0 {
		return 0
	}
	if d >= maxDuration {
		return 7
	}
	// Map 0-max to 0-7
	ratio := float64(d) / float64(maxDuration)
	return int(ratio * 8)
}

// BarChars are the Unicode block elements for bar visualization
var BarChars = []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

// LevelToBarChar converts a bar level (0-7) to the corresponding Unicode character
func LevelToBarChar(level int) rune {
	if level < 0 {
		level = 0
	}
	if level > 7 {
		level = 7
	}
	return BarChars[level]
}

// loadUsageCache reads cached usage data from disk (survives across process invocations).
// Returns (data, false, true) if the file is fresh (within usageDiskCacheTTL),
// (data, true, true) if the file exists but is stale, or (nil, false, false) if
// the file is missing or unreadable.
func loadUsageCache() (data *UsageResponse, stale bool, ok bool) {
	path := filepath.Join(os.TempDir(), usageDiskCacheFile)

	info, err := os.Stat(path)
	if err != nil {
		return nil, false, false
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, false, false
	}

	var usage UsageResponse
	if err := json.Unmarshal(raw, &usage); err != nil {
		return nil, false, false
	}

	if time.Since(info.ModTime()) > usageDiskCacheTTL {
		return &usage, true, true
	}

	return &usage, false, true
}

// saveUsageCache writes usage data to disk for cross-invocation persistence
func saveUsageCache(u *UsageResponse) {
	if u == nil {
		return
	}
	path := filepath.Join(os.TempDir(), usageDiskCacheFile)
	data, err := json.Marshal(u)
	if err != nil {
		return
	}
	os.WriteFile(path, data, 0644)
}
