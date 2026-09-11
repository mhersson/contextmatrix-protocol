package selection

import (
	"cmp"
	"maps"
	"math"
	"slices"
	"strings"

	protocol "github.com/mhersson/contextmatrix-protocol"
)

// PickSource says HOW a selection was reached. It is orthogonal to AtBar,
// which says what the selection is WORTH: a SourceDefault pick may clear
// the requested bar and a SourceAuto pick may not.
type PickSource uint8

const (
	SourceAuto     PickSource = iota // best-value pick from a rung's pool
	SourceFavorite                   // an eligible operator favorite at the rung
	SourcePinned                     // an operator pin, synthesized by the caller
	SourceDefault                    // the operator's capable default; off the ladder
)

func (s PickSource) String() string {
	switch s {
	case SourceFavorite:
		return "favorite"
	case SourcePinned:
		return "pinned"
	case SourceDefault:
		return "capable-default"
	default:
		return "auto"
	}
}

// ModelSpec is the model a caller configures for a role, with the context
// window when one is known.
type ModelSpec struct {
	Model         string
	ContextWindow int // 0 if unknown
}

// Pick is one selection outcome. A ModelSpec alone cannot say "I could not
// do what you asked": a caller that only sees the chosen model has no way
// to tell an at-bar pick from a degraded fallback. Pick carries that
// provenance explicitly. It embeds ModelSpec so callers that only need the
// model itself need no edit.
type Pick struct {
	ModelSpec

	Role          Role
	RequestedTier Tier
	// MetTier is the strictest configured tier at or below RequestedTier
	// whose bar this model's prior clears; "" when it has no prior. It is
	// MEASURED for every Source - a pin that clears nothing reports "".
	MetTier      Tier
	RequestedBar float64
	Prior        float64
	HasPrior     bool // separates "prior is 0" from "no prior"
	Source       PickSource
	Duplicate    bool    // panel seat repeating an earlier seat's model
	OK           bool    // false only when even the capable default is barred
	LowestBar    float64 // the bottom of the configured ladder that was walked
}

// AtBar reports that the pick clears the bar that was asked for. Callers
// using a tier as a GATE rather than a preference branch on this. It is the
// reason SelectByComplexity returns a value instead of taking a
// Refuse|Degrade knob: the caller knows what a shortfall costs.
func (p Pick) AtBar() bool { return p.OK && p.MetTier == p.RequestedTier }

// BelowBar is the reportable shortfall: a real selection that did not meet
// the requested bar.
func (p Pick) BelowBar() bool { return p.OK && p.MetTier != p.RequestedTier }

// DistinctModels: a panel whose seats collapse onto one model is not a
// panel; callers use this to say so.
func DistinctModels(picks []Pick) int {
	seen := make(map[string]bool, len(picks))
	for _, p := range picks {
		seen[p.Model] = true
	}

	return len(seen)
}

// favKey indexes operator-pinned favorites by complexity tier and (optionally)
// role. A zero Role applies the favorite list to every role at that tier.
type favKey struct {
	Tier Tier
	Role Role // "" = applies to all roles
}

// SelectInput describes a single best-value selection request.
type SelectInput struct {
	Role      Role
	Tier      Tier
	EstTokens int             // window-fit estimate; 0 skips the window check
	Exclude   map[string]bool // diversity: models to avoid if alternatives exist
	// ExcludeVendors is a hard filter in candidates(); the panel walk applies
	// it softly (vendor-filtered attempt first, retry without on an empty pool).
	ExcludeVendors map[string]bool
}

// SelectionReport explains where a selection landed and who competed for it.
// Rung is the tier the pick was made on and Bar that rung's configured bar;
// both are empty when the walk fell through the ladder to the capable
// default, which has no rung. Pool lists every candidate that reached the
// rung with its outcome; FilteredOut summarizes why the rest never got
// there. A favorite pick reports the pool of the rung it was looked up on,
// the favorite marked selected.
type SelectionReport struct {
	Rung Tier
	Bar  float64
	Pool []PoolEntry
	// FilteredOut is reason-aggregated with the model slugs in candidate
	// order, so a growing Exclude set reads as one growing entry.
	FilteredOut []FilteredOutEntry
}

