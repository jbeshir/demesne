package sandbox

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateMountPath(t *testing.T) {
	root := t.TempDir()
	allowed := filepath.Join(root, "allowed")
	sibling := filepath.Join(root, "allowed2")
	outside := filepath.Join(root, "outside")
	for _, d := range []string{allowed, sibling, outside} {
		require.NoError(t, os.MkdirAll(d, 0o750))
	}
	nestedFile := filepath.Join(allowed, "sub", "data.txt")
	require.NoError(t, os.MkdirAll(filepath.Dir(nestedFile), 0o750))
	require.NoError(t, os.WriteFile(nestedFile, []byte("x"), 0o600))
	innerSymlink := filepath.Join(allowed, "linked")
	require.NoError(t, os.Symlink(nestedFile, innerSymlink))
	escapeSymlink := filepath.Join(allowed, "escape")
	require.NoError(t, os.Symlink(outside, escapeSymlink))

	allowedList := []string{allowed}

	tests := []struct {
		name    string
		host    string
		wantErr string // substring check; empty means no error expected
		errIs   error  // if non-nil, use errors.Is instead of substring
	}{
		{name: "exact match", host: allowed},
		{name: "nested file", host: nestedFile},
		{name: "symlink within allowed", host: innerSymlink},
		{name: "symlink escapes allowed", host: escapeSymlink, errIs: ErrPathNotAllowed},
		{name: "outside allowed", host: outside, errIs: ErrPathNotAllowed},
		{name: "sibling-prefix not allowed", host: sibling, errIs: ErrPathNotAllowed},
		{name: "relative path rejected", host: "relative/path", wantErr: "absolute"},
		{name: "empty rejected", host: "", wantErr: "empty"},
		{name: "dotdot rejected", host: filepath.Join(allowed, "..", "outside"), errIs: ErrPathNotAllowed},
		{name: "missing path rejected", host: filepath.Join(allowed, "does-not-exist"), wantErr: "resolve"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ValidateMountPath(tt.host, allowedList)
			if tt.errIs != nil {
				require.Error(t, err, "resolved=%s", got)
				assert.ErrorIs(t, err, tt.errIs)
				return
			}
			if tt.wantErr != "" {
				require.Error(t, err, "resolved=%s", got)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.True(t, strings.HasPrefix(got, allowed),
				"resolved path %s is not under %s", got, allowed)
		})
	}
}
