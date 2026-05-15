package sandbox

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateMountPath(t *testing.T) {
	root := t.TempDir()
	allowed := filepath.Join(root, "allowed")
	sibling := filepath.Join(root, "allowed2")
	outside := filepath.Join(root, "outside")
	for _, d := range []string{allowed, sibling, outside} {
		mustMkdir(t, d)
	}
	nestedFile := filepath.Join(allowed, "sub", "data.txt")
	mustMkdir(t, filepath.Dir(nestedFile))
	mustWriteFile(t, nestedFile, "x")
	innerSymlink := filepath.Join(allowed, "linked")
	mustSymlink(t, nestedFile, innerSymlink)
	escapeSymlink := filepath.Join(allowed, "escape")
	mustSymlink(t, outside, escapeSymlink)

	allowedList := []string{allowed}

	tests := []struct {
		name    string
		host    string
		wantErr string
	}{
		{name: "exact match", host: allowed},
		{name: "nested file", host: nestedFile},
		{name: "symlink within allowed", host: innerSymlink},
		{name: "symlink escapes allowed", host: escapeSymlink, wantErr: "not within"},
		{name: "outside allowed", host: outside, wantErr: "not within"},
		{name: "sibling-prefix not allowed", host: sibling, wantErr: "not within"},
		{name: "relative path rejected", host: "relative/path", wantErr: "absolute"},
		{name: "empty rejected", host: "", wantErr: "empty"},
		{name: "dotdot rejected", host: filepath.Join(allowed, "..", "outside"), wantErr: "not within"},
		{name: "missing path rejected", host: filepath.Join(allowed, "does-not-exist"), wantErr: "resolve"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ValidateMountPath(tt.host, allowedList)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil (resolved=%s)", tt.wantErr, got)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.HasPrefix(got, allowed) {
				t.Fatalf("resolved path %s is not under %s", got, allowed)
			}
		})
	}
}

func mustMkdir(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
}

func mustWriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func mustSymlink(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
}
