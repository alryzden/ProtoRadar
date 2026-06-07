package governance

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/alryzden/ProtoRadar/internal/audit"
	"github.com/alryzden/ProtoRadar/internal/domain"
)

func TestCreateApprovalRequestForPassedReportCreatesNotRequiredRequest(t *testing.T) {
	fixture := newApprovalFixture(t)
	fixture.policy.result = GovernancePolicyResult{
		Status:     domain.ApprovalRequestStatusNotRequired,
		Reasons:    []GovernancePolicyReason{GovernancePolicyReasonNoBreakingChanges},
		ReasonText: policyReasonNoBreakingChanges,
	}

	request, err := fixture.workflow.CreateApprovalRequestForBreakingReport(context.Background(), CreateApprovalRequestForBreakingReportInput{
		BreakingReportID: "report-1",
		Actor:            "admin",
	})
	if err != nil {
		t.Fatalf("create approval request: %v", err)
	}
	if request.Status != domain.ApprovalRequestStatusNotRequired {
		t.Fatalf("status = %q", request.Status)
	}
	if len(request.Requirements) != 0 || request.RequiredApprovals != 0 {
		t.Fatalf("request = %#v", request)
	}
	if len(fixture.audit.events) != 1 || fixture.audit.events[0].EventType != domain.GovernanceAuditEventTypeApprovalRequestCreated {
		t.Fatalf("audit events = %#v", fixture.audit.events)
	}
	if fixture.audit.events[0].Actor != "admin" {
		t.Fatalf("audit actor = %q, want admin", fixture.audit.events[0].Actor)
	}
	assertAuditPayloadSafe(t, fixture.audit.events[0].PayloadJSON)
	if len(fixture.outbox.records) != 1 || fixture.outbox.records[0].EventType != "protoradar.approval_request.created" {
		t.Fatalf("outbox records = %#v", fixture.outbox.records)
	}
	assertOutboxActor(t, fixture.outbox.records[0].Payload, "admin")
	assertAuditPayloadSafe(t, fixture.outbox.records[0].Payload)
}

func TestCreateApprovalRequestForBreakingReportCreatesPendingOwnerRequirement(t *testing.T) {
	fixture := newApprovalFixture(t)
	fixture.policy.result = ownerApprovalPolicyResult(fixture.moduleID, fixture.moduleName, nil)

	request, err := fixture.workflow.CreateApprovalRequestForBreakingReport(context.Background(), CreateApprovalRequestForBreakingReportInput{BreakingReportID: "report-1", Actor: "admin"})
	if err != nil {
		t.Fatalf("create approval request: %v", err)
	}
	if request.Status != domain.ApprovalRequestStatusPending || request.RequiredApprovals != 1 {
		t.Fatalf("request = %#v", request)
	}
	requirement := request.Requirements[0]
	if requirement.RequirementType != domain.ApprovalRequirementTypeModuleOwnerApproval || requirement.TargetModuleName != fixture.moduleName {
		t.Fatalf("requirement = %#v", requirement)
	}
	if requirement.Reason != policyReasonModuleOwnerApproval {
		t.Fatalf("reason = %q", requirement.Reason)
	}
}

func TestCreateApprovalRequestWithProductionRuntimeImpactCreatesAffectedConsumerRequirement(t *testing.T) {
	fixture := newApprovalFixture(t)
	billingName := mustModuleName(t, "billing-api")
	billingID := domain.NewModuleID("module-2")
	fixture.dependencies.affected = []domain.AffectedModule{{ModuleName: billingName}}
	fixture.runtime.impacts = []domain.RuntimeImpact{runtimeImpact(t, "prod", "billing-api")}
	fixture.policy.result = GovernancePolicyResult{
		ApprovalRequired: true,
		Status:           domain.ApprovalRequestStatusPending,
		Reasons:          []GovernancePolicyReason{GovernancePolicyReasonModuleOwnerRequired, GovernancePolicyReasonProductionRuntimeConsumer},
		Requirements: []GovernanceRequirementPlan{
			ownerRequirementPlan(fixture.moduleID, fixture.moduleName, nil),
			{
				RequirementType:  domain.ApprovalRequirementTypeAffectedConsumerApproval,
				TargetModuleID:   &billingID,
				TargetModuleName: billingName,
				RequiredRole:     domain.ModuleOwnerRoleOwner,
				Status:           domain.ApprovalRequirementStatusPending,
				ReasonText:       policyReasonAffectedConsumerApproval,
			},
		},
	}

	request, err := fixture.workflow.CreateApprovalRequestForBreakingReport(context.Background(), CreateApprovalRequestForBreakingReportInput{BreakingReportID: "report-1", Actor: "admin"})
	if err != nil {
		t.Fatalf("create approval request: %v", err)
	}
	if len(fixture.policy.input.AffectedModules) != 1 || len(fixture.policy.input.RuntimeImpact) != 1 {
		t.Fatalf("policy input affected=%#v runtime=%#v", fixture.policy.input.AffectedModules, fixture.policy.input.RuntimeImpact)
	}
	if len(request.Requirements) != 2 {
		t.Fatalf("requirements = %#v", request.Requirements)
	}
	if request.Requirements[1].RequirementType != domain.ApprovalRequirementTypeAffectedConsumerApproval {
		t.Fatalf("requirement = %#v", request.Requirements[1])
	}
}

