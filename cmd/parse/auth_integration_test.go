package cmd

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"gitlab.intsig.net/xparse/xparse-client/internal/config"
	"gitlab.intsig.net/xparse/xparse-client/internal/credential"
	"gitlab.intsig.net/xparse/xparse-client/internal/oauthclient"
)

func TestDeviceAutoOpensCompleteURIExactlyOnce(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/oauth21/device/authorization":
			writeJSON(t, writer, map[string]any{
				"device_code": "private-device-code", "user_code": "ABCDE-FGHJK",
				"verification_uri":          serverURL(request) + "/oauth21/device",
				"verification_uri_complete": serverURL(request) + "/oauth21/device?user_code=ABCDE-FGHJK",
				"expires_in":                60, "interval": 1,
			})
		case "/oauth21/token":
			writeJSON(t, writer, map[string]any{
				"access_token": "private-access", "refresh_token": "private-refresh",
				"token_type": "Bearer", "expires_in": 900, "scope": "ocr:*",
			})
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	t.Setenv("XPARSE_CONFIG_DIR", t.TempDir())
	t.Setenv("XPARSE_BASE_URL", server.URL)
	t.Setenv("XPARSE_OAUTH_CLIENT_ID", "fixture-client")
	oldClient, oldOpener := oauthHTTPClient, oauthOpenURL
	oldOutput, oldPolicy, oldClientID, oldScope := authDeviceOutput, authDeviceOpenBrowser, authDeviceClientID, authDeviceScope
	oauthHTTPClient = server.Client()
	var opened []string
	oauthOpenURL = func(target string) error {
		opened = append(opened, target)
		return nil
	}
	authDeviceOutput = "text"
	authDeviceOpenBrowser = "auto"
	authDeviceClientID = ""
	authDeviceScope = ""
	t.Cleanup(func() {
		oauthHTTPClient, oauthOpenURL = oldClient, oldOpener
		authDeviceOutput, authDeviceOpenBrowser, authDeviceClientID, authDeviceScope = oldOutput, oldPolicy, oldClientID, oldScope
	})

	var stdout, stderr bytes.Buffer
	authDeviceCmd.SetOut(&stdout)
	authDeviceCmd.SetErr(&stderr)
	authDeviceCmd.SetContext(context.Background())
	t.Cleanup(func() {
		authDeviceCmd.SetOut(os.Stdout)
		authDeviceCmd.SetErr(os.Stderr)
	})
	if err := runAuthDevice(authDeviceCmd, nil); err != nil {
		t.Fatalf("device auth: %v\nstdout:%s\nstderr:%s", err, stdout.String(), stderr.String())
	}
	if len(opened) != 1 || !strings.Contains(opened[0], "user_code=ABCDE-FGHJK") {
		t.Fatalf("opened = %v", opened)
	}
}

func TestDeviceJSONLErrorSanitizesStdoutAndStderr(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/oauth21/device/authorization":
			writeJSON(t, writer, map[string]any{
				"device_code": "terminal-private-device-code",
				"user_code":   "ABCDE-FGHJK", "verification_uri": serverURL(request) + "/device",
				"expires_in": 60, "interval": 1,
			})
		case "/oauth21/token":
			writer.WriteHeader(http.StatusBadRequest)
			writeJSON(t, writer, map[string]any{
				"error":             "access_denied",
				"error_description": "terminal-private-device-code terminal-private-token terminal-private-secret",
			})
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	home := t.TempDir()
	result := runCLIHelperEnv(t, home,
		"auth device --open-browser never --output jsonl",
		"", map[string]string{
			"XPARSE_CONFIG_DIR":      filepath.Join(home, "isolated"),
			"XPARSE_BASE_URL":        server.URL,
			"XPARSE_OAUTH_CLIENT_ID": "fixture-client",
		})
	if result.err == nil {
		t.Fatal("terminal device error unexpectedly succeeded")
	}
	for _, secret := range []string{"terminal-private-device-code", "terminal-private-token", "terminal-private-secret"} {
		if strings.Contains(result.stdout, secret) || strings.Contains(result.stderr, secret) {
			t.Fatalf("%q leaked\nstdout:%s\nstderr:%s", secret, result.stdout, result.stderr)
		}
	}
	lines := strings.Split(strings.TrimSpace(result.stdout), "\n")
	if len(lines) != 2 || lines[1] != `{"type":"auth_result","status":"error","error":"access_denied"}` {
		t.Fatalf("JSONL error output = %q", result.stdout)
	}
}

