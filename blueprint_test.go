package protocol

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"
)

// The blueprint types reach the browser through CM unchanged, so these
// bytes are what the web UI parses.

func pinRoundTrip[T any](t *testing.T, in T, want string) {
	t.Helper()

	b, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}

	if string(b) != want {
		t.Errorf("wire drift:\n got %s\nwant %s", b, want)
	}

	var out T
	if err := json.Unmarshal([]byte(want), &out); err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(in, out) {
		t.Errorf("round-trip mismatch:\n got %+v\nwant %+v", out, in)
	}
}

func TestBlueprintViewWireShape(t *testing.T) {
	started := time.Date(2026, 10, 17, 9, 30, 0, 0, time.UTC)
	decided := time.Date(2026, 10, 17, 9, 45, 0, 0, time.UTC)

	v := BlueprintView{
		ID:    "s1",
		Mode:  ArchitectModeDefault,
		State: "running",
		Seq:   7,
		Pipeline: []PipelineStep{
			{Stage: "slicing", Persona: "Product owner", FoldsInto: "solution_design", Status: "folded"},
			{
				Stage: "solution_design", Persona: "System designer", Gate: true, Policy: "human",
				Status: "revising", Attempt: 1, Round: 2, MaxRounds: 3, LatestVersion: 2, CostUSD: 0.35,
			},
		},
		Current: &CurrentRun{
			Stage: "solution_design", Kind: "stage", Persona: "System designer", Model: "m",
			StartedAt: started, Turns: 6, ToolCalls: 4, Tokens: 11200,
		},
		Open: &OpenPrompt{PromptID: "gate:solution_design:v1", Kind: "gate"},
		Cost: BlueprintCost{SpentUSD: 0.35, RunSpentUSD: 0.12, CapUSD: 2, RunCapUSD: 0.5, Priced: true},
		Artifacts: []ArtifactRef{{
			Stage: "solution_design", Version: 1, CreatedAt: started, Status: "superseded",
			DecidedAt: &decided, DecidedBy: "you",
			Review: &ArtifactReview{Reviewer: "Senior developer", Score: 3, Threshold: 4, Round: 1, Findings: []ReviewFinding{
				{ID: "R1", Severity: "critical", Text: "No rollback plan", Status: "open", SentTo: 2, AddressedIn: 2, AddressedBy: "System designer", AddressedNote: "Added D4"},
				{ID: "R2", Severity: "minor", Text: "Naming", Status: "open", SentTo: 2},
			}},
		}},
	}

	want := `{"id":"s1","mode":"default","state":"running","seq":7,"pipeline":[` +
		`{"stage":"slicing","persona":"Product owner","gate":false,"folds_into":"solution_design","status":"folded","cost_usd":0},` +
		`{"stage":"solution_design","persona":"System designer","gate":true,"policy":"human","status":"revising",` +
		`"attempt":1,"round":2,"max_rounds":3,"latest_version":2,"cost_usd":0.35}],` +
		`"current":{"stage":"solution_design","kind":"stage","persona":"System designer","model":"m",` +
		`"started_at":"2026-10-17T09:30:00Z","turns":6,"tool_calls":4,"tokens":11200},` +
		`"open":{"prompt_id":"gate:solution_design:v1","kind":"gate"},` +
		`"cost":{"spent_usd":0.35,"run_spent_usd":0.12,"cap_usd":2,"run_cap_usd":0.5,"priced":true},` +
		`"artifacts":[{"stage":"solution_design","version":1,"created_at":"2026-10-17T09:30:00Z","status":"superseded",` +
		`"decided_at":"2026-10-17T09:45:00Z","decided_by":"you",` +
		`"review":{"reviewer":"Senior developer","score":3,"threshold":4,"passed":false,"round":1,"findings":[` +
		`{"id":"R1","severity":"critical","text":"No rollback plan","status":"open","sent_to":2,"addressed_in":2,"addressed_by":"System designer","addressed_note":"Added D4"},` +
		`{"id":"R2","severity":"minor","text":"Naming","status":"open","sent_to":2}]}}],"slices":null}`

	pinRoundTrip(t, v, want)
}

