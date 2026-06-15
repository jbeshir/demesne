package anthropic

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseClaudeCodeVersion(t *testing.T) {
	cases := []struct {
		name   string
		in     string
		want   string
		wantOK bool
	}{
		{"version with suffix", "2.1.0 (Claude Code)\n", "2.1.0", true},
		{"bare version", "1.0.123\n", "1.0.123", true},
		{"leading spaces", "   2.0.0 (Claude Code)", "2.0.0", true},
		{"empty", "", "", false},
		{"no dot", "latest\n", "", false},
		{"non-numeric token", "v2 (Claude Code)", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := parseClaudeCodeVersion(tc.in)
			assert.Equal(t, tc.wantOK, ok)
			assert.Equal(t, tc.want, got)
		})
	}
}
