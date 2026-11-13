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
	// RunID is the backend's id for the run this status belongs to (the
	// agent's per-run correlation id). Set on every status of a run the
	// backend admitted, running included; absent from older backends.
	RunID string `json:"run_id,omitempty"`
	// StopCause says why a run ended: one of the StopCause constants, on
	// terminal statuses only. Absent when the backend could not tell (an older
	// backend, a worker that died without reporting). Consumers read an absent
	// or unknown value as unknown.
	StopCause string `json:"stop_cause,omitempty"`
	// Kind is set on a run kind's status; absent for a card run. A run kind's
	// status never describes the card's worker.
	Kind string `json:"kind,omitempty"`
	// Result is a run kind's output, on a completed status with stop cause
	// done only.
	Result *RunKindResult `json:"result,omitempty"`
	// Usage is a model run kind's token usage, one row per model, on its
	// terminal status whenever it spent anything.
	Usage []RunUsage `json:"usage,omitempty"`
}
