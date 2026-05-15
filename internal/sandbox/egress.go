package sandbox

import (
	"fmt"

	opensandbox "github.com/alibaba/OpenSandbox/sdks/sandbox/go"
)

// PackageManagerHosts is the explicit allowlist applied when egress mode is
// "package-managers". Hosts are the public registries for the three images we
// support; nothing else is reachable.
var PackageManagerHosts = []string{
	"registry.npmjs.org",
	"pypi.org",
	"files.pythonhosted.org",
	"repo.anaconda.com",
	"conda.anaconda.org",
}

// BuildNetworkPolicy translates an EgressMode into the OpenSandbox network
// policy applied at sandbox creation time.
func BuildNetworkPolicy(mode EgressMode) (*opensandbox.NetworkPolicy, error) {
	switch mode {
	case EgressNone:
		return &opensandbox.NetworkPolicy{DefaultAction: "deny"}, nil
	case EgressPackageManagers:
		rules := make([]opensandbox.NetworkRule, 0, len(PackageManagerHosts))
		for _, host := range PackageManagerHosts {
			rules = append(rules, opensandbox.NetworkRule{Action: "allow", Target: host})
		}
		return &opensandbox.NetworkPolicy{DefaultAction: "deny", Egress: rules}, nil
	default:
		return nil, fmt.Errorf("egress mode %q is not in the whitelist (none, package-managers)", mode)
	}
}
