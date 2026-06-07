package httptransport

import (
	"net/http"
	"strings"
	"testing"
)

func TestTokenEndpointDoesNotReturnTokenHash(t *testing.T) {
	server := newTestServer()

	res := request(t, server, http.MethodPost, "/api/v1/tokens", strings.NewReader(`{"name":"ci"}`), "Bearer bootstrap", "application/json")
	if res.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}

	var body map[string]any
	decodeResponse(t, res, &body)
	if _, exists := body["token_hash"]; exists {
		t.Fatalf("token_hash leaked in response")
	}
	if body["token"] != "raw-token" {
		t.Fatalf("token = %#v", body["token"])
	}
	if strings.Contains(res.Body.String(), "stored-hash") {
		t.Fatalf("stored hash leaked in response")
	}
}
