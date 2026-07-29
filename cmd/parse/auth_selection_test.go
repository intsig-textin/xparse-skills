package cmd

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/intsig-textin/xparse-skills/cli/internal/config"
	"github.com/intsig-textin/xparse-skills/cli/internal/credential"
)

func TestAPIAuthMethodMatrix(t *testing.T) {
	t.Setenv("XPARSE_AUTH_METHOD", "")
	t.Setenv("XPARSE_CONFIG_DIR", t.TempDir())
	now := time.Now()
	oldNow := oauthNow
	oauthNow = func() time.Time { return now }
	t.Cleanup(func() { oauthNow = oldNow })

	store, err := credential.DefaultStore()
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(&credential.OAuthToken{
		AccessToken:  "oauth-access",
		RefreshToken: "oauth-refresh",
		ExpiresAt:    now.Add(time.Hour),
	}); err != nil {
		t.Fatal(err)
	}

	completeAppKey := &config.CredentialSource{AppID: "app", SecretCode: "secret"}
	noAppKey := &config.CredentialSource{}
	tests := []struct {
		name       string
		api        APIMode
		method     string
		appKey     *config.CredentialSource
		cfgMethod  string
		changeApp  bool
		wantFree   bool
		wantMethod authMethod
		wantErr    bool
	}{
		{name: "free anonymous", api: APIModeFree, appKey: completeAppKey, wantFree: true},
		{name: "free rejects explicit method", api: APIModeFree, method: "app-key", appKey: completeAppKey, wantErr: true},
		{name: "paid explicit appkey", api: APIModePaid, method: "app-key", appKey: completeAppKey, wantMethod: authMethodAppKey},
		{name: "paid explicit oauth", api: APIModePaid, method: "oauth", appKey: completeAppKey, wantMethod: authMethodOAuth},
		{name: "paid legacy appkey first", api: APIModePaid, appKey: completeAppKey, wantMethod: authMethodAppKey},
		{name: "paid oauth when no appkey", api: APIModePaid, appKey: noAppKey, wantMethod: authMethodOAuth},
		{name: "auto rejects explicit oauth", api: APIModeAuto, method: "oauth", appKey: completeAppKey, wantErr: true},
		{name: "auto ignores both credentials", api: APIModeAuto, appKey: completeAppKey, wantFree: true},
		{name: "auto ignores oauth only", api: APIModeAuto, appKey: noAppKey, wantFree: true},
		{name: "auto ignores configured method", api: APIModeAuto, appKey: completeAppKey, cfgMethod: "oauth", wantFree: true},
		{name: "oauth rejects appkey flags", api: APIModePaid, method: "oauth", appKey: completeAppKey, changeApp: true, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cmd := parseSelectionCommand(t, test.changeApp)
			got, err := selectParseAuthentication(cmd, test.api, test.method, test.appKey, &config.Config{DefaultAuthMethod: test.cfgMethod})
			if (err != nil) != test.wantErr {
				t.Fatalf("error = %v, wantErr %v", err, test.wantErr)
			}
			if err == nil && (got.IsFree != test.wantFree || got.Method != test.wantMethod) {
				t.Fatalf("selection = %#v, want free=%v method=%q", got, test.wantFree, test.wantMethod)
			}
		})
	}

	if err := store.Remove(); err != nil {
		t.Fatal(err)
	}
	got, err := selectParseAuthentication(parseSelectionCommand(t, false), APIModeAuto, "", noAppKey, &config.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if !got.IsFree || got.Method != "" {
		t.Fatalf("auto without credentials = %#v, want free", got)
	}
}

func TestVersionOutputNormalizesLeadingV(t *testing.T) {
	oldVersion := version
	version = "v2.1.0"
	t.Cleanup(func() { version = oldVersion })
	var output stringsBuilder
	versionCmd.SetOut(&output)
	t.Cleanup(func() { versionCmd.SetOut(os.Stdout) })
	versionCmd.Run(versionCmd, nil)
	if got := output.String(); got != "xparse-cli version 2.1.0\n" {
		t.Fatalf("version output = %q", got)
	}
}

func TestConfigResetPreservesOAuthInIsolatedDirectory(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv("XPARSE_CONFIG_DIR", configDir)
	store, err := credential.DefaultStore()
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(&credential.OAuthToken{AccessToken: "access", ExpiresAt: time.Now().Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	if err := config.Save(&config.Config{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(configDir, "oauth-token.json")); err != nil {
		t.Fatalf("config reset removed OAuth token: %v", err)
	}
}

func parseSelectionCommand(t *testing.T, changed bool) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{}
	cmd.Flags().String("app-id", "", "")
	cmd.Flags().String("secret-code", "", "")
	if changed {
		if err := cmd.Flags().Set("app-id", "flag-app"); err != nil {
			t.Fatal(err)
		}
	}
	return cmd
}

type stringsBuilder struct {
	value []byte
}

func (builder *stringsBuilder) Write(value []byte) (int, error) {
	builder.value = append(builder.value, value...)
	return len(value), nil
}

func (builder *stringsBuilder) String() string {
	return string(builder.value)
}
