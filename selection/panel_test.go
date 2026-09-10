package selection

import (
	"fmt"
	"maps"
	"testing"
)

// withCreator tags a fixture model with the CM-supplied creator, the vendor
// the diversity preference reads before falling back to the slug prefix.
func withCreator(mm model, creator string) model {
	mm.creator = creator

	return mm
}

func (s *Selector) reviewPanel(in SelectInput, n int) []Pick {
	return SeatPicks(s.SelectReviewPanelReport(in, n))
}

func (s *Selector) candidateModels(in SelectInput, n int, pin string) []Pick {
	return SeatPicks(s.SelectCandidateModelsReport(in, n, pin))
}

func TestSelectReviewPanel(t *testing.T) {
	// Four qualifying reviewers; one will be the coder's pick (excluded).
	// Blended $/Mtok: alpha 2.1, beta 2.7, gamma 3.0, delta 18. Bars come from
	// DefaultTierBars (moderate 0.76); headroom 1.5.
	s := sel("capable-default",
		m("alpha", 0.7, 1.4, 200000, 0, 0.80),
		m("beta", 0.9, 1.8, 200000, 0, 0.85),
		m("gamma", 1.0, 2.0, 200000, 0, 0.82),
		m("delta", 6.0, 12.0, 200000, 0, 0.95),
	)

	// moderate (bar 0.76): all four clear the bar. Exclude alpha (coder's pick).
	// Remaining candidates: beta(q0.85,$2.7), gamma(q0.82,$3.0), delta(q0.95,$18).
	// Pick 1: cheapest $2.7 -> band 4.05: beta, gamma in; delta out. Top: beta.
	// Pick 2 (exclude beta): gamma(q0.82,$3.0), delta(q0.95,$18). Cheapest $3.0 ->
	//   band 4.5: gamma only. -> gamma.
	// Pick 3 (exclude beta,gamma): delta only ($18). -> delta.
	in := SelectInput{Role: RoleReviewer, Tier: TierModerate, EstTokens: 50000, Exclude: map[string]bool{"alpha": true}}
	panel := s.reviewPanel(in, 3)
	eq(t, 3, len(panel))
	eq(t, "beta", panel[0].Model)
	eq(t, "gamma", panel[1].Model)
	eq(t, "delta", panel[2].Model)

	// Only two qualifying models -> reuse the last pick to fill 3 slots rather
	// than escalating price. Restrict the pool via priors: gamma/delta sit below
	// the moderate bar (0.76) so they are never candidates.
	s2 := sel("capable-default",
		m("alpha", 0.7, 1.4, 200000, 0, 0.80),
		m("beta", 0.9, 1.8, 200000, 0, 0.85),
		m("gamma", 1.0, 2.0, 200000, 0, 0.50),
		m("delta", 6.0, 12.0, 200000, 0, 0.50),
	)
	in2 := SelectInput{Role: RoleReviewer, Tier: TierModerate, EstTokens: 50000}
	// Candidates: alpha(q0.80,$2.1), beta(q0.85,$2.7). Pick1 cheapest $2.1 ->
	// band 3.15: both in; top quality beta(0.85). Pick1=beta.
	// Pick2 (exclude beta): alpha only. Pick2=alpha.
	// Pick3 (exclude beta,alpha): pool dry -> reuse last pick alpha.
	panel2 := s2.reviewPanel(in2, 3)
	eq(t, 3, len(panel2))
	eq(t, "beta", panel2[0].Model)
	eq(t, "alpha", panel2[1].Model)
	eq(t, "alpha", panel2[2].Model, "reuse, no price escalation")
}

