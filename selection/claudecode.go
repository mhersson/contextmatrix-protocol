package selection

import "strings"

// ClaudeCodeModelID maps a model slug to its Claude Code model ID.
// "anthropic/claude-<rest>" and a bare "claude-<rest>" map to
// "claude-<rest>" with any ":variant" suffix dropped and dots turned into
// hyphens ("anthropic/claude-opus-5.5" -> "claude-opus-5-5"). Anything
// else, or an empty <rest>, returns ok=false. It does not check the ID
// against any catalog; a caller that needs membership checks it against its
// own.
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
