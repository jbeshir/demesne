package anthropic

import (
	"strings"
)

// USD represents US dollars (indicative).
type USD float64

// ModelID is an Anthropic API model identifier, e.g. "claude-sonnet-4-6",
// "claude-opus-4-8-20260101". Dated IDs resolve to their family via
// longest-prefix match in LookupPricing.
type ModelID string

// Pricing is the per-million-token rate (USD) for one Anthropic model
// family. Cache rates follow Anthropic's published multipliers:
// cache write = 1.25x base input, cache read = 0.1x base input.
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

// IMPORTANT: cost figures below are INDICATIVE ONLY. The 1-hour cache tier
// is NOT modelled (single 5-minute cache tier only). Unknown models return
// 0 cost so they will not break a run.
//
// modelCatalog is the single source of truth for per-family pricing.
// sonnet sits at index 0 to match DefaultModel. The three IDPrefixes have
// no overlap so longest-prefix ordering is moot, but the contract is
// maintained. Add new families here when they ship.
//
// Source: https://docs.anthropic.com/en/docs/about-claude/pricing
var modelCatalog = []catalogEntry{
	// sonnet — index 0 = DefaultModel; verified rates per MTok (in/out/write/read).
	{
		Alias:    "sonnet",
		IDPrefix: "claude-sonnet-4-6",
		Pricing: Pricing{
			InputPerMTok:      3.00,
			OutputPerMTok:     15.00,
			CacheWritePerMTok: 3.75,
			CacheReadPerMTok:  0.30,
		},
	},
	// opus — verified rates per MTok (in/out/write/read).
	{
		Alias:    "opus",
		IDPrefix: "claude-opus-4-8",
		Pricing: Pricing{
			InputPerMTok:      5.00,
			OutputPerMTok:     25.00,
			CacheWritePerMTok: 6.25,
			CacheReadPerMTok:  0.50,
		},
	},
	// haiku — verified rates per MTok (in/out/write/read).
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
	for _, e := range modelCatalog {
		if strings.HasPrefix(string(id), string(e.IDPrefix)) {
			return e.Pricing, true
		}
	}
	return Pricing{}, false
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
