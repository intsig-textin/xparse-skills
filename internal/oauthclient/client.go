// Package oauthclient implements the OAuth flows used by xparse-cli.
package oauthclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"gitlab.intsig.net/xparse/xparse-client/internal/credential"
)

const (
	DeviceGrantType = "urn:ietf:params:oauth:grant-type:device_code"
	maxResponseSize = 1 << 20
)

// Client calls TextIn OAuth endpoints.
type Client struct {
	BaseURL    string
	ClientID   string
	Scope      string
	HTTPClient *http.Client
	Now        func() time.Time
}

// DeviceAuthorization is the RFC 8628 device authorization response. The
// DeviceCode field is private protocol state and must never be emitted.
type DeviceAuthorization struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete,omitempty"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval,omitempty"`
}

// OAuthError is a terminal or state-machine OAuth response.
type OAuthError struct {
	Code        string `json:"error"`
	Description string `json:"error_description,omitempty"`
	StatusCode  int    `json:"-"`
}

func (e *OAuthError) Error() string {
	if e.Description == "" {
		return e.Code
	}
	return e.Code + ": " + e.Description
}

// TransportError marks a request that did not receive an HTTP response.
type TransportError struct {
	Err error
}

func (e *TransportError) Error() string { return "OAuth transport error: " + e.Err.Error() }
func (e *TransportError) Unwrap() error { return e.Err }

// DeviceAuthorize starts a Device Authorization Grant.
func (c *Client) DeviceAuthorize(ctx context.Context) (*DeviceAuthorization, error) {
	form := url.Values{"client_id": {c.ClientID}}
	if c.Scope != "" {
		form.Set("scope", c.Scope)
	}
	req, err := c.formRequest(ctx, "/oauth21/device/authorization", form)
	if err != nil {
		return nil, err
	}
	body, status, err := c.do(req)
	if err != nil {
		return nil, err
	}
	if status < 200 || status >= 300 {
		return nil, decodeOAuthError(status, body)
	}
	var response DeviceAuthorization
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("decode device authorization response: %w", err)
	}
	if response.DeviceCode == "" || response.UserCode == "" || response.VerificationURI == "" || response.ExpiresIn <= 0 {
		return nil, errors.New("device authorization response missing required fields")
	}
	if response.Interval <= 0 {
		response.Interval = 5
	}
	return &response, nil
}

// PollDeviceToken performs one RFC 8628 token poll.
func (c *Client) PollDeviceToken(ctx context.Context, deviceCode string) (*credential.OAuthToken, error) {
	form := url.Values{
		"grant_type":  {DeviceGrantType},
		"client_id":   {c.ClientID},
		"device_code": {deviceCode},
	}
	return c.requestToken(ctx, form)
}

// ExchangeAuthorizationCode exchanges an Authorization Code + PKCE verifier.
func (c *Client) ExchangeAuthorizationCode(ctx context.Context, code, redirectURI, verifier string) (*credential.OAuthToken, error) {
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {c.ClientID},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"code_verifier": {verifier},
	}
	return c.requestToken(ctx, form)
}

// Refresh rotates a refresh token.
func (c *Client) Refresh(ctx context.Context, refreshToken string) (*credential.OAuthToken, error) {
	form := url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {c.ClientID},
		"refresh_token": {refreshToken},
	}
	return c.requestToken(ctx, form)
}

// Revoke asks the authorization server to invalidate a token per RFC 7009.
// The endpoint intentionally returns success even when a token is already
// unknown or revoked.
func (c *Client) Revoke(ctx context.Context, token, tokenTypeHint string) error {
	form := url.Values{
		"client_id": {c.ClientID},
		"token":     {token},
	}
	if tokenTypeHint != "" {
		form.Set("token_type_hint", tokenTypeHint)
	}
	req, err := c.formRequest(ctx, "/oauth21/revoke", form)
	if err != nil {
		return err
	}
	body, status, err := c.do(req)
	if err != nil {
		return err
	}
	if status < 200 || status >= 300 {
		return decodeOAuthError(status, body)
	}
	return nil
}

func (c *Client) requestToken(ctx context.Context, form url.Values) (*credential.OAuthToken, error) {
	req, err := c.formRequest(ctx, "/oauth21/token", form)
	if err != nil {
		return nil, err
	}
	body, status, err := c.do(req)
	if err != nil {
		return nil, err
	}
	if status < 200 || status >= 300 {
		return nil, decodeOAuthError(status, body)
	}
	var response struct {
		AccessToken  string `json:"access_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int    `json:"expires_in"`
		RefreshToken string `json:"refresh_token"`
		Scope        string `json:"scope"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("decode OAuth token response: %w", err)
	}
	if response.AccessToken == "" || response.ExpiresIn <= 0 {
		return nil, errors.New("OAuth token response missing required fields")
	}
	now := time.Now()
	if c.Now != nil {
		now = c.Now()
	}
	token := &credential.OAuthToken{
		ClientID:     c.ClientID,
		AccessToken:  response.AccessToken,
		TokenType:    response.TokenType,
		ExpiresIn:    response.ExpiresIn,
		RefreshToken: response.RefreshToken,
		Scope:        response.Scope,
		ExpiresAt:    now.Add(time.Duration(response.ExpiresIn) * time.Second),
	}
	if token.TokenType == "" {
		token.TokenType = "Bearer"
	}
	if token.RefreshToken != "" {
		token.RefreshExpiresAt = now.Add(30 * 24 * time.Hour)
	}
	return token, nil
}

func (c *Client) formRequest(ctx context.Context, path string, form url.Values) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(c.BaseURL, "/")+path, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return req, nil
}

func (c *Client) do(req *http.Request) ([]byte, int, error) {
	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, 0, &TransportError{Err: err}
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return nil, resp.StatusCode, &TransportError{
			Err: fmt.Errorf("read OAuth response: %w", err),
		}
	}
	return body, resp.StatusCode, nil
}

func decodeOAuthError(status int, body []byte) error {
	var oauthErr OAuthError
	if err := json.Unmarshal(body, &oauthErr); err == nil && oauthErr.Code != "" {
		oauthErr.StatusCode = status
		return &oauthErr
	}
	return fmt.Errorf("OAuth endpoint returned HTTP %d", status)
}
