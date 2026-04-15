package plugins

import (
	"context"
	"os/exec"
	"testing"
	"time"

	"github.com/himattm/prism/internal/cache"
	"github.com/himattm/prism/internal/plugin"
)

func TestAndroidPlugin_Name(t *testing.T) {
	p := &AndroidPlugin{}
	if p.Name() != "android_devices" {
		t.Errorf("expected name 'android_devices', got '%s'", p.Name())
	}
}

func TestParseAndroidConfig(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]any
		expected androidConfig
	}{
		{
			name:  "empty config defaults to serial",
			input: map[string]any{},
			expected: androidConfig{
				Display:  "serial",
				Packages: nil,
			},
		},
		{
			name: "display option",
			input: map[string]any{
				"android_devices": map[string]any{
					"display": "model:version",
				},
			},
			expected: androidConfig{
				Display:  "model:version",
				Packages: nil,
			},
		},
		{
			name: "packages option",
			input: map[string]any{
				"android_devices": map[string]any{
					"packages": []any{"com.example.app", "com.other.*"},
				},
			},
			expected: androidConfig{
				Display:  "serial",
				Packages: []string{"com.example.app", "com.other.*"},
			},
		},
		{
			name: "legacy displayMode support",
			input: map[string]any{
				"android_devices": map[string]any{
					"displayMode": "model",
				},
			},
			expected: androidConfig{
				Display:  "model",
				Packages: nil,
			},
		},
		{
			name: "invalid display falls back to serial",
			input: map[string]any{
				"android_devices": map[string]any{
					"display": "invalid",
				},
			},
			expected: androidConfig{
				Display:  "serial",
				Packages: nil,
			},
		},
		{
			name: "new display fields work",
			input: map[string]any{
				"android_devices": map[string]any{
					"display": "sdk",
				},
			},
			expected: androidConfig{
				Display:  "sdk",
				Packages: nil,
			},
		},
		{
			name: "compound display with new fields",
			input: map[string]any{
				"android_devices": map[string]any{
					"display": "device:version:build",
				},
			},
			expected: androidConfig{
				Display:  "device:version:build",
				Packages: nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseAndroidConfig(tt.input)
			if result.Display != tt.expected.Display {
				t.Errorf("Display: expected %s, got %s", tt.expected.Display, result.Display)
			}
			if len(result.Packages) != len(tt.expected.Packages) {
				t.Errorf("Packages length: expected %d, got %d", len(tt.expected.Packages), len(result.Packages))
			}
		})
	}
}

func TestParseAdbSerials(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name: "multiple devices",
			input: `List of devices attached
emulator-5560	device
emulator-5562	device
`,
			expected: []string{"emulator-5560", "emulator-5562"},
		},
		{
			name: "no devices",
			input: `List of devices attached

`,
			expected: nil,
		},
		{
			name: "offline device excluded",
			input: `List of devices attached
emulator-5560	device
emulator-5562	offline
`,
			expected: []string{"emulator-5560"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseAdbSerials(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("expected %d devices, got %d", len(tt.expected), len(result))
			}
			for i, expected := range tt.expected {
				if result[i] != expected {
					t.Errorf("device %d: expected %s, got %s", i, expected, result[i])
				}
			}
		})
	}
}

func TestGetDisplayField_Serial(t *testing.T) {
	serial := "test-serial-123"
	props := map[string]string{}

	result := getDisplayField(serial, "serial", props)
	if result != serial {
		t.Errorf("expected %s, got %s", serial, result)
	}
}

func TestGetDisplayField_Unknown(t *testing.T) {
	props := map[string]string{}

	result := getDisplayField("serial", "unknown", props)
	if result != "" {
		t.Errorf("expected empty string for unknown field, got %s", result)
	}
}

func TestGetDisplayField_FromProps(t *testing.T) {
	props := map[string]string{
		"ro.product.model":         "Pixel 6",
		"ro.build.version.release": "14",
		"ro.build.version.sdk":     "34",
		"ro.product.manufacturer":  "Google",
		"ro.product.device":        "cheetah",
		"ro.build.type":            "userdebug",
		"ro.product.cpu.abi":       "arm64-v8a",
	}

	tests := []struct {
		field    string
		expected string
	}{
		{"model", "Pixel 6"},
		{"version", "14"},
		{"sdk", "34"},
		{"manufacturer", "Google"},
		{"device", "cheetah"},
		{"build", "userdebug"},
		{"arch", "arm64-v8a"},
		{"serial", "test-serial"},
	}

	for _, tt := range tests {
		t.Run(tt.field, func(t *testing.T) {
			result := getDisplayField("test-serial", tt.field, props)
			if result != tt.expected {
				t.Errorf("field %s: expected %s, got %s", tt.field, tt.expected, result)
			}
		})
	}
}