func TestSelectReviewPanelDryFromStart(t *testing.T) {
	// Zero qualifying candidates (no model carries a prior for the role): the
	// panel must still be n non-empty specs - all the capable default, never
	// ModelSpec{}.
	s := sel("capable-default",
		m("alpha", 0.7, 1.4, 200000, 0, 0),
		m("beta", 0.9, 1.8, 200000, 0, 0),
	)

	panel := s.reviewPanel(SelectInput{Role: RoleReviewer, Tier: TierModerate, EstTokens: 50000}, 3)
	eq(t, 3, len(panel))

	for i, spec := range panel {
		eq(t, "capable-default", spec.Model, fmt.Sprintf("slot %d", i))
	}
}

func TestSelectCandidateModelsNoPinWrapsAround(t *testing.T) {
	// Three equally-priced, equally-qualified models: the exclude set built up
	// across rounds forces distinct picks in catalog order while the pool
	// lasts (m1, m2, m3), then the pool runs dry and the 4th slot reuses the
	// last real pick (candidateModels wrap semantics) rather than shrinking
	// n or escalating price.
	s := sel("capable-default",
		m("m1", 1.0, 2.0, 200000, 0.80, 0),
		m("m2", 1.0, 2.0, 200000, 0.80, 0),
		m("m3", 1.0, 2.0, 200000, 0.80, 0),
	)
	in := SelectInput{Role: RoleCoder, Tier: TierSimple, EstTokens: 50000}

	specs := s.candidateModels(in, 4, "")
	eq(t, 4, len(specs))
	eq(t, "m1", specs[0].Model)
	eq(t, "m2", specs[1].Model)
	eq(t, "m3", specs[2].Model)
	// Duplication and provenance are two independent facts. The wrapped seat is
	// the seat it wrapped in every measured field - model, window, source,
	// prior, requested and met tier - and carries the flag on top, so a caller
	// can count the panel's real models without losing what any seat is worth.
	want := specs[2]
	want.Duplicate = true

	falsy(t, specs[2].Duplicate, "the last real pick is not a repeat")
	eq(t, want, specs[3], "a repeated seat is the seat it wrapped, flagged")
}

func TestSelectCandidateModelsSingleModelPoolRepeatsThroughout(t *testing.T) {
	s := sel("capable-default", m("m1", 1.0, 2.0, 200000, 0.80, 0))
	in := SelectInput{Role: RoleCoder, Tier: TierSimple, EstTokens: 50000}

	specs := s.candidateModels(in, 3, "")
	eq(t, 3, len(specs))

	for i, spec := range specs {
		eq(t, "m1", spec.Model, fmt.Sprintf("slot %d", i))
		eq(t, 200000, spec.ContextWindow, fmt.Sprintf("slot %d", i))
	}
}

func TestSelectCandidateModelsPinOccupiesSlotOneExcludedFromRest(t *testing.T) {
	// pinned/x sits first in catalog order with the same price/quality profile
	// as m1..m3: if the pin were not excluded from the auto-pick rounds, tie
	// break would select it again for slot 2. Catching that requires pinned/x
	// to be genuinely attractive, not just present.
	s := sel("capable-default",
		m("pinned/x", 1.0, 2.0, 99000, 0.80, 0),
		m("m1", 1.0, 2.0, 200000, 0.80, 0),
		m("m2", 1.0, 2.0, 200000, 0.80, 0),
		m("m3", 1.0, 2.0, 200000, 0.80, 0),
	)
	in := SelectInput{Role: RoleCoder, Tier: TierSimple, EstTokens: 50000}

	specs := s.candidateModels(in, 3, "pinned/x")
	eq(t, 3, len(specs))
	eq(t, "pinned/x", specs[0].Model)
	eq(t, 99000, specs[0].ContextWindow, "pin must carry its own catalog context window")
	eq(t, "m1", specs[1].Model)
	eq(t, "m2", specs[2].Model)
}

