package credential

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStoreAtomicPrivateRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "isolated", "oauth-token.json")
	store := &Store{Path: path}
	now := time.Now().Round(time.Second)
	want := &OAuthToken{
		ClientID:         "public-client",
		AccessToken:      "access-private",
		TokenType:        "Bearer",
		RefreshToken:     "refresh-private",
		Scope:            "ocr:*",
		ExpiresAt:        now.Add(time.Hour),
		RefreshExpiresAt: now.Add(30 * 24 * time.Hour),
	}
	if err := store.Save(want); err != nil {
		t.Fatal(err)
	}
	fileInfo, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := fileInfo.Mode().Perm(); got != 0o600 {
		t.Fatalf("token mode = %o, want 600", got)
	}
	dirInfo, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if got := dirInfo.Mode().Perm(); got != 0o700 {
		t.Fatalf("directory mode = %o, want 700", got)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.ClientID != want.ClientID || got.AccessToken != want.AccessToken || got.RefreshToken != want.RefreshToken || !got.ExpiresAt.Equal(want.ExpiresAt) {
		t.Fatalf("round trip = %#v, want %#v", got, want)
	}
}

func TestExpiredAccessWithValidRefreshIsLoggedIn(t *testing.T) {
	now := time.Now()
	token := &OAuthToken{
		AccessToken:      "expired",
		ExpiresAt:        now.Add(-time.Minute),
		RefreshToken:     "refresh",
		RefreshExpiresAt: now.Add(time.Hour),
	}
	if !token.LoggedIn(now) {
		t.Fatal("expired access plus valid refresh must remain logged in")
	}
	token.RefreshExpiresAt = now.Add(-time.Second)
	if token.LoggedIn(now) {
		t.Fatal("expired access plus expired refresh must be logged out")
	}
}
