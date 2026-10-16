package protocol

import "time"

// LogEntry is one frame on a backend's /logs SSE stream (the `data:` JSON
// value). Field ORDER matters only for the pin test; field TAGS are the
// wire contract. CM consumes these frames in two places: the chat manager's
// log bridge and the task session-log manager.
type LogEntry struct {
	Timestamp time.Time `json:"ts"`
	CardID    string    `json:"card_id,omitempty"`
	Project   string    `json:"project,omitempty"`
	SessionID string    `json:"session_id,omitempty"`
	// Type is one of: text, thinking, tool_call, user_question, stderr,
	// system, user, usage, status. "user" is a HITL chat-input message
	// published directly by the backend's /message handler (bypasses the
	// backend's output redaction). "usage" frames carry Usage and Model with
	// empty Content - content-less metadata frames. "user_question" is not
	// emitted; its tag and ToolUseID exist for wire compatibility with
	// persisted entries that still carry it. "status" is an ephemeral
	// run-state frame (Content "working" or "idle") emitted by chat-mode
	// logbridges; consumers translate it to presence state and never
	// persist it as a transcript row.
	Type      string         `json:"type"`
	Content   string         `json:"content,omitempty"`
	ToolUseID string         `json:"tool_use_id,omitempty"`
	Usage     *LogTokenUsage `json:"usage,omitempty"`
	// Model is the slug that produced the frame. Set on usage frames and on
	// text frames (assistant responses and mob discussion utterances); empty
	// elsewhere.
	Model string `json:"model,omitempty"`
	// Agent labels the speaker on mob session discussion frames ("seat-1",
	// "guest-<name>", "moderator", "human"). Empty on every other frame -
	// absent from the wire, so consumers that predate mob sessions see
	// identical bytes.
	Agent string `json:"agent,omitempty"`
	// Prompt marks a text frame as a question or gate the panel renders with
	// reply buttons. Content stays the full readable message, so a consumer
	// without prompt support shows an ordinary message. Only the architect
	// sets it; absent on every other frame.
	Prompt *LogPrompt `json:"prompt,omitempty"`
}

// LogPrompt is the reply-button half of a prompt frame. Consumers never
// interpret a reply: a chosen action goes back as an ordinary user message,
// so the producer's own reply parsing is unchanged.
type LogPrompt struct {
	// ID is opaque to consumers, for example a question number or a gate's
	// stage and version. It labels the prompt and is never matched on.
	ID string `json:"id,omitempty"`
	// FreeText means any typed reply answers the prompt, not only an action's.
	FreeText bool        `json:"free_text,omitempty"`
	Actions  []LogAction `json:"actions,omitempty"`
}

// LogAction is one reply button.
type LogAction struct {
	Label string `json:"label"`
	// Reply is the exact message a click sends.
	Reply string `json:"reply"`
	// Feedback asks for text first and sends Reply, a space, then the text.
	Feedback bool `json:"feedback,omitempty"`
}

// LogTokenUsage carries per-turn token accounting on "usage"-type frames.
// Per-turn, NOT cumulative session totals. InputTokens + CacheReadTokens +
// CacheCreateTokens approximates the prompt size the model actually
// processed; consumers typically display that sum as "context used".
type LogTokenUsage struct {
	InputTokens       int64 `json:"input_tokens"`
	OutputTokens      int64 `json:"output_tokens"`
	CacheReadTokens   int64 `json:"cache_read_tokens"`
	CacheCreateTokens int64 `json:"cache_creation_tokens"`
}
