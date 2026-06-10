package protocol

import (
	"encoding/json"
	"testing"
)

// The wire shapes are the contract: field tags must match what cmr's
// webhook handler decodes today (cmr internal/webhook/types.go).
func TestTriggerPayloadWireShape(t *testing.T) {
	skills := []string{"s1"}
	p := TriggerPayload{
		CardID: "CM-001", Project: "alpha", RepoURL: "https://x/r.git",
		MCPAPIKey: "k", BaseBranch: "main", RunnerImage: "img", Interactive: true,
		Model: "m", TaskSkills: &skills,
	}
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"card_id":"CM-001","project":"alpha","repo_url":"https://x/r.git",` +
		`"mcp_api_key":"k","base_branch":"main","runner_image":"img",` +
		`"interactive":true,"model":"m","task_skills":["s1"]}`
	if string(b) != want {
		t.Errorf("wire drift:\n got %s\nwant %s", b, want)
	}
}

func TestMessagePayloadIsChat(t *testing.T) {
	if (MessagePayload{CardID: "c", Project: "p"}).IsChat() {
		t.Error("card-bound message reported as chat")
	}
	if !(MessagePayload{SessionID: "s"}).IsChat() {
		t.Error("session message not reported as chat")
	}
}

// Decoders must tolerate unknown fields (forward compatibility).
func TestDecodeToleratesUnknownFields(t *testing.T) {
	var p TriggerPayload
	if err := json.Unmarshal([]byte(`{"card_id":"c","future_field":42}`), &p); err != nil {
		t.Fatalf("unknown field broke decode: %v", err)
	}
	if p.CardID != "c" {
		t.Errorf("CardID = %q, want c", p.CardID)
	}
}

func TestErrorCodesAreStable(t *testing.T) {
	if CodeUnauthorized != "unauthorized" || CodeInvalidJSON != "invalid_json" {
		t.Error("stable code constants changed — this breaks clients")
	}
}

func TestChatStartResponseWireShape(t *testing.T) {
	b, err := json.Marshal(ChatStartResponse{OK: true, ContainerID: "abc"})
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `{"ok":true,"container_id":"abc"}` {
		t.Errorf("wire drift: %s", b)
	}
}

func TestErrorResponseWireShape(t *testing.T) {
	b, err := json.Marshal(ErrorResponse{OK: false, Code: CodeNotFound, Message: "no such card"})
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `{"ok":false,"code":"not_found","message":"no such card"}` {
		t.Errorf("wire drift: %s", b)
	}
}

func TestProtocolVersion(t *testing.T) {
	if ProtocolVersion != "1" || VersionHeader != "X-Protocol-Version" {
		t.Error("version contract changed")
	}
}
