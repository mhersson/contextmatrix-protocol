package changelog

import (
	"bytes"
	"regexp"
	"strconv"
	"strings"
	"time"

	protocol "github.com/mhersson/contextmatrix-protocol"
)

// Limit is how many changes a build reports after filtering.
const Limit = 20

// GitLogArgs is the git invocation every build runs, verbatim, to record
// its history. Output goes to the repo's history file. Makefiles write the
// literal command, so their argument list must equal this slice.
var GitLogArgs = []string{
	"log", "--no-show-signature", "--first-parent", "--merges",
	"-n", "100", "--format=%cI%x1f%s%x1f%b%x1e",
}

var (
	mergeSubject = regexp.MustCompile(`^Merge pull request #(\d+) from ([^/\s]+)/(\S+)$`)
	typedTitle   = regexp.MustCompile(`^(\w+)(\([^)]*\))?!?:\s*(.+)$`)
)

// conventionalTypes maps each lowercase conventional type to its kind; "" marks
// a type that never reaches users. Any other "word:" prefix, capitalized ones
// included, is part of a card title.
var conventionalTypes = map[string]string{
	"feat": protocol.ChangeKindFeat, "fix": protocol.ChangeKindFix,
	"perf": protocol.ChangeKindChange, "revert": protocol.ChangeKindChange,
	"ci": "", "docs": "", "chore": "", "build": "", "test": "", "refactor": "", "style": "",
}

// Parse turns GitLogArgs output into at most limit changes, newest first.
func Parse(raw []byte, limit int) []protocol.Change {
	// The capacity hint is capped: a caller may pass a huge limit to mean "all".
	out := make([]protocol.Change, 0, min(max(limit, 0), Limit))
	if limit <= 0 {
		return out
	}

	for record := range bytes.SplitSeq(raw, []byte{0x1e}) {
		c, ok := parseRecord(strings.TrimSpace(string(record)))
		if !ok {
			continue
		}

		out = append(out, c)
		if len(out) == limit {
			break
		}
	}

	return out
}

func parseRecord(record string) (protocol.Change, bool) {
	fields := strings.SplitN(record, "\x1f", 3)
	if len(fields) < 3 {
		return protocol.Change{}, false
	}

	m := mergeSubject.FindStringSubmatch(strings.TrimSpace(fields[1]))
	if m == nil || strings.HasPrefix(m[3], "dependabot/") {
		return protocol.Change{}, false
	}

	number, err := strconv.Atoi(m[1])
	if err != nil {
		return protocol.Change{}, false
	}

	title := firstLine(fields[2])
	if title == "" {
		return protocol.Change{}, false
	}

	kind := protocol.ChangeKindChange

	if t := typedTitle.FindStringSubmatch(title); t != nil {
		if k, ok := conventionalTypes[t[1]]; ok {
			if k == "" {
				return protocol.Change{}, false
			}

			kind, title = k, t[3]
		}
	}

	return protocol.Change{
		Number:   number,
		Kind:     kind,
		Title:    title,
		MergedAt: mergedAt(strings.TrimSpace(fields[0])),
	}, true
}

func firstLine(body string) string {
	for line := range strings.SplitSeq(body, "\n") {
		if s := strings.TrimSpace(line); s != "" {
			return s
		}
	}

	return ""
}

// mergedAt writes git's committer date in UTC so string order is merge order;
// a date that does not parse yields "" and the change is still kept.
func mergedAt(date string) string {
	t, err := time.Parse(time.RFC3339, date)
	if err != nil {
		return ""
	}

	return t.UTC().Format(time.RFC3339)
}
