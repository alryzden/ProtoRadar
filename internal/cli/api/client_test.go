package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientSetsAuthorizationHeader(t *testing.T) {
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.URL, "prr_token", server.Client())
	if err := client.CheckAuth(context.Background()); err != nil {
		t.Fatalf("check auth: %v", err)
	}

	if gotAuth != "Bearer prr_token" {
		t.Fatalf("authorization = %q", gotAuth)
	}
}

func TestClientMapsCommonAPIErrors(t *testing.T) {
	tests := []struct {
		name string
		code int
		want error
	}{
		{name: "unauthorized", code: http.StatusUnauthorized, want: ErrUnauthorized},
		{name: "conflict", code: http.StatusConflict, want: ErrConflict},
		{name: "not found", code: http.StatusNotFound, want: ErrNotFound},
		{name: "too large", code: http.StatusRequestEntityTooLarge, want: ErrTooLarge},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "error", tt.code)
			}))
			defer server.Close()

			client := NewClient(server.URL, "prr_token", server.Client())
			err := client.CheckAuth(context.Background())
			if !errors.Is(err, tt.want) {
				t.Fatalf("error = %v, want %v", err, tt.want)
			}
		})
	}
}
