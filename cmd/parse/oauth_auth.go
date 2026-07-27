package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/intsig-textin/xparse-skills/cli/internal/config"
	"github.com/intsig-textin/xparse-skills/cli/internal/credential"
	"github.com/intsig-textin/xparse-skills/cli/internal/exitcode"
	"github.com/intsig-textin/xparse-skills/cli/internal/oauthclient"
)

var (
	authDeviceClientID    string
	authDeviceScope       string
	authDeviceOpenBrowser string
	authDeviceOutput      string

	authBrowserClientID    string
	authBrowserScope       string
	authBrowserRedirectURI string
	authBrowserOpenBrowser string
	authBrowserPrompt      string
	authBrowserTimeout     time.Duration

	authStatusOutput string
	authLogoutMethod string
	authAppKeyShow   bool

	oauthHTTPClient       = &http.Client{Timeout: 30 * time.Second}
	oauthOpenURL          = oauthclient.SystemOpenURL
	oauthBrowserAvailable = oauthclient.BrowserAvailable
	oauthNow              = time.Now
)

var authAppKeyCmd = &cobra.Command{
	Use:   "app-key",
	Short: "Configure legacy App ID and Secret Code credentials",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if authAppKeyShow {
			return runAuthShow()
		}
		return runAuthSetupIO(cmd.InOrStdin(), cmd.OutOrStdout())
	},
}

var authDeviceCmd = &cobra.Command{
	Use:   "device",
	Short: "Log in with the OAuth Device Authorization Grant",
	Args:  cobra.NoArgs,
	RunE:  runAuthDevice,
}

var authBrowserCmd = &cobra.Command{
	Use:   "browser",
	Short: "Log in with Authorization Code + PKCE and a loopback callback",
	Args:  cobra.NoArgs,
	RunE:  runAuthBrowser,
}

var authStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show authentication status without network requests",
	Args:  cobra.NoArgs,
	RunE:  runAuthStatus,
}

var authLogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Remove OAuth, AppKey, or all credentials",
	Args:  cobra.NoArgs,
	RunE:  runAuthLogout,
}

func init() {
	authCmd.AddCommand(authAppKeyCmd, authDeviceCmd, authBrowserCmd, authStatusCmd, authLogoutCmd)
	authAppKeyCmd.Flags().BoolVar(&authAppKeyShow, "show", false, "Show current AppKey credential source")

	authDeviceCmd.Flags().StringVar(&authDeviceClientID, "client-id", "", "OAuth public client ID")
	authDeviceCmd.Flags().StringVar(&authDeviceScope, "scope", "", "OAuth scope (default: ocr:*)")
	authDeviceCmd.Flags().StringVar(&authDeviceOpenBrowser, "open-browser", "auto", "Browser policy: auto, always, never")
	authDeviceCmd.Flags().StringVar(&authDeviceOutput, "output", "text", "Output format: text, jsonl")

	authBrowserCmd.Flags().StringVar(&authBrowserClientID, "client-id", "", "OAuth public client ID")
	authBrowserCmd.Flags().StringVar(&authBrowserScope, "scope", "", "OAuth scope (default: ocr:*)")
	authBrowserCmd.Flags().StringVar(&authBrowserRedirectURI, "redirect-uri", "", "OAuth loopback redirect URI")
	authBrowserCmd.Flags().StringVar(&authBrowserOpenBrowser, "open-browser", "auto", "Browser policy: auto, always, never")
	authBrowserCmd.Flags().StringVar(&authBrowserPrompt, "prompt", "", "Authorization prompt: consent")
	authBrowserCmd.Flags().DurationVar(&authBrowserTimeout, "timeout", 5*time.Minute, "Maximum time to wait for the browser callback")

	authStatusCmd.Flags().StringVar(&authStatusOutput, "output", "text", "Output format: text, json")
	authLogoutCmd.Flags().StringVar(&authLogoutMethod, "method", "", "Credential to remove: oauth, app-key, all")
}

