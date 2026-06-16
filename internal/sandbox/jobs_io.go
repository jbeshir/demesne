package sandbox

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// tailFile returns the last maxBytes bytes of the file at path, dropping any
// partial first line that results from seeking to the middle of the file.
// A missing file returns an empty string without error. The path must be
// runner-composed under cfg.OutputRoot.
func tailFile(path string, maxBytes int64) (string, error) {
	f, err := os.Open(path) //nolint:gosec // path is runner-composed under job outHost
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("open file for tail: %w", err)
	}
	defer func() { _ = f.Close() }()

	info, err := f.Stat()
	if err != nil {
		return "", fmt.Errorf("stat file for tail: %w", err)
	}
	size := info.Size()
	if size == 0 {
		return "", nil
	}

	start := size - maxBytes
	if start < 0 {
		start = 0
	}

	if _, err := f.Seek(start, io.SeekStart); err != nil {
		return "", fmt.Errorf("seek file for tail: %w", err)
	}

	buf, err := io.ReadAll(f)
	if err != nil {
		return "", fmt.Errorf("read file tail: %w", err)
	}

	s := string(buf)
	if start > 0 {
		if idx := strings.Index(s, "\n"); idx >= 0 {
			s = s[idx+1:]
		}
	}
	return s, nil
}
