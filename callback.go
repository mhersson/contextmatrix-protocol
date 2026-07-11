package protocol

// Callback-direction DTOs: bodies the backend POSTs to ContextMatrix's
// per-backend callback endpoint ({callback_path}/status).

// StatusCallbackPayload reports a task's worker-status transition.
// Valid statuses are decided by CM's validator (running/failed/completed);
// the protocol does not constrain them.
type StatusCallbackPayload struct {
	CardID       string `json:"card_id"`
	Project      string `json:"project"`
	WorkerStatus string `json:"worker_status"`
	Message      string `json:"message,omitempty"`
	// Usage, when set, carries the run's usage that the backend could NOT
	// deliver via the primary channel (agent MCP report_usage). CM applies it
	// as a fallback so a poisoned-session teardown does not lose the run's
	// tokens/cost. Absent for backends/runs that reported usage successfully.
	Usage *CallbackUsage `json:"usage,omitempty"`
}

// CallbackUsage is fallback usage attached to a terminal status callback.
type CallbackUsage struct {
	Model            string  `json:"model,omitempty"`
	PromptTokens     int64   `json:"prompt_tokens"`
	CompletionTokens int64   `json:"completion_tokens"`
	CostUSD          float64 `json:"cost_usd"`
}
