package selection

import (
	"fmt"
	"maps"
	"math"
	"slices"
)

type Role string

const (
	RoleCoder    Role = "coder"
	RoleReviewer Role = "reviewer"
)

type Tier string

const (
	TierSimple   Tier = "simple"
	TierModerate Tier = "moderate"
	TierComplex  Tier = "complex"
	TierCritical Tier = "critical"
)

// tierRung pairs a tier with its default quality bar. tierLadder is the
// single ordered source both DefaultTierBars and the monotone check in
// TierBarsFromStrings are built from, so a tier added here is automatically
// present and ordered in both.
type tierRung struct {
	tier Tier
	bar  float64
}

var tierLadder = []tierRung{
	{TierSimple, 0.65},
	{TierModerate, 0.76},
	{TierComplex, 0.82},
	{TierCritical, 0.90},
}

// DefaultTierBars are the normalized-prior thresholds per complexity tier.
func DefaultTierBars() map[Tier]float64 {
	out := make(map[Tier]float64, len(tierLadder))
	for _, rung := range tierLadder {
		out[rung.tier] = rung.bar
	}

	return out
}

// TierBarsFromStrings converts an operator ladder to the typed map. It
// MERGES over the defaults rather than replacing them: a partial map raises
// or lowers the rungs it names and inherits the rest, so a one-line edit
// like {"critical": 0.95} cannot silently drop the other three bars to zero,
// which would admit every model carrying any prior while every Pick still
// reported AtBar() true.
//
// It also rejects a non-monotone ladder. An inverted ladder passes name and
// range validation but makes descent() treat the strictest tier as the
// weakest rung with nothing below it, so escalating would return a WORSE
// model and report no clamp. The check needs no threshold and catches
// transposed values.
func TierBarsFromStrings(in map[string]float64) (map[Tier]float64, error) {
	if len(in) == 0 {
		return nil, nil
	}

	out := DefaultTierBars()

	for name, bar := range in {
		t := Tier(name)
		if _, ok := out[t]; !ok {
			return nil, fmt.Errorf("tier bars: unknown tier %q (known: %v)",
				name, slices.Sorted(maps.Keys(DefaultTierBars())))
		}

		if math.IsNaN(bar) || bar < 0 || bar > 1 {
			return nil, fmt.Errorf("tier bars: %s must be in [0,1], got %g", name, bar)
		}

		out[t] = bar
	}

	for i := 1; i < len(tierLadder); i++ {
		hi, lo := tierLadder[i].tier, tierLadder[i-1].tier
		if out[hi] < out[lo] {
			return nil, fmt.Errorf("tier bars: ladder must not decrease: %s %g is below %s %g",
				hi, out[hi], lo, out[lo])
		}
	}

	return out, nil
}

// Ladders is the operator's quality ladder per role. A role absent from the
// map reads as DefaultTierBars through Bars, so a nil Ladders is the
// built-in behaviour and a caller never has to special-case it.
type Ladders map[Role]map[Tier]float64

// roleNames is the closed set a wire ladder may be keyed on.
var roleNames = map[string]Role{string(RoleCoder): RoleCoder, string(RoleReviewer): RoleReviewer}

// LaddersFromWire validates SelectionContext.TierBars role by role. Each
// role's map goes through TierBarsFromStrings, so a partial map merges over
// the defaults and a non-monotone ladder is rejected. An unknown role is an
// error rather than ignored: a typo silently leaving a role on the defaults
// is the failure the operator cannot see. Empty input is nil, nil.
func LaddersFromWire(in map[string]map[string]float64) (Ladders, error) {
	if len(in) == 0 {
		return nil, nil
	}

	out := make(Ladders, len(in))

	for name, bars := range in {
		role, ok := roleNames[name]
		if !ok {
			return nil, fmt.Errorf("tier bars: unknown role %q (known: %v)",
				name, slices.Sorted(maps.Keys(roleNames)))
		}

		ladder, err := TierBarsFromStrings(bars)
		if err != nil {
			return nil, fmt.Errorf("tier bars for %s: %w", name, err)
		}

		if ladder != nil {
			out[role] = ladder
		}
	}

	return out, nil
}

// Bars returns the ladder for role: the configured one when present and
// non-empty, else the built-in bars. This is the ONLY way the selector
// reads a bar, so every rung lookup is role-aware.
func (l Ladders) Bars(role Role) map[Tier]float64 {
	if bars := l[role]; len(bars) > 0 {
		return bars
	}

	return DefaultTierBars()
}
