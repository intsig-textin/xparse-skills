package oauthclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRevokeSendsRFC7009Form(t *testing.T) {
	var receivedToken string
	var receivedHint string
	var receivedClientID string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/oauth21/revoke" {
			http.NotFound(writer, request)
			return
		}
		if err := request.ParseForm(); err != nil {
			t.Fatal(err)
		}
		receivedToken = request.Form.Get("token")
		receivedHint = request.Form.Get("token_type_hint")
		receivedClientID = request.Form.Get("client_id")
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{}`))
	}))
	defer server.Close()

	client := &Client{
		BaseURL:    server.URL,
		ClientID:   "public-client",
		HTTPClient: server.Client(),
	}
	if err := client.Revoke(context.Background(), "refresh-private", "refresh_token"); err != nil {
		t.Fatal(err)
	}
	if receivedToken != "refresh-private" ||
		receivedHint != "refresh_token" ||
		receivedClientID != "public-client" {
		t.Fatalf("revoke form token=%q hint=%q client_id=%q",
			receivedToken, receivedHint, receivedClientID)
	}
}
