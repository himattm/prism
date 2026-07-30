package statusline

// Input is the JSON structure received from the host agent (Claude Code or Pi).
type Input struct {
	SessionID string        `json:"session_id"`
	Agent     string        `json:"agent"` // Source agent: "" / "claude-code" (default) or "pi"
	Model     ModelInfo     `json:"model"`
	Workspace WorkspaceInfo `json:"workspace"`
	Cost      CostInfo      `json:"cost"`
	Context   ContextInfo   `json:"context_window"`
}

// ModelInfo contains model information
type ModelInfo struct {
	DisplayName string `json:"display_name"`
}

// WorkspaceInfo contains workspace paths
type WorkspaceInfo struct {
	ProjectDir string `json:"project_dir"`
	CurrentDir string `json:"current_dir"`
}

// CostInfo contains cost and line change information
type CostInfo struct {
	TotalCostUSD      float64 `json:"total_cost_usd"`
	TotalLinesAdded   int     `json:"total_lines_added"`
	TotalLinesRemoved int     `json:"total_lines_removed"`
}

// ContextInfo contains context window usage
type ContextInfo struct {
	CurrentUsage        ContextUsage `json:"current_usage"`
	ContextWindow       int          `json:"context_window_size"`
	UsedPercentage      float64      `json:"used_percentage"`      // New in Claude Code 2.1.6
	RemainingPercentage float64      `json:"remaining_percentage"` // New in Claude Code 2.1.6
}

// ContextUsage contains token counts
type ContextUsage struct {
	InputTokens         int `json:"input_tokens"`
	OutputTokens        int `json:"output_tokens"`
	CacheCreationTokens int `json:"cache_creation_input_tokens"`
	CacheReadTokens     int `json:"cache_read_input_tokens"`
}
