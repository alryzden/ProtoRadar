package web

import (
	"net/http"
	"strings"
	"testing"
)

func TestApprovalRequestPageShowsRequirementsDecisionsAndAudit(t *testing.T) {
	handler := newTestHandler(t, newFakeQuery())
	res := request(t, handler, "/ui/approval-requests/approval-1")

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d", res.Code)
	}
	body := res.Body.String()
	for _, want := range []string{"Approval Request", "approval-1", "module_owner_approval", "decision-1", "alice", "looks good", "Governance Audit Trail", "approval_request_created"} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q: %s", want, body)
		}
	}
}

func TestApprovalRequestValuesAreHTMLEscaped(t *testing.T) {
	query := newFakeQuery()
	details := query.approvalDetails["approval-1"]
	details.Request.Requirements[0].Reason = `<script>alert(1)</script>`
	details.Request.Decisions[0].DecidedBy = `<b>alice</b>`
	details.Request.Decisions[0].Comment = `<i>comment</i>`
	details.AuditEvents[0].PayloadJSON = `<img src=x onerror=alert(1)>`
	query.approvalDetails["approval-1"] = details
	handler := newTestHandler(t, query)
	res := request(t, handler, "/ui/approval-requests/approval-1")

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d", res.Code)
	}
	body := res.Body.String()
	if strings.Contains(body, `<script>`) || strings.Contains(body, `<b>alice</b>`) || strings.Contains(body, `<i>comment</i>`) || strings.Contains(body, `<img`) {
		t.Fatalf("body rendered raw approval HTML: %s", body)
	}
	for _, want := range []string{`&lt;script&gt;alert(1)&lt;/script&gt;`, `&lt;b&gt;alice&lt;/b&gt;`, `&lt;i&gt;comment&lt;/i&gt;`, `&lt;img src=x onerror=alert(1)&gt;`} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing escaped %q: %s", want, body)
		}
	}
}

func TestMissingApprovalRequestReturnsNotFound(t *testing.T) {
	handler := newTestHandler(t, newFakeQuery())
	res := request(t, handler, "/ui/approval-requests/missing")

	if res.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusNotFound)
	}
	if !strings.Contains(res.Body.String(), "Approval request not found") {
		t.Fatalf("body = %s", res.Body.String())
	}
}
