package changelog

import (
	"bytes"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	protocol "github.com/mhersson/contextmatrix-protocol"
)

const (
	d  = "2026-10-17T16:04:32+02:00" // what %cI prints for a CEST merge
	du = "2026-10-17T14:04:32Z"
)

// rec builds one record the way GitLogArgs makes git print it.
func rec(date, subject, body string) string {
	return date + "\x1f" + subject + "\x1f" + body + "\x1e\n"
}

func subj(n int) string {
	return fmt.Sprintf("Merge pull request #%d from mhersson/feat/x", n)
}

func ch(n int, kind, title, mergedAt string) protocol.Change {
	return protocol.Change{Number: n, Kind: kind, Title: title, MergedAt: mergedAt}
}

func committedAt(date string) []string {
	return []string{"GIT_COMMITTER_DATE=" + date}
}

func TestParse(t *testing.T) {
	const (
		feat   = protocol.ChangeKindFeat
		fix    = protocol.ChangeKindFix
		change = protocol.ChangeKindChange
	)

	none := []protocol.Change{}

	tests := []struct {
		name  string
		raw   string
		limit int
		want  []protocol.Change
	}{
		{
			"feat with scope",
			rec(d, subj(26), "feat(logbridge): retain a per-key backlog\n"), Limit,
			[]protocol.Change{ch(26, feat, "retain a per-key backlog", du)},
		},
		{
			"fix without scope",
			rec(d, subj(7), "fix: requeue unread entries"), Limit,
			[]protocol.Change{ch(7, fix, "requeue unread entries", du)},
		},
		{
			"breaking marker",
			rec(d, subj(7), "feat(api)!: drop the legacy field"), Limit,
			[]protocol.Change{ch(7, feat, "drop the legacy field", du)},
		},
		{
			"capitalized prefix is a card title",
			rec(d, subj(7), "Fix: handle empty input"), Limit,
			[]protocol.Change{ch(7, change, "Fix: handle empty input", du)},
		},
		{
			"perf type is change",
			rec(d, subj(7), "perf(hub): faster fan-out"), Limit,
			[]protocol.Change{ch(7, change, "faster fan-out", du)},
		},
		{
			"revert type is change",
			rec(d, subj(7), "revert: undo the popup"), Limit,
			[]protocol.Change{ch(7, change, "undo the popup", du)},
		},
		{
			"unknown prefix is a card title",
			rec(d, subj(7), "pr_gates: Copilot gate is skipped"), Limit,
			[]protocol.Change{ch(7, change, "pr_gates: Copilot gate is skipped", du)},
		},
		{
			"noise types skipped",
			rec(d, subj(1), "ci: a") + rec(d, subj(2), "docs: a") + rec(d, subj(3), "chore(deps): a") +
				rec(d, subj(4), "build: a") + rec(d, subj(5), "test: a") + rec(d, subj(6), "refactor: a") +
				rec(d, subj(7), "style: a") + rec(d, subj(8), "feat: kept"),
			Limit,
			[]protocol.Change{ch(8, feat, "kept", du)},
		},
		{
			"capitalized noise prefix is a card title",
			rec(d, subj(7), "Chore: tidy"), Limit,
			[]protocol.Change{ch(7, change, "Chore: tidy", du)},
		},
		{
			"unprefixed card title",
			rec(d, subj(7), "Show the About popup"), Limit,
			[]protocol.Change{ch(7, change, "Show the About popup", du)},
		},
		{
			"colon after a space is no prefix",
			rec(d, subj(7), "Fix login: handle empty password"), Limit,
			[]protocol.Change{ch(7, change, "Fix login: handle empty password", du)},
		},
		{
			"dependabot branch skipped",
			rec(d, "Merge pull request #30 from mhersson/dependabot/go_modules/golang.org/x/sys-0.48.0",
				"Bump golang.org/x/sys from 0.47.0 to 0.48.0"),
			Limit, none,
		},
		{
			"non-PR subject skipped",
			rec(d, "Merge branch 'main' into feat/x", "feat: x") +
				rec(d, "Merge pull request #5 from noslash", "feat: x") +
				rec(d, "Merge pull request #5 from o/b extra", "feat: x"),
			Limit, none,
		},
		{
			"first non-blank body line",
			rec(d, subj(7), "\n\n  Show the popup  \n\nLonger description\nmore"), Limit,
			[]protocol.Change{ch(7, change, "Show the popup", du)},
		},
		{"empty title skipped", rec(d, subj(7), "") + rec(d, subj(8), "\n  \n"), Limit, none},
		{
			"malformed date kept",
			rec("yesterday", subj(7), "feat: x"), Limit,
			[]protocol.Change{ch(7, feat, "x", "")},
		},
		{
			"offset date normalized to UTC",
			rec("2026-10-01T00:30:00+02:00", subj(7), "fix: y"), Limit,
			[]protocol.Change{ch(7, fix, "y", "2026-09-30T22:30:00Z")},
		},
		{
			"input order kept, stops at limit",
			rec(d, subj(223), "feat: a") + rec(d, subj(220), "fix: b") + rec(d, subj(221), "feat: c"), 2,
			[]protocol.Change{ch(223, feat, "a", du), ch(220, fix, "b", du)},
		},
		{
			"record with missing fields skipped",
			"garbage\x1e\n" + d + "\x1f" + subj(8) + "\x1e\n" + rec(d, subj(7), "feat: x"), Limit,
			[]protocol.Change{ch(7, feat, "x", du)},
		},
		{
			"PR number overflowing int skipped",
			rec(d, "Merge pull request #99999999999999999999 from o/b", "feat: x"), Limit, none,
		},
		{
			"huge limit means no cap",
			rec(d, subj(7), "feat: x"), math.MaxInt,
			[]protocol.Change{ch(7, feat, "x", du)},
		},
		{
			"surrounding whitespace ignored",
			"\n\n" + rec(d, subj(7), "feat: x") + "  \n", Limit,
			[]protocol.Change{ch(7, feat, "x", du)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Parse([]byte(tt.raw), tt.limit)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Parse:\n got %#v\nwant %#v", got, tt.want)
			}
		})
	}
}

