package httptransport

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alryzden/ProtoRadar/internal/domain"
	"github.com/alryzden/ProtoRadar/internal/usecase/governance"
)

func TestGovernanceOwnerEndpointsRequireAuth(t *testing.T) {
	server := newGovernanceTestServer(newFakeGovernance())

	tests := []struct {
		method string
		path   string
		body   string
	}{
		{method: http.MethodGet, path: "/api/v1/modules/user-api/owners"},
		{method: http.MethodPost, path: "/api/v1/modules/user-api/owners", body: `{"subject_type":"user","subject":"alice","role":"owner"}`},
		{method: http.MethodDelete, path: "/api/v1/modules/user-api/owners/owner-1"},
	}
	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			res := request(t, server, tt.method, tt.path, strings.NewReader(tt.body), "", "application/json")
			if res.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want %d", res.Code, http.StatusUnauthorized)
			}
			assertAPIError(t, res, "unauthorized")
		})
	}
}

func TestGovernanceApprovalEndpointsRequireAuth(t *testing.T) {
	server := newGovernanceTestServer(newFakeGovernance())

	tests := []struct {
		method string
		path   string
		body   string
	}{
		{method: http.MethodPost, path: "/api/v1/breaking-reports/report-1/approval-request", body: `{}`},
		{method: http.MethodPost, path: "/api/v1/approval-requests/request-1/requirements/requirement-1/approve", body: `{"comment":"ok"}`},
		{method: http.MethodPost, path: "/api/v1/approval-requests/request-1/requirements/requirement-1/reject", body: `{"comment":"no"}`},
	}
	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			res := request(t, server, tt.method, tt.path, strings.NewReader(tt.body), "", "application/json")
			if res.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want %d", res.Code, http.StatusUnauthorized)
			}
			assertAPIError(t, res, "unauthorized")
		})
	}
}

func TestListModuleOwnersSuccess(t *testing.T) {
	fake := newFakeGovernance()
	fake.owners = []domain.ModuleOwner{testHTTPModuleOwner(t, "owner-1", "user-api", "user", "alice", "owner")}
	server := newGovernanceTestServer(fake)

	res := request(t, server, http.MethodGet, "/api/v1/modules/user-api/owners", nil, "Bearer valid", "")
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	var body listModuleOwnersResponse
	decodeResponse(t, res, &body)
	if body.Module != "user-api" || len(body.Owners) != 1 || body.Owners[0].Subject != "alice" {
		t.Fatalf("body = %#v", body)
	}
}

func TestAddModuleOwnerSuccess(t *testing.T) {
	fake := newFakeGovernance()
	server := newGovernanceTestServer(fake)

	res := request(t, server, http.MethodPost, "/api/v1/modules/user-api/owners", strings.NewReader(`{
		"subject_type": "team",
		"subject": "platform-team",
		"role": "owner"
	}`), "Bearer valid", "application/json")
	if res.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	var body moduleOwnerDTO
	decodeResponse(t, res, &body)
	if body.SubjectType != "team" || body.Subject != "platform-team" || body.Role != "owner" {
		t.Fatalf("body = %#v", body)
	}
	if fake.addOwnerInput.Actor != "test" {
		t.Fatalf("actor = %q, want principal subject", fake.addOwnerInput.Actor)
	}
}

func TestAddModuleOwnerSameActorAsPrincipalSucceeds(t *testing.T) {
	fake := newFakeGovernance()
	server := newGovernanceTestServer(fake)

	res := request(t, server, http.MethodPost, "/api/v1/modules/user-api/owners", strings.NewReader(`{
		"subject_type": "team",
		"subject": "platform-team",
		"role": "owner",
		"actor": "test"
	}`), "Bearer valid", "application/json")
	if res.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	if fake.addOwnerInput.Actor != "test" {
		t.Fatalf("actor = %q, want principal subject", fake.addOwnerInput.Actor)
	}
}

