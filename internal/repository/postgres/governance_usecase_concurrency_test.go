package postgres

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/alryzden/ProtoRadar/internal/audit"
	"github.com/alryzden/ProtoRadar/internal/domain"
	"github.com/alryzden/ProtoRadar/internal/usecase/governance"
)

func TestGovernanceUsecaseConcurrentCreateApprovalRequestReturnsOnePersistedRequest(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	moduleVersionID := createTestModuleVersion(t, ctx, db, "v10.2.0")
	report := createTestBreakingReport(t, ctx, db, moduleVersionID, "00000000-0000-0000-0000-000000030012")
	workflow := newPostgresApprovalWorkflow(db)

	const callers = 8
	requests, errs := runConcurrentApprovalRequests(callers, func() (domain.ApprovalRequest, error) {
		return workflow.CreateApprovalRequestForBreakingReport(ctx, governance.CreateApprovalRequestForBreakingReportInput{
			BreakingReportID: report.ID.String(),
			Actor:            "alice",
		})
	})

	for _, err := range errs {
		if err != nil {
			t.Fatalf("create approval request error = %v", err)
		}
	}
	persisted, err := NewApprovalRepository(db).GetRequestByBreakingReportID(ctx, report.ID)
	if err != nil {
		t.Fatalf("get persisted approval request: %v", err)
	}
	requestID := persisted.ID
	if requestID == "" {
		t.Fatalf("empty approval request id")
	}
	for _, request := range requests {
		if canonicalPostgresUUID(t, ctx, db, request.ID.String()) != requestID.String() {
			t.Fatalf("request id = %q, want %q", request.ID, requestID)
		}
	}
	assertApprovalRequestCountByBreakingReport(t, ctx, db, report.ID, 1)
	assertApprovalRequirementCountByRequest(t, ctx, db, requestID, 1)
	assertGovernanceAuditEventCount(t, ctx, db, domain.GovernanceAuditEventTypeApprovalRequestCreated.String(), 1)
	assertOutboxEventCount(t, ctx, db, "protoradar.approval_request.created", 1)
}

func TestGovernanceUsecaseConcurrentApproveSameRequirementRecordsOneDecision(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	request := createPostgresApprovalRequestWithOwner(t, ctx, db, "v10.2.1", "00000000-0000-0000-0000-000000030013")
	workflow := newPostgresApprovalWorkflow(db)
	requirementID := request.Requirements[0].ID

	const callers = 8
	_, errs := runConcurrentApprovalRequests(callers, func() (domain.ApprovalRequest, error) {
		return workflow.ApproveRequirement(ctx, governance.ApprovalDecisionInput{
			RequestID:     request.ID.String(),
			RequirementID: requirementID.String(),
			Actor:         "alice",
		})
	})

	assertOneSuccessRestRequirementConflicts(t, errs)
	assertDecisionCountByRequirement(t, ctx, db, requirementID, 1)
	assertApprovalStatus(t, ctx, db, request.ID, requirementID, domain.ApprovalRequestStatusApproved, 1, domain.ApprovalRequirementStatusApproved)
	assertGovernanceAuditEventCount(t, ctx, db, domain.GovernanceAuditEventTypeApprovalDecisionRecorded.String(), 1)
	assertOutboxEventCount(t, ctx, db, "protoradar.approval_decision.recorded", 1)
}

