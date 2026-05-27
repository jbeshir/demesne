package openai

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeJWT returns a minimal JWT string whose payload encodes {"exp": exp}.
// Only the header.payload.signature form is constructed — the signature
// segment is a dummy value since no verification is performed.
func makeJWT(exp int64) string {
	payload, err := json.Marshal(map[string]int64{"exp": exp})
	if err != nil {
		panic("makeJWT: " + err.Error())
	}
	return "header." + base64.RawURLEncoding.EncodeToString(payload) + ".sig"
}

// freshTokenSet returns a TokenSet with a valid id_token (exp = 1 hour
// from now) and a recent LastRefresh, so needsRefreshLocked returns false.
func freshTokenSet() TokenSet {
	return TokenSet{
		AccessToken:  "access-token-original",
		RefreshToken: "refresh-token-original",
		IDToken:      makeJWT(time.Now().Add(2 * time.Hour).Unix()),
		AccountID:    "acct-123",
		LastRefresh:  time.Now(),
	}
}

// expiredTokenSet returns a TokenSet whose id_token is already expired,
// so needsRefreshLocked returns true.
func expiredTokenSet() TokenSet {
	return TokenSet{
		AccessToken:  "access-token-old",
		RefreshToken: "refresh-token-old",
		IDToken:      makeJWT(time.Now().Add(-1 * time.Hour).Unix()),
		AccountID:    "acct-123",
		LastRefresh:  time.Now(),
	}
}

func TestJwtExpiry_Valid(t *testing.T) {
	want := time.Now().Add(2 * time.Hour).Truncate(time.Second)
	tok := makeJWT(want.Unix())
	got, ok := jwtExpiry(tok)
	require.True(t, ok)
	assert.Equal(t, want.Unix(), got.Unix())
}

func TestJwtExpiry_Malformed(t *testing.T) {
	cases := []string{"", "notajwt", "a.b", "a.!invalid!.c"}
	for _, tc := range cases {
		_, ok := jwtExpiry(tc)
		assert.False(t, ok, "expected ok=false for %q", tc)
	}
}

func TestParseAuthJSON_Full(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	raw := []byte(`{
		"auth_mode": "chatgpt",
		"last_refresh": "` + now.Format(time.RFC3339) + `",
		"tokens": {
			"id_token": "idtok",
			"access_token": "acc",
			"refresh_token": "ref",
			"account_id": "acct-x"
		}
	}`)
	ts, err := ParseAuthJSON(raw)
	require.NoError(t, err)
	assert.Equal(t, "acc", ts.AccessToken)
	assert.Equal(t, "ref", ts.RefreshToken)
	assert.Equal(t, "idtok", ts.IDToken)
	assert.Equal(t, "acct-x", ts.AccountID)
	assert.Equal(t, now.Unix(), ts.LastRefresh.Unix())
}

func TestParseAuthJSON_NullFields(t *testing.T) {
	raw := []byte(`{"tokens":{"id_token":"","access_token":"a","refresh_token":"r","account_id":null},"last_refresh":null}`)
	ts, err := ParseAuthJSON(raw)
	require.NoError(t, err)
	assert.Equal(t, "a", ts.AccessToken)
	assert.Empty(t, ts.AccountID)
	assert.True(t, ts.LastRefresh.IsZero())
}

func TestParseAuthJSON_MalformedJSON(t *testing.T) {
	_, err := ParseAuthJSON([]byte(`{bad json`))
	require.Error(t, err)
}

// TestCredentialRefreshesExpiredToken verifies that AccessToken triggers a
// refresh when the id_token is expired, updates the access and refresh
// tokens, and sets LastRefresh.
func TestCredentialRefreshesExpiredToken(t *testing.T) {
	newAccess := "access-token-new"
	newRefresh := "refresh-token-new"
	newID := makeJWT(time.Now().Add(2 * time.Hour).Unix())
	tokenEndpoint := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		var body refreshRequest
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, oauthClientID, body.ClientID)
		assert.Equal(t, "refresh_token", body.GrantType)
		assert.Equal(t, "refresh-token-old", body.RefreshToken)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(refreshResponse{ //nolint:errcheck,gosec
			AccessToken:  &newAccess,
			RefreshToken: &newRefresh,
			IDToken:      &newID,
		})
	}))
	defer tokenEndpoint.Close()

	ts := expiredTokenSet()
	cred := newCredential(ts, tokenEndpoint.URL, http.DefaultClient)

	before := time.Now()
	tok, err := cred.AccessToken(context.Background())
	require.NoError(t, err)
	assert.Equal(t, newAccess, tok, "must return the refreshed access token")
	assert.Equal(t, newRefresh, cred.tokens.RefreshToken, "refresh token must be rotated")
	assert.False(t, cred.tokens.LastRefresh.IsZero())
	assert.True(t, cred.tokens.LastRefresh.After(before) || cred.tokens.LastRefresh.Equal(before))
}

// TestCredentialNoRefreshWhenFresh verifies that AccessToken does NOT call
// the token endpoint when the credential is fresh.
func TestCredentialNoRefreshWhenFresh(t *testing.T) {
	endpointHit := false
	tokenEndpoint := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		endpointHit = true
		http.Error(w, "should not be called", http.StatusInternalServerError)
	}))
	defer tokenEndpoint.Close()

	cred := newCredential(freshTokenSet(), tokenEndpoint.URL, http.DefaultClient)
	tok, err := cred.AccessToken(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "access-token-original", tok)
	assert.False(t, endpointHit, "token endpoint must not be called for a fresh credential")
}

// TestCredentialRefreshErrorSurfaces verifies that a non-2xx response from
// the token endpoint is returned as an error.
func TestCredentialRefreshErrorSurfaces(t *testing.T) {
	tokenEndpoint := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}))
	defer tokenEndpoint.Close()

	cred := newCredential(expiredTokenSet(), tokenEndpoint.URL, http.DefaultClient)
	_, err := cred.AccessToken(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "401")
}

// TestCredentialZeroLastRefreshTriggersRefresh verifies that a zero
// LastRefresh (newly parsed auth.json with null last_refresh) triggers
// a refresh even if the id_token is fresh.
func TestCredentialZeroLastRefreshTriggersRefresh(t *testing.T) {
	endpointHit := false
	newAccess := "access-fresh"
	tokenEndpoint := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		endpointHit = true
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(refreshResponse{AccessToken: &newAccess}) //nolint:errcheck,gosec
	}))
	defer tokenEndpoint.Close()

	ts := TokenSet{
		AccessToken:  "old",
		RefreshToken: "ref",
		IDToken:      makeJWT(time.Now().Add(2 * time.Hour).Unix()),
		// LastRefresh is zero
	}
	cred := newCredential(ts, tokenEndpoint.URL, http.DefaultClient)
	tok, err := cred.AccessToken(context.Background())
	require.NoError(t, err)
	assert.Equal(t, newAccess, tok)
	assert.True(t, endpointHit)
}
