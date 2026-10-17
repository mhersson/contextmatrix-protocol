package protocol

import "time"

// LogEntry is one frame on a backend's /logs SSE stream (the `data:` JSON
// value). Field ORDER matters only for the pin test; field TAGS are the
// wire contract. CM consumes these frames in two places: the chat manager's
// log bridge and the task session-log manager.
type LogEntry struct {
	Timestamp time.Time `json:"ts"`
	CardID    string    `json:"card_id,omitempty"`
	Project   string    `json:"project,omitempty"`
	SessionID string    `json:"session_id,omitempty"`
	// Type is one of: text, thinking, tool_call, user_question, stderr,
	// system, user, usage, status. "user" is a HITL chat-input message
	// published directly by the backend's /message handler (bypasses the
	// backend's output redaction). "usage" frames carry Usage and Model with
	// empty Content - content-less metadata frames. "user_question" is not
	// emitted; its tag and ToolUseID exist for wire compatibility with
	// persisted entries that still carry it. "status" is an ephemeral
	// run-state frame (Content "working" or "idle") emitted by chat-mode
	// logbridges; consumers translate it to presence state and never
	// persist it as a transcript row.
	Type      string         `json:"type"`
	Content   string         `json:"content,omitempty"`
	ToolUseID string         `json:"tool_use_id,omitempty"`
	Usage     *LogTokenUsage `json:"usage,omitempty"`
	// Model is the slug that produced the frame. Set on usage frames and on
	// text frames (assistant responses and mob discussion utterances); empty
	// elsewhere.
	Model string `json:"model,omitempty"`
	// Agent labels the speaker on mob session discussion frames ("seat-1",
	// "guest-<name>", "moderator", "human"). Empty on every other frame -
	// absent from the wire, so consumers that predate mob sessions see
	// identical bytes.
	Agent string `json:"agent,omitempty"`
	// Prompt marks a text frame as a question or gate the panel renders with
	// reply buttons. Content stays the full readable message, so a consumer
	// without prompt support shows an ordinary message. Only the architect
	// sets it; absent on every other frame.
	Prompt *LogPrompt `json:"prompt,omitempty"`
	// Marker turns the frame into a blueprint transcript marker. Content stays
	// a plain fallback line, so a consumer without marker support shows an
	// ordinary row. Only the architect sets it; absent on every other frame.
	Marker *LogMarker `json:"marker,omitempty"`
}

// LogPrompt is the reply-button half of a prompt frame. Consumers never
// interpret a reply: a chosen action goes back as an ordinary user message,
// so the producer's own reply parsing is unchanged.
type LogPrompt struct {
	// ID is opaque to consumers, for example a question number or a gate's
	// stage and version. The blueprint gate bar finds the open prompt's
	// actions by matching OpenPrompt.PromptID against it; nothing else
	// matches on it.
	ID string `json:"id,omitempty"`
	// Kind is a display hint, "question", "gate" or "round". Consumers style by
	// it, treat any other value as neutral, and never match on it.
	Kind string `json:"kind,omitempty"`
	// Text is the prompt without its option list or reply instructions, for a
	// consumer that renders the actions as the answers. Empty means render the
	// frame's Content, which stays the full fallback message.
	Text string `json:"text,omitempty"`
	// FreeText means any typed reply answers the prompt, not only an action's.
	FreeText bool        `json:"free_text,omitempty"`
	Actions  []LogAction `json:"actions,omitempty"`
	// Round is set on a Kind "round" prompt: a tabbed question round. Actions
	// and FreeText are unused on it. The architect republishes the same ID
	// whenever the round changes (answers sent, chat, revised options); a
	// consumer shows the newest frame for an ID and treats older ones as
	// history.
	Round *LogRound `json:"round,omitempty"`
}

// LogAction is one reply button.
type LogAction struct {
	Label string `json:"label"`
	// Reply is the exact message a click sends.
	Reply string `json:"reply"`
	// Feedback asks for text first and sends Reply, a space, then the text.
	Feedback bool `json:"feedback,omitempty"`
	// Recommended marks the producer's suggested answer. The label carries no
	// "(recommended)" suffix; consumers render the hint themselves.
	Recommended bool `json:"recommended,omitempty"`
}