// PoolOutcome classifies how one pool candidate fared.
type PoolOutcome string

const (
	// PoolSelected: the candidate is the pick.
	PoolSelected PoolOutcome = "selected"
	// PoolInBand: the candidate was inside the price band but lost on
	// quality, or tied on quality and lost to a cheaper model.
	PoolInBand PoolOutcome = "in-band"
	// PoolOutOfBand: the candidate's price exceeds the cheapest candidate
	// times the headroom. With MaxCapability the band is unbounded, so no
	// candidate is out of band.
	PoolOutOfBand PoolOutcome = "out-of-band"
)

// PoolEntry is one candidate that reached a rung's pool.
type PoolEntry struct {
	Model string
	// Prior is the model's normalized prior for the requested role.
	Prior float64
	// Price is the combined prompt+completion price per token.
	Price   float64
	Outcome PoolOutcome
}

// FilterReason says why a candidate never reached a rung's pool.
type FilterReason string

const (
	FilterPriorBelowBar  FilterReason = "prior-below-bar"
	FilterNoPrior        FilterReason = "no-prior-for-role"
	FilterExcluded       FilterReason = "excluded"
	FilterBlacklisted    FilterReason = "blacklisted"
	FilterVendorExcluded FilterReason = "vendor-excluded"
	FilterWindowTooSmall FilterReason = "window-too-small"
)

// FilteredOutEntry aggregates every candidate kept out of a rung's pool by
// the same reason.
type FilteredOutEntry struct {
	Reason FilterReason
	Models []string
}

// rungResult is one rung's outcome: the winner with its source, and the
// classification of every candidate against that rung - the pool the winner
// was drawn from with per-candidate outcomes, and the reason buckets for the
// models that never reached it.
type rungResult struct {
	winner   string
	source   PickSource
	pool     []candidate
	outcomes []PoolOutcome
	filtered []FilteredOutEntry
}

// candidate is a model that passed the gate/bar/window filters, carried with the
// quality score and blended price used by the best-value rule.
type candidate struct {
	id      string
	quality float64
	price   float64
}

// DefaultPriceHeadroom is the best-value band multiplier applied when the
// operator has set none. CM reports it and falls back to it in the admin
// preview; the agent applies it, so both sides read one number.
const DefaultPriceHeadroom = 1.5

// model is one catalog row as the selector sees it. A prior of 0 means no
// measured prior for that role: CM encodes a missing Artificial Analysis
// index as 0 on the wire, and a normalised index is never genuinely 0, so
// the two readings coincide. Such a model is never a candidate for that
// role, and a pick of it (as the capable default) reports HasPrior false so
// no caller prints "prior 0.00" for a model nothing measured.
type model struct {
	id                 string
	prompt, completion float64
	window             int
	coder, reviewer    float64
	creator            string
}

func (m model) price() float64 { return m.prompt + m.completion }

func (m model) prior(role Role) (float64, bool) {
	var v float64

	switch role {
	case RoleCoder:
		v = m.coder
	case RoleReviewer:
		v = m.reviewer
	}

	return v, v > 0
}

// Input is everything a selection needs, all of it from the wire or the
// operator's per-run settings. Capable is the off-ladder default the walk
// falls to when no rung holds a candidate.
type Input struct {
	Candidates []protocol.CandidateModel
	Favorites  []protocol.FavoriteRule
	Blacklist  []string
	// Ladders is the operator's quality ladder per role. It is the ONLY
	// source of tier ordering: descent sorts it, so an operator ladder with
	// different rungs walks correctly. A role's map must be complete (every
	// tier in DefaultTierBars); build it with LaddersFromWire or
	// TierBarsFromStrings, never by hand, since a partial map reads a
	// missing tier as bar 0.
	Ladders Ladders
	// PriceHeadroom is the best-value band multiplier. A value below 1
	// would put the band below the cheapest candidate, so anything below 1
	// (including the 0 of an absent wire field) reads as
	// DefaultPriceHeadroom.
	PriceHeadroom float64
	// MaxCapability makes every pick choose the most capable candidate in the
	// tier regardless of price, and bypass operator favorites. It is a
	// per-request setting, not a property of the catalog.
	MaxCapability bool
	Capable       string
}