func runAuthDevice(cmd *cobra.Command, args []string) error {
	if authDeviceOutput != "text" && authDeviceOutput != "jsonl" {
		return usageErr("invalid --output value, must be text or jsonl", "[fix] use --output text or --output jsonl")
	}
	policy, err := oauthclient.ParseOpenPolicy(authDeviceOpenBrowser)
	if err != nil {
		return usageErr(err.Error(), "[fix] use --open-browser auto, always, or never")
	}
	if policy == oauthclient.OpenAuto {
		if available, _ := oauthBrowserAvailable(); !available {
			policy = oauthclient.OpenNever
		}
	}
	client, _, err := resolvedOAuthClient(cmd, authDeviceClientID, authDeviceScope)
	if err != nil {
		return err
	}
	authorization, err := client.DeviceAuthorize(cmd.Context())
	if err != nil {
		return deviceAuthFailure(
			cmd,
			authDeviceOutput,
			safeOAuthErrorCode(err),
			err,
		)
	}
	if authorization.DeviceCode != "" &&
		(strings.Contains(authorization.VerificationURI, authorization.DeviceCode) ||
			strings.Contains(authorization.VerificationURIComplete, authorization.DeviceCode) ||
			strings.Contains(authorization.UserCode, authorization.DeviceCode)) {
		return deviceAuthFailure(cmd, authDeviceOutput, "unsafe_device_response", errors.New("unsafe device authorization response"))
	}

	if authDeviceOutput == "jsonl" {
		event := struct {
			Type                    string `json:"type"`
			VerificationURI         string `json:"verification_uri"`
			VerificationURIComplete string `json:"verification_uri_complete,omitempty"`
			UserCode                string `json:"user_code"`
			ExpiresIn               int    `json:"expires_in"`
			Interval                int    `json:"interval"`
		}{
			Type:                    "device_authorization",
			VerificationURI:         authorization.VerificationURI,
			VerificationURIComplete: authorization.VerificationURIComplete,
			UserCode:                authorization.UserCode,
			ExpiresIn:               authorization.ExpiresIn,
			Interval:                authorization.Interval,
		}
		if err := json.NewEncoder(cmd.OutOrStdout()).Encode(event); err != nil {
			return generalErr("failed to write device authorization output", "")
		}
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "Open: %s\n", authorization.VerificationURI)
		fmt.Fprintf(cmd.OutOrStdout(), "Code: %s\n", authorization.UserCode)
		if authorization.VerificationURIComplete != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "Direct link: %s\n", authorization.VerificationURIComplete)
		}
	}

	openTarget := authorization.VerificationURIComplete
	if openTarget == "" {
		openTarget = authorization.VerificationURI
	}
	opened, openErr := oauthclient.ApplyOpenPolicy(openTarget, policy, oauthOpenURL)
	if openErr != nil {
		return deviceAuthFailure(cmd, authDeviceOutput, "browser_open_failed", openErr)
	}
	if policy == oauthclient.OpenAuto && !opened {
		fmt.Fprintln(cmd.ErrOrStderr(), "Browser was not opened; use the displayed authorization URL.")
	}

	token, err := oauthclient.PollDevice(cmd.Context(), client, authorization.DeviceCode, oauthclient.PollOptions{
		Interval:  time.Duration(authorization.Interval) * time.Second,
		ExpiresAt: oauthNow().Add(time.Duration(authorization.ExpiresIn) * time.Second),
		Now:       oauthNow,
	})
	if err != nil {
		code := safeOAuthErrorCode(err)
		return deviceAuthFailure(cmd, authDeviceOutput, code, err)
	}
	store, err := credential.DefaultStore()
	if err != nil {
		return deviceAuthFailure(cmd, authDeviceOutput, "credential_store_error", err)
	}
	if err := store.Save(token); err != nil {
		return deviceAuthFailure(cmd, authDeviceOutput, "credential_store_error", err)
	}
	if err := config.SetDefaultAuthMethod(string(authMethodOAuth)); err != nil {
		return deviceAuthFailure(cmd, authDeviceOutput, "credential_store_error", err)
	}
	if authDeviceOutput == "jsonl" {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(struct {
			Type   string `json:"type"`
			Status string `json:"status"`
			Method string `json:"method"`
		}{Type: "auth_result", Status: "logged_in", Method: "device"})
	}
	fmt.Fprintln(cmd.OutOrStdout(), "Logged in")
	return nil
}