// No cap, unpriced and a free step each mean something to the pane, so none
// may vanish from the wire. A nil list goes out as null; the architect sends
// [] instead, but consumers accept both.
func TestBlueprintZeroValuesStayOnTheWire(t *testing.T) {
	pinRoundTrip(t, BlueprintView{ID: "s1", Mode: ArchitectModeAuto, State: "new"},
		`{"id":"s1","mode":"auto","state":"new","seq":0,"pipeline":null,`+
			`"cost":{"spent_usd":0,"run_spent_usd":0,"cap_usd":0,"run_cap_usd":0,"priced":false},"artifacts":null,"slices":null}`)

	pinRoundTrip(t, PipelineStep{Stage: "intake", Status: "pending"},
		`{"stage":"intake","persona":"","gate":false,"status":"pending","cost_usd":0}`)
}

func TestBlueprintArtifactWireShape(t *testing.T) {
	tests := []struct {
		name string
		in   BlueprintArtifact
		want string
	}{
		{
			name: "stage artifact with check problems",
			in: BlueprintArtifact{
				Stage: "solution_design", Version: 2, Body: "# Design\n\nD1",
				Data:          json.RawMessage(`{"slices":[{"id":"S1"}]}`),
				CheckProblems: []string{"decision D2 has no basis"},
			},
			want: `{"stage":"solution_design","version":2,"body":"# Design\n\nD1",` +
				`"data":{"slices":[{"id":"S1"}]},"check_problems":["decision D2 has no basis"]}`,
		},
		{
			name: "emission pseudo-artifact keeps an empty body",
			in:   BlueprintArtifact{Stage: "emission", Version: 1, Data: json.RawMessage(`{"cards":[]}`)},
			want: `{"stage":"emission","version":1,"body":"","data":{"cards":[]}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) { pinRoundTrip(t, tt.in, tt.want) })
	}
}

func TestLiveFrameWireShape(t *testing.T) {
	tests := []struct {
		name string
		in   LiveFrame
		want string
	}{
		{
			name: "snapshot with the full draft",
			in: LiveFrame{
				Type:  "snapshot",
				View:  &BlueprintView{ID: "s1", Mode: ArchitectModeAuto, State: "running", Seq: 3, Pipeline: []PipelineStep{}, Artifacts: []ArtifactRef{}, Slices: []BlueprintSlice{}},
				Draft: &Draft{Stage: "solution_design", Version: 2, Text: "# Design"},
			},
			want: `{"type":"snapshot","view":{"id":"s1","mode":"auto","state":"running","seq":3,"pipeline":[],` +
				`"cost":{"spent_usd":0,"run_spent_usd":0,"cap_usd":0,"run_cap_usd":0,"priced":false},"artifacts":[],"slices":[]},` +
				`"draft":{"stage":"solution_design","version":2,"offset":0,"text":"# Design"}}`,
		},
		{
			name: "artifact stored",
			in:   LiveFrame{Type: "artifact_stored", Stage: "solution_design", Version: 2},
			want: `{"type":"artifact_stored","stage":"solution_design","version":2}`,
		},
		{
			name: "draft fragment",
			in:   LiveFrame{Type: "draft", Draft: &Draft{Stage: "solution_design", Version: 2, Offset: 8, Text: "\n\nD1"}},
			want: `{"type":"draft","draft":{"stage":"solution_design","version":2,"offset":8,"text":"\n\nD1"}}`,
		},
		{
			name: "refused draft",
			in: LiveFrame{Type: "draft", Draft: &Draft{
				Stage: "solution_design", Version: 2, Offset: 12, Refused: true, Reason: "decision D1 has no basis",
			}},
			want: `{"type":"draft","draft":{"stage":"solution_design","version":2,"offset":12,"text":"",` +
				`"refused":true,"reason":"decision D1 has no basis"}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) { pinRoundTrip(t, tt.in, tt.want) })
	}
}

