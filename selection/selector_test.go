package selection

import (
	"fmt"
	"maps"
	"slices"
	"testing"

	protocol "github.com/mhersson/contextmatrix-protocol"
)

// m builds a model record from prices in dollars per million tokens. A prior
// of 0 means "no measured prior for that role", the same convention the wire
// uses.
func m(id string, promptPerM, completionPerM float64, window int, coder, reviewer float64) model {
	return model{
		id: id, prompt: promptPerM / 1e6, completion: completionPerM / 1e6,
		window: window, coder: coder, reviewer: reviewer,
	}
}

// sel is the test constructor: models with optional priors, no favorites, no
// blacklist, the given capable default, built-in ladders, headroom 1.5.
func sel(capable string, models ...model) *Selector {
	return newSelector(models, nil, nil, capable)
}

// eqSlice compares two slices element by element, for the assertions eq
// cannot make because a slice is not comparable.
func eqSlice[T comparable](t *testing.T, want, got []T, msg ...string) {
	t.Helper()

	if !slices.Equal(want, got) {
		t.Errorf("%swant %v, got %v", prefix(msg), want, got)
	}
}

func poolByModel(rep SelectionReport) map[string]PoolEntry {
	out := make(map[string]PoolEntry, len(rep.Pool))
	for _, e := range rep.Pool {
		out[e.Model] = e
	}

	return out
}

func filteredByReason(rep SelectionReport) map[FilterReason][]string {
	out := make(map[FilterReason][]string, len(rep.FilteredOut))
	for _, e := range rep.FilteredOut {
		out[e.Reason] = e.Models
	}

	return out
}

func TestFitsWindow(t *testing.T) {
	s := sel("x", m("big", 0.5, 1.5, 131072, 0.9, 0), m("small", 0.1, 0.1, 8192, 0.9, 0))
	truthy(t, s.fitsWindow("big", 100000))
	falsy(t, s.fitsWindow("small", 100000))
	truthy(t, s.fitsWindow("unknown/model", 100000), "absent from the catalog fails open")
}

func TestNoPriorFallsBackToCapable(t *testing.T) {
	// A model with no prior for the role is never selectable, so selection
	// falls back to the capable default.
	s := sel("capable/default", m("unseeded/a", 1, 1, 8192, 0, 0), m("unseeded/b", 1, 1, 8192, 0, 0))
	got := s.SelectByComplexity(SelectInput{Role: RoleCoder, Tier: TierSimple})
	eq(t, "capable/default", got.Model)
	eq(t, SourceDefault, got.Source)
	falsy(t, got.HasPrior, "an unmeasured model reports no prior, never a zero")
}

func TestSelectByComplexityPriorsOnly(t *testing.T) {
	// Blended price ($/Mtok): cheap-weak 1.5, cheap-good 2.1, mid-better 2.7,
	// frontier 18.0, star 1.2, small-window 1.8. Built-in bars, headroom 1.5.
	s := sel("capable-default",
		m("cheap-weak", 0.5, 1.0, 200000, 0.50, 0),
		m("cheap-good", 0.7, 1.4, 200000, 0.70, 0),
		m("mid-better", 0.9, 1.8, 200000, 0.85, 0),
		m("frontier", 6.0, 12.0, 200000, 0.95, 0),
		m("star", 0.4, 0.8, 200000, 0.99, 0),
		m("small-window", 0.6, 1.2, 8000, 0.85, 0),
	)

	tests := []struct {
		name string
		in   SelectInput
		want string
	}{
		// simple (bar 0.65): candidates cheap-good, mid-better, frontier, star;
		// cheapest star $1.2 -> band 1.8: only star in band.
		{"cheapest in-band wins", SelectInput{Role: RoleCoder, Tier: TierSimple, EstTokens: 50000}, "star"},
		// star excluded: cheapest cheap-good $2.1 -> band 3.15: cheap-good and
		// mid-better in; highest quality in band is mid-better.
		{"best value beats cheapest", SelectInput{Role: RoleCoder, Tier: TierSimple, EstTokens: 50000, Exclude: map[string]bool{"star": true}}, "mid-better"},
		// moderate (bar 0.76), star excluded: mid-better and frontier; band from
		// $2.7 is 4.05, frontier out.
		{"bar filters weak models", SelectInput{Role: RoleCoder, Tier: TierModerate, EstTokens: 50000, Exclude: map[string]bool{"star": true}}, "mid-better"},
		// complex (bar 0.82): mid-better, frontier, star; cheapest star $1.2 ->
		// band 1.8: only star. A stricter bar does not abandon best value.
		{"complex bar still cost-optimal", SelectInput{Role: RoleCoder, Tier: TierComplex, EstTokens: 50000}, "star"},
		// critical (bar 0.90), star excluded: frontier alone.
		{"only frontier clears critical", SelectInput{Role: RoleCoder, Tier: TierCritical, EstTokens: 50000, Exclude: map[string]bool{"star": true}}, "frontier"},
		// Every wide-window model excluded; small-window clears the bar but
		// cannot hold the estimate, so the rung is dry and the walk falls
		// through to the capable default.
		{"window fit enforced", SelectInput{Role: RoleCoder, Tier: TierSimple, EstTokens: 50000, Exclude: map[string]bool{"star": true, "mid-better": true, "cheap-good": true, "cheap-weak": true, "frontier": true}}, "capable-default"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eq(t, tt.want, s.SelectByComplexity(tt.in).Model)
		})
	}
}