func TestParseAllDeviceProps(t *testing.T) {
	// Simulate adb shell getprop output format
	output := `[ro.product.model]: [Pixel 6]
[ro.build.version.release]: [14]
[ro.build.version.sdk]: [34]
[ro.product.manufacturer]: [Google]
[ro.product.device]: [cheetah]
[ro.build.type]: [userdebug]
[ro.product.cpu.abi]: [arm64-v8a]
`
	// Test the parsing logic directly
	props := parseDevicePropsOutput(output)
	if props["ro.product.model"] != "Pixel 6" {
		t.Errorf("expected 'Pixel 6', got '%s'", props["ro.product.model"])
	}
	if props["ro.build.version.sdk"] != "34" {
		t.Errorf("expected '34', got '%s'", props["ro.build.version.sdk"])
	}
	if len(props) != 7 {
		t.Errorf("expected 7 props, got %d", len(props))
	}
}

func TestFormatCompoundDisplay(t *testing.T) {
	serial := "test-serial"
	props := map[string]string{
		"ro.product.model":         "Pixel 6",
		"ro.build.version.release": "14",
	}

	// Test serial-only compound (should work without device)
	result := formatCompoundDisplay(serial, []string{"serial"}, props)
	if result != serial {
		t.Errorf("expected %s, got %s", serial, result)
	}

	// Test with model:version
	result = formatCompoundDisplay(serial, []string{"model", "version"}, props)
	expected := "Pixel 6 (14)"
	if result != expected {
		t.Errorf("expected %s, got %s", expected, result)
	}

	// Test with unknown fields (should fallback to serial)
	result = formatCompoundDisplay(serial, []string{"nonexistent"}, props)
	if result != serial {
		t.Errorf("expected %s for unknown fields, got %s", serial, result)
	}
}

// Integration tests - require connected Android device
func TestAndroidPlugin_Integration(t *testing.T) {
	// Skip if no adb
	if _, err := exec.LookPath("adb"); err != nil {
		t.Skip("adb not found, skipping integration tests")
	}

	// Check for connected devices
	cmd := exec.Command("adb", "devices")
	output, err := cmd.Output()
	if err != nil {
		t.Skip("adb devices failed, skipping integration tests")
	}

	serials := parseAdbSerials(string(output))
	if len(serials) == 0 {
		t.Skip("no Android devices connected, skipping integration tests")
	}

	serial := serials[0]
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Batch-fetch all properties
	props := getAllDeviceProps(ctx, serial)
	if props == nil {
		t.Fatal("getAllDeviceProps returned nil")
	}

	// Test all display fields
	displayFields := []struct {
		field    string
		property string
	}{
		{"model", "ro.product.model"},
		{"version", "ro.build.version.release"},
		{"sdk", "ro.build.version.sdk"},
		{"manufacturer", "ro.product.manufacturer"},
		{"device", "ro.product.device"},
		{"build", "ro.build.type"},
		{"arch", "ro.product.cpu.abi"},
	}

	for _, df := range displayFields {
		t.Run("display_"+df.field, func(t *testing.T) {
			result := getDisplayField(serial, df.field, props)

			// Get expected value directly from device
			cmd := exec.CommandContext(ctx, "adb", "-s", serial, "shell", "getprop", df.property)
			expected, err := cmd.Output()
			if err != nil {
				t.Skipf("could not get property %s: %v", df.property, err)
			}

			expectedStr := trimOutput(string(expected))
			if result != expectedStr {
				t.Errorf("field %s: expected '%s', got '%s'", df.field, expectedStr, result)
			}
		})
	}
}

func TestAndroidPlugin_Execute(t *testing.T) {
	// Skip if no adb
	if _, err := exec.LookPath("adb"); err != nil {
		t.Skip("adb not found")
	}

	p := &AndroidPlugin{}
	p.SetCache(cache.New())

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	input := plugin.Input{
		Config: map[string]any{
			"android_devices": map[string]any{
				"display": "serial",
			},
		},
		Colors: map[string]string{
			"green": "",
			"gray":  "",
			"reset": "",
		},
	}

	result, err := p.Execute(ctx, input)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Check for connected devices
	cmd := exec.Command("adb", "devices")
	output, _ := cmd.Output()
	serials := parseAdbSerials(string(output))

	if len(serials) == 0 {
		// No devices = empty result
		if result != "" {
			t.Errorf("expected empty result with no devices, got: %s", result)
		}
	} else {
		// Should contain device info
		if result == "" {
			t.Error("expected non-empty result with connected devices")
		}
	}
}

func trimOutput(s string) string {
	// Trim whitespace and clean up common prefixes
	s = string([]byte(s))
	for len(s) > 0 && (s[len(s)-1] == '\n' || s[len(s)-1] == '\r' || s[len(s)-1] == ' ') {
		s = s[:len(s)-1]
	}
	return s
}
