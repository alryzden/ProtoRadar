package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestModuleOwnersListRequiresModule(t *testing.T) {
	app := App{}

	err := app.Run(context.Background(), []string{"module", "owners", "list"})
	if err == nil || !strings.Contains(err.Error(), "module name") {
		t.Fatalf("error = %v", err)
	}
}

func TestModuleOwnersListCallsExpectedEndpoint(t *testing.T) {
	var gotPath string
	server := governanceCLIServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/modules/user-api/owners" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		writeJSON(t, w, http.StatusOK, map[string]any{
			"module": "user-api",
			"owners": []map[string]any{{
				"id":           "owner-1",
				"module_id":    "module-user-api",
				"module_name":  "user-api",
				"subject_type": "user",
				"subject":      "alice",
				"role":         "owner",
				"created_at":   "2026-06-05T12:00:00Z",
				"updated_at":   "2026-06-05T12:00:00Z",
			}},
		})
	})
	defer server.Close()

	output, err := runGovernanceCLI(t, server, []string{"module", "owners", "list", "user-api"})
	if err != nil {
		t.Fatalf("owners list: %v", err)
	}
	if gotPath != "/api/v1/modules/user-api/owners" {
		t.Fatalf("path = %q", gotPath)
	}
	if !strings.Contains(output, "OWNER ID") || !strings.Contains(output, "alice") || strings.Contains(output, "prr_token") {
		t.Fatalf("output = %q", output)
	}
}

func TestModuleOwnersListEmptyState(t *testing.T) {
	server := governanceCLIServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, http.StatusOK, map[string]any{"module": "user-api", "owners": []map[string]any{}})
	})
	defer server.Close()

	output, err := runGovernanceCLI(t, server, []string{"module", "owners", "list", "user-api"})
	if err != nil {
		t.Fatalf("owners list: %v", err)
	}
	if !strings.Contains(output, "No owners or maintainers configured") {
		t.Fatalf("output = %q", output)
	}
}

func TestGovernanceActorHelpIsDeprecated(t *testing.T) {
	commands := [][]string{
		{"module", "owners", "add", "--help"},
		{"module", "owners", "remove", "--help"},
		{"approvals", "request", "--help"},
		{"approvals", "approve", "--help"},
		{"approvals", "reject", "--help"},
	}
	for _, args := range commands {
		var output bytes.Buffer
		err := (App{Out: &output}).Run(context.Background(), args)
		if err != nil {
			t.Fatalf("help for %v: %v", args, err)
		}
		help := output.String()
		if !strings.Contains(help, "--actor is deprecated") || !strings.Contains(help, "server actor override") {
			t.Fatalf("help for %v = %q", args, help)
		}
	}
}

func TestModuleOwnersAddRequiresAndValidatesFields(t *testing.T) {
	app := App{}

	cases := [][]string{
		{"module", "owners", "add", "user-api", "--subject", "alice", "--role", "owner"},
		{"module", "owners", "add", "user-api", "--subject-type", "user", "--role", "owner"},
	}
	for _, args := range cases {
		if err := app.Run(context.Background(), args); err == nil {
			t.Fatalf("args %v expected error", args)
		}
	}
}

func TestModuleOwnersAddInvalidSubjectTypeExitsTwo(t *testing.T) {
	app := App{}

	err := app.Run(context.Background(), []string{"module", "owners", "add", "user-api", "--subject-type", "group", "--subject", "platform-team", "--role", "owner"})
	if exitCode(err) != 2 {
		t.Fatalf("exit code = %d err=%v", exitCode(err), err)
	}
	if err == nil || !strings.Contains(err.Error(), "--subject-type user|team") {
		t.Fatalf("error = %v", err)
	}
}

func TestModuleOwnersAddInvalidRoleExitsTwo(t *testing.T) {
	app := App{}

	err := app.Run(context.Background(), []string{"module", "owners", "add", "user-api", "--subject-type", "team", "--subject", "platform-team", "--role", "admin"})
	if exitCode(err) != 2 {
		t.Fatalf("exit code = %d err=%v", exitCode(err), err)
	}
	if err == nil || !strings.Contains(err.Error(), "--role owner|maintainer") {
		t.Fatalf("error = %v", err)
	}
}

