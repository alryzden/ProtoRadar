package httptransport

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alryzden/ProtoRadar/internal/authorization"
	"github.com/alryzden/ProtoRadar/internal/identity"
	"github.com/alryzden/ProtoRadar/internal/usecase/registry"
)

func newAuthorizerCoverageTestServer(authorizer authorization.Authorizer) http.Handler {
	return newProtectedRouteTestServer(authorizer)
}

func newProtectedRouteTestServer(authorizer authorization.Authorizer) http.Handler {
	return newProtectedRouteTestServerWithGovernance(authorizer, newFakeGovernance())
}

func newProtectedRouteTestServerWithGovernance(authorizer authorization.Authorizer, governanceFake *fakeGovernance) http.Handler {
	fake := newFakeRegistry()
	return NewServer(fake, Options{
		BootstrapToken: "bootstrap",
		AuthProvider: &fakeAuthProvider{
			principal: identity.Principal{
				Subject:     "test",
				DisplayName: "test",
				Type:        identity.PrincipalTypeAPIToken,
			},
			acceptedAuthorizationHeader: "Bearer valid",
		},
		Authorizer: authorizer,
		Runtime:    fake,
		Governance: governanceFake,
	}).Handler()
}

func newTestServer() http.Handler {
	fake := newFakeRegistry()
	return NewServer(fake, Options{BootstrapToken: "bootstrap", Runtime: fake}).Handler()
}

func newServerWithPublishError(err error) http.Handler {
	fake := newFakeRegistry()
	fake.publishErr = err
	_, _ = fake.CreateModule(context.Background(), registry.CreateModuleRequest{Name: "user-api"})
	return NewServer(fake, Options{BootstrapToken: "bootstrap", Runtime: fake}).Handler()
}

func newServerWithCheckError(err error) http.Handler {
	fake := newFakeRegistry()
	fake.checkErr = err
	_, _ = fake.CreateModule(context.Background(), registry.CreateModuleRequest{Name: "user-api"})
	return NewServer(fake, Options{BootstrapToken: "bootstrap", Runtime: fake}).Handler()
}

func newRuntimeTestServer() http.Handler {
	return newRuntimeTestServerWithFake(newFakeRegistry())
}

func newRuntimeTestServerWithFake(fake *fakeRegistry) http.Handler {
	return NewServer(fake, Options{BootstrapToken: "bootstrap", Runtime: fake}).Handler()
}

func request(t *testing.T, handler http.Handler, method string, path string, body io.Reader, auth string, contentType string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(method, path, body)
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	return res
}

func performRequestWithBearer(t *testing.T, handler http.Handler, method string, path string, body io.Reader, contentType string) *httptest.ResponseRecorder {
	t.Helper()
	return request(t, handler, method, path, body, "Bearer valid", contentType)
}

func assertForbiddenByAuthorizer(t *testing.T, res *httptest.ResponseRecorder) {
	t.Helper()
	if res.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body = %s", res.Code, http.StatusForbidden, res.Body.String())
	}
	assertAPIError(t, res, "forbidden")
}

func multipartRequest(t *testing.T, handler http.Handler, path string, version string, artifact []byte) *httptest.ResponseRecorder {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("version", version); err != nil {
		t.Fatalf("write version: %v", err)
	}
	file, err := writer.CreateFormFile("artifact", "artifact.tar.gz")
	if err != nil {
		t.Fatalf("create file: %v", err)
	}
	if _, err := file.Write(artifact); err != nil {
		t.Fatalf("write artifact: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart: %v", err)
	}

	return request(t, handler, http.MethodPost, path, &body, "Bearer valid", writer.FormDataContentType())
}

func multipartBreakingRequest(t *testing.T, handler http.Handler, path string, against string, targetRef string, artifact []byte) *httptest.ResponseRecorder {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("against", against); err != nil {
		t.Fatalf("write against: %v", err)
	}
	if targetRef != "" {
		if err := writer.WriteField("target_ref", targetRef); err != nil {
			t.Fatalf("write target ref: %v", err)
		}
	}
	file, err := writer.CreateFormFile("artifact", "source.tar.gz")
	if err != nil {
		t.Fatalf("create file: %v", err)
	}
	if _, err := file.Write(artifact); err != nil {
		t.Fatalf("write artifact: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart: %v", err)
	}

	return request(t, handler, http.MethodPost, path, &body, "Bearer valid", writer.FormDataContentType())
}

func decodeResponse(t *testing.T, res *httptest.ResponseRecorder, dst any) {
	t.Helper()
	if err := json.NewDecoder(res.Body).Decode(dst); err != nil {
		t.Fatalf("decode response: %v", err)
	}
}

func assertAPIError(t *testing.T, res *httptest.ResponseRecorder, wantCode string) {
	t.Helper()
	var body errorResponse
	decodeResponse(t, res, &body)
	if body.Error.Code != wantCode {
		t.Fatalf("error code = %q, want %q; body = %#v", body.Error.Code, wantCode, body)
	}
	if strings.TrimSpace(body.Error.Message) == "" {
		t.Fatalf("error message is empty: %#v", body)
	}
	for _, forbidden := range []string{"stack trace", "secret.go", "panic:", "pq:", "SQLSTATE"} {
		if strings.Contains(body.Error.Message, forbidden) {
			t.Fatalf("error message leaked %q: %#v", forbidden, body)
		}
	}
}

type protectedRouteAuthorizationCase struct {
	name          string
	method        string
	path          string
	requestBody   func() (io.Reader, string)
	action        authorization.Action
	resource      authorization.Resource
	allowedStatus int
}
