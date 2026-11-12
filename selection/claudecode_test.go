package selection

import "testing"

func TestClaudeCodeModelID(t *testing.T) {
	cases := []struct {
		slug, want string
		ok         bool
	}{
		{"anthropic/claude-opus-5.5", "claude-opus-5-5", true},
		{"anthropic/claude-sonnet-5-5", "claude-sonnet-5-5", true},
		{"anthropic/claude-haiku-5.5:thinking", "claude-haiku-5-5", true},
		{"claude-sonnet-5-5", "claude-sonnet-5-5", true}, // a stored Claude Code ID maps to itself
		{"claude-opus-5.5", "claude-opus-5-5", true},
		{"anthropic/claude-", "", false},
		{"anthropic/claude-:thinking", "", false},
		{"claude-", "", false},
		{"anthropic/gpt-oss", "", false},
		{"openai/gpt-6", "", false},
		{"z-ai/claude-lookalike", "", false},
		{"Anthropic/Claude-Opus-5.5", "", false}, // slugs are lower-case; no case folding
		{"", "", false},
	}
	for _, tc := range cases {
		id, ok := ClaudeCodeModelID(tc.slug)
		if id != tc.want || ok != tc.ok {
			t.Errorf("ClaudeCodeModelID(%q) = (%q, %v), want (%q, %v)", tc.slug, id, ok, tc.want, tc.ok)
		}
	}
}

func TestClaudeCodeModels(t *testing.T) {
	models := ClaudeCodeModels()
	if len(models) == 0 {
		t.Fatal("ClaudeCodeModels is empty")
	}

	seen := map[string]bool{}

	for _, m := range models {
		if id, ok := ClaudeCodeModelID(m.ID); !ok || id != m.ID {
			t.Errorf("ClaudeCodeModelID(%q) = (%q, %v), want the ID itself", m.ID, id, ok)
		}

		if seen[m.ID] {
			t.Errorf("duplicate model %q", m.ID)
		}

		seen[m.ID] = true

		if m.ContextWindow <= 0 {
			t.Errorf("model %q has context window %d", m.ID, m.ContextWindow)
		}
	}

	models[0].ID = "mutated"
	if ClaudeCodeModels()[0].ID == "mutated" {
		t.Error("ClaudeCodeModels shares its backing array with callers")
	}
}
