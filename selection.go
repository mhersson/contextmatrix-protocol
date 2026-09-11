package protocol

// SelectionContext carries the model-selection inputs CM resolves at trigger
// time and ships to the agent backend. Every field is optional: it is absent
// for any caller that does not populate it. The agent's
// selector is a pure consumer of this data - it performs no AA/OpenRouter
// fetches of its own.
type SelectionContext struct {
	// Candidates is the auto-selectable model set: the trusted-creator,
	// floor-clearing models with live prices, context windows, and per-role
	// quality priors.
	Candidates []CandidateModel `json:"candidates,omitempty"`
	// Favorites are operator preferences considered before cost-optimal
	// auto-selection (still subject to tier bar, blacklist, and window fit).
	Favorites []FavoriteRule `json:"favorites,omitempty"`
	// Blacklist is the set of OpenRouter slugs the agent must never
	// auto-select (learned harness-incompatibility).
	Blacklist []string `json:"blacklist,omitempty"`
	// TierBars is the operator's quality ladder per role: role name ("coder",
	// "reviewer") to tier name ("simple", "moderate", "complex", "critical")
	// to normalised-prior bar. A missing role or an empty map means the
	// built-in ladder for that role; a partial tier map merges over the
	// built-in bars. The agent validates each role's ladder with
	// selection.LaddersFromWire and falls back to the built-in bars for that
	// role, logging on the card, when it does not validate. Absent for CMs
	// older than this field, which is the same as sending nothing.
	TierBars map[string]map[string]float64 `json:"tier_bars,omitempty"`
	// PriceHeadroom is the operator's best-value band multiplier: a
	// candidate is in band when its blended price is at most the cheapest
	// eligible candidate's times this. 0 or absent means the built-in
	// selection.DefaultPriceHeadroom (1.5). The agent applies it through
	// selection.Input.PriceHeadroom, which reads any value below 1 as the
	// built-in one. Absent for CMs older than this field, which is the same
	// as sending nothing.
	PriceHeadroom float64 `json:"price_headroom,omitempty"`
}

// CandidateModel is one auto-selectable model with everything the agent's
// selector needs. Prices are per-token (USD); priors are normalized
// Artificial Analysis indices in [0,1] (coder = coding index / max,
// reviewer = intelligence index / max).
type CandidateModel struct {
	Slug                  string  `json:"slug"`
	PromptPricePerTok     float64 `json:"prompt_price_per_tok"`
	CompletionPricePerTok float64 `json:"completion_price_per_tok"`
	ContextWindow         int     `json:"context_window"`
	CoderPrior            float64 `json:"coder_prior"`
	ReviewerPrior         float64 `json:"reviewer_prior"`
	// Creator is the model creator's vendor namespace prefix (openai,
	// anthropic, z-ai, qwen, ...). Empty when unknown (operator prior
	// override, pre-v0.15 CM); an empty Creator exempts the model from
	// vendor-diversity treatment in the agent's selector.
	Creator string `json:"creator,omitempty"`
}

// FavoriteRule is an operator-configured preference: for the given complexity
// Tier (and optionally a specific Role), prefer these model slugs in order
// over cost-optimal auto-selection. An empty Role applies the rule to all
// roles. Tier is one of: simple, moderate, complex, critical.
type FavoriteRule struct {
	Tier   string   `json:"tier"`
	Role   string   `json:"role,omitempty"`
	Models []string `json:"models"`
}
