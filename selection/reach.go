package selection

import (
	"cmp"
	"maps"
	"slices"
)

// TierReach is one row of the reachability preflight: how many shipped
// candidates clear one tier's bar for one role, and how far short the
// catalog falls when none do.
type TierReach struct {
	Role  Role
	Tier  Tier
	Bar   float64
	Count int
	Best  float64 // highest prior available for Role, any tier
}

// Reachability reports whether the shipped candidate set can answer a
// request at each tier at all - blacklist applied, no run-time exclusions.
// Count 0 means every request at that tier degrades for the whole run
// regardless of card, which is the one thing card-level logs can never
// reveal. Rows are ordered role, then strictest tier first, each role
// measured against its own ladder.
func (s *Selector) Reachability() []TierReach {
	out := make([]TierReach, 0, 2*len(DefaultTierBars()))

	for _, role := range []Role{RoleCoder, RoleReviewer} {
		bars := s.bars(role)

		tiers := slices.Collect(maps.Keys(bars))
		slices.SortFunc(tiers, func(a, b Tier) int {
			if c := cmp.Compare(bars[b], bars[a]); c != 0 {
				return c
			}

			return cmp.Compare(a, b)
		})

		best := 0.0

		for _, m := range s.models {
			if s.blacklist[m.id] {
				continue
			}

			if q, ok := m.prior(role); ok && q > best {
				best = q
			}
		}

		for _, t := range tiers {
			out = append(out, TierReach{
				Role:  role,
				Tier:  t,
				Bar:   bars[t],
				Count: len(s.candidates(SelectInput{Role: role, Tier: t})),
				Best:  best,
			})
		}
	}

	return out
}

// OrphanFavoriteTiers lists favorite tiers outside the closed set
// DefaultTierBars defines. CM sends FavoriteRule.Tier as a free string and
// New converts it unchecked, so a typo produces a favorite no rung can ever
// consult. The tier set is closed - an operator ladder only reweights its
// rungs, it never adds or drops one - so the check is against the built-in
// set rather than a role's live ladder. Sorted for a stable log line.
func (s *Selector) OrphanFavoriteTiers() []Tier {
	bars := DefaultTierBars()

	seen := map[Tier]bool{}

	for key := range s.favorites {
		if _, ok := bars[key.Tier]; !ok {
			seen[key.Tier] = true
		}
	}

	return slices.Sorted(maps.Keys(seen))
}
