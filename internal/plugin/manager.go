package plugin

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/himattm/prism/internal/httputil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/himattm/prism/internal/fsutil"
)

var metadataRegex = regexp.MustCompile(`^#\s*@(\w+[-\w]*)\s+(.+)$`)
var versionRegex = regexp.MustCompile(`(?m)^#\s*@version\s+(.+)$`)

// sanitizeFilename ensures a filename cannot be used for path traversal
func sanitizeFilename(name string) string {
	return filepath.Base(filepath.Clean("/" + name))
}

// Manager handles plugin discovery, execution, and management
type Manager struct {
	pluginDir string
}

// NewManager creates a new plugin manager
func NewManager() *Manager {
	homeDir, _ := os.UserHomeDir()
	return &Manager{
		pluginDir: filepath.Join(homeDir, ".claude", "prism-plugins"),
	}
}

// Discover finds all installed plugins (both scripts and binaries)
func (m *Manager) Discover() ([]Plugin, error) {
	if err := os.MkdirAll(m.pluginDir, 0755); err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(m.pluginDir)
	if err != nil {
		return nil, err
	}

	var plugins []Plugin
	seen := make(map[string]bool) // Track plugin names to avoid duplicates

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, "prism-plugin-") {
			continue
		}
		// Skip metadata sidecar files
		if strings.HasSuffix(name, ".json") {
			continue
		}

		path := filepath.Join(m.pluginDir, name) // name from os.ReadDir is already a safe basename
		isBinary := !strings.HasSuffix(name, ".sh")

		var meta Metadata
		var pluginName string

		if isBinary {
			// Binary plugin: try sidecar JSON first
			meta = m.loadBinaryMetadata(path)
			pluginName = strings.TrimPrefix(name, "prism-plugin-")
		} else {
			// Script plugin: parse header comments
			var err error
			meta, err = ParseMetadata(path)
			if err != nil {
				pluginName = strings.TrimPrefix(name, "prism-plugin-")
				pluginName = strings.TrimSuffix(pluginName, ".sh")
				meta = Metadata{Name: pluginName}
			}
			pluginName = strings.TrimPrefix(name, "prism-plugin-")
			pluginName = strings.TrimSuffix(pluginName, ".sh")
		}

		if meta.Name == "" {
			meta.Name = pluginName
		}

		// Skip if we've already seen this plugin name
		if seen[meta.Name] {
			continue
		}
		seen[meta.Name] = true

		plugins = append(plugins, Plugin{
			Name:     meta.Name,
			Path:     path,
			Metadata: meta,
			IsBinary: isBinary,
		})
	}

	return plugins, nil
}

// loadBinaryMetadata loads metadata from sidecar JSON file
func (m *Manager) loadBinaryMetadata(binaryPath string) Metadata {
	jsonPath := binaryPath + ".json"
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		return Metadata{}
	}

	var meta Metadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return Metadata{}
	}
	return meta
}

// saveBinaryMetadata saves metadata to sidecar JSON file
func (m *Manager) saveBinaryMetadata(binaryPath string, meta Metadata) error {
	jsonPath := binaryPath + ".json"
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	return fsutil.SecureWriteFile(jsonPath, data, 0644)
}

// ParseMetadata extracts metadata from plugin header comments
func ParseMetadata(path string) (Metadata, error) {
	file, err := os.Open(path)
	if err != nil {
		return Metadata{}, err
	}
	defer file.Close()

	meta := Metadata{}
	scanner := bufio.NewScanner(file)
	lineCount := 0

	// Regex to match @key value lines

	for scanner.Scan() && lineCount < 20 {
		line := scanner.Text()
		lineCount++

		matches := metadataRegex.FindStringSubmatch(line)
		if len(matches) == 3 {
			key := strings.ToLower(matches[1])
			value := strings.TrimSpace(matches[2])

			switch key {
			case "name":
				meta.Name = value
			case "version":
				meta.Version = value
			case "description":
				meta.Description = value
			case "author":
				meta.Author = value
			case "source":
				meta.Source = value
			case "update-url":
				meta.UpdateURL = value
			}
		}
	}

	return meta, scanner.Err()
}

// Execute runs a plugin and returns its output
func (m *Manager) Execute(p Plugin, input Input, timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, p.Path)

	// Prepare input JSON
	inputJSON, err := json.Marshal(input)
	if err != nil {
		return "", fmt.Errorf("failed to marshal input: %w", err)
	}

	cmd.Stdin = bytes.NewReader(inputJSON)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("plugin timed out")
		}
		return "", fmt.Errorf("plugin error: %w (stderr: %s)", err, stderr.String())
	}

	return strings.TrimRight(stdout.String(), "\n"), nil
}

