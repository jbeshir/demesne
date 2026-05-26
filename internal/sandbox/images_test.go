package sandbox

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveImage(t *testing.T) {
	tests := []struct {
		name    string
		want    ImageURI
		wantErr bool
	}{
		{name: "", want: imageAnaconda},
		{name: "anaconda", want: imageAnaconda},
		{name: "python", want: "python:3.12"},
		{name: "node", want: "node:22"},
		{name: "go", want: "golang:1"},
		{name: "ubuntu", wantErr: true},
		{name: "Anaconda", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveImage(tt.name)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
