package selection

import (
	"maps"
	"math"
	"slices"
	"strings"
	"testing"
)

var (
	coderBars    = map[Tier]float64{TierSimple: 0.65, TierModerate: 0.75, TierComplex: 0.84, TierCritical: 0.94}
	reviewerBars = map[Tier]float64{TierSimple: 0.90, TierModerate: 0.90, TierComplex: 0.90, TierCritical: 0.92}
)

func TestDefaultTierBarsPerRole(t *testing.T) {
	truthy(t, maps.Equal(DefaultTierBars(RoleCoder), coderBars))
	truthy(t, maps.Equal(DefaultTierBars(RoleReviewer), reviewerBars))
	truthy(t, maps.Equal(DefaultTierBars(Role("judge")), coderBars), "an unknown role reads as coder")

	b := DefaultTierBars(RoleReviewer)
	b[TierComplex] = 0
	eq(t, 0.90, DefaultTierBars(RoleReviewer)[TierComplex], "each call returns a fresh map")
}

func TestTiersIsTheClosedSetInLadderOrder(t *testing.T) {
	want := []Tier{TierSimple, TierModerate, TierComplex, TierCritical}
	truthy(t, slices.Equal(want, Tiers()))

	got := Tiers()
	got[0] = "x"
	truthy(t, slices.Equal(want, Tiers()), "each call returns a fresh slice")
}

func TestTierBarsFromStringsMergesOverTheRolesBuiltIn(t *testing.T) {
	got, err := TierBarsFromStrings(RoleCoder, map[string]float64{"critical": 0.95})
	fatalIf(t, err)
	truthy(t, maps.Equal(map[Tier]float64{TierSimple: 0.65, TierModerate: 0.75, TierComplex: 0.84, TierCritical: 0.95}, got))

	got, err = TierBarsFromStrings(RoleReviewer, map[string]float64{"critical": 0.95})
	fatalIf(t, err)
	truthy(t, maps.Equal(map[Tier]float64{TierSimple: 0.90, TierModerate: 0.90, TierComplex: 0.90, TierCritical: 0.95}, got))
}

func TestTierBarsFromStringsValidatesAgainstTheRolesBuiltIn(t *testing.T) {
	_, err := TierBarsFromStrings(RoleCoder, map[string]float64{"simple": 0.85})
	truthy(t, err != nil, "0.85 is above the coder moderate bar")

	got, err := TierBarsFromStrings(RoleReviewer, map[string]float64{"simple": 0.85})
	fatalIf(t, err)
	truthy(t, maps.Equal(map[Tier]float64{TierSimple: 0.85, TierModerate: 0.90, TierComplex: 0.90, TierCritical: 0.92}, got))
}

func TestTierBarsFromStringsAcceptsEachBuiltInLadder(t *testing.T) {
	for _, role := range []Role{RoleCoder, RoleReviewer} {
		in := map[string]float64{}
		for tier, bar := range DefaultTierBars(role) {
			in[string(tier)] = bar
		}

		got, err := TierBarsFromStrings(role, in)
		fatalIf(t, err)
		truthy(t, maps.Equal(DefaultTierBars(role), got), string(role))
	}
}

func TestLaddersFromWireMergesEachRoleOverItsOwnBuiltIn(t *testing.T) {
	got, err := LaddersFromWire(map[string]map[string]float64{
		"coder":    {"complex": 0.86},
		"reviewer": {"critical": 0.93},
	})
	fatalIf(t, err)
	eq(t, 0.75, got[RoleCoder][TierModerate])
	eq(t, 0.86, got[RoleCoder][TierComplex])
	eq(t, 0.90, got[RoleReviewer][TierComplex])
	eq(t, 0.93, got[RoleReviewer][TierCritical])
}

func TestLaddersBarsFallsBackToTheRolesBuiltIn(t *testing.T) {
	truthy(t, maps.Equal(reviewerBars, Ladders(nil).Bars(RoleReviewer)))
	truthy(t, maps.Equal(coderBars, Ladders(nil).Bars(RoleCoder)))

	custom := maps.Clone(coderBars)
	custom[TierComplex] = 0.86
	truthy(t, maps.Equal(reviewerBars, Ladders{RoleCoder: custom}.Bars(RoleReviewer)))
}

func TestTierBarsFromStringsEmptyIsNil(t *testing.T) {
	got, err := TierBarsFromStrings(RoleCoder, nil)
	fatalIf(t, err)

	if got != nil {
		t.Errorf("empty input must yield nil (defaults), got %v", got)
	}
}

func TestTierBarsFromStringsRejects(t *testing.T) {
	cases := map[string]map[string]float64{
		"unknown tier": {"epic": 0.9},
		"above one":    {"complex": 1.5},
		"negative":     {"simple": -0.1},
		"non-monotone": {"complex": 0.70},
		"nan":          {"simple": nan()},
	}
	for name, in := range cases {
		for _, role := range []Role{RoleCoder, RoleReviewer} {
			t.Run(name+"/"+string(role), func(t *testing.T) {
				if _, err := TierBarsFromStrings(role, in); err == nil {
					t.Errorf("want an error for %v", in)
				}
			})
		}
	}
}