func TestDeviceJSONLInitialAuthorizationFailures(t *testing.T) {
	tests := []struct {
		name      string
		handler   http.Handler
		wantError string
	}{
		{
			name: "oauth error",
			handler: http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				writer.Header().Set("Content-Type", "application/json")
				writer.WriteHeader(http.StatusBadRequest)
				_, _ = writer.Write([]byte(
					`{"error":"invalid_client","error_description":"private-initial-secret"}`,
				))
			}),
			wantError: "invalid_client",
		},
		{
			name: "malformed success",
			handler: http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				writer.Header().Set("Content-Type", "application/json")
				_, _ = writer.Write([]byte(`{"device_code":"private-initial-device-code"}`))
			}),
			wantError: "oauth_error",
		},
		{
			name:      "transport error",
			wantError: "transport_error",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var baseURL string
			if test.handler != nil {
				server := httptest.NewServer(test.handler)
				baseURL = server.URL
				defer server.Close()
			} else {
				server := httptest.NewServer(http.NotFoundHandler())
				baseURL = server.URL
				server.Close()
			}

			home := t.TempDir()
			result := runCLIHelperEnv(t, home,
				"auth device --open-browser never --output jsonl",
				"", map[string]string{
					"XPARSE_CONFIG_DIR":      filepath.Join(home, "isolated"),
					"XPARSE_BASE_URL":        baseURL,
					"XPARSE_OAUTH_CLIENT_ID": "fixture-client",
				})
			if result.err == nil {
				t.Fatal("initial Device Authorization failure unexpectedly succeeded")
			}
			want := fmt.Sprintf(
				"{\"type\":\"auth_result\",\"status\":\"error\",\"error\":%q}\n",
				test.wantError,
			)
			if result.stdout != want {
				t.Fatalf("stdout = %q, want %q", result.stdout, want)
			}
			for _, private := range []string{
				"private-initial-secret",
				"private-initial-device-code",
			} {
				if strings.Contains(result.stdout, private) ||
					strings.Contains(result.stderr, private) {
					t.Fatalf("%q leaked\nstdout:%s\nstderr:%s",
						private, result.stdout, result.stderr)
				}
			}
		})
	}
}

