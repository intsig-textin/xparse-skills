package cmd

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type loggingRoundTripper struct {
	next http.RoundTripper
}

func (l *loggingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	start := time.Now()

	log.Printf("[DEBUG] → %s %s\n", req.Method, redactURL(req.URL))

	// Log request body for JSON payloads only (skip binary uploads)
	if req.Body != nil && strings.Contains(req.Header.Get("Content-Type"), "application/json") {
		bodyBytes, err := io.ReadAll(req.Body)
		req.Body.Close()
		if err == nil && len(bodyBytes) > 0 {
			log.Printf("[DEBUG]    body: %s\n", string(bodyBytes))
		}
		req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	}

	res, err := l.next.RoundTrip(req)
	duration := time.Since(start)

	if err != nil {
		log.Printf("[DEBUG] ← ERROR %s %s (%v): %v\n", req.Method, redactURL(req.URL), duration, err)
		return nil, err
	}

	log.Printf("[DEBUG] ← %d %s %s (%v)\n", res.StatusCode, req.Method, redactURL(req.URL), duration)
	return res, err
}

// redactURL masks sensitive query parameters in the URL for logging.
var sensitiveParams = map[string]bool{"pdf_pwd": true}

func redactURL(u *url.URL) string {
	if u.RawQuery == "" {
		return u.String()
	}
	q := u.Query()
	for key := range q {
		if sensitiveParams[key] {
			q.Set(key, "***")
		}
	}
	safe := *u
	safe.RawQuery = q.Encode()
	return safe.String()
}

func newVerboseHTTPClient() *http.Client {
	return &http.Client{
		Transport: &loggingRoundTripper{
			next: http.DefaultTransport,
		},
	}
}
