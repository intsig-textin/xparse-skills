package client

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/intsig-textin/xparse-skills/cli/internal/config"
)

func TestXParserClientFromHeader(t *testing.T) {
	if err := config.SetProfile(""); err != nil {
		t.Fatal(err)
	}
	for _, testCase := range []struct {
		name  string
		value string
		want  string
	}{
		{name: "standalone default", want: clientFromCLI},
		{name: "standalone explicit", value: "cli", want: clientFromCLI},
		{name: "workbuddy", value: "workbuddy", want: clientFromWorkBuddy},
		{name: "workbuddy normalized", value: " WorkBuddy ", want: clientFromWorkBuddy},
		{name: "unknown falls back", value: "untrusted-source", want: clientFromCLI},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Setenv(clientFromEnv, testCase.value)
			request := httptest.NewRequest("POST", "https://api.textin.com/api/v1/xparse/parse/sync", nil)
			(&Client{IsFreeAPI: true}).setAuthHeaders(request)
			if got := request.Header.Get("X-From"); got != testCase.want {
				t.Fatalf("X-From = %q, want %q", got, testCase.want)
			}
		})
	}
}

func TestResolveAPIModeRequiresExplicitPaid(t *testing.T) {
	withCredentials := &config.CredentialSource{AppID: "app", SecretCode: "secret"}
	withoutCredentials := &config.CredentialSource{}
	for _, testCase := range []struct {
		name     string
		mode     APIMode
		cred     *config.CredentialSource
		wantFree bool
	}{
		{name: "free without credentials", mode: APIModeFree, cred: withoutCredentials, wantFree: true},
		{name: "free with credentials", mode: APIModeFree, cred: withCredentials, wantFree: true},
		{name: "auto without credentials", mode: APIModeAuto, cred: withoutCredentials, wantFree: true},
		{name: "auto with credentials", mode: APIModeAuto, cred: withCredentials, wantFree: true},
		{name: "paid without credentials", mode: APIModePaid, cred: withoutCredentials, wantFree: false},
		{name: "paid with credentials", mode: APIModePaid, cred: withCredentials, wantFree: false},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			if got := ResolveAPIMode(testCase.mode, testCase.cred); got != testCase.wantFree {
				t.Fatalf("ResolveAPIMode(%q) = %v, want %v", testCase.mode, got, testCase.wantFree)
			}
		})
	}
}

func TestWorkBuddyProfileSetsClientFromHeader(t *testing.T) {
	t.Setenv(clientFromEnv, "")
	if err := config.SetProfile(config.ProfileWorkBuddy); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = config.SetProfile("")
	})

	request := httptest.NewRequest("POST", "https://api.textin.com/api/v1/xparse/parse/sync", nil)
	(&Client{IsFreeAPI: true}).setAuthHeaders(request)
	if got := request.Header.Get("X-From"); got != clientFromWorkBuddy {
		t.Fatalf("X-From = %q, want %q", got, clientFromWorkBuddy)
	}
}

func TestWorkBuddyProfileUsesConfiguredBaseURLForFreeAPI(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XPARSE_CONFIG_DIR", "")
	t.Setenv("XPARSE_BASE_URL", "")
	if err := config.SetProfile(config.ProfileWorkBuddy); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = config.SetProfile("")
	})
	if err := config.Save(&config.Config{BaseURL: "https://sandbox.example"}); err != nil {
		t.Fatal(err)
	}

	got := NewClientWithBearer(nil, &config.CredentialSource{}, "", true, nil)
	if got.BaseURL != "https://sandbox.example" {
		t.Fatalf("BaseURL = %q, want sandbox profile URL", got.BaseURL)
	}
}

func TestXParserClientFromHeaderAcrossAuthModes(t *testing.T) {
	if err := config.SetProfile(""); err != nil {
		t.Fatal(err)
	}
	t.Setenv(clientFromEnv, clientFromWorkBuddy)
	for _, testCase := range []struct {
		name       string
		client     Client
		wantHeader string
		wantValue  string
	}{
		{
			name:       "anonymous free",
			client:     Client{IsFreeAPI: true},
			wantHeader: "Authorization",
		},
		{
			name:       "oauth free",
			client:     Client{IsFreeAPI: true, BearerToken: "access-token"},
			wantHeader: "Authorization",
			wantValue:  "Bearer access-token",
		},
		{
			name:       "oauth",
			client:     Client{BearerToken: "access-token"},
			wantHeader: "Authorization",
			wantValue:  "Bearer access-token",
		},
		{
			name:       "appkey",
			client:     Client{AppID: "app-id", SecretCode: "secret-code"},
			wantHeader: "x-ti-app-id",
			wantValue:  "app-id",
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost,
				"https://api.textin.com/api/v1/xparse/parse/sync", nil)
			testCase.client.setAuthHeaders(request)
			if got := request.Header.Get("X-From"); got != clientFromWorkBuddy {
				t.Fatalf("X-From = %q, want %q", got, clientFromWorkBuddy)
			}
			if got := request.Header.Get(testCase.wantHeader); got != testCase.wantValue {
				t.Fatalf("%s = %q, want %q", testCase.wantHeader, got, testCase.wantValue)
			}
		})
	}
}
