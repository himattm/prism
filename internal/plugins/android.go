package plugins

import (
	"bytes"
	"context"
	"log"
	"os/exec"
	"regexp"
	"strings"

	"github.com/himattm/prism/internal/cache"
	"github.com/himattm/prism/internal/plugin"
)

// AndroidPlugin shows connected Android devices (via adb)
// Config options:
//   - display: what to show for each device (default: "serial")
//     Options: serial, model, version, sdk, manufacturer, device, build, arch
//     Combine with colons: "model:version", "device:sdk:build"
//   - packages: array of package names for version lookup (supports wildcards)
type AndroidPlugin struct {
	cache *cache.Cache
}

type androidConfig struct {
	Display  string   // What to display: "serial", "model", "version", "model:version"
	Packages []string // Package names to look up versions
}

func (p *AndroidPlugin) Name() string {
	return "android_devices"
}

func (p *AndroidPlugin) SetCache(c *cache.Cache) {
	p.cache = c
}

// OnHook invalidates cache when Claude becomes idle (fresh data on next render)
func (p *AndroidPlugin) OnHook(ctx context.Context, hookType HookType, hookCtx HookContext) (string, error) {
	if hookType == HookIdle && p.cache != nil {
		// Delete all android cache entries (any display config)
		p.cache.DeleteByPrefix("android:")
	}
	return "", nil
}

func (p *AndroidPlugin) Execute(ctx context.Context, input plugin.Input) (string, error) {
	// Parse config first (needed for cache key)
	cfg := parseAndroidConfig(input.Config)

	// Include display config in cache key so config changes invalidate cache
	cacheKey := "android:" + cfg.Display

	// Check cache first
	if p.cache != nil {
		if cached, ok := p.cache.Get(cacheKey); ok {
			return cached, nil
		}
	}

	// Check if adb is available
	if _, err := exec.LookPath("adb"); err != nil {
		return "", nil
	}

	// Get connected devices
	cmd := exec.CommandContext(ctx, "adb", "devices")
	var out bytes.Buffer
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		return "", nil
	}

	// Parse output - just get serial numbers
	serials := parseAdbSerials(out.String())
	if len(serials) == 0 {
		return "", nil
	}

	// Format output (dim emerald green for Android devices)
	dim := input.Colors["dim"]
	green := input.Colors["emerald"]
	gray := input.Colors["gray"]
	reset := input.Colors["reset"]

	var parts []string
	for _, serial := range serials {
		// Get display string based on config
		display := getDeviceDisplay(ctx, serial, cfg.Display)

		// Color the entire device entry uniformly (dim + emerald)
		deviceStr := dim + green + "⬡ " + display

		// Look up app version if packages configured
		if len(cfg.Packages) > 0 {
			if version := getAppVersion(ctx, serial, cfg.Packages); version != "" {
				deviceStr += " " + gray + version + green
			}
		}

		deviceStr += reset
		parts = append(parts, deviceStr)
	}

	// No prefix - just the devices (hexagon icon denotes Android)
	result := strings.Join(parts, " ")

	// Cache for 5 seconds
	if p.cache != nil {
		p.cache.Set(cacheKey, result, 5*cache.ProcessTTL)
	}

	return result, nil
}

// Valid display fields
var validDisplayFields = map[string]bool{
	"serial":       true,
	"model":        true,
	"version":      true,
	"sdk":          true,
	"manufacturer": true,
	"device":       true,
	"build":        true,
	"arch":         true,
}

func isValidDisplay(display string) bool {
	fields := strings.Split(display, ":")
	for _, field := range fields {
		if !validDisplayFields[field] {
			return false
		}
	}
	return true
}

var validPkgRegex = regexp.MustCompile(`^[a-zA-Z0-9._*\-]+$`)

func parseAndroidConfig(cfg map[string]any) androidConfig {
	result := androidConfig{
		Display: "serial", // Default to full serial
	}

	androidCfg, ok := cfg["android_devices"].(map[string]any)
	if !ok {
		return result
	}

	if display, ok := androidCfg["display"].(string); ok {
		// Validate that all fields in the display are valid
		if isValidDisplay(display) {
			result.Display = display
		}
	}

	// Legacy support: displayMode -> display
	if mode, ok := androidCfg["displayMode"].(string); ok {
		if mode == "model" {
			result.Display = "model"
		}
	}

	if packages, ok := androidCfg["packages"].([]any); ok {
		for _, p := range packages {
			if pkg, ok := p.(string); ok {
				// Basic validation to prevent command injection via adb shell
				// Allow letters, numbers, dots, underscores, hyphens, and wildcards
				if validPkgRegex.MatchString(pkg) {
					result.Packages = append(result.Packages, pkg)
				} else {
					// Invalid package name. Log a warning to standard error.
					log.Printf("[Prism] WARNING: Invalid package name in android_devices config: '%s'. Ignored to prevent command injection.", pkg)
				}
			}
		}
	}

	return result
}