func TestSelectCandidateModelsPinMergesExcludeWithoutMutatingCaller(t *testing.T) {
	s := sel("capable-default",
		m("pinned/x", 1.0, 2.0, 200000, 0.80, 0),
		m("m1", 1.0, 2.0, 200000, 0.80, 0),
		m("m2", 1.0, 2.0, 200000, 0.80, 0),
		m("m3", 1.0, 2.0, 200000, 0.80, 0),
		m("already-excluded", 1.0, 2.0, 200000, 0.80, 0),
	)

	origExclude := map[string]bool{"already-excluded": true}
	in := SelectInput{Role: RoleCoder, Tier: TierSimple, EstTokens: 50000, Exclude: origExclude}

	specs := s.candidateModels(in, 3, "pinned/x")
	eq(t, 3, len(specs))
	eq(t, "pinned/x", specs[0].Model)
	eq(t, "m1", specs[1].Model)
	eq(t, "m2", specs[2].Model)

	for _, spec := range specs {
		neq(t, "already-excluded", spec.Model, "pre-existing Exclude entries must carry through to the auto picks")
	}

	truthy(t, maps.Equal(map[string]bool{"already-excluded": true}, origExclude),
		"candidateModels must not mutate the caller's Exclude map")
}

func TestSelectCandidateModelsZeroOrNegativeNReturnsNil(t *testing.T) {
	s := sel("capable-default",
		m("big/window", 0.5, 1.5, 131072, 0, 0),
		m("small/window", 0.1, 0.1, 8192, 0, 0),
	)
	in := SelectInput{Role: RoleCoder, Tier: TierSimple}

	truthy(t, s.candidateModels(in, 0, "") == nil, "n = 0")
	truthy(t, s.candidateModels(in, -1, "") == nil, "n = -1")
	truthy(t, s.candidateModels(in, 0, "pinned/x") == nil, "n = 0 with a pin")
}

// TestSelectDiscussionPanel pins the mob session seat-selection seam: it must give
// distinct models first, honor the caller's exclusions (review discussions
// exclude the models that coded the card), and wrap around on scarcity
// instead of shrinking the panel - the SelectReviewPanelReport walk, by name.
func TestSelectDiscussionPanel(t *testing.T) {
	// Four qualifying reviewers at the complex bar (0.82) with distinct prices.
	full := []model{
		m("disc/alpha", 0.7, 1.4, 200000, 0, 0.95),
		m("disc/beta", 0.9, 1.8, 200000, 0, 0.92),
		m("disc/gamma", 1.0, 2.0, 200000, 0, 0.90),
		m("disc/delta", 6.0, 12.0, 200000, 0, 0.88),
	}
	// Scarce pool: only alpha and beta clear the complex bar.
	scarce := []model{
		m("disc/alpha", 0.7, 1.4, 200000, 0, 0.95),
		m("disc/beta", 0.9, 1.8, 200000, 0, 0.92),
		m("disc/gamma", 1.0, 2.0, 200000, 0, 0.50),
		m("disc/delta", 6.0, 12.0, 200000, 0, 0.50),
	}

	tests := []struct {
		name    string
		models  []model
		in      SelectInput
		n       int
		want    []string
		wantLen int
	}{
		{
			// Blended $/Mtok: alpha 2.1, beta 2.7, gamma 3.0, delta 18. Pick 1:
			// cheapest 2.1 -> band 3.15: alpha, beta, gamma in; top quality
			// alpha. Pick 2 (alpha excluded): band from 2.7 -> 4.05: beta,
			// gamma; top beta. Pick 3: gamma.
			name:   "distinct models across seats",
			models: full,
			in:     SelectInput{Role: RoleReviewer, Tier: TierComplex, EstTokens: 50000},
			n:      3,
			want:   []string{"disc/alpha", "disc/beta", "disc/gamma"},
		},
		{
			// Excluding the coder's model removes it from every seat.
			name:   "caller exclusions respected",
			models: full,
			in: SelectInput{
				Role: RoleReviewer, Tier: TierComplex, EstTokens: 50000,
				Exclude: map[string]bool{"disc/alpha": true},
			},
			n:    3,
			want: []string{"disc/beta", "disc/gamma", "disc/delta"},
		},
		{
			// Two qualifying models, three seats: wrap around on the last real
			// pick rather than escalating price or shrinking the panel.
			name:   "wrap-around on scarcity",
			models: scarce,
			in:     SelectInput{Role: RoleReviewer, Tier: TierComplex, EstTokens: 50000},
			n:      3,
			want:   []string{"disc/alpha", "disc/beta", "disc/beta"},
		},
		{
			// n far beyond the pool: still n non-empty specs.
			name:    "n greater than available",
			models:  scarce,
			in:      SelectInput{Role: RoleReviewer, Tier: TierComplex, EstTokens: 50000},
			n:       5,
			wantLen: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := sel("capable-default", tt.models...)

			panel := SeatPicks(s.SelectDiscussionPanelReport(tt.in, tt.n))

			if tt.wantLen > 0 {
				eq(t, tt.wantLen, len(panel))

				for i, spec := range panel {
					neq(t, "", spec.Model, fmt.Sprintf("slot %d must not be empty", i))
				}

				return
			}

			got := make([]string, len(panel))
			for i, spec := range panel {
				got[i] = spec.Model
			}

			eqSlice(t, tt.want, got)

			if tt.in.Exclude != nil {
				for i, spec := range panel {
					falsy(t, tt.in.Exclude[spec.Model], fmt.Sprintf("slot %d picked an excluded model %q", i, spec.Model))
				}
			}
		})
	}
}