func TestLaddersFromWirePerRole(t *testing.T) {
	got, err := LaddersFromWire(map[string]map[string]float64{
		"coder":    {"complex": 0.90},
		"reviewer": {"critical": 0.93},
	})
	fatalIf(t, err)
	eq(t, 0.90, got[RoleCoder][TierComplex])
	eq(t, 0.75, got[RoleCoder][TierModerate], "coder keeps the default moderate bar")
	eq(t, 0.93, got[RoleReviewer][TierCritical])
	eq(t, 0.90, got[RoleReviewer][TierComplex], "reviewer keeps the default complex bar")
}

func TestLaddersFromWireMissingRoleIsAbsent(t *testing.T) {
	got, err := LaddersFromWire(map[string]map[string]float64{"coder": {"complex": 0.9}})
	fatalIf(t, err)

	if _, ok := got[RoleReviewer]; ok {
		t.Error("a role not on the wire must not appear in the ladders")
	}

	eq(t, 0.90, got.Bars(RoleReviewer)[TierComplex], "Bars falls back to the defaults for an absent role")
	eq(t, 0.9, got.Bars(RoleCoder)[TierComplex])
}

func TestLaddersFromWireRejectsUnknownRole(t *testing.T) {
	_, err := LaddersFromWire(map[string]map[string]float64{"judge": {"complex": 0.9}})
	if err == nil || !strings.Contains(err.Error(), "judge") {
		t.Errorf("want an error naming the unknown role, got %v", err)
	}
}

func TestLaddersFromWireRejectsABadRoleLadder(t *testing.T) {
	_, err := LaddersFromWire(map[string]map[string]float64{"reviewer": {"complex": 0.5}})
	if err == nil || !strings.Contains(err.Error(), "reviewer") {
		t.Errorf("want an error naming the role whose ladder failed, got %v", err)
	}
}

func TestLaddersFromWireEmptyIsNil(t *testing.T) {
	got, err := LaddersFromWire(nil)
	fatalIf(t, err)

	if got != nil {
		t.Errorf("empty input must yield nil, got %v", got)
	}

	eq(t, 0.84, Ladders(nil).Bars(RoleCoder)[TierComplex], "nil ladders read as the defaults")
}

func nan() float64 {
	var z float64

	return z / z
}

func TestTierBarsFromStrings(t *testing.T) {
	tests := []struct {
		name    string
		in      map[string]float64
		want    map[Tier]float64
		wantErr bool
	}{
		{name: "empty ladder uses the defaults", in: nil, want: nil},
		{
			// The edit an operator actually makes. Merging is what stops the
			// three unnamed rungs from silently dropping to bar 0.
			name: "partial ladder inherits the unnamed rungs",
			in:   map[string]float64{"critical": 0.95},
			want: map[Tier]float64{TierSimple: 0.65, TierModerate: 0.75, TierComplex: 0.84, TierCritical: 0.95},
		},
		{
			name: "full ladder replaces every rung",
			in:   map[string]float64{"simple": 0.6, "moderate": 0.7, "complex": 0.8, "critical": 0.9},
			want: map[Tier]float64{TierSimple: 0.6, TierModerate: 0.7, TierComplex: 0.8, TierCritical: 0.9},
		},
		{name: "unknown tier rejected", in: map[string]float64{"trivial": 0.5}, wantErr: true},
		{name: "bar above one rejected", in: map[string]float64{"simple": 1.5}, wantErr: true},
		{name: "negative bar rejected", in: map[string]float64{"simple": -0.1}, wantErr: true},
		{
			// NaN fails both bar<0 and bar>1, so it must be checked explicitly:
			// without the check it would pass range and monotone validation and
			// then fail to marshal into CMX_SELECTOR_TIER_BARS, discarding the
			// operator's entire ladder.
			name:    "NaN bar rejected",
			in:      map[string]float64{"simple": math.NaN()},
			wantErr: true,
		},
		{
			// Passes names and range; makes critical the weakest rung with no
			// descent, so escalating would silently downgrade.
			name:    "inverted ladder rejected",
			in:      map[string]float64{"simple": 0.90, "moderate": 0.80, "complex": 0.70, "critical": 0.50},
			wantErr: true,
		},
		{
			// A single raised rung that crosses its neighbour is the transposed-
			// value case the monotone check is for.
			name:    "partial edit that inverts a neighbour pair is rejected",
			in:      map[string]float64{"moderate": 0.88},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := TierBarsFromStrings(RoleCoder, tt.in)
			if tt.wantErr {
				if err == nil {
					t.Error("want an error")
				}

				return
			}

			fatalIf(t, err)

			if !maps.Equal(tt.want, got) {
				t.Errorf("want %v, got %v", tt.want, got)
			}
		})
	}
}
