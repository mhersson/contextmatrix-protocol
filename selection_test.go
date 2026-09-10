package protocol

import (
	"encoding/json"
	"strings"
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
			Creator:               "deepseek",
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
		`"context_window":200000,"coder_prior":0.9,"reviewer_prior":0.8,` +
		`"creator":"deepseek"}],` +
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

func TestTriggerPayloadSelectionRoundTrip(t *testing.T) {
	p := TriggerPayload{
		CardID: "CM-001", Project: "alpha", RepoURL: "r",
		Selection: &SelectionContext{Blacklist: []string{"x"}},
	}
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	var got TriggerPayload
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got.Selection == nil || len(got.Selection.Blacklist) != 1 || got.Selection.Blacklist[0] != "x" {
		t.Errorf("Selection did not round-trip: %+v", got.Selection)
	}
}

func TestSelectionOmitsZeroValueFields(t *testing.T) {
	b, err := json.Marshal(SelectionContext{Candidates: []CandidateModel{{Slug: "a/model"}}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "creator") {
		t.Errorf("zero-value fields must be omitted, got %s", b)
	}
}

func TestSelectionContextTierBarsWireShape(t *testing.T) {
	sc := SelectionContext{
		TierBars: map[string]map[string]float64{
			"coder":    {"simple": 0.65, "moderate": 0.8, "complex": 0.9, "critical": 0.95},
			"reviewer": {"simple": 0.65, "moderate": 0.76, "complex": 0.82, "critical": 0.93},
		},
	}
	b, err := json.Marshal(sc)
	if err != nil {
		t.Fatal(err)
	}
	// Map keys marshal sorted, so the bytes are stable.
	want := `{"tier_bars":{` +
		`"coder":{"complex":0.9,"critical":0.95,"moderate":0.8,"simple":0.65},` +
		`"reviewer":{"complex":0.82,"critical":0.93,"moderate":0.76,"simple":0.65}}}`
	if string(b) != want {
		t.Errorf("wire drift:\n got %s\nwant %s", b, want)
	}

	var back SelectionContext
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if back.TierBars["coder"]["complex"] != 0.9 || back.TierBars["reviewer"]["critical"] != 0.93 {
		t.Errorf("round trip lost a bar: %+v", back.TierBars)
	}
}

func TestSelectionContextTierBarsAbsentIsOmitted(t *testing.T) {
	b, err := json.Marshal(SelectionContext{Blacklist: []string{"x"}})
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `{"blacklist":["x"]}` {
		t.Errorf("an unset ladder must not appear on the wire: %s", b)
	}
}
