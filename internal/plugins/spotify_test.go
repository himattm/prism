package plugins

import (
	"context"
	"os/exec"
	"runtime"
	"testing"
	"time"

	"github.com/himattm/prism/internal/cache"
	"github.com/himattm/prism/internal/plugin"
)

func TestSpotifyPlugin_Name(t *testing.T) {
	p := &SpotifyPlugin{}
	if p.Name() != "spotify" {
		t.Errorf("expected name 'spotify', got '%s'", p.Name())
	}
}

func TestParseSpotifyConfig(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]any
		expected spotifyConfig
	}{
		{
			name:  "empty config uses defaults",
			input: map[string]any{},
			expected: spotifyConfig{
				showIcon:       true,
				maxLength:      40,
				format:         "artist_track",
				showWhenPaused: false,
			},
		},
		{
			name: "show_icon false",
			input: map[string]any{
				"spotify": map[string]any{
					"show_icon": false,
				},
			},
			expected: spotifyConfig{
				showIcon:       false,
				maxLength:      40,
				format:         "artist_track",
				showWhenPaused: false,
			},
		},
		{
			name: "custom max_length",
			input: map[string]any{
				"spotify": map[string]any{
					"max_length": float64(30),
				},
			},
			expected: spotifyConfig{
				showIcon:       true,
				maxLength:      30,
				format:         "artist_track",
				showWhenPaused: false,
			},
		},
		{
			name: "track_artist format",
			input: map[string]any{
				"spotify": map[string]any{
					"format": "track_artist",
				},
			},
			expected: spotifyConfig{
				showIcon:       true,
				maxLength:      40,
				format:         "track_artist",
				showWhenPaused: false,
			},
		},
		{
			name: "track_only format",
			input: map[string]any{
				"spotify": map[string]any{
					"format": "track_only",
				},
			},
			expected: spotifyConfig{
				showIcon:       true,
				maxLength:      40,
				format:         "track_only",
				showWhenPaused: false,
			},
		},
		{
			name: "show_when_paused true",
			input: map[string]any{
				"spotify": map[string]any{
					"show_when_paused": true,
				},
			},
			expected: spotifyConfig{
				showIcon:       true,
				maxLength:      40,
				format:         "artist_track",
				showWhenPaused: true,
			},
		},
		{
			name: "all options",
			input: map[string]any{
				"spotify": map[string]any{
					"show_icon":        false,
					"max_length":       float64(50),
					"format":           "track_only",
					"show_when_paused": true,
				},
			},
			expected: spotifyConfig{
				showIcon:       false,
				maxLength:      50,
				format:         "track_only",
				showWhenPaused: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseSpotifyConfig(tt.input)
			if result.showIcon != tt.expected.showIcon {
				t.Errorf("showIcon: expected %v, got %v", tt.expected.showIcon, result.showIcon)
			}
			if result.maxLength != tt.expected.maxLength {
				t.Errorf("maxLength: expected %d, got %d", tt.expected.maxLength, result.maxLength)
			}
			if result.format != tt.expected.format {
				t.Errorf("format: expected %s, got %s", tt.expected.format, result.format)
			}
			if result.showWhenPaused != tt.expected.showWhenPaused {
				t.Errorf("showWhenPaused: expected %v, got %v", tt.expected.showWhenPaused, result.showWhenPaused)
			}
		})
	}
}