// Selector is an immutable, payload-driven selector. Build one per run (or
// per preview) with New; it holds no run-time state, so it is safe to share.
type Selector struct {
	capable       string
	models        []model
	byID          map[string]int
	blacklist     map[string]bool
	favorites     map[favKey][]string
	ladders       Ladders
	headroom      float64
	maxCapability bool
}

// New builds a Selector from the wire types. Every candidate is tool-capable
// by construction (CM filtered on it), so there is no tools gate here.
func New(in Input) *Selector {
	models := make([]model, 0, len(in.Candidates))

	for _, c := range in.Candidates {
		models = append(models, model{
			id: c.Slug, prompt: c.PromptPricePerTok, completion: c.CompletionPricePerTok,
			window: c.ContextWindow, coder: c.CoderPrior, reviewer: c.ReviewerPrior, creator: c.Creator,
		})
	}

	blacklist := make(map[string]bool, len(in.Blacklist))
	for _, s := range in.Blacklist {
		blacklist[s] = true
	}

	favorites := make(map[favKey][]string, len(in.Favorites))
	for _, fr := range in.Favorites {
		favorites[favKey{Tier: Tier(fr.Tier), Role: Role(fr.Role)}] = fr.Models
	}

	s := newSelector(models, blacklist, favorites, in.Capable)
	s.ladders = in.Ladders
	s.maxCapability = in.MaxCapability

	if in.PriceHeadroom >= 1 {
		s.headroom = in.PriceHeadroom
	}

	return s
}

// newSelector is the constructor tests use to hand in models directly; New
// is the only public way in.
func newSelector(models []model, blacklist map[string]bool, favorites map[favKey][]string, capable string) *Selector {
	if blacklist == nil {
		blacklist = map[string]bool{}
	}

	if favorites == nil {
		favorites = map[favKey][]string{}
	}

	byID := make(map[string]int, len(models))
	for i, m := range models {
		byID[m.id] = i
	}

	return &Selector{
		capable: capable, models: models, byID: byID,
		blacklist: blacklist, favorites: favorites, headroom: DefaultPriceHeadroom,
	}
}

func (s *Selector) find(id string) (model, bool) {
	i, ok := s.byID[id]
	if !ok {
		return model{}, false
	}

	return s.models[i], true
}

// Has reports whether model is a shipped candidate. Callers use it to decide
// whether a card-pinned slug is resolvable before honouring the pin.
func (s *Selector) Has(id string) bool {
	_, ok := s.find(id)

	return ok
}

// ContextWindow is the candidate's window, or 0 when absent (0 disables the
// caller's context-limit check for it).
func (s *Selector) ContextWindow(id string) int {
	m, ok := s.find(id)
	if !ok {
		return 0
	}

	return m.window
}

// Bars is the ladder the selector applies for role. The map is a clone: this
// type is documented immutable, so a caller cannot reconfigure the live
// selector by mutating what it gets back.
func (s *Selector) Bars(role Role) map[Tier]float64 { return maps.Clone(s.bars(role)) }

// BarFor is the bar a request at (role, tier) must clear. It reads through
// the unexported bars() rather than Bars(), so the hot selection path does
// not pay for a clone on every lookup.
func (s *Selector) BarFor(role Role, tier Tier) float64 { return s.bars(role)[tier] }

// TierOf is the strictest tier a prior clears for role, false when it clears
// none. It is the membership rule an operator page draws bands from, so it
// is exported rather than left to be re-implemented. When two tiers share the
// ladder's top bar, ties break to the tier name that sorts first, so a
// ladder with complex and critical configured equal reports complex.
func (s *Selector) TierOf(role Role, prior float64) (Tier, bool) {
	t := s.metTierFor(role, prior, prior > 0, s.strictest(role))

	return t, t != ""
}