func TestCandidatesArePriorsOnlyAndSkipBlacklist(t *testing.T) {
	s := newSelector(
		[]model{
			m("cheap/ok", 0.1, 0.2, 200000, 0.80, 0),
			m("black/listed", 0.01, 0.01, 200000, 0.95, 0),
		},
		map[string]bool{"black/listed": true}, nil, "capable/default",
	)

	// The complex bar (0.82) is above cheap/ok's 0.80, so the complex rung is
	// dry and the request clamps to moderate (0.76), returning the pick a
	// direct moderate request would have made. The answer names what it is
	// worth: a moderate met tier and a shortfall against the request. The
	// capable default is the floor BELOW the ladder, not the answer to the
	// first dry rung.
	got := s.SelectByComplexity(SelectInput{Role: RoleCoder, Tier: TierComplex})
	truthy(t, got.OK)
	eq(t, "cheap/ok", got.Model)
	eq(t, SourceAuto, got.Source, "a clamped pick is a real selection at its rung, not the default")
	eq(t, TierComplex, got.RequestedTier)
	eq(t, TierModerate, got.MetTier)
	falsy(t, got.AtBar(), "the caller must be able to see it did not get what it asked for")

	// The blacklisted model has the best prior and WOULD clear complex. It is
	// unselectable at every rung: the ladder relaxes the quality bar and
	// nothing else.
	got = s.SelectByComplexity(SelectInput{Role: RoleCoder, Tier: TierModerate})
	eq(t, "cheap/ok", got.Model, "blacklisted must never win")
	truthy(t, got.AtBar())
}

func TestFavoritesConsideredFirst(t *testing.T) {
	s := newSelector(
		[]model{
			m("cheap/win", 0.01, 0.01, 200000, 0.90, 0),
			m("fav/pick", 1.0, 1.0, 200000, 0.90, 0),
		},
		nil, map[favKey][]string{{Tier: TierComplex}: {"fav/pick"}}, "capable/default",
	)

	got := s.SelectByComplexity(SelectInput{Role: RoleCoder, Tier: TierComplex})
	eq(t, "fav/pick", got.Model, "favorite must win over cheaper cost-optimal")
}

func TestMaxCapabilityBypassesFavorites(t *testing.T) {
	models := []model{
		m("cheap/win", 0.01, 0.01, 200000, 0.90, 0),
		m("fav/pick", 1.0, 1.0, 200000, 0.90, 0),
	}
	favs := map[favKey][]string{{Tier: TierComplex}: {"fav/pick"}}

	t.Run("favorite honored by default", func(t *testing.T) {
		s := newSelector(models, nil, favs, "capable/default")
		eq(t, "fav/pick", s.SelectByComplexity(SelectInput{Role: RoleCoder, Tier: TierComplex}).Model)
	})

	t.Run("favorite bypassed with MaxCapability", func(t *testing.T) {
		s := newSelector(models, nil, favs, "capable/default")
		s.maxCapability = true
		// cheap/win is cheaper and same quality; tie breaks to cheaper.
		eq(t, "cheap/win", s.SelectByComplexity(SelectInput{Role: RoleCoder, Tier: TierComplex}).Model)
	})
}

func TestMaxCapabilitySelectsMostExpensiveQualifying(t *testing.T) {
	// frontier is the most expensive ($18, q0.95) and star the cheapest
	// ($1.2, q0.99). Excluding star makes frontier the highest-quality
	// remaining candidate.
	models := []model{
		m("cheap-weak", 0.5, 1.0, 200000, 0.50, 0),
		m("cheap-good", 0.7, 1.4, 200000, 0.70, 0),
		m("mid-better", 0.9, 1.8, 200000, 0.85, 0),
		m("frontier", 6.0, 12.0, 200000, 0.95, 0),
		m("star", 0.4, 0.8, 200000, 0.99, 0),
		m("small-window", 0.6, 1.2, 8000, 0.85, 0),
	}
	// TierSimple (bar 0.65): cheap-weak (0.50 < 0.65) out; small-window
	// (8k < 50k) out. star excluded; remaining: cheap-good, mid-better, frontier.
	in := SelectInput{Role: RoleCoder, Tier: TierSimple, EstTokens: 50000, Exclude: map[string]bool{"star": true}}

	t.Run("default picks best value", func(t *testing.T) {
		s := sel("capable-default", models...)
		// cheapest $2.1 -> band $3.15; frontier out; highest in band: mid-better.
		eq(t, "mid-better", s.SelectByComplexity(in).Model)
	})

	t.Run("MaxCapability picks most capable regardless of price", func(t *testing.T) {
		s := sel("capable-default", models...)
		s.maxCapability = true
		// band = +Inf; frontier has the highest quality (0.95).
		eq(t, "frontier", s.SelectByComplexity(in).Model)
	})
}

func TestMaxCapabilityRespectsTierBar(t *testing.T) {
	s := sel("capable-default",
		m("below-bar", 0.5, 1.0, 200000, 0.50, 0), // below the simple bar 0.65
		m("above-bar", 1.0, 2.0, 200000, 0.80, 0),
	)
	s.maxCapability = true

	got := s.SelectByComplexity(SelectInput{Role: RoleCoder, Tier: TierSimple, EstTokens: 50000})
	eq(t, "above-bar", got.Model, "below-bar must never be selected even with MaxCapability")
}

func TestMaxCapabilityRespectsBlacklist(t *testing.T) {
	s := newSelector(
		[]model{
			m("good/model", 1.0, 2.0, 200000, 0.80, 0),
			m("black/listed", 0.5, 1.0, 200000, 0.95, 0),
		},
		map[string]bool{"black/listed": true}, nil, "capable-default",
	)
	s.maxCapability = true

	got := s.SelectByComplexity(SelectInput{Role: RoleCoder, Tier: TierSimple, EstTokens: 50000})
	eq(t, "good/model", got.Model, "blacklisted must never be selected even with MaxCapability")
}

func TestMaxCapabilityEqualQualityTieBreaksToCheaper(t *testing.T) {
	s := sel("capable-default",
		m("cheap/model", 1.0, 2.0, 200000, 0.90, 0),       // $3
		m("expensive/model", 10.0, 20.0, 200000, 0.90, 0), // $30
	)
	s.maxCapability = true

	got := s.SelectByComplexity(SelectInput{Role: RoleCoder, Tier: TierSimple, EstTokens: 50000})
	eq(t, "cheap/model", got.Model, "equal quality must tie-break to the cheaper model")
}

func TestMaxCapabilityEmptyPoolFallsBack(t *testing.T) {
	// No model carries a prior; the pool is empty.
	s := sel("capable-default", m("any/model", 1.0, 2.0, 200000, 0, 0))
	s.maxCapability = true

	got := s.SelectByComplexity(SelectInput{Role: RoleCoder, Tier: TierSimple, EstTokens: 50000})
	eq(t, "capable-default", got.Model)
}