// CM renders "changes":[]; a nil slice would marshal as null.
func TestParse_EmptyInputIsNonNil(t *testing.T) {
	valid := []byte(rec(d, subj(7), "feat: x"))

	tests := []struct {
		name  string
		raw   []byte
		limit int
	}{
		{"nil input", nil, Limit},
		{"empty input", []byte(""), Limit},
		{"only separators and whitespace", []byte(" \n\x1e\n"), Limit},
		{"zero limit", valid, 0},
		{"negative limit", valid, -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Parse(tt.raw, tt.limit)
			if got == nil || len(got) != 0 {
				t.Errorf("want a non-nil empty slice, got %#v", got)
			}
		})
	}
}

// Proves GitLogArgs and Parse agree on what git really prints: direct commits
// and merges inside a merged branch are left out, Dependabot is dropped.
func TestGitLogArgs_ParseRealHistory(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}

	dir := t.TempDir()

	run := func(env []string, args ...string) []byte {
		t.Helper()

		return git(t, dir, env, args...)
	}

	at := committedAt

	run(nil, "init", "-q", "-b", "main")
	run(nil, "commit", "-q", "--allow-empty", "-m", "init")

	run(nil, "switch", "-q", "-c", "feat/x")
	run(nil, "commit", "-q", "--allow-empty", "-m", "x")
	run(nil, "switch", "-q", "main")
	run(at("2026-10-01T12:00:00+02:00"), "merge", "-q", "--no-ff",
		"-m", "Merge pull request #1 from o/feat/x", "-m", "feat(x): add x", "feat/x")

	run(nil, "commit", "-q", "--allow-empty", "-m", "fix: direct commit")
	// Only --merges keeps this merge-shaped direct commit out.
	run(nil, "commit", "-q", "--allow-empty", "-m", "Merge pull request #4 from o/direct", "-m", "feat: direct")

	run(nil, "switch", "-q", "-c", "dependabot/go_modules/y")
	run(nil, "commit", "-q", "--allow-empty", "-m", "bump y")
	run(nil, "switch", "-q", "main")
	run(nil, "merge", "-q", "--no-ff",
		"-m", "Merge pull request #2 from o/dependabot/go_modules/y", "-m", "Bump y from 1 to 2",
		"dependabot/go_modules/y")

	run(nil, "switch", "-q", "-c", "side")
	run(nil, "commit", "-q", "--allow-empty", "-m", "side")
	run(nil, "switch", "-q", "-c", "inner")
	run(nil, "commit", "-q", "--allow-empty", "-m", "inner")
	run(nil, "switch", "-q", "side")
	run(nil, "merge", "-q", "--no-ff", "-m", "Merge pull request #9 from o/inner", "-m", "feat: inner", "inner")
	run(nil, "switch", "-q", "main")
	run(at("2026-10-02T09:15:00+02:00"), "merge", "-q", "--no-ff",
		"-m", "Merge pull request #3 from o/side", "-m", "Show the popup", "side")

	got := Parse(run(nil, GitLogArgs...), Limit)

	want := []protocol.Change{
		ch(3, protocol.ChangeKindChange, "Show the popup", "2026-10-02T07:15:00Z"),
		ch(1, protocol.ChangeKindFeat, "add x", "2026-10-01T10:00:00Z"),
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Parse(git %s):\n got %#v\nwant %#v", strings.Join(GitLogArgs, " "), got, want)
	}
}