// strictest is the tier with the highest bar for role.
func (s *Selector) strictest(role Role) Tier {
	bars := s.bars(role)

	var top Tier

	for t, b := range bars {
		if top == "" || b > bars[top] || (b == bars[top] && t < top) {
			top = t
		}
	}

	return top
}

// Vendor is the model's vendor as the diversity preference sees it: the
// CM-supplied creator when known, else the slug prefix; "" when neither resolves.
func (s *Selector) Vendor(id string) string {
	return s.vendorOf(id)
}

// vendorOf resolves a model's vendor: the CM-supplied creator first, else the
// namespace prefix of a namespaced slug (OpenRouter-leg fallback for CMs that
// predate CandidateModel.Creator), else "". The two vocabularies (AA creator
// slugs like "zai" vs OR prefixes like "z-ai") never mix within one run: the
// fallback only fires when CM sent no creator for the slug.
func (s *Selector) vendorOf(id string) string {
	if m, ok := s.find(id); ok && m.creator != "" {
		return m.creator
	}

	if vendor, _, ok := strings.Cut(id, "/"); ok && vendor != "" {
		return vendor
	}

	return ""
}

// fitsWindow reports whether model's context window can hold estTokens. A
// model with no known window is treated as fitting (fail-open): the caller
// still enforces its own context limit at run time.
func (s *Selector) fitsWindow(id string, estTokens int) bool {
	m, ok := s.find(id)
	if !ok {
		return true
	}

	return m.window >= estTokens
}

func (s *Selector) bars(role Role) map[Tier]float64 { return s.ladders.Bars(role) }

// barFor returns the quality bar for t. A tier absent from the configured
// ladder has bar 0, so it can never gate a candidate out and descent always
// treats it as reachable. The zero-value Selector still works because bars()
// falls back to DefaultTierBars.
func (s *Selector) barFor(role Role, t Tier) float64 { return s.bars(role)[t] }

// lowestBar is the bottom of the configured ladder: the floor a walk can ever
// reach before falling to the capable default.
func (s *Selector) lowestBar(role Role) float64 {
	return slices.Min(slices.Collect(maps.Values(s.bars(role))))
}

// descent lists the rungs a request walks: the requested tier, then every
// configured tier with a STRICTLY LOWER bar, highest bar first. It never
// walks up. Derived by sorting the bar table, so there is no second
// ordering to keep in sync and an operator ladder with different rungs
// walks correctly. Ties on bar break by tier name for determinism; note
// that two tiers configured to the same bar collapse into one rung.
func (s *Selector) descent(role Role, requested Tier) []Tier {
	bars := s.bars(role)
	want := bars[requested]

	rungs := make([]Tier, 0, len(bars))

	for t, b := range bars {
		if t != requested && b < want {
			rungs = append(rungs, t)
		}
	}

	slices.SortFunc(rungs, func(a, b Tier) int {
		if c := cmp.Compare(bars[b], bars[a]); c != 0 {
			return c
		}

		return cmp.Compare(a, b)
	})

	return append([]Tier{requested}, rungs...)
}

// metTierFor is the strictest configured tier at or below requested whose
// bar prior clears; "" when none does or the model has no prior. Every Pick
// gets its MetTier from here, so a pin and a walked-down auto pick are
// reported on the same scale and an aggregate over MetTier is meaningful.
func (s *Selector) metTierFor(role Role, prior float64, hasPrior bool, requested Tier) Tier {
	if !hasPrior {
		return ""
	}

	bars := s.bars(role)

	for _, rung := range s.descent(role, requested) {
		if prior >= bars[rung] {
			return rung
		}
	}

	return ""
}

// pickFor assembles a Pick for a chosen model, measuring MetTier from the
// model's own prior rather than from the rung it was found on.
func (s *Selector) pickFor(id string, in SelectInput, src PickSource) Pick {
	found, _ := s.find(id)
	prior, has := found.prior(in.Role)
	met := s.metTierFor(in.Role, prior, has, in.Tier)

	return Pick{
		ModelSpec:     s.specFor(id),
		Role:          in.Role,
		RequestedTier: in.Tier,
		MetTier:       met,
		RequestedBar:  s.barFor(in.Role, in.Tier),
		Prior:         prior,
		HasPrior:      has,
		Source:        src,
		OK:            true,
		LowestBar:     s.lowestBar(in.Role),
	}
}

