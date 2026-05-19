package anthropic

import (
	"sort"
	"strings"
)

// Pricing is the per-million-token rate (USD) for one Anthropic model
// family. Cache rates follow Anthropic's published multipliers:
// cache write = 1.25x base input, cache read = 0.1x base input.
type Pricing struct {
	InputPerMTok      float64
	OutputPerMTok     float64
	CacheWritePerMTok float64
	CacheReadPerMTok  float64
}

// modelPricing is the per-family pricing table. Lookups are by longest
// prefix match against the model ID Anthropic returns in API responses,
// so dated IDs like "claude-opus-4-7-20251201" resolve to the
// "claude-opus-4-7" entry.
//
// Source: https://docs.anthropic.com/en/docs/about-claude/pricing
// Update this when prices change — it's the single point of truth.
var modelPricing = map[string]Pricing{
	"claude-opus-4-7": {
		InputPerMTok:      15.0,
		OutputPerMTok:     75.0,
		CacheWritePerMTok: 18.75,
		CacheReadPerMTok:  1.5,
	},
	"claude-opus-4": {
		InputPerMTok:      15.0,
		OutputPerMTok:     75.0,
		CacheWritePerMTok: 18.75,
		CacheReadPerMTok:  1.5,
	},
	"claude-sonnet-4-6": {
		InputPerMTok:      3.0,
		OutputPerMTok:     15.0,
		CacheWritePerMTok: 3.75,
		CacheReadPerMTok:  0.3,
	},
	"claude-sonnet-4": {
		InputPerMTok:      3.0,
		OutputPerMTok:     15.0,
		CacheWritePerMTok: 3.75,
		CacheReadPerMTok:  0.3,
	},
	"claude-haiku-4-5": {
		InputPerMTok:      0.8,
		OutputPerMTok:     4.0,
		CacheWritePerMTok: 1.0,
		CacheReadPerMTok:  0.08,
	},
	"claude-haiku-4": {
		InputPerMTok:      0.8,
		OutputPerMTok:     4.0,
		CacheWritePerMTok: 1.0,
		CacheReadPerMTok:  0.08,
	},
}

// pricingKeysByLength is the prefix-match search order (longest first).
// Computed once at package init so lookups are O(n) over a tiny n.
var pricingKeysByLength = func() []string {
	keys := make([]string, 0, len(modelPricing))
	for k := range modelPricing {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return len(keys[i]) > len(keys[j]) })
	return keys
}()

// LookupPricing returns the Pricing for the given Anthropic model ID by
// longest-prefix match, plus whether a match was found.
func LookupPricing(modelID string) (Pricing, bool) {
	for _, k := range pricingKeysByLength {
		if strings.HasPrefix(modelID, k) {
			return modelPricing[k], true
		}
	}
	return Pricing{}, false
}

// TokenCounts is the per-request usage breakdown the Anthropic API
// reports back in the usage block of message_start / message_delta /
// non-streaming responses.
type TokenCounts struct {
	InputTokens              int64
	OutputTokens             int64
	CacheCreationInputTokens int64
	CacheReadInputTokens     int64
}

// CostUSD computes the USD cost for the given token counts at the
// model's pricing. Models with no pricing entry return 0 — that lets
// unknown models pass through without breaking the run, at the cost of
// not counting toward the cap. Add new families to modelPricing when
// they ship.
func CostUSD(modelID string, t TokenCounts) float64 {
	p, ok := LookupPricing(modelID)
	if !ok {
		return 0
	}
	const perMTok = 1_000_000.0
	return float64(t.InputTokens)/perMTok*p.InputPerMTok +
		float64(t.OutputTokens)/perMTok*p.OutputPerMTok +
		float64(t.CacheCreationInputTokens)/perMTok*p.CacheWritePerMTok +
		float64(t.CacheReadInputTokens)/perMTok*p.CacheReadPerMTok
}
