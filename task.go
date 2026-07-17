package protocol

// TriggerPayload is sent by ContextMatrix to start a task.
type TriggerPayload struct {
	CardID      string `json:"card_id"`
	Project     string `json:"project"`
	RepoURL     string `json:"repo_url"`
	MCPAPIKey   string `json:"mcp_api_key,omitempty"`
	BaseBranch  string `json:"base_branch,omitempty"`
	WorkerImage string `json:"worker_image,omitempty"`
	Interactive bool   `json:"interactive,omitempty"`
	Model       string `json:"model,omitempty"`
	// BestOfN, when >= 2, asks the agent backend to race N candidate
	// implementations and judge the winner. 0/absent = normal run.
	BestOfN int `json:"best_of_n,omitempty"`
	// Mob configures mob session discussions for this run. Nil = solo run.
	Mob *MobSpec `json:"mob,omitempty"`
	// TaskSkills is the explicit task-skill filter, resolved by CM as card
	// skills over the project default. Nil = no filter - the worker gets the
	// full mounted skill set; non-nil (even empty) = engage exactly these.
	TaskSkills *[]string `json:"task_skills,omitempty"`
	// Selection carries auto-selection inputs for the agent backend
	// (candidates, favorites, blacklist).
	Selection *SelectionContext `json:"selection,omitempty"`
	// Verify is the resolved card-over-project verify config for this run.
	// Nil = nothing declared; the agent falls back to its own detection.
	Verify *VerifyConfig `json:"verify,omitempty"`
	// GitToken is a short-lived token for cloning/pushing the project repo,
	// minted by CM from the project's credential binding (or the instance
	// credential when unbound). The agent backend fail-closes without it: the
	// trigger is admitted, immediately rejected, and reported failed via the
	// status callback - there is no local fallback.
	GitToken string `json:"git_token,omitempty"`
	// GitTokenExpiresAt is the RFC3339 expiry of GitToken. App-backed tokens
	// live ~1h; backends refresh via GET /api/<backend>/git-credentials.
	// PAT-backed tokens carry a zero/absent expiry (the PAT itself).
	GitTokenExpiresAt string `json:"git_token_expires_at,omitempty"`
	// LLMEndpoint is the inference endpoint configuration provisioned by CM
	// (single admin-managed key, rotated in one place). The agent backend
	// fail-closes without it: the trigger is admitted, immediately rejected,
	// and reported failed via the status callback - there is no local
	// fallback.
	LLMEndpoint *LLMEndpoint `json:"llm_endpoint,omitempty"`
}

// GuestSpec is an operator-registered external A2A participant resolved by CM
// at trigger time. Token is a bearer secret: serve stages it into the per-run
// secrets file, never plain container env.
type GuestSpec struct {
	Name  string `json:"name"`
	URL   string `json:"url"`
	Token string `json:"token,omitempty"`
}

// MobSpec configures mob session discussions for a run (moderator + participant seats).
type MobSpec struct {
	Participants       int      `json:"participants"`
	Phases             []string `json:"phases,omitempty"`
	Rounds             int      `json:"rounds,omitempty"`
	BudgetFactor       float64  `json:"budget_factor,omitempty"`
	ExecuteCheckpoints bool     `json:"execute_checkpoints,omitempty"`
	CheckpointMinTier  string   `json:"checkpoint_min_tier,omitempty"`
	// CheckpointRounds is the critique-round count for execute-checkpoint
	// discussions (plan/review discussions use Rounds). 0 = backend default.
	CheckpointRounds int         `json:"checkpoint_rounds,omitempty"`
	Guests           []GuestSpec `json:"guests,omitempty"`
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

// PromotePayload is sent by ContextMatrix to switch a running interactive
// session to fully autonomous mode.
type PromotePayload struct {
	CardID  string `json:"card_id"`
	Project string `json:"project"`
}

// EndSessionPayload is sent by ContextMatrix to close the stdin of a
// running interactive worker so it sees EOF and exits. Used when the card
// reaches a terminal state and is released.
type EndSessionPayload struct {
	CardID  string `json:"card_id"`
	Project string `json:"project"`
}
