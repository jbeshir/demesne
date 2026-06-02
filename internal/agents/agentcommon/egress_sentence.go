package agentcommon

import "github.com/jbeshir/demesne/internal/egress"

// EgressSentence returns the human-readable sentence describing what
// the sandbox can reach over the network. vendorAPI names the vendor
// API the sandbox needs (e.g. "the Anthropic API", "the OpenAI API");
// it appears in the restricted-egress sentences. Unknown modes fall
// back to the strictest case so we never promise more than the policy
// allows.
func EgressSentence(mode egress.Mode, vendorAPI string) string {
	switch mode {
	case egress.Open:
		return "Outbound network access is unrestricted — you can reach any " +
			"HTTPS endpoint on the open internet."
	case egress.PackageManagers:
		return "Outbound network access is restricted to " + vendorAPI +
			" backing this CLI and the standard npm/PyPI/conda package registries."
	case egress.None:
		return "Outbound network access is restricted to " + vendorAPI +
			" backing this CLI; nothing else is reachable."
	default:
		return "Outbound network access is restricted to " + vendorAPI +
			" backing this CLI; nothing else is reachable."
	}
}
