package anthropic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLookupPricing_Sonnet_ExactPrices(t *testing.T) {
	p, ok := LookupPricing("claude-sonnet-5-5")
	require.True(t, ok)
	assert.InDelta(t, 2.0, float64(p.InputPerMTok), 1e-9)
	assert.InDelta(t, 10.0, float64(p.OutputPerMTok), 1e-9)
	assert.InDelta(t, 2.50, float64(p.CacheWritePerMTok), 1e-9)
	assert.InDelta(t, 0.20, float64(p.CacheReadPerMTok), 1e-9)
}

func TestLookupPricing_Opus_ExactPrices(t *testing.T) {
	p, ok := LookupPricing("claude-opus-5-5")
	require.True(t, ok)
	assert.InDelta(t, 4.0, float64(p.InputPerMTok), 1e-9)
	assert.InDelta(t, 20.0, float64(p.OutputPerMTok), 1e-9)
	assert.InDelta(t, 5.0, float64(p.CacheWritePerMTok), 1e-9)
	assert.InDelta(t, 0.20, float64(p.CacheReadPerMTok), 1e-9)
}

func TestLookupPricing_Fable_ExactPrices(t *testing.T) {
	p, ok := LookupPricing("claude-fable-5-1")
	require.True(t, ok)
	assert.InDelta(t, 10.0, float64(p.InputPerMTok), 1e-9)
	assert.InDelta(t, 50.0, float64(p.OutputPerMTok), 1e-9)
	assert.InDelta(t, 12.50, float64(p.CacheWritePerMTok), 1e-9)
	assert.InDelta(t, 0.25, float64(p.CacheReadPerMTok), 1e-9)
}

func TestLookupPricing_Haiku_ExactPrices(t *testing.T) {
	p, ok := LookupPricing("claude-haiku-4-5-20251001")
	require.True(t, ok)
	assert.InDelta(t, 1.0, float64(p.InputPerMTok), 1e-9)
	assert.InDelta(t, 5.0, float64(p.OutputPerMTok), 1e-9)
	assert.InDelta(t, 1.25, float64(p.CacheWritePerMTok), 1e-9)
	assert.InDelta(t, 0.10, float64(p.CacheReadPerMTok), 1e-9)
}

func TestLookupPricing_PrefixMatchDatedID(t *testing.T) {
	// Anthropic's API returns dated model IDs like claude-opus-5-5-20260101.
	// The longest-prefix match should hit the family entry.
	p, ok := LookupPricing("claude-opus-5-5-20260101")
	require.True(t, ok)
	assert.InDelta(t, 4.0, float64(p.InputPerMTok), 1e-9)

	p2, ok2 := LookupPricing("claude-sonnet-5-5-20260101")
	require.True(t, ok2)
	assert.InDelta(t, 2.0, float64(p2.InputPerMTok), 1e-9)

	p3, ok3 := LookupPricing("claude-fable-5-1-20260609")
	require.True(t, ok3)
	assert.InDelta(t, 10.0, float64(p3.InputPerMTok), 1e-9)
}

func TestLookupPricing_UnpricedIDs(t *testing.T) {
	// These model IDs are not in the catalog and must not match any entry.
	unpriced := []string{
		"claude-opus-4-8",          // not in catalog; must not match the claude-opus-5 prefix
		"claude-opus-4-8-20260101", // dated form; must not match the claude-opus-5 prefix
		"claude-opus-4-7",          // not in catalog; must not match the claude-opus-5 prefix
		"claude-opus-4-7-20251201", // dated form; must not match the claude-opus-5 prefix
		"claude-opus-4-anything",   // no loose claude-opus-4 prefix entry
		"claude-sonnet-4-3",        // must not match the claude-sonnet-5 prefix
		"claude-haiku-4-3",         // must not match the claude-haiku-4-5 prefix
		"claude-opus-3",            // no claude-opus-3 entry
	}
	for _, id := range unpriced {
		_, ok := LookupPricing(ModelID(id))
		assert.False(t, ok, "expected no match for %q", id)
	}
}

func TestLookupPricing_Unknown(t *testing.T) {
	_, ok := LookupPricing("gpt-4o")
	assert.False(t, ok)
}

func TestCostUSD_SonnetMath(t *testing.T) {
	// 1M input + 1M output on sonnet @ $2 / $10 per MTok = $12.
	c := CostUSD("claude-sonnet-5-5", TokenCounts{
		InputTokens:  1_000_000,
		OutputTokens: 1_000_000,
	})
	assert.InDelta(t, 12.0, float64(c), 1e-9)
}

func TestCostUSD_OpusMath(t *testing.T) {
	// 1M input + 1M output on opus @ $4 / $20 per MTok = $24.
	c := CostUSD("claude-opus-5-5", TokenCounts{
		InputTokens:  1_000_000,
		OutputTokens: 1_000_000,
	})
	assert.InDelta(t, 24.0, float64(c), 1e-9)
}

func TestCostUSD_FableMath(t *testing.T) {
	// 1M input + 1M output on fable @ $10 / $50 per MTok = $60.
	c := CostUSD("claude-fable-5-1", TokenCounts{
		InputTokens:  1_000_000,
		OutputTokens: 1_000_000,
	})
	assert.InDelta(t, 60.0, float64(c), 1e-9)
}

func TestCostUSD_HaikuMath(t *testing.T) {
	// 1M input + 1M output on haiku @ $1 / $5 per MTok = $6.
	c := CostUSD("claude-haiku-4-5-20251001", TokenCounts{
		InputTokens:  1_000_000,
		OutputTokens: 1_000_000,
	})
	assert.InDelta(t, 6.0, float64(c), 1e-9)
}

func TestCostUSD_SonnetCacheTokens(t *testing.T) {
	// 1M cache write + 1M cache read on sonnet @ $2.50 / $0.20 per MTok = $2.70.
	c := CostUSD("claude-sonnet-5-5", TokenCounts{
		CacheCreationInputTokens: 1_000_000,
		CacheReadInputTokens:     1_000_000,
	})
	assert.InDelta(t, 2.70, float64(c), 1e-9)
}

func TestCostUSD_OpusCacheTokens(t *testing.T) {
	// 1M cache write + 1M cache read on opus @ $5 / $0.20 per MTok = $5.20.
	c := CostUSD("claude-opus-5-5", TokenCounts{
		CacheCreationInputTokens: 1_000_000,
		CacheReadInputTokens:     1_000_000,
	})
	assert.InDelta(t, 5.20, float64(c), 1e-9)
}

func TestCostUSD_FableCacheTokens(t *testing.T) {
	// 1M cache write + 1M cache read on fable @ $12.50 / $0.25 per MTok = $12.75.
	c := CostUSD("claude-fable-5-1", TokenCounts{
		CacheCreationInputTokens: 1_000_000,
		CacheReadInputTokens:     1_000_000,
	})
	assert.InDelta(t, 12.75, float64(c), 1e-9)
}

func TestCostUSD_UnknownModelReturnsZero(t *testing.T) {
	c := CostUSD("gpt-4o", TokenCounts{InputTokens: 1_000_000})
	assert.InDelta(t, 0.0, float64(c), 1e-9)
}
