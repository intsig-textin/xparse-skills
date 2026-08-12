package oauthclient

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"gitlab.intsig.net/xparse/xparse-client/internal/credential"
)

// OpenPolicy controls whether an authorization URL is opened.
type OpenPolicy string

const (
	OpenAuto   OpenPolicy = "auto"
	OpenAlways OpenPolicy = "always"
	OpenNever  OpenPolicy = "never"
)

// ParseOpenPolicy validates a browser policy.
func ParseOpenPolicy(value string) (OpenPolicy, error) {
	switch OpenPolicy(value) {
	case OpenAuto, OpenAlways, OpenNever:
		return OpenPolicy(value), nil
	default:
		return "", fmt.Errorf("invalid --open-browser value %q, must be auto, always, or never", value)
	}
}

// OpenURL opens a URL using the platform browser.
type OpenURL func(string) error

// ErrBrowserOpen identifies a local browser launch failure.
var ErrBrowserOpen = errors.New("browser open failed")

// BrowserAvailable reports whether the current process appears to have a local
// graphical browser session. It intentionally treats SSH and CI as headless.
func BrowserAvailable() (bool, string) {
	if os.Getenv("SSH_CONNECTION") != "" || os.Getenv("SSH_TTY") != "" {
		return false, "SSH session"
	}
	switch strings.ToLower(strings.TrimSpace(os.Getenv("CI"))) {
	case "1", "true", "yes":
		return false, "CI environment"
	}
	if runtime.GOOS == "linux" &&
		os.Getenv("DISPLAY") == "" &&
		os.Getenv("WAYLAND_DISPLAY") == "" {
		return false, "Linux display is unavailable"
	}
	var command string
	switch runtime.GOOS {
	case "darwin":
		command = "open"
	case "windows":
		command = "rundll32"
	default:
		command = "xdg-open"
	}
	if _, err := exec.LookPath(command); err != nil {
		return false, command + " is unavailable"
	}
	return true, ""
}

// SystemOpenURL opens a URL with the platform default application.
func SystemOpenURL(target string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var command *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		command = exec.CommandContext(ctx, "open", target)
	case "windows":
		command = exec.CommandContext(ctx, "rundll32", "url.dll,FileProtocolHandler", target)
	default:
		command = exec.CommandContext(ctx, "xdg-open", target)
	}
	if err := command.Run(); err != nil {
		return fmt.Errorf("%w: %v", ErrBrowserOpen, err)
	}
	return nil
}

// ApplyOpenPolicy attempts to open once. Auto reports the error to the caller
// without making it fatal; always returns it as fatal.
func ApplyOpenPolicy(target string, policy OpenPolicy, opener OpenURL) (opened bool, err error) {
	if policy == OpenNever {
		return false, nil
	}
	if opener == nil {
		opener = SystemOpenURL
	}
	if err := opener(target); err != nil {
		if policy == OpenAlways {
			if errors.Is(err, ErrBrowserOpen) {
				return false, err
			}
			return false, fmt.Errorf("%w: %v", ErrBrowserOpen, err)
		}
		return false, nil
	}
	return true, nil
}

// BrowserOptions configures Authorization Code + PKCE.
type BrowserOptions struct {
	Client      *Client
	RedirectURI string
	Policy      OpenPolicy
	Opener      OpenURL
	Timeout     time.Duration
	Prompt      string
	OnURL       func(string)
}

// AuthorizeBrowser runs a loopback Authorization Code + PKCE flow.
func AuthorizeBrowser(ctx context.Context, options BrowserOptions) (*credential.OAuthToken, error) {
	if options.Client == nil {
		return nil, errors.New("OAuth client is required")
	}
	redirect, err := url.Parse(options.RedirectURI)
	if err != nil {
		return nil, fmt.Errorf("invalid redirect URI: %w", err)
	}
	if redirect.Scheme != "http" || redirect.Hostname() != "127.0.0.1" || redirect.User != nil {
		return nil, errors.New("redirect URI must use http://127.0.0.1 loopback")
	}
	if redirect.Path == "" {
		redirect.Path = "/callback"
	}
	listener, err := net.Listen("tcp", redirect.Host)
	if err != nil {
		return nil, fmt.Errorf("listen for OAuth callback: %w", err)
	}
	defer listener.Close()
	if redirect.Port() == "0" {
		redirect.Host = listener.Addr().String()
	}
	actualRedirect := redirect.String()

	verifier, err := randomURLSafe(48)
	if err != nil {
		return nil, err
	}
	state, err := randomURLSafe(32)
	if err != nil {
		return nil, err
	}
	challenge := PKCEChallenge(verifier)

	authorizeURL, err := url.Parse(strings.TrimRight(options.Client.BaseURL, "/") + "/oauth21/authorize")
	if err != nil {
		return nil, err
	}
	query := authorizeURL.Query()
	query.Set("response_type", "code")
	query.Set("client_id", options.Client.ClientID)
	query.Set("redirect_uri", actualRedirect)
	query.Set("scope", options.Client.Scope)
	query.Set("state", state)
	query.Set("code_challenge", challenge)
	query.Set("code_challenge_method", "S256")
	if options.Prompt != "" {
		query.Set("prompt", options.Prompt)
	}
	authorizeURL.RawQuery = query.Encode()

	type callbackResult struct {
		code string
		err  error
	}
	resultCh := make(chan callbackResult, 1)
	mux := http.NewServeMux()
	mux.HandleFunc(redirect.Path, func(writer http.ResponseWriter, request *http.Request) {
		send := func(result callbackResult) {
			select {
			case resultCh <- result:
			default:
			}
		}
		if request.URL.Query().Get("state") != state {
			http.Error(writer, "OAuth state validation failed. You may close this page.", http.StatusBadRequest)
			send(callbackResult{err: errors.New("OAuth state validation failed")})
			return
		}
		if oauthCode := request.URL.Query().Get("error"); oauthCode != "" {
			http.Error(writer, "OAuth authorization failed. You may close this page.", http.StatusBadRequest)
			send(callbackResult{err: &OAuthError{
				Code:        oauthCode,
				Description: request.URL.Query().Get("error_description"),
			}})
			return
		}
		code := request.URL.Query().Get("code")
		if code == "" {
			http.Error(writer, "Missing authorization code. You may close this page.", http.StatusBadRequest)
			send(callbackResult{err: errors.New("OAuth callback missing authorization code")})
			return
		}
		writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(writer, "<!doctype html><meta charset=\"utf-8\"><title>TextIn login complete</title><p>Login complete. You may close this page.</p>")
		send(callbackResult{code: code})
	})
	server := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	serverErrCh := make(chan error, 1)
	go func() {
		if serveErr := server.Serve(listener); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			serverErrCh <- serveErr
		}
	}()
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	target := authorizeURL.String()
	if options.OnURL != nil {
		options.OnURL(target)
	}
	if _, err := ApplyOpenPolicy(target, options.Policy, options.Opener); err != nil {
		return nil, err
	}

	timeout := options.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	var result callbackResult
	select {
	case result = <-resultCh:
	case serveErr := <-serverErrCh:
		return nil, fmt.Errorf("OAuth callback server: %w", serveErr)
	case <-waitCtx.Done():
		return nil, waitCtx.Err()
	}
	if result.err != nil {
		return nil, result.err
	}
	return options.Client.ExchangeAuthorizationCode(ctx, result.code, actualRedirect, verifier)
}

// PKCEChallenge computes an RFC 7636 S256 challenge.
func PKCEChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func randomURLSafe(size int) (string, error) {
	value := make([]byte, size)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("generate secure random value: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}