// GitHub signs every merge commit, so a developer's log.showSignature=true would
// otherwise print verification output into the date field of every record.
func TestGitLogArgs_IgnoreShowSignature(t *testing.T) {
	for _, bin := range []string{"git", "ssh-keygen"} {
		if _, err := exec.LookPath(bin); err != nil {
			t.Skip(bin + " not on PATH")
		}
	}

	key := filepath.Join(t.TempDir(), "signing-key")

	keygen := exec.CommandContext(t.Context(), "ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-f", key)
	if out, err := keygen.CombinedOutput(); err != nil {
		t.Fatalf("ssh-keygen: %v\n%s", err, out)
	}

	dir := t.TempDir()

	git(t, dir, nil, "init", "-q", "-b", "main")
	git(t, dir, nil, "commit", "-q", "--allow-empty", "-m", "init")
	git(t, dir, nil, "switch", "-q", "-c", "feat/x")
	git(t, dir, nil, "commit", "-q", "--allow-empty", "-m", "x")
	git(t, dir, nil, "switch", "-q", "main")
	git(t, dir, committedAt("2026-10-01T12:00:00+02:00"),
		"-c", "gpg.format=ssh", "-c", "user.signingKey="+key,
		"merge", "-q", "--no-ff", "-S", "-m", "Merge pull request #1 from o/feat/x", "-m", "feat(x): add x", "feat/x")

	showSignature := []string{"GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=log.showSignature", "GIT_CONFIG_VALUE_0=true"}
	got := Parse(git(t, dir, showSignature, GitLogArgs...), Limit)

	want := []protocol.Change{ch(1, protocol.ChangeKindFeat, "add x", "2026-10-01T10:00:00Z")}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Parse with log.showSignature=true:\n got %#v\nwant %#v", got, want)
	}
}

// A git hook exports GIT_DIR; the helper must not let these tests commit into
// the developer's own repository.
func TestGitHelperIgnoresInheritedGitEnv(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}

	outer := t.TempDir()
	git(t, outer, nil, "init", "-q")
	t.Setenv("GIT_DIR", filepath.Join(outer, ".git"))

	inner := t.TempDir()
	git(t, inner, nil, "init", "-q")

	got := strings.TrimSpace(string(git(t, inner, nil, "rev-parse", "--absolute-git-dir")))
	if want := filepath.Join(inner, ".git"); got != want {
		t.Errorf("git ran against %s, want %s", got, want)
	}
}

// git runs git in dir with no global or system config, so the developer's own
// settings cannot change the output under test.
func git(t *testing.T, dir string, env []string, args ...string) []byte {
	t.Helper()

	cmd := exec.CommandContext(t.Context(), "git", args...) //nolint:gosec // G204: test drives git with fixed arguments
	cmd.Dir = dir

	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "GIT_") {
			cmd.Env = append(cmd.Env, kv)
		}
	}

	cmd.Env = append(cmd.Env,
		"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
	cmd.Env = append(cmd.Env, env...)

	var stderr bytes.Buffer

	cmd.Stderr = &stderr

	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, stderr.String())
	}

	return out
}
