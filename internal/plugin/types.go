package plugin

// Metadata represents plugin header metadata parsed from @-prefixed comments
type Metadata struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
	Author      string `json:"author"`
	Source      string `json:"source"`
	UpdateURL   string `json:"update_url"`
}

// Input is the JSON structure sent to plugins via stdin
type Input struct {
	Prism   PrismContext      `json:"prism"`
	Session SessionContext    `json:"session"`
	Config  map[string]any    `json:"config"`
	Colors  map[string]string `json:"colors"`
}

// PrismContext provides context about the Prism environment
type PrismContext struct {
	Version    string `json:"version"`
	ProjectDir string `json:"project_dir"`
	CurrentDir string `json:"current_dir"`
	SessionID  string `json:"session_id"`
	IsIdle     bool   `json:"is_idle"`
}

// SessionContext provides context about the Claude session
type SessionContext struct {
	Model               string  `json:"model"`
	ContextPct          int     `json:"context_pct"`
	CostUSD             float64 `json:"cost_usd"`
	LinesAdded          int     `json:"lines_added"`
	LinesRemoved        int     `json:"lines_removed"`
	InputTokens         int     `json:"input_tokens"`
	OutputTokens        int     `json:"output_tokens"`
	CacheCreationTokens int     `json:"cache_creation_tokens"`
	CacheReadTokens     int     `json:"cache_read_tokens"`
	ContextWindowSize   int     `json:"context_window_size"`
}

// Plugin represents a discovered plugin
type Plugin struct {
	Name     string
	Path     string
	Metadata Metadata
	IsBinary bool // true for compiled binaries, false for scripts
}
