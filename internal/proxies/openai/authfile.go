package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// JSON field names inside the auth.json tokens object. Declared as
// constants so the selective-overwrite logic and tests share one spelling.
const (
	fieldIDToken      = "id_token"
	fieldAccessToken  = "access_token"
	fieldRefreshToken = "refresh_token"
)

// RefreshAuthFile reads the Codex auth.json at path, refreshes the token
// set if the id_token is expired/near-expiry (reusing the Credential
// refresh logic), and—if a refresh happened—atomically writes the rotated
// token set back, preserving every other field in the file verbatim. It
// returns the fresh TokenSet (access token + account id) to hand to the
// per-sandbox sidecar proxy.
//
// This is the host-side entry point: refresh+persist happen in the demesne
// host process before each Codex sandbox launch, so auth.json (shared with
// the host's own `codex`) always holds the current rotating refresh token.
// refreshClient is a dedicated HTTP client for the OAuth refresh path:
// 30s timeout so a hung auth.openai.com endpoint can't stall a Codex
// agent launch indefinitely, and isolated from process-wide mutation of
// http.DefaultClient.
var refreshClient = &http.Client{Timeout: 30 * time.Second}

func RefreshAuthFile(ctx context.Context, path string) (TokenSet, error) {
	return refreshAuthFile(ctx, path, oauthRefreshURL, refreshClient)
}

func refreshAuthFile(ctx context.Context, path, tokenURL string, client *http.Client) (TokenSet, error) {
	data, err := os.ReadFile(path) //nolint:gosec // path from config/home, not user-controlled input
	if err != nil {
		return TokenSet{}, fmt.Errorf("cannot read %s (run `codex login`): %w", path, err)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return TokenSet{}, fmt.Errorf("auth file %s is empty; run `codex login`", path)
	}
	ts, err := ParseAuthJSON(data)
	if err != nil {
		return TokenSet{}, fmt.Errorf("invalid auth file %s: %w", path, err)
	}
	if ts.AccessToken == "" || ts.RefreshToken == "" {
		return TokenSet{}, errors.New("codex auth expired or invalid; run `codex login`")
	}
	cred := newCredential(ts, tokenURL, client)
	didRefresh, err := cred.EnsureFresh(ctx)
	if err != nil {
		return TokenSet{}, fmt.Errorf(
			"codex auth refresh failed; the on-disk refresh token may be spent — run `codex login`: %w",
			err,
		)
	}
	fresh := cred.Tokens()
	if didRefresh {
		if err := writeAuthFileAtomic(path, data, fresh); err != nil {
			return TokenSet{}, err
		}
	}
	return fresh, nil
}

// writeAuthFileAtomic updates the token fields in the auth.json at path,
// preserving all other fields verbatim, using an atomic temp-file rename.
func writeAuthFileAtomic(path string, original []byte, ts TokenSet) error {
	top, tokMap, err := parseAuthForRewrite(original)
	if err != nil {
		return err
	}
	if err := applyTokenFields(tokMap, ts); err != nil {
		return err
	}
	rawTokens, err := json.Marshal(tokMap)
	if err != nil {
		return fmt.Errorf("marshal updated tokens: %w", err)
	}
	top["tokens"] = rawTokens
	rawLastRefresh, err := json.Marshal(ts.LastRefresh)
	if err != nil {
		return fmt.Errorf("marshal last_refresh: %w", err)
	}
	top["last_refresh"] = rawLastRefresh
	out, err := json.MarshalIndent(top, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal auth file: %w", err)
	}
	return writeFileAtomic(path, out)
}

// parseAuthForRewrite unmarshals original into a raw-message map and extracts
// the tokens sub-map, returning both for selective field overwrite.
func parseAuthForRewrite(original []byte) (map[string]json.RawMessage, map[string]json.RawMessage, error) {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(original, &top); err != nil {
		return nil, nil, fmt.Errorf("re-parse auth file for rewrite: %w", err)
	}
	tokMap := make(map[string]json.RawMessage)
	if raw, ok := top["tokens"]; ok && string(raw) != "null" {
		if err := json.Unmarshal(raw, &tokMap); err != nil {
			return nil, nil, fmt.Errorf("re-parse tokens for rewrite: %w", err)
		}
	}
	return top, tokMap, nil
}

// applyTokenFields overwrites id_token, access_token, and refresh_token in
// tokMap with the values from ts, leaving all other keys untouched.
func applyTokenFields(tokMap map[string]json.RawMessage, ts TokenSet) error {
	fields := []struct{ k, v string }{
		{fieldIDToken, ts.IDToken},
		{fieldAccessToken, ts.AccessToken},
		{fieldRefreshToken, ts.RefreshToken},
	}
	for _, f := range fields {
		raw, err := json.Marshal(f.v)
		if err != nil {
			return fmt.Errorf("marshal %s: %w", f.k, err)
		}
		tokMap[f.k] = raw
	}
	return nil
}

// writeFileAtomic writes data to a temp file in the same directory as path
// then renames it into place, ensuring atomicity and no leftover temp files.
func writeFileAtomic(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".auth-*.json.tmp")
	if err != nil {
		return fmt.Errorf("create temp auth file: %w", err)
	}
	tmpName := tmp.Name()
	if err := writeAndClose(tmp, data); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("install auth file: %w", err)
	}
	return nil
}

// writeAndClose writes data to f, sets permissions to 0o600, and closes it.
func writeAndClose(f *os.File, data []byte) error {
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return fmt.Errorf("write temp auth file: %w", err)
	}
	if err := f.Chmod(0o600); err != nil {
		_ = f.Close()
		return fmt.Errorf("chmod temp auth file: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close temp auth file: %w", err)
	}
	return nil
}
