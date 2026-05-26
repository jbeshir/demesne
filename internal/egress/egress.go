// Package egress defines the shared egress-mode vocabulary used by both the
// sandbox and agents packages without creating an import cycle.
package egress

// Mode controls outbound network access for a sandbox.
type Mode string

const (
	None            Mode = "none"
	PackageManagers Mode = "package-managers"
	// Open is unrestricted outbound access. Only sandbox_research
	// uses it; sandbox_agent rejects it because pairing read-only inputs
	// with open egress is the data-exfiltration shape we want to keep
	// off the surface.
	Open Mode = "open"
)