// ladderModels is the seven-rung fixture the walk tests are written against:
// one model above every bar, two in the complex band, two moderate, one
// simple and one below the floor.
func ladderModels() []model {
	return []model{
		m("top/one", 5.0, 10.0, 200000, 0.93, 0.93),
		m("high/one", 1.0, 2.0, 200000, 0.85, 0.85),
		m("high/two", 1.1, 2.2, 200000, 0.83, 0.83),
		m("mid/one", 0.6, 1.2, 200000, 0.79, 0.79),
		m("mid/two", 0.5, 1.0, 200000, 0.77, 0.77),
		m("low/one", 0.2, 0.4, 200000, 0.66, 0.66),
		m("sub/floor", 0.1, 0.2, 200000, 0.40, 0.40),
	}
}

func ladderSelector(favorites map[favKey][]string) *Selector {
	return newSelector(ladderModels(), nil, favorites, "capable/default")
}

// oldPick recomputes the selection for in.Tier ALONE - no walk, no rung
// ordering - as the oracle for the at-bar guarantee. It deliberately reuses
// the same helpers the walk calls, so its job is to prove the walk never
// touches an answer the walk should not reach.
func oldPick(s *Selector, in SelectInput) string {
	blind := in
	blind.ExcludeVendors = nil

	if fav := s.favoriteAmong(s.candidates(blind), in.Tier, in.Role); fav != "" {
		return fav
	}

	cands := s.candidates(in)
	if len(cands) == 0 {
		return ""
	}

	best, _ := valuePick(cands, priceBand(cands, s.headroomOrDefault(), s.maxCapability))

	return best.id
}

// TestAtBarSelectionsAreUnchanged is THE production-behaviour guard. Every
// selection whose requested tier has a non-empty pool must return exactly the
// model the un-walked selector returned, down to the tie-break.
func TestAtBarSelectionsAreUnchanged(t *testing.T) {
	tests := []struct {
		name string
		favs map[favKey][]string
	}{
		{name: "no favorites"},
		{name: "operator favorites configured", favs: map[favKey][]string{
			{Tier: TierModerate}:                    {"mid/two"},
			{Tier: TierComplex, Role: RoleReviewer}: {"high/two"},
		}},
	}

	tiers := []Tier{TierSimple, TierModerate, TierComplex, TierCritical}
	exclusions := []map[string]bool{
		nil,
		{"mid/two": true},
		{"low/one": true, "mid/two": true},
		{"top/one": true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := ladderSelector(tt.favs)

			covered := 0

			for _, role := range []Role{RoleCoder, RoleReviewer} {
				for _, tier := range tiers {
					for i, excl := range exclusions {
						in := SelectInput{Role: role, Tier: tier, Exclude: excl, EstTokens: 50000}
						if len(s.candidates(in)) == 0 {
							continue // the ladder case, covered by the walk tests
						}

						covered++
						got := s.SelectByComplexity(in)

						truthy(t, got.AtBar(), fmt.Sprintf(
							"a non-empty pool at %s must be served at %s (role=%s excl=%d)", tier, tier, role, i))
						eq(t, oldPick(s, in), got.Model, fmt.Sprintf(
							"role=%s tier=%s excl=%d: the ladder changed an at-bar answer", role, tier, i))
					}
				}
			}

			truthy(t, covered > 0, "fixture drift: the matrix exercised no at-bar selection")
		})
	}
}

func TestSelectByComplexityClampsDownTheLadder(t *testing.T) {
	s := ladderSelector(nil)

	tests := []struct {
		name    string
		in      SelectInput
		wantID  string
		wantMet Tier
	}{
		{
			name:    "requested tier has a pool",
			in:      SelectInput{Role: RoleCoder, Tier: TierCritical},
			wantID:  "top/one",
			wantMet: TierCritical,
		},
		{
			// Critical is dry, so the pick is the one a DIRECT complex request
			// would have made, not the capable default - which may be weaker
			// than high/one.
			name:    "critical dry clamps to complex",
			in:      SelectInput{Role: RoleCoder, Tier: TierCritical, Exclude: map[string]bool{"top/one": true}},
			wantID:  "high/one",
			wantMet: TierComplex,
		},
		{
			name: "two rungs dry clamps to moderate",
			in: SelectInput{Role: RoleCoder, Tier: TierCritical, Exclude: map[string]bool{
				"top/one": true, "high/one": true, "high/two": true,
			}},
			wantID:  "mid/one",
			wantMet: TierModerate,
		},
		{
			name: "three rungs dry clamps to simple",
			in: SelectInput{Role: RoleCoder, Tier: TierCritical, Exclude: map[string]bool{
				"top/one": true, "high/one": true, "high/two": true, "mid/one": true, "mid/two": true,
			}},
			wantID:  "low/one",
			wantMet: TierSimple,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.SelectByComplexity(tt.in)

			truthy(t, got.OK)
			eq(t, tt.wantID, got.Model)
			eq(t, tt.wantMet, got.MetTier)
			eq(t, tt.in.Tier, got.RequestedTier, "the request must be reported unchanged")
			eq(t, tt.wantMet == tt.in.Tier, got.AtBar())
			eq(t, SourceAuto, got.Source)
			truthy(t, got.ContextWindow > 0, "a real pick carries its window")
		})
	}
}