// TestSelectReviewPanelSpansVendors reproduces the reported incident: an
// OpenAI-compatible gateway (bare slugs, creators supplied by CM) whose top
// reviewer priors all belong to one vendor. Without vendor awareness the
// panel came out gpt/gpt/gpt even though a qualifying model from another
// vendor was available.
func TestSelectReviewPanelSpansVendors(t *testing.T) {
	// Blended $/Mtok: gpt-a 2.1, gpt-b 2.7, gpt-c 3.0, claude-x 3.6. All
	// clear the complex bar (0.82).
	s := sel("capable-default",
		withCreator(m("gpt-a", 0.7, 1.4, 200000, 0, 0.95), "openai"),
		withCreator(m("gpt-b", 0.9, 1.8, 200000, 0, 0.90), "openai"),
		withCreator(m("gpt-c", 1.0, 2.0, 200000, 0, 0.88), "openai"),
		withCreator(m("claude-x", 1.2, 2.4, 200000, 0, 0.85), "anthropic"),
	)

	in := SelectInput{Role: RoleReviewer, Tier: TierComplex, EstTokens: 50000}
	panel := s.reviewPanel(in, 3)
	eq(t, 3, len(panel))

	// Seat 1 is unchanged from the vendor-blind walk: band 2.1*1.5=3.15 holds
	// gpt-a/b/c; top quality gpt-a. Seat 2 prefers the unseated vendor:
	// claude-x (band re-anchors on the filtered subset). Seat 3 has no unseated
	// vendor left and falls back to the vendor-blind pick: gpt-b.
	eq(t, "gpt-a", panel[0].Model)
	eq(t, "claude-x", panel[1].Model)
	eq(t, "gpt-b", panel[2].Model)
}

// TestSelectReviewPanelFavoriteBypassesVendorFilter pins config precedence:
// an operator favorite wins its seat even when its vendor is already on the
// panel - the vendor preference is an emergent heuristic and must never
// override explicit favorites. The seated favorite's vendor still counts as
// used for later seats.
func TestSelectReviewPanelFavoriteBypassesVendorFilter(t *testing.T) {
	favs := map[favKey][]string{{Tier: TierComplex}: {"v1-fav", "v1-fav2"}}
	s := newSelector(
		[]model{
			withCreator(m("v1-fav", 1.0, 2.0, 200000, 0, 0.90), "openai"),
			withCreator(m("v1-fav2", 1.0, 2.0, 200000, 0, 0.85), "openai"),
			withCreator(m("v2-b", 1.0, 2.0, 200000, 0, 0.95), "anthropic"),
		},
		nil, favs, "capable-default",
	)

	in := SelectInput{Role: RoleReviewer, Tier: TierComplex, EstTokens: 50000}
	panel := s.reviewPanel(in, 3)
	eq(t, 3, len(panel))

	// Seat 1: first favorite. Seat 2: the second favorite must win despite
	// openai already being seated (bypass), beating the unseated-vendor v2-b.
	// Seat 3: both favorites consumed; v2-b remains.
	eq(t, "v1-fav", panel[0].Model)
	eq(t, "v1-fav2", panel[1].Model)
	eq(t, "v2-b", panel[2].Model)
}

