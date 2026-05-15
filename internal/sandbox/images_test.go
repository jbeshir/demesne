package sandbox

import "testing"

func TestResolveImage(t *testing.T) {
	tests := []struct {
		name    string
		want    string
		wantErr bool
	}{
		{name: "", want: "continuumio/anaconda3:latest"},
		{name: "anaconda", want: "continuumio/anaconda3:latest"},
		{name: "python", want: "python:3.12"},
		{name: "node", want: "node:22"},
		{name: "ubuntu", wantErr: true},
		{name: "Anaconda", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveImage(tt.name)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q, got %s", tt.name, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("ResolveImage(%q) = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}
