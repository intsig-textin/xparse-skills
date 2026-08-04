package cmd

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/intsig-textin/xparse-skills/cli/internal/config"
	"github.com/intsig-textin/xparse-skills/cli/internal/credential"
)

func TestBareAuthNonTerminalPreservesAppKeySetup(t *testing.T) {
	t.Setenv("XPARSE_CONFIG_DIR", t.TempDir())
	command := &cobra.Command{}
	command.SetIn(strings.NewReader("legacy-app\nlegacy-secret\n"))
	var output bytes.Buffer
	command.SetOut(&output)

	if err := runAuth(command, nil); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AppID != "legacy-app" || cfg.SecretCode != "legacy-secret" ||
		cfg.DefaultAuthMethod != "app-key" {
		t.Fatalf("config = %#v", cfg)
	}
}

func TestAuthMenuCanConfigureAppKey(t *testing.T) {
	t.Setenv("XPARSE_CONFIG_DIR", t.TempDir())
	stubAuthTUIAction(t, authMenuAppKey)
	oldReadAppKey := authTUIReadAppKey
	authTUIReadAppKey = func(*cobra.Command, *config.CredentialSource) (authTUICredentials, error) {
		return authTUICredentials{AppID: "menu-app", SecretCode: "menu-secret"}, nil
	}
	t.Cleanup(func() { authTUIReadAppKey = oldReadAppKey })

	command := &cobra.Command{}
	var output bytes.Buffer
	command.SetOut(&output)

	if err := runAuthMenu(command); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AppID != "menu-app" || cfg.SecretCode != "menu-secret" ||
		cfg.DefaultAuthMethod != "app-key" {
		t.Fatalf("config = %#v", cfg)
	}
	if !strings.Contains(output.String(), "Credentials saved") {
		t.Fatalf("output = %q", output.String())
	}
}

func TestAuthMenuFallsBackToDeviceWhenBrowserLaunchFails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/oauth21/device/authorization":
			writeJSON(t, writer, map[string]any{
				"device_code":      "private-device-code",
				"user_code":        "ABCDE-FGHJK",
				"verification_uri": serverURL(request) + "/oauth21/device",
				"expires_in":       60,
				"interval":         1,
			})
		case "/oauth21/token":
			writeJSON(t, writer, map[string]any{
				"access_token":  "private-access",
				"refresh_token": "private-refresh",
				"token_type":    "Bearer",
				"expires_in":    900,
				"scope":         "ocr:*",
			})
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	t.Setenv("XPARSE_CONFIG_DIR", t.TempDir())
	t.Setenv("XPARSE_BASE_URL", server.URL)
	t.Setenv("XPARSE_OAUTH_CLIENT_ID", "fixture-client")
	stubAuthTUIAction(t, authMenuOAuth)
	oldHTTPClient, oldOpener, oldAvailable := oauthHTTPClient, oauthOpenURL, oauthBrowserAvailable
	oldBrowserClientID, oldBrowserScope := authBrowserClientID, authBrowserScope
	oldBrowserRedirect, oldBrowserPolicy := authBrowserRedirectURI, authBrowserOpenBrowser
	oldBrowserPrompt, oldBrowserTimeout := authBrowserPrompt, authBrowserTimeout
	oldDeviceClientID, oldDeviceScope := authDeviceClientID, authDeviceScope
	oldDevicePolicy, oldDeviceOutput := authDeviceOpenBrowser, authDeviceOutput
	oauthHTTPClient = server.Client()
	oauthOpenURL = func(string) error { return errors.New("fixture opener failure") }
	oauthBrowserAvailable = func() (bool, string) { return true, "" }
	authBrowserClientID = ""
	authBrowserScope = ""
	authBrowserRedirectURI = ""
	authBrowserOpenBrowser = "auto"
	authBrowserPrompt = ""
	authBrowserTimeout = time.Second
	authDeviceClientID = ""
	authDeviceScope = ""
	authDeviceOpenBrowser = "auto"
	authDeviceOutput = "text"
	t.Cleanup(func() {
		oauthHTTPClient, oauthOpenURL, oauthBrowserAvailable = oldHTTPClient, oldOpener, oldAvailable
		authBrowserClientID, authBrowserScope = oldBrowserClientID, oldBrowserScope
		authBrowserRedirectURI, authBrowserOpenBrowser = oldBrowserRedirect, oldBrowserPolicy
		authBrowserPrompt, authBrowserTimeout = oldBrowserPrompt, oldBrowserTimeout
		authDeviceClientID, authDeviceScope = oldDeviceClientID, oldDeviceScope
		authDeviceOpenBrowser, authDeviceOutput = oldDevicePolicy, oldDeviceOutput
	})

	command := &cobra.Command{}
	command.SetContext(context.Background())
	command.SetIn(strings.NewReader("1\n"))
	var stdout, stderr bytes.Buffer
	command.SetOut(&stdout)
	command.SetErr(&stderr)

	if err := runAuthMenu(command); err != nil {
		t.Fatalf("menu OAuth: %v\nstdout:%s\nstderr:%s", err, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "falling back to Device OAuth") {
		t.Fatalf("stderr = %q", stderr.String())
	}
	store, err := credential.DefaultStore()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load(); err != nil {
		t.Fatalf("OAuth token was not saved: %v", err)
	}
	cfg, err := config.Load()
	if err != nil || cfg.DefaultAuthMethod != "oauth" {
		t.Fatalf("config = %#v, err=%v", cfg, err)
	}
}

