package protocol

// ArchitectStart carries what a blueprint session needs beyond the chat
// fields: the target projects, the autonomy mode and the cost cap. CM sets
// it only on starts to an architect-kind backend; chat backends never see it.
type ArchitectStart struct {
	Projects   []string `json:"projects"`
	Mode       string   `json:"mode"`
	CostCapUSD float64  `json:"cost_cap_usd,omitempty"`
}

// Architect modes, chosen when a blueprint session is created. Default stops
// at every human gate the pack declares; auto answers them itself and stops
// before emission; auto+emit also emits; yolo emits and starts the run.
const (
	ArchitectModeDefault  = "default"
	ArchitectModeAuto     = "auto"
	ArchitectModeAutoEmit = "auto+emit"
	ArchitectModeYolo     = "yolo"
)

// ValidArchitectMode reports whether m is one of the architect modes.
func ValidArchitectMode(m string) bool {
	switch m {
	case ArchitectModeDefault, ArchitectModeAuto, ArchitectModeAutoEmit, ArchitectModeYolo:
		return true
	default:
		return false
	}
}
