package plugins

import (
	"bytes"
	"context"
	"os/exec"
	"runtime"
	"strings"
	"unicode/utf8"

	"github.com/himattm/prism/internal/cache"
	"github.com/himattm/prism/internal/plugin"
)

// SpotifyPlugin displays the currently playing Spotify track
type SpotifyPlugin struct {
	cache *cache.Cache
}

// spotifyTrack holds track information
type spotifyTrack struct {
	Artist    string
	Title     string
	IsPlaying bool
}

// spotifyConfig holds plugin configuration
type spotifyConfig struct {
	showIcon       bool
	maxLength      int
	format         string // "artist_track", "track_artist", "track_only"
	showWhenPaused bool
}

func (p *SpotifyPlugin) Name() string {
	return "spotify"
}

func (p *SpotifyPlugin) SetCache(c *cache.Cache) {
	p.cache = c
}

// OnHook invalidates Spotify cache when Claude becomes idle
func (p *SpotifyPlugin) OnHook(_ context.Context, hookType HookType, _ HookContext) (string, error) {
	InvalidateCacheOnHook(hookType, HookIdle, p.cache, "spotify:")
	return "", nil
}

func (p *SpotifyPlugin) Execute(ctx context.Context, input plugin.Input) (string, error) {
	cacheKey := "spotify:now_playing"

	// Check cache first
	if p.cache != nil {
		if cached, ok := p.cache.Get(cacheKey); ok {
			return cached, nil
		}
	}

	// Parse configuration
	cfg := parseSpotifyConfig(input.Config)

	// Get track info based on platform
	var track *spotifyTrack
	switch runtime.GOOS {
	case "darwin":
		track = getSpotifyTrackMacOS(ctx)
	case "linux":
		track = getSpotifyTrackLinux(ctx)
	default:
		return "", nil // Unsupported platform
	}

	if track == nil {
		return "", nil
	}

	// Don't show if paused and config says to hide
	if !track.IsPlaying && !cfg.showWhenPaused {
		return "", nil
	}

	// Format output
	output := formatSpotifyOutput(input, track, cfg)

	// Cache result
	if p.cache != nil {
		p.cache.Set(cacheKey, output, cache.SpotifyTTL)
	}

	return output, nil
}

func parseSpotifyConfig(config map[string]any) spotifyConfig {
	cfg := spotifyConfig{
		showIcon:       true,
		maxLength:      40,
		format:         "artist_track",
		showWhenPaused: false,
	}

	spotifyCfg, ok := config["spotify"].(map[string]any)
	if !ok {
		return cfg
	}

	if v, ok := spotifyCfg["show_icon"].(bool); ok {
		cfg.showIcon = v
	}
	if v, ok := spotifyCfg["max_length"].(float64); ok {
		cfg.maxLength = int(v)
	}
	if v, ok := spotifyCfg["format"].(string); ok {
		cfg.format = v
	}
	if v, ok := spotifyCfg["show_when_paused"].(bool); ok {
		cfg.showWhenPaused = v
	}

	return cfg
}

// getSpotifyTrackMacOS uses AppleScript to get track info on macOS
func getSpotifyTrackMacOS(ctx context.Context) *spotifyTrack {
	// Single AppleScript call to get all info (avoids timeout from multiple calls)
	script := `
if application "Spotify" is running then
	tell application "Spotify"
		if player state is stopped then
			return "stopped"
		end if
		set playerState to player state as string
		set trackName to name of current track
		set artistName to artist of current track
		return playerState & "	" & artistName & "	" & trackName
	end tell
else
	return "not_running"
end if`

	cmd := exec.CommandContext(ctx, "osascript", "-e", script)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil
	}

	result := strings.TrimSpace(out.String())
	if result == "not_running" || result == "stopped" {
		return nil
	}

	// Parse tab-separated result: state\tartist\ttitle
	parts := strings.Split(result, "\t")
	if len(parts) != 3 {
		return nil
	}

	state := parts[0]
	if state != "playing" && state != "paused" {
		return nil
	}

	return &spotifyTrack{
		Artist:    parts[1],
		Title:     parts[2],
		IsPlaying: state == "playing",
	}
}

// getSpotifyTrackLinux uses playerctl to get track info on Linux.
// Single command fetches status + metadata together (same pattern as the macOS AppleScript path).
func getSpotifyTrackLinux(ctx context.Context) *spotifyTrack {
	cmd := exec.CommandContext(ctx, "playerctl", "-p", "spotify", "metadata", "--format", "{{status}}\t{{artist}}\t{{title}}")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil // playerctl not installed or Spotify not running
	}

	// Parse tab-separated result: status\tartist\ttitle
	parts := strings.Split(strings.TrimSpace(out.String()), "\t")
	if len(parts) != 3 {
		return nil
	}

	status := parts[0]
	if status != "Playing" && status != "Paused" {
		return nil
	}

	return &spotifyTrack{
		Artist:    parts[1],
		Title:     parts[2],
		IsPlaying: status == "Playing",
	}
}

func formatSpotifyOutput(input plugin.Input, track *spotifyTrack, cfg spotifyConfig) string {
	var result strings.Builder

	// Choose color based on state
	var color string
	if track.IsPlaying {
		color = input.Colors["emerald"]
	} else {
		color = input.Colors["gray"]
	}
	reset := input.Colors["reset"]

	result.WriteString(color)

	// Add icon
	if cfg.showIcon {
		if track.IsPlaying {
			result.WriteString("♫ ")
		} else {
			result.WriteString("⏸ ")
		}
	}

	// Format track info
	var trackText string
	switch cfg.format {
	case "track_artist":
		trackText = track.Title + " - " + track.Artist
	case "track_only":
		trackText = track.Title
	default: // "artist_track"
		trackText = track.Artist + " - " + track.Title
	}

	// Truncate if needed (use rune count for proper unicode handling)
	if cfg.maxLength > 0 && utf8.RuneCountInString(trackText) > cfg.maxLength {
		runes := []rune(trackText)
		trackText = string(runes[:cfg.maxLength-1]) + "…"
	}

	result.WriteString(trackText)
	result.WriteString(reset)

	return result.String()
}