// TestEscalationNeverDowngrades pins the escalation-is-a-downgrade
// regression: under identical exclusions, asking for a HIGHER tier must
// never return a model with a LOWER prior. Clamping down the ladder instead
// of jumping straight to the capable default is what keeps this true even
// when the requested tier's pool is empty.
func TestEscalationNeverDowngrades(t *testing.T) {
	s := ladderSelector(nil)

	// The exclusion sets a run actually produces: the panel walk and the
	// incapable-model recovery both feed growing Exclude sets in.
	exclusionSets := []map[string]bool{
		nil,
		{"top/one": true},
		{"top/one": true, "high/one": true},
		{"top/one": true, "high/one": true, "high/two": true},
		{"top/one": true, "high/one": true, "high/two": true, "mid/one": true},
		{"top/one": true, "high/one": true, "high/two": true, "mid/one": true, "mid/two": true, "low/one": true},
	}

	ladder := []Tier{TierSimple, TierModerate, TierComplex, TierCritical}

	for i, excl := range exclusionSets {
		t.Run(fmt.Sprintf("exclusions_%d", i), func(t *testing.T) {
			for lo := range ladder {
				for hi := lo + 1; hi < len(ladder); hi++ {
					low := s.SelectByComplexity(SelectInput{Role: RoleCoder, Tier: ladder[lo], Exclude: excl})
					high := s.SelectByComplexity(SelectInput{Role: RoleCoder, Tier: ladder[hi], Exclude: excl})

					eq(t, low.OK, high.OK, fmt.Sprintf(
						"a higher tier is selectable exactly when a lower one is (%s vs %s)", ladder[lo], ladder[hi]))

					if !low.OK {
						continue
					}

					truthy(t, high.Prior >= low.Prior, fmt.Sprintf(
						"escalating %s -> %s downgraded: %s (%.2f) -> %s (%.2f)",
						ladder[lo], ladder[hi], low.Model, low.Prior, high.Model, high.Prior))
				}
			}
		})
	}
}

// TestCapableDefaultIsTheFloorAndIsHardFiltered pins that the bottom of the
// ladder is the OPERATOR's default (it is a configured slug, not junk), but
// it is subject to every hard filter the same as any candidate - so an
// excluded or blacklisted default can never be handed back.
func TestCapableDefaultIsTheFloorAndIsHardFiltered(t *testing.T) {
	allAboveFloorGone := map[string]bool{
		"top/one": true, "high/one": true, "high/two": true,
		"mid/one": true, "mid/two": true, "low/one": true,
	}

	withDefaultExcluded := maps.Clone(allAboveFloorGone)
	withDefaultExcluded["capable/default"] = true

	// A tier absent from the configured ladder has bar 0 (barFor's documented
	// fallback), so every fixture model's prior would trivially clear it if
	// any reached the pool - excluding all seven is what forces the walk down
	// to the capable default at this tier too.
	everyLadderModelGone := map[string]bool{
		"top/one": true, "high/one": true, "high/two": true,
		"mid/one": true, "mid/two": true, "low/one": true, "sub/floor": true,
	}

	tests := []struct {
		name      string
		in        SelectInput
		blacklist map[string]bool
		// extra ships the capable default as a scored candidate, the shape it
		// takes when the wire carries a rating for the operator's fallback.
		extra        []model
		wantOK       bool
		wantModel    string
		wantHasPrior bool
		wantPrior    float64
		wantMetTier  Tier
	}{
		{
			name:      "ladder dry falls to the operator default",
			in:        SelectInput{Role: RoleCoder, Tier: TierCritical, Exclude: allAboveFloorGone},
			wantOK:    true,
			wantModel: "capable/default",
		},
		{
			// The case that matters: a caller that has seen the default fail
			// puts it in Exclude, and it must never be handed back regardless.
			name:   "an excluded default is never resurrected",
			in:     SelectInput{Role: RoleCoder, Tier: TierCritical, Exclude: withDefaultExcluded},
			wantOK: false,
		},
		{
			name:      "a blacklisted default is never resurrected",
			in:        SelectInput{Role: RoleCoder, Tier: TierCritical, Exclude: allAboveFloorGone},
			blacklist: map[string]bool{"capable/default": true},
			wantOK:    false,
		},
		{
			// A tier with no configured bar is a degenerate rung, not a free
			// pass: the capable default's MetTier must still come from a real
			// prior, never from a prior-less model trivially clearing a bar of
			// zero at the tier that was asked for.
			name:      "an off-ladder tier still measures the default honestly",
			in:        SelectInput{Role: RoleCoder, Tier: Tier("unrecognised"), Exclude: everyLadderModelGone},
			wantOK:    true,
			wantModel: "capable/default",
		},
		{
			// The other direction of the same encoding: a default the wire did
			// score reports that score, and its MetTier stays measured - 0.40
			// clears no rung, so it is empty rather than the critical that was
			// asked for. A score at or above the lowest bar cannot reach this
			// branch at all: it would make the default a candidate and the
			// walk would pick it on a rung instead of falling past the ladder.
			name:         "a scored default reports its measurement",
			in:           SelectInput{Role: RoleCoder, Tier: TierCritical, Exclude: allAboveFloorGone},
			extra:        []model{m("capable/default", 1.0, 2.0, 200000, 0.40, 0)},
			wantOK:       true,
			wantModel:    "capable/default",
			wantHasPrior: true,
			wantPrior:    0.40,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newSelector(append(ladderModels(), tt.extra...), nil, nil, "capable/default")
			for id := range tt.blacklist {
				s.blacklist[id] = true
			}

			got := s.SelectByComplexity(tt.in)

			eq(t, tt.wantOK, got.OK)

			if !tt.wantOK {
				eq(t, "", got.Model, "a refusal carries no model at all")
				eq(t, tt.in.Tier, got.RequestedTier)

				return
			}

			eq(t, tt.wantModel, got.Model)
			eq(t, SourceDefault, got.Source)
			falsy(t, got.AtBar(), "the default sits below the ladder, so it never meets the bar asked for")
			eq(t, tt.wantHasPrior, got.HasPrior)
			near(t, tt.wantPrior, got.Prior, 1e-9)
			eq(t, tt.wantMetTier, got.MetTier, "MetTier is measured, never asserted")
		})
	}
}

