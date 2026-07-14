package openai

import (
	"github.com/jbeshir/demesne/internal/proxies/proxycommon"
)

// USD represents US dollars (indicative).
type USD float64

// ModelID is an OpenAI Responses API model identifier, e.g. "gpt-5.6-sol",
// "gpt-5.5". Versioned or dated IDs resolve to their family via longest-prefix
// match in LookupPricing.
type ModelID string

// Pricing is the per-million-token rate (USD) for one OpenAI model
// family. CachedInputPerMTok covers cached prompt tokens which OpenAI
// bills at a reduced rate relative to full input tokens.
type Pricing struct {
	InputPerMTok       USD
	OutputPerMTok      USD
	CachedInputPerMTok USD
}

type catalogEntry struct {
	Alias    string
	IDPrefix ModelID
	Pricing
}

func (e catalogEntry) Prefix() string { return string(e.IDPrefix) }
func (e catalogEntry) Price() Pricing { return e.Pricing }

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

// InputTokenDetails is the observable Responses-API breakdown of prompt
// tokens. Cached reads are billed at the cheaper cached rate. OpenAI bills
// cache writes at 1.25x input, but Responses usage does not currently expose
// cache-write token counts, so they are not modeled in estimates.
type InputTokenDetails struct {
	CachedTokens int64 `json:"cached_tokens"`
}

// OutputTokenDetails is the Responses-API breakdown of completion tokens;
// reasoning tokens are reported for accounting but priced as output.
type OutputTokenDetails struct {
	ReasoningTokens int64 `json:"reasoning_tokens"`
}

// IMPORTANT: ChatGPT-OAuth billing is subscription-based, so the per-token
// costs below are INDICATIVE ONLY and do NOT reflect what the user is
// charged. They are useful for relative cost accounting between model
// families but must not be used for budget enforcement or billing.
// The >272K long-context surcharge tier is NOT modelled (single standard tier only).
// Unknown models return 0 cost so they will not break a run.
//
// modelCatalog is ordered longest-prefix-first so LookupPricing picks the
// most specific match. The current set has no ambiguous prefix overlap, but the
// contract is maintained for future additions.
// Add new families here (longest prefix first) when they ship.
var modelCatalog = []catalogEntry{
	// gpt-5.6-sol — verified observable rates: $5.00/$30.00/$0.50 per MTok (in/out/cached).
	{
		Alias:    "gpt-5.6-sol",
		IDPrefix: "gpt-5.6-sol",
		Pricing: Pricing{
			InputPerMTok:       5.00,
			OutputPerMTok:      30.0,
			CachedInputPerMTok: 0.50,
		},
	},
	// gpt-5.6-terra — verified observable rates: $2.50/$15.00/$0.25 per MTok (in/out/cached).
	{
		Alias:    "gpt-5.6-terra",
		IDPrefix: "gpt-5.6-terra",
		Pricing: Pricing{
			InputPerMTok:       2.50,
			OutputPerMTok:      15.0,
			CachedInputPerMTok: 0.25,
		},
	},
	// gpt-5.6-luna — verified observable rates: $1.00/$6.00/$0.10 per MTok (in/out/cached).
	{
		Alias:    "gpt-5.6-luna",
		IDPrefix: "gpt-5.6-luna",
		Pricing: Pricing{
			InputPerMTok:       1.00,
			OutputPerMTok:      6.0,
			CachedInputPerMTok: 0.10,
		},
	},
	// gpt-5.5 — verified observable rates: $5.00/$30.00/$0.50 per MTok (in/out/cached).
	{
		Alias:    "gpt-5.5",
		IDPrefix: "gpt-5.5",
		Pricing: Pricing{
			InputPerMTok:       5.00,
			OutputPerMTok:      30.0,
			CachedInputPerMTok: 0.50,
		},
	},
	// gpt-5.4-mini — verified observable rates: $0.75/$4.50/$0.075 per MTok (in/out/cached).
	{
		Alias:    "gpt-5.4-mini",
		IDPrefix: "gpt-5.4-mini",
		Pricing: Pricing{
			InputPerMTok:       0.75,
			OutputPerMTok:      4.50,
			CachedInputPerMTok: 0.075,
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

// LookupPricing returns the Pricing for the given OpenAI model ID by
// longest-prefix match, plus whether a match was found.
func LookupPricing(id ModelID) (Pricing, bool) {
	return proxycommon.LookupPricing[catalogEntry, Pricing](modelCatalog, string(id))
}

// CostUSD computes the USD cost for the given token counts at the
// model's pricing. Models with no pricing entry return 0 — that lets
// unknown models pass through without breaking the run, at the cost of
// not counting toward the cap. Add new families to modelCatalog when
// they ship.
//
// OpenAI's input_tokens is the TOTAL prompt token count;
// input_tokens_details.cached_tokens is the cached subset, billed at the
// cheaper CachedInputPerMTok rate. Responses usage does not currently expose
// cache-write token counts, so cost estimates use only observable input,
// cached-read, output, and reasoning counts. Cost formula:
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