func TestAddModuleOwnerRejectsSpoofedActorByDefault(t *testing.T) {
	fake := newFakeGovernance()
	server := newGovernanceTestServer(fake)

	res := request(t, server, http.MethodPost, "/api/v1/modules/user-api/owners", strings.NewReader(`{
		"subject_type": "team",
		"subject": "platform-team",
		"role": "owner",
		"actor": "alice"
	}`), "Bearer valid", "application/json")
	if res.Code != http.StatusForbidden {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	if fake.addOwnerInput.Actor != "" {
		t.Fatalf("usecase should not be called: %#v", fake.addOwnerInput)
	}
}

func TestAddModuleOwnerAllowsActorOverrideWhenEnabled(t *testing.T) {
	fake := newFakeGovernance()
	server := newGovernanceTestServerWithOptions(fake, Options{GovernanceActorOverrideEnabled: true})

	res := request(t, server, http.MethodPost, "/api/v1/modules/user-api/owners", strings.NewReader(`{
		"subject_type": "team",
		"subject": "platform-team",
		"role": "owner",
		"actor": "alice"
	}`), "Bearer valid", "application/json")
	if res.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	if fake.addOwnerInput.Actor != "alice" {
		t.Fatalf("actor = %q, want override", fake.addOwnerInput.Actor)
	}
}

func TestAddModuleOwnerInvalidSubjectTypeReturnsBadRequest(t *testing.T) {
	server := newGovernanceTestServer(newFakeGovernance())

	res := request(t, server, http.MethodPost, "/api/v1/modules/user-api/owners", strings.NewReader(`{
		"subject_type": "group",
		"subject": "platform-team",
		"role": "owner"
	}`), "Bearer valid", "application/json")
	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	assertAPIError(t, res, "validation_error")
}

func TestAddModuleOwnerInvalidRoleReturnsBadRequest(t *testing.T) {
	server := newGovernanceTestServer(newFakeGovernance())

	res := request(t, server, http.MethodPost, "/api/v1/modules/user-api/owners", strings.NewReader(`{
		"subject_type": "user",
		"subject": "alice",
		"role": "admin"
	}`), "Bearer valid", "application/json")
	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	assertAPIError(t, res, "validation_error")
}

func TestMappedGovernanceCheckViolationReturnsValidationWithoutSQLLeak(t *testing.T) {
	fake := newFakeGovernance()
	fake.addOwnerErr = domain.ErrInvalidModuleOwnerRole
	server := newGovernanceTestServer(fake)

	res := request(t, server, http.MethodPost, "/api/v1/modules/user-api/owners", strings.NewReader(`{
		"subject_type": "user",
		"subject": "alice",
		"role": "owner"
	}`), "Bearer valid", "application/json")
	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	assertAPIError(t, res, "validation_error")
	for _, forbidden := range []string{"SQLSTATE", "23514", "module_owners_role_check", "CHECK"} {
		if strings.Contains(res.Body.String(), forbidden) {
			t.Fatalf("response leaked %q: %s", forbidden, res.Body.String())
		}
	}
}

func TestAddModuleOwnerDuplicateReturnsConflict(t *testing.T) {
	fake := newFakeGovernance()
	fake.addOwnerErr = governance.ErrModuleOwnerAlreadyExists
	server := newGovernanceTestServer(fake)

	res := request(t, server, http.MethodPost, "/api/v1/modules/user-api/owners", strings.NewReader(`{
		"subject_type": "user",
		"subject": "alice",
		"role": "owner"
	}`), "Bearer valid", "application/json")
	if res.Code != http.StatusConflict {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	assertAPIError(t, res, "conflict")
}

func TestRemoveModuleOwnerSuccess(t *testing.T) {
	fake := newFakeGovernance()
	server := newGovernanceTestServer(fake)

	res := request(t, server, http.MethodDelete, "/api/v1/modules/user-api/owners/owner-1", nil, "Bearer valid", "")
	if res.Code != http.StatusNoContent {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	if fake.removeOwnerInput.Actor != "test" {
		t.Fatalf("actor = %q, want principal subject", fake.removeOwnerInput.Actor)
	}
}

func TestRemoveModuleOwnerRejectsSpoofedActorByDefault(t *testing.T) {
	fake := newFakeGovernance()
	server := newGovernanceTestServer(fake)

	res := request(t, server, http.MethodDelete, "/api/v1/modules/user-api/owners/owner-1?actor=alice", nil, "Bearer valid", "")
	if res.Code != http.StatusForbidden {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	if fake.removeOwnerInput.Actor != "" {
		t.Fatalf("usecase should not be called: %#v", fake.removeOwnerInput)
	}
}

func TestRemoveUnknownOwnerReturnsNotFound(t *testing.T) {
	fake := newFakeGovernance()
	fake.removeOwnerErr = governance.ErrModuleOwnerNotFound
	server := newGovernanceTestServer(fake)

	res := request(t, server, http.MethodDelete, "/api/v1/modules/user-api/owners/missing-owner", nil, "Bearer valid", "")
	if res.Code != http.StatusNotFound {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	assertAPIError(t, res, "not_found")
}

func TestCreateApprovalRequestSuccess(t *testing.T) {
	fake := newFakeGovernance()
	server := newGovernanceTestServer(fake)

	res := request(t, server, http.MethodPost, "/api/v1/breaking-reports/report-1/approval-request", strings.NewReader(`{}`), "Bearer valid", "application/json")
	if res.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	var body approvalRequestDTO
	decodeResponse(t, res, &body)
	if body.ID != "request-1" || body.Status != "pending" || len(body.Requirements) != 1 {
		t.Fatalf("body = %#v", body)
	}
	if fake.createApprovalInput.Actor != "test" {
		t.Fatalf("actor = %q, want principal subject", fake.createApprovalInput.Actor)
	}
}

func TestCreateApprovalRequestRejectsSpoofedActorByDefault(t *testing.T) {
	fake := newFakeGovernance()
	server := newGovernanceTestServer(fake)

	res := request(t, server, http.MethodPost, "/api/v1/breaking-reports/report-1/approval-request", strings.NewReader(`{"actor":"alice"}`), "Bearer valid", "application/json")
	if res.Code != http.StatusForbidden {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	if fake.createApprovalInput.Actor != "" {
		t.Fatalf("usecase should not be called: %#v", fake.createApprovalInput)
	}
}

func TestCreateApprovalRequestForPassedReportReturnsNotRequired(t *testing.T) {
	server := newGovernanceTestServer(newFakeGovernance())

	res := request(t, server, http.MethodPost, "/api/v1/breaking-reports/passed-report/approval-request", strings.NewReader(`{"actor":"test"}`), "Bearer valid", "application/json")
	if res.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	var body approvalRequestDTO
	decodeResponse(t, res, &body)
	if body.Status != "not_required" || len(body.Requirements) != 0 {
		t.Fatalf("body = %#v", body)
	}
}

func TestDuplicateApprovalRequestReturnsExistingSuccess(t *testing.T) {
	fake := newFakeGovernance()
	existing := fake.approvalRequest(domain.ApprovalRequestStatusApproved)
	existing.ID = domain.NewApprovalRequestID("existing-request")
	existing.Requirements[0].ApprovalRequestID = existing.ID
	existing.Requirements[0].Status = domain.ApprovalRequirementStatusApproved
	existing.ReceivedApprovals = 1
	existing.Decisions = []domain.ApprovalDecision{{
		ID:                domain.NewApprovalDecisionID("decision-existing"),
		ApprovalRequestID: existing.ID,
		RequirementID:     existing.Requirements[0].ID,
		Decision:          domain.ApprovalDecisionValueApproved,
		DecidedBy:         "alice",
		CreatedAt:         fake.now,
	}}
	fake.createApprovalResponse = existing
	server := newGovernanceTestServer(fake)

	res := request(t, server, http.MethodPost, "/api/v1/breaking-reports/report-1/approval-request", strings.NewReader(`{"actor":"test"}`), "Bearer valid", "application/json")
	if res.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	var body approvalRequestDTO
	decodeResponse(t, res, &body)
	if body.ID != "existing-request" || body.Status != "approved" || len(body.Decisions) != 1 {
		t.Fatalf("body = %#v", body)
	}
}

func TestCreateApprovalRequestUnknownReportReturnsNotFound(t *testing.T) {
	fake := newFakeGovernance()
	fake.createApprovalErr = governance.ErrBreakingReportNotFound
	server := newGovernanceTestServer(fake)

	res := request(t, server, http.MethodPost, "/api/v1/breaking-reports/missing-report/approval-request", strings.NewReader(`{"actor":"test"}`), "Bearer valid", "application/json")
	if res.Code != http.StatusNotFound {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	assertAPIError(t, res, "not_found")
}

func TestGetApprovalStatusSuccess(t *testing.T) {
	server := newGovernanceTestServer(newFakeGovernance())

	res := request(t, server, http.MethodGet, "/api/v1/breaking-reports/report-1/approval-status", nil, "Bearer valid", "")
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	var body approvalRequestDTO
	decodeResponse(t, res, &body)
	if body.ID != "request-1" || body.Status != "pending" {
		t.Fatalf("body = %#v", body)
	}
}

func TestApproveRequirementSuccess(t *testing.T) {
	fake := newFakeGovernance()
	server := newGovernanceTestServer(fake)

	res := request(t, server, http.MethodPost, "/api/v1/approval-requests/request-1/requirements/requirement-1/approve", strings.NewReader(`{
		"comment": "Approved because consumers have been notified."
	}`), "Bearer valid", "application/json")
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	var body approvalRequestDTO
	decodeResponse(t, res, &body)
	if body.Status != "approved" || len(body.Decisions) != 1 || body.Decisions[0].Decision != "approved" {
		t.Fatalf("body = %#v", body)
	}
	if fake.decisionInput.Actor != "test" || body.Decisions[0].DecidedBy != "test" {
		t.Fatalf("input=%#v decision=%#v", fake.decisionInput, body.Decisions[0])
	}
}

func TestApproveRequirementDoesNotUseRawBearerTokenAsActor(t *testing.T) {
	fake := newFakeGovernance()
	server := newGovernanceTestServer(fake)

	res := request(t, server, http.MethodPost, "/api/v1/approval-requests/request-1/requirements/requirement-1/approve", strings.NewReader(`{"comment":"ok"}`), "Bearer valid", "application/json")
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	if fake.decisionInput.Actor != "test" || fake.decisionInput.Actor == "valid" {
		t.Fatalf("actor = %q, want principal subject and not raw token", fake.decisionInput.Actor)
	}
}

func TestApproveRequirementRejectsSpoofedActorByDefault(t *testing.T) {
	fake := newFakeGovernance()
	server := newGovernanceTestServer(fake)

	res := request(t, server, http.MethodPost, "/api/v1/approval-requests/request-1/requirements/requirement-1/approve", strings.NewReader(`{
		"actor": "alice",
		"comment": "Approved because consumers have been notified."
	}`), "Bearer valid", "application/json")
	if res.Code != http.StatusForbidden {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	if fake.decisionInput.Actor != "" {
		t.Fatalf("usecase should not be called: %#v", fake.decisionInput)
	}
}

func TestRejectRequirementSuccess(t *testing.T) {
	fake := newFakeGovernance()
	server := newGovernanceTestServer(fake)

	res := request(t, server, http.MethodPost, "/api/v1/approval-requests/request-1/requirements/requirement-1/reject", strings.NewReader(`{
		"comment": "Rejected because billing-api is still using this version in production."
	}`), "Bearer valid", "application/json")
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	var body approvalRequestDTO
	decodeResponse(t, res, &body)
	if body.Status != "rejected" || len(body.Decisions) != 1 || body.Decisions[0].Decision != "rejected" {
		t.Fatalf("body = %#v", body)
	}
	if fake.decisionInput.Actor != "test" || body.Decisions[0].DecidedBy != "test" {
		t.Fatalf("input=%#v decision=%#v", fake.decisionInput, body.Decisions[0])
	}
}

func TestRepeatedApprovalReturnsConflict(t *testing.T) {
	fake := newFakeGovernance()
	fake.decisionErr = governance.ErrApprovalRequirementConflict
	server := newGovernanceTestServer(fake)

	res := request(t, server, http.MethodPost, "/api/v1/approval-requests/request-1/requirements/requirement-1/approve", strings.NewReader(`{"actor":"test"}`), "Bearer valid", "application/json")
	if res.Code != http.StatusConflict {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	assertConflictError(t, res)
}

func TestRepeatedRejectionReturnsConflict(t *testing.T) {
	fake := newFakeGovernance()
	fake.decisionErr = governance.ErrApprovalRequirementConflict
	server := newGovernanceTestServer(fake)

	res := request(t, server, http.MethodPost, "/api/v1/approval-requests/request-1/requirements/requirement-1/reject", strings.NewReader(`{"actor":"test"}`), "Bearer valid", "application/json")
	if res.Code != http.StatusConflict {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	assertConflictError(t, res)
}

func TestInvalidApprovalActorReturnsBadRequest(t *testing.T) {
	fake := newFakeGovernance()
	fake.decisionErr = governance.ErrInvalidApprovalActor
	server := newGovernanceTestServer(fake)

	res := request(t, server, http.MethodPost, "/api/v1/approval-requests/request-1/requirements/requirement-1/approve", strings.NewReader(`{"actor":""}`), "Bearer valid", "application/json")
	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	assertAPIError(t, res, "validation_error")
}

func TestUnauthorizedApprovalActorReturnsForbidden(t *testing.T) {
	fake := newFakeGovernance()
	fake.decisionErr = governance.ErrApprovalActorForbidden
	server := newGovernanceTestServer(fake)

	res := request(t, server, http.MethodPost, "/api/v1/approval-requests/request-1/requirements/requirement-1/approve", strings.NewReader(`{"actor":"mallory"}`), "Bearer valid", "application/json")
	if res.Code != http.StatusForbidden {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	assertAPIError(t, res, "forbidden")
}

func TestGetApprovalRequestAuditReturnsEvents(t *testing.T) {
	server := newGovernanceTestServer(newFakeGovernance())

	res := request(t, server, http.MethodGet, "/api/v1/approval-requests/request-1/audit", nil, "Bearer valid", "")
	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	var body approvalAuditResponse
	decodeResponse(t, res, &body)
	if body.ApprovalRequestID != "request-1" || len(body.Events) != 1 || body.Events[0].EventType != "approval_request_created" {
		t.Fatalf("body = %#v", body)
	}
}

func TestGovernanceInternalErrorDoesNotExposeStackTrace(t *testing.T) {
	fake := newFakeGovernance()
	fake.listOwnersErr = errors.New("panic: stack trace secret.go SQLSTATE 42P01")
	server := newGovernanceTestServer(fake)

	res := request(t, server, http.MethodGet, "/api/v1/modules/user-api/owners", nil, "Bearer valid", "")
	if res.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, body = %s", res.Code, res.Body.String())
	}
	assertAPIError(t, res, "internal_error")
}

func newGovernanceTestServer(fake *fakeGovernance) http.Handler {
	return newGovernanceTestServerWithOptions(fake, Options{})
}

func newGovernanceTestServerWithOptions(fake *fakeGovernance, options Options) http.Handler {
	registry := newFakeRegistry()
	options.BootstrapToken = "bootstrap"
	options.Runtime = registry
	options.Governance = fake
	return NewServer(registry, options).Handler()
}

type fakeGovernance struct {
	now                    time.Time
	owners                 []domain.ModuleOwner
	addOwnerErr            error
	removeOwnerErr         error
	listOwnersErr          error
	createApprovalErr      error
	createApprovalResponse domain.ApprovalRequest
	decisionErr            error
	addOwnerInput          governance.AddModuleOwnerInput
	removeOwnerInput       governance.RemoveModuleOwnerInput
	createApprovalInput    governance.CreateApprovalRequestForBreakingReportInput
	decisionInput          governance.ApprovalDecisionInput
	addOwnerCalls          int
	removeOwnerCalls       int
	listOwnersCalls        int
	createApprovalCalls    int
	getApprovalStatusCalls int
	approveCalls           int
	rejectCalls            int
	listAuditCalls         int
}

func newFakeGovernance() *fakeGovernance {
	return &fakeGovernance{now: time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)}
}

func (fake *fakeGovernance) AddModuleOwner(ctx context.Context, input governance.AddModuleOwnerInput) (domain.ModuleOwner, error) {
	fake.addOwnerCalls++
	fake.addOwnerInput = input
	if fake.addOwnerErr != nil {
		return domain.ModuleOwner{}, fake.addOwnerErr
	}
	subjectType, err := domain.NewGovernanceSubjectType(input.SubjectType)
	if err != nil {
		return domain.ModuleOwner{}, governance.ErrInvalidSubjectType
	}
	role, err := domain.NewModuleOwnerRole(input.Role)
	if err != nil {
		return domain.ModuleOwner{}, governance.ErrInvalidModuleOwnerRole
	}
	if strings.TrimSpace(input.Subject) == "" {
		return domain.ModuleOwner{}, governance.ErrInvalidSubject
	}
	moduleName, err := domain.NewModuleName(input.ModuleName)
	if err != nil {
		return domain.ModuleOwner{}, governance.ErrInvalidModuleName
	}
	owner := domain.ModuleOwner{
		ID:          domain.NewModuleOwnerID("owner-1"),
		ModuleID:    domain.NewModuleID("module-" + input.ModuleName),
		ModuleName:  moduleName,
		SubjectType: subjectType,
		Subject:     input.Subject,
		Role:        role,
		CreatedAt:   fake.now,
		UpdatedAt:   fake.now,
	}
	fake.owners = append(fake.owners, owner)
	return owner, nil
}

func (fake *fakeGovernance) RemoveModuleOwner(ctx context.Context, input governance.RemoveModuleOwnerInput) error {
	fake.removeOwnerCalls++
	fake.removeOwnerInput = input
	if fake.removeOwnerErr != nil {
		return fake.removeOwnerErr
	}
	return nil
}

func (fake *fakeGovernance) ListModuleOwners(ctx context.Context, input governance.ListModuleOwnersInput) ([]domain.ModuleOwner, error) {
	fake.listOwnersCalls++
	if fake.listOwnersErr != nil {
		return nil, fake.listOwnersErr
	}
	return fake.owners, nil
}

func (fake *fakeGovernance) CreateApprovalRequestForBreakingReport(ctx context.Context, input governance.CreateApprovalRequestForBreakingReportInput) (domain.ApprovalRequest, error) {
	fake.createApprovalCalls++
	fake.createApprovalInput = input
	if fake.createApprovalErr != nil {
		return domain.ApprovalRequest{}, fake.createApprovalErr
	}
	if fake.createApprovalResponse.ID != "" {
		return fake.createApprovalResponse, nil
	}
	if input.BreakingReportID == "passed-report" {
		request := fake.approvalRequest(domain.ApprovalRequestStatusNotRequired)
		request.Requirements = nil
		request.RequiredApprovals = 0
		return request, nil
	}
	return fake.approvalRequest(domain.ApprovalRequestStatusPending), nil
}

func (fake *fakeGovernance) GetApprovalStatusForBreakingReport(ctx context.Context, input governance.GetApprovalStatusForBreakingReportInput) (domain.ApprovalRequest, error) {
	fake.getApprovalStatusCalls++
	return fake.approvalRequest(domain.ApprovalRequestStatusPending), nil
}

func (fake *fakeGovernance) ApproveRequirement(ctx context.Context, input governance.ApprovalDecisionInput) (domain.ApprovalRequest, error) {
	fake.approveCalls++
	fake.decisionInput = input
	if fake.decisionErr != nil {
		return domain.ApprovalRequest{}, fake.decisionErr
	}
	request := fake.approvalRequest(domain.ApprovalRequestStatusApproved)
	request.Requirements[0].Status = domain.ApprovalRequirementStatusApproved
	request.ReceivedApprovals = 1
	request.Decisions = []domain.ApprovalDecision{fake.approvalDecision(domain.ApprovalDecisionValueApproved, input)}
	return request, nil
}

func (fake *fakeGovernance) RejectRequirement(ctx context.Context, input governance.ApprovalDecisionInput) (domain.ApprovalRequest, error) {
	fake.rejectCalls++
	fake.decisionInput = input
	if fake.decisionErr != nil {
		return domain.ApprovalRequest{}, fake.decisionErr
	}
	request := fake.approvalRequest(domain.ApprovalRequestStatusRejected)
	request.Requirements[0].Status = domain.ApprovalRequirementStatusRejected
	request.Decisions = []domain.ApprovalDecision{fake.approvalDecision(domain.ApprovalDecisionValueRejected, input)}
	return request, nil
}

func (fake *fakeGovernance) ListApprovalRequestAudit(ctx context.Context, input governance.ListApprovalRequestAuditInput) ([]domain.GovernanceAuditEvent, error) {
	fake.listAuditCalls++
	requestID := domain.NewApprovalRequestID(input.RequestID)
	moduleID := domain.NewModuleID("module-user-api")
	reportID := domain.NewBreakingReportID("report-1")
	moduleName, _ := domain.NewModuleName("user-api")
	return []domain.GovernanceAuditEvent{{
		ID:                domain.NewGovernanceAuditEventID("audit-1"),
		EventType:         domain.GovernanceAuditEventTypeApprovalRequestCreated,
		Actor:             "alice",
		ModuleID:          &moduleID,
		ModuleName:        moduleName,
		ApprovalRequestID: &requestID,
		BreakingReportID:  &reportID,
		PayloadJSON:       []byte(`{"requirement_count":1}`),
		CreatedAt:         fake.now,
	}}, nil
}

func (fake *fakeGovernance) totalCalls() int {
	return fake.addOwnerCalls +
		fake.removeOwnerCalls +
		fake.listOwnersCalls +
		fake.createApprovalCalls +
		fake.getApprovalStatusCalls +
		fake.approveCalls +
		fake.rejectCalls +
		fake.listAuditCalls
}

func (fake *fakeGovernance) approvalRequest(status domain.ApprovalRequestStatus) domain.ApprovalRequest {
	moduleName, _ := domain.NewModuleName("user-api")
	requestID := domain.NewApprovalRequestID("request-1")
	reportID := domain.NewBreakingReportID("report-1")
	moduleID := domain.NewModuleID("module-user-api")
	requirementID := domain.NewApprovalRequirementID("requirement-1")
	return domain.ApprovalRequest{
		ID:                requestID,
		ModuleID:          moduleID,
		ModuleName:        moduleName,
		BreakingReportID:  &reportID,
		TargetRef:         "feature/governance",
		Status:            status,
		RequiredApprovals: 1,
		CreatedAt:         fake.now,
		UpdatedAt:         fake.now,
		Requirements: []domain.ApprovalRequirement{{
			ID:                requirementID,
			ApprovalRequestID: requestID,
			RequirementType:   domain.ApprovalRequirementTypeModuleOwnerApproval,
			TargetModuleID:    &moduleID,
			TargetModuleName:  moduleName,
			RequiredRole:      domain.ModuleOwnerRoleOwner,
			Status:            domain.ApprovalRequirementStatusPending,
			Reason:            "Breaking changes require approval from module owner.",
			CreatedAt:         fake.now,
			UpdatedAt:         fake.now,
		}},
	}
}

func (fake *fakeGovernance) approvalDecision(decision domain.ApprovalDecisionValue, input governance.ApprovalDecisionInput) domain.ApprovalDecision {
	return domain.ApprovalDecision{
		ID:                domain.NewApprovalDecisionID("decision-1"),
		ApprovalRequestID: domain.NewApprovalRequestID(input.RequestID),
		RequirementID:     domain.NewApprovalRequirementID(input.RequirementID),
		Decision:          decision,
		DecidedBy:         input.Actor,
		Comment:           input.Comment,
		CreatedAt:         fake.now,
	}
}

func assertConflictError(t *testing.T, res *httptest.ResponseRecorder) {
	t.Helper()
	var body errorResponse
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Error.Code != "conflict" {
		t.Fatalf("error code = %q, want conflict; body = %#v", body.Error.Code, body)
	}
	if body.Error.Message != "Requested operation conflicts with existing state." {
		t.Fatalf("conflict message = %q", body.Error.Message)
	}
	for _, forbidden := range []string{"stack trace", "secret.go", "panic:", "pq:", "SQLSTATE"} {
		if strings.Contains(body.Error.Message, forbidden) {
			t.Fatalf("error message leaked %q: %#v", forbidden, body)
		}
	}
}

func testHTTPModuleOwner(t testing.TB, id string, module string, subjectTypeValue string, subject string, roleValue string) domain.ModuleOwner {
	t.Helper()
	moduleName, err := domain.NewModuleName(module)
	if err != nil {
		t.Fatalf("module name: %v", err)
	}
	subjectType, err := domain.NewGovernanceSubjectType(subjectTypeValue)
	if err != nil {
		t.Fatalf("subject type: %v", err)
	}
	role, err := domain.NewModuleOwnerRole(roleValue)
	if err != nil {
		t.Fatalf("role: %v", err)
	}
	now := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	return domain.ModuleOwner{
		ID:          domain.NewModuleOwnerID(id),
		ModuleID:    domain.NewModuleID("module-" + module),
		ModuleName:  moduleName,
		SubjectType: subjectType,
		Subject:     subject,
		Role:        role,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}
