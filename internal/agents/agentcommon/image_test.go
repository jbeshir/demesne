package agentcommon

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestImageTag_NoArgsMatchesDockerfileHash pins the backward-compatible
// contract: with no build args the tag is sha256(Dockerfile)[:12], so
// existing cached images for builders that don't use build args keep their
// tag.
func TestImageTag_NoArgsMatchesDockerfileHash(t *testing.T) {
	df := []byte("FROM scratch\n")
	sum := sha256.Sum256(df)
	want := hex.EncodeToString(sum[:])[:12]
	assert.Equal(t, want, imageTag(df, nil))
	assert.Equal(t, want, imageTag(df, map[string]string{}))
}

// TestImageTag_VariesWithBuildArgs asserts a build-arg value change forces a
// new tag (so the image rebuilds), while equal args are stable and key order
// is irrelevant.
func TestImageTag_VariesWithBuildArgs(t *testing.T) {
	const verArg = "CLAUDE_CODE_VERSION"
	df := []byte("FROM node:22-slim\n")
	v1 := imageTag(df, map[string]string{verArg: "2.1.0"})
	v2 := imageTag(df, map[string]string{verArg: "2.2.0"})
	assert.NotEqual(t, v1, v2, "different version must change the tag")

	again := imageTag(df, map[string]string{verArg: "2.1.0"})
	assert.Equal(t, v1, again, "same args must produce the same tag")

	assert.NotEqual(t, imageTag(df, nil), v1, "adding an arg must change the tag")

	// Key order independence.
	a := imageTag(df, map[string]string{"A": "1", "B": "2"})
	b := imageTag(df, map[string]string{"B": "2", "A": "1"})
	assert.Equal(t, a, b)
}
