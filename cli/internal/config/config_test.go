package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
)

func TestResolveCredentialsLegacyPrecedence(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XPARSE_CONFIG_DIR", "")
	t.Setenv("XPARSE_APP_ID", "env-app")
	t.Setenv("XPARSE_SECRET_CODE", "env-secret")
	if err := Save(&Config{AppID: "yaml-app", SecretCode: "yaml-secret"}); err != nil {
		t.Fatal(err)
	}

	cmd := credentialTestCommand(t, "flag-app", "flag-secret")
	got, err := ResolveCredentials(cmd)
	if err != nil {
		t.Fatal(err)
	}
	if got.AppID != "flag-app" || got.SecretCode != "flag-secret" || got.Source != "flag" {
		t.Fatalf("flag credentials = %#v", got)
	}

	cmd = credentialTestCommand(t, "partial-flag", "")
	got, err = ResolveCredentials(cmd)
	if err != nil {
		t.Fatal(err)
	}
	if got.AppID != "env-app" || got.SecretCode != "env-secret" || got.Source != "env" {
		t.Fatalf("partial flag must fall back to complete env pair, got %#v", got)
	}

	t.Setenv("XPARSE_SECRET_CODE", "")
	got, err = ResolveCredentials(cmd)
	if err != nil {
		t.Fatal(err)
	}
	if got.AppID != "yaml-app" || got.SecretCode != "yaml-secret" || got.Source != "config" {
		t.Fatalf("partial flag/env must fall back to complete YAML pair, got %#v", got)
	}
}

func TestLegacyConfigPathAndBaseURL(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XPARSE_CONFIG_DIR", "")
	t.Setenv("XPARSE_BASE_URL", "")
	if err := SetProfile(""); err != nil {
		t.Fatal(err)
	}

	if want := filepath.Join(home, ".xparse-cli", "config.yaml"); Path() != want {
		t.Fatalf("Path() = %q, want %q", Path(), want)
	}

	cfg := &Config{BaseURL: "https://yaml.example"}
	if got := GetBaseURL(nil, cfg); got != cfg.BaseURL {
		t.Fatalf("YAML base URL = %q, want %q", got, cfg.BaseURL)
	}

	cmd := &cobra.Command{}
	cmd.Flags().String("base-url", "", "")
	if err := cmd.Flags().Set("base-url", "https://flag.example"); err != nil {
		t.Fatal(err)
	}
	if got := GetBaseURL(cmd, cfg); got != "https://flag.example" {
		t.Fatalf("flag base URL = %q", got)
	}
}

func TestWorkBuddyProfileUsesIsolatedDirectory(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XPARSE_CONFIG_DIR", "")
	if err := SetProfile(ProfileWorkBuddy); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = SetProfile("")
	})

	want := filepath.Join(home, ".xparse-cli", "profiles", "workbuddy", "config.yaml")
	if got := Path(); got != want {
		t.Fatalf("Path() = %q, want %q", got, want)
	}
}

func TestExplicitConfigDirectoryOverridesProfile(t *testing.T) {
	override := t.TempDir()
	t.Setenv("XPARSE_CONFIG_DIR", override)
	if err := SetProfile(ProfileWorkBuddy); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = SetProfile("")
	})

	if got, want := Path(), filepath.Join(override, "config.yaml"); got != want {
		t.Fatalf("Path() = %q, want %q", got, want)
	}
}

func TestRejectsUnsupportedProfile(t *testing.T) {
	if err := SetProfile("../other"); err == nil {
		t.Fatal("SetProfile accepted an unsupported profile")
	}
}

func TestSaveUsesPrivatePermissions(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XPARSE_CONFIG_DIR", "")

	if err := Save(&Config{AppID: "app", SecretCode: "secret"}); err != nil {
		t.Fatal(err)
	}
	path := Path()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("config permissions = %o, want 600", got)
	}
	dirInfo, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if got := dirInfo.Mode().Perm(); got != 0o700 {
		t.Fatalf("config directory permissions = %o, want 700", got)
	}
}

func TestOAuthAndBaseURLPrecedence(t *testing.T) {
	t.Setenv("XPARSE_CONFIG_DIR", t.TempDir())
	cfg := &Config{
		BaseURL: "https://yaml.example/",
		OAuth: OAuthConfig{
			ClientID:    "yaml-client",
			Scope:       "yaml:scope",
			RedirectURI: "http://127.0.0.1:9000/yaml",
		},
	}
	t.Setenv("XPARSE_BASE_URL", "https://env.example/")
	t.Setenv("XPARSE_OAUTH_CLIENT_ID", "env-client")
	t.Setenv("XPARSE_OAUTH_SCOPE", "env:scope")
	t.Setenv("XPARSE_OAUTH_REDIRECT_URI", "http://127.0.0.1:9001/env")

	if got := GetBaseURL(nil, cfg); got != "https://env.example" {
		t.Fatalf("base URL = %q", got)
	}
	if got := ResolveOAuthClientID("", cfg); got != "env-client" {
		t.Fatalf("client ID = %q", got)
	}
	if got := ResolveOAuthScope("", cfg); got != "env:scope" {
		t.Fatalf("scope = %q", got)
	}
	if got := ResolveOAuthRedirectURI("", cfg); got != "http://127.0.0.1:9001/env" {
		t.Fatalf("redirect URI = %q", got)
	}
	if got := ResolveOAuthClientID("flag-client", cfg); got != "flag-client" {
		t.Fatalf("flag client ID = %q", got)
	}
	if got := ResolveOAuthScope("flag:scope", cfg); got != "flag:scope" {
		t.Fatalf("flag scope = %q", got)
	}
	if got := ResolveOAuthRedirectURI("http://127.0.0.1:9002/flag", cfg); got != "http://127.0.0.1:9002/flag" {
		t.Fatalf("flag redirect URI = %q", got)
	}
}

func TestOAuthDefaults(t *testing.T) {
	t.Setenv("XPARSE_OAUTH_CLIENT_ID", "")
	t.Setenv("XPARSE_OAUTH_SCOPE", "")
	t.Setenv("XPARSE_OAUTH_REDIRECT_URI", "")
	if got := ResolveOAuthClientID("", &Config{}); got != DefaultOAuthClientID {
		t.Fatalf("client ID = %q", got)
	}
	if got := ResolveOAuthScope("", &Config{}); got != "ocr:*" {
		t.Fatalf("scope = %q", got)
	}
	if got := ResolveOAuthRedirectURI("", &Config{}); got != "http://127.0.0.1:0/callback" {
		t.Fatalf("redirect URI = %q", got)
	}
}

func credentialTestCommand(t *testing.T, appID, secretCode string) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{}
	cmd.Flags().String("app-id", "", "")
	cmd.Flags().String("secret-code", "", "")
	if err := cmd.Flags().Set("app-id", appID); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Flags().Set("secret-code", secretCode); err != nil {
		t.Fatal(err)
	}
	return cmd
}