func TestModuleOwnersAddCallsExpectedEndpoint(t *testing.T) {
	var gotPath string
	var gotRequest map[string]string
	server := governanceCLIServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/modules/user-api/owners" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&gotRequest); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		writeJSON(t, w, http.StatusCreated, map[string]any{
			"id":           "owner-1",
			"module_id":    "module-user-api",
			"module_name":  "user-api",
			"subject_type": gotRequest["subject_type"],
			"subject":      gotRequest["subject"],
			"role":         gotRequest["role"],
			"created_at":   "2026-06-05T12:00:00Z",
			"updated_at":   "2026-06-05T12:00:00Z",
		})
	})
	defer server.Close()

	output, err := runGovernanceCLI(t, server, []string{"module", "owners", "add", "user-api", "--subject-type", "team", "--subject", "platform", "--role", "owner", "--actor", "alice"})
	if err != nil {
		t.Fatalf("owners add: %v", err)
	}
	if gotPath != "/api/v1/modules/user-api/owners" || gotRequest["actor"] != "alice" || gotRequest["subject_type"] != "team" || gotRequest["role"] != "owner" {
		t.Fatalf("path=%q request=%#v", gotPath, gotRequest)
	}
	if !strings.Contains(output, "owner_id=owner-1") {
		t.Fatalf("output = %q", output)
	}
}

func TestModuleOwnersAddDoesNotRequireActorAndOmitsItByDefault(t *testing.T) {
	var gotRequest map[string]string
	server := governanceCLIServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/modules/user-api/owners" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&gotRequest); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		writeJSON(t, w, http.StatusCreated, map[string]any{
			"id":           "owner-1",
			"module_id":    "module-user-api",
			"module_name":  "user-api",
			"subject_type": gotRequest["subject_type"],
			"subject":      gotRequest["subject"],
			"role":         gotRequest["role"],
			"created_at":   "2026-06-05T12:00:00Z",
			"updated_at":   "2026-06-05T12:00:00Z",
		})
	})
	defer server.Close()

	output, err := runGovernanceCLI(t, server, []string{"module", "owners", "add", "user-api", "--subject-type", "team", "--subject", "platform", "--role", "owner"})
	if err != nil {
		t.Fatalf("owners add: %v", err)
	}
	if _, ok := gotRequest["actor"]; ok {
		t.Fatalf("actor should be omitted by default: %#v", gotRequest)
	}
	if !strings.Contains(output, "owner_id=owner-1") || strings.Contains(output, "prr_token") {
		t.Fatalf("output = %q", output)
	}
}

func TestModuleOwnersRemoveRequiresOwnerID(t *testing.T) {
	app := App{}

	err := app.Run(context.Background(), []string{"module", "owners", "remove", "user-api"})
	if err == nil || !strings.Contains(err.Error(), "--owner-id") {
		t.Fatalf("error = %v", err)
	}
}

func TestModuleOwnersRemoveCallsExpectedEndpoint(t *testing.T) {
	var gotPath string
	var gotActor string
	server := governanceCLIServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotActor = r.URL.Query().Get("actor")
		if r.Method != http.MethodDelete || r.URL.Path != "/api/v1/modules/user-api/owners/owner-1" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	defer server.Close()

	output, err := runGovernanceCLI(t, server, []string{"module", "owners", "remove", "user-api", "--owner-id", "owner-1", "--actor", "alice"})
	if err != nil {
		t.Fatalf("owners remove: %v", err)
	}
	if gotPath != "/api/v1/modules/user-api/owners/owner-1" || gotActor != "alice" {
		t.Fatalf("path=%q actor=%q", gotPath, gotActor)
	}
	if !strings.Contains(output, "Removed owner owner-1") {
		t.Fatalf("output = %q", output)
	}
}

func TestModuleOwnersRemoveDoesNotRequireActorAndOmitsItByDefault(t *testing.T) {
	var gotRawQuery string
	server := governanceCLIServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotRawQuery = r.URL.RawQuery
		if r.Method != http.MethodDelete || r.URL.Path != "/api/v1/modules/user-api/owners/owner-1" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	defer server.Close()

	output, err := runGovernanceCLI(t, server, []string{"module", "owners", "remove", "user-api", "--owner-id", "owner-1"})
	if err != nil {
		t.Fatalf("owners remove: %v", err)
	}
	if strings.Contains(gotRawQuery, "actor=") {
		t.Fatalf("actor should be omitted by default: raw_query=%q", gotRawQuery)
	}
	if !strings.Contains(output, "Removed owner owner-1") {
		t.Fatalf("output = %q", output)
	}
}

