package anthropic

import (
	"github.com/jbeshir/demesne/internal/proxies/proxycommon"
)

// USD represents US dollars (indicative).
type USD float64

// ModelID is an Anthropic API model identifier, e.g. "claude-sonnet-5-5",
// "claude-opus-5-5-20260101". Dated IDs resolve to their family via
// longest-prefix match in LookupPricing.
type ModelID string

// Pricing is the per-million-token rate (USD) for one Anthropic model
// family. Each field is the published per-family rate: the 5-minute cache
// write is 1.25x base input, but the cache-read multiplier varies by family
// (0.1x on most, 0.05x on Opus 5.5, 0.025x on Fable 5.1), so read rates are
// stored as published rather than derived from input.
type Pricing struct {
	InputPerMTok      USD
	OutputPerMTok     USD
	CacheWritePerMTok USD
	CacheReadPerMTok  USD
}

type catalogEntry struct {
	Alias    string
	IDPrefix ModelID
	Pricing
}

func (e catalogEntry) Prefix() string { return string(e.IDPrefix) }
func (e catalogEntry) Price() Pricing { return e.Pricing }

// IMPORTANT: cost figures below are INDICATIVE ONLY. The 1-hour cache tier
// is NOT modelled (single 5-minute cache tier only). Unknown models return
// 0 cost so they will not break a run.
//
// modelCatalog is the single source of truth for per-family pricing.
// sonnet sits at index 0 to match DefaultModel. The IDPrefixes have no
// overlap so longest-prefix ordering is moot, but the contract is
// maintained. Add new families here when they ship.
//
// Source: https://platform.claude.com/docs/en/about-claude/pricing (accessed 2026-10-04).
var modelCatalog = []catalogEntry{
	// sonnet — index 0 = DefaultModel; $2/$10/$2.50/$0.20 per MTok (in/out/write/read).
	{
		Alias:    "sonnet",
		IDPrefix: "claude-sonnet-5",
		Pricing: Pricing{
			InputPerMTok:      2.00,
			OutputPerMTok:     10.00,
			CacheWritePerMTok: 2.50,
			CacheReadPerMTok:  0.20,
		},
	},
	// opus — $4/$20/$5/$0.20 per MTok (in/out/write/read).
	{
		Alias:    "opus",
		IDPrefix: "claude-opus-5",
		Pricing: Pricing{
			InputPerMTok:      4.00,
			OutputPerMTok:     20.00,
			CacheWritePerMTok: 5.00,
			CacheReadPerMTok:  0.20,
		},
	},
	// fable — most capable tier, above opus; $10/$50/$12.50/$0.25 per MTok (in/out/write/read).
	{
		Alias:    "fable",
		IDPrefix: "claude-fable-5",
		Pricing: Pricing{
			InputPerMTok:      10.00,
			OutputPerMTok:     50.00,
			CacheWritePerMTok: 12.50,
			CacheReadPerMTok:  0.25,
		},
	},
	// haiku — $1/$5/$1.25/$0.10 per MTok (in/out/write/read).
	{
		Alias:    "haiku",
		IDPrefix: "claude-haiku-4-5",
		Pricing: Pricing{
			InputPerMTok:      1.00,
			OutputPerMTok:     5.00,
			CacheWritePerMTok: 1.25,
			CacheReadPerMTok:  0.10,
		},
	},
}

// Aliases returns the catalog's user-facing model aliases in catalog order.
// The default alias is index 0.
func Aliases() []string {
	out := make([]string, len(modelCatalog))
	for i, e := range modelCatalog {
		out[i] = e.Alias
	}
	return out
}

// LookupPricing returns the Pricing for the given Anthropic model ID by
// longest-prefix match, plus whether a match was found.
func LookupPricing(id ModelID) (Pricing, bool) {
	return proxycommon.LookupPricing[catalogEntry, Pricing](modelCatalog, string(id))
}

// TokenCounts is the per-request usage breakdown the Anthropic API
// reports back in the usage block of message_start / message_delta /
// non-streaming responses.
type TokenCounts struct {
	InputTokens              int64 `json:"input_tokens"`
	OutputTokens             int64 `json:"output_tokens"`
	CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
}

// CostUSD computes the USD cost for the given token counts at the
// model's pricing. Models with no pricing entry return 0 — that lets
// unknown models pass through without breaking the run, at the cost of
// not counting toward the cap. Add new families to modelCatalog when
// they ship.
func CostUSD(id ModelID, t TokenCounts) USD {
	p, ok := LookupPricing(id)
	if !ok {
		return 0
	}
	const perMTok = 1_000_000.0
	return USD(
		float64(t.InputTokens)/perMTok*float64(p.InputPerMTok) +
			float64(t.OutputTokens)/perMTok*float64(p.OutputPerMTok) +
			float64(t.CacheCreationInputTokens)/perMTok*float64(p.CacheWritePerMTok) +
			float64(t.CacheReadInputTokens)/perMTok*float64(p.CacheReadPerMTok),
	)
}