// TestMetTierEqualsTheReachedRung pins the theorem the walk relies on: because
// each rung's pool is a superset of the rung above it under identical hard
// filters, a rung is reached only when nothing in it clears the next bar up. So
// the measured MetTier is exactly the rung the walk stopped at, and the two
// encodings can never disagree.
func TestMetTierEqualsTheReachedRung(t *testing.T) {
	s := ladderSelector(nil)

	for _, excl := range []map[string]bool{
		nil,
		{"top/one": true},
		{"top/one": true, "high/one": true, "high/two": true},
	} {
		got := s.SelectByComplexity(SelectInput{Role: RoleReviewer, Tier: TierCritical, Exclude: excl})
		truthy(t, got.OK)
		neq(t, Tier(""), got.MetTier)

		at := SelectInput{Role: RoleReviewer, Tier: got.MetTier, Exclude: excl}
		truthy(t, len(s.candidates(at)) > 0, "the met rung must hold the pick")
		truthy(t, got.Prior >= s.barFor(RoleReviewer, got.MetTier))

		for _, rung := range s.descent(RoleReviewer, TierCritical) {
			if s.barFor(RoleReviewer, rung) > s.barFor(RoleReviewer, got.MetTier) {
				above := SelectInput{Role: RoleReviewer, Tier: rung, Exclude: excl}
				eq(t, 0, len(s.candidates(above)), fmt.Sprintf(
					"rung %s was skipped but is not dry - the walk stopped too early", rung))
			}
		}
	}
}

func TestFavoritesAreConsultedAtTheMetRung(t *testing.T) {
	tests := []struct {
		name       string
		favorites  map[favKey][]string
		in         SelectInput
		wantID     string
		wantMet    Tier
		wantSource PickSource
	}{
		{
			// The requested tier is dry, so the pick is made on the moderate
			// rung - and a moderate selection must honour the operator's
			// moderate favorite even though the request itself was critical.
			name:      "clamped pick honours the met rung's favorite",
			favorites: map[favKey][]string{{Tier: TierModerate}: {"mid/two"}},
			in: SelectInput{Role: RoleCoder, Tier: TierCritical, Exclude: map[string]bool{
				"top/one": true, "high/one": true, "high/two": true,
			}},
			wantID: "mid/two", wantMet: TierModerate, wantSource: SourceFavorite,
		},
		{
			// mid/two (0.77) is below the complex bar and complex has a pool, so
			// the moderate favorite must not hijack it.
			name:      "lower-tier favorite never hijacks a live higher rung",
			favorites: map[favKey][]string{{Tier: TierModerate}: {"mid/two"}},
			in:        SelectInput{Role: RoleCoder, Tier: TierComplex},
			wantID:    "high/one", wantMet: TierComplex, wantSource: SourceAuto,
		},
		{
			// Even a favorite that WOULD clear the higher bar is not inherited
			// upward: top/one clears complex, but the operator configured it for
			// moderate.
			name:      "favorites are not inherited upward even when eligible",
			favorites: map[favKey][]string{{Tier: TierModerate}: {"top/one"}},
			in:        SelectInput{Role: RoleCoder, Tier: TierComplex},
			wantID:    "high/one", wantMet: TierComplex, wantSource: SourceAuto,
		},
		{
			name:      "favorites are not inherited downward",
			favorites: map[favKey][]string{{Tier: TierComplex}: {"high/one"}},
			in:        SelectInput{Role: RoleCoder, Tier: TierSimple},
			wantID:    "low/one", wantMet: TierSimple, wantSource: SourceAuto,
		},
		{
			name: "role-specific favorite beats the any-role favorite at the same tier",
			favorites: map[favKey][]string{
				{Tier: TierComplex, Role: RoleCoder}: {"high/two"},
				{Tier: TierComplex}:                  {"high/one"},
			},
			in:     SelectInput{Role: RoleCoder, Tier: TierComplex},
			wantID: "high/two", wantMet: TierComplex, wantSource: SourceFavorite,
		},
		{
			// A favorite that does not clear its own tier's bar is not eligible,
			// so the rung falls through to the cost-optimal pick.
			name:      "favorite below its own tier bar is ignored",
			favorites: map[favKey][]string{{Tier: TierComplex}: {"mid/one"}},
			in:        SelectInput{Role: RoleCoder, Tier: TierComplex},
			wantID:    "high/one", wantMet: TierComplex, wantSource: SourceAuto,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ladderSelector(tt.favorites).SelectByComplexity(tt.in)

			truthy(t, got.OK)
			eq(t, tt.wantID, got.Model)
			eq(t, tt.wantMet, got.MetTier)
			eq(t, tt.wantSource, got.Source)
		})
	}
}

// TestMonotonicityHoldsOnTheAutoPathAndFavoritesAreTheException sweeps favorite
// configurations rather than testing only the input class where monotonicity is
// trivially true. The auto path must be monotone under every configuration; a
// violation is permitted ONLY where a favorite fired, and the test asserts that
// pairing so the exception cannot silently widen.
func TestMonotonicityHoldsOnTheAutoPathAndFavoritesAreTheException(t *testing.T) {
	favoriteSets := []map[favKey][]string{
		nil,
		{{Tier: TierModerate}: {"mid/two"}},
		{{Tier: TierComplex}: {"high/two"}},
		{{Tier: TierSimple}: {"low/one"}, {Tier: TierCritical}: {"top/one"}},
		// The counterexample the exception exists for: an expensive, strong
		// favorite at moderate that the complex band would exclude on price.
		{{Tier: TierModerate}: {"top/one"}},
	}

	ladder := []Tier{TierSimple, TierModerate, TierComplex, TierCritical}
	exclusions := []map[string]bool{nil, {"top/one": true}, {"top/one": true, "high/one": true}}

	sawException := false

	for fi, favs := range favoriteSets {
		t.Run(fmt.Sprintf("favorites_%d", fi), func(t *testing.T) {
			s := ladderSelector(favs)

			for ei, excl := range exclusions {
				for lo := range ladder {
					for hi := lo + 1; hi < len(ladder); hi++ {
						low := s.SelectByComplexity(SelectInput{Role: RoleCoder, Tier: ladder[lo], Exclude: excl})
						high := s.SelectByComplexity(SelectInput{Role: RoleCoder, Tier: ladder[hi], Exclude: excl})

						// A pick with no prior reports Prior 0, which is not a
						// measurement: comparing it would invent a violation, or
						// hide one behind a zero neither model earned.
						if !low.OK || !high.OK || !low.HasPrior || !high.HasPrior {
							continue
						}

						if high.Prior >= low.Prior {
							continue
						}

						sawException = true

						eq(t, SourceFavorite, low.Source, fmt.Sprintf(
							"excl=%d %s(%.2f) > %s(%.2f) is only permitted when a favorite fired at the lower tier",
							ei, low.Model, low.Prior, high.Model, high.Prior))
					}
				}
			}
		})
	}

	truthy(t, sawException, "fixture drift: no favorite configuration exercised the documented exception")
}

