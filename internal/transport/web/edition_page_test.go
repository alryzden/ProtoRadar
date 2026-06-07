package web

import (
	"net/http"
	"strings"
	"testing"

	"github.com/alryzden/ProtoRadar/internal/usecase/uiquery"
)

func TestAboutPageRendersEditionVersionAndCapabilities(t *testing.T) {
	handler := newTestHandler(t, newFakeQuery())

	res := request(t, handler, "/ui/about")
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	body := res.Body.String()
	for _, want := range []string{"ProtoRadar Community Edition", "v1.2.0", "abc123", "registry", "breaking_checks", "oidc_auth", "advanced_rbac"} {
		if !strings.Contains(body, want) {
			t.Fatalf("about page missing %q: %s", want, body)
		}
	}
}

func TestAboutPageEscapesValues(t *testing.T) {
	query := newFakeQuery()
	query.edition = uiquery.EditionDetails{
		Edition:   `community<script>`,
		Version:   `v1.2.0<script>`,
		Commit:    `abc<script>`,
		BuildDate: `2026<script>`,
		Capabilities: []uiquery.CapabilityStatus{
			{Name: `registry<script>`, Enabled: true},
			{Name: `oidc_auth<script>`, Enabled: false},
		},
	}
	handler := newTestHandler(t, query)

	res := request(t, handler, "/ui/about")
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	body := res.Body.String()
	if strings.Contains(body, "<script>") {
		t.Fatalf("about page rendered raw HTML: %s", body)
	}
	for _, want := range []string{"community&lt;script&gt;", "registry&lt;script&gt;", "oidc_auth&lt;script&gt;"} {
		if !strings.Contains(body, want) {
			t.Fatalf("escaped value %q missing: %s", want, body)
		}
	}
}
