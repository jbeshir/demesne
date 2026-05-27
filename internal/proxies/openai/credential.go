package openai

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// OAuth constants for the ChatGPT token endpoint.
const (
	oauthRefreshURL = "https://auth.openai.com/oauth/token"
	// oauthClientID is Codex's public OAuth client_id (shipped in the binary), not a secret.
	oauthClientID    = "app_EMoamEEZ73f0CkXaXp7hrann"
	tokenRefreshDays = 8
	expiryMargin     = 60 * time.Second
)

// TokenSet holds the ChatGPT OAuth token set. Fields match the Codex
// auth.json tokens object and the OAuth refresh response.
type TokenSet struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	IDToken      string    `json:"id_token"`
	AccountID    string    `json:"account_id"`
	LastRefresh  time.Time `json:"last_refresh"`
}

// authJSON is the top-level shape of the Codex auth.json file.
type authJSON struct {
	LastRefresh *time.Time `json:"last_refresh"`
	Tokens      *struct {
		IDToken      string  `json:"id_token"`
		AccessToken  string  `json:"access_token"`
		RefreshToken string  `json:"refresh_token"`
		AccountID    *string `json:"account_id"`
	} `json:"tokens"`
}

// ParseAuthJSON parses the Codex auth.json format (top-level last_refresh
// and tokens object) into a TokenSet. Returns an error only if the JSON is
// malformed; absent or null fields are tolerated and left as zero values.
func ParseAuthJSON(b []byte) (TokenSet, error) {
	var a authJSON
	if err := json.Unmarshal(b, &a); err != nil {
		return TokenSet{}, fmt.Errorf("parse auth.json: %w", err)
	}
	var ts TokenSet
	if a.LastRefresh != nil {
		ts.LastRefresh = *a.LastRefresh
	}
	if a.Tokens != nil {
		ts.IDToken = a.Tokens.IDToken
		ts.AccessToken = a.Tokens.AccessToken
		ts.RefreshToken = a.Tokens.RefreshToken
		if a.Tokens.AccountID != nil {
			ts.AccountID = *a.Tokens.AccountID
		}
	}
	return ts, nil
}

// jwtExpiry decodes the exp claim from an id_token JWT payload segment.
// Returns ok=false if the token is malformed or the claim is absent.
func jwtExpiry(idToken string) (time.Time, bool) {
	parts := strings.Split(idToken, ".")
	if len(parts) < 2 {
		return time.Time{}, false
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return time.Time{}, false
	}
	var claims struct {
		Exp int64 `json:"exp"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil || claims.Exp == 0 {
		return time.Time{}, false
	}
	return time.Unix(claims.Exp, 0), true
}

// Credential holds a ChatGPT OAuth token set. It is used host-side to refresh
// the token set before each Codex launch (see RefreshAuthFile). Safe for
// concurrent use.
type Credential struct {
	mu        sync.Mutex
	tokens    TokenSet
	accountID string // set at construction; not returned by OAuth refresh
	tokenURL  string
	client    *http.Client
}

// newCredential is the test-seam constructor: it accepts an arbitrary
// token URL and http.Client so unit tests can inject a mock endpoint and
// http.DefaultTransport without requiring CAP_NET_ADMIN.
func newCredential(ts TokenSet, tokenURL string, client *http.Client) *Credential {
	return &Credential{
		tokens:    ts,
		accountID: ts.AccountID,
		tokenURL:  tokenURL,
		client:    client,
	}
}

// AccountID returns the account ID established at construction. It is not
// modified by token refresh (the OAuth endpoint does not return account_id).
func (c *Credential) AccountID() string { return c.accountID }

// EnsureFresh refreshes the token set if needed and reports whether a
// refresh actually occurred. Safe for concurrent use.
func (c *Credential) EnsureFresh(ctx context.Context) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.needsRefreshLocked() {
		return false, nil
	}
	if err := c.refreshLocked(ctx); err != nil {
		return false, err
	}
	return true, nil
}

// Tokens returns a snapshot of the current token set. Safe for concurrent use.
func (c *Credential) Tokens() TokenSet {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.tokens
}

// needsRefreshLocked reports whether the token set needs a refresh.
// Caller must hold c.mu.
func (c *Credential) needsRefreshLocked() bool {
	if c.tokens.LastRefresh.IsZero() {
		return true
	}
	if time.Since(c.tokens.LastRefresh) > tokenRefreshDays*24*time.Hour {
		return true
	}
	if exp, ok := jwtExpiry(c.tokens.IDToken); ok {
		if time.Until(exp) < expiryMargin {
			return true
		}
	}
	return false
}

// refreshRequest is the body of the OAuth token refresh POST.
type refreshRequest struct {
	ClientID     string `json:"client_id"`
	GrantType    string `json:"grant_type"`
	RefreshToken string `json:"refresh_token"`
}

// refreshResponse carries the optional fields from the token endpoint.
// All fields are optional — the proxy updates only those present in the
// response, then rotates the refresh token if a new one was returned.
type refreshResponse struct {
	IDToken      *string `json:"id_token"`
	AccessToken  *string `json:"access_token"`
	RefreshToken *string `json:"refresh_token"`
}

// refreshLocked POSTs to the token endpoint and updates the token set.
// Caller must hold c.mu.
func (c *Credential) refreshLocked(ctx context.Context) error {
	body, err := json.Marshal(refreshRequest{ //nolint:gosec // refresh token serialized for the OAuth call, by design
		ClientID:     oauthClientID,
		GrantType:    "refresh_token",
		RefreshToken: c.tokens.RefreshToken,
	})
	if err != nil {
		return fmt.Errorf("openai credential: marshal refresh request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.tokenURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("openai credential: build refresh request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("openai credential: token refresh: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("openai credential: token refresh: upstream returned %s", resp.Status)
	}
	var rr refreshResponse
	if err := json.NewDecoder(resp.Body).Decode(&rr); err != nil {
		return fmt.Errorf("openai credential: decode refresh response: %w", err)
	}
	if rr.IDToken != nil {
		c.tokens.IDToken = *rr.IDToken
	}
	if rr.AccessToken != nil {
		c.tokens.AccessToken = *rr.AccessToken
	}
	// Persist rotated refresh token so subsequent refreshes succeed.
	if rr.RefreshToken != nil {
		c.tokens.RefreshToken = *rr.RefreshToken
	}
	c.tokens.LastRefresh = time.Now()
	return nil
}
