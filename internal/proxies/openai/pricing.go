package openai

import (
	"strings"
)

// USD represents US dollars (indicative).
type USD float64

// ModelID is an OpenAI Responses API model identifier, e.g. "gpt-5.5",
// "gpt-5.4-mini". Versioned or dated IDs resolve to their family via
// longest-prefix match in LookupPricing.
type ModelID string

// Pricing is the per-million-token rate (USD) for one OpenAI model
// family. CachedInputPerMTok covers cached prompt tokens which OpenAI
// bills at a reduced rate relative to full input tokens.
type Pricing struct {
	InputPerMTok       USD
	OutputPerMTok      USD
	CachedInputPerMTok USD
}

type pricingEntry struct {
	prefix ModelID
	Pricing
}

// TokenCounts is the per-request usage breakdown the OpenAI Responses
// API reports in the usage block of response.completed events and
// non-streaming responses. The nested detail structs mirror the API JSON
// shape exactly.
type TokenCounts struct {
	InputTokens         int64              `json:"input_tokens"`
	OutputTokens        int64              `json:"output_tokens"`
	TotalTokens         int64              `json:"total_tokens"`
	InputTokensDetails  InputTokenDetails  `json:"input_tokens_details"`
	OutputTokensDetails OutputTokenDetails `json:"output_tokens_details"`
}

// InputTokenDetails is the Responses-API breakdown of prompt tokens; the
// cached subset is billed at the cheaper cached rate.
type InputTokenDetails struct {
	CachedTokens int64 `json:"cached_tokens"`
}

// OutputTokenDetails is the Responses-API breakdown of completion tokens;
// reasoning tokens are reported for accounting but priced as output.
type OutputTokenDetails struct {
	ReasoningTokens int64 `json:"reasoning_tokens"`
}

const gpt55 = "gpt-5.5"

// IMPORTANT: The pricing rates below are UNVERIFIED PLACEHOLDER ESTIMATES.
// Real OpenAI pricing for the gpt-5.x model families must be confirmed at
// https://openai.com/pricing before any cost accounting or budget
// enforcement relies on these figures. The values here are indicative
// only and are clearly labelled as guesses. Unknown models return 0 cost
// so they will not break a run — they simply won't count toward any cost cap.
//
// modelPricingTable is ordered longest-prefix-first so LookupPricing
// picks the most specific match without a secondary sort. "gpt-5.4-mini"
// must appear before "gpt-5.4", and "gpt-5.3-codex-spark" before
// "gpt-5.3-codex". Add new families here when they ship.
var modelPricingTable = []pricingEntry{
	// gpt-5.5 — placeholder estimate: $1.25/$10.00/$0.125 per MTok (in/out/cached)
	{gpt55, Pricing{InputPerMTok: 1.25, OutputPerMTok: 10.0, CachedInputPerMTok: 0.125}},
	// gpt-5.4-mini — placeholder estimate (cheaper mini variant)
	{"gpt-5.4-mini", Pricing{InputPerMTok: 0.30, OutputPerMTok: 1.20, CachedInputPerMTok: 0.030}},
	// gpt-5.4 — placeholder estimate
	{"gpt-5.4", Pricing{InputPerMTok: 1.25, OutputPerMTok: 10.0, CachedInputPerMTok: 0.125}},
	// gpt-5.3-codex-spark — placeholder estimate (lighter codex variant)
	{"gpt-5.3-codex-spark", Pricing{InputPerMTok: 0.50, OutputPerMTok: 2.0, CachedInputPerMTok: 0.050}},
	// gpt-5.3-codex — placeholder estimate
	{"gpt-5.3-codex", Pricing{InputPerMTok: 1.25, OutputPerMTok: 10.0, CachedInputPerMTok: 0.125}},
	// gpt-5.2 — placeholder estimate
	{"gpt-5.2", Pricing{InputPerMTok: 1.25, OutputPerMTok: 10.0, CachedInputPerMTok: 0.125}},
}

// LookupPricing returns the Pricing for the given OpenAI model ID by
// longest-prefix match, plus whether a match was found.
func LookupPricing(id ModelID) (Pricing, bool) {
	for _, e := range modelPricingTable {
		if strings.HasPrefix(string(id), string(e.prefix)) {
			return e.Pricing, true
		}
	}
	return Pricing{}, false
}

// CostUSD computes the USD cost for the given token counts at the
// model's pricing. Models with no pricing entry return 0 — that lets
// unknown models pass through without breaking the run, at the cost of
// not counting toward the cap. Add new families to modelPricingTable when
// they ship.
//
// OpenAI's input_tokens is the TOTAL prompt token count;
// input_tokens_details.cached_tokens is the cached subset, billed at the
// cheaper CachedInputPerMTok rate. Cost formula:
//
//	(input - cached) * inputRate + cached * cachedRate + output * outputRate
func CostUSD(id ModelID, t TokenCounts) USD {
	p, ok := LookupPricing(id)
	if !ok {
		return 0
	}
	const perMTok = 1_000_000.0
	cached := t.InputTokensDetails.CachedTokens
	return USD(
		float64(t.InputTokens-cached)/perMTok*float64(p.InputPerMTok) +
			float64(cached)/perMTok*float64(p.CachedInputPerMTok) +
			float64(t.OutputTokens)/perMTok*float64(p.OutputPerMTok),
	)
}