func TestFormatSpotifyOutput(t *testing.T) {
	colors := map[string]string{
		"emerald": "[emerald]",
		"gray":    "[gray]",
		"reset":   "[reset]",
	}

	input := plugin.Input{
		Colors: colors,
	}

	tests := []struct {
		name     string
		track    *spotifyTrack
		cfg      spotifyConfig
		expected string
	}{
		{
			name: "playing with default config",
			track: &spotifyTrack{
				Artist:    "Artist Name",
				Title:     "Song Title",
				IsPlaying: true,
			},
			cfg: spotifyConfig{
				showIcon:       true,
				maxLength:      40,
				format:         "artist_track",
				showWhenPaused: false,
			},
			expected: "[emerald]♫ Artist Name - Song Title[reset]",
		},
		{
			name: "paused",
			track: &spotifyTrack{
				Artist:    "Artist Name",
				Title:     "Song Title",
				IsPlaying: false,
			},
			cfg: spotifyConfig{
				showIcon:       true,
				maxLength:      40,
				format:         "artist_track",
				showWhenPaused: true,
			},
			expected: "[gray]⏸ Artist Name - Song Title[reset]",
		},
		{
			name: "no icon",
			track: &spotifyTrack{
				Artist:    "Artist",
				Title:     "Track",
				IsPlaying: true,
			},
			cfg: spotifyConfig{
				showIcon:       false,
				maxLength:      40,
				format:         "artist_track",
				showWhenPaused: false,
			},
			expected: "[emerald]Artist - Track[reset]",
		},
		{
			name: "track_artist format",
			track: &spotifyTrack{
				Artist:    "Artist",
				Title:     "Track",
				IsPlaying: true,
			},
			cfg: spotifyConfig{
				showIcon:       true,
				maxLength:      40,
				format:         "track_artist",
				showWhenPaused: false,
			},
			expected: "[emerald]♫ Track - Artist[reset]",
		},
		{
			name: "track_only format",
			track: &spotifyTrack{
				Artist:    "Artist",
				Title:     "Track",
				IsPlaying: true,
			},
			cfg: spotifyConfig{
				showIcon:       true,
				maxLength:      40,
				format:         "track_only",
				showWhenPaused: false,
			},
			expected: "[emerald]♫ Track[reset]",
		},
		{
			name: "truncation",
			track: &spotifyTrack{
				Artist:    "Very Long Artist Name",
				Title:     "Very Long Track Title",
				IsPlaying: true,
			},
			cfg: spotifyConfig{
				showIcon:       false,
				maxLength:      20,
				format:         "artist_track",
				showWhenPaused: false,
			},
			expected: "[emerald]Very Long Artist Na…[reset]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatSpotifyOutput(input, tt.track, tt.cfg)
			if result != tt.expected {
				t.Errorf("expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestSpotifyPlugin_SetCache(t *testing.T) {
	p := &SpotifyPlugin{}
	c := cache.New()
	p.SetCache(c)

	if p.cache != c {
		t.Error("cache was not set correctly")
	}
}

func TestSpotifyPlugin_OnHook_InvalidatesCache(t *testing.T) {
	p := &SpotifyPlugin{}
	c := cache.New()
	p.SetCache(c)

	// Add an item to cache
	c.Set("spotify:now_playing", "test value", time.Minute)

	// Verify it's there
	if _, ok := c.Get("spotify:now_playing"); !ok {
		t.Fatal("cache item not found before hook")
	}

	// Fire idle hook
	_, err := p.OnHook(context.Background(), HookIdle, HookContext{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Cache should be invalidated
	if _, ok := c.Get("spotify:now_playing"); ok {
		t.Error("cache item should be invalidated after idle hook")
	}
}

func TestSpotifyPlugin_OnHook_OtherHooksIgnored(t *testing.T) {
	p := &SpotifyPlugin{}
	c := cache.New()
	p.SetCache(c)

	// Add an item to cache
	c.Set("spotify:now_playing", "test value", time.Minute)

	// Fire busy hook (not idle)
	_, err := p.OnHook(context.Background(), HookBusy, HookContext{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Cache should NOT be invalidated
	if _, ok := c.Get("spotify:now_playing"); !ok {
		t.Error("cache item should not be invalidated by busy hook")
	}
}

// Integration test - requires Spotify running (macOS only)
func TestSpotifyPlugin_Integration_MacOS(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS integration test, skipping on " + runtime.GOOS)
	}

	// Check if osascript is available
	if _, err := exec.LookPath("osascript"); err != nil {
		t.Skip("osascript not found")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	track := getSpotifyTrackMacOS(ctx)

	// Test just validates the function doesn't panic or error
	// Result may be nil if Spotify is not running
	if track != nil {
		t.Logf("Got track: %s - %s (playing: %v)", track.Artist, track.Title, track.IsPlaying)
	} else {
		t.Log("No track info (Spotify may not be running)")
	}
}

// Integration test - requires playerctl and Spotify (Linux only)
func TestSpotifyPlugin_Integration_Linux(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux integration test, skipping on " + runtime.GOOS)
	}

	// Check if playerctl is available
	if _, err := exec.LookPath("playerctl"); err != nil {
		t.Skip("playerctl not found")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	track := getSpotifyTrackLinux(ctx)

	// Test just validates the function doesn't panic or error
	if track != nil {
		t.Logf("Got track: %s - %s (playing: %v)", track.Artist, track.Title, track.IsPlaying)
	} else {
		t.Log("No track info (Spotify may not be running)")
	}
}

func TestSpotifyPlugin_Execute_Caching(t *testing.T) {
	p := &SpotifyPlugin{}
	c := cache.New()
	p.SetCache(c)

	// Pre-populate cache
	expectedOutput := "[emerald]♫ Cached Artist - Cached Track[reset]"
	c.Set("spotify:now_playing", expectedOutput, time.Minute)

	ctx := context.Background()
	input := plugin.Input{
		Config: map[string]any{},
		Colors: map[string]string{
			"emerald": "[emerald]",
			"gray":    "[gray]",
			"reset":   "[reset]",
		},
	}

	result, err := p.Execute(ctx, input)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Should return cached value
	if result != expectedOutput {
		t.Errorf("expected cached output '%s', got '%s'", expectedOutput, result)
	}
}

func TestSpotifyPlugin_OnHook_NilCache(t *testing.T) {
	p := &SpotifyPlugin{} // No cache set

	// Should not panic
	_, err := p.OnHook(context.Background(), HookIdle, HookContext{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestFormatSpotifyOutput_UnicodeTruncation(t *testing.T) {
	colors := map[string]string{
		"emerald": "[emerald]",
		"gray":    "[gray]",
		"reset":   "[reset]",
	}

	input := plugin.Input{
		Colors: colors,
	}

	// Test with Japanese characters (multi-byte)
	track := &spotifyTrack{
		Artist:    "アーティスト",
		Title:     "タイトル",
		IsPlaying: true,
	}

	cfg := spotifyConfig{
		showIcon:  false,
		maxLength: 10,
		format:    "artist_track",
	}

	result := formatSpotifyOutput(input, track, cfg)

	// Should truncate by rune count, not byte count
	// "アーティスト - タイトル" is 13 runes, should become 9 + "…"
	expected := "[emerald]アーティスト - …[reset]"
	if result != expected {
		t.Errorf("expected '%s', got '%s'", expected, result)
	}
}

func TestFormatSpotifyOutput_MaxLengthZero(t *testing.T) {
	colors := map[string]string{
		"emerald": "[emerald]",
		"reset":   "[reset]",
	}

	input := plugin.Input{
		Colors: colors,
	}

	track := &spotifyTrack{
		Artist:    "Very Long Artist Name That Would Normally Be Truncated",
		Title:     "Very Long Title",
		IsPlaying: true,
	}

	cfg := spotifyConfig{
		showIcon:  false,
		maxLength: 0, // Should disable truncation
		format:    "artist_track",
	}

	result := formatSpotifyOutput(input, track, cfg)

	// Should not truncate
	expected := "[emerald]Very Long Artist Name That Would Normally Be Truncated - Very Long Title[reset]"
	if result != expected {
		t.Errorf("expected '%s', got '%s'", expected, result)
	}
}

func TestFormatSpotifyOutput_EmptyArtist(t *testing.T) {
	colors := map[string]string{
		"emerald": "[emerald]",
		"reset":   "[reset]",
	}

	input := plugin.Input{
		Colors: colors,
	}

	track := &spotifyTrack{
		Artist:    "",
		Title:     "Track Title",
		IsPlaying: true,
	}

	cfg := spotifyConfig{
		showIcon:  true,
		maxLength: 40,
		format:    "artist_track",
	}

	result := formatSpotifyOutput(input, track, cfg)

	// Should handle empty artist gracefully
	expected := "[emerald]♫  - Track Title[reset]"
	if result != expected {
		t.Errorf("expected '%s', got '%s'", expected, result)
	}
}

func TestFormatSpotifyOutput_TrackOnlyFormat(t *testing.T) {
	colors := map[string]string{
		"emerald": "[emerald]",
		"reset":   "[reset]",
	}

	input := plugin.Input{
		Colors: colors,
	}

	track := &spotifyTrack{
		Artist:    "Artist",
		Title:     "Track Title",
		IsPlaying: true,
	}

	cfg := spotifyConfig{
		showIcon:  false,
		maxLength: 40,
		format:    "track_only",
	}

	result := formatSpotifyOutput(input, track, cfg)

	// Should only show track, no artist
	expected := "[emerald]Track Title[reset]"
	if result != expected {
		t.Errorf("expected '%s', got '%s'", expected, result)
	}
}
