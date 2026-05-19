package sandbox

import (
	"testing"

	opensandbox "github.com/alibaba/OpenSandbox/sdks/sandbox/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const defaultDeny = "deny"

func TestBuildNetworkPolicy_None(t *testing.T) {
	p, err := BuildNetworkPolicy(EgressNone, nil)
	require.NoError(t, err)
	assertDefaultDeny(t, p)
	assert.Empty(t, p.Egress)
}

func TestBuildNetworkPolicy_PackageManagers(t *testing.T) {
	p, err := BuildNetworkPolicy(EgressPackageManagers, nil)
	require.NoError(t, err)
	assertDefaultDeny(t, p)
	require.Len(t, p.Egress, len(PackageManagerHosts))
	for i, host := range PackageManagerHosts {
		assert.Equal(t, "allow", p.Egress[i].Action, "rule %d action", i)
		assert.Equal(t, host, p.Egress[i].Target, "rule %d target", i)
	}
}

func TestBuildNetworkPolicy_UnknownMode(t *testing.T) {
	_, err := BuildNetworkPolicy(EgressMode("anywhere"), nil)
	assert.Error(t, err)
}

func TestBuildNetworkPolicy_ExtraAllow(t *testing.T) {
	p, err := BuildNetworkPolicy(EgressNone, []string{"api.anthropic.com"})
	require.NoError(t, err)
	assertDefaultDeny(t, p)
	require.Len(t, p.Egress, 1)
	assert.Equal(t, "api.anthropic.com", p.Egress[0].Target)
	assert.Equal(t, "allow", p.Egress[0].Action)
}

func TestBuildNetworkPolicy_Open(t *testing.T) {
	p, err := BuildNetworkPolicy(EgressOpen, nil)
	require.NoError(t, err)
	assert.Equal(t, "allow", p.DefaultAction)
	assert.Empty(t, p.Egress)
}

func TestBuildNetworkPolicy_OpenIgnoresExtraAllow(t *testing.T) {
	p, err := BuildNetworkPolicy(EgressOpen, []string{"example.com"})
	require.NoError(t, err)
	assert.Equal(t, "allow", p.DefaultAction)
	assert.Empty(t, p.Egress)
}

func assertDefaultDeny(t *testing.T, p *opensandbox.NetworkPolicy) {
	t.Helper()
	assert.Equal(t, defaultDeny, p.DefaultAction)
}
