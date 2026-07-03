package protocol

// LLMEndpoint is CM-provisioned inference endpoint configuration.
type LLMEndpoint struct {
	// Type selects the dialect: "openrouter" or "openai".
	Type string `json:"type"`
	// BaseURL is the endpoint base (empty = the type's canonical default).
	BaseURL string `json:"base_url,omitempty"`
	// APIKey authenticates inference calls. Treat as a secret: never log.
	APIKey string `json:"api_key,omitempty"`
}
