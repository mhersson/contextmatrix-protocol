package selection

import (
	"testing"

	protocol "github.com/mhersson/contextmatrix-protocol"
)

func TestNewBuildsCatalogPriorsAndFavorites(t *testing.T) {
	s := New(Input{
		Candidates: []protocol.CandidateModel{{
			Slug: "z-ai/glm-5.2", PromptPricePerTok: 1.2e-6, CompletionPricePerTok: 4.1e-6,
			ContextWindow: 1048576, CoderPrior: 0.90, ReviewerPrior: 0.85,
		}},
		// No Role: the favorite applies to every role, unlike
		// TestNewBuildsFromWire's role-scoped favorite.
		Favorites: []protocol.FavoriteRule{{Tier: "complex", Models: []string{"z-ai/glm-5.2"}}},
		Blacklist: []string{"bad/model"},
	})

	got := s.SelectByComplexity(SelectInput{Role: RoleCoder, Tier: TierComplex})
	eq(t, "z-ai/glm-5.2", got.Model)

	truthy(t, s.blacklist["bad/model"], "blacklist not applied")
}

func TestNewNilReturnsCapableDefault(t *testing.T) {
	s := New(Input{Capable: "capable/default"})

	got := s.SelectByComplexity(SelectInput{Role: RoleCoder, Tier: TierComplex})
	eq(t, "capable/default", got.Model, "an empty candidate set must yield the capable default")
}

func TestNewThreadsPriceHeadroom(t *testing.T) {
	// premium is higher quality but priced >1.5x and <3x the cheapest, so the
	// applied headroom decides the winner: 1.5 -> cheap wins; 3.0 -> premium wins.
	candidates := []protocol.CandidateModel{
		{Slug: "cheap/model", PromptPricePerTok: 1, CompletionPricePerTok: 1, ContextWindow: 200000, CoderPrior: 0.80, ReviewerPrior: 0.80},
		{Slug: "premium/model", PromptPricePerTok: 2, CompletionPricePerTok: 2.5, ContextWindow: 200000, CoderPrior: 0.95, ReviewerPrior: 0.95},
	}
	in := SelectInput{Role: RoleCoder, Tier: TierModerate}

	sDefault := New(Input{Candidates: candidates, Capable: "capable/default"}) // 0 -> worker default (1.5)
	eq(t, "cheap/model", sDefault.SelectByComplexity(in).Model)

	sWide := New(Input{Candidates: candidates, Capable: "capable/default", PriceHeadroom: 3.0})
	eq(t, "premium/model", sWide.SelectByComplexity(in).Model,
		"a non-default headroom must widen the best-value band")
}

func TestNewThreadsMaxCapability(t *testing.T) {
	// With the default headroom (1.5x) cheap wins. With MaxCapability=true,
	// the expensive high-quality model wins regardless of price.
	candidates := []protocol.CandidateModel{
		{Slug: "cheap/model", PromptPricePerTok: 1, CompletionPricePerTok: 1, ContextWindow: 200000, CoderPrior: 0.80, ReviewerPrior: 0.80},
		{Slug: "premium/model", PromptPricePerTok: 2, CompletionPricePerTok: 2.5, ContextWindow: 200000, CoderPrior: 0.95, ReviewerPrior: 0.95},
	}
	in := SelectInput{Role: RoleCoder, Tier: TierModerate}

	sDefault := New(Input{Candidates: candidates, Capable: "capable/default"})
	eq(t, "cheap/model", sDefault.SelectByComplexity(in).Model, "default must pick the cheaper model")

	sMax := New(Input{Candidates: candidates, Capable: "capable/default", MaxCapability: true})
	eq(t, "premium/model", sMax.SelectByComplexity(in).Model,
		"MaxCapability=true must pick the premium (more capable) model regardless of price")
}

