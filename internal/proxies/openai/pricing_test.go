package openai

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLookupPricing_GPT61Sol_ExactPrices(t *testing.T) {
	p, ok := LookupPricing("gpt-6.1-sol")
	require.True(t, ok)
	assert.InDelta(t, 2.0, float64(p.InputPerMTok), 1e-9)
	assert.InDelta(t, 0.10, float64(p.CachedInputPerMTok), 1e-9)
	assert.InDelta(t, 10.0, float64(p.OutputPerMTok), 1e-9)
}

func TestLookupPricing_GPT6Sol_ExactPrices(t *testing.T) {
	p, ok := LookupPricing("gpt-6-sol")
	require.True(t, ok)
	assert.InDelta(t, 2.0, float64(p.InputPerMTok), 1e-9)
	assert.InDelta(t, 0.20, float64(p.CachedInputPerMTok), 1e-9)
	assert.InDelta(t, 10.0, float64(p.OutputPerMTok), 1e-9)
}

func TestLookupPricing_GPT6Astra_ExactPrices(t *testing.T) {
	p, ok := LookupPricing("gpt-6-astra")
	require.True(t, ok)
	assert.InDelta(t, 10.0, float64(p.InputPerMTok), 1e-9)
	assert.InDelta(t, 1.0, float64(p.CachedInputPerMTok), 1e-9)
	assert.InDelta(t, 50.0, float64(p.OutputPerMTok), 1e-9)
}

func TestLookupPricing_GPT6Luna_ExactPrices(t *testing.T) {
	p, ok := LookupPricing("gpt-6-luna")
	require.True(t, ok)
	assert.InDelta(t, 0.10, float64(p.InputPerMTok), 1e-9)
	assert.InDelta(t, 0.01, float64(p.CachedInputPerMTok), 1e-9)
	assert.InDelta(t, 0.50, float64(p.OutputPerMTok), 1e-9)
}

func TestLookupPricing_GPT56Sol_ExactPrices(t *testing.T) {
	p, ok := LookupPricing("gpt-5.6-sol")
	require.True(t, ok)
	assert.InDelta(t, 4.0, float64(p.InputPerMTok), 1e-9)
	assert.InDelta(t, 0.40, float64(p.CachedInputPerMTok), 1e-9)
	assert.InDelta(t, 20.0, float64(p.OutputPerMTok), 1e-9)
}

func TestLookupPricing_GPT56Terra_ExactPrices(t *testing.T) {
	p, ok := LookupPricing("gpt-5.6-terra")
	require.True(t, ok)
	assert.InDelta(t, 2.0, float64(p.InputPerMTok), 1e-9)
	assert.InDelta(t, 0.20, float64(p.CachedInputPerMTok), 1e-9)
	assert.InDelta(t, 12.0, float64(p.OutputPerMTok), 1e-9)
}

func TestLookupPricing_GPT56Luna_ExactPrices(t *testing.T) {
	p, ok := LookupPricing("gpt-5.6-luna")
	require.True(t, ok)
	assert.InDelta(t, 0.20, float64(p.InputPerMTok), 1e-9)
	assert.InDelta(t, 0.02, float64(p.CachedInputPerMTok), 1e-9)
	assert.InDelta(t, 1.20, float64(p.OutputPerMTok), 1e-9)
}

func TestLookupPricing_GPT55_ExactPrices(t *testing.T) {
	p, ok := LookupPricing("gpt-5.5")
	require.True(t, ok)
	assert.InDelta(t, 5.0, float64(p.InputPerMTok), 1e-9)
	assert.InDelta(t, 0.5, float64(p.CachedInputPerMTok), 1e-9)
	assert.InDelta(t, 30.0, float64(p.OutputPerMTok), 1e-9)
}

