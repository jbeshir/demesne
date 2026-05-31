package openai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// expiredAuthJSON returns auth.json bytes with an expired id_token and all
// extra fields needed for field-preservation tests (OPENAI_API_KEY, auth_mode,
// unknown top-level field, account_id, unknown token field).
func expiredAuthJSON(t *testing.T) []byte {
	t.Helper()
	data, err := json.MarshalIndent(map[string]interface{}{
		"OPENAI_API_KEY":   "sk-preserve-me",
		"auth_mode":        "chatgpt",
		"some_unknown_top": map[string]interface{}{"x": 1},
		"last_refresh":     "2020-01-01T00:00:00Z",
		"tokens": map[string]interface{}{
			fieldIDToken:      makeJWT(time.Now().Add(-1 * time.Hour).Unix()),
			fieldAccessToken:  "old-access",
			fieldRefreshToken: "old-refresh",
			"account_id":      "acct-keep",
			"unknown_tok":     "keep",
		},
	}, "", "  ")
	require.NoError(t, err)
	return data
}

// newRefreshSrv returns a test server that responds with the given new token values.
func newRefreshSrv(t *testing.T, newAccess, newRefresh, newID string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(refreshResponse{ //nolint:errcheck,gosec // test handler; Encode to the httptest ResponseWriter can't fail
			AccessToken:  &newAccess,
			RefreshToken: &newRefresh,
			IDToken:      &newID,
		})
	}))
}

// readFileRaw parses a JSON file into a raw top-level map and its tokens sub-map.
func readFileRaw(t *testing.T, path string) (top map[string]json.RawMessage, tokens map[string]json.RawMessage) {
	t.Helper()
	raw, err := os.ReadFile(path) //nolint:gosec // path is a test temp dir
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(raw, &top))
	require.NoError(t, json.Unmarshal(top["tokens"], &tokens))
	return top, tokens
}

// unmarshalStr decodes a json.RawMessage that must be a JSON string.
func unmarshalStr(t *testing.T, raw json.RawMessage) string {
	t.Helper()
	var s string
	require.NoError(t, json.Unmarshal(raw, &s))
	return s
}

// TestRefreshAuthFile_RotatesRefreshToken verifies that an expired token
// causes a refresh, the returned TokenSet has the new access token, and
// the on-disk tokens are all updated with the rotated values.
func TestRefreshAuthFile_RotatesRefreshToken(t *testing.T) {
	newAccess := "new-access-tok"
	newRefresh := "new-refresh-tok"
	newID := makeJWT(time.Now().Add(2 * time.Hour).Unix())
	srv := newRefreshSrv(t, newAccess, newRefresh, newID)
	defer srv.Close()

	dir := t.TempDir()
	path := dir + "/auth.json"
	require.NoError(t, os.WriteFile(path, expiredAuthJSON(t), 0o600))

	ts, err := refreshAuthFile(context.Background(), path, srv.URL, http.DefaultClient)
	require.NoError(t, err)
	assert.Equal(t, newAccess, ts.AccessToken)

	top, tokens := readFileRaw(t, path)
	assert.Equal(t, newAccess, unmarshalStr(t, tokens["access_token"]))
	assert.Equal(t, newRefresh, unmarshalStr(t, tokens["refresh_token"]))
	assert.Equal(t, newID, unmarshalStr(t, tokens["id_token"]))

	var lastRefresh time.Time
	require.NoError(t, json.Unmarshal(top["last_refresh"], &lastRefresh))
	assert.True(t, lastRefresh.After(time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC)),
		"last_refresh must be updated from the 2020 value")
}

// TestRefreshAuthFile_PreservesOtherFields verifies that OPENAI_API_KEY,
// auth_mode, unknown top-level fields, account_id, and unknown token fields
// are all preserved verbatim after a refresh+write.
func TestRefreshAuthFile_PreservesOtherFields(t *testing.T) {
	newAccess := "new-access-2"
	newRefresh := "new-refresh-2"
	newID := makeJWT(time.Now().Add(2 * time.Hour).Unix())
	srv := newRefreshSrv(t, newAccess, newRefresh, newID)
	defer srv.Close()

	dir := t.TempDir()
	path := dir + "/auth.json"
	require.NoError(t, os.WriteFile(path, expiredAuthJSON(t), 0o600))

	_, err := refreshAuthFile(context.Background(), path, srv.URL, http.DefaultClient)
	require.NoError(t, err)

	top, tokens := readFileRaw(t, path)

	assert.Equal(t, "sk-preserve-me", unmarshalStr(t, top["OPENAI_API_KEY"]))
	assert.Equal(t, "chatgpt", unmarshalStr(t, top["auth_mode"]))

	var unk map[string]interface{}
	require.NoError(t, json.Unmarshal(top["some_unknown_top"], &unk))
	assert.Equal(t, map[string]interface{}{"x": float64(1)}, unk)

	assert.Equal(t, "acct-keep", unmarshalStr(t, tokens["account_id"]))
	assert.Equal(t, "keep", unmarshalStr(t, tokens["unknown_tok"]))
}

