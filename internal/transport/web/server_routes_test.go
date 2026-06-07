package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewServerParsesTemplates(t *testing.T) {
	if _, err := NewServer(Options{BasePath: "/ui", StaticPath: "/ui/static"}); err != nil {
		t.Fatalf("new server: %v", err)
	}
}

func TestLayoutRendersNavigation(t *testing.T) {
	handler := newTestHandler(t, newFakeQuery())
	res := request(t, handler, "/ui/modules")

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusOK)
	}
	body := res.Body.String()
	for _, want := range []string{"ProtoRadar", `href="/ui/modules"`, `href="/ui/runtime/services"`, `href="/ui/breaking-reports"`, `href="/ui/about"`, "Modules", "Runtime", "Breaking Reports", "About"} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q: %s", want, body)
		}
	}
}

func TestIndexRedirectsToModules(t *testing.T) {
	handler := newTestHandler(t, newFakeQuery())
	res := request(t, handler, "/ui")

	if res.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusFound)
	}
	if got := res.Header().Get("Location"); got != "/ui/modules" {
		t.Fatalf("location = %q", got)
	}
}

func TestStylesheetReturnsCSS(t *testing.T) {
	handler := newTestHandler(t, newFakeQuery())
	res := request(t, handler, "/ui/static/app.css")

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusOK)
	}
	if contentType := res.Header().Get("Content-Type"); !strings.Contains(contentType, "text/css") {
		t.Fatalf("content type = %q", contentType)
	}
	body := res.Body.String()
	for _, want := range []string{".badge-published", ".badge-breaking", ".table-wrap", ".empty-state", ".error-panel"} {
		if !strings.Contains(body, want) {
			t.Fatalf("css missing %q", want)
		}
	}
}

func TestNotFoundRouteRendersSafeErrorPage(t *testing.T) {
	handler := newTestHandler(t, newFakeQuery())
	res := request(t, handler, "/ui/does-not-exist")

	if res.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusNotFound)
	}
	body := res.Body.String()
	if !strings.Contains(body, "Page Not Found") || !strings.Contains(body, "Back to Modules") {
		t.Fatalf("body missing safe not found content: %s", body)
	}
}

func TestErrorPageDoesNotExposeStackTrace(t *testing.T) {
	server, err := NewServer(Options{BasePath: "/ui", StaticPath: "/ui/static", Query: newFakeQuery()})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	recorder := httptest.NewRecorder()
	server.RenderError(recorder, http.StatusInternalServerError, "panic: secret\ngoroutine 1 [running]\nstack trace")

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
	body := recorder.Body.String()
	for _, forbidden := range []string{"panic", "goroutine", "stack trace", "secret"} {
		if strings.Contains(strings.ToLower(body), forbidden) {
			t.Fatalf("error page exposes %q: %s", forbidden, body)
		}
	}
	if !strings.Contains(body, "Check server logs") {
		t.Fatalf("body missing user-friendly message: %s", body)
	}
}
