package protocol

import (
	"encoding/json"
	"time"
)

// BlueprintView is the architect's snapshot of one blueprint for the
// session pane: the pipeline, the run in flight, the open prompt, the cost
// and the stored artifact versions. The architect serves it on
// GET /blueprints/{id}, where the id is the chat session id, and in snapshot
// LiveFrames; CM relays the bytes unchanged. The architect sends every list
// in it (pipeline, artifacts, review findings, slices) as [], never null; a
// consumer still treats a missing or null list as empty.
type BlueprintView struct {
	ID string `json:"id"`
	// Mode is one of the ArchitectMode constants.
	Mode string `json:"mode"`
	// State is the architect's coarse lifecycle state: new (no need stated
	// yet), running (a run is in flight), waiting (a question, gate or
	// sign-off waits for the operator), idle (nothing runs; the next message
	// decides what follows), failed (a run or step failed; the next message
	// retries it), emitting (the approved cards are being created), emitted
	// (the cards exist; the watch may still report on them) or done (the
	// watch posted its closing summary). Consumers may branch on it for
	// display, but step statuses and Open carry the detail. New states may be
	// added, so render an unknown State neutrally.
	State string `json:"state"`
	// Seq is the id of the newest architect events row the view reflects, so
	// it keeps rising across architect restarts. A consumer always applies a
	// snapshot LiveFrame and applies a fetched view only when its Seq is at
	// least the held one.
	Seq       uint64         `json:"seq"`
	Pipeline  []PipelineStep `json:"pipeline"`
	Current   *CurrentRun    `json:"current,omitempty"`
	Open      *OpenPrompt    `json:"open,omitempty"`
	Cost      BlueprintCost  `json:"cost"`
	Artifacts []ArtifactRef  `json:"artifacts"`
	// Slices lists the pieces of the newest stored slicing version in slicing
	// order; [] before slicing stores one, and [] when no stage of the
	// blueprint's pack repeats per slice. Consumers show slice tabs, the
	// grouped scale bar and slice names only with two or more.
	Slices []BlueprintSlice `json:"slices"`
}

// BlueprintSlice is one piece of the blueprint, in slicing order.
type BlueprintSlice struct {
	// ID is SL<n>, n from 1 in slicing order; consumers never parse it.
	ID    string `json:"id"`
	Title string `json:"title"`
}

// PipelineStep is one segment of the blueprint's scale bar, in pipeline
// order.
type PipelineStep struct {
	// Stage is a pack stage id, "signoff" or "emission".
	Stage string `json:"stage"`
	// Slice is the piece a per-slice step belongs to; "" for a whole-blueprint
	// step and for per-slice steps before slicing stores a version. After
	// slicing, the four per-slice steps repeat once per slice, contiguous and in
	// slicing order (design, grounding, acceptance, decomposition for SL1, then
	// for SL2 ...).
	Slice   string `json:"slice,omitempty"`
	Persona string `json:"persona"`
	Gate    bool   `json:"gate"`
	// Policy is the gate's policy, "human" or "auto".
	Policy string `json:"policy,omitempty"`
	// FoldsInto names the stage this one folds into, when the pack folds it.
	FoldsInto string `json:"folds_into,omitempty"`
	// Status is one of: pending, running, reviewing, revising, waiting, done,
	// failed, folded, skipped. A collapsed stage (shared_decisions on a
	// one-slice blueprint: recorded, never run) is skipped.
	Status string `json:"status"`
	// Attempt is the attempt number of the step's newest stage run, from 1;
	// 0 until the stage first runs, and for signoff and emission. A fresh run
	// (the first, or a re-run after a failure, an abort, a gate rejection or
	// a readiness loop) starts a new attempt; builder rounds after a review
	// continue it.
	Attempt int `json:"attempt,omitempty"`
	// Round is set only on a loop stage (readiness): the loop round since the
	// sign-off's latest rejection, from 1, counting a run in flight ("round 2
	// of 3"). It is not the review or revision round; 0 on every other step
	// and before the loop stage first runs.
	Round int `json:"round,omitempty"`
	// MaxRounds is the most rounds a loop stage runs (the pack's max_rounds);
	// 0 elsewhere.
	MaxRounds int `json:"max_rounds,omitempty"`
	// LatestVersion is the newest stored version of the step's artifact, from
	// 1; 0 when none is stored. For signoff it is the readiness version the
	// latest sign-off judged (the n of gate:signoff:v<n>); for emission, 1
	// once the cards exist.
	LatestVersion int     `json:"latest_version,omitempty"`
	CostUSD       float64 `json:"cost_usd"`
}

