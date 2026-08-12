// Package credential stores OAuth credentials separately from the legacy
// AppKey YAML configuration.
package credential

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gitlab.intsig.net/xparse/xparse-client/internal/config"
)

// OAuthToken is the persisted OAuth token set.
type OAuthToken struct {
	ClientID         string    `json:"client_id,omitempty"`
	AccessToken      string    `json:"access_token"`
	TokenType        string    `json:"token_type,omitempty"`
	ExpiresIn        int       `json:"expires_in,omitempty"`
	RefreshToken     string    `json:"refresh_token,omitempty"`
	Scope            string    `json:"scope,omitempty"`
	ExpiresAt        time.Time `json:"expires_at"`
	RefreshExpiresAt time.Time `json:"refresh_expires_at,omitempty"`
}

// Store persists OAuth tokens with private permissions and atomic replacement.
type Store struct {
	Path string
}

// DefaultStore returns the XPARSE_CONFIG_DIR-aware OAuth store.
func DefaultStore() (*Store, error) {
	path, err := config.OAuthTokenPath()
	if err != nil {
		return nil, err
	}
	return &Store{Path: path}, nil
}

// Load reads a token set.
func (s *Store) Load() (*OAuthToken, error) {
	data, err := os.ReadFile(s.Path)
	if err != nil {
		return nil, err
	}
	var token OAuthToken
	if err := json.Unmarshal(data, &token); err != nil {
		return nil, fmt.Errorf("parse OAuth credential: %w", err)
	}
	if token.AccessToken == "" && token.RefreshToken == "" {
		return nil, fmt.Errorf("OAuth credential has no usable token")
	}
	return &token, nil
}

// Save atomically persists a token set using directory 0700 and file 0600.
func (s *Store) Save(token *OAuthToken) (retErr error) {
	dir := filepath.Dir(s.Path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create OAuth credential directory: %w", err)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return fmt.Errorf("secure OAuth credential directory: %w", err)
	}
	data, err := json.MarshalIndent(token, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal OAuth credential: %w", err)
	}
	data = append(data, '\n')

	tmp, err := os.CreateTemp(dir, ".oauth-token.tmp-*")
	if err != nil {
		return fmt.Errorf("create OAuth credential temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() {
		_ = tmp.Close()
		if retErr != nil {
			_ = os.Remove(tmpPath)
		}
	}()
	if err := tmp.Chmod(0o600); err != nil {
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, s.Path); err != nil {
		return fmt.Errorf("replace OAuth credential: %w", err)
	}
	if err := os.Chmod(s.Path, 0o600); err != nil {
		return err
	}
	if dirFile, err := os.Open(dir); err == nil {
		defer dirFile.Close()
		_ = dirFile.Sync()
	}
	return nil
}

// Remove deletes only the OAuth token file.
func (s *Store) Remove() error {
	err := os.Remove(s.Path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// AccessValid reports whether the access token can be used after applying skew.
func (t *OAuthToken) AccessValid(now time.Time, skew time.Duration) bool {
	return t != nil && t.AccessToken != "" && now.Add(skew).Before(t.ExpiresAt)
}

// RefreshValid reports whether a refresh token can restore the session. Older
// files without refresh_expires_at remain compatible and are treated as valid.
func (t *OAuthToken) RefreshValid(now time.Time) bool {
	if t == nil || t.RefreshToken == "" {
		return false
	}
	return t.RefreshExpiresAt.IsZero() || now.Before(t.RefreshExpiresAt)
}

// LoggedIn reports status without performing network I/O.
func (t *OAuthToken) LoggedIn(now time.Time) bool {
	return t.AccessValid(now, 0) || t.RefreshValid(now)
}
