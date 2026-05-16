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
// policy applied at sandbox creation time. Extra allow targets — hostnames
// or IPs supplied by the caller — are unioned with the mode's allowlist.
// This is how the agent runner adds host.docker.internal on top of the
// caller-visible egress mode without inventing extra egress modes.
func BuildNetworkPolicy(mode EgressMode, extraAllow []string) (*opensandbox.NetworkPolicy, error) {
	var hosts []string
	switch mode {
	case EgressNone:
		hosts = nil
	case EgressPackageManagers:
		hosts = append(hosts, PackageManagerHosts...)
	default:
		return nil, fmt.Errorf("egress mode %q is not in the whitelist (none, package-managers)", mode)
	}
	hosts = append(hosts, extraAllow...)

	rules := make([]opensandbox.NetworkRule, 0, len(hosts))
	for _, host := range hosts {
		rules = append(rules, opensandbox.NetworkRule{Action: "allow", Target: host})
	}
	return &opensandbox.NetworkPolicy{DefaultAction: "deny", Egress: rules}, nil
}