func TestCreateApprovalRequestWithMissingOwnersPersistsVisibleWarning(t *testing.T) {
	fixture := newApprovalFixture(t)
	fixture.policy.result = ownerApprovalPolicyResult(fixture.moduleID, fixture.moduleName, []GovernancePolicyWarning{{
		RequirementType:  domain.ApprovalRequirementTypeModuleOwnerApproval,
		TargetModuleID:   &fixture.moduleID,
		TargetModuleName: fixture.moduleName,
		Reason:           GovernancePolicyReasonMissingModuleOwner,
		Message:          policyWarningMissingOwners,
	}})

	request, err := fixture.workflow.CreateApprovalRequestForBreakingReport(context.Background(), CreateApprovalRequestForBreakingReportInput{BreakingReportID: "report-1", Actor: "admin"})
	if err != nil {
		t.Fatalf("create approval request: %v", err)
	}
	if request.Status != domain.ApprovalRequestStatusPending || len(request.Requirements) != 1 {
		t.Fatalf("request = %#v", request)
	}
	if !strings.Contains(request.Requirements[0].Reason, policyWarningMissingOwners) {
		t.Fatalf("requirement reason = %q", request.Requirements[0].Reason)
	}
}

func TestCreateApprovalRequestRejectsEmptyActor(t *testing.T) {
	fixture := newApprovalFixture(t)

	_, err := fixture.workflow.CreateApprovalRequestForBreakingReport(context.Background(), CreateApprovalRequestForBreakingReportInput{BreakingReportID: "report-1"})
	if !errors.Is(err, ErrInvalidApprovalActor) {
		t.Fatalf("error = %v, want ErrInvalidApprovalActor", err)
	}
	if fixture.policy.called || len(fixture.audit.events) != 0 || len(fixture.outbox.records) != 0 {
		t.Fatalf("empty actor should not evaluate policy or write audit/outbox")
	}
}

func TestDuplicateCreateApprovalRequestReturnsExisting(t *testing.T) {
	fixture := newApprovalFixture(t)
	existing := fixture.existingRequest()
	existing.Decisions = []domain.ApprovalDecision{{
		ID:                domain.NewApprovalDecisionID("decision-1"),
		ApprovalRequestID: existing.ID,
		RequirementID:     existing.Requirements[0].ID,
		Decision:          domain.ApprovalDecisionValueApproved,
		DecidedBy:         "alice",
	}}
	fixture.approvals.requests[existing.ID.String()] = existing

	request, err := fixture.workflow.CreateApprovalRequestForBreakingReport(context.Background(), CreateApprovalRequestForBreakingReportInput{BreakingReportID: "report-1", Actor: "admin"})
	if err != nil {
		t.Fatalf("create approval request: %v", err)
	}
	if request.ID != existing.ID {
		t.Fatalf("request id = %q, want %q", request.ID, existing.ID)
	}
	if len(request.Requirements) != 1 || len(request.Decisions) != 1 {
		t.Fatalf("existing request was not fully populated: %#v", request)
	}
	if fixture.policy.called {
		t.Fatalf("policy should not be evaluated when existing request is returned")
	}
	if len(fixture.audit.events) != 0 || len(fixture.outbox.records) != 0 {
		t.Fatalf("duplicate existing request should not write audit/outbox: audit=%#v outbox=%#v", fixture.audit.events, fixture.outbox.records)
	}
}