// NativePluginInfo describes a built-in plugin for listing
type NativePluginInfo struct {
	Name    string
	Version string
}

// List prints all installed plugins (native + community)
func (m *Manager) List(nativePlugins []NativePluginInfo) {
	// Sort native plugins by name
	sort.Slice(nativePlugins, func(i, j int) bool {
		return nativePlugins[i].Name < nativePlugins[j].Name
	})

	// Get community plugins
	communityPlugins, err := m.Discover()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error discovering plugins: %v\n", err)
		return
	}

	// Calculate column widths based on content
	nameWidth := len("NAME")
	for _, np := range nativePlugins {
		if len(np.Name) > nameWidth {
			nameWidth = len(np.Name)
		}
	}
	for _, p := range communityPlugins {
		if len(p.Name) > nameWidth {
			nameWidth = len(p.Name)
		}
	}

	fmt.Println("Installed plugins:")
	fmt.Println()
	fmt.Printf("  %-*s %-10s %-10s %s\n", nameWidth, "NAME", "VERSION", "TYPE", "SOURCE")
	fmt.Printf("  %-*s %-10s %-10s %s\n", nameWidth, "----", "-------", "----", "------")

	// Print native plugins first
	for _, np := range nativePlugins {
		fmt.Printf("  %-*s %-10s %-10s %s\n", nameWidth, np.Name, np.Version, "built-in", "prism")
	}

	// Print community plugins
	for _, p := range communityPlugins {
		ver := p.Metadata.Version
		if ver == "" {
			ver = "?"
		}
		source := p.Metadata.Source
		if source == "" {
			source = "-"
		}
		pluginType := "script"
		if p.IsBinary {
			pluginType = "binary"
		}
		fmt.Printf("  %-*s %-10s %-10s %s\n", nameWidth, p.Name, ver, pluginType, source)
	}

	if len(nativePlugins) == 0 && len(communityPlugins) == 0 {
		fmt.Println("  (no plugins installed)")
	}

	fmt.Println()
	fmt.Printf("Community plugins: %s\n", m.pluginDir)
}

// Add installs a plugin from a URL (supports both binary and script plugins)
func (m *Manager) Add(url string) error {
	// Parse GitHub URL
	if strings.HasPrefix(url, "https://github.com/") {
		parts := strings.Split(strings.TrimPrefix(url, "https://github.com/"), "/")
		if len(parts) >= 2 {
			owner, repo := parts[0], parts[1]
			pluginName := strings.TrimPrefix(repo, "prism-plugin-")

			// Try binary release first
			if err := m.addBinaryPlugin(owner, repo, pluginName); err == nil {
				return nil
			}

			// Fall back to script
			fmt.Println("No binary release found, trying script...")
			return m.addScriptPlugin(owner, repo, pluginName)
		}
	}

	// Direct URL - try to download as-is
	return m.addFromDirectURL(url)
}

