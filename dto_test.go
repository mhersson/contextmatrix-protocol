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

func TestChangeKindsAreStable(t *testing.T) {
	if ChangeKindFeat != "feat" || ChangeKindFix != "fix" || ChangeKindChange != "change" {
		t.Error("change kind constants changed - this breaks CM's About popup")
	}
}

// CM shows "version unknown" exactly when Build is nil, so a body from a backend
// older than these fields must decode to nil, not to an empty BuildInfo.
func TestHealthResponseDecodesLegacyBody(t *testing.T) {
	var hr HealthResponse
	if err := json.Unmarshal([]byte(`{"ok":true,"running_containers":1,"max_concurrent":4}`), &hr); err != nil {
		t.Fatalf("legacy health body broke decode: %v", err)
	}

	if hr.Build != nil || hr.WorkerImageRevision != "" {
		t.Errorf("legacy body must leave build nil and revision empty, got %+v", hr)
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

func TestLogTokenUsageCostWireShape(t *testing.T) {
	// A tiny OpenRouter-sized figure keeps its digits.
	u := LogTokenUsage{InputTokens: 1, OutputTokens: 2, CacheReadTokens: 3, CacheCreateTokens: 4, CostUSD: 0.000123}

	b, err := json.Marshal(u)
	if err != nil {
		t.Fatal(err)
	}

	want := `{"input_tokens":1,"output_tokens":2,"cache_read_tokens":3,"cache_creation_tokens":4,"cost_usd":0.000123}`
	if string(b) != want {
		t.Errorf("wire drift:\n got %s\nwant %s", b, want)
	}

	// No reported cost: byte-identical to the shape before the field existed.
	u.CostUSD = 0

	b, err = json.Marshal(u)
	if err != nil {
		t.Fatal(err)
	}

	want = `{"input_tokens":1,"output_tokens":2,"cache_read_tokens":3,"cache_creation_tokens":4}`
	if string(b) != want {
		t.Errorf("omitempty drift:\n got %s\nwant %s", b, want)
	}
}

func TestLogTokenUsageCostDecode(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want float64
	}{
		{"legacy frame without cost", `{"input_tokens":1,"output_tokens":2,"cache_read_tokens":3,"cache_creation_tokens":4}`, 0},
		{"frame with cost", `{"input_tokens":1,"output_tokens":2,"cache_read_tokens":3,"cache_creation_tokens":4,"cost_usd":0.000123}`, 0.000123},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var u LogTokenUsage
			if err := json.Unmarshal([]byte(tc.in), &u); err != nil {
				t.Fatal(err)
			}

			if u.CostUSD != tc.want {
				t.Errorf("CostUSD = %v, want %v", u.CostUSD, tc.want)
			}
		})
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

// Pins the project POC flag: set marshals to poc and round-trips, false is
// absent from the wire so an older backend sees nothing.
func TestTriggerPayloadPOCWireShape(t *testing.T) {
	p := TriggerPayload{CardID: "CM-001", Project: "alpha", RepoURL: "https://x/r.git", POC: true}

	b, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}

	want := `{"card_id":"CM-001","project":"alpha","repo_url":"https://x/r.git","poc":true}`
	if string(b) != want {
		t.Errorf("wire drift:\n got %s\nwant %s", b, want)
	}

	var got TriggerPayload
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}

	if !got.POC {
		t.Errorf("POC must round-trip, got %+v", got)
	}

	b, err = json.Marshal(TriggerPayload{CardID: "CM-001", Project: "alpha", RepoURL: "https://x/r.git"})
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(string(b), "poc") {
		t.Errorf("false POC must be omitted, got %s", b)
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

// Pins reply-button prompts on text frames. TestLogEntryWireShape stays
// byte-identical, which proves a frame without a prompt is unchanged.
func TestLogEntryPromptWireShape(t *testing.T) {
	ts := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

	tests := []struct {
		name   string
		prompt *LogPrompt
		want   string
	}{
		{
			name: "gate",
			prompt: &LogPrompt{ID: "gate:sign-off:v2", Actions: []LogAction{
				{Label: "Approve", Reply: "approve"},
				{Label: "Reject", Reply: "reject", Feedback: true},
			}},
			want: `{"ts":"2026-01-02T03:04:05Z","type":"text","content":"Pick one.",` +
				`"prompt":{"id":"gate:sign-off:v2","actions":[{"label":"Approve","reply":"approve"},` +
				`{"label":"Reject","reply":"reject","feedback":true}]}}`,
		},
		{
			name:   "free-text question",
			prompt: &LogPrompt{ID: "Q3", FreeText: true, Actions: []LogAction{{Label: "Postgres", Reply: "Postgres"}}},
			want: `{"ts":"2026-01-02T03:04:05Z","type":"text","content":"Pick one.",` +
				`"prompt":{"id":"Q3","free_text":true,"actions":[{"label":"Postgres","reply":"Postgres"}]}}`,
		},
		{
			name:   "no id and no actions",
			prompt: &LogPrompt{FreeText: true},
			want:   `{"ts":"2026-01-02T03:04:05Z","type":"text","content":"Pick one.","prompt":{"free_text":true}}`,
		},
		{
			name: "display fields",
			prompt: &LogPrompt{ID: "Q1", Kind: "question", Text: "Which database?", FreeText: true, Actions: []LogAction{
				{Label: "Postgres", Reply: "1", Recommended: true},
				{Label: "SQLite", Reply: "2"},
			}},
			want: `{"ts":"2026-01-02T03:04:05Z","type":"text","content":"Pick one.",` +
				`"prompt":{"id":"Q1","kind":"question","text":"Which database?","free_text":true,` +
				`"actions":[{"label":"Postgres","reply":"1","recommended":true},{"label":"SQLite","reply":"2"}]}}`,
		},
		{
			name: "a feedback box with its own label",
			prompt: &LogPrompt{ID: "gate:signoff:v2", Kind: "gate", Actions: []LogAction{
				{Label: "Approve over critical findings", Reply: "approve", Feedback: true, Ask: "Why approve over the critical findings?"},
				{Label: "Reject", Reply: "reject", Feedback: true},
			}},
			want: `{"ts":"2026-01-02T03:04:05Z","type":"text","content":"Pick one.",` +
				`"prompt":{"id":"gate:signoff:v2","kind":"gate","actions":[` +
				`{"label":"Approve over critical findings","reply":"approve","feedback":true,"ask":"Why approve over the critical findings?"},` +
				`{"label":"Reject","reply":"reject","feedback":true}]}}`,
		},
		{
			name: "a plain prompt with its technical detail",
			prompt: &LogPrompt{
				ID: "gate:signoff:v3", Kind: "gate", Text: "The architect could not settle part of the design on its own.",
				Detail:  "X2 (critical): DEC2 keeps field order but D3 sorts keys.",
				Actions: []LogAction{{Label: "Continue anyway", Reply: "approve"}, {Label: "Stop", Reply: "reject"}},
			},
			want: `{"ts":"2026-01-02T03:04:05Z","type":"text","content":"Pick one.",` +
				`"prompt":{"id":"gate:signoff:v3","kind":"gate","text":"The architect could not settle part of the design on its own.",` +
				`"detail":"X2 (critical): DEC2 keeps field order but D3 sorts keys.",` +
				`"actions":[{"label":"Continue anyway","reply":"approve"},{"label":"Stop","reply":"reject"}]}}`,
		},
		{
			name: "absent",
			want: `{"ts":"2026-01-02T03:04:05Z","type":"text","content":"Pick one."}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := LogEntry{Timestamp: ts, Type: "text", Content: "Pick one.", Prompt: tt.prompt}

			b, err := json.Marshal(in)
			if err != nil {
				t.Fatal(err)
			}

			if string(b) != tt.want {
				t.Errorf("wire drift:\n got %s\nwant %s", b, tt.want)
			}

			var out LogEntry
			if err := json.Unmarshal([]byte(tt.want), &out); err != nil {
				t.Fatal(err)
			}

			if !reflect.DeepEqual(in, out) {
				t.Errorf("round-trip mismatch:\n got %+v\nwant %+v", out, in)
			}
		})
	}
}

// Pins blueprint transcript markers. TestLogEntryWireShape stays
// byte-identical, which proves a frame without a marker is unchanged.
func TestLogEntryMarkerWireShape(t *testing.T) {
	ts := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

	tests := []struct {
		name    string
		typ     string
		content string
		marker  *LogMarker
		want    string
	}{
		{
			name:    "stage divider",
			typ:     "system",
			content: "Design - System designer",
			marker:  &LogMarker{Kind: "stage", Stage: "solution_design", Persona: "System designer"},
			want: `{"ts":"2026-01-02T03:04:05Z","type":"system","content":"Design - System designer",` +
				`"marker":{"kind":"stage","stage":"solution_design","persona":"System designer"}}`,
		},
		{
			name:    "sheet reference",
			typ:     "text",
			content: "Design rev 2: revised for the 3 findings",
			marker:  &LogMarker{Kind: "artifact", Stage: "solution_design", Version: 2, Note: "Revised for the 3 findings"},
			want: `{"ts":"2026-01-02T03:04:05Z","type":"text","content":"Design rev 2: revised for the 3 findings",` +
				`"marker":{"kind":"artifact","stage":"solution_design","version":2,"note":"Revised for the 3 findings"}}`,
		},
		{
			name:    "a slice's sheet reference",
			typ:     "text",
			content: "Acceptance (SL2) rev 1: drawn by senior developer",
			marker:  &LogMarker{Kind: "artifact", Stage: "acceptance", Slice: "SL2", Version: 1, Note: "Drawn by senior developer"},
			want: `{"ts":"2026-01-02T03:04:05Z","type":"text","content":"Acceptance (SL2) rev 1: drawn by senior developer",` +
				`"marker":{"kind":"artifact","stage":"acceptance","slice":"SL2","version":1,"note":"Drawn by senior developer"}}`,
		},
		{
			name:    "a review",
			typ:     "text",
			content: "The reviewer scored Acceptance (SL2) version 3 4 of 5 (passes at 4), round 1.",
			marker:  &LogMarker{Kind: "review", Stage: "acceptance", Slice: "SL2", Persona: "tester", Version: 3},
			want: `{"ts":"2026-01-02T03:04:05Z","type":"text","content":"The reviewer scored Acceptance (SL2) version 3 4 of 5 (passes at 4), round 1.",` +
				`"marker":{"kind":"review","stage":"acceptance","slice":"SL2","persona":"tester","version":3}}`,
		},
		{
			name:    "absent",
			typ:     "text",
			content: "Design rev 2",
			want:    `{"ts":"2026-01-02T03:04:05Z","type":"text","content":"Design rev 2"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := LogEntry{Timestamp: ts, Type: tt.typ, Content: tt.content, Marker: tt.marker}

			b, err := json.Marshal(in)
			if err != nil {
				t.Fatal(err)
			}

			if string(b) != tt.want {
				t.Errorf("wire drift:\n got %s\nwant %s", b, tt.want)
			}

			var out LogEntry
			if err := json.Unmarshal([]byte(tt.want), &out); err != nil {
				t.Fatal(err)
			}

			if !reflect.DeepEqual(in, out) {
				t.Errorf("round-trip mismatch:\n got %+v\nwant %+v", out, in)
			}
		})
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

// Pins the playbook base-branch fields: set together they marshal right after
// base_branch and round-trip; absent they add nothing to the wire, so a
// backend that predates them sees byte-identical JSON. A legacy payload
// without them decodes to the zero values.
func TestTriggerPayloadCreateBaseBranchWireShape(t *testing.T) {
	p := TriggerPayload{
		CardID: "CM-001", Project: "alpha", RepoURL: "https://x/r.git",
		BaseBranch: "playbook/rollout", CreateBaseBranch: true, BaseBranchFrom: "main",
	}

	b, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}

	want := `{"card_id":"CM-001","project":"alpha","repo_url":"https://x/r.git","base_branch":"playbook/rollout","create_base_branch":true,"base_branch_from":"main"}`
	if string(b) != want {
		t.Errorf("wire drift:\n got %s\nwant %s", b, want)
	}

	var out TriggerPayload
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}

	if !out.CreateBaseBranch || out.BaseBranchFrom != "main" {
		t.Errorf("round trip lost the fields: %+v", out)
	}

	// Zero values must be absent from the wire.
	b, err = json.Marshal(TriggerPayload{CardID: "CM-001", Project: "alpha", RepoURL: "https://x/r.git", BaseBranch: "main"})
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(string(b), "create_base_branch") || strings.Contains(string(b), "base_branch_from") {
		t.Errorf("zero playbook fields must be omitted, got %s", b)
	}

	// A legacy payload without the fields decodes to the zero values.
	var legacy TriggerPayload
	if err := json.Unmarshal([]byte(`{"card_id":"CM-001","project":"alpha","repo_url":"https://x/r.git","base_branch":"main"}`), &legacy); err != nil {
		t.Fatal(err)
	}

	if legacy.CreateBaseBranch || legacy.BaseBranchFrom != "" {
		t.Errorf("legacy payload must decode to zero playbook fields, got %+v", legacy)
	}
}

// Pins the lifecycle payloads and webhook response bodies: tags, field order,
// and omitempty behavior for the zero and fully populated value of each.
func TestLifecycleAndResponseDTOsWireShape(t *testing.T) {
	tests := []struct {
		name string
		v    any
		want string
	}{
		{"KillPayload zero", KillPayload{}, `{"card_id":"","project":""}`},
		{
			"KillPayload full",
			KillPayload{CardID: "CM-001", Project: "alpha"},
			`{"card_id":"CM-001","project":"alpha"}`,
		},
		{"StopAllPayload zero", StopAllPayload{}, `{}`},
		{"StopAllPayload full", StopAllPayload{Project: "alpha"}, `{"project":"alpha"}`},
		{"PromotePayload zero", PromotePayload{}, `{"card_id":"","project":""}`},
		{
			"PromotePayload full",
			PromotePayload{CardID: "CM-001", Project: "alpha"},
			`{"card_id":"CM-001","project":"alpha"}`,
		},
		{"EndSessionPayload zero", EndSessionPayload{}, `{"card_id":"","project":""}`},
		{
			"EndSessionPayload full",
			EndSessionPayload{CardID: "CM-001", Project: "alpha"},
			`{"card_id":"CM-001","project":"alpha"}`,
		},
		{"ChatEndPayload zero", ChatEndPayload{}, `{"session_id":""}`},
		{"ChatEndPayload full", ChatEndPayload{SessionID: "sess-1"}, `{"session_id":"sess-1"}`},
		{"SuccessResponse zero", SuccessResponse{}, `{"ok":false}`},
		{
			"SuccessResponse full",
			SuccessResponse{OK: true, Message: "done", MessageID: "msg-1"},
			`{"ok":true,"message":"done","message_id":"msg-1"}`,
		},
		{"CardKillResult zero", CardKillResult{}, `{"ok":false}`},
		{
			"CardKillResult full",
			CardKillResult{CardID: "CM-001", Project: "alpha", OK: true, Error: "boom"},
			`{"card_id":"CM-001","project":"alpha","ok":true,"error":"boom"}`,
		},
		{
			"ContainerListItem zero",
			ContainerListItem{},
			`{"container_id":"","card_id":"","project":"","state":"","started_at":"","tracked":false}`,
		},
		{
			"ContainerListItem full",
			ContainerListItem{
				ContainerID: "c1", CardID: "CM-001", SessionID: "sess-1", Project: "alpha",
				State: "running", StartedAt: "2026-01-02T03:04:05Z", Tracked: true,
			},
			`{"container_id":"c1","card_id":"CM-001","session_id":"sess-1","project":"alpha",` +
				`"state":"running","started_at":"2026-01-02T03:04:05Z","tracked":true}`,
		},
		{"ListContainersResponse zero", ListContainersResponse{}, `{"ok":false,"containers":null}`},
		{
			"ListContainersResponse full",
			ListContainersResponse{OK: true, Containers: []ContainerListItem{{
				ContainerID: "c1", CardID: "CM-001", SessionID: "sess-1", Project: "alpha",
				State: "running", StartedAt: "2026-01-02T03:04:05Z", Tracked: true,
			}}},
			`{"ok":true,"containers":[{"container_id":"c1","card_id":"CM-001","session_id":"sess-1",` +
				`"project":"alpha","state":"running","started_at":"2026-01-02T03:04:05Z","tracked":true}]}`,
		},
		{
			"StopAllResponse zero",
			StopAllResponse{},
			`{"ok":false,"total":0,"stopped":0,"failed":0,"results":null}`,
		},
		{
			"StopAllResponse full",
			StopAllResponse{OK: true, Total: 2, Stopped: 1, Failed: 1, Results: []CardKillResult{
				{CardID: "CM-001", Project: "alpha", OK: true},
				{CardID: "CM-002", Project: "alpha", Error: "boom"},
			}},
			`{"ok":true,"total":2,"stopped":1,"failed":1,"results":[` +
				`{"card_id":"CM-001","project":"alpha","ok":true},` +
				`{"card_id":"CM-002","project":"alpha","ok":false,"error":"boom"}]}`,
		},
		{
			"HealthResponse zero",
			HealthResponse{},
			`{"ok":false,"running_containers":0,"max_concurrent":0}`,
		},
		{
			"HealthResponse full",
			HealthResponse{OK: true, RunningContainers: 3, MaxConcurrent: 8},
			`{"ok":true,"running_containers":3,"max_concurrent":8}`,
		},
		{"BuildInfo zero", BuildInfo{}, `{"changes":null}`},
		{"BuildInfo no history", BuildInfo{Changes: []Change{}}, `{"changes":[]}`},
		{"Change zero", Change{}, `{"number":0,"kind":"","title":"","merged_at":""}`},
		{
			"HealthResponse with build",
			HealthResponse{
				OK: true, RunningContainers: 1, MaxConcurrent: 4,
				Build: &BuildInfo{
					Version:   "v1.2.0",
					Commit:    "0123456789abcdef0123456789abcdef01234567",
					BuildTime: "2026-10-21T10:00:00Z",
					Changes: []Change{{
						Number: 26, Kind: ChangeKindFeat,
						Title: "retain a per-key backlog", MergedAt: "2026-10-17T14:04:32Z",
					}},
				},
				WorkerImageRevision: "fedcba9876543210fedcba9876543210fedcba98",
			},
			`{"ok":true,"running_containers":1,"max_concurrent":4,"build":{"version":"v1.2.0",` +
				`"commit":"0123456789abcdef0123456789abcdef01234567","build_time":"2026-10-21T10:00:00Z",` +
				`"changes":[{"number":26,"kind":"feat","title":"retain a per-key backlog",` +
				`"merged_at":"2026-10-17T14:04:32Z"}]},` +
				`"worker_image_revision":"fedcba9876543210fedcba9876543210fedcba98"}`,
		},
		{
			"HealthResponse unstamped build",
			HealthResponse{OK: true, Build: &BuildInfo{Changes: []Change{}}},
			`{"ok":true,"running_containers":0,"max_concurrent":0,"build":{"changes":[]}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := json.Marshal(tt.v)
			if err != nil {
				t.Fatal(err)
			}

			if string(b) != tt.want {
				t.Errorf("wire drift:\n got %s\nwant %s", b, tt.want)
			}
		})
	}
}

// Pins question-round prompts. TestLogEntryPromptWireShape stays
// byte-identical, which proves a prompt without a round is unchanged.
func TestLogPromptRoundWireShape(t *testing.T) {
	tests := []struct {
		name string
		in   LogPrompt
		want string
	}{
		{
			name: "full",
			in: LogPrompt{ID: "round:2", Kind: "round", Round: &LogRound{
				Persona: "system designer", Stage: "solution_design",
				Questions: []RoundQuestion{{
					Number: 3, Title: "Postgres", Text: "How should Postgres run?", Why: "Sessions live there.",
					Options: []RoundOption{
						{
							Title: "Operator", Summary: "Failover and backups.", Pros: []string{"failover"}, Cons: []string{"CRDs"},
							Sources: []RoundSource{{Tag: "S3", Title: "cloudnative-pg.io", URL: "https://cloudnative-pg.io"}, {Tag: "S1", Title: "alpha"}},
						},
						{Title: "StatefulSet"},
					},
					Recommended: 1,
				}},
				Assumptions: []RoundAssumption{{ID: "A1", Text: "One region."}},
				Answers:     []RoundAnswer{{Number: 3, Option: 1}, {Number: 4, Text: "my own"}},
				Decisions:   []AssumptionDecision{{ID: "A1", Accept: true}, {ID: "A2"}},
				Chat:        []RoundChat{{From: "user", Text: "Why?"}, {From: "system_designer", Text: "Because."}},
				ResearchOff: true, Responder: true, Closed: true,
			}},
			want: `{"id":"round:2","kind":"round","round":{"persona":"system designer","stage":"solution_design",` +
				`"questions":[{"number":3,"title":"Postgres","text":"How should Postgres run?","why":"Sessions live there.",` +
				`"options":[{"title":"Operator","summary":"Failover and backups.","pros":["failover"],"cons":["CRDs"],` +
				`"sources":[{"tag":"S3","title":"cloudnative-pg.io","url":"https://cloudnative-pg.io"},{"tag":"S1","title":"alpha"}]},` +
				`{"title":"StatefulSet"}],"recommended":1}],` +
				`"assumptions":[{"id":"A1","text":"One region."}],` +
				`"answers":[{"number":3,"option":1},{"number":4,"text":"my own"}],` +
				`"decisions":[{"id":"A1","accept":true},{"id":"A2","accept":false}],` +
				`"chat":[{"from":"user","text":"Why?"},{"from":"system_designer","text":"Because."}],` +
				`"research_off":true,"responder":true,"closed":true}}`,
		},
		{
			name: "minimal",
			in: LogPrompt{ID: "round:1", Kind: "round", Round: &LogRound{
				Persona: "product owner", Stage: "intake",
				Questions: []RoundQuestion{{Number: 1, Title: "Scope", Text: "Which users?", Options: []RoundOption{}}},
			}},
			want: `{"id":"round:1","kind":"round","round":{"persona":"product owner","stage":"intake",` +
				`"questions":[{"number":1,"title":"Scope","text":"Which users?","options":[]}]}}`,
		},
		{
			name: "a slice's round",
			in: LogPrompt{ID: "round:4", Kind: "round", Round: &LogRound{
				Persona: "system designer", Stage: "solution_design", Slice: "SL2",
				Questions: []RoundQuestion{{Number: 9, Title: "Format", Text: "CSV or JSON?", Options: []RoundOption{}}},
			}},
			want: `{"id":"round:4","kind":"round","round":{"persona":"system designer","stage":"solution_design","slice":"SL2",` +
				`"questions":[{"number":9,"title":"Format","text":"CSV or JSON?","options":[]}]}}`,
		},
		{
			name: "a plain round's consequence with its technical detail",
			in: LogPrompt{ID: "round:3", Kind: "round", Round: &LogRound{
				Persona: "product owner", Stage: "intake", Questions: []RoundQuestion{},
				Assumptions: []RoundAssumption{{
					ID: "A1", Text: "Both phones see changes within seconds, but adding items needs internet.",
					Detail: "Items sync through a hosted database; the app keeps no offline copy.",
				}},
			}},
			want: `{"id":"round:3","kind":"round","round":{"persona":"product owner","stage":"intake","questions":[],` +
				`"assumptions":[{"id":"A1","text":"Both phones see changes within seconds, but adding items needs internet.",` +
				`"detail":"Items sync through a hosted database; the app keeps no offline copy."}]}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := json.Marshal(tt.in)
			if err != nil {
				t.Fatal(err)
			}

			if string(b) != tt.want {
				t.Errorf("wire drift:\n got %s\nwant %s", b, tt.want)
			}

			var out LogPrompt
			if err := json.Unmarshal([]byte(tt.want), &out); err != nil {
				t.Fatal(err)
			}

			if !reflect.DeepEqual(tt.in, out) {
				t.Errorf("round-trip mismatch:\n got %+v\nwant %+v", out, tt.in)
			}
		})
	}
}

// Pins a question-round reply. TestMessagePayloadWireShape stays
// byte-identical, which proves a message without a round is unchanged.
func TestMessagePayloadRoundWireShape(t *testing.T) {
	tests := []struct {
		name string
		in   MessagePayload
		want string
	}{
		{
			name: "final",
			in: MessagePayload{SessionID: "s1", Content: "Q3: Operator", MessageID: "m1", Round: &RoundReply{
				PromptID: "round:2", Answers: []RoundAnswer{{Number: 3, Option: 1}},
				Decisions: []AssumptionDecision{{ID: "A1", Accept: true}}, Final: true,
			}},
			want: `{"session_id":"s1","content":"Q3: Operator","message_id":"m1",` +
				`"round":{"prompt_id":"round:2","answers":[{"number":3,"option":1}],"decisions":[{"id":"A1","accept":true}],"final":true}}`,
		},
		{
			name: "partial without entries",
			in:   MessagePayload{SessionID: "s1", Content: "", Round: &RoundReply{PromptID: "round:2"}},
			want: `{"session_id":"s1","content":"","round":{"prompt_id":"round:2"}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := json.Marshal(tt.in)
			if err != nil {
				t.Fatal(err)
			}

			if string(b) != tt.want {
				t.Errorf("wire drift:\n got %s\nwant %s", b, tt.want)
			}

			var out MessagePayload
			if err := json.Unmarshal([]byte(tt.want), &out); err != nil {
				t.Fatal(err)
			}

			if !reflect.DeepEqual(tt.in, out) {
				t.Errorf("round-trip mismatch:\n got %+v\nwant %+v", out, tt.in)
			}
		})
	}
}

// Pins a mockup reply and its action values. TestMessagePayloadWireShape stays
// byte-identical, which proves a message without a mockup is unchanged.
func TestMessagePayloadMockupWireShape(t *testing.T) {
	tests := []struct {
		name string
		in   MessagePayload
		want string
	}{
		{
			name: "use",
			in: MessagePayload{SessionID: "s1", Content: "Use this design", MessageID: "m1", Mockup: &MockupReply{
				PromptID: "mockup:1", Revision: 2, Action: MockupUse,
				Settings: map[string]string{"layout": "cards", "group": "on"},
			}},
			want: `{"session_id":"s1","content":"Use this design","message_id":"m1",` +
				`"mockup":{"prompt_id":"mockup:1","revision":2,"action":"use","settings":{"group":"on","layout":"cards"}}}`,
		},
		{
			name: "skip omits settings",
			in: MessagePayload{SessionID: "s1", Content: "Skip mockups", Mockup: &MockupReply{
				PromptID: "mockup:1", Revision: 2, Action: MockupSkip,
			}},
			want: `{"session_id":"s1","content":"Skip mockups",` +
				`"mockup":{"prompt_id":"mockup:1","revision":2,"action":"skip"}}`,
		},
		{
			name: "chat",
			in: MessagePayload{SessionID: "s1", Content: "Make the rows taller", Mockup: &MockupReply{
				PromptID: "mockup:1", Revision: 1, Action: MockupChat,
				Settings: map[string]string{"layout": "rows"},
			}},
			want: `{"session_id":"s1","content":"Make the rows taller",` +
				`"mockup":{"prompt_id":"mockup:1","revision":1,"action":"chat","settings":{"layout":"rows"}}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := json.Marshal(tt.in)
			if err != nil {
				t.Fatal(err)
			}

			if string(b) != tt.want {
				t.Errorf("wire drift:\n got %s\nwant %s", b, tt.want)
			}

			var out MessagePayload
			if err := json.Unmarshal([]byte(tt.want), &out); err != nil {
				t.Fatal(err)
			}

			if !reflect.DeepEqual(tt.in, out) {
				t.Errorf("round-trip mismatch:\n got %+v\nwant %+v", out, tt.in)
			}
		})
	}
}
