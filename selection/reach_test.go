package selection

import (
	"testing"

	protocol "github.com/mhersson/contextmatrix-protocol"
)

func TestReachabilityReportsTheGapAtUnreachableTiers(t *testing.T) {
	s := ladderSelector(nil)

	reviewerByTier := func() map[Tier]TierReach {
		out := map[Tier]TierReach{}

		for _, rc := range s.Reachability() {
			if rc.Role == RoleReviewer {
				out[rc.Tier] = rc
			}
		}

		return out
	}

	got := reviewerByTier()
	eq(t, 4, len(got))
	truthy(t, got[TierCritical].Count > 0)
	near(t, 0.93, got[TierCritical].Best, 1e-9)
	near(t, 0.90, got[TierCritical].Bar, 1e-9)

	// Blacklisting the only critical-clearing model makes the tier structurally
	// unreachable for the whole run - what the preflight exists to surface.
	s.blacklist["top/one"] = true

	got = reviewerByTier()
	eq(t, 0, got[TierCritical].Count, "critical is now unreachable")
	near(t, 0.85, got[TierCritical].Best, 1e-9,
		"the report must say how far short the catalog falls, not only that it does")
	truthy(t, got[TierComplex].Count > 0, "lower tiers stay reachable")
}

// TestReachabilityIsOrderedStrictestFirst pins that the report walks the
// operator's ladder rather than a hardcoded tier list, so a replaced ladder
// reports in its own order.
func TestReachabilityIsOrderedStrictestFirst(t *testing.T) {
	s := ladderSelector(nil)
	bars := map[Tier]float64{
		TierSimple: 0.50, TierModerate: 0.60, TierComplex: 0.70, TierCritical: 0.95,
	}
	s.ladders = Ladders{RoleCoder: bars, RoleReviewer: bars}

	var coderTiers []Tier

	for _, rc := range s.Reachability() {
		if rc.Role == RoleCoder {
			coderTiers = append(coderTiers, rc.Tier)
		}
	}

	eqSlice(t, []Tier{TierCritical, TierComplex, TierModerate, TierSimple}, coderTiers)
}

// TestOrphanFavoriteTiersNamesFavoritesNoRungCanConsult pins the dead-favorite
// class the rung-local lookup does not fix: CM sends the tier as a free string
// and New converts it unchecked.
func TestOrphanFavoriteTiersNamesFavoritesNoRungCanConsult(t *testing.T) {
	tests := []struct {
		name string
		favs map[favKey][]string
		want []Tier
	}{
		{name: "no favorites", want: nil},
		{
			name: "every favorite tier is on the ladder",
			favs: map[favKey][]string{{Tier: TierModerate}: {"mid/two"}, {Tier: TierComplex}: {"high/one"}},
			want: nil,
		},
		{
			name: "misspelled and empty tiers are reported",
			favs: map[favKey][]string{
				{Tier: TierModerate}:      {"mid/two"},
				{Tier: Tier("moderatte")}: {"mid/one"},
				{Tier: Tier("")}:          {"low/one"},
			},
			want: []Tier{Tier(""), Tier("moderatte")},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eqSlice(t, tt.want, ladderSelector(tt.favs).OrphanFavoriteTiers())
		})
	}
}

// TestOrphanFavoriteTiersUsesTheClosedTierSet pins that a favorite's tier is
// checked against Tiers(), not a role's live ladder: a custom ladder that omits
// a rung does not orphan a favorite configured on it, since the name is still
// a real tier.
func TestOrphanFavoriteTiersUsesTheClosedTierSet(t *testing.T) {
	s := ladderSelector(map[favKey][]string{{Tier: TierModerate}: {"mid/two"}})
	bars := map[Tier]float64{TierSimple: 0.50, TierComplex: 0.70, TierCritical: 0.90}
	s.ladders = Ladders{RoleCoder: bars, RoleReviewer: bars}

	eqSlice(t, []Tier(nil), s.OrphanFavoriteTiers(),
		"moderate is still a member of the closed tier set even though this ladder omits it")
}

func TestReachabilityUsesEachRolesLadder(t *testing.T) {
	s := sel("capable/default", m("a/one", 1, 1, 200000, 0.85, 0.85))
	s.ladders = Ladders{RoleReviewer: map[Tier]float64{TierSimple: 0.65, TierModerate: 0.76, TierComplex: 0.90, TierCritical: 0.95}}

	var coderComplex, reviewerComplex TierReach

	for _, r := range s.Reachability() {
		if r.Tier == TierComplex && r.Role == RoleCoder {
			coderComplex = r
		}

		if r.Tier == TierComplex && r.Role == RoleReviewer {
			reviewerComplex = r
		}
	}

	eq(t, 0.84, coderComplex.Bar, "the coder keeps its own built-in complex bar")
	eq(t, 1, coderComplex.Count)
	eq(t, 0.90, reviewerComplex.Bar)
	eq(t, 0, reviewerComplex.Count, "the raised reviewer bar is not cleared")
}

func TestReachabilityWithTiedReviewerBars(t *testing.T) {
	s := New(Input{
		Candidates: []protocol.CandidateModel{reviewerCandidate("r/mid", 0.91, 1)},
		Capable:    "capable/default",
	})

	var tiers []Tier

	byTier := map[Tier]TierReach{}

	for _, rc := range s.Reachability() {
		if rc.Role == RoleReviewer {
			tiers = append(tiers, rc.Tier)
			byTier[rc.Tier] = rc
		}
	}

	eqSlice(t, []Tier{TierCritical, TierComplex, TierModerate, TierSimple}, tiers)
	eq(t, 0, byTier[TierCritical].Count)

	for _, tier := range []Tier{TierComplex, TierModerate, TierSimple} {
		near(t, 0.90, byTier[tier].Bar, 1e-9)
		eq(t, 1, byTier[tier].Count, string(tier))
	}
}
