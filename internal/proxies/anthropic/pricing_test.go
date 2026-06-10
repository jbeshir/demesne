package anthropic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLookupPricing_Sonnet_ExactPrices(t *testing.T) {
	p, ok := LookupPricing("claude-sonnet-4-6")
	require.True(t, ok)
	assert.InDelta(t, 3.0, float64(p.InputPerMTok), 1e-9)
	assert.InDelta(t, 15.0, float64(p.OutputPerMTok), 1e-9)
	assert.InDelta(t, 3.75, float64(p.CacheWritePerMTok), 1e-9)
	assert.InDelta(t, 0.30, float64(p.CacheReadPerMTok), 1e-9)
}

func TestLookupPricing_Opus_ExactPrices(t *testing.T) {
	p, ok := LookupPricing("claude-opus-4-8")
	require.True(t, ok)
	assert.InDelta(t, 5.0, float64(p.InputPerMTok), 1e-9)
	assert.InDelta(t, 25.0, float64(p.OutputPerMTok), 1e-9)
	assert.InDelta(t, 6.25, float64(p.CacheWritePerMTok), 1e-9)
	assert.InDelta(t, 0.50, float64(p.CacheReadPerMTok), 1e-9)
}

func TestLookupPricing_Fable_ExactPrices(t *testing.T) {
	p, ok := LookupPricing("claude-fable-5")
	require.True(t, ok)
	assert.InDelta(t, 10.0, float64(p.InputPerMTok), 1e-9)
	assert.InDelta(t, 50.0, float64(p.OutputPerMTok), 1e-9)
	assert.InDelta(t, 12.50, float64(p.CacheWritePerMTok), 1e-9)
	assert.InDelta(t, 1.00, float64(p.CacheReadPerMTok), 1e-9)
}

func TestLookupPricing_Haiku_ExactPrices(t *testing.T) {
	p, ok := LookupPricing("claude-haiku-4-5")
	require.True(t, ok)
	assert.InDelta(t, 1.0, float64(p.InputPerMTok), 1e-9)
	assert.InDelta(t, 5.0, float64(p.OutputPerMTok), 1e-9)
	assert.InDelta(t, 1.25, float64(p.CacheWritePerMTok), 1e-9)
	assert.InDelta(t, 0.10, float64(p.CacheReadPerMTok), 1e-9)
}

func TestLookupPricing_PrefixMatchDatedID(t *testing.T) {
	// Anthropic's API returns dated model IDs like claude-opus-4-8-20260101.
	// The longest-prefix match should hit the family entry.
	p, ok := LookupPricing("claude-opus-4-8-20260101")
	require.True(t, ok)
	assert.InDelta(t, 5.0, float64(p.InputPerMTok), 1e-9)

	p2, ok2 := LookupPricing("claude-sonnet-4-6-20260101")
	require.True(t, ok2)
	assert.InDelta(t, 3.0, float64(p2.InputPerMTok), 1e-9)

	p3, ok3 := LookupPricing("claude-fable-5-20260609")
	require.True(t, ok3)
	assert.InDelta(t, 10.0, float64(p3.InputPerMTok), 1e-9)
}

func TestLookupPricing_RemovedFallbacks(t *testing.T) {
	// These model IDs must not match any entry in the updated catalog.
	removed := []string{
		"claude-opus-4-7",          // old explicit entry — removed
		"claude-opus-4-7-20251201", // dated form of the old entry — removed
		"claude-opus-4-anything",   // loose claude-opus-4 fallback — removed
		"claude-sonnet-4-3",        // does not prefix-match claude-sonnet-4-6 — removed
		"claude-haiku-4-3",         // does not prefix-match claude-haiku-4-5 — removed
		"claude-opus-3",            // older series — never had an entry
	}
	for _, id := range removed {
		_, ok := LookupPricing(ModelID(id))
		assert.False(t, ok, "expected no match for %q", id)
	}
}

func TestLookupPricing_Unknown(t *testing.T) {
	_, ok := LookupPricing("gpt-4o")
	assert.False(t, ok)
}

func TestCostUSD_SonnetMath(t *testing.T) {
	// 1M input + 1M output on sonnet @ $3 / $15 per MTok = $18.
	c := CostUSD("claude-sonnet-4-6", TokenCounts{
		InputTokens:  1_000_000,
		OutputTokens: 1_000_000,
	})
	assert.InDelta(t, 18.0, float64(c), 1e-9)
}

func TestCostUSD_OpusMath(t *testing.T) {
	// 1M input + 1M output on opus @ $5 / $25 per MTok = $30.
	c := CostUSD("claude-opus-4-8", TokenCounts{
		InputTokens:  1_000_000,
		OutputTokens: 1_000_000,
	})
	assert.InDelta(t, 30.0, float64(c), 1e-9)
}

func TestCostUSD_FableMath(t *testing.T) {
	// 1M input + 1M output on fable @ $10 / $50 per MTok = $60.
	c := CostUSD("claude-fable-5", TokenCounts{
		InputTokens:  1_000_000,
		OutputTokens: 1_000_000,
	})
	assert.InDelta(t, 60.0, float64(c), 1e-9)
}

func TestCostUSD_HaikuMath(t *testing.T) {
	// 1M input + 1M output on haiku @ $1 / $5 per MTok = $6.
	c := CostUSD("claude-haiku-4-5", TokenCounts{
		InputTokens:  1_000_000,
		OutputTokens: 1_000_000,
	})
	assert.InDelta(t, 6.0, float64(c), 1e-9)
}

func TestCostUSD_SonnetCacheTokens(t *testing.T) {
	// 1M cache write + 1M cache read on sonnet @ $3.75 / $0.30 per MTok = $4.05.
	c := CostUSD("claude-sonnet-4-6", TokenCounts{
		CacheCreationInputTokens: 1_000_000,
		CacheReadInputTokens:     1_000_000,
	})
	assert.InDelta(t, 4.05, float64(c), 1e-9)
}

func TestCostUSD_UnknownModelReturnsZero(t *testing.T) {
	c := CostUSD("gpt-4o", TokenCounts{InputTokens: 1_000_000})
	assert.InDelta(t, 0.0, float64(c), 1e-9)
}