// SelectByComplexity picks the best-value model for (role, tier). A
// candidate must not be excluded, not blacklisted, carry a normalized prior
// for the role clearing the tier bar, and fit the window estimate. An
// eligible operator favorite for the rung wins outright; otherwise the most
// capable candidate within PriceHeadroom of the cheapest wins, quality ties
// breaking to the cheaper model.
//
// When the requested tier's pool is empty the selection CLAMPS DOWN the
// configured ladder: it re-runs at the next tier with a strictly lower bar,
// and so on. Each rung is exactly the selection a direct request at that
// rung would have made, so escalating a tier can never yield a worse model
// than asking for less. Favorites and pins are declared exceptions to that
// invariant: both are operator intent, and operator intent outranks a rule
// the selector derived, so a favorite bypassing the price band may out-quality
// a higher tier's automatic pick.
//
// Below the lowest configured bar the answer is the operator's capable
// default, marked SourceDefault and subject to the SAME hard filters as any
// candidate (Exclude, blacklist, ExcludeVendors, window fit). Hard-filtering
// the default is what keeps a model this run has already proven unusable
// from being handed back indefinitely: it answers "what did the operator
// choose for this run?", not "what is selectable regardless of what has
// already failed?".
//
// OK is false only when even that default is excluded, blacklisted, or
// window-short. The caller decides what a refusal costs.
func (s *Selector) SelectByComplexity(in SelectInput) Pick {
	pick, _ := s.SelectByComplexityReport(in)

	return pick
}

// SelectByComplexityReport is SelectByComplexity with the competing pool:
// the pick the plain selector returns, plus a report classifying every
// candidate at the rung the pick landed on. Pins and off-ladder picks
// synthesized by callers never go through the selector and get no report.
func (s *Selector) SelectByComplexityReport(in SelectInput) (Pick, SelectionReport) {
	for _, rung := range s.descent(in.Role, in.Tier) {
		at := in
		at.Tier = rung

		res := s.selectAtRung(at)
		if res.winner == "" {
			continue
		}

		pool := make([]PoolEntry, len(res.pool))
		for i, c := range res.pool {
			pool[i] = PoolEntry{Model: c.id, Prior: c.quality, Price: c.price, Outcome: res.outcomes[i]}
		}

		return s.pickFor(res.winner, in, res.source),
			SelectionReport{Rung: rung, Bar: s.barFor(in.Role, rung), Pool: pool, FilteredOut: res.filtered}
	}

	if s.capable != "" && s.employable(s.capable, in) {
		return s.pickFor(s.capable, in, SourceDefault), SelectionReport{}
	}

	return Pick{
			Role: in.Role, RequestedTier: in.Tier,
			RequestedBar: s.barFor(in.Role, in.Tier), LowestBar: s.lowestBar(in.Role),
		},
		SelectionReport{}
}

