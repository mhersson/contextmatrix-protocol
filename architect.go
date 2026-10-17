package protocol

// ArchitectStart carries what a blueprint session needs beyond the chat
// fields: the target projects, the autonomy mode, the cost cap and the price
// of the session's model. CM sets it only on starts to an architect-kind
// backend; chat backends never see it.
type ArchitectStart struct {
	Projects   []string `json:"projects"`
	Mode       string   `json:"mode"`
	CostCapUSD float64  `json:"cost_cap_usd,omitempty"`
	// Price is the per-token price CM resolved for Model, with the cache
	// rates already resolved. Nil when CM has no price for the model; the
	// architect then cannot bind a cost cap and must refuse a capped run.
	Price *ModelPrice `json:"price,omitempty"`
	// Research is nil when research is off (the default).
	Research *ResearchStart `json:"research,omitempty"`
}

// ModelPrice is a model's price in USD per million tokens, one rate per
// token bucket. CM fills every field: a provider that reports no cache
// rates gets CM's defaults, so the receiver multiplies and never guesses.
type ModelPrice struct {
	PromptUSDPerMTok     float64 `json:"prompt_usd_per_mtok"`
	CompletionUSDPerMTok float64 `json:"completion_usd_per_mtok"`
	CacheReadUSDPerMTok  float64 `json:"cache_read_usd_per_mtok"`
	CacheWriteUSDPerMTok float64 `json:"cache_write_usd_per_mtok"`
}

// ResearchStart is the web research setting CM delivers with an architect
// start: the search provider and its key. APIKey is a secret.
type ResearchStart struct {
	Provider string `json:"provider"` // "brave"
	APIKey   string `json:"api_key"`
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