func TestCreateApprovalRequestDuplicateInsertReturnsExistingWithoutDuplicateAuditOrOutbox(t *testing.T) {
	fixture := newApprovalFixture(t)
	existing := fixture.existingRequest()
	fixture.approvals.requests[existing.ID.String()] = existing
	fixture.approvals.missBreakingReportLookups = 1
	fixture.approvals.createRequestErr = domain.ErrDuplicate
	fixture.policy.result = ownerApprovalPolicyResult(fixture.moduleID, fixture.moduleName, nil)

	request, err := fixture.workflow.CreateApprovalRequestForBreakingReport(context.Background(), CreateApprovalRequestForBreakingReportInput{BreakingReportID: "report-1", Actor: "admin"})
	if err != nil {
		t.Fatalf("create approval request: %v", err)
	}
	if request.ID != existing.ID {
		t.Fatalf("request id = %q, want %q", request.ID, existing.ID)
	}
	if len(fixture.approvals.requests) != 1 {
		t.Fatalf("requests = %d, want 1", len(fixture.approvals.requests))
	}
	if len(fixture.audit.events) != 0 || len(fixture.outbox.records) != 0 {
		t.Fatalf("duplicate insert should not write audit/outbox: audit=%#v outbox=%#v", fixture.audit.events, fixture.outbox.records)
	}
}

func TestGetApprovalStatusForBreakingReportReturnsRequest(t *testing.T) {
	fixture := newApprovalFixture(t)
	existing := fixture.existingRequest()
	decision := domain.ApprovalDecision{ID: domain.NewApprovalDecisionID("decision-1"), ApprovalRequestID: existing.ID, RequirementID: existing.Requirements[0].ID, Decision: domain.ApprovalDecisionValueApproved, DecidedBy: "alice"}
	existing.Decisions = []domain.ApprovalDecision{decision}
	fixture.approvals.requests[existing.ID.String()] = existing

	request, err := fixture.workflow.GetApprovalStatusForBreakingReport(context.Background(), GetApprovalStatusForBreakingReportInput{BreakingReportID: "report-1"})
	if err != nil {
		t.Fatalf("get status: %v", err)
	}
	if request.ID != existing.ID || len(request.Requirements) != 1 || len(request.Decisions) != 1 {
		t.Fatalf("request = %#v", request)
	}
}

func TestApproveRequirementRecordsDecisionAndKeepsRequestPendingWhenMoreRequirementsRemain(t *testing.T) {
	fixture := newApprovalFixture(t)
	request := fixture.existingRequest()
	second := request.Requirements[0]
	second.ID = domain.NewApprovalRequirementID("requirement-2")
	second.TargetModuleID = &fixture.moduleID
	request.Requirements = append(request.Requirements, second)
	request.RequiredApprovals = 2
	fixture.approvals.requests[request.ID.String()] = request

	updated, err := fixture.workflow.ApproveRequirement(context.Background(), ApprovalDecisionInput{RequirementID: "requirement-1", Actor: "alice", Comment: "ship it"})
	if err != nil {
		t.Fatalf("approve requirement: %v", err)
	}
	if updated.Status != domain.ApprovalRequestStatusPending || updated.ReceivedApprovals != 1 {
		t.Fatalf("updated = %#v", updated)
	}
	if len(updated.Decisions) != 1 || updated.Decisions[0].Decision != domain.ApprovalDecisionValueApproved {
		t.Fatalf("decisions = %#v", updated.Decisions)
	}
	if updated.Decisions[0].DecidedBy != "alice" {
		t.Fatalf("decided_by = %q, want alice", updated.Decisions[0].DecidedBy)
	}
}

