package sandbox

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testDomain = "localhost:8080"
	testKey    = "key"
)

func TestLoadConfigFromEnv(t *testing.T) {
	outRoot := t.TempDir()

	tests := []struct {
		name         string
		allowedPaths string
		outputRoot   string
		domain       string
		apiKey       string
		wantErr      string
		checkCfg     func(t *testing.T, cfg Config)
	}{
		{
			name:    "missing DEMESNE_ALLOWED_PATHS",
			domain:  testDomain,
			apiKey:  testKey,
			wantErr: "DEMESNE_ALLOWED_PATHS is required",
		},
		{
			name:         "whitespace-only DEMESNE_ALLOWED_PATHS",
			allowedPaths: "  :  ",
			domain:       testDomain,
			apiKey:       testKey,
			wantErr:      "must contain at least one path",
		},
		{
			name:         "missing OPEN_SANDBOX_DOMAIN",
			allowedPaths: outRoot,
			apiKey:       testKey,
			wantErr:      "OPEN_SANDBOX_DOMAIN is required",
		},
		{
			name:         "missing OPEN_SANDBOX_API_KEY",
			allowedPaths: outRoot,
			domain:       testDomain,
			wantErr:      "OPEN_SANDBOX_API_KEY is required",
		},
		{
			name:         "happy path",
			allowedPaths: outRoot,
			outputRoot:   outRoot,
			domain:       testDomain,
			apiKey:       "test-key",
			checkCfg: func(t *testing.T, cfg Config) {
				t.Helper()
				assert.Equal(t, testDomain, cfg.OpenSandboxDomain)
				assert.Equal(t, "test-key", cfg.OpenSandboxAPIKey)
				// Length stays at 1 because the configured path and the output root
				// coincide — the auto-include de-dups them.
				require.Len(t, cfg.AllowedPaths, 1)
				assert.Equal(t, outRoot, cfg.AllowedPaths[0])
				assert.Equal(t, outRoot, cfg.OutputRoot)
			},
		},
		{
			name:         "two-path DEMESNE_ALLOWED_PATHS",
			allowedPaths: "/a:/b",
			outputRoot:   outRoot,
			domain:       testDomain,
			apiKey:       "test-key",
			checkCfg: func(t *testing.T, cfg Config) {
				t.Helper()
				require.Len(t, cfg.AllowedPaths, 3)
				assert.Equal(t, "/a", cfg.AllowedPaths[0])
				assert.Equal(t, "/b", cfg.AllowedPaths[1])
				assert.Equal(t, outRoot, cfg.AllowedPaths[2])
			},
		},
		{
			name:         "output root auto-included when not in DEMESNE_ALLOWED_PATHS",
			allowedPaths: "/a",
			outputRoot:   outRoot,
			domain:       testDomain,
			apiKey:       testKey,
			checkCfg: func(t *testing.T, cfg Config) {
				t.Helper()
				require.Len(t, cfg.AllowedPaths, 2)
				assert.Equal(t, "/a", cfg.AllowedPaths[0])
				assert.Equal(t, outRoot, cfg.AllowedPaths[1])
				assert.Equal(t, outRoot, cfg.OutputRoot)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set every variable explicitly so pre-existing env values don't
			// bleed into the test. t.Setenv restores the original on cleanup.
			t.Setenv("DEMESNE_ALLOWED_PATHS", tt.allowedPaths)
			t.Setenv("DEMESNE_OUTPUT_ROOT", tt.outputRoot)
			t.Setenv("OPEN_SANDBOX_DOMAIN", tt.domain)
			t.Setenv("OPEN_SANDBOX_API_KEY", tt.apiKey)
			t.Setenv("OPEN_SANDBOX_PROTOCOL", "")
			t.Setenv("DEMESNE_CLAUDE_CODE_OAUTH_TOKEN", "")
			t.Setenv("DEMESNE_CODEX_AUTH_FILE", "")

			cfg, err := LoadConfigFromEnv()
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			if tt.checkCfg != nil {
				tt.checkCfg(t, cfg)
			}
		})
	}
}

func TestResolveCodexAuthFile(t *testing.T) {
	t.Run("env var set returns that path", func(t *testing.T) {
		t.Setenv("DEMESNE_CODEX_AUTH_FILE", "/custom/auth.json")
		assert.Equal(t, "/custom/auth.json", resolveCodexAuthFile())
	})

	t.Run("env var unset falls back to home dir", func(t *testing.T) {
		t.Setenv("DEMESNE_CODEX_AUTH_FILE", "")
		got := resolveCodexAuthFile()
		// When a home directory is available the fallback is ~/.codex/auth.json;
		// otherwise the function returns "" (e.g. in containers with no HOME).
		if got != "" {
			assert.True(t, strings.HasSuffix(got, ".codex/auth.json"),
				"expected fallback to end with .codex/auth.json, got %s", got)
		}
	})
}
