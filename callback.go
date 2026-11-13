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
}