// addBinaryPlugin downloads a binary plugin from GitHub releases
func (m *Manager) addBinaryPlugin(owner, repo, pluginName string) error {
	osName := runtime.GOOS
	arch := runtime.GOARCH

	// Try to fetch release info
	releaseURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", owner, repo)
	client := &http.Client{Timeout: 10 * time.Second}

	req, err := http.NewRequest("GET", releaseURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("no releases found")
	}

	var release struct {
		TagName string `json:"tag_name"`
		Assets  []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return err
	}

	// Find binary for our platform
	binaryName := fmt.Sprintf("prism-plugin-%s-%s-%s", pluginName, osName, arch)
	var downloadURL string
	for _, asset := range release.Assets {
		if asset.Name == binaryName {
			downloadURL = asset.BrowserDownloadURL
			break
		}
	}

	if downloadURL == "" {
		return fmt.Errorf("no binary for %s-%s", osName, arch)
	}

	fmt.Printf("Downloading %s (%s-%s)...\n", pluginName, osName, arch)

	// Download binary
	resp, err = client.Get(downloadURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	// Install
	if err := os.MkdirAll(m.pluginDir, 0755); err != nil {
		return err
	}

	pluginName = sanitizeFilename(pluginName)
	destPath := filepath.Join(m.pluginDir, fmt.Sprintf("prism-plugin-%s", pluginName))

	// Check if already installed
	if err := m.checkExistingPlugin(destPath, pluginName); err != nil {
		return err
	}

	if err := fsutil.SecureWriteFile(destPath, content, 0755); err != nil {
		return fmt.Errorf("failed to write plugin: %w", err)
	}

	// Save metadata
	version := strings.TrimPrefix(release.TagName, "v")
	meta := Metadata{
		Name:      pluginName,
		Version:   version,
		Source:    fmt.Sprintf("https://github.com/%s/%s", owner, repo),
		UpdateURL: fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", owner, repo),
	}
	m.saveBinaryMetadata(destPath, meta)

	fmt.Printf("Installed: %s v%s (binary)\n", pluginName, version)
	return nil
}

// addScriptPlugin downloads a script plugin from GitHub
func (m *Manager) addScriptPlugin(owner, repo, pluginName string) error {
	rawURL := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/main/prism-plugin-%s.sh", owner, repo, pluginName)

	fmt.Printf("Fetching script from: %s\n", rawURL)

	client := httputil.SafeClient(30 * time.Second)
	resp, err := client.Get(rawURL)
	if err != nil {
		return fmt.Errorf("failed to fetch plugin: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to fetch plugin: HTTP %d", resp.StatusCode)
	}

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read plugin: %w", err)
	}

	// Validate it's a prism plugin
	if !bytes.Contains(content, []byte("@prism-plugin")) {
		return fmt.Errorf("file doesn't appear to be a Prism plugin (missing @prism-plugin header)")
	}

	// Write to temp file to parse metadata
	tmpFile, err := os.CreateTemp("", "prism-plugin-*")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := tmpFile.Write(content); err != nil {
		tmpFile.Close()
		return err
	}
	tmpFile.Close()

	// Parse metadata
	meta, _ := ParseMetadata(tmpPath)
	if meta.Name == "" {
		meta.Name = pluginName
	}

	// Install
	if err := os.MkdirAll(m.pluginDir, 0755); err != nil {
		return err
	}

	meta.Name = sanitizeFilename(meta.Name)
	destPath := filepath.Join(m.pluginDir, fmt.Sprintf("prism-plugin-%s.sh", meta.Name))

	if err := m.checkExistingPlugin(destPath, meta.Name); err != nil {
		return err
	}

	if err := fsutil.SecureWriteFile(destPath, content, 0755); err != nil {
		return fmt.Errorf("failed to write plugin: %w", err)
	}

	fmt.Printf("Installed: %s v%s (script)\n", meta.Name, meta.Version)
	return nil
}

// addFromDirectURL downloads a plugin from a direct URL
func (m *Manager) addFromDirectURL(rawURL string) error {
	fmt.Printf("Fetching plugin from: %s\n", rawURL)

	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("unsupported URL scheme: %s", parsedURL.Scheme)
	}

	client := httputil.SafeClient(30 * time.Second)
	resp, err := client.Get(parsedURL.String())
	if err != nil {
		return fmt.Errorf("failed to fetch plugin: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to fetch plugin: HTTP %d", resp.StatusCode)
	}

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read plugin: %w", err)
	}

	// Determine if binary or script
	isScript := bytes.Contains(content, []byte("@prism-plugin")) || bytes.HasPrefix(content, []byte("#!"))

	// Extract name from URL
	base := filepath.Base(parsedURL.Path)
	pluginName := strings.TrimPrefix(base, "prism-plugin-")
	pluginName = strings.TrimSuffix(pluginName, ".sh")
	// Remove platform suffix if present
	for _, suffix := range []string{"-darwin-arm64", "-darwin-amd64", "-linux-amd64", "-linux-arm64"} {
		pluginName = strings.TrimSuffix(pluginName, suffix)
	}

	if err := os.MkdirAll(m.pluginDir, 0755); err != nil {
		return err
	}

	pluginName = sanitizeFilename(pluginName)
	var destPath string
	if isScript {
		destPath = filepath.Join(m.pluginDir, fmt.Sprintf("prism-plugin-%s.sh", pluginName))
	} else {
		destPath = filepath.Join(m.pluginDir, fmt.Sprintf("prism-plugin-%s", pluginName))
	}

	if err := m.checkExistingPlugin(destPath, pluginName); err != nil {
		return err
	}

	if err := fsutil.SecureWriteFile(destPath, content, 0755); err != nil {
		return fmt.Errorf("failed to write plugin: %w", err)
	}

	pluginType := "script"
	if !isScript {
		pluginType = "binary"
		// Save basic metadata for binary
		meta := Metadata{Name: pluginName, Source: parsedURL.String()}
		m.saveBinaryMetadata(destPath, meta)
	}

	fmt.Printf("Installed: %s (%s)\n", pluginName, pluginType)
	return nil
}