func TestPartialLadderCannotZeroTheUnnamedBars(t *testing.T) {
	bars, err := TierBarsFromStrings(map[string]float64{"critical": 0.95})
	fatalIf(t, err)

	s := ladderSelector(nil)
	s.ladders = Ladders{RoleCoder: bars, RoleReviewer: bars}

	got := s.SelectByComplexity(SelectInput{Role: RoleCoder, Tier: TierModerate})
	truthy(t, got.OK)
	eq(t, "mid/one", got.Model, "the moderate bar must still be 0.76")
	truthy(t, got.AtBar())
	near(t, 0.76, got.RequestedBar, 1e-9)
}

// TestOperatorLadderDrivesTheWalk proves the knob is not decorative: replacing
// the bars replaces both the thresholds and the descent order.
func TestOperatorLadderDrivesTheWalk(t *testing.T) {
	bars := map[Tier]float64{
		TierSimple: 0.50, TierModerate: 0.60, TierComplex: 0.70, TierCritical: 0.95,
	}
	s := ladderSelector(nil)
	s.ladders = Ladders{RoleCoder: bars, RoleReviewer: bars}

	// Nothing clears 0.95, so the request clamps to the operator's complex bar
	// (0.70), which admits top/one, high/one, high/two, mid/one and mid/two
	// (low/one's 0.66 prior still falls short of 0.70). Cheapest in that pool
	// is mid/two at $1.5, band $2.25, and mid/one ($1.8, quality 0.79) beats
	// mid/two (quality 0.77) within the band.
	got := s.SelectByComplexity(SelectInput{Role: RoleCoder, Tier: TierCritical})

	truthy(t, got.OK)
	eq(t, TierComplex, got.MetTier)
	near(t, 0.95, got.RequestedBar, 1e-9)
	eq(t, "mid/one", got.Model)

	// And the descent order follows the configured table.
	eqSlice(t, []Tier{TierCritical, TierComplex, TierModerate, TierSimple}, s.descent(RoleCoder, TierCritical))
}

// TestDescentCollapsesTiedAdjacentBars pins the tie-collapse case descent's
// doc comment describes: this ladder is not order-isomorphic to the default
// one, since the default has four distinct bars and this one ties complex
// and critical, so only a genuinely-threaded ladder can produce this order.
func TestDescentCollapsesTiedAdjacentBars(t *testing.T) {
	s := sel("")
	s.ladders = Ladders{RoleCoder: {
		TierSimple: 0.50, TierModerate: 0.60, TierComplex: 0.70, TierCritical: 0.70,
	}}

	// complex ties critical's bar rather than falling strictly below it, so
	// the walk skips it: three rungs, not four.
	eqSlice(t, []Tier{TierCritical, TierModerate, TierSimple}, s.descent(RoleCoder, TierCritical))
}

func TestTierBarsIncludeCritical(t *testing.T) {
	s := &Selector{}
	if got := s.barFor(RoleCoder, TierCritical); got != 0.90 {
		t.Errorf("critical bar = %v, want 0.90", got)
	}

	if got := s.barFor(RoleCoder, TierSimple); got != 0.65 {
		t.Errorf("simple bar = %v, want 0.65", got)
	}
}

func TestContextWindow(t *testing.T) {
	s := sel("x", m("big", 0.5, 1.5, 131072, 0.9, 0), m("small", 0.1, 0.1, 8192, 0.9, 0))

	eq(t, 131072, s.ContextWindow("big"))
	eq(t, 8192, s.ContextWindow("small"))
	eq(t, 0, s.ContextWindow("unknown/model"))
}

// TestTierOfIsTheStrictestTierAPriorClears pins the membership rule an
// operator page draws its bands from: the same descent the selector walks,
// read from the top of the role's own ladder.
func TestTierOfIsTheStrictestTierAPriorClears(t *testing.T) {
	s := sel("capable/default")

	for _, tc := range []struct {
		prior  float64
		want   Tier
		wantOK bool
	}{
		{0.99, TierCritical, true},
		{0.90, TierCritical, true},
		{0.85, TierComplex, true},
		{0.79, TierModerate, true},
		{0.65, TierSimple, true},
		{0.64, "", false},
		{0, "", false},
	} {
		got, ok := s.TierOf(RoleCoder, tc.prior)
		eq(t, tc.want, got, fmt.Sprintf("prior %.2f", tc.prior))
		eq(t, tc.wantOK, ok, fmt.Sprintf("prior %.2f", tc.prior))
	}

	// Each role reads its own ladder, so the same prior lands differently.
	s.ladders = Ladders{RoleReviewer: {
		TierSimple: 0.10, TierModerate: 0.20, TierComplex: 0.30, TierCritical: 0.40,
	}}

	got, ok := s.TierOf(RoleReviewer, 0.35)
	eq(t, TierComplex, got)
	truthy(t, ok)

	got, ok = s.TierOf(RoleCoder, 0.35)
	eq(t, Tier(""), got, "the role the operator left out keeps the built-in bars")
	falsy(t, ok)
}

