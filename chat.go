package protocol

// ChatStartPayload is sent by ContextMatrix to start a chat-mode container.
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
	// the entrypoint falls back to the default
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
	// LLMEndpoint is the CM-provisioned inference endpoint configuration.
	// Nil on pre-multi-user CM versions — the chat service falls back to
	// its local llm_endpoint config.
	LLMEndpoint *LLMEndpoint `json:"llm_endpoint,omitempty"`
	// GitToken is a short-lived token for cloning/pushing the project repo,
	// minted by CM from the project's credential binding (or the instance
	// credential when unbound). Superseded before any CM release populated it
	// by GitCredentialsToken + CM's worker credentials endpoint; retained for
	// wire compatibility, never sent by CM.
	GitToken string `json:"git_token,omitempty"`
	// GitTokenExpiresAt is the RFC3339 expiry of GitToken. App-backed tokens
	// live ~1h; backends refresh TriggerPayload's identical field via
	// GET /api/<backend>/git-credentials — chat never gained that loop.
	// PAT-backed tokens carry a zero/absent expiry (the PAT itself).
	// Superseded before any CM release populated it by GitCredentialsToken + CM's
	// worker credentials endpoint; retained for wire compatibility, never sent by CM.
	GitTokenExpiresAt string `json:"git_token_expires_at,omitempty"`
	// GitHost is the bare host the token is scoped to (empty = github.com).
	// Chat needs it explicitly: sessions can be cross-project with no repo URL
	// to derive a host from, unlike TriggerPayload.
	// Superseded before any CM release populated it by GitCredentialsToken + CM's
	// worker credentials endpoint; retained for wire compatibility, never sent by CM.
	GitHost string `json:"git_host,omitempty"`
	// GitCredentialsToken is the per-session bearer the worker presents to CM's
	// GET /api/worker/git-credentials endpoint to fetch per-repo git credentials
	// on demand. Form: "<session_id>.<base64url mac>" — opaque to the backend,
	// forwarded verbatim to the worker. Empty on CM versions that predate
	// worker-fetched credentials — the chat service falls back to its local
	// github config (deprecated).
	GitCredentialsToken string `json:"git_credentials_token,omitempty"`
}

// ChatResumeContext is the rehydration payload wire shape: the prior
// transcript handed to a resumed chat container.
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

// ChatEndPayload is sent by ContextMatrix to close the stdin of a running
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
