package openai

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
