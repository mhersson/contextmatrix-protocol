package protocol

import (
	"encoding/json"
	"testing"
)

func TestTaskSkillsSourceWirePin(t *testing.T) {
	full := TaskSkillsSource{
		GitRemoteURL:   "https://github.com/org/skills.git",
		Ref:            "abc123",
		Token:          "ghs_tok",
		TokenExpiresAt: "2026-07-17T10:00:00Z",
	}
	b, err := json.Marshal(full)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"git_remote_url":"https://github.com/org/skills.git",` +
		`"ref":"abc123","token":"ghs_tok","token_expires_at":"2026-07-17T10:00:00Z"}`
	if string(b) != want {
		t.Errorf("wire drift:\n got %s\nwant %s", b, want)
	}

	empty := TaskSkillsSource{}
	b, err = json.Marshal(empty)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `{"git_remote_url":"","ref":""}` {
		t.Errorf("url and ref always emitted; token fields omitted when empty, got %s", b)
	}
}