// TestSelectCandidateModelsPinSeedsVendor pins that a Best-of-N pin counts as
// a seated vendor: the auto-filled slots steer toward other vendors first.
func TestSelectCandidateModelsPinSeedsVendor(t *testing.T) {
	s := sel("capable-default",
		withCreator(m("gpt-pin", 1.0, 2.0, 200000, 0.90, 0), "openai"),
		withCreator(m("gpt-d", 1.0, 2.0, 200000, 0.95, 0), "openai"),
		withCreator(m("claude-y", 1.0, 2.0, 200000, 0.85, 0), "anthropic"),
	)

	in := SelectInput{Role: RoleCoder, Tier: TierComplex, EstTokens: 50000}
	specs := s.candidateModels(in, 3, "gpt-pin")
	eq(t, 3, len(specs))
	eq(t, "gpt-pin", specs[0].Model)
	// Slot 2 prefers the unseated vendor over the higher-prior gpt-d.
	eq(t, "claude-y", specs[1].Model)
	eq(t, "gpt-d", specs[2].Model)

	// A pin with no resolvable vendor (absent from the catalog, bare slug)
	// seeds nothing and must not panic.
	specs = s.candidateModels(in, 2, "mystery")
	eq(t, 2, len(specs))
	eq(t, "mystery", specs[0].Model)
	eq(t, "gpt-d", specs[1].Model, "no vendor seed: slot 2 stays the vendor-blind pick")
}

