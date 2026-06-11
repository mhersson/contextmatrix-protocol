package protocol

// Callback-direction DTOs: bodies the backend POSTs to ContextMatrix's
// per-backend callback endpoints ({callback_path}/status, /skill-engaged,
// /knowledge-status). Extracted in A2 from the hand-written mirrors in
// contextmatrix-runner internal/callback and CM internal/api.

// StatusCallbackPayload reports a task's runner-status transition.
// Valid statuses are decided by CM's validator (running/failed/completed
// today); the protocol does not constrain them.
type StatusCallbackPayload struct {
	CardID       string `json:"card_id"`
	Project      string `json:"project"`
	RunnerStatus string `json:"runner_status"`
	Message      string `json:"message,omitempty"`
}

// SkillEngagedPayload notifies CM that the in-container agent invoked a
// named skill.
type SkillEngagedPayload struct {
	CardID    string `json:"card_id"`
	Project   string `json:"project"`
	SkillName string `json:"skill_name"`
}

// KnowledgeStatusPayload is the terminal callback for a knowledge-refresh
// job. Rides along with RefreshKnowledgePayload until the KB machinery is
// retired (C3). State is "succeeded" or "failed" as reported by the backend.
type KnowledgeStatusPayload struct {
	Project string `json:"project"`
	Repo    string `json:"repo"`
	State   string `json:"state"`
	Error   string `json:"error,omitempty"`
}