func TestApproveAllRequirementsMarksRequestApproved(t *testing.T) {
	fixture := newApprovalFixture(t)
	request := fixture.existingRequest()
	fixture.approvals.requests[request.ID.String()] = request

	updated, err := fixture.workflow.ApproveRequirement(context.Background(), ApprovalDecisionInput{RequirementID: "requirement-1", Actor: "alice"})
	if err != nil {
		t.Fatalf("approve requirement: %v", err)
	}
	if updated.Status != domain.ApprovalRequestStatusApproved || updated.ReceivedApprovals != 1 {
		t.Fatalf("updated = %#v", updated)
	}
	if len(fixture.audit.events) != 2 {
		t.Fatalf("audit events = %#v", fixture.audit.events)
	}
	if !slices.ContainsFunc(fixture.audit.events, func(event domain.GovernanceAuditEvent) bool {
		return event.EventType == domain.GovernanceAuditEventTypeApprovalRequestStatusChanged
	}) {
		t.Fatalf("status change audit not found: %#v", fixture.audit.events)
	}
	for _, event := range fixture.audit.events {
		assertAuditPayloadSafe(t, event.PayloadJSON)
	}
	if len(fixture.outbox.records) != 1 || fixture.outbox.records[0].EventType != "protoradar.approval_decision.recorded" {
		t.Fatalf("outbox records = %#v", fixture.outbox.records)
	}
	if !slices.ContainsFunc(fixture.audit.events, func(event domain.GovernanceAuditEvent) bool {
		return event.EventType == domain.GovernanceAuditEventTypeApprovalDecisionRecorded && event.Actor == "alice"
	}) {
		t.Fatalf("decision audit actor not found: %#v", fixture.audit.events)
	}
	assertDecisionOutboxActor(t, fixture.outbox.records[0].Payload, "alice")
	assertAuditPayloadSafe(t, fixture.outbox.records[0].Payload)
}

func TestRejectRequirementRecordsDecisionAndMarksRequestRejected(t *testing.T) {
	fixture := newApprovalFixture(t)
	request := fixture.existingRequest()
	fixture.approvals.requests[request.ID.String()] = request

	updated, err := fixture.workflow.RejectRequirement(context.Background(), ApprovalDecisionInput{RequirementID: "requirement-1", Actor: "alice", Comment: "unsafe"})
	if err != nil {
		t.Fatalf("reject requirement: %v", err)
	}
	if updated.Status != domain.ApprovalRequestStatusRejected {
		t.Fatalf("status = %q", updated.Status)
	}
	if len(updated.Decisions) != 1 || updated.Decisions[0].Decision != domain.ApprovalDecisionValueRejected {
		t.Fatalf("decisions = %#v", updated.Decisions)
	}
	if updated.Decisions[0].DecidedBy != "alice" {
		t.Fatalf("decided_by = %q, want alice", updated.Decisions[0].DecidedBy)
	}
	if len(fixture.audit.events) != 2 {
		t.Fatalf("audit events = %#v", fixture.audit.events)
	}
	for _, event := range fixture.audit.events {
		if event.Actor != "alice" {
			t.Fatalf("audit actor = %q, want alice", event.Actor)
		}
	}
	assertDecisionOutboxActor(t, fixture.outbox.records[0].Payload, "alice")
}

func TestRejectRequirementRejectsEmptyActor(t *testing.T) {
	fixture := newApprovalFixture(t)
	request := fixture.existingRequest()
	fixture.approvals.requests[request.ID.String()] = request

	_, err := fixture.workflow.RejectRequirement(context.Background(), ApprovalDecisionInput{RequirementID: "requirement-1"})
	if !errors.Is(err, ErrInvalidApprovalActor) {
		t.Fatalf("error = %v, want ErrInvalidApprovalActor", err)
	}
	if len(fixture.audit.events) != 0 || len(fixture.outbox.records) != 0 {
		t.Fatalf("empty actor wrote audit/outbox: audit=%#v outbox=%#v", fixture.audit.events, fixture.outbox.records)
	}
}

func TestRepeatedDecisionReturnsConflict(t *testing.T) {
	fixture := newApprovalFixture(t)
	request := fixture.existingRequest()
	request.Requirements[0].Status = domain.ApprovalRequirementStatusApproved
	fixture.approvals.requests[request.ID.String()] = request

	_, err := fixture.workflow.ApproveRequirement(context.Background(), ApprovalDecisionInput{RequirementID: "requirement-1", Actor: "alice"})
	if !errors.Is(err, ErrApprovalRequirementConflict) {
		t.Fatalf("error = %v, want ErrApprovalRequirementConflict", err)
	}
}

func TestRepeatedApproveReturnsConflictWithoutDuplicateAuditOrOutbox(t *testing.T) {
	fixture := newApprovalFixture(t)
	request := fixture.existingRequest()
	fixture.approvals.requests[request.ID.String()] = request

	if _, err := fixture.workflow.ApproveRequirement(context.Background(), ApprovalDecisionInput{RequirementID: "requirement-1", Actor: "alice"}); err != nil {
		t.Fatalf("approve requirement: %v", err)
	}
	auditCount := len(fixture.audit.events)
	outboxCount := len(fixture.outbox.records)

	_, err := fixture.workflow.ApproveRequirement(context.Background(), ApprovalDecisionInput{RequirementID: "requirement-1", Actor: "alice"})
	if !errors.Is(err, ErrApprovalRequirementConflict) {
		t.Fatalf("error = %v, want ErrApprovalRequirementConflict", err)
	}
	if len(fixture.audit.events) != auditCount || len(fixture.outbox.records) != outboxCount {
		t.Fatalf("duplicate approve wrote audit/outbox: audit=%#v outbox=%#v", fixture.audit.events, fixture.outbox.records)
	}
}

