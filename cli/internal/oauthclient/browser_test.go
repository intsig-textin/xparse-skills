package oauthclient

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestApplyOpenPolicy(t *testing.T) {
	calls := 0
	opener := func(string) error {
		calls++
		return errors.New("no browser")
	}
	if opened, err := ApplyOpenPolicy("https://example", OpenNever, opener); err != nil || opened || calls != 0 {
		t.Fatalf("never = opened %v, err %v, calls %d", opened, err, calls)
	}
	if opened, err := ApplyOpenPolicy("https://example", OpenAuto, opener); err != nil || opened || calls != 1 {
		t.Fatalf("auto = opened %v, err %v, calls %d", opened, err, calls)
	}
	if opened, err := ApplyOpenPolicy("https://example", OpenAlways, opener); err == nil || opened || calls != 2 {
		t.Fatalf("always = opened %v, err %v, calls %d", opened, err, calls)
	} else if !errors.Is(err, ErrBrowserOpen) {
		t.Fatalf("always error = %v, want ErrBrowserOpen", err)
	}
}

func TestBrowserAvailableTreatsSSHAndCIAsHeadless(t *testing.T) {
	t.Setenv("SSH_CONNECTION", "client server")
	t.Setenv("SSH_TTY", "")
	t.Setenv("CI", "")
	if available, reason := BrowserAvailable(); available || !strings.Contains(reason, "SSH") {
		t.Fatalf("SSH available=%v reason=%q", available, reason)
	}

	t.Setenv("SSH_CONNECTION", "")
	t.Setenv("CI", "true")
	if available, reason := BrowserAvailable(); available || !strings.Contains(reason, "CI") {
		t.Fatalf("CI available=%v reason=%q", available, reason)
	}
}

func TestAuthorizeBrowserPKCELoopback(t *testing.T) {
	occupied8085, err := net.Listen("tcp", "127.0.0.1:8085")
	if err == nil {
		defer occupied8085.Close()
	}
	var mu sync.Mutex
	var expectedChallenge string
	var tokenRequests int
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/oauth21/token" {
			http.NotFound(writer, request)
			return
		}
		if err := request.ParseForm(); err != nil {
			t.Error(err)
		}
		mu.Lock()
		tokenRequests++
		challenge := expectedChallenge
		mu.Unlock()
		if request.Form.Get("grant_type") != "authorization_code" ||
			request.Form.Get("client_id") != "browser-client" ||
			PKCEChallenge(request.Form.Get("code_verifier")) != challenge {
			http.Error(writer, `{"error":"invalid_grant"}`, http.StatusBadRequest)
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"access_token":  "browser-access-private",
			"refresh_token": "browser-refresh-private",
			"token_type":    "Bearer",
			"expires_in":    900,
			"scope":         "ocr:*",
		})
	}))
	defer server.Close()

	opener := func(target string) error {
		authorizeURL, err := url.Parse(target)
		if err != nil {
			return err
		}
		query := authorizeURL.Query()
		if query.Get("code_challenge_method") != "S256" || query.Get("state") == "" ||
			query.Get("prompt") != "consent" {
			return errors.New("missing PKCE/state")
		}
		mu.Lock()
		expectedChallenge = query.Get("code_challenge")
		mu.Unlock()
		callback, err := url.Parse(query.Get("redirect_uri"))
		if err != nil {
			return err
		}
		if callback.Port() == "" || callback.Port() == "0" {
			return errors.New("dynamic loopback port was not resolved")
		}
		callbackQuery := callback.Query()
		callbackQuery.Set("code", "authorization-code")
		callbackQuery.Set("state", query.Get("state"))
		callback.RawQuery = callbackQuery.Encode()
		go func() {
			response, getErr := http.Get(callback.String())
			if getErr == nil {
				_ = response.Body.Close()
			}
		}()
		return nil
	}

	token, err := AuthorizeBrowser(context.Background(), BrowserOptions{
		Client: &Client{
			BaseURL:    server.URL,
			ClientID:   "browser-client",
			Scope:      "ocr:*",
			HTTPClient: server.Client(),
		},
		RedirectURI: "http://127.0.0.1:0/callback",
		Policy:      OpenAlways,
		Opener:      opener,
		Timeout:     2 * time.Second,
		Prompt:      "consent",
	})
	if err != nil {
		t.Fatal(err)
	}
	if token.AccessToken != "browser-access-private" || token.RefreshToken != "browser-refresh-private" {
		t.Fatalf("token = %#v", token)
	}
	mu.Lock()
	defer mu.Unlock()
	if tokenRequests != 1 {
		t.Fatalf("token requests = %d, want 1", tokenRequests)
	}
}

func TestAuthorizeBrowserRejectsWrongState(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		t.Errorf("token endpoint must not be called")
	}))
	defer server.Close()
	opener := func(target string) error {
		authorizeURL, err := url.Parse(target)
		if err != nil {
			return err
		}
		callback, err := url.Parse(authorizeURL.Query().Get("redirect_uri"))
		if err != nil {
			return err
		}
		callback.RawQuery = url.Values{"code": {"code"}, "state": {"wrong"}}.Encode()
		go func() {
			response, getErr := http.Get(callback.String())
			if getErr == nil {
				_ = response.Body.Close()
			}
		}()
		return nil
	}
	_, err := AuthorizeBrowser(context.Background(), BrowserOptions{
		Client:      &Client{BaseURL: server.URL, ClientID: "client", Scope: "ocr:*", HTTPClient: server.Client()},
		RedirectURI: "http://127.0.0.1:0/callback",
		Policy:      OpenAlways,
		Opener:      opener,
		Timeout:     time.Second,
	})
	if err == nil || !strings.Contains(err.Error(), "state validation") {
		t.Fatalf("error = %v", err)
	}
}

func TestAuthorizeBrowserTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		t.Errorf("token endpoint must not be called")
	}))
	defer server.Close()
	_, err := AuthorizeBrowser(context.Background(), BrowserOptions{
		Client:      &Client{BaseURL: server.URL, ClientID: "client", Scope: "ocr:*", HTTPClient: server.Client()},
		RedirectURI: "http://127.0.0.1:0/callback",
		Policy:      OpenNever,
		Timeout:     20 * time.Millisecond,
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v, want deadline exceeded", err)
	}
}