func TestDeviceJSONLAndParseAuthenticationModes(t *testing.T) {
	var mu sync.Mutex
	var parseHeaders []http.Header
	var parsePaths []string
	devicePolls := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/oauth21/device/authorization":
			if err := request.ParseForm(); err != nil {
				t.Error(err)
			}
			if request.Form.Get("client_id") != "fixture-client" || request.Form.Get("scope") != "ocr:*" {
				t.Errorf("device form = %v", request.Form)
			}
			writeJSON(t, writer, map[string]any{
				"device_code":               "private-device-code-must-not-leak",
				"user_code":                 "ABCDE-FGHJK",
				"verification_uri":          serverURL(request) + "/oauth21/device",
				"verification_uri_complete": serverURL(request) + "/oauth21/device?user_code=ABCDE-FGHJK",
				"expires_in":                60,
				"interval":                  1,
			})
		case "/oauth21/token":
			if err := request.ParseForm(); err != nil {
				t.Error(err)
			}
			if request.Form.Get("grant_type") != "urn:ietf:params:oauth:grant-type:device_code" ||
				request.Form.Get("device_code") != "private-device-code-must-not-leak" {
				t.Errorf("device token form = %v", request.Form)
			}
			devicePolls++
			writeJSON(t, writer, map[string]any{
				"access_token":  "private-access-token-must-not-leak",
				"refresh_token": "private-refresh-token-must-not-leak",
				"token_type":    "Bearer",
				"expires_in":    900,
				"scope":         "ocr:*",
			})
		case capabilityResolveAPIPath:
			writeJSON(t, writer, map[string]any{
				"code":    200,
				"message": "success",
				"data": map[string]any{
					"snapshot_version":          "parse-capability.v1",
					"supported":                 true,
					"server_preflight_required": false,
					"channels": []map[string]any{{
						"id":                         "free",
						"available":                  true,
						"automatic_use_allowed":      true,
						"requires_user_confirmation": false,
						"creates_new_charge":         false,
						"remaining_pages":            500,
						"max_pages_per_request":      50,
						"max_file_size_bytes":        10 * 1024 * 1024,
						"max_parallel_requests":      2,
					}},
				},
			})
		case paidParseAPIPath, freeParseAPIPath:
			mu.Lock()
			parseHeaders = append(parseHeaders, request.Header.Clone())
			parsePaths = append(parsePaths, request.URL.Path)
			mu.Unlock()
			writeJSON(t, writer, map[string]any{
				"code":    200,
				"message": "success",
				"data":    map[string]any{"markdown": "parsed", "success_count": 1},
			})
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	home := t.TempDir()
	configDir := filepath.Join(home, "workbuddy-credentials")
	environment := map[string]string{
		"XPARSE_CONFIG_DIR":  configDir,
		"XPARSE_BASE_URL":    server.URL,
		"XPARSE_OAUTH_SCOPE": "",
		"XPARSE_AUTH_METHOD": "",
		"XPARSE_APP_ID":      "",
		"XPARSE_SECRET_CODE": "",
	}
	device := runCLIHelperEnv(t, home,
		"auth device --client-id fixture-client --open-browser never --output jsonl",
		"", environment)
	if device.err != nil {
		t.Fatalf("device auth failed: %v\nstdout:\n%s\nstderr:\n%s", device.err, device.stdout, device.stderr)
	}
	if devicePolls != 1 {
		t.Fatalf("device polls = %d, want 1", devicePolls)
	}
	assertNoCredentialLeak(t, device.stdout, device.stderr)
	var events []map[string]any
	scanner := bufio.NewScanner(strings.NewReader(device.stdout))
	for scanner.Scan() {
		var event map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			t.Fatalf("stdout is not JSONL: %q: %v", scanner.Text(), err)
		}
		events = append(events, event)
	}
	if len(events) != 2 || events[0]["type"] != "device_authorization" ||
		events[1]["status"] != "logged_in" || events[1]["method"] != "device" {
		t.Fatalf("events = %#v", events)
	}
	if _, exists := events[0]["device_code"]; exists {
		t.Fatalf("device_code leaked in event: %#v", events[0])
	}
	persisted, err := (&credential.Store{Path: filepath.Join(configDir, "oauth-token.json")}).Load()
	if err != nil {
		t.Fatal(err)
	}
	if persisted.ClientID != "fixture-client" {
		t.Fatalf("flag-selected client ID was not persisted: %#v", persisted)
	}

	status := runCLIHelperEnv(t, home, "auth status --output json", "", environment)
	if status.err != nil || strings.TrimSpace(status.stdout) != `{"logged_in":true,"method":"oauth"}` {
		t.Fatalf("status = stdout %q stderr %q err %v", status.stdout, status.stderr, status.err)
	}

	sample := filepath.Join(home, "sample.pdf")
	writeValidPDF(t, sample)
	oauthParse := runCLIHelperEnv(t, home,
		"parse --api paid --auth-method oauth --view json "+sample,
		"", environment)
	if oauthParse.err != nil {
		t.Fatalf("oauth parse failed: %v\nstdout:%s\nstderr:%s", oauthParse.err, oauthParse.stdout, oauthParse.stderr)
	}
	appKeyEnvironment := cloneEnvironment(environment)
	appKeyEnvironment["XPARSE_APP_ID"] = "fixture-app-id"
	appKeyEnvironment["XPARSE_SECRET_CODE"] = "fixture-secret-code"
	appKeyParse := runCLIHelperEnv(t, home,
		"parse --api paid --auth-method app-key --view json "+sample,
		"", appKeyEnvironment)
	if appKeyParse.err != nil {
		t.Fatalf("appkey parse failed: %v\nstdout:%s\nstderr:%s", appKeyParse.err, appKeyParse.stdout, appKeyParse.stderr)
	}
	freeParse := runCLIHelperEnv(t, home,
		"parse --api free --view json "+sample,
		"", environment)
	if freeParse.err != nil {
		t.Fatalf("free parse failed: %v\nstdout:%s\nstderr:%s", freeParse.err, freeParse.stdout, freeParse.stderr)
	}
	defaultParse := runCLIHelperEnv(t, home,
		"parse --view json "+sample,
		"", appKeyEnvironment)
	if defaultParse.err != nil {
		t.Fatalf("default parse failed: %v\nstdout:%s\nstderr:%s", defaultParse.err, defaultParse.stdout, defaultParse.stderr)
	}
	autoParse := runCLIHelperEnv(t, home,
		"parse --api auto --view json "+sample,
		"", appKeyEnvironment)
	if autoParse.err != nil {
		t.Fatalf("auto parse failed: %v\nstdout:%s\nstderr:%s", autoParse.err, autoParse.stdout, autoParse.stderr)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(parseHeaders) != 5 {
		t.Fatalf("parse requests = %d, want 5", len(parseHeaders))
	}
	if got := parseHeaders[0].Get("Authorization"); got != "Bearer private-access-token-must-not-leak" {
		t.Fatalf("OAuth Authorization = %q", got)
	}
	if parseHeaders[0].Get("x-ti-app-id") != "" || parseHeaders[0].Get("x-ti-secret-code") != "" {
		t.Fatalf("OAuth request leaked AppKey headers: %v", parseHeaders[0])
	}
	if parseHeaders[1].Get("Authorization") != "" ||
		parseHeaders[1].Get("x-ti-app-id") != "fixture-app-id" ||
		parseHeaders[1].Get("x-ti-secret-code") != "fixture-secret-code" {
		t.Fatalf("AppKey headers = %v", parseHeaders[1])
	}
	for index, label := range []string{"explicit free", "default", "auto"} {
		header := parseHeaders[index+2]
		if header.Get("Authorization") != "Bearer private-access-token-must-not-leak" ||
			header.Get("x-ti-app-id") != "" ||
			header.Get("x-ti-secret-code") != "" {
			t.Fatalf("%s request authentication = %v", label, header)
		}
		if got := parsePaths[index+2]; got != freeParseAPIPath {
			t.Fatalf("%s path = %q, want %q", label, got, freeParseAPIPath)
		}
	}
	for index, want := range []string{paidParseAPIPath, paidParseAPIPath} {
		if got := parsePaths[index]; got != want {
			t.Fatalf("paid path[%d] = %q, want %q", index, got, want)
		}
	}
}