// checkExistingPlugin prompts user if plugin already exists
func (m *Manager) checkExistingPlugin(destPath, pluginName string) error {
	if _, err := os.Stat(destPath); err == nil {
		fmt.Printf("Plugin '%s' already installed. Overwrite? [y/N] ", pluginName)
		var response string
		fmt.Scanln(&response)
		if strings.ToLower(response) != "y" {
			return fmt.Errorf("cancelled")
		}
	}
	return nil
}

// CheckUpdates checks all plugins for available updates
func (m *Manager) CheckUpdates() {
	plugins, err := m.Discover()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error discovering plugins: %v\n", err)
		return
	}

	fmt.Println("Checking for plugin updates...")
	fmt.Println()

	updatesAvailable := false
	client := &http.Client{Timeout: 5 * time.Second}

	for _, p := range plugins {
		if p.Metadata.UpdateURL == "" {
			fmt.Printf("  %-12s %-10s (no update URL)\n", p.Name, p.Metadata.Version)
			continue
		}

		var remoteVersion string
		var err error

		if p.IsBinary {
			remoteVersion, err = m.checkBinaryVersion(p, client)
		} else {
			remoteVersion, err = m.checkScriptVersion(p, client)
		}

		if err != nil {
			fmt.Printf("  %-12s %-10s (%s)\n", p.Name, p.Metadata.Version, err)
			continue
		}

		if CompareVersions(p.Metadata.Version, remoteVersion) < 0 {
			fmt.Printf("  %-12s %-10s -> %-10s \033[33m(update available)\033[0m\n",
				p.Name, p.Metadata.Version, remoteVersion)
			updatesAvailable = true
		} else {
			fmt.Printf("  %-12s %-10s (up to date)\n", p.Name, p.Metadata.Version)
		}
	}

	fmt.Println()
	if updatesAvailable {
		fmt.Println("Run 'prism plugin update <name>' or 'prism plugin update --all' to update.")
	} else {
		fmt.Println("All plugins are up to date.")
	}
}

func (m *Manager) checkBinaryVersion(p Plugin, client *http.Client) (string, error) {
	req, err := http.NewRequest("GET", p.Metadata.UpdateURL, nil)
	if err != nil {
		return "", fmt.Errorf("request failed")
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch failed")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("no releases")
	}

	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", fmt.Errorf("parse failed")
	}

	return strings.TrimPrefix(release.TagName, "v"), nil
}

func (m *Manager) checkScriptVersion(p Plugin, client *http.Client) (string, error) {
	resp, err := client.Get(p.Metadata.UpdateURL)
	if err != nil {
		return "", fmt.Errorf("fetch failed")
	}
	defer resp.Body.Close()

	content, _ := io.ReadAll(resp.Body)

	matches := versionRegex.FindSubmatch(content)
	if len(matches) < 2 {
		return "", fmt.Errorf("no remote version")
	}

	return strings.TrimSpace(string(matches[1])), nil
}

// Update updates a specific plugin or all plugins
func (m *Manager) Update(target string) error {
	plugins, err := m.Discover()
	if err != nil {
		return err
	}

	if target == "--all" || target == "-a" {
		fmt.Println("Updating all plugins...")
		for _, p := range plugins {
			m.updatePlugin(p)
		}
		return nil
	}

	// Find specific plugin
	for _, p := range plugins {
		if p.Name == target {
			return m.updatePlugin(p)
		}
	}

	return fmt.Errorf("plugin '%s' not found", target)
}

func (m *Manager) updatePlugin(p Plugin) error {
	if p.Metadata.UpdateURL == "" {
		fmt.Printf("  %s: no update URL configured\n", p.Name)
		return nil
	}

	fmt.Printf("  %s: checking...\n", p.Name)

	client := &http.Client{Timeout: 10 * time.Second}

	if p.IsBinary {
		return m.updateBinaryPlugin(p, client)
	}
	return m.updateScriptPlugin(p, client)
}