func TestApprovalsRequestRequiresReportID(t *testing.T) {
	app := App{}

	err := app.Run(context.Background(), []string{"approvals", "request"})
	if err == nil || !strings.Contains(err.Error(), "--report-id") {
		t.Fatalf("error = %v", err)
	}
}

func TestApprovalsRequestPrintsRequirements(t *testing.T) {
	var gotRequest map[string]string
	server := governanceCLIServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/breaking-reports/report-1/approval-request" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&gotRequest); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		writeJSON(t, w, http.StatusCreated, approvalRequestJSON("pending", "approved"))
	})
	defer server.Close()

	output, err := runGovernanceCLI(t, server, []string{"approvals", "request", "--report-id", "report-1", "--actor", "alice"})
	if err != nil {
		t.Fatalf("approvals request: %v", err)
	}
	if gotRequest["actor"] != "alice" {
		t.Fatalf("request=%#v", gotRequest)
	}
	if !strings.Contains(output, "Status: pending") || !strings.Contains(output, "Requirements:") || !strings.Contains(output, "requirement-1") {
		t.Fatalf("output = %q", output)
	}
}

func TestApprovalsRequestDoesNotRequireActorAndOmitsItByDefault(t *testing.T) {
	var gotRequest map[string]string
	server := governanceCLIServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/breaking-reports/report-1/approval-request" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&gotRequest); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		writeJSON(t, w, http.StatusCreated, approvalRequestJSON("pending", ""))
	})
	defer server.Close()

	output, err := runGovernanceCLI(t, server, []string{"approvals", "request", "--report-id", "report-1"})
	if err != nil {
		t.Fatalf("approvals request: %v", err)
	}
	if _, ok := gotRequest["actor"]; ok {
		t.Fatalf("actor should be omitted by default: %#v", gotRequest)
	}
	if !strings.Contains(output, "Status: pending") || strings.Contains(output, "prr_token") {
		t.Fatalf("output = %q", output)
	}
}

func TestApprovalsRequestExistingRequestExitsZero(t *testing.T) {
	server := governanceCLIServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/breaking-reports/report-1/approval-request" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		writeJSON(t, w, http.StatusCreated, approvalRequestJSON("approved", "approved"))
	})
	defer server.Close()

	output, err := runGovernanceCLI(t, server, []string{"approvals", "request", "--report-id", "report-1", "--actor", "alice"})
	if exitCode(err) != 0 {
		t.Fatalf("exit code = %d err=%v", exitCode(err), err)
	}
	if !strings.Contains(output, "Status: approved") || !strings.Contains(output, "Decisions:") || strings.Contains(output, "prr_token") {
		t.Fatalf("output = %q", output)
	}
}

func TestApprovalsStatusPrintsStatusRequirementsAndDecisions(t *testing.T) {
	server := governanceCLIServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/breaking-reports/report-1/approval-status" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		writeJSON(t, w, http.StatusOK, approvalRequestJSON("approved", "approved"))
	})
	defer server.Close()

	output, err := runGovernanceCLI(t, server, []string{"approvals", "status", "--report-id", "report-1"})
	if err != nil {
		t.Fatalf("approvals status: %v", err)
	}
	for _, want := range []string{"Status: approved", "Requirements:", "Decisions:", "decision-1", "decided_by=alice"} {
		if !strings.Contains(output, want) {
			t.Fatalf("output missing %q: %q", want, output)
		}
	}
}

func TestApprovalsApproveRejectRequireInputs(t *testing.T) {
	app := App{}

	cases := [][]string{
		{"approvals", "approve", "--request-id", "request-1", "--actor", "alice"},
		{"approvals", "approve", "requirement-1", "--actor", "alice"},
	}
	for _, args := range cases {
		if err := app.Run(context.Background(), args); err == nil {
			t.Fatalf("args %v expected error", args)
		}
	}
}

