package changelog

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
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
			"type is case-insensitive",
			rec(d, subj(7), "Fix: handle empty input"), Limit,
			[]protocol.Change{ch(7, fix, "handle empty input", du)},
		},
		{
			"other type is change",
			rec(d, subj(7), "perf(hub): faster fan-out"), Limit,
			[]protocol.Change{ch(7, change, "faster fan-out", du)},
		},
		{
			"noise types skipped",
			rec(d, subj(1), "ci: a") + rec(d, subj(2), "docs: a") + rec(d, subj(3), "chore(deps): a") +
				rec(d, subj(4), "build: a") + rec(d, subj(5), "test: a") + rec(d, subj(6), "refactor: a") +
				rec(d, subj(7), "style: a") + rec(d, subj(8), "feat: kept"),
			Limit,
			[]protocol.Change{ch(8, feat, "kept", du)},
		},
		{"noise type case-insensitive", rec(d, subj(7), "Chore: tidy"), Limit, none},
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

	at := func(date string) []string { return []string{"GIT_COMMITTER_DATE=" + date} }

	run(nil, "init", "-q", "-b", "main")
	run(nil, "commit", "-q", "--allow-empty", "-m", "init")

	run(nil, "switch", "-q", "-c", "feat/x")
	run(nil, "commit", "-q", "--allow-empty", "-m", "x")
	run(nil, "switch", "-q", "main")
	run(at("2026-10-01T12:00:00+02:00"), "merge", "-q", "--no-ff",
		"-m", "Merge pull request #1 from o/feat/x", "-m", "feat(x): add x", "feat/x")

	run(nil, "commit", "-q", "--allow-empty", "-m", "fix: direct commit")

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

// git runs git in dir with no global or system config, so the developer's own
// settings cannot change the output under test.
func git(t *testing.T, dir string, env []string, args ...string) []byte {
	t.Helper()

	cmd := exec.CommandContext(t.Context(), "git", args...) //nolint:gosec // G204: test drives git with fixed arguments
	cmd.Dir = dir

	cmd.Env = append(os.Environ(),
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
