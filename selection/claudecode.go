package selection

import "strings"

// ClaudeCodeModelID maps a model slug to its Claude Code model ID.
// "anthropic/claude-<rest>" and a bare "claude-<rest>" map to
// "claude-<rest>" with any ":variant" suffix dropped and dots turned into
// hyphens ("anthropic/claude-opus-5.5" -> "claude-opus-5-5"). Anything
// else, or an empty <rest>, returns ok=false. It does not check the ID
// against ClaudeCodeModels: CM and the architect do that, and the agent checks
// it against the trigger's selection candidates.
func ClaudeCodeModelID(slug string) (id string, ok bool) {
	const prefix = "claude-"

	rest, found := strings.CutPrefix(strings.TrimPrefix(slug, "anthropic/"), prefix)
	if !found {
		return "", false
	}

	rest, _, _ = strings.Cut(rest, ":")
	if rest == "" {
		return "", false
	}

	return prefix + strings.ReplaceAll(rest, ".", "-"), true
}

// ClaudeCodeModel is one model the Claude Code subscription channel serves.
type ClaudeCodeModel struct {
	ID            string // Claude Code model ID, e.g. "claude-opus-5-5"
	ContextWindow int    // tokens
}

// ClaudeCodeModels is the Claude Code catalog, a fresh slice on every call.
// Fable is left out: in -p it spends usage credits once half the weekly limit
// is used.
func ClaudeCodeModels() []ClaudeCodeModel {
	return []ClaudeCodeModel{
		{ID: "claude-opus-5-5", ContextWindow: 1_000_000},
		{ID: "claude-sonnet-5-5", ContextWindow: 1_000_000},
		{ID: "claude-haiku-5-5", ContextWindow: 1_000_000},
	}
}
