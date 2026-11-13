package protocol

// Stop causes the worker reports for its own run.
const (
	StopCauseDone             = "done"
	StopCauseEnded            = "ended"
	StopCauseReviewParked     = "review_parked"
	StopCauseGatesParked      = "gates_parked"
	StopCauseUsageLimit       = "usage_limit"
	StopCauseBudgetExceeded   = "budget_exceeded"
	StopCauseContextLimit     = "context_limit"
	StopCauseMaxTurns         = "max_turns"
	StopCauseToolchainMissing = "toolchain_missing"
	StopCauseNoModel          = "no_model"
	StopCauseVerifyParked     = "verify_parked"
	StopCauseEmptySubtask     = "empty_subtask"
	StopCauseSplitOverflow    = "split_overflow"
	StopCauseError            = "error"
)

// Stop causes only the backend host sets: the run did not end on its own terms.
const (
	StopCauseKilled         = "killed"
	StopCauseShutdown       = "shutdown"
	StopCauseTimeout        = "timeout"
	StopCauseInfrastructure = "infrastructure"
)
