package codex

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	proxyopenai "github.com/jbeshir/demesne/internal/proxies/openai"
)

func TestCodexBuildArgs_PinCodexVersion(t *testing.T) {
	args, err := imageBuilder.BuildArgs(context.Background())
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"CODEX_VERSION": proxyopenai.CodexVersion}, args)
}

// TestDockerfile_CodexVersionArgHasNoDefault ensures the image can only be
// built with the version supplied by codexBuildArgs.
func TestDockerfile_CodexVersionArgHasNoDefault(t *testing.T) {
	var argLines []string
	for line := range strings.Lines(string(dockerfileBytes)) {
		if strings.HasPrefix(line, "ARG CODEX_VERSION") {
			argLines = append(argLines, strings.TrimSpace(line))
		}
	}
	assert.Equal(t, []string{"ARG CODEX_VERSION"}, argLines)
}
