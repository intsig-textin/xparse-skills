package oauthclient

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestPollDeviceBackoffStateMachine(t *testing.T) {
	var mu sync.Mutex
	responses := []roundTripResult{
		{err: errors.New("timeout")},
		{status: http.StatusBadRequest, body: `{"error":"authorization_pending"}`},
		{status: http.StatusBadRequest, body: `{"error":"slow_down"}`},
		{err: errors.New("temporary network failure")},
		{err: errors.New("temporary network failure")},
		{err: errors.New("temporary network failure")},
		{err: errors.New("temporary network failure")},
		{status: http.StatusOK, body: `{"access_token":"access-private","token_type":"Bearer","expires_in":900,"refresh_token":"refresh-private","scope":"ocr:*"}`},
	}
	transport := roundTripperFunc(func(request *http.Request) (*http.Response, error) {
		mu.Lock()
		defer mu.Unlock()
		if len(responses) == 0 {
			t.Fatal("unexpected extra poll")
		}
		result := responses[0]
		responses = responses[1:]
		if result.err != nil {
			return nil, result.err
		}
		return &http.Response{
			StatusCode: result.status,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(result.body)),
			Request:    request,
		}, nil
	})

	now := time.Unix(1_000, 0)
	var waits []time.Duration
	client := &Client{
		BaseURL:    "https://oauth.example",
		ClientID:   "client",
		HTTPClient: &http.Client{Transport: transport},
		Now:        func() time.Time { return now },
	}
	token, err := PollDevice(context.Background(), client, "private-device-code", PollOptions{
		Interval:  5 * time.Second,
		ExpiresAt: now.Add(10 * time.Minute),
		Now:       func() time.Time { return now },
		Wait: func(ctx context.Context, duration time.Duration) error {
			waits = append(waits, duration)
			now = now.Add(duration)
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if token.AccessToken != "access-private" {
		t.Fatalf("access token = %q", token.AccessToken)
	}
	want := []time.Duration{
		5 * time.Second,
		10 * time.Second,
		5 * time.Second,
		10 * time.Second,
		20 * time.Second,
		40 * time.Second,
		60 * time.Second,
		60 * time.Second,
	}
	if len(waits) != len(want) {
		t.Fatalf("waits = %v, want %v", waits, want)
	}
	for i := range want {
		if waits[i] != want[i] {
			t.Fatalf("waits[%d] = %s, want %s (all %v)", i, waits[i], want[i], waits)
		}
	}
}

func TestPollDeviceTerminalErrorStopsImmediately(t *testing.T) {
	requests := 0
	client := &Client{
		BaseURL:  "https://oauth.example",
		ClientID: "client",
		HTTPClient: &http.Client{Transport: roundTripperFunc(func(request *http.Request) (*http.Response, error) {
			requests++
			return &http.Response{
				StatusCode: http.StatusBadRequest,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"error":"access_denied"}`)),
				Request:    request,
			}, nil
		})},
	}
	_, err := PollDevice(context.Background(), client, "private-device-code", PollOptions{
		Wait: func(context.Context, time.Duration) error { return nil },
	})
	var oauthErr *OAuthError
	if !errors.As(err, &oauthErr) || oauthErr.Code != "access_denied" {
		t.Fatalf("error = %#v", err)
	}
	if requests != 1 {
		t.Fatalf("requests = %d, want 1", requests)
	}
}

func TestPollDeviceRetriesResponseBodyReadFailure(t *testing.T) {
	requests := 0
	client := &Client{
		BaseURL:  "https://oauth.example",
		ClientID: "client",
		HTTPClient: &http.Client{Transport: roundTripperFunc(func(request *http.Request) (*http.Response, error) {
			requests++
			body := io.ReadCloser(io.NopCloser(strings.NewReader(
				`{"access_token":"access-private","token_type":"Bearer","expires_in":900}`,
			)))
			if requests == 1 {
				body = io.NopCloser(errorReader{err: context.DeadlineExceeded})
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       body,
				Request:    request,
			}, nil
		})},
	}

	var waits []time.Duration
	token, err := PollDevice(context.Background(), client, "private-device-code", PollOptions{
		Interval: 5 * time.Second,
		Wait: func(_ context.Context, duration time.Duration) error {
			waits = append(waits, duration)
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if token.AccessToken != "access-private" {
		t.Fatalf("access token = %q", token.AccessToken)
	}
	if requests != 2 {
		t.Fatalf("requests = %d, want 2", requests)
	}
	want := []time.Duration{5 * time.Second, 10 * time.Second}
	if len(waits) != len(want) || waits[0] != want[0] || waits[1] != want[1] {
		t.Fatalf("waits = %v, want %v", waits, want)
	}
}

func TestPollDeviceWaitIsContextAware(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	requests := 0
	client := &Client{
		BaseURL:  "https://oauth.example",
		ClientID: "client",
		HTTPClient: &http.Client{Transport: roundTripperFunc(func(request *http.Request) (*http.Response, error) {
			requests++
			return nil, errors.New("must not poll")
		})},
	}
	_, err := PollDevice(ctx, client, "private-device-code", PollOptions{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context canceled", err)
	}
	if requests != 0 {
		t.Fatalf("requests = %d, want 0", requests)
	}
}

type roundTripResult struct {
	status int
	body   string
	err    error
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (function roundTripperFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

type errorReader struct {
	err error
}

func (reader errorReader) Read([]byte) (int, error) {
	return 0, reader.err
}
