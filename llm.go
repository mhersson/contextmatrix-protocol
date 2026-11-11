package protocol

// LLMEndpointTypeClaudeCode selects the Claude Code executor: the backend runs
// the `claude` binary on the operator's subscription instead of calling
// an inference endpoint.
const LLMEndpointTypeClaudeCode = "claude_code"

// LLMEndpoint is CM-provisioned inference endpoint configuration.
type LLMEndpoint struct {
	// Type selects the dialect: "openrouter", "openai" or "claude_code".
	Type string `json:"type"`
	// BaseURL is the endpoint base (empty = the type's canonical default).
	BaseURL string `json:"base_url,omitempty"`
	// APIKey authenticates inference calls. Treat as a secret: never log.
	APIKey string `json:"api_key,omitempty"`
	// OAuthToken is the Claude Code subscription token (`claude setup-token`),
	// set only for Type "claude_code", where BaseURL and APIKey are empty.
	// Secret: never log it, never send it to an HTTP endpoint; the backend
	// hands it to the `claude` process environment and nothing else.
	OAuthToken string `json:"oauth_token,omitempty"`
}
