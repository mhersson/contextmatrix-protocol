package protocol

// Callback-direction DTOs: bodies the backend POSTs to ContextMatrix's
// per-backend callback endpoints ({callback_path}/status, /skill-engaged).

// StatusCallbackPayload reports a task's runner-status transition.
// Valid statuses are decided by CM's validator (running/failed/completed);
// the protocol does not constrain them.
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
