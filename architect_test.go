package protocol

import (
	"encoding/json"
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
