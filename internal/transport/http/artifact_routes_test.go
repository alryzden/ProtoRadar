package httptransport

import (
	"bytes"
	"errors"
	"net/http"
	"strings"
	"testing"
)

func TestGetArtifactStreamsArtifact(t *testing.T) {
	server := newTestServer()
	request(t, server, http.MethodPost, "/api/v1/modules", strings.NewReader(`{"name":"user-api"}`), "Bearer valid", "application/json")
	multipartRequest(t, server, "/api/v1/modules/user-api/versions", "v1.0.0", []byte("artifact"))

	res := request(t, server, http.MethodGet, "/api/v1/modules/user-api/versions/v1.0.0/artifact", nil, "Bearer valid", "")
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d", res.Code)
	}
	if res.Body.String() != "artifact" {
		t.Fatalf("body = %q", res.Body.String())
	}
	if res.Header().Get("X-ProtoRadar-Digest") == "" {
		t.Fatalf("missing digest header")
	}
	if res.Header().Get("X-ProtoRadar-Checksum-SHA256") == "" {
		t.Fatalf("missing checksum header")
	}
}

func TestGetArtifactStreamCopyErrorLogsMetricAndClosesBody(t *testing.T) {
	fake := newFakeRegistry()
	var logs bytes.Buffer
	logger, err := NewLogger("info", "json", &logs)
	if err != nil {
		t.Fatalf("new logger: %v", err)
	}
	metrics := NewMetrics()
	server := NewServer(fake, Options{
		BootstrapToken: "bootstrap",
		Runtime:        fake,
		Logger:         logger,
		Metrics:        metrics,
	}).Handler()

	request(t, server, http.MethodPost, "/api/v1/modules", strings.NewReader(`{"name":"user-api"}`), "Bearer valid", "application/json")
	multipartRequest(t, server, "/api/v1/modules/user-api/versions", "v1.0.0", []byte("artifact"))
	reader := &failingReadCloser{chunks: [][]byte{[]byte("part")}, err: errors.New("backend failed for secret-object-key")}
	fake.objectReaders["user-api:v1.0.0"] = reader
	storageKey := fake.artifacts["user-api:v1.0.0"][0].StorageKey

	res := request(t, server, http.MethodGet, "/api/v1/modules/user-api/versions/v1.0.0/artifact", nil, "Bearer valid", "")

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d", res.Code)
	}
	if res.Body.String() != "part" {
		t.Fatalf("body = %q", res.Body.String())
	}
	if !reader.closed {
		t.Fatalf("artifact body was not closed")
	}
	if strings.Contains(res.Body.String(), "backend failed") || strings.Contains(res.Body.String(), "secret-object-key") || strings.Contains(res.Body.String(), "internal_error") {
		t.Fatalf("streaming response exposed error details: %q", res.Body.String())
	}
	logText := logs.String()
	if !strings.Contains(logText, "artifact_download_stream_error") {
		t.Fatalf("missing stream error log: %s", logText)
	}
	for _, forbidden := range []string{storageKey, "secret-object-key", "backend failed"} {
		if strings.Contains(logText, forbidden) {
			t.Fatalf("log leaked %q: %s", forbidden, logText)
		}
	}

	metricsRes := request(t, server, http.MethodGet, "/metrics", nil, "", "")
	if !strings.Contains(metricsRes.Body.String(), "protoradar_artifact_stream_errors_total 1") {
		t.Fatalf("metrics missing stream error counter:\n%s", metricsRes.Body.String())
	}
}