func TestRepeatedRejectReturnsConflictWithoutDuplicateAuditOrOutbox(t *testing.T) {
	fixture := newApprovalFixture(t)
	request := fixture.existingRequest()
	fixture.approvals.requests[request.ID.String()] = request

	if _, err := fixture.workflow.RejectRequirement(context.Background(), ApprovalDecisionInput{RequirementID: "requirement-1", Actor: "alice"}); err != nil {
		t.Fatalf("reject requirement: %v", err)
	}
	auditCount := len(fixture.audit.events)
	outboxCount := len(fixture.outbox.records)

	_, err := fixture.workflow.RejectRequirement(context.Background(), ApprovalDecisionInput{RequirementID: "requirement-1", Actor: "alice"})
	if !errors.Is(err, ErrApprovalRequirementConflict) {
		t.Fatalf("error = %v, want ErrApprovalRequirementConflict", err)
	}
	if len(fixture.audit.events) != auditCount || len(fixture.outbox.records) != outboxCount {
		t.Fatalf("duplicate reject wrote audit/outbox: audit=%#v outbox=%#v", fixture.audit.events, fixture.outbox.records)
	}
}

func TestApproveThenRejectReturnsConflict(t *testing.T) {
	fixture := newApprovalFixture(t)
	request := fixture.existingRequest()
	fixture.approvals.requests[request.ID.String()] = request

	if _, err := fixture.workflow.ApproveRequirement(context.Background(), ApprovalDecisionInput{RequirementID: "requirement-1", Actor: "alice"}); err != nil {
		t.Fatalf("approve requirement: %v", err)
	}
	_, err := fixture.workflow.RejectRequirement(context.Background(), ApprovalDecisionInput{RequirementID: "requirement-1", Actor: "alice"})
	if !errors.Is(err, ErrApprovalRequirementConflict) {
		t.Fatalf("error = %v, want ErrApprovalRequirementConflict", err)
	}
}

func TestRejectThenApproveReturnsConflict(t *testing.T) {
	fixture := newApprovalFixture(t)
	request := fixture.existingRequest()
	fixture.approvals.requests[request.ID.String()] = request

	if _, err := fixture.workflow.RejectRequirement(context.Background(), ApprovalDecisionInput{RequirementID: "requirement-1", Actor: "alice"}); err != nil {
		t.Fatalf("reject requirement: %v", err)
	}
	_, err := fixture.workflow.ApproveRequirement(context.Background(), ApprovalDecisionInput{RequirementID: "requirement-1", Actor: "alice"})
	if !errors.Is(err, ErrApprovalRequirementConflict) {
		t.Fatalf("error = %v, want ErrApprovalRequirementConflict", err)
	}
}

func TestDuplicateDecisionUniqueViolationReturnsConflictWithoutAuditOrOutbox(t *testing.T) {
	fixture := newApprovalFixture(t)
	request := fixture.existingRequest()
	fixture.approvals.requests[request.ID.String()] = request
	fixture.approvals.addDecisionErr = domain.ErrDuplicate

	_, err := fixture.workflow.ApproveRequirement(context.Background(), ApprovalDecisionInput{RequirementID: "requirement-1", Actor: "alice"})
	if !errors.Is(err, ErrApprovalRequirementConflict) {
		t.Fatalf("error = %v, want ErrApprovalRequirementConflict", err)
	}
	rolledBack := fixture.approvals.requests[request.ID.String()]
	if len(rolledBack.Decisions) != 0 || rolledBack.Requirements[0].Status != domain.ApprovalRequirementStatusPending || rolledBack.Status != domain.ApprovalRequestStatusPending {
		t.Fatalf("request was not rolled back: %#v", rolledBack)
	}
	if len(fixture.audit.events) != 0 || len(fixture.outbox.records) != 0 {
		t.Fatalf("duplicate decision should not write audit/outbox: audit=%#v outbox=%#v", fixture.audit.events, fixture.outbox.records)
	}
}

