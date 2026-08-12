package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestResolveCapabilitySendsSafeSpecAndBearer(t *testing.T) {
	var received CapabilityResolveRequest
	server := httptest.NewServer(http.HandlerFunc(func(
		writer http.ResponseWriter,
		request *http.Request,
	) {
		if request.URL.Path != CapabilityResolveAPIPath {
			http.NotFound(writer, request)
			return
		}
		if request.Header.Get("Authorization") != "Bearer access-token" ||
			request.Header.Get("X-From") != "workbuddy" {
			t.Fatalf("headers = %v", request.Header)
		}
		if err := json.NewDecoder(request.Body).Decode(&received); err != nil {
			t.Fatal(err)
		}
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"code": 200,
			"data": map[string]any{
				"snapshot_version":          "parse-capability.v1",
				"supported":                 true,
				"server_preflight_required": false,
				"channels": []map[string]any{{
					"id":                         "free",
					"available":                  true,
					"automatic_use_allowed":      true,
					"remaining_pages":            465,
					"max_pages_per_request":      50,
					"max_file_size_bytes":        10485760,
					"max_parallel_requests":      2,
					"split_strategies":           []string{"page_range"},
					"creates_new_charge":         false,
					"requires_user_confirmation": false,
				}},
			},
		})
	}))
	defer server.Close()

	client := &Client{
		BaseURL:     server.URL,
		BearerToken: "access-token",
		IsFreeAPI:   true,
		HTTPClient:  server.Client(),
	}
	t.Setenv("XPARSE_CLIENT_FROM", "workbuddy")
	response, err := client.ResolveCapability(context.Background(), &CapabilityResolveRequest{
		ClientVersion: "2.2.1",
		Profile:       "workbuddy",
		RequestedAPI:  "auto",
		File: CapabilityFileSpec{
			SourceType:   "local",
			DetectedType: "pdf",
			SizeBytes:    1024,
			PageCount:    2,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if received.File.SourceType != "local" || received.File.PageCount != 2 {
		t.Fatalf("request = %#v", received)
	}
	if response.Code != 200 || response.Data == nil || !response.Data.Supported ||
		len(response.Data.Channels) != 1 || response.Data.Channels[0].ID != "free" {
		t.Fatalf("response = %#v", response)
	}
}

func TestResolveCapabilityRejectsInvalidServiceResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(
		writer http.ResponseWriter,
		_ *http.Request,
	) {
		writer.WriteHeader(http.StatusBadGateway)
		_, _ = writer.Write([]byte("gateway unavailable"))
	}))
	defer server.Close()
	client := &Client{BaseURL: server.URL, HTTPClient: server.Client()}

	if _, err := client.ResolveCapability(
		context.Background(),
		&CapabilityResolveRequest{},
	); err == nil {
		t.Fatal("invalid capability response unexpectedly succeeded")
	}
}
