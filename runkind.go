package protocol

// Run kinds are sandboxed agent runs Sentinel dispatches, each for one card or,
// for guidance_pr, one project. They get no MCP key, never write the card, and
// report only on their terminal status.
const (
	RunKindCheck       = "check"       // no model
	RunKindInvestigate = "investigate" // model
	RunKindAnalyse     = "analyse"     // model
	RunKindGuidancePR  = "guidance_pr" // no model
)

// GuidanceBranchPrefix + the project name is the standing guidance PR's head
// branch. Project names are branch-safe. The slash keeps it out of the
// card-branch shape cm/<id>.
const GuidanceBranchPrefix = "cm/sentinel/"

// RunKindPayload is the body of POST /run-kind.
type RunKindPayload struct {
	Kind string `json:"kind"`
	// CardID is empty for guidance_pr, a project-level kind. It stays required
	// for every other kind.
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
	Analyse     *AnalyseInput     `json:"analyse,omitempty"`
	GuidancePR  *GuidancePRInput  `json:"guidance_pr,omitempty"`
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

// AnalyseInput drives the analyse kind on one finished card run.
type AnalyseInput struct {
	// RunID is the finished run's run_id; its durable-log section is read.
	RunID string `json:"run_id"`
	// StopCause is that run's stop cause; empty when CM's stall or lost-worker
	// detection ended it or the cause is unknown.
	StopCause string `json:"stop_cause,omitempty"`
	// Card is the card file as CM stores it now.
	Card string `json:"card"`
	// Verify is the card-over-project verify in force.
	Verify *VerifyConfig `json:"verify,omitempty"`
	// Subtasks are the card's subtasks, each as its card file.
	Subtasks []string `json:"subtasks,omitempty"`
}

// GuidancePRInput drives the no-model guidance_pr kind: it writes Section
// verbatim into the project's block of the repo's instruction file on
// GuidanceBranchPrefix+project and opens or updates the PR.
type GuidancePRInput struct {
	// ExpectHead is the branch tip of Sentinel's last push while its PR is
	// open. Empty = no open Sentinel PR: start the branch from the base branch.
	ExpectHead string `json:"expect_head,omitempty"`
	// Section is the guidance this PR proposes, CM-vetted, appended to the
	// block's content on the base branch. Never empty.
	Section string `json:"section"`
	// Allowed are text-check entries CM accepts although the base branch lacks
	// them (each appears in the source card's human-written text).
	Allowed []string `json:"allowed,omitempty"`
	// Title is the PR title and the commit subject; Body is the PR body.
	Title string `json:"title"`
	Body  string `json:"body"`
}

// RunKindResult holds the output of the kind named on the status; exactly that
// field is set.
type RunKindResult struct {
	Check       *CheckResult       `json:"check,omitempty"`
	Investigate *InvestigateResult `json:"investigate,omitempty"`
	Analyse     *AnalyseResult     `json:"analyse,omitempty"`
	GuidancePR  *GuidancePRResult  `json:"guidance_pr,omitempty"`
}

// RunUsage is one model's usage over a run kind. A run kind has no MCP key, so
// this replaces report_usage. On the wire: "model" plus LogTokenUsage's fields.
type RunUsage struct {
	Model string `json:"model"`
	LogTokenUsage
}

// CheckResult is the check kind's output.
type CheckResult struct {
	// Candidates are the verify commands the base branch declares that resolve
	// in this worker image: the agent's repo-convention detection, every
	// runnable row rather than the first, each as the display string the gate
	// would run, in detection order. Never null; empty = none.
	Candidates []string `json:"candidates"`
	// Trial is the trial run's outcome, a Trial constant; absent when the input
	// had no Verify.
	Trial string `json:"trial,omitempty"`
	// AbsentFromRepo lists the commands, URLs and hostnames in the input text
	// that appear nowhere on the base branch; absent when there are none or no
	// text was checked.
	AbsentFromRepo []string `json:"absent_from_repo,omitempty"`
}

const (
	TrialFinished       = "finished" // exited within its timeout; failing tests count
	TrialTimeout        = "timeout"
	TrialToolchain      = "toolchain"      // exit 127, a start error, or a wrapper's tool-missing output
	TrialInfrastructure = "infrastructure" // resource exhaustion, or a container runtime the worker lacks
	TrialRejected       = "rejected"       // not run: a changed command that is not a candidate or not plain argv
)

// InvestigateResult is the investigate kind's output.
type InvestigateResult struct {
	// Diagnosis is the finding a human reads on the Sentinel tab. It names any
	// check a verify fix drops, and why.
	Diagnosis string `json:"diagnosis"`
	// Cause is a Diagnosis constant.
	Cause string `json:"cause"`
	// Fix is set exactly when Cause is DiagnosisCard.
	Fix *CardFix `json:"fix,omitempty"`
	// ForwardTo lists the IDs, among the input's Siblings, that the same fix
	// applies to without loosening any of their criteria.
	ForwardTo []string `json:"forward_to,omitempty"`
	// Findings are the run's friction findings.
	Findings []FrictionFinding `json:"findings,omitempty"`
}

const (
	DiagnosisCard        = "card"        // the card is the problem; Fix holds the change
	DiagnosisModel       = "model"       // the card is fine; rerun unchanged so the agent's escalation applies
	DiagnosisEnvironment = "environment" // outside the card and the ecosystem: credentials, network, the worker image
	DiagnosisEcosystem   = "ecosystem"   // a bug in ContextMatrix or the agent
)

// AnalyseResult is the analyse kind's output.
type AnalyseResult struct {
	// Findings are the run's friction findings. Never null; empty = none.
	Findings []FrictionFinding `json:"findings"`
}

// GuidancePRResult is the guidance_pr kind's output.
type GuidancePRResult struct {
	// Outcome is a Guidance constant.
	Outcome string `json:"outcome"`
	// PRURL, HeadSHA and File are set on pushed and unchanged: the PR, the
	// branch tip, and AGENTS.md or CLAUDE.md.
	PRURL   string `json:"pr_url,omitempty"`
	HeadSHA string `json:"head_sha,omitempty"`
	File    string `json:"file,omitempty"`
	// AbsentFromRepo is set on refused: the section's commands, URLs and
	// hostnames the base branch lacks that Allowed does not list.
	AbsentFromRepo []string `json:"absent_from_repo,omitempty"`
}

const (
	GuidancePushed    = "pushed"    // one commit pushed; PR opened or its title and body updated
	GuidanceUnchanged = "unchanged" // the block already holds Section; no commit; title and body updated
	GuidanceStale     = "stale"     // the branch or PR is not as ExpectHead says; nothing written
	GuidanceRefused   = "refused"   // the text check failed; nothing written
)

// CardFix is the closed fix shape. At least one field is set.
type CardFix struct {
	// Section is the new ## Sentinel content without its heading; it holds no
	// H2 line. Empty = unchanged.
	Section string `json:"section,omitempty"`
	// Verify replaces the card's verify fields whole; nil = unchanged.
	Verify *VerifyConfig `json:"verify,omitempty"`
	// RetireSubtasks are IDs, among the input's Subtasks, to retire.
	RetireSubtasks []string `json:"retire_subtasks,omitempty"`
}

// FrictionFinding is one friction finding from a run.
type FrictionFinding struct {
	// Kind is a Finding constant.
	Kind string `json:"kind"`
	// Title names the card or issue: code and ecosystem findings only.
	Title string `json:"title,omitempty"`
	// Text is the finding as it would be published: guidance lines, a card or
	// issue body, or a note.
	Text string `json:"text"`
	// Evidence summarises what in the run shows the friction. It names no card
	// IDs.
	Evidence string `json:"evidence"`
}

const (
	FindingGuidance      = "guidance"       // for the project's AGENTS.md or CLAUDE.md
	FindingCode          = "code"           // friction in the project's code: a card
	FindingCardAdvice    = "card_advice"    // note: card-authoring advice
	FindingProjectVerify = "project_verify" // note for admins: a project verify proposal
	FindingHardCard      = "hard_card"      // note: a hard card
	FindingEcosystem     = "ecosystem"      // an ecosystem bug or the generic agent prompt
)
