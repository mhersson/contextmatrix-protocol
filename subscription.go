package protocol

import "time"

// CostBasisSubscription marks token usage served by a flat-rate subscription:
// the counts are real, no per-call money was billed, and consumers price it at
// list price as an API-equivalent estimate. Absent means metered usage
// (gateway-billed, or priced from the rate table when the gateway sent no cost).
const CostBasisSubscription = "subscription"

// SubscriptionUsage is one subscription limit window as last reported by the
// vendor CLI.
type SubscriptionUsage struct {
	// Window is the vendor's window name, e.g. "seven_day" or "five_hour".
	Window string `json:"window"`
	// Utilization is the fraction of the window used (0.13 = 13%). It can
	// exceed 1 while overage is in use. Zero means unused, so it is always sent.
	Utilization float64 `json:"utilization"`
	// Status is the vendor's status for the window: "allowed", "allowed_warning"
	// or "rejected". Display only; consumers never branch on it.
	Status string `json:"status,omitempty"`
	// ResetsAt is when the window resets. Always set: a producer drops a window
	// whose reset time it does not know.
	ResetsAt time.Time `json:"resets_at"`
}

// UsageLimit is the payload of a usage-limit frame: a subscription limit
// stopped the run or turn, and work can resume after ResetsAt.
type UsageLimit struct {
	// Window is the window that hit its limit; empty when the vendor named none.
	Window string `json:"window,omitempty"`
	// ResetsAt is absent when the vendor reported no reset time.
	ResetsAt time.Time `json:"resets_at,omitzero"`
}