// LogRound is a question round the architect asks: one tab per question,
// an optional assumptions tab, and what is stored so far.
type LogRound struct {
	// Persona asks the round, for the block's header ("system designer").
	Persona string `json:"persona"`
	// Stage is the pack stage id that asks.
	Stage     string          `json:"stage"`
	Questions []RoundQuestion `json:"questions"`
	// Assumptions, when set, is the round's last tab: each one accepted or
	// turned into a question.
	Assumptions []RoundAssumption `json:"assumptions,omitempty"`
	// Answers already stored for this round (a partial send, or a reopen).
	Answers []RoundAnswer `json:"answers,omitempty"`
	// Decisions already stored on the assumptions tab.
	Decisions []AssumptionDecision `json:"decisions,omitempty"`
	// Chat is the "Chat about these questions" thread, oldest first.
	Chat []RoundChat `json:"chat,omitempty"`
	// ResearchOff means option sources come from the model's knowledge;
	// consumers say so instead of showing links.
	ResearchOff bool `json:"research_off,omitempty"`
	// Responder means the auto responder answered; consumers show the round
	// read-only.
	Responder bool `json:"responder,omitempty"`
	// Closed means every question is answered and the stage resumed;
	// consumers collapse the block into its summary.
	Closed bool `json:"closed,omitempty"`
}

// RoundQuestion is one tab of a round.
type RoundQuestion struct {
	// Number is the blueprint-wide question number, shown as Q<n>.
	Number  int           `json:"number"`
	Title   string        `json:"title"` // the tab label, a few words
	Text    string        `json:"text"`
	Why     string        `json:"why,omitempty"`
	Options []RoundOption `json:"options"`
	// Recommended is the 1-based position of the recommended option; 0 none.
	Recommended int `json:"recommended,omitempty"`
}

// RoundOption is one candidate answer with its trade-offs.
type RoundOption struct {
	Title   string        `json:"title"`
	Summary string        `json:"summary,omitempty"`
	Pros    []string      `json:"pros,omitempty"`
	Cons    []string      `json:"cons,omitempty"`
	Sources []RoundSource `json:"sources,omitempty"`
}

// RoundSource is a source an option rests on.
type RoundSource struct {
	// Tag is the architect's source tag (e.g. S3); URL is empty for a code
	// source. Title is the link text.
	Tag   string `json:"tag"`
	Title string `json:"title"`
	URL   string `json:"url,omitempty"`
}

// RoundAssumption is one entry of a round's assumptions tab.
type RoundAssumption struct {
	ID   string `json:"id"` // A<n>
	Text string `json:"text"`
}

// RoundAnswer is one question's stored answer.
type RoundAnswer struct {
	Number int `json:"number"` // the question's Number
	// Option is the chosen option's 1-based position; 0 means Text is the
	// person's own answer.
	Option int    `json:"option,omitempty"`
	Text   string `json:"text,omitempty"`
}

// AssumptionDecision is one decision on the assumptions tab.
type AssumptionDecision struct {
	ID     string `json:"id"`
	Accept bool   `json:"accept"` // false = turn it into a question
}

// RoundChat is one message of a round's chat thread.
type RoundChat struct {
	// From is "user" or the asking persona's id.
	From string `json:"from"`
	Text string `json:"text"`
}

// LogMarker turns a blueprint transcript row into a stage divider or a
// sheet reference; the frame's Content keeps a plain fallback line.
type LogMarker struct {
	// Kind is "stage" (a stage divider, on a "system" frame) or "artifact"
	// (a sheet reference that opens Stage at Version, on a "text" frame), so
	// a consumer without marker support shows the fallback as a fitting row.
	// A consumer shows Content for any other Kind.
	Kind  string `json:"kind"`
	Stage string `json:"stage"`
	// Persona is who runs the stage, on a stage divider.
	Persona string `json:"persona,omitempty"`
	// Version is the artifact version a sheet reference opens.
	Version int `json:"version,omitempty"`
	// Note is a sheet reference's one-line summary, for example a review
	// outcome or who approved the version.
	Note string `json:"note,omitempty"`
}

// LogTokenUsage carries per-turn token accounting on "usage"-type frames.
// Per-turn, NOT cumulative session totals. InputTokens + CacheReadTokens +
// CacheCreateTokens approximates the prompt size the model actually
// processed; consumers typically display that sum as "context used".
type LogTokenUsage struct {
	InputTokens       int64 `json:"input_tokens"`
	OutputTokens      int64 `json:"output_tokens"`
	CacheReadTokens   int64 `json:"cache_read_tokens"`
	CacheCreateTokens int64 `json:"cache_creation_tokens"`
}
