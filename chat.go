package protocol

// ChatStartPayload is sent by ContextMatrix to start a chat-mode container.
type ChatStartPayload struct {
	SessionID string `json:"session_id"`
	Project   string `json:"project,omitempty"`
	RepoURL   string `json:"repo_url,omitempty"`
	// WorkerImage is the per-project chat worker image override (the board's
	// remote_execution.chat_worker_image). Empty = the chat service's
	// configured base_image.
	WorkerImage string `json:"worker_image,omitempty"`
	// MCPAPIKey is forwarded to the container as CM_MCP_API_KEY so the
	// in-container worker can authenticate to CM's MCP endpoint. May be
	// empty when CM's MCP listener has no auth (loopback dev mode); the
	// container then merges an MCP entry with no Authorization header.
	MCPAPIKey string `json:"mcp_api_key,omitempty"`
	// Model is the model slug the chat worker should run. CM supplies it on
	// every chat-start (the chat backend has no server-side default).
	Model string `json:"model,omitempty"`
	// Resume, when non-nil, is the rehydration payload describing the prior
	// transcript. The backend writes it to /run/cm-chat/resume.jsonl inside
	// the container and sets CM_CHAT_RESUME=1 so the worker switches into
	// the rehydration prompt branch.
	Resume *ChatResumeContext `json:"resume,omitempty"`
	// LLMEndpoint is the CM-provisioned inference endpoint configuration.
	// The chat backend fail-closed rejects a start without it - there is no
	// local fallback.
	LLMEndpoint *LLMEndpoint `json:"llm_endpoint,omitempty"`
	// GitCredentialsToken is the per-session bearer the worker presents to CM's
	// GET /api/worker/git-credentials endpoint to fetch per-repo git credentials
	// on demand. Form: "<session_id>.<base64url mac>" - opaque to the backend,
	// forwarded verbatim to the worker. The chat backend fail-closed rejects
	// a start without it - there is no local fallback.
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
// chat container so the worker receives EOF and exits.
type ChatEndPayload struct {
	SessionID string `json:"session_id"`
}

// ChatStartResponse is the success body returned by POST /chat/start. ContainerID
// is the Docker container ID assigned by the runtime; CM records it on the
// session.
type ChatStartResponse struct {
	OK          bool   `json:"ok"`
	ContainerID string `json:"container_id"`
}