func TestGovernanceUsecaseApproveRejectRaceRecordsOneWinningDecision(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	request := createPostgresApprovalRequestWithOwner(t, ctx, db, "v10.2.2", "00000000-0000-0000-0000-000000030014")
	workflow := newPostgresApprovalWorkflow(db)
	requirementID := request.Requirements[0].ID

	_, errs := runConcurrentApprovalRequestOperations(
		func() (domain.ApprovalRequest, error) {
			return workflow.ApproveRequirement(ctx, governance.ApprovalDecisionInput{
				RequestID:     request.ID.String(),
				RequirementID: requirementID.String(),
				Actor:         "alice",
			})
		},
		func() (domain.ApprovalRequest, error) {
			return workflow.RejectRequirement(ctx, governance.ApprovalDecisionInput{
				RequestID:     request.ID.String(),
				RequirementID: requirementID.String(),
				Actor:         "alice",
			})
		},
	)

	assertOneSuccessRestRequirementConflicts(t, errs)
	assertDecisionCountByRequirement(t, ctx, db, requirementID, 1)
	decision := getApprovalDecisionByRequirement(t, ctx, db, requirementID)
	switch decision.Decision {
	case domain.ApprovalDecisionValueApproved:
		assertApprovalStatus(t, ctx, db, request.ID, requirementID, domain.ApprovalRequestStatusApproved, 1, domain.ApprovalRequirementStatusApproved)
	case domain.ApprovalDecisionValueRejected:
		assertApprovalStatus(t, ctx, db, request.ID, requirementID, domain.ApprovalRequestStatusRejected, 0, domain.ApprovalRequirementStatusRejected)
	default:
		t.Fatalf("unexpected decision = %#v", decision)
	}
	assertGovernanceAuditEventCount(t, ctx, db, domain.GovernanceAuditEventTypeApprovalDecisionRecorded.String(), 1)
	assertOutboxEventCount(t, ctx, db, "protoradar.approval_decision.recorded", 1)
}

func newPostgresApprovalWorkflow(db *DB) *governance.DefaultApprovalWorkflow {
	auditRepo := NewGovernanceAuditRepository(db)
	service := governance.NewApprovalService(
		NewModuleRepository(db),
		NewBreakingReportRepository(db),
		NewModuleOwnerRepository(db),
		NewApprovalRepository(db),
		audit.NewCommunityAuditSink(auditRepo),
		auditRepo,
		nil,
		nil,
		db,
		NewOutboxWriter(db),
		nil,
		governance.DefaultPolicyConfig(),
		governance.SystemClock{},
		governance.RandomIDGenerator{},
	)
	return governance.NewDefaultApprovalWorkflow(service)
}

func createPostgresApprovalRequestWithOwner(t *testing.T, ctx context.Context, db *DB, version string, reportID string) domain.ApprovalRequest {
	t.Helper()
	moduleVersionID := createTestModuleVersion(t, ctx, db, version)
	report := createTestBreakingReport(t, ctx, db, moduleVersionID, reportID)
	ownerRepo := NewModuleOwnerRepository(db)
	module := domain.Module{ID: report.ModuleID, Name: report.ModuleName}
	owner := testModuleOwner(t, module, "00000000-0000-0000-0000-000000020010", domain.GovernanceSubjectTypeUser, "alice", domain.ModuleOwnerRoleOwner)
	if err := ownerRepo.Add(ctx, owner); err != nil {
		t.Fatalf("add module owner: %v", err)
	}
	request, err := newPostgresApprovalWorkflow(db).CreateApprovalRequestForBreakingReport(ctx, governance.CreateApprovalRequestForBreakingReportInput{
		BreakingReportID: report.ID.String(),
		Actor:            "alice",
	})
	if err != nil {
		t.Fatalf("create approval request: %v", err)
	}
	if len(request.Requirements) != 1 {
		t.Fatalf("requirements = %d, want 1", len(request.Requirements))
	}
	persisted, err := NewApprovalRepository(db).GetRequestByBreakingReportID(ctx, report.ID)
	if err != nil {
		t.Fatalf("get persisted approval request: %v", err)
	}
	return persisted
}

func runConcurrentApprovalRequests(callers int, operation func() (domain.ApprovalRequest, error)) ([]domain.ApprovalRequest, []error) {
	operations := make([]func() (domain.ApprovalRequest, error), callers)
	for i := range operations {
		operations[i] = operation
	}
	return runConcurrentApprovalRequestOperations(operations...)
}

func runConcurrentApprovalRequestOperations(operations ...func() (domain.ApprovalRequest, error)) ([]domain.ApprovalRequest, []error) {
	requests := make([]domain.ApprovalRequest, len(operations))
	errs := make([]error, len(operations))
	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(len(operations))
	for i, operation := range operations {
		go func() {
			defer wg.Done()
			<-start
			requests[i], errs[i] = operation()
		}()
	}
	close(start)
	wg.Wait()
	return requests, errs
}

