package sandbox

import (
	"os"
	"path/filepath"
	"testing"

	opensandbox "github.com/alibaba/OpenSandbox/sdks/sandbox/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveMounts(t *testing.T) {
	allowedDir := t.TempDir()
	outsideDir := t.TempDir()

	// Create fixtures under the allowed root.
	regularFile := filepath.Join(allowedDir, "file.txt")
	require.NoError(t, os.WriteFile(regularFile, []byte("test"), 0o600))

	subdir := filepath.Join(allowedDir, "subdir")
	require.NoError(t, os.MkdirAll(subdir, 0o750))

	// Two files with identical basenames under separate subdirs (collision pair).
	dir1 := filepath.Join(allowedDir, "dir1")
	dir2 := filepath.Join(allowedDir, "dir2")
	require.NoError(t, os.MkdirAll(dir1, 0o750))
	require.NoError(t, os.MkdirAll(dir2, 0o750))
	collFile1 := filepath.Join(dir1, "foo.txt")
	collFile2 := filepath.Join(dir2, "foo.txt")
	require.NoError(t, os.WriteFile(collFile1, []byte("a"), 0o600))
	require.NoError(t, os.WriteFile(collFile2, []byte("b"), 0o600))

	// File in a separate temp dir — outside AllowedPaths.
	outsideFile := filepath.Join(outsideDir, "outside.txt")
	require.NoError(t, os.WriteFile(outsideFile, []byte("outside"), 0o600))

	// Pre-resolve expected paths in case the temp dir itself contains symlinks.
	wantRegularFile, err := filepath.EvalSymlinks(regularFile)
	require.NoError(t, err)
	wantSubdir, err := filepath.EvalSymlinks(subdir)
	require.NoError(t, err)

	r := NewRunner(Config{AllowedPaths: []string{allowedDir}})

	tests := []struct {
		name      string
		files     []string
		dirs      []string
		wantErr   string
		wantNVols int
		checkVols func(t *testing.T, vols []opensandbox.Volume)
	}{
		{
			name:      "valid file under allowed path",
			files:     []string{regularFile},
			wantNVols: 1,
			checkVols: func(t *testing.T, vols []opensandbox.Volume) {
				t.Helper()
				assert.Equal(t, "/in/file.txt", vols[0].MountPath)
				require.NotNil(t, vols[0].Host)
				assert.Equal(t, wantRegularFile, vols[0].Host.Path)
				assert.True(t, vols[0].ReadOnly)
			},
		},
		{
			name:      "valid directory under allowed path",
			dirs:      []string{subdir},
			wantNVols: 1,
			checkVols: func(t *testing.T, vols []opensandbox.Volume) {
				t.Helper()
				assert.Equal(t, "/in/subdir", vols[0].MountPath)
				require.NotNil(t, vols[0].Host)
				assert.Equal(t, wantSubdir, vols[0].Host.Path)
				assert.True(t, vols[0].ReadOnly)
			},
		},
		{
			name:    "file outside allowed paths",
			files:   []string{outsideFile},
			wantErr: "not within DEMESNE_ALLOWED_PATHS",
		},
		{
			name:    "missing file under allowed path",
			files:   []string{filepath.Join(allowedDir, "nonexistent.txt")},
			wantErr: "no such file",
		},
		{
			name:    "basename collision between two files",
			files:   []string{collFile1, collFile2},
			wantErr: "would collide",
		},
		{
			name:    "directory passed as file",
			files:   []string{subdir},
			wantErr: "is not a regular file",
		},
		{
			name:    "file passed as directory",
			dirs:    []string{regularFile},
			wantErr: "is not a directory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vols, err := r.resolveMounts(tt.files, tt.dirs)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Len(t, vols, tt.wantNVols)
			if tt.checkVols != nil {
				tt.checkVols(t, vols)
			}
		})
	}
}
