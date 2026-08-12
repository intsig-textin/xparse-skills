package authsession

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"gitlab.intsig.net/xparse/xparse-client/internal/config"
	"gitlab.intsig.net/xparse/xparse-client/internal/credential"
	"gitlab.intsig.net/xparse/xparse-client/internal/oauthclient"
)

// AccessToken loads a valid OAuth access token and refreshes it when possible.
func AccessToken(ctx context.Context, cmd *cobra.Command, cfg *config.Config, httpClient *http.Client, now func() time.Time) (string, error) {
	store, err := credential.DefaultStore()
	if err != nil {
		return "", err
	}
	token, err := store.Load()
	if err != nil {
		return "", err
	}
	if now == nil {
		now = time.Now
	}
	currentTime := now()
	if token.AccessValid(currentTime, 30*time.Second) {
		return token.AccessToken, nil
	}
	if !token.RefreshValid(currentTime) {
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
		BaseURL: config.GetBaseURL(cmd, cfg), ClientID: clientID,
		Scope: config.ResolveOAuthScope("", cfg), HTTPClient: httpClient, Now: now,
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
