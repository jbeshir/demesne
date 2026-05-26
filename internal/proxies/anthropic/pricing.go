package anthropic

import (
	"strings"
)

// USD represents US dollars (indicative).
type USD float64

// ModelID is an Anthropic API model identifier, e.g. "claude-sonnet-4-6",
// "claude-opus-4-7-20251201". Dated IDs resolve to their family via
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

type pricingEntry struct {
	prefix ModelID
	Pricing
}

const claudeSonnet46 = "claude-sonnet-4-6"

// modelPricingTable is the single source of truth for per-family pricing,
// ordered longest-prefix-first so LookupPricing picks the most specific
// match without a secondary sort. Add new model families here when they ship.
//
// Source: https://docs.anthropic.com/en/docs/about-claude/pricing
var modelPricingTable = []pricingEntry{
	{"claude-opus-4-7", Pricing{InputPerMTok: 15.0, OutputPerMTok: 75.0, CacheWritePerMTok: 18.75, CacheReadPerMTok: 1.5}},
	{"claude-opus-4", Pricing{InputPerMTok: 15.0, OutputPerMTok: 75.0, CacheWritePerMTok: 18.75, CacheReadPerMTok: 1.5}},
	{claudeSonnet46, Pricing{InputPerMTok: 3.0, OutputPerMTok: 15.0, CacheWritePerMTok: 3.75, CacheReadPerMTok: 0.3}},
	{"claude-sonnet-4", Pricing{InputPerMTok: 3.0, OutputPerMTok: 15.0, CacheWritePerMTok: 3.75, CacheReadPerMTok: 0.3}},
	{"claude-haiku-4-5", Pricing{InputPerMTok: 0.8, OutputPerMTok: 4.0, CacheWritePerMTok: 1.0, CacheReadPerMTok: 0.08}},
	{"claude-haiku-4", Pricing{InputPerMTok: 0.8, OutputPerMTok: 4.0, CacheWritePerMTok: 1.0, CacheReadPerMTok: 0.08}},
}

// LookupPricing returns the Pricing for the given Anthropic model ID by
// longest-prefix match, plus whether a match was found.
func LookupPricing(id ModelID) (Pricing, bool) {
	for _, e := range modelPricingTable {
		if strings.HasPrefix(string(id), string(e.prefix)) {
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
// not counting toward the cap. Add new families to modelPricingTable when
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