// TestSelectReviewPanelVendorEdgeCases pins the soft-preference semantics:
// the vendor filter never downgrades quality below the tier bar, never
// touches models without a resolvable vendor, re-anchors the price band on
// the filtered subset, degrades to the vendor-blind walk on single-vendor
// pools, and keeps wrap-around scarcity semantics.
func TestSelectReviewPanelVendorEdgeCases(t *testing.T) {
	in := SelectInput{Role: RoleReviewer, Tier: TierComplex, EstTokens: 50000}

	t.Run("soft fallback when the only unseated vendor misses the bar", func(t *testing.T) {
		s := sel("capable-default",
			withCreator(m("gpt-a", 1.0, 2.0, 200000, 0, 0.95), "openai"),
			withCreator(m("gpt-b", 1.0, 2.0, 200000, 0, 0.90), "openai"),
			// claude-weak sits below the complex bar 0.82.
			withCreator(m("claude-weak", 1.0, 2.0, 200000, 0, 0.80), "anthropic"),
		)

		panel := s.reviewPanel(in, 2)
		eq(t, 2, len(panel))
		eq(t, "gpt-a", panel[0].Model)
		eq(t, "gpt-b", panel[1].Model, "diversity must never seat a below-bar model")
	})

	t.Run("diverse seat may cost above the vendor-blind band", func(t *testing.T) {
		s := sel("capable-default",
			withCreator(m("gpt-a", 0.7, 1.4, 200000, 0, 0.95), "openai"), // $2.1
			withCreator(m("gpt-b", 0.9, 1.8, 200000, 0, 0.90), "openai"), // $2.7
			// claude-exp is $30, far outside 2.7*1.5.
			withCreator(m("claude-exp", 10.0, 20.0, 200000, 0, 0.93), "anthropic"),
		)

		panel := s.reviewPanel(in, 2)
		eq(t, 2, len(panel))
		eq(t, "gpt-a", panel[0].Model)
		eq(t, "claude-exp", panel[1].Model,
			"the band re-anchors on the vendor-filtered subset (documented cost of diversity)")
	})

	t.Run("models without a resolvable vendor pass every vendor filter", func(t *testing.T) {
		s := sel("capable-default",
			withCreator(m("gpt-a", 1.0, 2.0, 200000, 0, 0.95), "openai"),
			withCreator(m("gpt-b", 1.0, 2.0, 200000, 0, 0.93), "openai"),
			m("bare-n", 1.0, 2.0, 200000, 0, 0.85), // no creator, no slug prefix
		)

		panel := s.reviewPanel(in, 3)
		eq(t, 3, len(panel))
		eq(t, "gpt-a", panel[0].Model)
		eq(t, "bare-n", panel[1].Model,
			"vendor-less models stay selectable during a vendor-filtered attempt")
		eq(t, "gpt-b", panel[2].Model)
	})

	t.Run("single-vendor pool matches the vendor-blind walk", func(t *testing.T) {
		bare := []model{
			m("gpt-a", 0.7, 1.4, 200000, 0, 0.95),
			m("gpt-b", 0.9, 1.8, 200000, 0, 0.90),
			m("gpt-c", 1.0, 2.0, 200000, 0, 0.88),
		}

		tagged := make([]model, len(bare))
		for i, mm := range bare {
			tagged[i] = withCreator(mm, "openai")
		}

		blind := sel("capable-default", bare...).reviewPanel(in, 3)
		aware := sel("capable-default", tagged...).reviewPanel(in, 3)
		eqSlice(t, blind, aware)
	})

	t.Run("wrap-around scarcity keeps reusing the last pick", func(t *testing.T) {
		s := sel("capable-default",
			withCreator(m("gpt-a", 1.0, 2.0, 200000, 0, 0.95), "openai"),
			withCreator(m("claude-x", 1.0, 2.0, 200000, 0, 0.90), "anthropic"),
		)

		panel := s.reviewPanel(in, 4)
		eq(t, 4, len(panel))
		eq(t, "gpt-a", panel[0].Model)
		eq(t, "claude-x", panel[1].Model)
		eq(t, "claude-x", panel[2].Model, "dry pool wraps on the last real pick")
		eq(t, "claude-x", panel[3].Model)
	})

	t.Run("namespaced slugs diversify without a creators map", func(t *testing.T) {
		// Old-CM OpenRouter leg: no creators shipped, vendors recovered from
		// the slug prefix.
		s := sel("capable-default",
			m("openai/one", 1.0, 2.0, 200000, 0, 0.95),
			m("openai/two", 1.0, 2.0, 200000, 0, 0.90),
			m("anthropic/x", 1.0, 2.0, 200000, 0, 0.85),
		)

		panel := s.reviewPanel(in, 3)
		eq(t, 3, len(panel))
		eq(t, "openai/one", panel[0].Model)
		eq(t, "anthropic/x", panel[1].Model)
		eq(t, "openai/two", panel[2].Model)
	})
}

func TestSelectReviewPanelClampsBeforeDuplicating(t *testing.T) {
	panel := ladderSelector(nil).reviewPanel(SelectInput{Role: RoleReviewer, Tier: TierCritical}, 3)
	eq(t, 3, len(panel))

	seated := make([]string, len(panel))

	for i, spec := range panel {
		truthy(t, spec.OK, fmt.Sprintf("seat %d", i))
		falsy(t, spec.Duplicate, fmt.Sprintf("seat %d: a clamped-but-distinct seat is not a duplicate", i))
		seated[i] = spec.Model
	}

	eq(t, 3, DistinctModels(panel),
		fmt.Sprintf("seats must be distinct models, not one model repeated: %v", seated))

	// Seat 1 holds the requested tier; the seats below it clamp down a rung to
	// stay distinct rather than duplicating seat 1 at critical price.
	eq(t, "top/one", panel[0].Model)
	eq(t, TierCritical, panel[0].MetTier)
	truthy(t, panel[0].AtBar())

	for _, spec := range panel[1:] {
		truthy(t, spec.BelowBar(), fmt.Sprintf("seat %s must report its clamp", spec.Model))
		eq(t, TierCritical, spec.RequestedTier)
	}
}