func runAuthBrowser(cmd *cobra.Command, args []string) error {
	policy, err := oauthclient.ParseOpenPolicy(authBrowserOpenBrowser)
	if err != nil {
		return usageErr(err.Error(), "[fix] use --open-browser auto, always, or never")
	}
	prompt := strings.TrimSpace(authBrowserPrompt)
	if prompt != "" && prompt != "consent" {
		return usageErr("invalid --prompt value, only consent is supported",
			"[fix] use --prompt consent or omit --prompt")
	}
	if policy == oauthclient.OpenAuto {
		if available, reason := oauthBrowserAvailable(); !available {
			return usageErr("local browser OAuth is unavailable: "+reason,
				"[fix] use xparse-cli auth device, or use --open-browser never with explicit port forwarding")
		}
	}
	client, cfg, err := resolvedOAuthClient(cmd, authBrowserClientID, authBrowserScope)
	if err != nil {
		return err
	}
	redirectURI := config.ResolveOAuthRedirectURI(authBrowserRedirectURI, cfg)
	token, err := oauthclient.AuthorizeBrowser(cmd.Context(), oauthclient.BrowserOptions{
		Client:      client,
		RedirectURI: redirectURI,
		Policy:      policy,
		Opener:      oauthOpenURL,
		Timeout:     authBrowserTimeout,
		Prompt:      prompt,
		OnURL: func(target string) {
			fmt.Fprintf(cmd.OutOrStdout(), "Open: %s\n", target)
		},
	})
	if err != nil {
		return oauthCommandError(cmd, "browser authorization failed", err)
	}
	store, err := credential.DefaultStore()
	if err != nil {
		return generalErr("failed to locate OAuth credential store", "")
	}
	if err := store.Save(token); err != nil {
		return generalErr("failed to save OAuth credential", "")
	}
	if err := config.SetDefaultAuthMethod(string(authMethodOAuth)); err != nil {
		return generalErr("failed to save default authentication method", "")
	}
	fmt.Fprintln(cmd.OutOrStdout(), "Logged in")
	return nil
}

func runAuthStatus(cmd *cobra.Command, args []string) error {
	if authStatusOutput != "text" && authStatusOutput != "json" {
		return usageErr("invalid --output value, must be text or json", "[fix] use --output text or --output json")
	}
	cfg, err := config.Load()
	if err != nil {
		return generalErr("failed to load authentication configuration", "")
	}
	appKey, err := config.ResolveCredentials(cmd)
	if err != nil {
		return generalErr("failed to resolve AppKey configuration", "")
	}
	selection, selectionErr := selectParseAuthentication(cmd, APIModeAuto, "", appKey, cfg)
	loggedIn := selectionErr == nil && !selection.IsFree
	method := ""
	if loggedIn {
		method = string(selection.Method)
	}
	if authStatusOutput == "json" {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(struct {
			LoggedIn bool   `json:"logged_in"`
			Method   string `json:"method,omitempty"`
		}{LoggedIn: loggedIn, Method: method})
	}
	if loggedIn {
		fmt.Fprintf(cmd.OutOrStdout(), "Logged in (%s)\n", method)
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), "Logged out")
	}
	return nil
}

