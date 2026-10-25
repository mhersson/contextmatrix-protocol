package protocol

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestChatStartPayloadArchitectWireShape(t *testing.T) {
	p := ChatStartPayload{
		SessionID: "S1",
		Model:     "m",
		Architect: &ArchitectStart{Projects: []string{"alpha", "beta"}, Mode: ArchitectModeYolo, CostCapUSD: 12.5},
	}

	b, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}

	want := `{"session_id":"S1","model":"m","architect":{"projects":["alpha","beta"],"mode":"yolo","cost_cap_usd":12.5}}`
	if string(b) != want {
		t.Errorf("wire drift:\n got %s\nwant %s", b, want)
	}
}

func TestChatStartPayloadWithoutArchitectIsUnchanged(t *testing.T) {
	b, err := json.Marshal(ChatStartPayload{SessionID: "S1"})
	if err != nil {
		t.Fatal(err)
	}

	if want := `{"session_id":"S1"}`; string(b) != want {
		t.Errorf("legacy wire drift:\n got %s\nwant %s", b, want)
	}
}

func TestArchitectStartZeroCostCapOmitted(t *testing.T) {
	b, err := json.Marshal(ArchitectStart{Projects: []string{"alpha"}, Mode: ArchitectModeDefault})
	if err != nil {
		t.Fatal(err)
	}

	if want := `{"projects":["alpha"],"mode":"default"}`; string(b) != want {
		t.Errorf("got %s want %s", b, want)
	}
}

func TestValidArchitectMode(t *testing.T) {
	cases := map[string]bool{
		"default": true, "auto": true, "auto+emit": true, "yolo": true,
		"": false, "AUTO": false, "autoemit": false,
	}
	for in, want := range cases {
		if got := ValidArchitectMode(in); got != want {
			t.Errorf("ValidArchitectMode(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestArchitectStartPriceWireShape(t *testing.T) {
	b, err := json.Marshal(ArchitectStart{
		Projects: []string{"alpha"},
		Mode:     ArchitectModeDefault,
		Price: &ModelPrice{
			PromptUSDPerMTok:     3,
			CompletionUSDPerMTok: 15,
			CacheReadUSDPerMTok:  0.3,
			CacheWriteUSDPerMTok: 3.75,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	want := `{"projects":["alpha"],"mode":"default","price":{"prompt_usd_per_mtok":3,"completion_usd_per_mtok":15,"cache_read_usd_per_mtok":0.3,"cache_write_usd_per_mtok":3.75}}`
	if string(b) != want {
		t.Errorf("wire drift:\n got %s\nwant %s", b, want)
	}
}

func TestArchitectStartPriceRoundTrip(t *testing.T) {
	in := ArchitectStart{Projects: []string{"alpha"}, Mode: ArchitectModeAuto, Price: &ModelPrice{PromptUSDPerMTok: 1.25, CompletionUSDPerMTok: 10}}

	b, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}

	var out ArchitectStart
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}

	if out.Price == nil || *out.Price != *in.Price {
		t.Errorf("price did not round-trip: got %+v want %+v", out.Price, in.Price)
	}
}

// Pins the research setting; TestArchitectStartZeroCostCapOmitted stays
// byte-identical, which proves research off sends nothing.
func TestArchitectStartResearchWireShape(t *testing.T) {
	in := ArchitectStart{Projects: []string{"alpha"}, Mode: ArchitectModeDefault, Research: &ResearchStart{Provider: "brave", APIKey: "k"}}

	b, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}

	want := `{"projects":["alpha"],"mode":"default","research":{"provider":"brave","api_key":"k"}}`
	if string(b) != want {
		t.Errorf("wire drift:\n got %s\nwant %s", b, want)
	}

	var out ArchitectStart
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(in, out) {
		t.Errorf("round-trip mismatch:\n got %+v\nwant %+v", out, in)
	}
}

// The window CM knows for the session's model rides the start; an unknown
// one stays off the wire.
func TestArchitectStartContextWindowWireShape(t *testing.T) {
	b, err := json.Marshal(ArchitectStart{Projects: []string{"alpha"}, Mode: ArchitectModeDefault, ContextWindow: 200000})
	if err != nil {
		t.Fatal(err)
	}

	if want := `{"projects":["alpha"],"mode":"default","context_window":200000}`; string(b) != want {
		t.Errorf("got %s want %s", b, want)
	}

	var back ArchitectStart
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}

	if back.ContextWindow != 200000 {
		t.Errorf("round trip lost the window: %+v", back)
	}
}

// A plain-language blueprint says so on every start; a start without it stays
// byte-identical (TestArchitectStartZeroCostCapOmitted).
func TestArchitectStartPlainLanguageWireShape(t *testing.T) {
	in := ArchitectStart{Projects: []string{"alpha"}, Mode: ArchitectModeDefault, PlainLanguage: true}

	b, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}

	if want := `{"projects":["alpha"],"mode":"default","plain_language":true}`; string(b) != want {
		t.Errorf("got %s want %s", b, want)
	}

	var back ArchitectStart
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(in, back) {
		t.Errorf("round trip: got %+v want %+v", back, in)
	}
}

func TestArchitectStartUnattendedOmittedWhenFalse(t *testing.T) {
	b, err := json.Marshal(ArchitectStart{Mode: ArchitectModeYolo})
	if err != nil {
		t.Fatal(err)
	}

	var keys map[string]json.RawMessage
	if err := json.Unmarshal(b, &keys); err != nil {
		t.Fatal(err)
	}

	if _, ok := keys["unattended"]; ok {
		t.Errorf("unattended on the wire when false: %s", b)
	}
}

func TestArchitectStartUnattendedRoundTrip(t *testing.T) {
	const wire = `{"projects":["alpha"],"mode":"yolo","unattended":true}`

	var in ArchitectStart
	if err := json.Unmarshal([]byte(wire), &in); err != nil {
		t.Fatal(err)
	}

	if !in.Unattended {
		t.Fatalf("decode lost unattended: %+v", in)
	}

	b, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}

	if string(b) != wire {
		t.Errorf("wire drift:\n got %s\nwant %s", b, wire)
	}
}
