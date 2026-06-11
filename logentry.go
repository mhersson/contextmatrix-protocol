package protocol

import "time"

// LogEntry is one frame on a backend's /logs SSE stream (the `data:` JSON
// value). The shape is owned by contextmatrix-runner's logbroadcast package
// historically; field ORDER matters only for the pin test, but field TAGS
// are the wire contract. CM consumes these frames in two places: the chat
// manager's runner-log bridge and the task session-log manager.
type LogEntry struct {
	Timestamp time.Time `json:"ts"`
	CardID    string    `json:"card_id,omitempty"`
	Project   string    `json:"project,omitempty"`
	SessionID string    `json:"session_id,omitempty"`
	// Type is one of: text, thinking, tool_call, user_question, stderr,
	// system, user, usage. "user_question" is LEGACY (no longer emitted);
	// the tag and ToolUseID are retained for wire compatibility with
	// persisted entries.
	Type      string         `json:"type"`
	Content   string         `json:"content,omitempty"`
	ToolUseID string         `json:"tool_use_id,omitempty"`
	Usage     *LogTokenUsage `json:"usage,omitempty"`
	Model     string         `json:"model,omitempty"`
}

// LogTokenUsage carries per-turn token accounting on "usage"-type frames.
// Per-turn, NOT cumulative session totals.
type LogTokenUsage struct {
	InputTokens       int64 `json:"input_tokens"`
	OutputTokens      int64 `json:"output_tokens"`
	CacheReadTokens   int64 `json:"cache_read_tokens"`
	CacheCreateTokens int64 `json:"cache_creation_tokens"`
}