func TestNewBuildsFromWire(t *testing.T) {
	candidates := []protocol.CandidateModel{
		{
			Slug: "vendor/strong", PromptPricePerTok: 5e-6, CompletionPricePerTok: 1e-5,
			ContextWindow: 200000, CoderPrior: 0.93, ReviewerPrior: 0.93, Creator: "alpha",
		},
		{
			Slug: "vendor/cheap", PromptPricePerTok: 1e-6, CompletionPricePerTok: 2e-6,
			ContextWindow: 200000, CoderPrior: 0.85, ReviewerPrior: 0.85,
		},
		{
			Slug: "vendor/barred", PromptPricePerTok: 1e-8, CompletionPricePerTok: 1e-8,
			ContextWindow: 200000, CoderPrior: 0.99, ReviewerPrior: 0.99,
		},
	}

	ladders, err := LaddersFromWire(map[string]map[string]float64{"coder": {"critical": 0.95}})
	fatalIf(t, err)

	s := New(Input{
		Candidates: candidates,
		Favorites:  []protocol.FavoriteRule{{Tier: "complex", Role: "coder", Models: []string{"vendor/strong"}}},
		Blacklist:  []string{"vendor/barred"},
		Ladders:    ladders,
		Capable:    "capable/default",
	})

	truthy(t, s.Has("vendor/strong"))
	falsy(t, s.Has("vendor/missing"))
	eq(t, 200000, s.ContextWindow("vendor/cheap"))
	eq(t, "alpha", s.Vendor("vendor/strong"), "the wire creator wins over the slug prefix")
	eq(t, "vendor", s.Vendor("vendor/cheap"), "no creator falls back to the slug namespace")
	near(t, 0.95, s.BarFor(RoleCoder, TierCritical), 1e-9, "the operator ladder reaches the selector")
	near(t, 0.90, s.BarFor(RoleReviewer, TierCritical), 1e-9, "a role the operator left out keeps the built-in bars")
	near(t, defaultPriceHeadroom, s.headroomOrDefault(), 1e-9, "an unset headroom is the built-in one")

	coder := s.SelectByComplexity(SelectInput{Role: RoleCoder, Tier: TierComplex})
	eq(t, "vendor/strong", coder.Model, "the wire favorite is honoured at its own tier and role")
	eq(t, SourceFavorite, coder.Source)

	// The favorite is a coder rule, so the reviewer falls to the best-value
	// pick; the blacklisted model carries the best prior and still never wins.
	reviewer := s.SelectByComplexity(SelectInput{Role: RoleReviewer, Tier: TierComplex})
	eq(t, "vendor/cheap", reviewer.Model)
	eq(t, SourceAuto, reviewer.Source)

	t.Run("per-run knobs reach the selector", func(t *testing.T) {
		s := New(Input{Candidates: candidates, PriceHeadroom: 10, MaxCapability: true, Capable: "capable/default"})

		near(t, 10, s.headroomOrDefault(), 1e-9)
		eq(t, "vendor/barred", s.SelectByComplexity(SelectInput{Role: RoleCoder, Tier: TierComplex}).Model,
			"MaxCapability takes the strongest candidate regardless of price")
	})
}

func TestSelectByComplexityReportClassifiesThePool(t *testing.T) {
	// The complex rung (bar 0.82): the pool is top/one ($15, q0.93), high/one
	// ($3, q0.85), high/two ($3.3, q0.83). Cheapest $3 -> band $4.5: high/one
	// wins, high/two is in band, top/one is priced out. Everything below the
	// bar (mid/one 0.79, mid/two 0.77, low/one 0.66, sub/floor 0.40) never
	// reaches the pool.
	s := ladderSelector(nil)
	in := SelectInput{Role: RoleCoder, Tier: TierComplex, EstTokens: 50000}

	pick, rep := s.SelectByComplexityReport(in)

	eq(t, "high/one", pick.Model)
	eq(t, s.SelectByComplexity(in), pick, "the report variant must not change the pick")
	eq(t, TierComplex, rep.Rung)
	near(t, 0.82, rep.Bar, 1e-9)

	pool := poolByModel(rep)
	eq(t, 3, len(pool), "the report pool is the rung's candidates, no more")

	eq(t, PoolSelected, pool["high/one"].Outcome)
	near(t, 0.85, pool["high/one"].Prior, 1e-9)
	near(t, 3.0/1e6, pool["high/one"].Price, 1e-15)
	eq(t, PoolInBand, pool["high/two"].Outcome)
	eq(t, PoolOutOfBand, pool["top/one"].Outcome, "the stronger model is out of band: $15 exceeds the $4.5 band")

	filtered := filteredByReason(rep)
	eq(t, 1, len(filtered))
	eqSlice(t, []string{"mid/one", "mid/two", "low/one", "sub/floor"}, filtered[FilterPriorBelowBar])
}

func TestSelectByComplexityReportNamesTheRungItLandedOn(t *testing.T) {
	// The complex rung is dry (every complex-clearing model excluded), so the
	// pick clamps to moderate. The report must describe the moderate rung's
	// pool - mid/one and mid/two - not the complex one, and the filtered-out
	// summary is the moderate rung's view too: low/one (0.66) sits below the
	// moderate bar while the excluded top of the ladder shows as excluded.
	s := ladderSelector(nil)
	in := SelectInput{Role: RoleCoder, Tier: TierComplex, EstTokens: 50000, Exclude: map[string]bool{
		"top/one": true, "high/one": true, "high/two": true,
	}}

	pick, rep := s.SelectByComplexityReport(in)

	eq(t, "mid/one", pick.Model)
	eq(t, TierModerate, rep.Rung, "the report names the rung the pick was made on, not the tier asked for")
	near(t, 0.76, rep.Bar, 1e-9)

	pool := poolByModel(rep)
	eq(t, 2, len(pool))
	eq(t, PoolSelected, pool["mid/one"].Outcome)
	eq(t, PoolInBand, pool["mid/two"].Outcome, "mid/two ($1.5 -> band $2.25) is in band but loses on quality")

	_, seated := pool["top/one"]
	falsy(t, seated, "the dry requested rung contributes nothing to the report")

	filtered := filteredByReason(rep)
	eqSlice(t, []string{"top/one", "high/one", "high/two"}, filtered[FilterExcluded])
	eqSlice(t, []string{"low/one", "sub/floor"}, filtered[FilterPriorBelowBar])
}

func TestSelectByComplexityReportDefaultHasNoRungPool(t *testing.T) {
	// Every ladder model excluded: the walk falls through to the capable
	// default, which sits below the ladder and has no rung - the report is
	// empty and the pick carries the provenance.
	s := ladderSelector(nil)
	in := SelectInput{Role: RoleCoder, Tier: TierCritical, EstTokens: 50000, Exclude: map[string]bool{
		"top/one": true, "high/one": true, "high/two": true,
		"mid/one": true, "mid/two": true, "low/one": true, "sub/floor": true,
	}}

	pick, rep := s.SelectByComplexityReport(in)

	eq(t, "capable/default", pick.Model)
	eq(t, SourceDefault, pick.Source)
	eq(t, Tier(""), rep.Rung, "an off-ladder default has no rung")
	near(t, 0, rep.Bar, 1e-9)
	eq(t, 0, len(rep.Pool), "no rung means no pool")
	eq(t, 0, len(rep.FilteredOut), "no rung means no filtered-out summary")
}

