package sandbox

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ValidateMountPath resolves host (following symlinks), then verifies the
// resolved path is contained within at least one entry of allowed (also
// symlink-resolved). It returns the resolved host path that should be used
// for the actual mount, so the authorisation decision and the mount target
// are based on the same path.
func ValidateMountPath(host string, allowed []string) (string, error) {
	if host == "" {
		return "", errors.New("mount path is empty")
	}
	cleaned := filepath.Clean(host)
	if !filepath.IsAbs(cleaned) {
		return "", fmt.Errorf("mount path must be absolute: %s", host)
	}

	resolved, err := filepath.EvalSymlinks(cleaned)
	if err != nil {
		return "", fmt.Errorf("resolve mount path %s: %w", host, err)
	}

	for _, root := range allowed {
		rootResolved, err := filepath.EvalSymlinks(root)
		if err != nil {
			continue
		}
		if resolved == rootResolved {
			return resolved, nil
		}
		if strings.HasPrefix(resolved, rootResolved+string(os.PathSeparator)) {
			return resolved, nil
		}
	}

	return "", fmt.Errorf("mount path %s is not within DEMESNE_ALLOWED_PATHS", host)
}
