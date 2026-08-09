package protocol

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"
)

// The wire shapes are the contract: field tags must match what the
// contextmatrix-agent and contextmatrix-chat webhook handlers decode.
func TestTriggerPayloadWireShape(t *testing.T) {
	skills := []string{"s1"}
	p := TriggerPayload{
		CardID: "CM-001", Project: "alpha", RepoURL: "https://x/r.git",
		MCPAPIKey: "k", BaseBranch: "main", WorkerImage: "img", Interactive: true,
		Model: "m", TaskSkills: &skills,
	}
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"card_id":"CM-001","project":"alpha","repo_url":"https://x/r.git",` +
		`"mcp_api_key":"k","base_branch":"main","worker_image":"img",` +
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

// Pins the original_seq tag - the only non-derivable tag in the module.
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

// Pins the v0.8.0 migration posture: payloads from a pre-v0.8.0 sender
// carrying the retired runner-era tags decode cleanly into the current
// structs with those keys ignored - never an error.
func TestDecodeToleratesLegacyRunnerTags(t *testing.T) {
	var trig TriggerPayload
	if err := json.Unmarshal([]byte(`{"card_id":"c","runner_image":"img"}`), &trig); err != nil {
		t.Fatalf("legacy runner_image broke decode: %v", err)
	}
	if trig.WorkerImage != "" {
		t.Errorf("legacy runner_image must be ignored, got %q", trig.WorkerImage)
	}

	var cb StatusCallbackPayload
	if err := json.Unmarshal([]byte(`{"card_id":"c","project":"p","runner_status":"running"}`), &cb); err != nil {
		t.Fatalf("legacy runner_status broke decode: %v", err)
	}
	if cb.WorkerStatus != "" {
		t.Errorf("legacy runner_status must be ignored, got %q", cb.WorkerStatus)
	}

	var cs ChatStartPayload
	if err := json.Unmarshal([]byte(`{"session_id":"s1","primer":"text","runner_image":"img"}`), &cs); err != nil {
		t.Fatalf("legacy primer/runner_image broke decode: %v", err)
	}
	if cs.WorkerImage != "" {
		t.Errorf("legacy runner_image must be ignored, got %q", cs.WorkerImage)
	}
}

func TestErrorCodesAreStable(t *testing.T) {
	if CodeUnauthorized != "unauthorized" || CodeInvalidJSON != "invalid_json" {
		t.Error("stable code constants changed - this breaks clients")
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

// Pins the git credentials token field on chat start: absent marshals to nothing,
// so pre-v0.5.2 consumers see identical bytes.
func TestChatStartPayloadGitCredentialsTokenWireShape(t *testing.T) {
	b, err := json.Marshal(ChatStartPayload{SessionID: "s1"})
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `{"session_id":"s1"}` {
		t.Errorf("omitempty drift: %s", b)
	}

	full := ChatStartPayload{
		SessionID:           "s1",
		GitCredentialsToken: "sess1.abcd1234",
	}
	b, err = json.Marshal(full)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"session_id":"s1","git_credentials_token":"sess1.abcd1234"}`
	if string(b) != want {
		t.Errorf("wire drift:\n got %s\nwant %s", b, want)
	}

	var out ChatStartPayload
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	if out.GitCredentialsToken != full.GitCredentialsToken {
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

func TestStatusCallbackPayloadWireShape(t *testing.T) {
	b, err := json.Marshal(StatusCallbackPayload{
		CardID: "CM-001", Project: "alpha", WorkerStatus: "running", Message: "started",
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"card_id":"CM-001","project":"alpha","worker_status":"running","message":"started"}`
	if string(b) != want {
		t.Errorf("wire drift:\n got %s\nwant %s", b, want)
	}
	// message omits when empty
	b, err = json.Marshal(StatusCallbackPayload{CardID: "c", Project: "p", WorkerStatus: "failed"})
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `{"card_id":"c","project":"p","worker_status":"failed"}` {
		t.Errorf("omitempty drift: %s", b)
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

func TestTriggerPayloadBestOfNWireShape(t *testing.T) {
	p := TriggerPayload{CardID: "CM-001", Project: "alpha", RepoURL: "https://x/r.git", BestOfN: 3}
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"card_id":"CM-001","project":"alpha","repo_url":"https://x/r.git","best_of_n":3}`
	if string(b) != want {
		t.Errorf("wire drift:\n got %s\nwant %s", b, want)
	}

	// Zero value must be absent from the wire.
	b, err = json.Marshal(TriggerPayload{CardID: "CM-001", Project: "alpha", RepoURL: "https://x/r.git"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "best_of_n") {
		t.Errorf("zero BestOfN must be omitted, got %s", b)
	}
}

// Pins the per-card max-capability flag: set marshals to max_capability and
// round-trips, false is absent from the wire so an older backend sees nothing.
func TestTriggerPayloadMaxCapabilityWireShape(t *testing.T) {
	p := TriggerPayload{CardID: "CM-001", Project: "alpha", RepoURL: "https://x/r.git", MaxCapability: true}

	b, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}

	want := `{"card_id":"CM-001","project":"alpha","repo_url":"https://x/r.git","max_capability":true}`
	if string(b) != want {
		t.Errorf("wire drift:\n got %s\nwant %s", b, want)
	}

	var got TriggerPayload
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}

	if !got.MaxCapability {
		t.Errorf("MaxCapability must round-trip, got %+v", got)
	}

	// Zero value must be absent from the wire.
	b, err = json.Marshal(TriggerPayload{CardID: "CM-001", Project: "alpha", RepoURL: "https://x/r.git"})
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(string(b), "max_capability") {
		t.Errorf("false MaxCapability must be omitted, got %s", b)
	}
}

// Pins the optional verify gate on trigger: absent marshals to nothing and
// decodes to nil, so pre-verify-gate consumers see identical bytes.
func TestTriggerPayloadVerifyWireShape(t *testing.T) {
	full := TriggerPayload{
		CardID: "CM-001", Project: "alpha", RepoURL: "https://x/r.git",
		Verify: &VerifyConfig{Command: "make test", TimeoutSeconds: 300, Env: []string{"JAVA_HOME"}},
	}
	b, err := json.Marshal(full)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"card_id":"CM-001","project":"alpha","repo_url":"https://x/r.git",` +
		`"verify":{"command":"make test","timeout_seconds":300,"env":["JAVA_HOME"]}}`
	if string(b) != want {
		t.Errorf("wire drift:\n got %s\nwant %s", b, want)
	}

	// Old producers omit verify; it must decode to nil.
	var out TriggerPayload
	if err := json.Unmarshal([]byte(`{"card_id":"CM-001","project":"alpha","repo_url":"https://x/r.git"}`), &out); err != nil {
		t.Fatal(err)
	}
	if out.Verify != nil {
		t.Errorf("absent verify must decode to nil, got %+v", out.Verify)
	}

	// Round-trip the populated config.
	out = TriggerPayload{}
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	if out.Verify == nil || out.Verify.Command != full.Verify.Command ||
		out.Verify.TimeoutSeconds != full.Verify.TimeoutSeconds ||
		len(out.Verify.Env) != 1 || out.Verify.Env[0] != "JAVA_HOME" {
		t.Errorf("round-trip mismatch: %+v", out.Verify)
	}
}

// Pins the per-project worker image override on chat start: absent marshals to
// nothing, so consumers predating the override see identical bytes.
func TestChatStartPayloadWorkerImageWireShape(t *testing.T) {
	b, err := json.Marshal(ChatStartPayload{SessionID: "s1"})
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `{"session_id":"s1"}` {
		t.Errorf("omitempty drift: %s", b)
	}

	full := ChatStartPayload{SessionID: "s1", WorkerImage: "ghcr.io/x/worker:latest"}
	b, err = json.Marshal(full)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `{"session_id":"s1","worker_image":"ghcr.io/x/worker:latest"}` {
		t.Errorf("wire drift: %s", b)
	}

	var out ChatStartPayload
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	if out.WorkerImage != full.WorkerImage {
		t.Errorf("round-trip mismatch: %+v", out)
	}
}

// Pins the mob session discussion spec on trigger (agent backend). Absent Mob
// marshals to nothing, so consumers that predate mob sessions see identical
// bytes - the base TestTriggerPayloadWireShape stays byte-identical and
// proves it.
func TestTriggerPayloadMobWireShape(t *testing.T) {
	p := TriggerPayload{
		CardID: "CM-001", Project: "alpha", RepoURL: "https://x/r.git",
		Mob: &MobSpec{
			Participants: 3,
			Phases:       []string{"plan", "review"},
			Rounds:       2,
			BudgetFactor: 0.75,
			Guests:       []GuestSpec{{Name: "laptop", URL: "http://192.168.1.50:8484", Token: "secret"}},
		},
	}
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"card_id":"CM-001","project":"alpha","repo_url":"https://x/r.git",` +
		`"mob":{"participants":3,"phases":["plan","review"],"rounds":2,` +
		`"budget_factor":0.75,"guests":[{"name":"laptop",` +
		`"url":"http://192.168.1.50:8484","token":"secret"}]}}`
	if string(b) != want {
		t.Errorf("wire drift:\n got %s\nwant %s", b, want)
	}

	// Nil Mob must be absent from the wire.
	b, err = json.Marshal(TriggerPayload{CardID: "CM-001", Project: "alpha", RepoURL: "https://x/r.git"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "mob") {
		t.Errorf("nil Mob must be omitted, got %s", b)
	}
}

// Round-trips a fully-populated MobSpec, including the forward-compat
// checkpoint fields and a token-less guest.
func TestMobSpecRoundTrip(t *testing.T) {
	in := MobSpec{
		Participants:       4,
		Phases:             []string{"plan", "review", "execute"},
		Rounds:             3,
		BudgetFactor:       1.25,
		ExecuteCheckpoints: true,
		CheckpointMinTier:  "complex",
		Guests: []GuestSpec{
			{Name: "laptop", URL: "http://192.168.1.50:8484", Token: "secret"},
			{Name: "lab", URL: "https://lab.example:8484"},
		},
	}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}

	var out MobSpec
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(in, out) {
		t.Errorf("round-trip mismatch:\n got %+v\nwant %+v", out, in)
	}
}

// Pins speaker attribution on log frames. Absent Agent marshals to nothing -
// TestLogEntryWireShape stays byte-identical and proves the additive change;
// this guards the omitempty tag directly.
func TestLogEntryAgentWireShape(t *testing.T) {
	e := LogEntry{
		Timestamp: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
		Type:      "text",
		Content:   "the plan looks solid",
		Agent:     "seat-2",
	}
	b, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"ts":"2026-01-02T03:04:05Z","type":"text","content":"the plan looks solid","agent":"seat-2"}`
	if string(b) != want {
		t.Errorf("wire drift:\n got %s\nwant %s", b, want)
	}

	// Empty Agent must be absent from the wire.
	b, err = json.Marshal(LogEntry{Timestamp: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC), Type: "system"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "agent") {
		t.Errorf("empty Agent must be omitted, got %s", b)
	}
}

func TestListImagesResponseWireShape(t *testing.T) {
	b, err := json.Marshal(ListImagesResponse{OK: true, Images: []ImageListItem{{
		Tags:    []string{"contextmatrix-agent-worker:go-node"},
		Digests: []string{"contextmatrix-agent-worker@sha256:abc"},
		Created: 1750000000,
		Size:    2560000000,
	}}})
	if err != nil {
		t.Fatal(err)
	}

	want := `{"ok":true,"images":[{"tags":["contextmatrix-agent-worker:go-node"],` +
		`"digests":["contextmatrix-agent-worker@sha256:abc"],` +
		`"created":1750000000,"size":2560000000}]}`
	if string(b) != want {
		t.Errorf("wire drift:\n got %s\nwant %s", b, want)
	}
}

func TestListImagesResponseWireShape_OmitsEmptyOptionals(t *testing.T) {
	b, err := json.Marshal(ListImagesResponse{OK: true, Images: []ImageListItem{{
		Tags: []string{"contextmatrix-chat-worker:dev"},
	}}})
	if err != nil {
		t.Fatal(err)
	}

	if string(b) != `{"ok":true,"images":[{"tags":["contextmatrix-chat-worker:dev"]}]}` {
		t.Errorf("wire drift: %s", b)
	}
}
