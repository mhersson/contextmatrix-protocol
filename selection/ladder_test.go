package selection

import (
	"maps"
	"math"
	"strings"
	"testing"
)

func TestDefaultTierBars(t *testing.T) {
	b := DefaultTierBars()
	eq(t, 0.65, b[TierSimple])
	eq(t, 0.76, b[TierModerate])
	eq(t, 0.82, b[TierComplex])
	eq(t, 0.90, b[TierCritical])
}

func TestTierBarsFromStringsMergesOverDefaults(t *testing.T) {
	got, err := TierBarsFromStrings(map[string]float64{"critical": 0.95})
	fatalIf(t, err)
	eq(t, 0.95, got[TierCritical])
	eq(t, 0.82, got[TierComplex], "an unnamed rung keeps its default")
	eq(t, 0.65, got[TierSimple])
}

func TestTierBarsFromStringsEmptyIsNil(t *testing.T) {
	got, err := TierBarsFromStrings(nil)
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
		t.Run(name, func(t *testing.T) {
			if _, err := TierBarsFromStrings(in); err == nil {
				t.Errorf("want an error for %v", in)
			}
		})
	}
}

func TestLaddersFromWirePerRole(t *testing.T) {
	got, err := LaddersFromWire(map[string]map[string]float64{
		"coder":    {"complex": 0.90},
		"reviewer": {"critical": 0.93},
	})
	fatalIf(t, err)
	eq(t, 0.90, got[RoleCoder][TierComplex])
	eq(t, 0.76, got[RoleCoder][TierModerate], "coder keeps the default moderate bar")
	eq(t, 0.93, got[RoleReviewer][TierCritical])
	eq(t, 0.82, got[RoleReviewer][TierComplex], "reviewer keeps the default complex bar")
}

func TestLaddersFromWireMissingRoleIsAbsent(t *testing.T) {
	got, err := LaddersFromWire(map[string]map[string]float64{"coder": {"complex": 0.9}})
	fatalIf(t, err)
	if _, ok := got[RoleReviewer]; ok {
		t.Error("a role not on the wire must not appear in the ladders")
	}
	eq(t, 0.82, got.Bars(RoleReviewer)[TierComplex], "Bars falls back to the defaults for an absent role")
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
	eq(t, 0.82, Ladders(nil).Bars(RoleCoder)[TierComplex], "nil ladders read as the defaults")
}

func nan() float64 { var z float64; return z / z }

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
			want: map[Tier]float64{TierSimple: 0.65, TierModerate: 0.76, TierComplex: 0.82, TierCritical: 0.95},
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
			got, err := TierBarsFromStrings(tt.in)
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