func parseAdbSerials(output string) []string {
	var serials []string
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Skip header and empty lines
		if line == "" || strings.HasPrefix(line, "List of") {
			continue
		}

		// Parse "SERIAL\tSTATE" format
		parts := strings.Fields(line)
		if len(parts) >= 2 && parts[1] == "device" {
			serials = append(serials, parts[0])
		}
	}

	return serials
}

// Available display fields:
// - serial: Full device serial (e.g., emulator-5560)
// - model: Device model (e.g., Pixel 6 Pro)
// - version: Android version (e.g., 14)
// - sdk: SDK/API level (e.g., 34)
// - manufacturer: Device manufacturer (e.g., Google)
// - device: Device codename (e.g., cheetah)
// - build: Build type (e.g., userdebug, user)
// - arch: CPU architecture (e.g., arm64-v8a)
//
// Combine with colons: "model:version", "device:sdk", "manufacturer:model:version"
func getDeviceDisplay(ctx context.Context, serial string, display string) string {
	// Handle compound display (e.g., "model:version", "manufacturer:model:version")
	fields := strings.Split(display, ":")
	if len(fields) > 1 {
		return formatCompoundDisplay(ctx, serial, fields)
	}

	// Single field
	value := getDisplayField(ctx, serial, display)
	if value == "" {
		return serial // Fallback
	}
	return value
}

func getDisplayField(ctx context.Context, serial string, field string) string {
	switch field {
	case "serial":
		return serial
	case "model":
		return getDeviceProp(ctx, serial, "ro.product.model")
	case "version":
		return getDeviceProp(ctx, serial, "ro.build.version.release")
	case "sdk":
		return getDeviceProp(ctx, serial, "ro.build.version.sdk")
	case "manufacturer":
		return getDeviceProp(ctx, serial, "ro.product.manufacturer")
	case "device":
		return getDeviceProp(ctx, serial, "ro.product.device")
	case "build":
		return getDeviceProp(ctx, serial, "ro.build.type")
	case "arch":
		return getDeviceProp(ctx, serial, "ro.product.cpu.abi")
	default:
		return ""
	}
}

func formatCompoundDisplay(ctx context.Context, serial string, fields []string) string {
	var values []string
	for _, field := range fields {
		if v := getDisplayField(ctx, serial, field); v != "" {
			values = append(values, v)
		}
	}

	if len(values) == 0 {
		return serial
	}

	// Format: first value, then rest in parentheses
	// e.g., "Pixel 6 (14)" for model:version
	// e.g., "Google Pixel 6 (14)" for manufacturer:model:version
	if len(values) == 1 {
		return values[0]
	}

	return values[0] + " (" + strings.Join(values[1:], " ") + ")"
}

func getDeviceProp(ctx context.Context, serial string, prop string) string {
	cmd := exec.CommandContext(ctx, "adb", "-s", serial, "shell", "getprop", prop)
	var out bytes.Buffer
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		return ""
	}

	result := strings.TrimSpace(out.String())
	// Clean up common prefixes
	result = strings.TrimPrefix(result, "Android SDK built for ")
	return result
}

func getAppVersion(ctx context.Context, serial string, packages []string) string {
	for _, pkg := range packages {
		if strings.Contains(pkg, "*") {
			// Wildcard pattern - find matching package
			actualPkg := findMatchingPackage(ctx, serial, pkg)
			if actualPkg != "" {
				if version := getPackageVersion(ctx, serial, actualPkg); version != "" {
					return version
				}
			}
		} else {
			// Exact package name
			if version := getPackageVersion(ctx, serial, pkg); version != "" {
				return version
			}
		}
	}
	return ""
}

func findMatchingPackage(ctx context.Context, serial string, pattern string) string {
	cmd := exec.CommandContext(ctx, "adb", "-s", serial, "shell", "pm", "list", "packages")
	var out bytes.Buffer
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		return ""
	}

	// Convert glob pattern to regex
	regexPattern := "^" + regexp.QuoteMeta(pattern)
	regexPattern = strings.ReplaceAll(regexPattern, `\*`, ".*")
	regexPattern += "$"
	re, err := regexp.Compile(regexPattern)
	if err != nil {
		return ""
	}

	lines := strings.Split(out.String(), "\n")
	for _, line := range lines {
		line = strings.TrimPrefix(strings.TrimSpace(line), "package:")
		if re.MatchString(line) {
			return line
		}
	}

	return ""
}

func getPackageVersion(ctx context.Context, serial string, pkg string) string {
	cmd := exec.CommandContext(ctx, "adb", "-s", serial, "shell", "dumpsys", "package", pkg)
	var out bytes.Buffer
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		return ""
	}

	// Parse versionName from dumpsys output
	lines := strings.Split(out.String(), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "versionName=") {
			return strings.TrimPrefix(line, "versionName=")
		}
	}

	return ""
}
