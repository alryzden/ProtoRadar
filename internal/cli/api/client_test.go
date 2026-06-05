package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestClientParsesStructuredAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":{"code":"not_found","message":"Requested resource was not found."}}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "prr_token", server.Client())
	err := client.CheckAuth(context.Background())
	var apiErr Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("error = %T %[1]v", err)
	}
	if apiErr.Code != "not_found" || apiErr.Body != "Requested resource was not found." {
		t.Fatalf("api error = %#v", apiErr)
	}
}

func TestClientRedactsTokenFromErrorBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":{"code":"internal_error","message":"token prr_super_secret failed"}}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "prr_super_secret", server.Client())
	err := client.CheckAuth(context.Background())
	if err == nil {
		t.Fatalf("expected error")
	}
	if strings.Contains(err.Error(), "prr_super_secret") {
		t.Fatalf("token leaked in error: %v", err)
	}
	if !strings.Contains(err.Error(), "[redacted]") {
		t.Fatalf("redacted marker missing: %v", err)
	}
}

func TestClientGetsBreakingReportRuntimeImpact(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"report_id":"report-2","impacts":[{"service_name":"billing-service","environment":"production","used_module":"user-api","used_version":"v1.2.0","git_commit":"abc1234","build_version":"2026.06.04-15","impact_status":"potentially_affected_by_breaking_change","reason":"exact version match"}]}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "prr_token", server.Client())
	impact, err := client.GetBreakingReportRuntimeImpact(context.Background(), "report-2")
	if err != nil {
		t.Fatalf("runtime impact: %v", err)
	}
	if gotPath != "/api/v1/breaking-reports/report-2/runtime-impact" {
		t.Fatalf("path = %q", gotPath)
	}
	if impact.ReportID != "report-2" || len(impact.Impacts) != 1 {
		t.Fatalf("impact = %#v", impact)
	}
	if !strings.Contains(impact.Impacts[0].BuildVersion, "2026.06.04") {
		t.Fatalf("impact row = %#v", impact.Impacts[0])
	}
}