func TestUnauthorizedActorReturnsForbidden(t *testing.T) {
	fixture := newApprovalFixture(t)
	request := fixture.existingRequest()
	fixture.approvals.requests[request.ID.String()] = request

	_, err := fixture.workflow.ApproveRequirement(context.Background(), ApprovalDecisionInput{RequirementID: "requirement-1", Actor: "mallory"})
	if !errors.Is(err, ErrApprovalActorForbidden) {
		t.Fatalf("error = %v, want ErrApprovalActorForbidden", err)
	}
}

func TestDecisionRollbackPreventsDecisionStatusAuditAndOutbox(t *testing.T) {
	fixture := newApprovalFixture(t)
	request := fixture.existingRequest()
	fixture.approvals.requests[request.ID.String()] = request
	fixture.audit.failAppend = errors.New("audit failed")

	_, err := fixture.workflow.ApproveRequirement(context.Background(), ApprovalDecisionInput{RequirementID: "requirement-1", Actor: "alice"})
	if !errors.Is(err, fixture.audit.failAppend) {
		t.Fatalf("error = %v, want audit failure", err)
	}
	rolledBack := fixture.approvals.requests[request.ID.String()]
	if len(rolledBack.Decisions) != 0 || rolledBack.Requirements[0].Status != domain.ApprovalRequirementStatusPending || rolledBack.Status != domain.ApprovalRequestStatusPending {
		t.Fatalf("request was not rolled back: %#v", rolledBack)
	}
	if len(fixture.audit.events) != 0 || len(fixture.outbox.records) != 0 {
		t.Fatalf("audit/outbox not rolled back: audit=%#v outbox=%#v", fixture.audit.events, fixture.outbox.records)
	}
}

func newApprovalFixture(t *testing.T) *approvalFixture {
	t.Helper()
	base := newFixture(t)
	moduleName := mustModuleName(t, "user-api")
	base.owners.byID["owner-1"] = owner("module-1", "user-api", domain.ModuleOwnerRoleOwner)
	report := breakingReport(domain.BreakingReportStatusBreaking)
	report.BaseVersionID = domain.NewModuleVersionID("version-1")
	report.ModuleID = domain.NewModuleID("module-1")
	report.ModuleName = moduleName
	approvals := &fakeApprovals{requests: map[string]domain.ApprovalRequest{}}
	policy := &fakePolicy{}
	tx := &fakeTransactions{owners: base.owners, approvals: approvals, audit: base.audit, outbox: base.outbox}
	service := NewApprovalService(
		base.modules,
		&fakeReports{reports: map[string]domain.BreakingReport{"report-1": report}},
		base.owners,
		approvals,
		audit.NewCommunityAuditSink(base.audit),
		base.audit,
		&fakeAffectedModules{},
		&fakeRuntimeImpact{},
		tx,
		base.outbox,
		policy,
		DefaultPolicyConfig(),
		fixedClock{now: time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)},
		base.ids,
	)
	workflow := NewDefaultApprovalWorkflow(service)
	return &approvalFixture{
		service:      service,
		workflow:     workflow,
		modules:      base.modules,
		owners:       base.owners,
		approvals:    approvals,
		audit:        base.audit,
		outbox:       base.outbox,
		ids:          base.ids,
		policy:       policy,
		dependencies: service.dependencies.(*fakeAffectedModules),
		runtime:      service.runtime.(*fakeRuntimeImpact),
		moduleID:     domain.NewModuleID("module-1"),
		moduleName:   moduleName,
		report:       report,
	}
}

type approvalFixture struct {
	service      *ApprovalService
	workflow     *DefaultApprovalWorkflow
	modules      *fakeModules
	owners       *fakeOwners
	approvals    *fakeApprovals
	audit        *fakeAudit
	outbox       *fakeOutbox
	ids          *fakeIDs
	policy       *fakePolicy
	dependencies *fakeAffectedModules
	runtime      *fakeRuntimeImpact
	moduleID     domain.ModuleID
	moduleName   domain.ModuleName
	report       domain.BreakingReport
}