func runAuthLogout(cmd *cobra.Command, args []string) error {
	switch authLogoutMethod {
	case "oauth", "app-key", "all":
	default:
		return usageErr("--method is required and must be oauth, app-key, or all", "[fix] use auth logout --method oauth|app-key|all")
	}
	cfg, err := config.Load()
	configUnavailable := err != nil
	if err != nil {
		if authLogoutMethod == "app-key" {
			return generalErr("failed to load authentication configuration", "")
		}
		// OAuth/all logout must still be able to remove a local token when a
		// malformed YAML file prevents loading non-secret preferences.
		cfg = &config.Config{}
	}
	if authLogoutMethod == "oauth" || authLogoutMethod == "all" {
		store, err := credential.DefaultStore()
		if err != nil {
			return generalErr("failed to locate OAuth credential store", "")
		}
		if token, loadErr := store.Load(); loadErr == nil {
			rawToken, hint := token.RefreshToken, "refresh_token"
			if rawToken == "" {
				rawToken, hint = token.AccessToken, "access_token"
			}
			clientID := strings.TrimSpace(token.ClientID)
			if clientID == "" {
				clientID = config.ResolveOAuthClientID("", cfg)
			}
			if rawToken != "" && clientID != "" {
				revokeCtx, cancel := context.WithTimeout(cmd.Context(), 5*time.Second)
				client := &oauthclient.Client{
					BaseURL:    config.GetBaseURL(cmd, cfg),
					ClientID:   clientID,
					HTTPClient: oauthHTTPClient,
					Now:        oauthNow,
				}
				revokeErr := client.Revoke(revokeCtx, rawToken, hint)
				cancel()
				if revokeErr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(),
						"Warning: remote OAuth revocation failed (%s); local credentials were still removed.\n",
						safeOAuthErrorCode(revokeErr))
				}
			}
		}
		if err := store.Remove(); err != nil {
			return generalErr("failed to remove OAuth credential", "")
		}
	}
	if authLogoutMethod == "app-key" || authLogoutMethod == "all" {
		cfg.AppID = ""
		cfg.SecretCode = ""
	}
	if configUnavailable {
		if authLogoutMethod == "all" {
			if err := config.Save(&config.Config{}); err != nil {
				return generalErr("failed to reset malformed authentication configuration", "")
			}
			fmt.Fprintln(cmd.ErrOrStderr(),
				"Warning: malformed authentication preferences were reset during logout all.")
		} else {
			fmt.Fprintln(cmd.ErrOrStderr(),
				"Warning: authentication preferences could not be read; the local OAuth token was still removed.")
		}
		fmt.Fprintln(cmd.OutOrStdout(), "Logged out")
		return nil
	}
	reconcileDefaultAuthMethod(cfg)
	if err := config.Save(cfg); err != nil {
		return generalErr("failed to update authentication configuration", "")
	}
	fmt.Fprintln(cmd.OutOrStdout(), "Logged out")
	return nil
}

func reconcileDefaultAuthMethod(cfg *config.Config) {
	hasAppKey := strings.TrimSpace(cfg.AppID) != "" && strings.TrimSpace(cfg.SecretCode) != ""
	hasOAuth := hasOAuthSession(oauthNow())
	switch normalizeAuthMethod(cfg.DefaultAuthMethod) {
	case string(authMethodOAuth):
		if !hasOAuth {
			if hasAppKey {
				cfg.DefaultAuthMethod = string(authMethodAppKey)
			} else {
				cfg.DefaultAuthMethod = ""
			}
		}
	case string(authMethodAppKey):
		if !hasAppKey {
			if hasOAuth {
				cfg.DefaultAuthMethod = string(authMethodOAuth)
			} else {
				cfg.DefaultAuthMethod = ""
			}
		}
	}
}