func TestAuthTUISnapshotShowsTestEnvironmentAndCredentialState(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv("XPARSE_CONFIG_DIR", configDir)
	t.Setenv("XPARSE_BASE_URL", "https://textin-sandbox.intsig.com")
	if err := config.Save(&config.Config{
		AppID:             "configured-app",
		SecretCode:        "configured-secret",
		DefaultAuthMethod: "oauth",
	}); err != nil {
		t.Fatal(err)
	}
	store := &credential.Store{Path: filepath.Join(configDir, "oauth-token.json")}
	if err := store.Save(&credential.OAuthToken{
		ClientID:         "cli_textin_xparse",
		AccessToken:      "private-access",
		RefreshToken:     "private-refresh",
		ExpiresAt:        time.Now().Add(time.Hour),
		RefreshExpiresAt: time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatal(err)
	}

	snapshot, err := loadAuthTUISnapshot(&cobra.Command{})
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Environment != "TEST" || !snapshot.OAuth || !snapshot.AppKey || snapshot.Active != "OAuth" {
		t.Fatalf("snapshot = %#v", snapshot)
	}
	header := renderAuthTUIHeader(snapshot)
	for _, expected := range []string{"TextIn xParse", "TEST", "OAuth signed in", "AppKey configured", "Active OAuth"} {
		if !strings.Contains(header, expected) {
			t.Fatalf("header %q does not contain %q", header, expected)
		}
	}
}

func TestAuthTUIEnvironmentLabels(t *testing.T) {
	for rawURL, want := range map[string]string{
		"https://api.textin.com":                "PRODUCTION",
		"https://textin-sandbox.intsig.com":     "TEST",
		"http://127.0.0.1:8080/custom-endpoint": "CUSTOM",
		"://invalid":                            "CUSTOM",
	} {
		if got := authEnvironmentLabel(rawURL); got != want {
			t.Errorf("authEnvironmentLabel(%q) = %q, want %q", rawURL, got, want)
		}
	}
}

func TestExplicitBrowserAutoFailsFastInHeadlessEnvironment(t *testing.T) {
	oldAvailable := oauthBrowserAvailable
	oldPolicy, oldPrompt := authBrowserOpenBrowser, authBrowserPrompt
	oauthBrowserAvailable = func() (bool, string) { return false, "SSH session" }
	authBrowserOpenBrowser = "auto"
	authBrowserPrompt = ""
	t.Cleanup(func() {
		oauthBrowserAvailable = oldAvailable
		authBrowserOpenBrowser, authBrowserPrompt = oldPolicy, oldPrompt
	})

	command := &cobra.Command{}
	var stderr bytes.Buffer
	command.SetErr(&stderr)
	err := runAuthBrowser(command, nil)
	if err == nil || !strings.Contains(err.Error(), "local browser OAuth is unavailable") {
		t.Fatalf("error = %v", err)
	}
}

func TestOAuthLogoutRemoteFailureStillRemovesLocalToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusServiceUnavailable)
		_, _ = writer.Write([]byte(`{"error":"server_error"}`))
	}))
	defer server.Close()

	configDir := t.TempDir()
	t.Setenv("XPARSE_CONFIG_DIR", configDir)
	t.Setenv("XPARSE_BASE_URL", server.URL)
	if err := config.Save(&config.Config{
		AppID:             "remaining-app",
		SecretCode:        "remaining-secret",
		DefaultAuthMethod: "oauth",
	}); err != nil {
		t.Fatal(err)
	}
	store := &credential.Store{Path: filepath.Join(configDir, "oauth-token.json")}
	if err := store.Save(&credential.OAuthToken{
		ClientID:         "public-client",
		AccessToken:      "private-access",
		RefreshToken:     "private-refresh",
		ExpiresAt:        time.Now().Add(time.Hour),
		RefreshExpiresAt: time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatal(err)
	}

	oldMethod, oldHTTPClient := authLogoutMethod, oauthHTTPClient
	authLogoutMethod = "oauth"
	oauthHTTPClient = server.Client()
	t.Cleanup(func() {
		authLogoutMethod = oldMethod
		oauthHTTPClient = oldHTTPClient
	})
	command := &cobra.Command{}
	command.SetContext(context.Background())
	var stdout, stderr bytes.Buffer
	command.SetOut(&stdout)
	command.SetErr(&stderr)

	if err := runAuthLogout(command, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(store.Path); !os.IsNotExist(err) {
		t.Fatalf("token file still exists: %v", err)
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DefaultAuthMethod != "app-key" {
		t.Fatalf("default method = %q", cfg.DefaultAuthMethod)
	}
	combined := stdout.String() + stderr.String()
	if !strings.Contains(stderr.String(), "remote OAuth revocation failed") ||
		strings.Contains(combined, "private-refresh") ||
		strings.Contains(combined, "private-access") {
		t.Fatalf("stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func TestLogoutAllResetsMalformedConfigAndRemovesOAuthToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{}`))
	}))
	defer server.Close()

	configDir := t.TempDir()
	t.Setenv("XPARSE_CONFIG_DIR", configDir)
	t.Setenv("XPARSE_BASE_URL", server.URL)
	store := &credential.Store{Path: filepath.Join(configDir, "oauth-token.json")}
	if err := store.Save(&credential.OAuthToken{
		ClientID:         "public-client",
		AccessToken:      "private-access",
		RefreshToken:     "private-refresh",
		ExpiresAt:        time.Now().Add(time.Hour),
		RefreshExpiresAt: time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte("app_id: [broken\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	oldMethod, oldHTTPClient := authLogoutMethod, oauthHTTPClient
	authLogoutMethod = "all"
	oauthHTTPClient = server.Client()
	t.Cleanup(func() {
		authLogoutMethod = oldMethod
		oauthHTTPClient = oldHTTPClient
	})
	command := &cobra.Command{}
	command.SetContext(context.Background())
	var stdout, stderr bytes.Buffer
	command.SetOut(&stdout)
	command.SetErr(&stderr)

	if err := runAuthLogout(command, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(store.Path); !os.IsNotExist(err) {
		t.Fatalf("token file still exists: %v", err)
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("reset config is invalid: %v", err)
	}
	if cfg.AppID != "" || cfg.SecretCode != "" || cfg.DefaultAuthMethod != "" {
		t.Fatalf("reset config = %#v", cfg)
	}
	if !strings.Contains(stderr.String(), "malformed authentication preferences were reset") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func stubAuthTUIAction(t *testing.T, action authMenuAction) {
	t.Helper()
	oldSelectAction := authTUISelectAction
	authTUISelectAction = func(*cobra.Command) (authMenuAction, error) {
		return action, nil
	}
	t.Cleanup(func() { authTUISelectAction = oldSelectAction })
}