func TestApprovalsApproveCallsExpectedEndpoint(t *testing.T) {
	var gotPath string
	var gotRequest map[string]string
	server := governanceCLIServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/approval-requests/request-1/requirements/requirement-1/approve" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&gotRequest); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		writeJSON(t, w, http.StatusOK, approvalRequestJSON("approved", "approved"))
	})
	defer server.Close()

	output, err := runGovernanceCLI(t, server, []string{"approvals", "approve", "requirement-1", "--request-id", "request-1", "--actor", "alice", "--comment", "ok"})
	if err != nil {
		t.Fatalf("approvals approve: %v", err)
	}
	if gotPath != "/api/v1/approval-requests/request-1/requirements/requirement-1/approve" || gotRequest["actor"] != "alice" || gotRequest["comment"] != "ok" {
		t.Fatalf("path=%q request=%#v", gotPath, gotRequest)
	}
	if !strings.Contains(output, "Status: approved") || !strings.Contains(output, "decided_by=alice") {
		t.Fatalf("output = %q", output)
	}
}

func TestApprovalsApproveDoesNotSendActorByDefault(t *testing.T) {
	var gotRequest map[string]string
	server := governanceCLIServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/approval-requests/request-1/requirements/requirement-1/approve" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&gotRequest); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		writeJSON(t, w, http.StatusOK, approvalRequestJSON("approved", "approved"))
	})
	defer server.Close()

	if _, err := runGovernanceCLI(t, server, []string{"approvals", "approve", "requirement-1", "--request-id", "request-1", "--comment", "ok"}); err != nil {
		t.Fatalf("approvals approve: %v", err)
	}
	if _, ok := gotRequest["actor"]; ok {
		t.Fatalf("actor should be omitted by default: %#v", gotRequest)
	}
	if gotRequest["comment"] != "ok" {
		t.Fatalf("request=%#v", gotRequest)
	}
}

func TestApprovalsRejectExitsZero(t *testing.T) {
	server := governanceCLIServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/approval-requests/request-1/requirements/requirement-1/reject" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		writeJSON(t, w, http.StatusOK, approvalRequestJSON("rejected", "rejected"))
	})
	defer server.Close()

	output, err := runGovernanceCLI(t, server, []string{"approvals", "reject", "requirement-1", "--request-id", "request-1", "--actor", "alice", "--comment", "no"})
	if exitCode(err) != 0 {
		t.Fatalf("exit code = %d err=%v", exitCode(err), err)
	}
	if !strings.Contains(output, "Status: rejected") {
		t.Fatalf("output = %q", output)
	}
}

func TestApprovalsRejectDoesNotSendActorByDefault(t *testing.T) {
	var gotRequest map[string]string
	server := governanceCLIServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/approval-requests/request-1/requirements/requirement-1/reject" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&gotRequest); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		writeJSON(t, w, http.StatusOK, approvalRequestJSON("rejected", "rejected"))
	})
	defer server.Close()

	output, err := runGovernanceCLI(t, server, []string{"approvals", "reject", "requirement-1", "--request-id", "request-1", "--comment", "no"})
	if err != nil {
		t.Fatalf("approvals reject: %v", err)
	}
	if _, ok := gotRequest["actor"]; ok {
		t.Fatalf("actor should be omitted by default: %#v", gotRequest)
	}
	if gotRequest["comment"] != "no" || !strings.Contains(output, "decided_by=alice") {
		t.Fatalf("request=%#v output=%q", gotRequest, output)
	}
}

func TestGovernanceActorOverrideRejectionIsClear(t *testing.T) {
	server := governanceCLIServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, http.StatusForbidden, map[string]any{
			"error": map[string]string{
				"code":    "forbidden",
				"message": "Governance actor is not allowed.",
			},
		})
	})
	defer server.Close()

	output, err := runGovernanceCLI(t, server, []string{"approvals", "approve", "requirement-1", "--request-id", "request-1", "--actor", "alice"})
	if exitCode(err) != 2 {
		t.Fatalf("exit code = %d err=%v", exitCode(err), err)
	}
	if err == nil || !strings.Contains(err.Error(), "server rejected the provided --actor") || !strings.Contains(err.Error(), "omit --actor") {
		t.Fatalf("error = %v", err)
	}
	if strings.Contains(output, "prr_token") || strings.Contains(err.Error(), "prr_token") {
		t.Fatalf("output=%q err=%v", output, err)
	}
}

