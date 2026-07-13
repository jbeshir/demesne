package openai

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLookupPricing_GPT56Sol_ExactPrices(t *testing.T) {
	p, ok := LookupPricing("gpt-5.6-sol")
	require.True(t, ok)
	assert.InDelta(t, 5.0, float64(p.InputPerMTok), 1e-9)
	assert.InDelta(t, 0.5, float64(p.CachedInputPerMTok), 1e-9)
	assert.InDelta(t, 30.0, float64(p.OutputPerMTok), 1e-9)
}

func TestLookupPricing_GPT56Terra_ExactPrices(t *testing.T) {
	p, ok := LookupPricing("gpt-5.6-terra")
	require.True(t, ok)
	assert.InDelta(t, 2.5, float64(p.InputPerMTok), 1e-9)
	assert.InDelta(t, 0.25, float64(p.CachedInputPerMTok), 1e-9)
	assert.InDelta(t, 15.0, float64(p.OutputPerMTok), 1e-9)
}

func TestLookupPricing_GPT56Luna_ExactPrices(t *testing.T) {
	p, ok := LookupPricing("gpt-5.6-luna")
	require.True(t, ok)
	assert.InDelta(t, 1.0, float64(p.InputPerMTok), 1e-9)
	assert.InDelta(t, 0.10, float64(p.CachedInputPerMTok), 1e-9)
	assert.InDelta(t, 6.0, float64(p.OutputPerMTok), 1e-9)
}

func TestLookupPricing_GPT55_ExactPrices(t *testing.T) {
	p, ok := LookupPricing("gpt-5.5")
	require.True(t, ok)
	assert.InDelta(t, 5.0, float64(p.InputPerMTok), 1e-9)
	assert.InDelta(t, 0.5, float64(p.CachedInputPerMTok), 1e-9)
	assert.InDelta(t, 30.0, float64(p.OutputPerMTok), 1e-9)
}

func TestLookupPricing_GPT54Mini_ExactPrices(t *testing.T) {
	p, ok := LookupPricing("gpt-5.4-mini")
	require.True(t, ok)
	assert.InDelta(t, 0.75, float64(p.InputPerMTok), 1e-9)
	assert.InDelta(t, 0.075, float64(p.CachedInputPerMTok), 1e-9)
	assert.InDelta(t, 4.50, float64(p.OutputPerMTok), 1e-9)
}

func TestLookupPricing_PrefixMatchVersionedID(t *testing.T) {
	p56, ok := LookupPricing("gpt-5.6-sol-2026-07-13")
	require.True(t, ok)
	assert.InDelta(t, 5.0, float64(p56.InputPerMTok), 1e-9)
	assert.InDelta(t, 30.0, float64(p56.OutputPerMTok), 1e-9)

	// gpt-5.5-2026-01-01 must resolve to the gpt-5.5 entry.
	p55, ok := LookupPricing("gpt-5.5-2026-01-01")
	require.True(t, ok)
	assert.InDelta(t, 5.0, float64(p55.InputPerMTok), 1e-9)
	assert.InDelta(t, 30.0, float64(p55.OutputPerMTok), 1e-9)

	// gpt-5.4-mini-20260101 must resolve to the gpt-5.4-mini entry, not gpt-5.5.
	pmini, ok := LookupPricing("gpt-5.4-mini-20260101")
	require.True(t, ok)
	assert.InDelta(t, 0.75, float64(pmini.InputPerMTok), 1e-9)
	assert.InDelta(t, 0.075, float64(pmini.CachedInputPerMTok), 1e-9)
	assert.InDelta(t, 4.50, float64(pmini.OutputPerMTok), 1e-9)
}

func TestLookupPricing_UnknownRemoved(t *testing.T) {
	for _, id := range []ModelID{"gpt-5.4", "gpt-5.3-codex", "gpt-5.2", "claude-sonnet-4-6"} {
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

func TestCostUSD_GPT56Sol_CachedSubset(t *testing.T) {
	// 1M total input: 500k uncached, 500k cached read; 1M output on gpt-5.6-sol.
	// Cost = 500k * $5.00/MTok + 500k * $0.50/MTok + 1M * $30.00/MTok
	//      = $2.50 + $0.25 + $30.00 = $32.75.
	var tc TokenCounts
	tc.InputTokens = 1_000_000
	tc.InputTokensDetails.CachedTokens = 500_000
	tc.OutputTokens = 1_000_000
	c := CostUSD("gpt-5.6-sol", tc)
	assert.InDelta(t, 32.75, float64(c), 1e-9)
}

func TestCostUSD_GPT56Terra_CachedSubset(t *testing.T) {
	// 1M total input: 500k uncached, 500k cached read; 1M output on gpt-5.6-terra.
	// Cost = 500k * $2.50/MTok + 500k * $0.25/MTok + 1M * $15.00/MTok
	//      = $1.25 + $0.125 + $15.00 = $16.375.
	var tc TokenCounts
	tc.InputTokens = 1_000_000
	tc.InputTokensDetails.CachedTokens = 500_000
	tc.OutputTokens = 1_000_000
	c := CostUSD("gpt-5.6-terra", tc)
	assert.InDelta(t, 16.375, float64(c), 1e-9)
}

func TestCostUSD_GPT56Luna_CachedSubset(t *testing.T) {
	// 1M total input: 500k uncached, 500k cached read; 1M output on gpt-5.6-luna.
	// Cost = 500k * $1.00/MTok + 500k * $0.10/MTok + 1M * $6.00/MTok
	//      = $0.50 + $0.05 + $6.00 = $6.55.
	var tc TokenCounts
	tc.InputTokens = 1_000_000
	tc.InputTokensDetails.CachedTokens = 500_000
	tc.OutputTokens = 1_000_000
	c := CostUSD("gpt-5.6-luna", tc)
	assert.InDelta(t, 6.55, float64(c), 1e-9)
}

func TestCostUSD_GPT54Mini_Math(t *testing.T) {
	// 1M input (0 cached) + 1M output on gpt-5.4-mini @ $0.75/$4.50 per MTok = $5.25.
	c := CostUSD("gpt-5.4-mini", TokenCounts{
		InputTokens:  1_000_000,
		OutputTokens: 1_000_000,
	})
	assert.InDelta(t, 5.25, float64(c), 1e-9)
}

func TestCostUSD_UnknownModelReturnsZero(t *testing.T) {
	c := CostUSD("claude-sonnet-4-6", TokenCounts{InputTokens: 1_000_000})
	assert.InDelta(t, 0.0, float64(c), 1e-9)
}