func (m *Manager) updateBinaryPlugin(p Plugin, client *http.Client) error {
	// UpdateURL for binaries points to GitHub releases API
	req, err := http.NewRequest("GET", p.Metadata.UpdateURL, nil)
	if err != nil {
		fmt.Printf("  %s: request failed\n", p.Name)
		return nil
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("  %s: fetch failed\n", p.Name)
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("  %s: no releases found\n", p.Name)
		return nil
	}

	var release struct {
		TagName string `json:"tag_name"`
		Assets  []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		fmt.Printf("  %s: parse failed\n", p.Name)
		return nil
	}

	remoteVersion := strings.TrimPrefix(release.TagName, "v")
	if CompareVersions(p.Metadata.Version, remoteVersion) >= 0 {
		fmt.Printf("  %s: already up to date (%s)\n", p.Name, p.Metadata.Version)
		return nil
	}

	// Find binary for our platform
	osName := runtime.GOOS
	arch := runtime.GOARCH
	binaryName := fmt.Sprintf("prism-plugin-%s-%s-%s", p.Name, osName, arch)

	var downloadURL string
	for _, asset := range release.Assets {
		if asset.Name == binaryName {
			downloadURL = asset.BrowserDownloadURL
			break
		}
	}

	if downloadURL == "" {
		fmt.Printf("  %s: no binary for %s-%s\n", p.Name, osName, arch)
		return nil
	}

	// Download new binary
	resp, err = client.Get(downloadURL)
	if err != nil {
		fmt.Printf("  %s: download failed\n", p.Name)
		return nil
	}
	defer resp.Body.Close()

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("  %s: read failed\n", p.Name)
		return nil
	}

	if err := fsutil.SecureWriteFile(p.Path, content, 0755); err != nil {
		return fmt.Errorf("failed to update %s: %w", p.Name, err)
	}

	// Update metadata
	p.Metadata.Version = remoteVersion
	m.saveBinaryMetadata(p.Path, p.Metadata)

	fmt.Printf("  %s: updated %s -> %s\n", p.Name, p.Metadata.Version, remoteVersion)
	return nil
}

func (m *Manager) updateScriptPlugin(p Plugin, client *http.Client) error {
	resp, err := client.Get(p.Metadata.UpdateURL)
	if err != nil {
		fmt.Printf("  %s: fetch failed\n", p.Name)
		return nil
	}
	defer resp.Body.Close()

	content, _ := io.ReadAll(resp.Body)

	// Parse remote version
	matches := versionRegex.FindSubmatch(content)
	if len(matches) < 2 {
		fmt.Printf("  %s: no version in remote file\n", p.Name)
		return nil
	}

	remoteVersion := strings.TrimSpace(string(matches[1]))
	if CompareVersions(p.Metadata.Version, remoteVersion) < 0 {
		if err := fsutil.SecureWriteFile(p.Path, content, 0755); err != nil {
			return fmt.Errorf("failed to update %s: %w", p.Name, err)
		}
		fmt.Printf("  %s: updated %s -> %s\n", p.Name, p.Metadata.Version, remoteVersion)
	} else {
		fmt.Printf("  %s: already up to date (%s)\n", p.Name, p.Metadata.Version)
	}

	return nil
}

// Remove uninstalls a plugin (handles both binaries and scripts)
func (m *Manager) Remove(name string) error {
	safeName := sanitizeFilename(name)
	// Try binary first, then script
	binaryPath := filepath.Join(m.pluginDir, fmt.Sprintf("prism-plugin-%s", safeName))
	scriptPath := filepath.Join(m.pluginDir, fmt.Sprintf("prism-plugin-%s.sh", safeName))

	var path string
	if _, err := os.Stat(binaryPath); err == nil {
		path = binaryPath
		// Also remove sidecar metadata
		os.Remove(binaryPath + ".json")
	} else if _, err := os.Stat(scriptPath); err == nil {
		path = scriptPath
	} else {
		return fmt.Errorf("plugin '%s' not found", name)
	}

	if err := os.Remove(path); err != nil {
		return fmt.Errorf("failed to remove plugin: %w", err)
	}

	fmt.Printf("Removed: %s\n", name)
	return nil
}

// CompareVersions compares two semver strings
// Returns -1 if a < b, 0 if a == b, 1 if a > b
func CompareVersions(a, b string) int {
	partsA := strings.Split(a, ".")
	partsB := strings.Split(b, ".")

	maxLen := len(partsA)
	if len(partsB) > maxLen {
		maxLen = len(partsB)
	}

	for i := 0; i < maxLen; i++ {
		var numA, numB int
		if i < len(partsA) {
			fmt.Sscanf(partsA[i], "%d", &numA)
		}
		if i < len(partsB) {
			fmt.Sscanf(partsB[i], "%d", &numB)
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