func assertOneSuccessRestRequirementConflicts(t *testing.T, errs []error) {
	t.Helper()
	successes := 0
	conflicts := 0
	for _, err := range errs {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, governance.ErrApprovalRequirementConflict):
			conflicts++
		default:
			t.Fatalf("unexpected error = %v", err)
		}
	}
	if successes != 1 || conflicts != len(errs)-1 {
		t.Fatalf("successes=%d conflicts=%d errors=%#v", successes, conflicts, errs)
	}
}

func assertApprovalRequirementCountByRequest(t *testing.T, ctx context.Context, db *DB, requestID domain.ApprovalRequestID, want int) {
	t.Helper()
	var got int
	if err := db.executor(ctx).QueryRow(ctx, `SELECT count(*) FROM approval_requirements WHERE approval_request_id = $1`, requestID.String()).Scan(&got); err != nil {
		t.Fatalf("count approval requirements by request: %v", err)
	}
	if got != want {
		t.Fatalf("approval requirement count = %d, want %d", got, want)
	}
}

func canonicalPostgresUUID(t *testing.T, ctx context.Context, db *DB, value string) string {
	t.Helper()
	var canonical string
	if err := db.executor(ctx).QueryRow(ctx, `SELECT $1::uuid::text`, value).Scan(&canonical); err != nil {
		t.Fatalf("canonicalize postgres uuid: %v", err)
	}
	return canonical
}

func assertGovernanceAuditEventCount(t *testing.T, ctx context.Context, db *DB, eventType string, want int) {
	t.Helper()
	var got int
	if err := db.executor(ctx).QueryRow(ctx, `SELECT count(*) FROM governance_audit_events WHERE event_type = $1`, eventType).Scan(&got); err != nil {
		t.Fatalf("count governance audit events: %v", err)
	}
	if got != want {
		t.Fatalf("governance audit event count = %d, want %d", got, want)
	}
}

func assertOutboxEventCount(t *testing.T, ctx context.Context, db *DB, eventType string, want int) {
	t.Helper()
	var got int
	if err := db.executor(ctx).QueryRow(ctx, `SELECT count(*) FROM outbox_records WHERE event_type = $1`, eventType).Scan(&got); err != nil {
		t.Fatalf("count outbox events: %v", err)
	}
	if got != want {
		t.Fatalf("outbox event count = %d, want %d", got, want)
	}
}

func assertApprovalStatus(t *testing.T, ctx context.Context, db *DB, requestID domain.ApprovalRequestID, requirementID domain.ApprovalRequirementID, wantRequestStatus domain.ApprovalRequestStatus, wantReceivedApprovals int, wantRequirementStatus domain.ApprovalRequirementStatus) {
	t.Helper()
	var requestStatus string
	var receivedApprovals int
	if err := db.executor(ctx).QueryRow(ctx, `SELECT status, received_approvals FROM approval_requests WHERE id = $1`, requestID.String()).Scan(&requestStatus, &receivedApprovals); err != nil {
		t.Fatalf("get approval request status: %v", err)
	}
	if requestStatus != wantRequestStatus.String() || receivedApprovals != wantReceivedApprovals {
		t.Fatalf("request status=%q received=%d, want status=%q received=%d", requestStatus, receivedApprovals, wantRequestStatus, wantReceivedApprovals)
	}
	var requirementStatus string
	if err := db.executor(ctx).QueryRow(ctx, `SELECT status FROM approval_requirements WHERE id = $1`, requirementID.String()).Scan(&requirementStatus); err != nil {
		t.Fatalf("get approval requirement status: %v", err)
	}
	if requirementStatus != wantRequirementStatus.String() {
		t.Fatalf("requirement status = %q, want %q", requirementStatus, wantRequirementStatus)
	}
}

func getApprovalDecisionByRequirement(t *testing.T, ctx context.Context, db *DB, requirementID domain.ApprovalRequirementID) domain.ApprovalDecision {
	t.Helper()
	var decisionValue string
	if err := db.executor(ctx).QueryRow(ctx, `SELECT decision FROM approval_decisions WHERE requirement_id = $1`, requirementID.String()).Scan(&decisionValue); err != nil {
		t.Fatalf("get approval decision: %v", err)
	}
	decision, err := domain.NewApprovalDecisionValue(decisionValue)
	if err != nil {
		t.Fatalf("approval decision value: %v", err)
	}
	return domain.ApprovalDecision{RequirementID: requirementID, Decision: decision}
}
