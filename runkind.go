package protocol

// Run kinds are sandboxed agent runs Sentinel dispatches for one card. They get
// no MCP key, never write the card, and report only on their terminal status.
const (
	RunKindCheck       = "check"       // no model
	RunKindInvestigate = "investigate" // model
	// Reserved; their inputs and results are added when they ship.
	RunKindAnalyse    = "analyse"
	RunKindGuidancePR = "guidance_pr"
)

// RunKindPayload is the body of POST /run-kind.
type RunKindPayload struct {
	Kind    string `json:"kind"`
	CardID  string `json:"card_id"`
	Project string `json:"project"`
	RepoURL string `json:"repo_url"`
	// BaseBranch is the branch the run clones: the card's base, or its
	// playbook branch. Empty = the remote default.
	BaseBranch string `json:"base_branch,omitempty"`
	// WorkerImage is the project's worker image, so a trial run sees the
	// toolchain the card's runs see. Empty = the backend default.
	WorkerImage string `json:"worker_image,omitempty"`
	// GitToken clones RepoURL. It carries no expiry and is never
	// refreshed: the run clones once, at start.
	GitToken string `json:"git_token"`

	// Model kinds only; MaxCostUSD must be > 0 for them.
	MaxCostUSD  float64           `json:"max_cost_usd,omitempty"`
	Model       string            `json:"model,omitempty"`
	Selection   *SelectionContext `json:"selection,omitempty"`
	LLMEndpoint *LLMEndpoint      `json:"llm_endpoint,omitempty"`

	// Exactly the input named by Kind is set.
	Check       *CheckInput       `json:"check,omitempty"`
	Investigate *InvestigateInput `json:"investigate,omitempty"`
}

// CheckInput drives the check kind on the base branch. It always lists the
// verify candidates; Verify adds one trial run, Text adds the text check.
type CheckInput struct {
	// Verify is the command to trial-run once, with its timeout and env names.
	// Nil = no trial.
	Verify *VerifyConfig `json:"verify,omitempty"`
	// CurrentCommand is the card-or-project verify command in force today. It
	// counts as declared: a trial of this command skips the candidate and
	// plain-argv checks. Empty = none in force.
	CurrentCommand string `json:"current_command,omitempty"`
	// Text is proposed ## Sentinel content to check. Empty = no text check.
	Text string `json:"text,omitempty"`
}

// InvestigateInput drives the investigate kind on one stopped card run.
type InvestigateInput struct {
	// RunID is the stopped run's run_id; its durable-log section is what the
	// investigation reads.
	RunID string `json:"run_id"`
	// StopCause is that run's stop cause; empty when CM's stall or lost-worker
	// detection ended it.
	StopCause string `json:"stop_cause,omitempty"`
	// Card is the card file as CM stores it: YAML frontmatter and markdown body.
	Card string `json:"card"`
	// Verify is the card-over-project verify CM resolves for the card's runs.
	Verify *VerifyConfig `json:"verify,omitempty"`
	// Subtasks are the card's subtasks, each as its card file.
	Subtasks []string `json:"subtasks,omitempty"`
	// Siblings are the not-yet-started cards of the same playbook run in the
	// same project, each as its card file; empty outside a playbook run.
	Siblings []string `json:"siblings,omitempty"`
}
