package plugins

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/himattm/prism/internal/cache"
	"github.com/himattm/prism/internal/plugin"
)

// JSON response types for `tq list --json`
type tqTask struct {
	ID        int     `json:"id"`
	QueueName string  `json:"queue_name"`
	Status    string  `json:"status"`  // "running" or "waiting"
	Command   *string `json:"command"` // nullable
	PID       int     `json:"pid"`
	ChildPID  int     `json:"child_pid"`
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
}

type tqSummary struct {
	Total   int `json:"total"`
	Running int `json:"running"`
	Waiting int `json:"waiting"`
}

type tqListResponse struct {
	Tasks   []tqTask  `json:"tasks"`
	Summary tqSummary `json:"summary"`
}

type taskQueueConfig struct {
	MaxCommandLength int
	ShowQueueCount   bool
}

// TaskQueuePlugin displays task queue status from the tq CLI
type TaskQueuePlugin struct {
	cache *cache.Cache
}

func (p *TaskQueuePlugin) Name() string {
	return "agent_task_queue"
}

func (p *TaskQueuePlugin) SetCache(c *cache.Cache) {
	p.cache = c
}

// OnHook handles hook events:
//   - HookIdle: invalidates task queue data cache (fresh data on next render)
//   - HookSessionStart: attempts auto-install of tq if not found (runs once per session,
//     outside the render hot path where it would block for up to 30 seconds)
func (p *TaskQueuePlugin) OnHook(ctx context.Context, hookType HookType, hookCtx HookContext) (string, error) {
	switch hookType {
	case HookIdle:
		if p.cache != nil {
			// Only invalidate data cache, NOT the install_attempted flag
			p.cache.DeleteByPrefix("taskqueue:data:")
		}
	case HookSessionStart:
		if _, err := exec.LookPath("tq"); err != nil {
			p.tryAutoInstall(ctx)
		}
	}
	return "", nil
}

func (p *TaskQueuePlugin) Execute(ctx context.Context, input plugin.Input) (string, error) {
	config := parseTaskQueueConfig(input.Config)

	cacheKey := fmt.Sprintf("taskqueue:data:%d:%t", config.MaxCommandLength, config.ShowQueueCount)

	// Check cache first
	if p.cache != nil {
		if cached, ok := p.cache.Get(cacheKey); ok {
			return cached, nil
		}
	}

	// Check if tq is installed (auto-install happens in OnHook at session start)
	tqPath, err := exec.LookPath("tq")
	if err != nil {
		return "", nil
	}

	// Run tq list --json with timeout
	cmdCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, tqPath, "list", "--json")
	var out bytes.Buffer
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		return "", nil
	}

	// Parse JSON response
	var response tqListResponse
	if err := json.Unmarshal(out.Bytes(), &response); err != nil {
		return "", nil
	}

	// Format output
	output := formatTaskQueueOutput(response, config, input.Colors)

	// Cache result
	if p.cache != nil {
		p.cache.Set(cacheKey, output, cache.GitTTL)
	}

	return output, nil
}

func (p *TaskQueuePlugin) tryAutoInstall(ctx context.Context) bool {
	if p.cache == nil {
		return false
	}

	// Check if we've already attempted install this session
	if _, ok := p.cache.Get("taskqueue:install_attempted"); ok {
		return false
	}

	// Mark install as attempted (use very long TTL to persist for session)
	p.cache.Set("taskqueue:install_attempted", "true", 24*time.Hour)

	// Check if uv is available
	uvPath, err := exec.LookPath("uv")
	if err != nil {
		return false
	}

	// Run uv tool install silently with timeout
	installCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(installCtx, uvPath, "tool", "install", "agent-task-queue")
	_ = cmd.Run() // Ignore errors - best effort install

	// Check if tq is now available
	_, err = exec.LookPath("tq")
	return err == nil
}

func parseTaskQueueConfig(config map[string]any) taskQueueConfig {
	cfg := taskQueueConfig{
		MaxCommandLength: 30,
		ShowQueueCount:   true,
	}

	tqConfig, ok := config["agent_task_queue"].(map[string]any)
	if !ok {
		return cfg
	}

	if maxLen, ok := tqConfig["max_command_length"].(float64); ok {
		cfg.MaxCommandLength = int(maxLen)
	}

	if showCount, ok := tqConfig["show_queue_count"].(bool); ok {
		cfg.ShowQueueCount = showCount
	}

	return cfg
}

func formatTaskQueueOutput(response tqListResponse, config taskQueueConfig, colors map[string]string) string {
	if response.Summary.Total == 0 {
		return ""
	}

	powderBlue := colors["powder_blue"]
	red := colors["red"]
	reset := colors["reset"]

	// Error state: waiting tasks but nothing running (queue is stalled)
	if response.Summary.Running == 0 && response.Summary.Waiting > 0 {
		return red + fmt.Sprintf("tq: ! %d waiting (run 'tq clear')", response.Summary.Waiting) + reset
	}

	var parts []string
	parts = append(parts, "tq:")

	// Find running task command (if any)
	var runningCmd string
	for _, task := range response.Tasks {
		if task.Status == "running" && task.Command != nil {
			runningCmd = simplifyCommand(*task.Command, config.MaxCommandLength)
			break
		}
	}

	// Add running task display
	if response.Summary.Running > 0 {
		if runningCmd != "" {
			parts = append(parts, fmt.Sprintf("▸ %s", runningCmd))
		} else {
			// Show count when command is null (e.g., MCP tasks)
			parts = append(parts, fmt.Sprintf("▸ %d", response.Summary.Running))
		}
	}

	// Add waiting count
	if config.ShowQueueCount && response.Summary.Waiting > 0 {
		parts = append(parts, fmt.Sprintf("⧗ %d", response.Summary.Waiting))
	}

	return powderBlue + strings.Join(parts, " ") + reset
}

func simplifyCommand(cmd string, maxLen int) string {
	// Remove ./ prefix
	cmd = strings.TrimPrefix(cmd, "./")

	// Simplify gradle commands: gradlew :app:assembleDebug -> gradlew:assembleDebug
	if strings.HasPrefix(cmd, "gradlew ") {
		rest := strings.TrimPrefix(cmd, "gradlew ")
		// Remove leading colons and module prefixes for common patterns
		parts := strings.Fields(rest)
		if len(parts) > 0 {
			task := parts[0]
			// Remove leading module path (e.g., ":app:" or just ":")
			if idx := strings.LastIndex(task, ":"); idx != -1 && idx < len(task)-1 {
				task = task[idx+1:]
			}
			cmd = "gradlew:" + task
		}
	}

	// Truncate if needed
	if len(cmd) > maxLen {
		return cmd[:maxLen-3] + "..."
	}

	return cmd
}
