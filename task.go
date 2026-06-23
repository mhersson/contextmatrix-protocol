package protocol

// TriggerPayload is sent by ContextMatrix to start a task.
type TriggerPayload struct {
	CardID      string    `json:"card_id"`
	Project     string    `json:"project"`
	RepoURL     string    `json:"repo_url"`
	MCPAPIKey   string    `json:"mcp_api_key,omitempty"`
	BaseBranch  string    `json:"base_branch,omitempty"`
	RunnerImage string    `json:"runner_image,omitempty"`
	Interactive bool      `json:"interactive,omitempty"`
	Model       string    `json:"model,omitempty"`
	TaskSkills  *[]string `json:"task_skills,omitempty"`
	// Selection carries auto-selection inputs for the agent backend
	// (candidates, favorites, blacklist). Nil for the runner backend.
	Selection *SelectionContext `json:"selection,omitempty"`
}

// KillPayload is sent by ContextMatrix to stop a specific task.
type KillPayload struct {
	CardID  string `json:"card_id"`
	Project string `json:"project"`
}

// StopAllPayload is sent by ContextMatrix to stop all tasks.
type StopAllPayload struct {
	Project string `json:"project,omitempty"`
}

// MessagePayload is the body for POST /message. Exactly one of (card_id +
// project) for card-bound HITL or session_id for chat must be set.
type MessagePayload struct {
	CardID    string `json:"card_id,omitempty"`
	Project   string `json:"project,omitempty"`
	SessionID string `json:"session_id,omitempty"`
	Content   string `json:"content"`
	MessageID string `json:"message_id,omitempty"`
}

// IsChat reports whether the payload targets a chat session.
func (p MessagePayload) IsChat() bool { return p.SessionID != "" }

// PromotePayload is sent by ContextMatrix to switch a running interactive
// session to fully autonomous mode.
type PromotePayload struct {
	CardID  string `json:"card_id"`
	Project string `json:"project"`
}

// EndSessionPayload is sent by ContextMatrix to close the stdin of a
// running interactive container so claude exits on EOF. Used when the card
// reaches a terminal state and is released.
type EndSessionPayload struct {
	CardID  string `json:"card_id"`
	Project string `json:"project"`
}

// RefreshKnowledgePayload is sent by ContextMatrix to start a KB
// refresh container for (project, repo). Retired with the KB machinery in
// C3; rides along until then. No card_id — the (project, repo) pair is the
// job key.
type RefreshKnowledgePayload struct {
	Project       string   `json:"project"`
	Repo          string   `json:"repo"`
	RepoURL       string   `json:"repo_url"`
	BaseBranch    string   `json:"base_branch,omitempty"`
	AgentID       string   `json:"agent_id"`
	OverwriteDocs []string `json:"overwrite_docs,omitempty"`
	MCPAPIKey     string   `json:"mcp_api_key,omitempty"`
	RunnerImage   string   `json:"runner_image,omitempty"`
	Model         string   `json:"model,omitempty"`
}
