package sandbox

import (
	"os"
	"path/filepath"
)

// readOutputFile reads a file composed from a runner-controlled host dir
// (cfg.OutputRoot, resultsHost, outputHost) and a constant basename. The
// dir always sits under cfg.OutputRoot; gosec G304 false-positive — the
// suppression lives here so callers don't repeat it.
func readOutputFile(dir, base string) ([]byte, error) {
	return os.ReadFile(filepath.Join(dir, base)) //nolint:gosec // dir is runner-composed; base is a constant
}

// readOutputPath is readOutputFile for callers that already hold the full
// path (e.g. copyUsageToOut, which computes one path for both read and
// write). The path must still be runner-composed under cfg.OutputRoot.
func readOutputPath(path string) ([]byte, error) {
	return os.ReadFile(path) //nolint:gosec // path is runner-composed under cfg.OutputRoot
}

// writeOutputFile writes data at dir/base with 0o600. The dir is always
// a runner-composed host path under cfg.OutputRoot; base is a constant.
func writeOutputFile(dir, base string, data []byte) error {
	return os.WriteFile(filepath.Join(dir, base), data, 0o600)
}

// openValidatedHostFile opens a host file whose path was validated by
// ValidateMountPath against cfg.AllowedPaths. The validation invariant
// is the caller's responsibility; this helper just carries the gosec
// G304 suppression.
func openValidatedHostFile(path string) (*os.File, error) {
	return os.Open(path) //nolint:gosec // path validated against AllowedPaths via ValidateMountPath before this call
}

// createDownloadFile creates an output file under cfg.OutputRoot for a
// Download. The hostPath is OutputRoot/<jobID>/downloads/<filepath.Base(src)>
// — the basename is traversal-stripped at the call site.
func createDownloadFile(hostPath string) (*os.File, error) {
	return os.OpenFile(hostPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600) //nolint:gosec // hostPath under cfg.OutputRoot
}