// selectAtRung makes one rung's selection. An empty winner means the rung
// is dry. The pool is the pool the pick was actually made from: the
// vendor-blind classification when a favorite fired (explicit operator
// intent beats the vendor heuristic, so the vendor filter does not apply to
// it), the vendor-filtered one otherwise.
func (s *Selector) selectAtRung(in SelectInput) rungResult {
	if !s.maxCapability {
		// Favorites bypass the vendor-diversity preference: explicit
		// operator intent beats the emergent heuristic. Evaluated per rung,
		// so a clamped pick consults the favorites of the tier it actually
		// landed on. The favorite lookup runs vendor-blind and before the
		// vendor-filtered candidate pool, so a rung counts as non-dry when
		// an eligible favorite exists even if the vendor-filtered pool is
		// empty - a soft vendor preference must never silently override an
		// explicit favorite.
		blind := in
		blind.ExcludeVendors = nil

		pool, filtered := s.classify(blind)
		if fav := s.favoriteAmong(pool, in.Tier, in.Role); fav != "" {
			// A favorite bypasses the price band, so it is marked selected
			// wherever it sits; the band classification still shows the
			// remaining candidates what the automatic rule would have done.
			band := priceBand(pool, s.headroomOrDefault(), false)
			_, bandWinner := valuePick(pool, band)
			outcomes := classifyPool(pool, band, bandWinner)

			favIdx := slices.IndexFunc(pool, func(c candidate) bool { return c.id == fav })
			outcomes[favIdx] = PoolSelected

			if bandWinner != favIdx && bandWinner >= 0 {
				outcomes[bandWinner] = PoolInBand
			}

			return rungResult{winner: fav, source: SourceFavorite, pool: pool, outcomes: outcomes, filtered: filtered}
		}
	}

	pool, filtered := s.classify(in)
	if len(pool) == 0 {
		return rungResult{filtered: filtered}
	}

	band := priceBand(pool, s.headroomOrDefault(), s.maxCapability)
	best, bandWinner := valuePick(pool, band)

	return rungResult{
		winner:   best.id,
		source:   SourceAuto,
		pool:     pool,
		outcomes: classifyPool(pool, band, bandWinner),
		filtered: filtered,
	}
}

// employable applies the hard filters - and only the hard filters - to a
// model that is not being judged on quality. The capable default answers a
// question the bars cannot ("what did the operator choose for this run?"),
// but it must never resurrect a model this run has already barred.
func (s *Selector) employable(id string, in SelectInput) bool {
	if in.Exclude[id] || s.blacklist[id] {
		return false
	}

	if len(in.ExcludeVendors) > 0 {
		if v := s.vendorOf(id); v != "" && in.ExcludeVendors[v] {
			return false
		}
	}

	return in.EstTokens <= 0 || s.fitsWindow(id, in.EstTokens)
}

// headroomOrDefault guards the zero value a test-built Selector may carry;
// New never stores a headroom below 1.
func (s *Selector) headroomOrDefault() float64 {
	if s.headroom < 1 {
		return DefaultPriceHeadroom
	}

	return s.headroom
}

// priceBand returns the price ceiling the best-value rule admits: the
// cheapest candidate times the headroom, or unbounded with MaxCapability.
func priceBand(cands []candidate, headroom float64, maxCapability bool) float64 {
	cheapest := cands[0].price
	for _, c := range cands[1:] {
		if c.price < cheapest {
			cheapest = c.price
		}
	}

	if maxCapability {
		return math.Inf(1)
	}

	return cheapest * headroom
}

// valuePick applies the best-value rule to the in-band candidates: the
// highest quality inside the band, quality ties breaking to the cheaper
// model. It returns the winner and its index in cands; -1 when every
// candidate is out of band.
func valuePick(cands []candidate, band float64) (candidate, int) {
	best := candidate{}
	bestIdx := -1

	for i, c := range cands {
		if c.price > band {
			continue
		}

		switch {
		case bestIdx < 0:
			best, bestIdx = c, i
		case c.quality > best.quality:
			best, bestIdx = c, i
		case c.quality == best.quality && c.price < best.price:
			best, bestIdx = c, i
		}
	}

	return best, bestIdx
}

// classifyPool pairs every candidate with its outcome under the band: the
// band winner is selected, the rest in band are in-band (lower quality, or
// a tie lost to a cheaper model), and anything priced above the band is
// out-of-band.
func classifyPool(cands []candidate, band float64, winnerIdx int) []PoolOutcome {
	outcomes := make([]PoolOutcome, len(cands))

	for i, c := range cands {
		switch {
		case i == winnerIdx:
			outcomes[i] = PoolSelected
		case c.price > band:
			outcomes[i] = PoolOutOfBand
		default:
			outcomes[i] = PoolInBand
		}
	}

	return outcomes
}