// TestSelectReviewPanelSeatsDegradeIndependently pins that one seat dropping a
// rung never drags the seats above it down with it. high/two is excluded so
// the complex pool holds exactly two models (top/one, high/one): both seats 1
// and 2 stay at complex, and only seat 3, with the complex pool now empty,
// clamps to moderate.
func TestSelectReviewPanelSeatsDegradeIndependently(t *testing.T) {
	in := SelectInput{Role: RoleReviewer, Tier: TierComplex, Exclude: map[string]bool{"high/two": true}}
	panel := ladderSelector(nil).reviewPanel(in, 3)
	eq(t, 3, len(panel))

	eq(t, TierComplex, panel[0].MetTier)
	eq(t, TierComplex, panel[1].MetTier, "a second complex-clearing model must stay at complex")
	eq(t, TierModerate, panel[2].MetTier)
	eq(t, 3, DistinctModels(panel))
}

// TestSelectReviewPanelFillsWithARealPickNotAnEscalation pins the last-resort
// order: a repeat is reached only when no rung holds an unseated model, and it
// repeats a real, quality-bearing pick rather than escalating price.
func TestSelectReviewPanelFillsWithARealPickNotAnEscalation(t *testing.T) {
	s := sel("capable/default",
		m("only/one", 1.0, 2.0, 200000, 0, 0.88),
		m("sub/floor", 0.1, 0.2, 200000, 0, 0.40),
	)

	panel := s.reviewPanel(SelectInput{Role: RoleReviewer, Tier: TierCritical}, 3)
	eq(t, 3, len(panel), "the panel is always n seats")

	eq(t, "only/one", panel[0].Model)
	near(t, 0.88, panel[0].Prior, 1e-9)
	falsy(t, panel[0].Duplicate)

	// Seats 2 and 3: only/one is excluded now, so the ladder is dry and the
	// walk lands on the capable default - which repeats the last real pick
	// (only/one) rather than seating the off-ladder default fresh.
	eq(t, "only/one", panel[1].Model, "a dry ladder repeats the last real pick, not the capable default")

	for i, spec := range panel[1:] {
		truthy(t, spec.OK, fmt.Sprintf("seat %d", i+1))
		truthy(t, spec.Duplicate, fmt.Sprintf("seat %d must be flagged as a repeat", i+1))
	}

	truthy(t, DistinctModels(panel) < 3, "the collapse must be countable")
}

// TestSelectReviewPanelVendorPreferenceIsBoundedToTheRung pins that the soft
// diversity preference breaks ties WITHIN a rung and never overrides the
// quality ladder. Without the rung bound, seat 2 walks down to find the fresh
// vendor and the panel trades a measured 0.83 for a measured 0.66.
func TestSelectReviewPanelVendorPreferenceIsBoundedToTheRung(t *testing.T) {
	creators := map[string]string{
		"top/one": "alpha", "high/one": "alpha", "high/two": "alpha",
		"mid/one": "alpha", "mid/two": "alpha", "low/one": "beta",
	}

	models := ladderModels()
	for i, mm := range models {
		if c, ok := creators[mm.id]; ok {
			models[i] = withCreator(mm, c)
		}
	}

	s := newSelector(models, nil, nil, "capable/default")

	panel := s.reviewPanel(SelectInput{Role: RoleReviewer, Tier: TierComplex}, 2)
	eq(t, 2, len(panel))

	eq(t, "high/one", panel[0].Model)
	eq(t, "high/two", panel[1].Model,
		"seat 2 must stay on the complex rung; diversity must not buy a rung of quality")
	truthy(t, panel[1].AtBar())
}