// CurrentRun is the run in flight and its progress so far.
type CurrentRun struct {
	Stage string `json:"stage"`
	// Slice is the piece this belongs to; "" for the whole blueprint.
	Slice string `json:"slice,omitempty"`
	// Kind is "stage", "reviewer" or "responder".
	Kind      string    `json:"kind"`
	Persona   string    `json:"persona"`
	Model     string    `json:"model"`
	StartedAt time.Time `json:"started_at"`
	Turns     int       `json:"turns"`
	ToolCalls int       `json:"tool_calls"`
	// Tokens sums the usage buckets of the innermost run in flight (a
	// responder inside its stage run, else the run itself): prompt tokens
	// excluding the cached part, completion, cache-read and cache-write.
	Tokens int64 `json:"tokens"`
}

// OpenPrompt is the question or gate waiting for the operator.
type OpenPrompt struct {
	// PromptID equals the LogPrompt ID of the prompt's transcript row, for
	// example Q<n>, round:<n>, gate:<stage>:v<n>, gate:<stage>:<slice>:v<n> for a
	// per-slice gate, gate:signoff:v<n> or emission:v<n>.
	// Consumers match it by equality and never parse it.
	PromptID string `json:"prompt_id"`
	// Kind is "question", "gate" or "round".
	Kind string `json:"kind"`
}

// BlueprintCost is the architect's own ledger for the blueprint, never CM's
// estimate.
type BlueprintCost struct {
	// SpentUSD is the ledger plus the in-flight run.
	SpentUSD float64 `json:"spent_usd"`
	// RunSpentUSD is what the outermost run in flight (the stage or reviewer
	// run the per-run cap binds) has spent so far, its responder runs
	// included; compare it with RunCapUSD. 0 when no run is in flight.
	RunSpentUSD float64 `json:"run_spent_usd"`
	// CapUSD is the blueprint cap; 0 means no blueprint cap.
	CapUSD float64 `json:"cap_usd"`
	// RunCapUSD is the per-run cap; 0 means no per-run cap.
	RunCapUSD float64 `json:"run_cap_usd"`
	// Priced is false when any run was unpriced.
	Priced bool `json:"priced"`
}

// ArtifactRef indexes one stored artifact version in a BlueprintView.
type ArtifactRef struct {
	Stage string `json:"stage"`
	// Slice is the piece this belongs to; "" for the whole blueprint.
	Slice     string    `json:"slice,omitempty"`
	Version   int       `json:"version"`
	CreatedAt time.Time `json:"created_at"`
	// Status is one of: current, reviewing, at_gate, approved, rejected,
	// superseded, folded.
	Status    string     `json:"status"`
	DecidedAt *time.Time `json:"decided_at,omitempty"`
	// DecidedBy is "you" or "automatic": a human, or the auto policy on the
	// reviewer's verdict for a stage gate and an emitting mode for a
	// sign-off. An architect before v0.28.0 sent "responder" for an auto
	// stage gate; read it as "automatic".
	DecidedBy string          `json:"decided_by,omitempty"`
	Review    *ArtifactReview `json:"review,omitempty"`
}

// ArtifactReview is the reviewer's verdict on one artifact version. Each
// review lists every finding its round judged: the prior findings it
// re-checked, keeping their ID with Status fixed or still_open and
// VerifiedIn set to the reviewed version, and its new findings with Status
// open. An earlier version's review is never rewritten. SentTo and the
// Addressed fields keep their meaning: what the version the finding was
// sent to claimed about it.
type ArtifactReview struct {
	Reviewer  string `json:"reviewer"`
	Score     int    `json:"score"`
	Threshold int    `json:"threshold"`
	// Passed is the reviewer's verdict under the pack's rule: score at or
	// above Threshold and no open critical finding.
	Passed bool `json:"passed"`
	// Round is the reviewer round within the stage attempt, 1-based.
	Round    int             `json:"round,omitempty"`
	Findings []ReviewFinding `json:"findings"`
}

