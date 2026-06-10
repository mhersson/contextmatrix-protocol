package protocol

// ChatStartPayload is received from ContextMatrix to start a chat-mode container.
type ChatStartPayload struct {
	SessionID string `json:"session_id"`
	Project   string `json:"project,omitempty"`
	RepoURL   string `json:"repo_url,omitempty"`
	// MCPAPIKey is forwarded to the container as CM_MCP_API_KEY so the
	// in-container claude can authenticate to CM's MCP endpoint. May be
	// empty when CM's MCP listener has no auth (loopback dev mode); the
	// container then merges an MCP entry with no Authorization header.
	MCPAPIKey string `json:"mcp_api_key,omitempty"`
	// Model is the Claude model the chat container should run. When empty
	// the entrypoint falls back to the historical default
	// (claude-sonnet-4-6). Validated against an allowlist regex; the real
	// allowlist (label, max context tokens) lives on the CM side.
	Model string `json:"model,omitempty"`
	// Resume, when non-nil, is the rehydration payload describing the prior
	// transcript. The runner writes it to /run/cm-chat/resume.jsonl inside
	// the container and sets CM_CHAT_RESUME=1 so the entrypoint switches
	// into the rehydration prompt branch.
	Resume *ChatResumeContext `json:"resume,omitempty"`
	// Primer is the chat-mode orientation text written to the
	// container's stdin as a stream-json user envelope before any
	// rehydration priming. Empty means "no primer" — the handler
	// skips the stdin write. Sourced from CM's
	// workflow-skills/chat-mode.md on each cold open.
	Primer string `json:"primer,omitempty"`
}

// ChatResumeContext mirrors chat.ResumeContext on the CM side. Wire shape
// only — the runner doesn't import CM types.
type ChatResumeContext struct {
	Turns   []ChatResumeTurn `json:"turns"`
	Clipped bool             `json:"clipped"`
	OrigSeq int64            `json:"original_seq"`
}

// ChatResumeTurn is one filtered transcript entry in the rehydration payload.
type ChatResumeTurn struct {
	Seq     int64  `json:"seq"`
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatEndPayload is received from ContextMatrix to close the stdin of a running
// chat container so claude receives EOF and exits.
type ChatEndPayload struct {
	SessionID string `json:"session_id"`
}

// ChatStartResponse is the success body returned by POST /chat/start. ContainerID
// is the Docker container ID assigned by the runtime; CM correlates against this
// when reconciling chat sessions.
type ChatStartResponse struct {
	OK          bool   `json:"ok"`
	ContainerID string `json:"container_id"`
}
