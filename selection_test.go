package protocol

import (
	"encoding/json"
	"testing"
)

func TestSelectionContextWireShape(t *testing.T) {
	sc := SelectionContext{
		Candidates: []CandidateModel{{
			Slug:                  "deepseek/deepseek-v4-flash",
			PromptPricePerTok:     0.5,
			CompletionPricePerTok: 1.5,
			ContextWindow:         200000,
			CoderPrior:            0.9,
			ReviewerPrior:         0.8,
		}},
		Favorites: []FavoriteRule{
			{Tier: "complex", Models: []string{"a", "b"}},
			{Tier: "critical", Role: "reviewer", Models: []string{"c"}},
		},
		Blacklist: []string{"bad/model"},
	}
	b, err := json.Marshal(sc)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"candidates":[{"slug":"deepseek/deepseek-v4-flash",` +
		`"prompt_price_per_tok":0.5,"completion_price_per_tok":1.5,` +
		`"context_window":200000,"coder_prior":0.9,"reviewer_prior":0.8}],` +
		`"favorites":[{"tier":"complex","models":["a","b"]},` +
		`{"tier":"critical","role":"reviewer","models":["c"]}],` +
		`"blacklist":["bad/model"]}`
	if string(b) != want {
		t.Errorf("wire drift:\n got %s\nwant %s", b, want)
	}
}

func TestSelectionContextEmptyOmits(t *testing.T) {
	b, err := json.Marshal(SelectionContext{})
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `{}` {
		t.Errorf("empty SelectionContext should marshal to {}, got %s", b)
	}
}
