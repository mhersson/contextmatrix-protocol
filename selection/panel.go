package selection

import "maps"

// SeatReport pairs one panel or candidate seat with the SelectionReport of
// the selection that chose it, so the caller can log the competing pool
// beside the pick. A seat filled by duplication carries no report: the seat
// it repeats already logged that pool.
type SeatReport struct {
	Pick   Pick
	Report SelectionReport
}

// SeatPicks strips a seat list to its picks, for callers that consume a
// panel positionally.
func SeatPicks(seats []SeatReport) []Pick {
	if seats == nil {
		return nil
	}

	picks := make([]Pick, 0, len(seats))
	for _, s := range seats {
		picks = append(picks, s.Pick)
	}

	return picks
}

// SelectReviewPanelReport returns exactly n seats for the review specialists.
// Distinct models first, each seat preferring an unseated vendor and walking a
// rung down rather than repeating the seat above it; a duplicate is the last
// resort, flagged so the synthesizer cannot read it as agreement. Always n
// seats. Only selector-chosen seats carry a report; duplicates carry none.
func (s *Selector) SelectReviewPanelReport(in SelectInput, n int) []SeatReport {
	if n <= 0 {
		return nil
	}

	exclude := maps.Clone(in.Exclude)
	if exclude == nil {
		exclude = map[string]bool{}
	}

	usedVendors := maps.Clone(in.ExcludeVendors) // e.g. a Best-of-N pin's vendor
	if usedVendors == nil {
		usedVendors = map[string]bool{}
	}

	panel := make([]SeatReport, 0, n)

	var last SeatReport

	for len(panel) < n {
		seat := in
		seat.Exclude = exclude
		seat.ExcludeVendors = nil

		blind, blindRep := s.SelectByComplexityReport(seat)

		// No distinct model remains at any rung, and even the capable
		// default is barred: repeat the last real pick.
		if !blind.OK {
			if len(panel) == 0 {
				return nil // nothing is selectable for this role at all
			}

			dup := last.Pick
			dup.Duplicate = true
			panel = append(panel, SeatReport{Pick: dup})

			continue
		}

		// The capable default sits below every configured rung. Once a real
		// seat already holds the panel, an off-ladder default is not a
		// second independent judgement - it is the same terminal fill as an
		// unselectable rung, so it repeats the previous seat instead of
		// occupying a seat of its own. The first seat is the exception:
		// with nothing on the panel yet, the default IS the answer.
		if blind.Source == SourceDefault && len(panel) > 0 {
			dup := last.Pick
			dup.Duplicate = true
			panel = append(panel, SeatReport{Pick: dup})

			continue
		}

		pick := blind
		rep := blindRep

		// Soft vendor preference, bounded to the rung the vendor-blind pick
		// landed on: diversity breaks ties within a rung, it never
		// overrides the quality ladder. Accepting a filtered pick from a
		// lower rung would let a soft, emergent preference outrank a
		// measured bar - and high-prior models cluster in a handful of
		// vendors, so a 3-seat panel would routinely push seats 2-3 far
		// down the ladder chasing a fresh vendor. The price band still
		// re-anchors on the filtered subset, so a diverse seat may cost
		// more than the vendor-blind pick - accepted, that is the
		// documented cost of diversity.
		if len(usedVendors) > 0 {
			filtered := seat
			filtered.ExcludeVendors = usedVendors

			if f, frep := s.SelectByComplexityReport(filtered); f.OK && f.MetTier == blind.MetTier {
				pick = f
				rep = frep
			}
		}

		panel = append(panel, SeatReport{Pick: pick, Report: rep})
		last = panel[len(panel)-1]
		exclude[pick.Model] = true

		if v := s.vendorOf(pick.Model); v != "" {
			usedVendors[v] = true
		}
	}

	return panel
}

// SelectDiscussionPanelReport returns n seats for mob session discussion,
// honoring the caller's exclusions (review discussions exclude the models
// that coded the card). It is the review-panel selection by construction -
// distinct models first, a flagged repeat of the last seat as the last
// resort, nil when nothing is selectable at all - named as its own seam so
// discussion selection can diverge from review selection without touching
// call sites.
func (s *Selector) SelectDiscussionPanelReport(in SelectInput, n int) []SeatReport {
	return s.SelectReviewPanelReport(in, n)
}

// SelectCandidateModelsReport picks n coder models for a Best-of-N fan-out with
// each seat's competing pool, on SelectReviewPanelReport's contract. A non-empty
// pin occupies slot 1, is never degraded away and carries no report; with a pin
// an unselectable auto pool fills the rest with flagged repeats of the pin, so a
// pinned fan-out still gets n candidates. With no pin it returns nil instead.
func (s *Selector) SelectCandidateModelsReport(in SelectInput, n int, pin string) []SeatReport {
	if n <= 0 {
		return nil
	}

	if pin == "" {
		return s.SelectReviewPanelReport(in, n)
	}

	next := in
	next.Exclude = map[string]bool{pin: true}

	for id := range in.Exclude {
		next.Exclude[id] = true
	}

	// The pin occupies a seat, so its vendor counts as seated for the
	// auto-picked slots (fresh map; the caller's is never mutated).
	if v := s.vendorOf(pin); v != "" {
		next.ExcludeVendors = map[string]bool{v: true}
		for u := range in.ExcludeVendors {
			next.ExcludeVendors[u] = true
		}
	}

	// The pin is operator intent and is never degraded away - but its MetTier
	// and Prior are measured like every other seat's. Asserting MetTier ==
	// in.Tier would make the field mean "measured" for auto picks and
	// "asserted" for pins, and any aggregate over it would then be silently
	// wrong for every pinned run. Source SourcePinned is what carries the
	// authority; AtBar carries the measurement.
	pinPick := s.pickFor(pin, in, SourcePinned)

	out := make([]SeatReport, 0, n)
	out = append(out, SeatReport{Pick: pinPick})

	rest := s.SelectReviewPanelReport(next, n-1)
	if rest == nil && n > 1 {
		// Nothing else is selectable: fill the remaining seats with the pin,
		// flagged, so the fan-out still gets n candidates.
		for range n - 1 {
			dup := pinPick
			dup.Duplicate = true
			out = append(out, SeatReport{Pick: dup})
		}

		return out
	}

	return append(out, rest...)
}