func TestLookupPricing_PrefixMatchVersionedID(t *testing.T) {
	p61, ok := LookupPricing("gpt-6.1-sol-2026-09-29")
	require.True(t, ok)
	assert.InDelta(t, 2.0, float64(p61.InputPerMTok), 1e-9)
	assert.InDelta(t, 0.10, float64(p61.CachedInputPerMTok), 1e-9)
	assert.InDelta(t, 10.0, float64(p61.OutputPerMTok), 1e-9)

	// gpt-6-sol-2026-09-01 must resolve to the gpt-6-sol entry, not gpt-6.1-sol.
	p6, ok := LookupPricing("gpt-6-sol-2026-09-01")
	require.True(t, ok)
	assert.InDelta(t, 2.0, float64(p6.InputPerMTok), 1e-9)
	assert.InDelta(t, 0.20, float64(p6.CachedInputPerMTok), 1e-9)
	assert.InDelta(t, 10.0, float64(p6.OutputPerMTok), 1e-9)

	p6astra, ok := LookupPricing("gpt-6-astra-2026-09-01")
	require.True(t, ok)
	assert.InDelta(t, 10.0, float64(p6astra.InputPerMTok), 1e-9)
	assert.InDelta(t, 50.0, float64(p6astra.OutputPerMTok), 1e-9)

	p6luna, ok := LookupPricing("gpt-6-luna-2026-09-01")
	require.True(t, ok)
	assert.InDelta(t, 0.10, float64(p6luna.InputPerMTok), 1e-9)
	assert.InDelta(t, 0.50, float64(p6luna.OutputPerMTok), 1e-9)

	p56, ok := LookupPricing("gpt-5.6-sol-2026-07-13")
	require.True(t, ok)
	assert.InDelta(t, 4.0, float64(p56.InputPerMTok), 1e-9)
	assert.InDelta(t, 20.0, float64(p56.OutputPerMTok), 1e-9)

	// gpt-5.5-2026-01-01 must resolve to the gpt-5.5 entry.
	p55, ok := LookupPricing("gpt-5.5-2026-01-01")
	require.True(t, ok)
	assert.InDelta(t, 5.0, float64(p55.InputPerMTok), 1e-9)
	assert.InDelta(t, 30.0, float64(p55.OutputPerMTok), 1e-9)
}

func TestLookupPricing_UnpricedIDs(t *testing.T) {
	for _, id := range []ModelID{"gpt-5.4", "gpt-5.4-mini", "gpt-5.3-codex", "gpt-5.2", "claude-sonnet-4-6"} {
		_, ok := LookupPricing(id)
		assert.False(t, ok, "expected no match for %q", id)
	}
}

func TestCostUSD_GPT55_Math(t *testing.T) {
	// 1M input (0 cached) + 1M output on gpt-5.5 @ $5.00/$30.00 per MTok = $35.00.
	c := CostUSD("gpt-5.5", TokenCounts{
		InputTokens:  1_000_000,
		OutputTokens: 1_000_000,
	})
	assert.InDelta(t, 35.00, float64(c), 1e-9)
}

func TestCostUSD_GPT55_CachedSubset(t *testing.T) {
	// 1M total input, 800k cached, 200k uncached; 1M output on gpt-5.5.
	// Cost = 200k * $5.00/MTok + 800k * $0.50/MTok + 1M * $30.00/MTok
	//      = $1.00 + $0.40 + $30.00 = $31.40.
	var tc TokenCounts
	tc.InputTokens = 1_000_000
	tc.InputTokensDetails.CachedTokens = 800_000
	tc.OutputTokens = 1_000_000
	c := CostUSD("gpt-5.5", tc)
	assert.InDelta(t, 31.40, float64(c), 1e-9)
}

func TestCostUSD_GPT61Sol_CachedSubset(t *testing.T) {
	// 1M total input: 500k uncached, 500k cached read; 1M output on gpt-6.1-sol.
	// Cost = 500k * $2.00/MTok + 500k * $0.10/MTok + 1M * $10.00/MTok
	//      = $1.00 + $0.05 + $10.00 = $11.05.
	var tc TokenCounts
	tc.InputTokens = 1_000_000
	tc.InputTokensDetails.CachedTokens = 500_000
	tc.OutputTokens = 1_000_000
	c := CostUSD("gpt-6.1-sol", tc)
	assert.InDelta(t, 11.05, float64(c), 1e-9)
}

func TestCostUSD_GPT6Sol_CachedSubset(t *testing.T) {
	// 1M total input: 500k uncached, 500k cached read; 1M output on gpt-6-sol.
	// Cost = 500k * $2.00/MTok + 500k * $0.20/MTok + 1M * $10.00/MTok
	//      = $1.00 + $0.10 + $10.00 = $11.10.
	var tc TokenCounts
	tc.InputTokens = 1_000_000
	tc.InputTokensDetails.CachedTokens = 500_000
	tc.OutputTokens = 1_000_000
	c := CostUSD("gpt-6-sol", tc)
	assert.InDelta(t, 11.10, float64(c), 1e-9)
}