func TestNewThreadsCreators(t *testing.T) {
	// The incident scenario end-to-end: an OpenAI-endpoint payload (bare
	// slugs, creators supplied by CM) must come out of New with the
	// vendor-diversity preference live in the discussion panel.
	candidates := []protocol.CandidateModel{
		{Slug: "gpt-a", PromptPricePerTok: 1e-6, CompletionPricePerTok: 2e-6, ContextWindow: 200000, ReviewerPrior: 0.95, Creator: "openai"},
		{Slug: "gpt-b", PromptPricePerTok: 1e-6, CompletionPricePerTok: 2e-6, ContextWindow: 200000, ReviewerPrior: 0.90, Creator: "openai"},
		{Slug: "gpt-c", PromptPricePerTok: 1e-6, CompletionPricePerTok: 2e-6, ContextWindow: 200000, ReviewerPrior: 0.88, Creator: "openai"},
		{Slug: "claude-x", PromptPricePerTok: 1e-6, CompletionPricePerTok: 2e-6, ContextWindow: 200000, ReviewerPrior: 0.85, Creator: "anthropic"},
	}
	s := New(Input{Candidates: candidates, Capable: "capable-default"})

	panel := SeatPicks(s.SelectDiscussionPanelReport(SelectInput{Role: RoleReviewer, Tier: TierComplex, EstTokens: 50000}, 3))
	eq(t, 3, len(panel))
	eq(t, "gpt-a", panel[0].Model)
	eq(t, "claude-x", panel[1].Model, "creators from the payload must drive vendor diversity")
	eq(t, "gpt-b", panel[2].Model)

	// Without creators (older CM, bare slugs) the walk stays vendor-blind.
	for i := range candidates {
		candidates[i].Creator = ""
	}

	sBlind := New(Input{Candidates: candidates, Capable: "capable-default"})

	panel = SeatPicks(sBlind.SelectDiscussionPanelReport(SelectInput{Role: RoleReviewer, Tier: TierComplex, EstTokens: 50000}, 3))
	eq(t, 3, len(panel))
	eq(t, "gpt-a", panel[0].Model)
	eq(t, "gpt-b", panel[1].Model)
	eq(t, "gpt-c", panel[2].Model)
}

func TestNewAppliesPerRoleLadders(t *testing.T) {
	s := New(Input{
		Candidates: []protocol.CandidateModel{
			{Slug: "z-ai/glm-5.3", PromptPricePerTok: 2.9e-6, CompletionPricePerTok: 2.9e-6, ContextWindow: 200000, CoderPrior: 0.917, ReviewerPrior: 0.841},
			{Slug: "openai/gpt-5.6-sol", PromptPricePerTok: 6e-6, CompletionPricePerTok: 6e-6, ContextWindow: 400000, CoderPrior: 0.949, ReviewerPrior: 0.882},
		},
		Ladders: Ladders{
			RoleCoder:    map[Tier]float64{TierSimple: 0.65, TierModerate: 0.80, TierComplex: 0.90, TierCritical: 0.95},
			RoleReviewer: map[Tier]float64{TierSimple: 0.65, TierModerate: 0.76, TierComplex: 0.82, TierCritical: 0.93},
		},
		Capable: "capable/default",
	})

	// The same model is complex as a coder (0.917 >= 0.90) and complex as a
	// reviewer (0.841 >= 0.82) under different bars.
	tier, ok := s.TierOf(RoleCoder, 0.917)
	truthy(t, ok)
	eq(t, TierComplex, tier)
	tier, ok = s.TierOf(RoleReviewer, 0.841)
	truthy(t, ok)
	eq(t, TierComplex, tier)

	// Below the coder critical bar but above the reviewer one.
	tier, _ = s.TierOf(RoleCoder, 0.94)
	eq(t, TierComplex, tier)
	tier, _ = s.TierOf(RoleReviewer, 0.94)
	eq(t, TierCritical, tier)

	eq(t, 0.90, s.BarFor(RoleCoder, TierComplex))
	eq(t, 0.82, s.BarFor(RoleReviewer, TierComplex))

	pick := s.SelectByComplexity(SelectInput{Role: RoleReviewer, Tier: TierComplex})
	eq(t, "z-ai/glm-5.3", pick.Model, "cheapest in band wins for reviewers")
	pick = s.SelectByComplexity(SelectInput{Role: RoleCoder, Tier: TierCritical})
	eq(t, "z-ai/glm-5.3", pick.Model, "nothing clears the raised coder critical bar, so the walk lands on complex")
	eq(t, TierComplex, pick.MetTier)
	truthy(t, pick.BelowBar())
}

func TestNewWithNoLaddersUsesTheDefaults(t *testing.T) {
	s := New(Input{Capable: "x"})
	eq(t, 0.82, s.BarFor(RoleCoder, TierComplex))
	eq(t, 0.82, s.BarFor(RoleReviewer, TierComplex))
	_, ok := s.TierOf(RoleCoder, 0.5)
	falsy(t, ok, "below every bar clears no tier")
}