// ReviewFinding is one finding of a review's ledger. Every revision is
// re-reviewed, so Status is the reviewer's verdict, and the Addressed fields
// stay the builder's claim beside it, never proof that it is fixed.
type ReviewFinding struct {
	// ID is stable within the stage attempt (R1, R2, ...); a later round's
	// verdict on the same finding keeps its ID.
	ID string `json:"id"`
	// Severity is "critical", "major" or "minor".
	Severity string `json:"severity"`
	Text     string `json:"text"`
	// Status is the reviewer's verdict: "open" when raised in this review,
	// "fixed" or "still_open" when a later round re-checked it against the
	// version in VerifiedIn.
	Status string `json:"status"`
	// VerifiedIn is the version a later round checked this finding against;
	// zero while it is only open.
	VerifiedIn int `json:"verified_in,omitempty"`
	// SentTo is the revision version the finding went to.
	SentTo int `json:"sent_to,omitempty"`
	// AddressedIn, AddressedBy and AddressedNote are set only when the run
	// that wrote SentTo listed the finding as addressed.
	AddressedIn   int    `json:"addressed_in,omitempty"`
	AddressedBy   string `json:"addressed_by,omitempty"`
	AddressedNote string `json:"addressed_note,omitempty"`
}

// BlueprintArtifact is one stored artifact version, served on
// GET /blueprints/{id}/artifacts/{stage}/{version}, with ?slice=SL<n> for a
// per-slice stage's. Stage "signoff" and stage "emission" (version 1) are
// pseudo-artifacts: Data is the sign-off event payload, or for emission the
// created cards, the playbook, compare URLs and total cost.
type BlueprintArtifact struct {
	Stage string `json:"stage"`
	// Slice is the piece this belongs to; "" for the whole blueprint.
	Slice   string `json:"slice,omitempty"`
	Version int    `json:"version"`
	Body    string `json:"body"`
	// Data is the stage's structured payload; its shape is per stage and
	// owned by the architect.
	Data json.RawMessage `json:"data,omitempty"`
	// CheckProblems lists the contract checks an accepted submit still
	// failed.
	CheckProblems []string `json:"check_problems,omitempty"`
}

// LiveFrame is one data: frame on the architect's GET /blueprints/{id}/live
// SSE stream. The first frame on every connection is a snapshot.
type LiveFrame struct {
	// Type is "snapshot" (View plus the current Draft in full at offset 0),
	// "artifact_stored" (Stage at Version, for Slice, is stored; fetch it) or
	// "draft" (the next Draft fragment). A snapshot without Draft means no
	// draft is in progress, so the consumer drops any held draft. A consumer
	// ignores a frame of any other Type.
	Type  string         `json:"type"`
	View  *BlueprintView `json:"view,omitempty"`
	Draft *Draft         `json:"draft,omitempty"`
	Stage string         `json:"stage,omitempty"`
	// Slice is the piece this belongs to; "" for the whole blueprint. Set on
	// artifact_stored frames, with Stage and Version.
	Slice   string `json:"slice,omitempty"`
	Version int    `json:"version,omitempty"`
}

// Draft is the artifact text a run is writing, before it is stored.
type Draft struct {
	Stage string `json:"stage"`
	// Slice is the piece this belongs to; "" for the whole blueprint.
	Slice   string `json:"slice,omitempty"`
	Version int    `json:"version"`
	// Offset is how many Unicode code points (Go runes, not bytes or UTF-16
	// units; in JS, Array.from(text).length) were sent before Text. Offset 0
	// replaces the held text, an Offset equal to the held length appends, and
	// any other Offset means frames were missed: the consumer drops the frame
	// and waits for the next snapshot LiveFrame.
	Offset int    `json:"offset"`
	Text   string `json:"text"`
	// Refused marks a draft the contract checks refused, with the check
	// error in Reason; the next attempt restarts at offset 0.
	Refused bool   `json:"refused,omitempty"`
	Reason  string `json:"reason,omitempty"`
}
