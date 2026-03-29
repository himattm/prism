package plugins

import (
	"context"
	"encoding/json"
	"os/exec"
	"testing"
	"time"

	"github.com/himattm/prism/internal/cache"
	"github.com/himattm/prism/internal/plugin"
)

func TestTaskQueuePlugin_Name(t *testing.T) {
	p := &TaskQueuePlugin{}
	if p.Name() != "agent_task_queue" {
		t.Errorf("expected name 'agent_task_queue', got '%s'", p.Name())
	}
}

func TestParseJSONResponse(t *testing.T) {
	tests := []struct {
		name            string
		json            string
		expectedTotal   int
		expectedRunning int
		expectedWaiting int
		expectedTasks   int
		expectedCmd     string
	}{
		{
			name: "single running task",
			json: `{
				"tasks": [
					{"id": 1, "queue_name": "global", "status": "running", "command": "./gradlew build"}
				],
				"summary": {"total": 1, "running": 1, "waiting": 0}
			}`,
			expectedTotal:   1,
			expectedRunning: 1,
			expectedWaiting: 0,
			expectedTasks:   1,
			expectedCmd:     "./gradlew build",
		},
		{
			name: "running with waiting tasks",
			json: `{
				"tasks": [
					{"id": 1, "queue_name": "global", "status": "running", "command": "pytest tests/"},
					{"id": 2, "queue_name": "global", "status": "waiting", "command": "npm test"},
					{"id": 3, "queue_name": "global", "status": "waiting", "command": "go test ./..."}
				],
				"summary": {"total": 3, "running": 1, "waiting": 2}
			}`,
			expectedTotal:   3,
			expectedRunning: 1,
			expectedWaiting: 2,
			expectedTasks:   3,
			expectedCmd:     "pytest tests/",
		},
		{
			name: "empty queue",
			json: `{
				"tasks": [],
				"summary": {"total": 0, "running": 0, "waiting": 0}
			}`,
			expectedTotal:   0,
			expectedRunning: 0,
			expectedWaiting: 0,
			expectedTasks:   0,
			expectedCmd:     "",
		},
		{
			name: "multiple queues",
			json: `{
				"tasks": [
					{"id": 1, "queue_name": "build", "status": "running", "command": "./gradlew assembleDebug"},
					{"id": 2, "queue_name": "test", "status": "running", "command": "pytest"}
				],
				"summary": {"total": 2, "running": 2, "waiting": 0}
			}`,
			expectedTotal:   2,
			expectedRunning: 2,
			expectedWaiting: 0,
			expectedTasks:   2,
			expectedCmd:     "./gradlew assembleDebug",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var response tqListResponse
			err := json.Unmarshal([]byte(tt.json), &response)
			if err != nil {
				t.Fatalf("failed to parse JSON: %v", err)
			}

			if response.Summary.Total != tt.expectedTotal {
				t.Errorf("Summary.Total: expected %d, got %d", tt.expectedTotal, response.Summary.Total)
			}
			if response.Summary.Running != tt.expectedRunning {
				t.Errorf("Summary.Running: expected %d, got %d", tt.expectedRunning, response.Summary.Running)
			}
			if response.Summary.Waiting != tt.expectedWaiting {
				t.Errorf("Summary.Waiting: expected %d, got %d", tt.expectedWaiting, response.Summary.Waiting)
			}
			if len(response.Tasks) != tt.expectedTasks {
				t.Errorf("Tasks length: expected %d, got %d", tt.expectedTasks, len(response.Tasks))
			}

			// Check first task's command if expected
			if tt.expectedCmd != "" && len(response.Tasks) > 0 {
				if response.Tasks[0].Command == nil {
					t.Error("expected command but got nil")
				} else if *response.Tasks[0].Command != tt.expectedCmd {
					t.Errorf("Command: expected '%s', got '%s'", tt.expectedCmd, *response.Tasks[0].Command)
				}
			}
		})
	}
}

func TestParseTaskQueueConfig(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]any
		expected taskQueueConfig
	}{
		{
			name:  "empty config uses defaults",
			input: map[string]any{},
			expected: taskQueueConfig{
				MaxCommandLength: 30,
				ShowQueueCount:   true,
			},
		},
		{
			name: "custom max_command_length",
			input: map[string]any{
				"agent_task_queue": map[string]any{
					"max_command_length": float64(50),
				},
			},
			expected: taskQueueConfig{
				MaxCommandLength: 50,
				ShowQueueCount:   true,
			},
		},
		{
			name: "show_queue_count disabled",
			input: map[string]any{
				"agent_task_queue": map[string]any{
					"show_queue_count": false,
				},
			},
			expected: taskQueueConfig{
				MaxCommandLength: 30,
				ShowQueueCount:   false,
			},
		},
		{
			name: "all options set",
			input: map[string]any{
				"agent_task_queue": map[string]any{
					"max_command_length": float64(40),
					"show_queue_count":   false,
				},
			},
			expected: taskQueueConfig{
				MaxCommandLength: 40,
				ShowQueueCount:   false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseTaskQueueConfig(tt.input)
			if result.MaxCommandLength != tt.expected.MaxCommandLength {
				t.Errorf("MaxCommandLength: expected %d, got %d", tt.expected.MaxCommandLength, result.MaxCommandLength)
			}
			if result.ShowQueueCount != tt.expected.ShowQueueCount {
				t.Errorf("ShowQueueCount: expected %t, got %t", tt.expected.ShowQueueCount, result.ShowQueueCount)
			}
		})
	}
}