func TestSelectReviewPanelReturnsNothingWhenNoModelIsSelectable(t *testing.T) {
	// No capable default and nothing employable: the only honest answer is none.
	s := sel("", m("sub/floor", 0.1, 0.2, 200000, 0, 0.40))

	truthy(t, s.reviewPanel(SelectInput{Role: RoleReviewer, Tier: TierSimple}, 3) == nil)
}

// TestSelectCandidateModelsPinReportsMeasuredNotAsserted pins that the pin seat
// does not fabricate a met tier: authority is Source, measurement is MetTier.
func TestSelectCandidateModelsPinReportsMeasuredNotAsserted(t *testing.T) {
	s := ladderSelector(nil)

	picks := s.candidateModels(SelectInput{Role: RoleCoder, Tier: TierCritical}, 2, "sub/floor")
	eq(t, 2, len(picks))

	eq(t, "sub/floor", picks[0].Model)
	eq(t, SourcePinned, picks[0].Source, "a pin is authoritative")
	eq(t, Tier(""), picks[0].MetTier, "a 0.40 prior clears no configured bar - say so")
	falsy(t, picks[0].AtBar())
	near(t, 0.40, picks[0].Prior, 1e-9)
	truthy(t, picks[0].HasPrior)

	eq(t, "top/one", picks[1].Model, "the auto seat beside a pin is unaffected")
	truthy(t, picks[1].AtBar())
}

func TestMaxCapabilityReviewPanelSpansVendors(t *testing.T) {
	// Reuse the TestSelectReviewPanelSpansVendors scenario with MaxCapability set.
	// With band = +Inf the vendor-diversity preference still applies because it
	// is driven by ExcludeVendors filtering, not price.
	s := sel("capable-default",
		withCreator(m("gpt-a", 0.7, 1.4, 200000, 0, 0.95), "openai"),
		withCreator(m("gpt-b", 0.9, 1.8, 200000, 0, 0.90), "openai"),
		withCreator(m("gpt-c", 1.0, 2.0, 200000, 0, 0.88), "openai"),
		withCreator(m("claude-x", 1.2, 2.4, 200000, 0, 0.85), "anthropic"),
	)
	s.maxCapability = true

	in := SelectInput{Role: RoleReviewer, Tier: TierComplex, EstTokens: 50000}
	panel := s.reviewPanel(in, 3)
	eq(t, 3, len(panel))

	// Seat 1: all candidates; highest quality gpt-a (0.95). Seat 2: prefers
	// unseated vendor claude-x (0.85). Seat 3: no unseated vendor left,
	// vendor-blind pick gpt-b (0.90).
	eq(t, "gpt-a", panel[0].Model)
	eq(t, "claude-x", panel[1].Model)
	eq(t, "gpt-b", panel[2].Model)
}

func TestPanelUsesTheReviewerLadder(t *testing.T) {
	// The same model clears complex as a coder (0.90 >= 0.82) but not as a
	// reviewer once the reviewer ladder is raised to 0.95: a reviewer panel
	// at complex must descend, a coder pick must not.
	s := sel("capable/default",
		m("a/one", 1, 1, 200000, 0.90, 0.90),
		m("b/two", 2, 2, 200000, 0.80, 0.80),
	)
	s.ladders = Ladders{RoleReviewer: map[Tier]float64{TierSimple: 0.65, TierModerate: 0.76, TierComplex: 0.95, TierCritical: 0.97}}

	coder := s.SelectByComplexity(SelectInput{Role: RoleCoder, Tier: TierComplex})
	eq(t, TierComplex, coder.MetTier)

	seats := s.SelectReviewPanelReport(SelectInput{Role: RoleReviewer, Tier: TierComplex}, 2)
	eq(t, 2, len(seats))
	eq(t, TierModerate, seats[0].Pick.MetTier, "the reviewer walk lands one rung down")
	eq(t, "a/one", seats[0].Pick.Model)
}
