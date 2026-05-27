package proxycommon

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// EnsureUsageDir creates the parent directory for usagePath if missing.
// Caller invokes this once at startup so per-request writes can't fail
// on a missing dir. Empty usagePath is a no-op.
func EnsureUsageDir(usagePath string) error {
	if usagePath == "" {
		return nil
	}
	return os.MkdirAll(filepath.Dir(usagePath), 0o750)
}

// WriteUsageAtomic marshals snap as indented JSON and atomically replaces
// usagePath (write .tmp then rename). Failures are logged to stderr with
// logPrefix (e.g. "anthropic proxy") and otherwise ignored so the proxy
// keeps serving. Caller is responsible for skipping the call when the
// usage path is empty.
func WriteUsageAtomic(usagePath, logPrefix string, snap any) {
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: marshal usage snapshot: %v\n", logPrefix, err)
		return
	}
	tmp := usagePath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		fmt.Fprintf(os.Stderr, "%s: write usage tmp: %v\n", logPrefix, err)
		return
	}
	if err := os.Rename(tmp, usagePath); err != nil {
		fmt.Fprintf(os.Stderr, "%s: rename usage: %v\n", logPrefix, err)
	}
}

// ScanSSELines consumes complete '\n'-terminated lines from buf, calling
// handle for each with the trailing \r\n stripped. When eof is true it
// also drains a final unterminated line. Partial lines remain buffered
// for the next call.
func ScanSSELines(buf *bytes.Buffer, eof bool, handle func(string)) {
	for {
		idx := bytes.IndexByte(buf.Bytes(), '\n')
		if idx < 0 {
			if !eof {
				return
			}
			if buf.Len() == 0 {
				return
			}
			line := buf.String()
			buf.Reset()
			handle(line)
			return
		}
		raw := buf.Next(idx + 1)
		// Strip trailing \r\n or \n.
		line := strings.TrimRight(string(raw), "\r\n")
		handle(line)
	}
}
