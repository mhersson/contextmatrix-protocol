package protocol

import (
	"encoding/json"
	"testing"
	"time"
)

// The wire shapes are the contract: field tags must match what
// contextmatrix-runner's webhook handler decodes (internal/webhook/types.go).
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

// Pins the token-authority fields (multi-user CM). All optional: absent
// fields marshal to nothing, so pre-multi-user consumers see identical bytes.
func TestTriggerPayloadTokenAuthorityWireShape(t *testing.T) {
	legacy := TriggerPayload{CardID: "CM-001", Project: "alpha", RepoURL: "https://x/r.git"}
	b, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `{"card_id":"CM-001","project":"alpha","repo_url":"https://x/r.git"}` {
		t.Errorf("omitempty drift: %s", b)
	}

	full := TriggerPayload{
		CardID: "CM-001", Project: "alpha", RepoURL: "https://x/r.git",
		GitToken:          "ghs_short_lived",
		GitTokenExpiresAt: "2026-07-05T12:00:00Z",
		LLMEndpoint:       &LLMEndpoint{Type: "openrouter", BaseURL: "https://openrouter.ai/api/v1", APIKey: "sk-x"},
	}
	b, err = json.Marshal(full)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"card_id":"CM-001","project":"alpha","repo_url":"https://x/r.git",` +
		`"git_token":"ghs_short_lived","git_token_expires_at":"2026-07-05T12:00:00Z",` +
		`"llm_endpoint":{"type":"openrouter","base_url":"https://openrouter.ai/api/v1","api_key":"sk-x"}}`
	if string(b) != want {
		t.Errorf("wire drift:\n got %s\nwant %s", b, want)
	}

	var out TriggerPayload
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	if out.GitToken != full.GitToken || out.GitTokenExpiresAt != full.GitTokenExpiresAt ||
		out.LLMEndpoint == nil || *out.LLMEndpoint != *full.LLMEndpoint {
		t.Errorf("round-trip mismatch: %+v", out)
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

// Pins the MessagePayload wire shape: it carries session_id and omits
// empty card_id/project/message_id.
func TestMessagePayloadWireShape(t *testing.T) {
	full := MessagePayload{
		CardID: "CM-001", Project: "alpha", SessionID: "s1",
		Content: "hi", MessageID: "m1",
	}
	b, err := json.Marshal(full)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"card_id":"CM-001","project":"alpha","session_id":"s1",` +
		`"content":"hi","message_id":"m1"}`
	if string(b) != want {
		t.Errorf("wire drift:\n got %s\nwant %s", b, want)
	}

	chat := MessagePayload{SessionID: "s1", Content: "hi"}
	b, err = json.Marshal(chat)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `{"session_id":"s1","content":"hi"}` {
		t.Errorf("omitempty drift: %s", b)
	}
}

// Pins the original_seq tag — the only non-derivable tag in the module.
func TestChatResumeContextWireShape(t *testing.T) {
	c := ChatResumeContext{
		Turns:   []ChatResumeTurn{{Seq: 1, Role: "user", Content: "hi"}},
		Clipped: true,
		OrigSeq: 42,
	}
	b, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"turns":[{"seq":1,"role":"user","content":"hi"}],` +
		`"clipped":true,"original_seq":42}`
	if string(b) != want {
		t.Errorf("wire drift:\n got %s\nwant %s", b, want)
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

// Pins the optional llm_endpoint on chat start: absent marshals to nothing,
// so pre-multi-user consumers see identical bytes.
func TestChatStartPayloadLLMEndpointWireShape(t *testing.T) {
	b, err := json.Marshal(ChatStartPayload{SessionID: "s1"})
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `{"session_id":"s1"}` {
		t.Errorf("omitempty drift: %s", b)
	}

	full := ChatStartPayload{
		SessionID:   "s1",
		LLMEndpoint: &LLMEndpoint{Type: "openai", BaseURL: "https://llm.example/v1", APIKey: "k"},
	}
	b, err = json.Marshal(full)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"session_id":"s1",` +
		`"llm_endpoint":{"type":"openai","base_url":"https://llm.example/v1","api_key":"k"}}`
	if string(b) != want {
		t.Errorf("wire drift:\n got %s\nwant %s", b, want)
	}

	var out ChatStartPayload
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	if out.LLMEndpoint == nil || *out.LLMEndpoint != *full.LLMEndpoint {
		t.Errorf("round-trip mismatch: %+v", out)
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

func TestStatusCallbackPayloadWireShape(t *testing.T) {
	b, err := json.Marshal(StatusCallbackPayload{
		CardID: "CM-001", Project: "alpha", RunnerStatus: "running", Message: "started",
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"card_id":"CM-001","project":"alpha","runner_status":"running","message":"started"}`
	if string(b) != want {
		t.Errorf("wire drift:\n got %s\nwant %s", b, want)
	}
	// message omits when empty
	b, err = json.Marshal(StatusCallbackPayload{CardID: "c", Project: "p", RunnerStatus: "failed"})
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `{"card_id":"c","project":"p","runner_status":"failed"}` {
		t.Errorf("omitempty drift: %s", b)
	}
}

func TestSkillEngagedPayloadWireShape(t *testing.T) {
	b, err := json.Marshal(SkillEngagedPayload{CardID: "CM-001", Project: "alpha", SkillName: "go-development"})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"card_id":"CM-001","project":"alpha","skill_name":"go-development"}`
	if string(b) != want {
		t.Errorf("wire drift:\n got %s\nwant %s", b, want)
	}
}

func TestLogEntryWireShape(t *testing.T) {
	e := LogEntry{
		Timestamp: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
		CardID:    "CM-001",
		Project:   "alpha",
		SessionID: "s1",
		Type:      "text",
		Content:   "hi",
		ToolUseID: "tu1",
		Usage: &LogTokenUsage{
			InputTokens: 1, OutputTokens: 2, CacheReadTokens: 3, CacheCreateTokens: 4,
		},
		Model: "claude-sonnet-4-6",
	}
	b, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"ts":"2026-01-02T03:04:05Z","card_id":"CM-001","project":"alpha",` +
		`"session_id":"s1","type":"text","content":"hi","tool_use_id":"tu1",` +
		`"usage":{"input_tokens":1,"output_tokens":2,"cache_read_tokens":3,` +
		`"cache_creation_tokens":4},"model":"claude-sonnet-4-6"}`
	if string(b) != want {
		t.Errorf("wire drift:\n got %s\nwant %s", b, want)
	}
	// minimal frame: everything optional except ts and type
	b, err = json.Marshal(LogEntry{Timestamp: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC), Type: "system"})
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `{"ts":"2026-01-02T03:04:05Z","type":"system"}` {
		t.Errorf("omitempty drift: %s", b)
	}
}