func TestExpiredOAuthRefreshRotatesBeforeFreeParseWithoutAppKeyFallback(t *testing.T) {
	var mu sync.Mutex
	refreshRequests := 0
	parseRequests := 0
	var parseHeader http.Header
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/oauth21/token":
			if err := request.ParseForm(); err != nil {
				t.Error(err)
			}
			mu.Lock()
			refreshRequests++
			mu.Unlock()
			if request.Form.Get("grant_type") != "refresh_token" ||
				request.Form.Get("client_id") != "fixture-client" ||
				request.Form.Get("refresh_token") != "old-refresh-private" {
				t.Errorf("refresh form = %v", request.Form)
			}
			writeJSON(t, writer, map[string]any{
				"access_token": "rotated-access-private", "refresh_token": "rotated-refresh-private",
				"token_type": "Bearer", "expires_in": 900, "scope": "ocr:*",
			})
		case freeParseAPIPath:
			mu.Lock()
			parseRequests++
			parseHeader = request.Header.Clone()
			mu.Unlock()
			writeJSON(t, writer, map[string]any{
				"code": 200, "message": "success", "data": map[string]any{"markdown": "parsed"},
			})
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	home := t.TempDir()
	configDir := filepath.Join(home, "isolated")
	store := &credential.Store{Path: filepath.Join(configDir, "oauth-token.json")}
	now := time.Now()
	if err := store.Save(&credential.OAuthToken{
		ClientID:         "fixture-client",
		AccessToken:      "expired-access-private",
		RefreshToken:     "old-refresh-private",
		ExpiresAt:        now.Add(-time.Minute),
		RefreshExpiresAt: now.Add(time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	sample := filepath.Join(home, "sample.pdf")
	writeValidPDF(t, sample)
	environment := map[string]string{
		"XPARSE_CONFIG_DIR":  configDir,
		"XPARSE_BASE_URL":    server.URL,
		"XPARSE_APP_ID":      "fallback-app-must-not-be-used",
		"XPARSE_SECRET_CODE": "fallback-secret-must-not-be-used",
	}
	result := runCLIHelperEnv(t, home,
		"parse --api free --view json "+sample,
		"", environment)
	if result.err != nil {
		t.Fatalf("parse failed: %v\nstdout:%s\nstderr:%s", result.err, result.stdout, result.stderr)
	}
	assertNoCredentialLeak(t, result.stdout, result.stderr)
	mu.Lock()
	defer mu.Unlock()
	if refreshRequests != 1 || parseRequests != 1 {
		t.Fatalf("refresh=%d parse=%d", refreshRequests, parseRequests)
	}
	if parseHeader.Get("Authorization") != "Bearer rotated-access-private" ||
		parseHeader.Get("x-ti-app-id") != "" || parseHeader.Get("x-ti-secret-code") != "" {
		t.Fatalf("parse headers = %v", parseHeader)
	}
	rotated, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if rotated.RefreshToken != "rotated-refresh-private" {
		t.Fatalf("refresh token was not rotated")
	}
}

func TestBrowserPKCETokenParsesWithBearer(t *testing.T) {
	var mu sync.Mutex
	var challenge string
	var parseHeader http.Header
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/oauth21/token":
			if err := request.ParseForm(); err != nil {
				t.Error(err)
			}
			mu.Lock()
			expectedChallenge := challenge
			mu.Unlock()
			if request.Form.Get("grant_type") != "authorization_code" ||
				oauthclient.PKCEChallenge(request.Form.Get("code_verifier")) != expectedChallenge {
				t.Errorf("browser token form = %v", request.Form)
			}
			writeJSON(t, writer, map[string]any{
				"access_token": "browser-access-private", "refresh_token": "browser-refresh-private",
				"token_type": "Bearer", "expires_in": 900, "scope": "ocr:*",
			})
		case paidParseAPIPath:
			mu.Lock()
			parseHeader = request.Header.Clone()
			mu.Unlock()
			writeJSON(t, writer, map[string]any{
				"code": 200, "message": "success", "data": map[string]any{"markdown": "browser parsed"},
			})
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	opener := func(target string) error {
		authorizeURL, err := url.Parse(target)
		if err != nil {
			return err
		}
		query := authorizeURL.Query()
		mu.Lock()
		challenge = query.Get("code_challenge")
		mu.Unlock()
		callback, err := url.Parse(query.Get("redirect_uri"))
		if err != nil {
			return err
		}
		callback.RawQuery = url.Values{
			"code":  {"browser-code"},
			"state": {query.Get("state")},
		}.Encode()
		go func() {
			response, getErr := http.Get(callback.String())
			if getErr == nil {
				_ = response.Body.Close()
			}
		}()
		return nil
	}
	token, err := oauthclient.AuthorizeBrowser(context.Background(), oauthclient.BrowserOptions{
		Client: &oauthclient.Client{
			BaseURL: server.URL, ClientID: "browser-client", Scope: "ocr:*", HTTPClient: server.Client(),
		},
		RedirectURI: "http://127.0.0.1:0/callback",
		Policy:      oauthclient.OpenAlways,
		Opener:      opener,
		Timeout:     time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	sample := filepath.Join(t.TempDir(), "browser.pdf")
	writeValidPDF(t, sample)
	client := &XParserClient{
		BearerToken: token.AccessToken,
		BaseURL:     server.URL,
		ParsePath:   paidParseAPIPath,
		HTTPClient:  server.Client(),
	}
	response, err := client.ParseFile(sample, &ParseOptions{TableView: "html"})
	if err != nil {
		t.Fatal(err)
	}
	if response.Code != 200 || response.GetMarkdown() != "browser parsed" {
		t.Fatalf("parse response = %#v", response)
	}
	mu.Lock()
	defer mu.Unlock()
	if parseHeader.Get("Authorization") != "Bearer browser-access-private" ||
		parseHeader.Get("x-ti-app-id") != "" || parseHeader.Get("x-ti-secret-code") != "" {
		t.Fatalf("browser parse headers = %v", parseHeader)
	}
}

func TestOAuthRefreshFailureOnFreeParseFallsBackToAnonymous(t *testing.T) {
	refreshRequests := 0
	parseRequests := 0
	var parseHeader http.Header
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/oauth21/token":
			refreshRequests++
			writer.Header().Set("Content-Type", "application/json")
			writer.WriteHeader(http.StatusBadRequest)
			_, _ = writer.Write([]byte(`{"error":"invalid_grant","error_description":"expired"}`))
		case freeParseAPIPath:
			parseRequests++
			parseHeader = request.Header.Clone()
			writeJSON(t, writer, map[string]any{
				"code": 200, "message": "success", "data": map[string]any{"markdown": "anonymous parsed"},
			})
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	home := t.TempDir()
	configDir := filepath.Join(home, "isolated")
	store := &credential.Store{Path: filepath.Join(configDir, "oauth-token.json")}
	if err := store.Save(&credential.OAuthToken{
		AccessToken:      "expired-access",
		RefreshToken:     "expired-refresh",
		ExpiresAt:        time.Now().Add(-time.Minute),
		RefreshExpiresAt: time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	sample := filepath.Join(home, "sample.pdf")
	writeValidPDF(t, sample)
	result := runCLIHelperEnv(t, home,
		"parse --api free --view json "+sample,
		"", map[string]string{
			"XPARSE_CONFIG_DIR":      configDir,
			"XPARSE_BASE_URL":        server.URL,
			"XPARSE_OAUTH_CLIENT_ID": "fixture-client",
			"XPARSE_APP_ID":          "fallback-app",
			"XPARSE_SECRET_CODE":     "fallback-secret",
		})
	if result.err != nil {
		t.Fatalf("anonymous fallback failed: %v\nstdout:%s\nstderr:%s",
			result.err, result.stdout, result.stderr)
	}
	if refreshRequests != 1 || parseRequests != 1 {
		t.Fatalf("refresh=%d parse=%d", refreshRequests, parseRequests)
	}
	if parseHeader.Get("Authorization") != "" ||
		parseHeader.Get("x-ti-app-id") != "" ||
		parseHeader.Get("x-ti-secret-code") != "" {
		t.Fatalf("anonymous fallback headers = %v", parseHeader)
	}
	if !strings.Contains(result.stderr,
		"OAuth refresh failed; continuing with anonymous free parse") {
		t.Fatalf("fallback warning missing: %q", result.stderr)
	}
	if strings.Contains(result.stdout+result.stderr, "fallback-secret") ||
		strings.Contains(result.stdout+result.stderr, "expired-refresh") {
		t.Fatalf("credential leaked\nstdout:%s\nstderr:%s", result.stdout, result.stderr)
	}
}

func TestOAuthRefreshFailureOnPaidParseStillFails(t *testing.T) {
	refreshRequests := 0
	parseRequests := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/oauth21/token":
			refreshRequests++
			writer.Header().Set("Content-Type", "application/json")
			writer.WriteHeader(http.StatusBadRequest)
			_, _ = writer.Write([]byte(`{"error":"invalid_grant","error_description":"expired"}`))
		case paidParseAPIPath:
			parseRequests++
			t.Errorf("paid parse must not run after OAuth refresh failure")
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	home := t.TempDir()
	configDir := filepath.Join(home, "isolated")
	store := &credential.Store{Path: filepath.Join(configDir, "oauth-token.json")}
	if err := store.Save(&credential.OAuthToken{
		AccessToken:      "expired-paid-access",
		RefreshToken:     "expired-paid-refresh",
		ExpiresAt:        time.Now().Add(-time.Minute),
		RefreshExpiresAt: time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	sample := filepath.Join(home, "sample.pdf")
	writeValidPDF(t, sample)
	result := runCLIHelperEnv(t, home,
		"parse --api paid --auth-method oauth --view json "+sample,
		"", map[string]string{
			"XPARSE_CONFIG_DIR":      configDir,
			"XPARSE_BASE_URL":        server.URL,
			"XPARSE_OAUTH_CLIENT_ID": "fixture-client",
		})
	if result.err == nil {
		t.Fatal("paid OAuth refresh failure unexpectedly succeeded")
	}
	if refreshRequests != 1 || parseRequests != 0 {
		t.Fatalf("refresh=%d parse=%d", refreshRequests, parseRequests)
	}
	if strings.Contains(result.stdout+result.stderr, "expired-paid-refresh") {
		t.Fatalf("credential leaked\nstdout:%s\nstderr:%s", result.stdout, result.stderr)
	}
}

func TestAuthStatusAndLogoutKeepCredentialStoresIndependent(t *testing.T) {
	home := t.TempDir()
	configDir := filepath.Join(home, "isolated")
	var revokeMu sync.Mutex
	var revokedToken, revokedHint, revokedClientID string
	revokeServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/oauth21/revoke" {
			http.NotFound(writer, request)
			return
		}
		if err := request.ParseForm(); err != nil {
			t.Fatal(err)
		}
		revokeMu.Lock()
		revokedToken = request.Form.Get("token")
		revokedHint = request.Form.Get("token_type_hint")
		revokedClientID = request.Form.Get("client_id")
		revokeMu.Unlock()
		writeJSON(t, writer, map[string]any{})
	}))
	defer revokeServer.Close()
	t.Setenv("XPARSE_CONFIG_DIR", configDir)
	if err := config.Save(&config.Config{
		AppID: "app-id", SecretCode: "secret-code",
		OAuth: config.OAuthConfig{ClientID: "yaml-client"},
	}); err != nil {
		t.Fatal(err)
	}
	store := &credential.Store{Path: filepath.Join(configDir, "oauth-token.json")}
	if err := store.Save(&credential.OAuthToken{
		ClientID:         "flag-client",
		AccessToken:      "expired-access",
		RefreshToken:     "valid-refresh",
		ExpiresAt:        time.Now().Add(-time.Minute),
		RefreshExpiresAt: time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	environment := map[string]string{
		"XPARSE_CONFIG_DIR":  configDir,
		"XPARSE_BASE_URL":    revokeServer.URL,
		"XPARSE_APP_ID":      "",
		"XPARSE_SECRET_CODE": "",
	}
	status := runCLIHelperEnv(t, home, "auth status --output json", "", environment)
	if status.err != nil || strings.TrimSpace(status.stdout) != `{"logged_in":true,"method":"app-key"}` {
		t.Fatalf("status = stdout %q stderr %q err %v", status.stdout, status.stderr, status.err)
	}

	appKeyLogout := runCLIHelperEnv(t, home, "auth logout --method app-key", "", environment)
	if appKeyLogout.err != nil {
		t.Fatalf("app-key logout: %v\n%s", appKeyLogout.err, appKeyLogout.stderr)
	}
	if _, err := store.Load(); err != nil {
		t.Fatalf("AppKey logout removed OAuth token: %v", err)
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AppID != "" || cfg.SecretCode != "" || cfg.OAuth.ClientID != "yaml-client" {
		t.Fatalf("AppKey logout damaged config: %#v", cfg)
	}

	oauthLogout := runCLIHelperEnv(t, home, "auth logout --method oauth", "", environment)
	if oauthLogout.err != nil {
		t.Fatalf("oauth logout: %v\n%s", oauthLogout.err, oauthLogout.stderr)
	}
	if _, err := os.Stat(store.Path); !os.IsNotExist(err) {
		t.Fatalf("OAuth logout did not remove token: %v", err)
	}
	revokeMu.Lock()
	gotRevokedToken, gotRevokedHint, gotRevokedClientID := revokedToken, revokedHint, revokedClientID
	revokeMu.Unlock()
	if gotRevokedToken != "valid-refresh" || gotRevokedHint != "refresh_token" ||
		gotRevokedClientID != "flag-client" {
		t.Fatalf("revoke token=%q hint=%q client=%q",
			gotRevokedToken, gotRevokedHint, gotRevokedClientID)
	}
	cfg, err = config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.OAuth.ClientID != "yaml-client" {
		t.Fatalf("OAuth logout damaged YAML preferences: %#v", cfg)
	}
}

func TestOAuthLoginUsesStablePublicClientID(t *testing.T) {
	home := t.TempDir()
	var receivedClientID string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if err := request.ParseForm(); err != nil {
			t.Fatal(err)
		}
		receivedClientID = request.Form.Get("client_id")
		writeJSON(t, writer, map[string]any{
			"error":             "invalid_client",
			"error_description": "fixture terminal response",
		})
	}))
	defer server.Close()
	result := runCLIHelperEnv(t, home,
		"auth device --open-browser never",
		"", map[string]string{
			"XPARSE_CONFIG_DIR":      filepath.Join(home, "isolated"),
			"XPARSE_OAUTH_CLIENT_ID": "",
			"XPARSE_BASE_URL":        server.URL,
		})
	if result.err == nil {
		t.Fatal("fixture invalid_client unexpectedly succeeded")
	}
	if receivedClientID != config.DefaultOAuthClientID {
		t.Fatalf("client_id = %q, want %q", receivedClientID, config.DefaultOAuthClientID)
	}
}

func assertNoCredentialLeak(t *testing.T, values ...string) {
	t.Helper()
	joined := strings.Join(values, "\n")
	for _, secret := range []string{
		"private-device-code-must-not-leak",
		"private-access-token-must-not-leak",
		"private-refresh-token-must-not-leak",
		"fixture-secret-code",
		"old-refresh-private",
		"rotated-access-private",
		"rotated-refresh-private",
		"fallback-secret-must-not-be-used",
	} {
		if strings.Contains(joined, secret) {
			t.Fatalf("credential %q leaked in output:\n%s", secret, joined)
		}
	}
}

func writeJSON(t *testing.T, writer http.ResponseWriter, value any) {
	t.Helper()
	writer.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(writer).Encode(value); err != nil {
		t.Error(err)
	}
}

func writeValidPDF(t *testing.T, path string) {
	t.Helper()
	var document bytes.Buffer
	document.WriteString("%PDF-1.4\n")
	offsets := make([]int, 4)
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] >>",
	}
	for index, object := range objects {
		offsets[index+1] = document.Len()
		fmt.Fprintf(&document, "%d 0 obj\n%s\nendobj\n", index+1, object)
	}
	xrefOffset := document.Len()
	document.WriteString("xref\n0 4\n0000000000 65535 f \n")
	for index := 1; index <= 3; index++ {
		fmt.Fprintf(&document, "%010d 00000 n \n", offsets[index])
	}
	fmt.Fprintf(
		&document,
		"trailer\n<< /Size 4 /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n",
		xrefOffset,
	)
	if err := os.WriteFile(path, document.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
}

func serverURL(request *http.Request) string {
	return fmt.Sprintf("http://%s", request.Host)
}

func cloneEnvironment(input map[string]string) map[string]string {
	output := make(map[string]string, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}
