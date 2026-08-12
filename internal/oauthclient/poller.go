package oauthclient

import (
	"context"
	"errors"
	"time"

	"gitlab.intsig.net/xparse/xparse-client/internal/credential"
)

// WaitFunc is injectable for deterministic polling tests.
type WaitFunc func(context.Context, time.Duration) error

// PollOptions controls the Device Flow state machine.
type PollOptions struct {
	Interval  time.Duration
	ExpiresAt time.Time
	Now       func() time.Time
	Wait      WaitFunc
}

// PollDevice waits and polls until success or a terminal condition. Only
// pending, slow_down, and transport errors continue.
func PollDevice(ctx context.Context, client *Client, deviceCode string, options PollOptions) (*credential.OAuthToken, error) {
	baseInterval := options.Interval
	if baseInterval <= 0 {
		baseInterval = 5 * time.Second
	}
	now := options.Now
	if now == nil {
		now = time.Now
	}
	wait := options.Wait
	if wait == nil {
		wait = waitContext
	}

	var transportBackoff time.Duration
	for {
		if !options.ExpiresAt.IsZero() && !now().Before(options.ExpiresAt) {
			return nil, &OAuthError{Code: "expired_token"}
		}
		delay := baseInterval
		if transportBackoff > 0 {
			delay = transportBackoff
		}
		if !options.ExpiresAt.IsZero() {
			remaining := options.ExpiresAt.Sub(now())
			if remaining <= 0 {
				return nil, &OAuthError{Code: "expired_token"}
			}
			if delay > remaining {
				delay = remaining
			}
		}
		if err := wait(ctx, delay); err != nil {
			return nil, err
		}
		if !options.ExpiresAt.IsZero() && !now().Before(options.ExpiresAt) {
			return nil, &OAuthError{Code: "expired_token"}
		}

		token, err := client.PollDeviceToken(ctx, deviceCode)
		if err == nil {
			return token, nil
		}
		var transportErr *TransportError
		if errors.As(err, &transportErr) {
			if transportBackoff == 0 {
				transportBackoff = minDuration(baseInterval*2, 60*time.Second)
			} else {
				transportBackoff = minDuration(transportBackoff*2, 60*time.Second)
			}
			continue
		}
		transportBackoff = 0

		var oauthErr *OAuthError
		if !errors.As(err, &oauthErr) {
			return nil, err
		}
		switch oauthErr.Code {
		case "authorization_pending":
			continue
		case "slow_down":
			baseInterval += 5 * time.Second
			continue
		default:
			return nil, err
		}
	}
}

func waitContext(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func minDuration(left, right time.Duration) time.Duration {
	if left < right {
		return left
	}
	return right
}
