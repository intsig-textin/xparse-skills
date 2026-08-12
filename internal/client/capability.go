package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// CapabilityResolveAPIPath is the domestic read-only capability endpoint.
const CapabilityResolveAPIPath = "/api/v1/agent/parse/capabilities/resolve"

// CapabilityResolveRequest contains no local path, filename, URL, or content.
type CapabilityResolveRequest struct {
	ClientVersion       string             `json:"client_version"`
	Profile             string             `json:"profile,omitempty"`
	RequestedAPI        string             `json:"requested_api"`
	ExplicitPaidConsent bool               `json:"explicit_paid_consent"`
	File                CapabilityFileSpec `json:"file"`
}

// CapabilityFileSpec is the privacy-minimized preflight result.
type CapabilityFileSpec struct {
	SourceType   string `json:"source_type"`
	DetectedType string `json:"detected_type,omitempty"`
	SizeBytes    int64  `json:"size_bytes,omitempty"`
	PageCount    int    `json:"page_count,omitempty"`
}

// CapabilityResolveResponse is the standard service response wrapper.
type CapabilityResolveResponse struct {
	Code       int                      `json:"code"`
	Message    string                   `json:"message"`
	XRequestID string                   `json:"x_request_id,omitempty"`
	Data       *ParseCapabilitySnapshot `json:"data,omitempty"`
}

// ParseCapabilitySnapshot is valid only for its server-provided lifetime.
type ParseCapabilitySnapshot struct {
	SnapshotVersion         string              `json:"snapshot_version"`
	GeneratedAt             string              `json:"generated_at"`
	ExpiresAt               string              `json:"expires_at"`
	Supported               bool                `json:"supported"`
	ServerPreflightRequired bool                `json:"server_preflight_required"`
	Channels                []CapabilityChannel `json:"channels"`
	NextAction              *string             `json:"next_action"`
	RequestID               string              `json:"request_id,omitempty"`
}

// CapabilityChannel carries the server-authoritative routing and limit decision.
type CapabilityChannel struct {
	ID                       string   `json:"id"`
	Available                bool     `json:"available"`
	AutomaticUseAllowed      bool     `json:"automatic_use_allowed"`
	RequiresUserConfirmation bool     `json:"requires_user_confirmation"`
	CreatesNewCharge         bool     `json:"creates_new_charge"`
	RemainingPages           int      `json:"remaining_pages"`
	MaxPagesPerRequest       int      `json:"max_pages_per_request"`
	MaxFileSizeBytes         int64    `json:"max_file_size_bytes"`
	MaxParallelRequests      int      `json:"max_parallel_requests"`
	SplitStrategies          []string `json:"split_strategies"`
}

// ResolveCapability queries the server without consuming parse quota.
func (c *Client) ResolveCapability(
	ctx context.Context,
	request *CapabilityResolveRequest,
) (*CapabilityResolveResponse, error) {
	payload, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("marshal capability request: %w", err)
	}
	httpRequest, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.BaseURL+CapabilityResolveAPIPath,
		bytes.NewReader(payload),
	)
	if err != nil {
		return nil, fmt.Errorf("create capability request: %w", err)
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	c.setAuthHeaders(httpRequest)

	httpResponse, err := c.HTTPClient.Do(httpRequest)
	if err != nil {
		return nil, fmt.Errorf("capability request failed: %w", err)
	}
	defer httpResponse.Body.Close()
	body, err := io.ReadAll(httpResponse.Body)
	if err != nil {
		return nil, fmt.Errorf("read capability response: %w", err)
	}
	if httpResponse.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(
			"capability request HTTP %d",
			httpResponse.StatusCode,
		)
	}
	var response CapabilityResolveResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("decode capability response: %w", err)
	}
	return &response, nil
}
