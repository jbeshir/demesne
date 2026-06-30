package sandbox

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStaticImageURI(t *testing.T) {
	tests := []struct {
		name    string
		want    ImageURI
		wantErr bool
	}{
		{name: "", want: imageAnaconda},
		{name: DefaultImage, want: imageAnaconda},
		{name: imagePython, want: "python:3.12"},
		{name: imageNode, want: "node:22"},
		{name: imageGo, want: "golang:1"},
		// browser is a locally-built image, not a static one; the runner
		// routes it to its builder before staticImageURI is reached.
		{name: imageBrowser, wantErr: true},
		// media is a locally-built image, not a static one; the runner
		// routes it to its builder before staticImageURI is reached.
		{name: imageMedia, wantErr: true},
		// twine is a locally-built image, not a static one; the runner
		// routes it to its builder before staticImageURI is reached.
		{name: imageTwine, wantErr: true},
		// webgamedev is a locally-built image, not a static one; the runner
		// routes it to its builder before staticImageURI is reached.
		{name: imageWebgamedev, wantErr: true},
		{name: "ubuntu", wantErr: true},
		{name: "Anaconda", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := staticImageURI(tt.name)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
