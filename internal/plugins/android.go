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
func (p *AndroidPlugin) OnHook(_ context.Context, hookType HookType, _ HookContext) (string, error) {
	InvalidateCacheOnHook(hookType, HookIdle, p.cache, "android:")
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
		// Batch-fetch all properties for this device in a single adb call
		props := getAllDeviceProps(ctx, serial)

		// Get display string based on config
		display := getDeviceDisplay(serial, cfg.Display, props)

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

// displayFieldToProp maps display field names to Android system property keys.
var displayFieldToProp = map[string]string{
	"model":        "ro.product.model",
	"version":      "ro.build.version.release",
	"sdk":          "ro.build.version.sdk",
	"manufacturer": "ro.product.manufacturer",
	"device":       "ro.product.device",
	"build":        "ro.build.type",
	"arch":         "ro.product.cpu.abi",
}

// getAllDeviceProps fetches all system properties from a device in a single
// adb call, replacing N individual getprop calls with one bulk dump.
func getAllDeviceProps(ctx context.Context, serial string) map[string]string {
	cmd := exec.CommandContext(ctx, "adb", "-s", serial, "shell", "getprop")
	var out bytes.Buffer
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		return nil
	}

	return parseDevicePropsOutput(out.String())
}

// parseDevicePropsOutput parses the output of `adb shell getprop`.
// Each line has the format: [key]: [value]
func parseDevicePropsOutput(output string) map[string]string {
	props := make(map[string]string)
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "[") {
			continue
		}
		sepIdx := strings.Index(line, "]: [")
		if sepIdx < 0 {
			continue
		}
		key := line[1:sepIdx]
		value := line[sepIdx+4:]
		if strings.HasSuffix(value, "]") {
			value = value[:len(value)-1]
		}
		props[key] = value
	}
	return props
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
func getDeviceDisplay(serial string, display string, props map[string]string) string {
	fields := strings.Split(display, ":")
	if len(fields) > 1 {
		return formatCompoundDisplay(serial, fields, props)
	}

	value := getDisplayField(serial, display, props)
	if value == "" {
		return serial // Fallback
	}
	return value
}

func getDisplayField(serial string, field string, props map[string]string) string {
	if field == "serial" {
		return serial
	}
	propKey, ok := displayFieldToProp[field]
	if !ok {
		return ""
	}
	value := props[propKey]
	value = strings.TrimPrefix(value, "Android SDK built for ")
	return value
}

func formatCompoundDisplay(serial string, fields []string, props map[string]string) string {
	var values []string
	for _, field := range fields {
		if v := getDisplayField(serial, field, props); v != "" {
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
