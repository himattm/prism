package harness

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/himattm/prism/internal/statusline"
)

// Claude implements the Harness interface for Claude Code.
type Claude struct {
	Base
}

func init() {
	homeDir, _ := os.UserHomeDir()
	Register(&Claude{
		Base: NewBase(filepath.Join(homeDir, ".claude")),
	})
}

func (c *Claude) ID() string   { return "claude" }
func (c *Claude) Name() string { return "Claude Code" }

func (c *Claude) ParseInput(raw []byte) (statusline.Input, error) {
	var input statusline.Input
	err := json.Unmarshal(raw, &input)
	return input, err
}

func (c *Claude) DefaultSections() [][]string {
	return [][]string{
		{"dir", "model", "context", "usage", "peakhours", "git"},
		{"supabase", "vercel", "android"},
		{"spotify"},
	}
}

func (c *Claude) Hooks() []HookDef {
	bin := "$HOME/.claude/prism"
	return []HookDef{
		{Event: "UserPromptSubmit", Command: bin + " hook busy"},
		{Event: "Stop", Command: bin + " hook idle", Async: true},
		{Event: "SessionStart", Command: bin + " hook session-start", Async: true},
		{Event: "SessionEnd", Command: bin + " hook session-end", Async: true},
		{Event: "PreCompact", Command: bin + " hook pre-compact"},
		{Event: "Setup", Command: bin + " hook setup"},
		{Event: "PreToolUse", Command: bin + " hook pre-tool-use"},
		{Event: "PostToolUse", Command: bin + " hook post-tool-use", Async: true},
		{Event: "PermissionRequest", Command: bin + " hook permission-request"},
		{Event: "Notification", Command: bin + " hook notification", Async: true},
		{Event: "SubagentStop", Command: bin + " hook subagent-stop", Async: true},
	}
}

func (c *Claude) SetupInstructions() string {
	return `Add the following to ~/.claude/settings.json:

{
  "statusLine": {
    "type": "command",
    "command": "$HOME/.claude/prism"
  },
  "hooks": {
    "UserPromptSubmit": [{"hooks": [{"type": "command", "command": "$HOME/.claude/prism hook busy"}]}],
    "Stop": [{"hooks": [{"type": "command", "command": "$HOME/.claude/prism hook idle", "async": true}]}],
    "SessionStart": [{"hooks": [{"type": "command", "command": "$HOME/.claude/prism hook session-start", "async": true}]}],
    "SessionEnd": [{"hooks": [{"type": "command", "command": "$HOME/.claude/prism hook session-end", "async": true}]}]
  }
}`
}
