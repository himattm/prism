package harness

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/himattm/prism/internal/statusline"
)

// Codex implements the Harness interface for OpenAI's Codex CLI.
// The input schema is based on the proposed external status line protocol
// from https://github.com/openai/codex/pull/10170 and may evolve as
// the feature stabilizes.
type Codex struct {
	Base
}

func init() {
	homeDir, _ := os.UserHomeDir()
	Register(&Codex{
		Base: NewBase(filepath.Join(homeDir, ".codex")),
	})
}

func (c *Codex) ID() string   { return "codex" }
func (c *Codex) Name() string { return "Codex CLI" }

// codexInput represents the JSON structure that Codex CLI passes on stdin.
type codexInput struct {
	SessionID             string         `json:"session_id"`
	CWD                   string         `json:"cwd"`
	Model                 string         `json:"model"`
	ContextWindowPercent  float64        `json:"context_window_percent"`
	ContextWindowUsed     int            `json:"context_window_used_tokens"`
	ContextWindowSize     int            `json:"context_window_size"`
	LinesAdded            int            `json:"lines_added"`
	LinesRemoved          int            `json:"lines_removed"`
	RateLimits            codexRateLimit `json:"rate_limits"`
}

type codexRateLimit struct {
	FiveHourLimitPercent float64 `json:"5h_limit_percent"`
	WeeklyLimitPercent   float64 `json:"7d_limit_percent"`
}

func (c *Codex) ParseInput(raw []byte) (statusline.Input, error) {
	var ci codexInput
	if err := json.Unmarshal(raw, &ci); err != nil {
		return statusline.Input{}, err
	}

	windowSize := ci.ContextWindowSize
	if windowSize == 0 {
		windowSize = 200000
	}

	usedPct := ci.ContextWindowPercent
	if usedPct == 0 && windowSize > 0 && ci.ContextWindowUsed > 0 {
		usedPct = float64(ci.ContextWindowUsed) * 100 / float64(windowSize)
	}

	return statusline.Input{
		SessionID: ci.SessionID,
		Model: statusline.ModelInfo{
			DisplayName: ci.Model,
		},
		Workspace: statusline.WorkspaceInfo{
			ProjectDir: ci.CWD,
			CurrentDir: ci.CWD,
		},
		Cost: statusline.CostInfo{
			TotalLinesAdded:   ci.LinesAdded,
			TotalLinesRemoved: ci.LinesRemoved,
		},
		Context: statusline.ContextInfo{
			ContextWindow:       windowSize,
			UsedPercentage:      usedPct,
			RemainingPercentage: 100 - usedPct,
			CurrentUsage: statusline.ContextUsage{
				InputTokens: ci.ContextWindowUsed,
			},
		},
	}, nil
}

func (c *Codex) DefaultSections() [][]string {
	return [][]string{
		{"dir", "model", "context", "git"},
		{"spotify"},
	}
}

func (c *Codex) Hooks() []HookDef {
	// Codex CLI hook event names are placeholders — to be updated
	// when Codex ships external hook support.
	bin := "$HOME/.codex/prism"
	return []HookDef{
		{Event: "UserPromptSubmit", Command: bin + " hook busy"},
		{Event: "Stop", Command: bin + " hook idle", Async: true},
		{Event: "SessionStart", Command: bin + " hook session-start", Async: true},
		{Event: "SessionEnd", Command: bin + " hook session-end", Async: true},
	}
}

func (c *Codex) SetupInstructions() string {
	return `Add the following to ~/.codex/config.toml:

[tui]
status_line = ["$HOME/.codex/prism"]
status_line_timeout_ms = 800

Then set the environment variable so Prism knows which harness is calling it:

  export PRISM_HARNESS=codex

Or configure Codex to pass PRISM_HARNESS=codex when invoking the status line command.`
}