func TestApprovalsApproveDuplicateConflictExitsTwo(t *testing.T) {
	server := governanceCLIServer(t, governanceConflictHandler(t))
	defer server.Close()

	output, err := runGovernanceCLI(t, server, []string{"approvals", "approve", "requirement-1", "--request-id", "request-1", "--actor", "alice"})
	assertGovernanceConflictExit(t, output, err)
}

func TestApprovalsRejectDuplicateConflictExitsTwo(t *testing.T) {
	server := governanceCLIServer(t, governanceConflictHandler(t))
	defer server.Close()

	output, err := runGovernanceCLI(t, server, []string{"approvals", "reject", "requirement-1", "--request-id", "request-1", "--actor", "alice"})
	assertGovernanceConflictExit(t, output, err)
}

func TestGovernanceAPIErrorExitsTwoAndDoesNotPrintToken(t *testing.T) {
	server := governanceCLIServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, http.StatusConflict, map[string]any{
			"error": map[string]string{
				"code":    "conflict",
				"message": "Requested operation conflicts with existing state. prr_token",
			},
		})
	})
	defer server.Close()

	output, err := runGovernanceCLI(t, server, []string{"approvals", "approve", "requirement-1", "--request-id", "request-1", "--actor", "alice"})
	if exitCode(err) != 2 {
		t.Fatalf("exit code = %d err=%v", exitCode(err), err)
	}
	if err == nil || !strings.Contains(err.Error(), "[redacted]") || strings.Contains(err.Error(), "prr_token") || strings.Contains(output, "prr_token") {
		t.Fatalf("output=%q err=%v", output, err)
	}
}

func governanceConflictHandler(t *testing.T) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, http.StatusConflict, map[string]any{
			"error": map[string]string{
				"code":    "conflict",
				"message": "Requested operation conflicts with existing state. prr_token",
			},
		})
	}
}

func assertGovernanceConflictExit(t *testing.T, output string, err error) {
	t.Helper()
	if exitCode(err) != 2 {
		t.Fatalf("exit code = %d err=%v", exitCode(err), err)
	}
	if err == nil || !strings.Contains(err.Error(), "Requested operation conflicts with existing state.") || !strings.Contains(err.Error(), "[redacted]") {
		t.Fatalf("error = %v", err)
	}
	if strings.Contains(err.Error(), "prr_token") || strings.Contains(output, "prr_token") {
		t.Fatalf("output=%q err=%v", output, err)
	}
}

func governanceCLIServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer prr_token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		handler(w, r)
	}))
}

func runGovernanceCLI(t *testing.T, server *httptest.Server, args []string) (string, error) {
	t.Helper()
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	saveCLIConfig(t, configPath, server.URL)
	var output bytes.Buffer
	app := App{ConfigPath: configPath, HTTPClient: server.Client(), Out: &output}
	err := app.Run(context.Background(), args)
	return output.String(), err
}

func approvalRequestJSON(status string, decision string) map[string]any {
	requirementStatus := "pending"
	decisions := []map[string]any{}
	received := 0
	if decision != "" {
		requirementStatus = decision
		received = 1
		decisions = append(decisions, map[string]any{
			"id":                  "decision-1",
			"approval_request_id": "request-1",
			"requirement_id":      "requirement-1",
			"decision":            decision,
			"decided_by":          "alice",
			"comment":             "ok",
			"created_at":          "2026-06-05T12:00:00Z",
		})
	}
	return map[string]any{
		"id":                 "request-1",
		"module_id":          "module-user-api",
		"module_name":        "user-api",
		"breaking_report_id": "report-1",
		"target_ref":         "feature",
		"status":             status,
		"required_approvals": 1,
		"received_approvals": received,
		"created_at":         "2026-06-05T12:00:00Z",
		"updated_at":         "2026-06-05T12:00:00Z",
		"requirements": []map[string]any{{
			"id":                  "requirement-1",
			"approval_request_id": "request-1",
			"requirement_type":    "module_owner_approval",
			"target_module_id":    "module-user-api",
			"target_module_name":  "user-api",
			"required_role":       "owner",
			"status":              requirementStatus,
			"reason":              "Breaking changes require approval from module owner.",
			"created_at":          "2026-06-05T12:00:00Z",
			"updated_at":          "2026-06-05T12:00:00Z",
		}},
		"decisions": decisions,
	}
}