func (fixture *approvalFixture) existingRequest() domain.ApprovalRequest {
	reportID := fixture.report.ID
	requirementID := domain.NewApprovalRequirementID("requirement-1")
	requestID := domain.NewApprovalRequestID("request-1")
	return domain.ApprovalRequest{
		ID:                requestID,
		ModuleID:          fixture.moduleID,
		ModuleName:        fixture.moduleName,
		BreakingReportID:  &reportID,
		TargetRef:         "feature/governance",
		Status:            domain.ApprovalRequestStatusPending,
		RequiredApprovals: 1,
		Requirements: []domain.ApprovalRequirement{{
			ID:                requirementID,
			ApprovalRequestID: requestID,
			RequirementType:   domain.ApprovalRequirementTypeModuleOwnerApproval,
			TargetModuleID:    &fixture.moduleID,
			TargetModuleName:  fixture.moduleName,
			RequiredRole:      domain.ModuleOwnerRoleOwner,
			Status:            domain.ApprovalRequirementStatusPending,
			Reason:            policyReasonModuleOwnerApproval,
		}},
	}
}

type fakeReports struct {
	reports map[string]domain.BreakingReport
}

func (repo *fakeReports) Create(ctx context.Context, report domain.BreakingReport, changes []domain.BreakingChange) error {
	repo.reports[report.ID.String()] = report
	return nil
}

func (repo *fakeReports) GetByID(ctx context.Context, id domain.BreakingReportID) (domain.BreakingReport, []domain.BreakingChange, error) {
	report, ok := repo.reports[id.String()]
	if !ok {
		return domain.BreakingReport{}, nil, domain.ErrNotFound
	}
	return report, nil, nil
}

func (repo *fakeReports) ListByModule(ctx context.Context, moduleID domain.ModuleID, limit int, offset int) ([]domain.BreakingReport, error) {
	return nil, nil
}

func (repo *fakeReports) CountChangesByReport(ctx context.Context, reportID domain.BreakingReportID) (int, error) {
	return 0, nil
}

func (repo *fakeReports) ListChangesByReport(ctx context.Context, reportID domain.BreakingReportID, limit int, offset int) ([]domain.BreakingChange, error) {
	return nil, nil
}

type fakeApprovals struct {
	requests                  map[string]domain.ApprovalRequest
	createRequestErr          error
	addDecisionErr            error
	missBreakingReportLookups int
}

func (repo *fakeApprovals) CreateRequest(ctx context.Context, request domain.ApprovalRequest) error {
	if repo.createRequestErr != nil {
		return repo.createRequestErr
	}
	for _, existing := range repo.requests {
		if existing.BreakingReportID != nil && request.BreakingReportID != nil && *existing.BreakingReportID == *request.BreakingReportID {
			return domain.ErrDuplicate
		}
	}
	repo.requests[request.ID.String()] = cloneApprovalRequest(request)
	return nil
}

func (repo *fakeApprovals) GetRequestByID(ctx context.Context, id domain.ApprovalRequestID) (domain.ApprovalRequest, error) {
	request, ok := repo.requests[id.String()]
	if !ok {
		return domain.ApprovalRequest{}, domain.ErrNotFound
	}
	return cloneApprovalRequest(request), nil
}

func (repo *fakeApprovals) GetRequestByBreakingReportID(ctx context.Context, reportID domain.BreakingReportID) (domain.ApprovalRequest, error) {
	if repo.missBreakingReportLookups > 0 {
		repo.missBreakingReportLookups--
		return domain.ApprovalRequest{}, domain.ErrNotFound
	}
	for _, request := range repo.requests {
		if request.BreakingReportID != nil && *request.BreakingReportID == reportID {
			return cloneApprovalRequest(request), nil
		}
	}
	return domain.ApprovalRequest{}, domain.ErrNotFound
}

func (repo *fakeApprovals) GetRequestByRequirementID(ctx context.Context, requirementID domain.ApprovalRequirementID) (domain.ApprovalRequest, error) {
	for _, request := range repo.requests {
		for _, requirement := range request.Requirements {
			if requirement.ID == requirementID {
				return cloneApprovalRequest(request), nil
			}
		}
	}
	return domain.ApprovalRequest{}, domain.ErrNotFound
}

func (repo *fakeApprovals) AddDecision(ctx context.Context, decision domain.ApprovalDecision) error {
	if repo.addDecisionErr != nil {
		return repo.addDecisionErr
	}
	request, ok := repo.requests[decision.ApprovalRequestID.String()]
	if !ok {
		return domain.ErrNotFound
	}
	for _, existing := range request.Decisions {
		if existing.RequirementID == decision.RequirementID {
			return domain.ErrDuplicate
		}
	}
	request.Decisions = append(request.Decisions, decision)
	repo.requests[request.ID.String()] = request
	return nil
}

