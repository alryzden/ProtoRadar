package web

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func newTestHandler(t *testing.T, query Query) http.Handler {
	t.Helper()
	server, err := NewServer(Options{BasePath: "/ui", StaticPath: "/ui/static", Query: query})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	return server.Handler()
}

func request(t *testing.T, handler http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	return res
}