// A later round's review keeps each re-checked finding's id with its
// verdict and the version it was verified in; passed stays on the wire when
// false, and round is left out by a review that carries none.
func TestArtifactReviewWireShape(t *testing.T) {
	pinRoundTrip(t, ArtifactReview{Reviewer: "tester", Score: 4, Threshold: 4, Passed: true, Round: 2, Findings: []ReviewFinding{
		{ID: "R1", Severity: "critical", Text: "AC2 cannot fail", Status: "fixed", VerifiedIn: 2},
		{ID: "R2", Severity: "major", Text: "No criterion covers an empty input", Status: "still_open", VerifiedIn: 2, SentTo: 3},
		{ID: "R3", Severity: "minor", Text: "Name the fixture", Status: "open", SentTo: 3},
	}}, `{"reviewer":"tester","score":4,"threshold":4,"passed":true,"round":2,"findings":[`+
		`{"id":"R1","severity":"critical","text":"AC2 cannot fail","status":"fixed","verified_in":2},`+
		`{"id":"R2","severity":"major","text":"No criterion covers an empty input","status":"still_open","verified_in":2,"sent_to":3},`+
		`{"id":"R3","severity":"minor","text":"Name the fixture","status":"open","sent_to":3}]}`)

	pinRoundTrip(t, ArtifactReview{Reviewer: "tester", Score: 2, Threshold: 4, Findings: []ReviewFinding{}},
		`{"reviewer":"tester","score":2,"threshold":4,"passed":false,"findings":[]}`)
}

// A per-slice step, run, artifact, frame and draft carry their slice; a
// whole-blueprint one leaves it off the wire. The view lists the slices in
// slicing order, and a per-slice gate's id carries its slice.
func TestBlueprintSlicesWireShape(t *testing.T) {
	created := time.Date(2026, 10, 18, 9, 0, 0, 0, time.UTC)

	pinRoundTrip(t, BlueprintView{
		ID: "s1", Mode: ArchitectModeDefault, State: "waiting", Seq: 9,
		Pipeline: []PipelineStep{
			{Stage: "shared_decisions", Persona: "system_designer", Gate: true, Policy: "human", Status: "done", LatestVersion: 1},
			{Stage: "solution_design", Slice: "SL2", Persona: "system_designer", Gate: true, Policy: "human", Status: "waiting", Attempt: 1, LatestVersion: 1},
		},
		Current:   &CurrentRun{Stage: "grounding", Slice: "SL1", Kind: "stage", Persona: "senior_developer", Model: "m", StartedAt: created},
		Open:      &OpenPrompt{PromptID: "gate:solution_design:SL2:v1", Kind: "gate"},
		Artifacts: []ArtifactRef{{Stage: "solution_design", Slice: "SL2", Version: 1, CreatedAt: created, Status: "at_gate"}},
		Slices:    []BlueprintSlice{{ID: "SL1", Title: "Export one report"}, {ID: "SL2", Title: "Schedule the exports"}},
	}, `{"id":"s1","mode":"default","state":"waiting","seq":9,"pipeline":[`+
		`{"stage":"shared_decisions","persona":"system_designer","gate":true,"policy":"human","status":"done","latest_version":1,"cost_usd":0},`+
		`{"stage":"solution_design","slice":"SL2","persona":"system_designer","gate":true,"policy":"human","status":"waiting","attempt":1,"latest_version":1,"cost_usd":0}],`+
		`"current":{"stage":"grounding","slice":"SL1","kind":"stage","persona":"senior_developer","model":"m",`+
		`"started_at":"2026-10-18T09:00:00Z","turns":0,"tool_calls":0,"tokens":0},`+
		`"open":{"prompt_id":"gate:solution_design:SL2:v1","kind":"gate"},`+
		`"cost":{"spent_usd":0,"run_spent_usd":0,"cap_usd":0,"run_cap_usd":0,"priced":false},`+
		`"artifacts":[{"stage":"solution_design","slice":"SL2","version":1,"created_at":"2026-10-18T09:00:00Z","status":"at_gate"}],`+
		`"slices":[{"id":"SL1","title":"Export one report"},{"id":"SL2","title":"Schedule the exports"}]}`)

	pinRoundTrip(t, BlueprintArtifact{Stage: "acceptance", Slice: "SL1", Version: 2, Body: "# Acceptance"},
		`{"stage":"acceptance","slice":"SL1","version":2,"body":"# Acceptance"}`)

	pinRoundTrip(t, LiveFrame{Type: "artifact_stored", Stage: "decomposition", Slice: "SL2", Version: 1},
		`{"type":"artifact_stored","stage":"decomposition","slice":"SL2","version":1}`)

	pinRoundTrip(t, LiveFrame{Type: "draft", Draft: &Draft{Stage: "solution_design", Slice: "SL2", Version: 1, Text: "# Design"}},
		`{"type":"draft","draft":{"stage":"solution_design","slice":"SL2","version":1,"offset":0,"text":"# Design"}}`)
}