func TestCostUSD_GPT6Astra_CachedSubset(t *testing.T) {
	// 1M total input: 500k uncached, 500k cached read; 1M output on gpt-6-astra.
	// Cost = 500k * $10.00/MTok + 500k * $1.00/MTok + 1M * $50.00/MTok
	//      = $5.00 + $0.50 + $50.00 = $55.50.
	var tc TokenCounts
	tc.InputTokens = 1_000_000
	tc.InputTokensDetails.CachedTokens = 500_000
	tc.OutputTokens = 1_000_000
	c := CostUSD("gpt-6-astra", tc)
	assert.InDelta(t, 55.50, float64(c), 1e-9)
}

func TestCostUSD_GPT6Luna_CachedSubset(t *testing.T) {
	// 1M total input: 500k uncached, 500k cached read; 1M output on gpt-6-luna.
	// Cost = 500k * $0.10/MTok + 500k * $0.01/MTok + 1M * $0.50/MTok
	//      = $0.05 + $0.005 + $0.50 = $0.555.
	var tc TokenCounts
	tc.InputTokens = 1_000_000
	tc.InputTokensDetails.CachedTokens = 500_000
	tc.OutputTokens = 1_000_000
	c := CostUSD("gpt-6-luna", tc)
	assert.InDelta(t, 0.555, float64(c), 1e-9)
}

func TestCostUSD_GPT56Sol_CachedSubset(t *testing.T) {
	// 1M total input: 500k uncached, 500k cached read; 1M output on gpt-5.6-sol.
	// Cost = 500k * $4.00/MTok + 500k * $0.40/MTok + 1M * $20.00/MTok
	//      = $2.00 + $0.20 + $20.00 = $22.20.
	var tc TokenCounts
	tc.InputTokens = 1_000_000
	tc.InputTokensDetails.CachedTokens = 500_000
	tc.OutputTokens = 1_000_000
	c := CostUSD("gpt-5.6-sol", tc)
	assert.InDelta(t, 22.20, float64(c), 1e-9)
}

func TestCostUSD_GPT56Terra_CachedSubset(t *testing.T) {
	// 1M total input: 500k uncached, 500k cached read; 1M output on gpt-5.6-terra.
	// Cost = 500k * $2.00/MTok + 500k * $0.20/MTok + 1M * $12.00/MTok
	//      = $1.00 + $0.10 + $12.00 = $13.10.
	var tc TokenCounts
	tc.InputTokens = 1_000_000
	tc.InputTokensDetails.CachedTokens = 500_000
	tc.OutputTokens = 1_000_000
	c := CostUSD("gpt-5.6-terra", tc)
	assert.InDelta(t, 13.10, float64(c), 1e-9)
}

func TestCostUSD_GPT56Luna_CachedSubset(t *testing.T) {
	// 1M total input: 500k uncached, 500k cached read; 1M output on gpt-5.6-luna.
	// Cost = 500k * $0.20/MTok + 500k * $0.02/MTok + 1M * $1.20/MTok
	//      = $0.10 + $0.01 + $1.20 = $1.31.
	var tc TokenCounts
	tc.InputTokens = 1_000_000
	tc.InputTokensDetails.CachedTokens = 500_000
	tc.OutputTokens = 1_000_000
	c := CostUSD("gpt-5.6-luna", tc)
	assert.InDelta(t, 1.31, float64(c), 1e-9)
}

func TestCostUSD_UnknownModelReturnsZero(t *testing.T) {
	c := CostUSD("claude-sonnet-4-6", TokenCounts{InputTokens: 1_000_000})
	assert.InDelta(t, 0.0, float64(c), 1e-9)
}

// TestModelCatalog_NoPrefixShadowing guards LookupPricing's first-match
// contract: no entry's IDPrefix may start with an earlier entry's IDPrefix,
// or IDs of the later family would resolve to the earlier family's prices.
func TestModelCatalog_NoPrefixShadowing(t *testing.T) {
	for i, earlier := range modelCatalog {
		for _, later := range modelCatalog[i+1:] {
			assert.False(t, strings.HasPrefix(string(later.IDPrefix), string(earlier.IDPrefix)),
				"%q is shadowed by earlier prefix %q", later.IDPrefix, earlier.IDPrefix)
		}
	}
}