func (repo *fakeApprovals) UpdateRequirementStatus(ctx context.Context, id domain.ApprovalRequirementID, status domain.ApprovalRequirementStatus, updatedAt time.Time) error {
	for requestID, request := range repo.requests {
		for i := range request.Requirements {
			if request.Requirements[i].ID == id {
				request.Requirements[i].Status = status
				request.Requirements[i].UpdatedAt = updatedAt
				repo.requests[requestID] = request
				return nil
			}
		}
	}
	return domain.ErrNotFound
}

func (repo *fakeApprovals) UpdateRequestStatus(ctx context.Context, id domain.ApprovalRequestID, status domain.ApprovalRequestStatus, requiredApprovals int, receivedApprovals int, updatedAt time.Time) error {
	request, ok := repo.requests[id.String()]
	if !ok {
		return domain.ErrNotFound
	}
	request.Status = status
	request.RequiredApprovals = requiredApprovals
	request.ReceivedApprovals = receivedApprovals
	request.UpdatedAt = updatedAt
	repo.requests[id.String()] = request
	return nil
}

func (repo *fakeApprovals) ListRequestsByModule(ctx context.Context, moduleID domain.ModuleID, limit int, offset int) ([]domain.ApprovalRequest, error) {
	items := make([]domain.ApprovalRequest, 0, len(repo.requests))
	for _, request := range repo.requests {
		if moduleID == "" || request.ModuleID == moduleID {
			items = append(items, cloneApprovalRequest(request))
		}
	}
	return items, nil
}

func (repo *fakeApprovals) snapshot() map[string]domain.ApprovalRequest {
	snapshot := make(map[string]domain.ApprovalRequest, len(repo.requests))
	for key, request := range repo.requests {
		snapshot[key] = cloneApprovalRequest(request)
	}
	return snapshot
}

func (repo *fakeApprovals) restore(snapshot map[string]domain.ApprovalRequest) {
	repo.requests = snapshot
}

type fakeAffectedModules struct {
	affected []domain.AffectedModule
}

func (fake *fakeAffectedModules) ListAffectedModules(ctx context.Context, providerModuleID domain.ModuleID) ([]domain.AffectedModule, error) {
	return fake.affected, nil
}

type fakeRuntimeImpact struct {
	impacts []domain.RuntimeImpact
}

func (fake *fakeRuntimeImpact) ListRuntimeImpactByModuleVersion(ctx context.Context, reportID domain.BreakingReportID, moduleVersionID domain.ModuleVersionID, limit int, offset int) ([]domain.RuntimeImpact, error) {
	return fake.impacts, nil
}

type fakePolicy struct {
	result GovernancePolicyResult
	input  EvaluatePolicyInput
	called bool
}

func (fake *fakePolicy) Evaluate(input EvaluatePolicyInput) GovernancePolicyResult {
	fake.called = true
	fake.input = input
	return fake.result
}

func ownerApprovalPolicyResult(moduleID domain.ModuleID, moduleName domain.ModuleName, warnings []GovernancePolicyWarning) GovernancePolicyResult {
	plan := ownerRequirementPlan(moduleID, moduleName, warnings)
	return GovernancePolicyResult{
		ApprovalRequired: true,
		Status:           domain.ApprovalRequestStatusPending,
		Reasons:          []GovernancePolicyReason{GovernancePolicyReasonBreakingChanges, GovernancePolicyReasonModuleOwnerRequired},
		Requirements:     []GovernanceRequirementPlan{plan},
		Warnings:         warnings,
	}
}

func ownerRequirementPlan(moduleID domain.ModuleID, moduleName domain.ModuleName, warnings []GovernancePolicyWarning) GovernanceRequirementPlan {
	return GovernanceRequirementPlan{
		RequirementType:  domain.ApprovalRequirementTypeModuleOwnerApproval,
		TargetModuleID:   &moduleID,
		TargetModuleName: moduleName,
		RequiredRole:     domain.ModuleOwnerRoleOwner,
		Status:           domain.ApprovalRequirementStatusPending,
		ReasonText:       policyReasonModuleOwnerApproval,
		Warnings:         warnings,
	}
}

func cloneApprovalRequest(request domain.ApprovalRequest) domain.ApprovalRequest {
	request.Requirements = append([]domain.ApprovalRequirement(nil), request.Requirements...)
	request.Decisions = append([]domain.ApprovalDecision(nil), request.Decisions...)
	return request
}