// TestRefreshAuthFile_NoTempDebris verifies that no .tmp files remain in
// the auth.json directory after a successful refresh+write.
func TestRefreshAuthFile_NoTempDebris(t *testing.T) {
	newAccess := "new-access-3"
	newRefresh := "new-refresh-3"
	newID := makeJWT(time.Now().Add(2 * time.Hour).Unix())
	srv := newRefreshSrv(t, newAccess, newRefresh, newID)
	defer srv.Close()

	dir := t.TempDir()
	path := dir + "/auth.json"
	require.NoError(t, os.WriteFile(path, expiredAuthJSON(t), 0o600))

	_, err := refreshAuthFile(context.Background(), path, srv.URL, http.DefaultClient)
	require.NoError(t, err)

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Len(t, entries, 1, "only auth.json should exist in the directory")
	for _, e := range entries {
		assert.False(t, strings.HasSuffix(e.Name(), ".tmp"), "no .tmp files should remain: %s", e.Name())
	}
}

// TestRefreshAuthFile_NoRefreshWhenFresh verifies that a fresh auth.json
// does not hit the token endpoint, does not overwrite the file, and returns
// the on-disk access token.
func TestRefreshAuthFile_NoRefreshWhenFresh(t *testing.T) {
	endpointHit := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		endpointHit = true
		t.Errorf("token endpoint must not be called for a fresh credential")
		http.Error(w, "must not be called", http.StatusInternalServerError)
	}))
	defer srv.Close()

	dir := t.TempDir()
	path := dir + "/auth.json"

	fs := freshTokenSet()
	raw, err := json.MarshalIndent(map[string]interface{}{
		"OPENAI_API_KEY": "sk-fresh",
		"auth_mode":      "chatgpt",
		"last_refresh":   fs.LastRefresh.UTC().Format(time.RFC3339),
		"tokens": map[string]interface{}{
			fieldIDToken:      fs.IDToken,
			fieldAccessToken:  fs.AccessToken,
			fieldRefreshToken: fs.RefreshToken,
			"account_id":      fs.AccountID,
		},
	}, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, raw, 0o600))

	beforeBytes, err := os.ReadFile(path) //nolint:gosec // path is a test temp dir
	require.NoError(t, err)

	ts, err := refreshAuthFile(context.Background(), path, srv.URL, http.DefaultClient)
	require.NoError(t, err)
	assert.Equal(t, fs.AccessToken, ts.AccessToken)
	assert.False(t, endpointHit, "token endpoint must not be hit for a fresh credential")

	afterBytes, err := os.ReadFile(path) //nolint:gosec // path is a test temp dir
	require.NoError(t, err)
	assert.Equal(t, beforeBytes, afterBytes, "file must not be modified when no refresh occurs")
}

// TestRefreshAuthFile_Errors verifies actionable error messages for the
// three main failure modes.
func TestRefreshAuthFile_Errors(t *testing.T) {
	t.Run("refresh_fail_401", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
		}))
		defer srv.Close()

		dir := t.TempDir()
		path := dir + "/auth.json"
		require.NoError(t, os.WriteFile(path, expiredAuthJSON(t), 0o600))

		_, err := refreshAuthFile(context.Background(), path, srv.URL, http.DefaultClient)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "codex login")
	})

	t.Run("missing_file", func(t *testing.T) {
		_, err := refreshAuthFile(context.Background(), "/nonexistent/path/auth.json", "http://unused", http.DefaultClient)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "codex login")
	})

	t.Run("empty_file", func(t *testing.T) {
		dir := t.TempDir()
		path := dir + "/auth.json"
		require.NoError(t, os.WriteFile(path, []byte{}, 0o600))

		_, err := refreshAuthFile(context.Background(), path, "http://unused", http.DefaultClient)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "codex login")
	})
}
