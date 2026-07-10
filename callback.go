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
}
