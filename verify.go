package protocol

// VerifyConfig is the operator-declared verify gate for a task run. CM resolves
// card-level over project-level settings field-by-field before sending;
// consumers treat an absent config as "nothing declared" and fall back to their
// own detection.
type VerifyConfig struct {
	// Command is a shell string the agent runs via bash -c with pipefail.
	// Empty means no declared command.
	Command string `json:"command,omitempty"`
	// TimeoutSeconds bounds the verify subprocess. 0 means the consumer's
	// default. It applies to detected/proposed commands too — the timeout is
	// independent of where the command came from.
	TimeoutSeconds int `json:"timeout_seconds,omitempty"`
	// Env names container environment variables passed through to the verify
	// subprocess on top of the consumer's scrubbed allowlist (e.g. JAVA_HOME).
	// Names only, never values.
	Env []string `json:"env,omitempty"`
}
