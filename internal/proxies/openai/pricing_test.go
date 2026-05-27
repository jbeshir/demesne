package openai

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLookupPricing_ExactFamily(t *testing.T) {
	p, ok := LookupPricing(gpt55)
	require.True(t, ok)
	assert.InDelta(t, 1.25, float64(p.InputPerMTok), 1e-9)
	assert.InDelta(t, 10.0, float64(p.OutputPerMTok), 1e-9)
}

func TestLookupPricing_MiniBeforeBase(t *testing.T) {
	// gpt-5.4-mini-2026... must match gpt-5.4-mini, not the shorter gpt-5.4 entry.
	miniP, ok := LookupPricing("gpt-5.4-mini-20260101")
	require.True(t, ok)
	baseP, ok2 := LookupPricing("gpt-5.4-anything")
	require.True(t, ok2)
	// The two entries have different rates, so prefix match correctness is observable.
	assert.NotEqual(t, miniP.InputPerMTok, baseP.InputPerMTok,
		"gpt-5.4-mini must resolve to the mini entry, not the gpt-5.4 entry")
	assert.InDelta(t, 0.30, float64(miniP.InputPerMTok), 1e-9)
	assert.InDelta(t, 1.25, float64(baseP.InputPerMTok), 1e-9)
}

func TestLookupPricing_CodexSparkBeforeCodex(t *testing.T) {
	// gpt-5.3-codex-spark must match the spark entry, not the shorter gpt-5.3-codex.
	sparkP, ok := LookupPricing("gpt-5.3-codex-spark")
	require.True(t, ok)
	codexP, ok2 := LookupPricing("gpt-5.3-codex-20260101")
	require.True(t, ok2)
	assert.NotEqual(t, sparkP.InputPerMTok, codexP.InputPerMTok,
		"gpt-5.3-codex-spark must resolve to the spark entry, not the codex entry")
	assert.InDelta(t, 0.50, float64(sparkP.InputPerMTok), 1e-9)
	assert.InDelta(t, 1.25, float64(codexP.InputPerMTok), 1e-9)
}

func TestLookupPricing_Unknown(t *testing.T) {
	_, ok := LookupPricing("claude-sonnet-4-6")
	assert.False(t, ok)
}

func TestCostUSD_BasicMath(t *testing.T) {
	// 1M input (0 cached) + 1M output on gpt-5.5 @ $1.25 / $10.00 per MTok = $11.25.
	c := CostUSD(gpt55, TokenCounts{
		InputTokens:  1_000_000,
		OutputTokens: 1_000_000,
	})
	assert.InDelta(t, 11.25, float64(c), 1e-9)
}

func TestCostUSD_CachedSubset(t *testing.T) {
	// 1M total input, 800k cached, 200k uncached; 1M output on gpt-5.5.
	// Cost = 200k * $1.25/MTok + 800k * $0.125/MTok + 1M * $10.00/MTok
	//      = $0.25 + $0.10 + $10.00 = $10.35.
	var tc TokenCounts
	tc.InputTokens = 1_000_000
	tc.InputTokensDetails.CachedTokens = 800_000
	tc.OutputTokens = 1_000_000
	c := CostUSD(gpt55, tc)
	assert.InDelta(t, 10.35, float64(c), 1e-9)
}

func TestCostUSD_AllCached(t *testing.T) {
	// All input tokens are cached: cost = 0 * inputRate + 1M * cachedRate + 0 * outputRate.
	var tc TokenCounts
	tc.InputTokens = 1_000_000
	tc.InputTokensDetails.CachedTokens = 1_000_000
	c := CostUSD(gpt55, tc)
	assert.InDelta(t, 0.125, float64(c), 1e-9)
}

func TestCostUSD_UnknownModelReturnsZero(t *testing.T) {
	c := CostUSD("gpt-4o", TokenCounts{InputTokens: 1_000_000})
	assert.InDelta(t, 0.0, float64(c), 1e-9)
}