func resolvedOAuthClient(cmd *cobra.Command, clientIDFlag, scopeFlag string) (*oauthclient.Client, *config.Config, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, nil, generalErr("failed to load OAuth configuration", "")
	}
	clientID := config.ResolveOAuthClientID(clientIDFlag, cfg)
	if clientID == "" {
		return nil, nil, usageErr("OAuth client ID is required", "[fix] use --client-id, XPARSE_OAUTH_CLIENT_ID, or config.yaml oauth.client_id")
	}
	return &oauthclient.Client{
		BaseURL:    config.GetBaseURL(cmd, cfg),
		ClientID:   clientID,
		Scope:      config.ResolveOAuthScope(scopeFlag, cfg),
		HTTPClient: oauthHTTPClient,
		Now:        oauthNow,
	}, cfg, nil
}

func deviceAuthFailure(cmd *cobra.Command, output, code string, cause error) error {
	if output == "jsonl" {
		_ = json.NewEncoder(cmd.OutOrStdout()).Encode(struct {
			Type   string `json:"type"`
			Status string `json:"status"`
			Error  string `json:"error"`
		}{Type: "auth_result", Status: "error", Error: code})
		// Do not include a potentially sensitive upstream response in stderr.
		fmt.Fprintf(cmd.ErrOrStderr(), "Device authorization failed: %s\n", code)
		return &exitError{code: exitcode.GeneralError, msg: code}
	}
	return oauthCommandError(cmd, "device authorization failed", cause)
}

func oauthCommandError(cmd *cobra.Command, prefix string, err error) error {
	code := safeOAuthErrorCode(err)
	fmt.Fprintf(cmd.ErrOrStderr(), "%s: %s\n", prefix, code)
	return &exitError{code: exitcode.GeneralError, msg: code, cause: err}
}

func safeOAuthErrorCode(err error) string {
	var oauthErr *oauthclient.OAuthError
	if errors.As(err, &oauthErr) && oauthErr.Code != "" {
		return oauthErr.Code
	}
	if errors.Is(err, context.Canceled) {
		return "canceled"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout"
	}
	var transportErr *oauthclient.TransportError
	if errors.As(err, &transportErr) {
		return "transport_error"
	}
	return "oauth_error"
}

func loadOAuthAccessToken(ctx context.Context, cmd *cobra.Command, cfg *config.Config) (string, error) {
	store, err := credential.DefaultStore()
	if err != nil {
		return "", err
	}
	token, err := store.Load()
	if err != nil {
		return "", err
	}
	now := oauthNow()
	if token.AccessValid(now, 30*time.Second) {
		return token.AccessToken, nil
	}
	if !token.RefreshValid(now) {
		return "", errors.New("OAuth session expired")
	}
	clientID := strings.TrimSpace(token.ClientID)
	if clientID == "" {
		clientID = config.ResolveOAuthClientID("", cfg)
	}
	if clientID == "" {
		return "", errors.New("OAuth client ID is required to refresh the session")
	}
	client := &oauthclient.Client{
		BaseURL:    config.GetBaseURL(cmd, cfg),
		ClientID:   clientID,
		Scope:      config.ResolveOAuthScope("", cfg),
		HTTPClient: oauthHTTPClient,
		Now:        oauthNow,
	}
	refreshed, err := client.Refresh(ctx, token.RefreshToken)
	if err != nil {
		return "", err
	}
	if refreshed.RefreshToken == "" {
		refreshed.RefreshToken = token.RefreshToken
		refreshed.RefreshExpiresAt = token.RefreshExpiresAt
	}
	if err := store.Save(refreshed); err != nil {
		return "", err
	}
	return refreshed.AccessToken, nil
}

func hasOAuthSession(now time.Time) bool {
	store, err := credential.DefaultStore()
	if err != nil {
		return false
	}
	token, err := store.Load()
	return err == nil && token.LoggedIn(now)
}

func explicitAppKeyFlags(cmd *cobra.Command) bool {
	return cmd.Flags().Changed("app-id") || cmd.Flags().Changed("secret-code")
}

func normalizeAuthMethod(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