// candidates returns the models passing every filter for the given input.
// Quality is the normalized prior for the role; a model with no prior for the
// role, a prior below the tier bar, an exclusion, a blacklist entry, or a
// window that cannot hold the estimate is dropped.
func (s *Selector) candidates(in SelectInput) []candidate {
	pool, _ := s.classify(in)

	return pool
}

// hardFilterReason returns the hard filter that keeps m out of the pool, or
// "" when m passes the hard filters. The hard filters (run exclusions,
// blacklist, vendor exclusion) apply at every rung; the quality and window
// checks after them are per-rung and stay in classify. Models with no
// resolvable vendor are never vendor-filtered.
func (s *Selector) hardFilterReason(m model, in SelectInput) FilterReason {
	switch {
	case in.Exclude[m.id]:
		return FilterExcluded
	case s.blacklist[m.id]:
		return FilterBlacklisted
	case len(in.ExcludeVendors) > 0 && s.vendorOf(m.id) != "" && in.ExcludeVendors[s.vendorOf(m.id)]:
		return FilterVendorExcluded
	default:
		return ""
	}
}

// classify splits the candidate set for one rung's input into the pool that
// reached the rung (candidates in wire order) and the reason-aggregated
// summary of everything that did not. The filter reasons are evaluated in a
// fixed order per model and the first hit wins, so a model excluded AND
// blacklisted lands in exactly one bucket.
func (s *Selector) classify(in SelectInput) ([]candidate, []FilteredOutEntry) {
	bar := s.barFor(in.Role, in.Tier)

	buckets := make(map[FilterReason][]string, 6)

	var cands []candidate

	for _, m := range s.models {
		hard := s.hardFilterReason(m, in)
		if hard != "" {
			buckets[hard] = append(buckets[hard], m.id)

			continue
		}

		quality, ok := m.prior(in.Role)
		if !ok {
			buckets[FilterNoPrior] = append(buckets[FilterNoPrior], m.id)

			continue
		}

		if quality < bar {
			buckets[FilterPriorBelowBar] = append(buckets[FilterPriorBelowBar], m.id)

			continue
		}

		if in.EstTokens > 0 && !s.fitsWindow(m.id, in.EstTokens) {
			buckets[FilterWindowTooSmall] = append(buckets[FilterWindowTooSmall], m.id)

			continue
		}

		cands = append(cands, candidate{id: m.id, quality: quality, price: m.price()})
	}

	if len(buckets) == 0 {
		return cands, nil
	}

	// Fixed emission order for stable reports, matching the per-model check
	// order above.
	order := []FilterReason{
		FilterExcluded, FilterBlacklisted, FilterVendorExcluded,
		FilterNoPrior, FilterPriorBelowBar, FilterWindowTooSmall,
	}

	filtered := make([]FilteredOutEntry, 0, len(buckets))
	for _, reason := range order {
		if models := buckets[reason]; len(models) > 0 {
			filtered = append(filtered, FilteredOutEntry{Reason: reason, Models: models})
		}
	}

	return cands, filtered
}

// favoriteAmong returns the first operator favorite for (tier, role), then
// (tier, any role), present in cands. cands is the caller's already-filtered
// pool for the rung the pick is being made on, so eligibility is enforced by
// construction and a favorite is only ever consulted at its own tier.
// Favorites bypass the price band (see SelectByComplexity).
func (s *Selector) favoriteAmong(cands []candidate, tier Tier, role Role) string {
	if len(s.favorites) == 0 || len(cands) == 0 {
		return ""
	}

	eligible := make(map[string]bool, len(cands))
	for _, c := range cands {
		eligible[c.id] = true
	}

	for _, key := range []favKey{{Tier: tier, Role: role}, {Tier: tier}} {
		for _, slug := range s.favorites[key] {
			if eligible[slug] {
				return slug
			}
		}
	}

	return ""
}

// specFor builds a ModelSpec for id, filling the context window when the
// candidate set carries one.
func (s *Selector) specFor(id string) ModelSpec {
	spec := ModelSpec{Model: id}
	if m, ok := s.find(id); ok {
		spec.ContextWindow = m.window
	}

	return spec
}