func TestSelectByComplexityReportGrowingExcludeStaysOutOfThePool(t *testing.T) {
	// A panel-style walk: seat N calls the report variant with seats 1..N-1
	// excluded, and every earlier seat shows up in the filtered-out summary,
	// never in the pool.
	s := ladderSelector(nil)

	exclude := map[string]bool{}

	for seat, want := range []string{"high/one", "high/two", "top/one"} {
		in := SelectInput{Role: RoleReviewer, Tier: TierComplex, EstTokens: 50000, Exclude: exclude}

		pick, rep := s.SelectByComplexityReport(in)
		eq(t, want, pick.Model, fmt.Sprintf("seat %d", seat+1))

		pool := poolByModel(rep)
		filtered := filteredByReason(rep)

		for slug := range exclude {
			_, seated := pool[slug]
			falsy(t, seated, fmt.Sprintf("seat %d: previously seated model %q must not be in the pool", seat+1, slug))
			truthy(t, slices.Contains(filtered[FilterExcluded], slug), fmt.Sprintf(
				"seat %d: seated model %q must be attributable in the filtered-out summary", seat+1, slug))
		}

		exclude[pick.Model] = true
	}
}

func TestSelectByComplexityReportFavoriteMarksThePool(t *testing.T) {
	// The favorite is the most expensive model in the pool: it is marked
	// selected wherever it sits, and the band winner it displaced is reported
	// as in band, so the log shows what the automatic rule would have done.
	s := ladderSelector(map[favKey][]string{{Tier: TierComplex}: {"top/one"}})
	in := SelectInput{Role: RoleCoder, Tier: TierComplex, EstTokens: 50000}

	pick, rep := s.SelectByComplexityReport(in)

	eq(t, "top/one", pick.Model)
	eq(t, SourceFavorite, pick.Source)
	eq(t, TierComplex, rep.Rung)

	pool := poolByModel(rep)
	eq(t, 3, len(pool))
	eq(t, PoolSelected, pool["top/one"].Outcome)
	eq(t, PoolInBand, pool["high/one"].Outcome, "the favorite displaced the band winner, which reports in band")
	eq(t, PoolInBand, pool["high/two"].Outcome)
}

func TestSelectByComplexityReportMaxCapabilityBandsNothing(t *testing.T) {
	// With MaxCapability the band is unbounded: no candidate is out of band,
	// and the highest-quality candidate wins.
	s := ladderSelector(nil)
	s.maxCapability = true
	in := SelectInput{Role: RoleCoder, Tier: TierComplex, EstTokens: 50000}

	pick, rep := s.SelectByComplexityReport(in)

	eq(t, "top/one", pick.Model)

	for _, e := range rep.Pool {
		neq(t, PoolOutOfBand, e.Outcome, fmt.Sprintf("an infinite band must not mark %s out of band", e.Model))
	}

	eq(t, PoolSelected, poolByModel(rep)["top/one"].Outcome)
	eq(t, PoolInBand, poolByModel(rep)["high/one"].Outcome)
}

func TestSelectByComplexityReportBucketingIsCompleteAndDisjoint(t *testing.T) {
	// Every candidate lands in exactly one bucket: the pool or one filtered
	// reason, and the whole candidate set is accounted for.
	models := ladderModels()
	for i := range models {
		if models[i].id == "mid/one" || models[i].id == "mid/two" {
			models[i].creator = "alpha"
		}
	}

	s := newSelector(models, map[string]bool{"low/one": true}, nil, "capable/default")

	in := SelectInput{
		Role: RoleCoder, Tier: TierModerate, EstTokens: 15000,
		Exclude:        map[string]bool{"sub/floor": true},
		ExcludeVendors: map[string]bool{"alpha": true},
	}

	pick, rep := s.SelectByComplexityReport(in)

	eq(t, "high/one", pick.Model)

	seen := map[string]bool{}
	for _, e := range rep.Pool {
		falsy(t, seen[e.Model], fmt.Sprintf("%s appears twice", e.Model))
		seen[e.Model] = true
	}

	for _, entry := range rep.FilteredOut {
		for _, slug := range entry.Models {
			falsy(t, seen[slug], fmt.Sprintf("%s appears in both the pool and %s", slug, entry.Reason))
			seen[slug] = true
		}
	}

	eq(t, 7, len(seen), "every candidate is accounted for exactly once")

	filtered := filteredByReason(rep)
	eqSlice(t, []string{"mid/one", "mid/two"}, filtered[FilterVendorExcluded],
		"models with a resolvable, excluded vendor are vendor-excluded")
	eqSlice(t, []string{"low/one"}, filtered[FilterBlacklisted])
	eqSlice(t, []string{"sub/floor"}, filtered[FilterExcluded])

	_, unscored := filtered[FilterNoPrior]
	falsy(t, unscored, "every candidate carries a coder prior here, so the no-prior bucket is absent")
}

func TestSelectByComplexityReportBucketsModelsWithoutAPrior(t *testing.T) {
	// A candidate with no prior for the role is bucketed, not silently
	// dropped from the report.
	s := sel("capable-default",
		m("scored/a", 1.0, 2.0, 200000, 0.80, 0),
		m("unscored/b", 1.0, 2.0, 200000, 0, 0),
	)

	pick, rep := s.SelectByComplexityReport(SelectInput{Role: RoleCoder, Tier: TierSimple, EstTokens: 50000})

	eq(t, "scored/a", pick.Model)
	eqSlice(t, []string{"unscored/b"}, filteredByReason(rep)[FilterNoPrior])
	eqSlice(t, []PoolEntry{{Model: "scored/a", Prior: 0.80, Price: 3.0 / 1e6, Outcome: PoolSelected}}, rep.Pool)
}