func TestSimplifyCommand(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxLen   int
		expected string
	}{
		{
			name:     "removes ./ prefix",
			input:    "./gradlew build",
			maxLen:   30,
			expected: "gradlew:build",
		},
		{
			name:     "simplifies gradle with module path",
			input:    "gradlew :app:assembleDebug",
			maxLen:   30,
			expected: "gradlew:assembleDebug",
		},
		{
			name:     "simplifies gradle with full path",
			input:    "./gradlew :app:assembleDebug",
			maxLen:   30,
			expected: "gradlew:assembleDebug",
		},
		{
			name:     "truncates long commands",
			input:    "pytest tests/unit/very_long_module_name_test.py",
			maxLen:   20,
			expected: "pytest tests/unit...",
		},
		{
			name:     "keeps short commands intact",
			input:    "sleep 30",
			maxLen:   30,
			expected: "sleep 30",
		},
		{
			name:     "gradle task only",
			input:    "gradlew :assembleDebug",
			maxLen:   30,
			expected: "gradlew:assembleDebug",
		},
		{
			name:     "gradle with deep module path",
			input:    "gradlew :app:feature:ui:compileDebugKotlin",
			maxLen:   40,
			expected: "gradlew:compileDebugKotlin",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := simplifyCommand(tt.input, tt.maxLen)
			if result != tt.expected {
				t.Errorf("expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestFormatTaskQueueOutput(t *testing.T) {
	colors := map[string]string{
		"powder_blue": "\x1b[38;5;152m",
		"red":         "\x1b[31m",
		"reset":       "\x1b[0m",
	}

	noColors := map[string]string{
		"powder_blue": "",
		"red":         "",
		"reset":       "",
	}

	defaultConfig := taskQueueConfig{
		MaxCommandLength: 30,
		ShowQueueCount:   true,
	}

	tests := []struct {
		name     string
		response tqListResponse
		config   taskQueueConfig
		colors   map[string]string
		expected string
	}{
		{
			name: "no tasks returns empty",
			response: tqListResponse{
				Tasks:   []tqTask{},
				Summary: tqSummary{Total: 0, Running: 0, Waiting: 0},
			},
			config:   defaultConfig,
			colors:   noColors,
			expected: "",
		},
		{
			name: "stalled queue shows error",
			response: tqListResponse{
				Tasks: []tqTask{
					{ID: 1, Status: "waiting", Command: strPtr("pytest")},
					{ID: 2, Status: "waiting", Command: strPtr("npm test")},
					{ID: 3, Status: "waiting", Command: strPtr("go test")},
				},
				Summary: tqSummary{Total: 3, Running: 0, Waiting: 3},
			},
			config:   defaultConfig,
			colors:   noColors,
			expected: "tq: ! 3 waiting (run 'tq clear')",
		},
		{
			name: "running task only",
			response: tqListResponse{
				Tasks: []tqTask{
					{ID: 1, Status: "running", Command: strPtr("pytest")},
				},
				Summary: tqSummary{Total: 1, Running: 1, Waiting: 0},
			},
			config:   defaultConfig,
			colors:   noColors,
			expected: "tq: ▸ pytest",
		},
		{
			name: "running task with waiting",
			response: tqListResponse{
				Tasks: []tqTask{
					{ID: 1, Status: "running", Command: strPtr("gradlew :app:assembleDebug")},
					{ID: 2, Status: "waiting", Command: strPtr("pytest")},
					{ID: 3, Status: "waiting", Command: strPtr("npm test")},
				},
				Summary: tqSummary{Total: 3, Running: 1, Waiting: 2},
			},
			config:   defaultConfig,
			colors:   noColors,
			expected: "tq: ▸ gradlew:assembleDebug ⧗ 2",
		},
		{
			name: "show_queue_count disabled",
			response: tqListResponse{
				Tasks: []tqTask{
					{ID: 1, Status: "running", Command: strPtr("pytest")},
					{ID: 2, Status: "waiting", Command: strPtr("npm test")},
				},
				Summary: tqSummary{Total: 2, Running: 1, Waiting: 1},
			},
			config: taskQueueConfig{
				MaxCommandLength: 30,
				ShowQueueCount:   false,
			},
			colors:   noColors,
			expected: "tq: ▸ pytest",
		},
		{
			name: "with colors",
			response: tqListResponse{
				Tasks: []tqTask{
					{ID: 1, Status: "running", Command: strPtr("pytest")},
				},
				Summary: tqSummary{Total: 1, Running: 1, Waiting: 0},
			},
			config:   defaultConfig,
			colors:   colors,
			expected: "\x1b[38;5;152mtq: ▸ pytest\x1b[0m",
		},
		{
			name: "running task with nil command shows count",
			response: tqListResponse{
				Tasks: []tqTask{
					{ID: 1, Status: "running", Command: nil},
				},
				Summary: tqSummary{Total: 1, Running: 1, Waiting: 0},
			},
			config:   defaultConfig,
			colors:   noColors,
			expected: "tq: ▸ 1",
		},
		{
			name: "running task with nil command and waiting",
			response: tqListResponse{
				Tasks: []tqTask{
					{ID: 1, Status: "running", Command: nil},
					{ID: 2, Status: "waiting", Command: strPtr("pytest")},
				},
				Summary: tqSummary{Total: 2, Running: 1, Waiting: 1},
			},
			config:   defaultConfig,
			colors:   noColors,
			expected: "tq: ▸ 1 ⧗ 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatTaskQueueOutput(tt.response, tt.config, tt.colors)
			if result != tt.expected {
				t.Errorf("expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestTaskQueuePlugin_OnHook(t *testing.T) {
	p := &TaskQueuePlugin{}
	c := cache.New()
	p.SetCache(c)

	// Set up cache with data and install flag
	c.Set("taskqueue:data:30:true", "cached_value", 1*time.Minute)
	c.Set("taskqueue:install_attempted", "true", 24*time.Hour)

	ctx := context.Background()
	hookCtx := HookContext{SessionID: "test"}

	// Call OnHook with HookIdle
	_, err := p.OnHook(ctx, HookIdle, hookCtx)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Data cache should be invalidated
	if _, ok := c.Get("taskqueue:data:30:true"); ok {
		t.Error("data cache should be invalidated on HookIdle")
	}

	// Install flag should NOT be invalidated
	if _, ok := c.Get("taskqueue:install_attempted"); !ok {
		t.Error("install_attempted flag should persist after HookIdle")
	}
}

func TestTaskQueuePlugin_OnHook_OtherHooks(t *testing.T) {
	p := &TaskQueuePlugin{}
	c := cache.New()
	p.SetCache(c)

	// Set up cache
	c.Set("taskqueue:data:30:true", "cached_value", 1*time.Minute)

	ctx := context.Background()
	hookCtx := HookContext{SessionID: "test"}

	// Other hooks should not invalidate cache
	otherHooks := []HookType{HookBusy, HookSessionStart, HookSessionEnd}
	for _, hook := range otherHooks {
		_, _ = p.OnHook(ctx, hook, hookCtx)
	}

	// Cache should still exist
	if _, ok := c.Get("taskqueue:data:30:true"); !ok {
		t.Error("cache should not be invalidated by non-Idle hooks")
	}
}

func TestAutoInstallLogic(t *testing.T) {
	p := &TaskQueuePlugin{}
	c := cache.New()
	p.SetCache(c)

	// Without cache, should return false
	pNoCache := &TaskQueuePlugin{}
	ctx := context.Background()
	if pNoCache.tryAutoInstall(ctx) {
		t.Error("should return false without cache")
	}

	// After first attempt, flag should be set
	_ = p.tryAutoInstall(ctx)
	if _, ok := c.Get("taskqueue:install_attempted"); !ok {
		t.Error("install_attempted flag should be set after first attempt")
	}

	// Second attempt should return false due to flag
	if p.tryAutoInstall(ctx) {
		t.Error("should return false on second attempt due to flag")
	}
}

// Integration test - skip if tq not installed
func TestTaskQueuePlugin_Integration(t *testing.T) {
	if _, err := exec.LookPath("tq"); err != nil {
		t.Skip("tq not found, skipping integration test")
	}

	p := &TaskQueuePlugin{}
	p.SetCache(cache.New())

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	input := plugin.Input{
		Config: map[string]any{},
		Colors: map[string]string{
			"powder_blue": "",
			"red":         "",
			"reset":       "",
		},
	}

	// Execute should not error
	_, err := p.Execute(ctx, input)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// Helper function for creating string pointers
func strPtr(s string) *string {
	return &s
}
