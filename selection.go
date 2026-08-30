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
