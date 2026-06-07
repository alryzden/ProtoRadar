package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/alryzden/ProtoRadar/internal/domain"
	"github.com/alryzden/ProtoRadar/internal/outbox"
)

func TestModuleOwnerRepositoryAddListRemove(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	module := createTestModule(t, ctx, db, "00000000-0000-0000-0000-000000010001", "owners-api")
	repo := NewModuleOwnerRepository(db)
	owner := testModuleOwner(t, module, "00000000-0000-0000-0000-000000020001", domain.GovernanceSubjectTypeUser, "alice", domain.ModuleOwnerRoleOwner)

	if err := repo.Add(ctx, owner); err != nil {
		t.Fatalf("add owner: %v", err)
	}

	got, err := repo.GetByID(ctx, owner.ID)
	if err != nil {
		t.Fatalf("get owner: %v", err)
	}
	assertModuleOwner(t, got, owner)

	owners, err := repo.ListByModule(ctx, module.ID)
	if err != nil {
		t.Fatalf("list owners: %v", err)
	}
	if len(owners) != 1 {
		t.Fatalf("owners = %d, want 1", len(owners))
	}
	assertModuleOwner(t, owners[0], owner)

	hasOwner, err := repo.HasRole(ctx, module.ID, domain.GovernanceSubjectTypeUser, "alice", []domain.ModuleOwnerRole{domain.ModuleOwnerRoleOwner})
	if err != nil {
		t.Fatalf("has owner role: %v", err)
	}
	if !hasOwner {
		t.Fatalf("expected alice to have owner role")
	}
	hasMaintainer, err := repo.HasRole(ctx, module.ID, domain.GovernanceSubjectTypeUser, "alice", []domain.ModuleOwnerRole{domain.ModuleOwnerRoleMaintainer})
	if err != nil {
		t.Fatalf("has maintainer role: %v", err)
	}
	if hasMaintainer {
		t.Fatalf("did not expect alice to have maintainer role")
	}

	if err := repo.Remove(ctx, owner.ID, time.Now().UTC()); err != nil {
		t.Fatalf("remove owner: %v", err)
	}
	if _, err := repo.GetByID(ctx, owner.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("removed owner error = %v, want ErrNotFound", err)
	}
}

func TestModuleOwnerRepositoryDuplicateOwnerConstraint(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	module := createTestModule(t, ctx, db, "00000000-0000-0000-0000-000000010002", "duplicate-owner-api")
	repo := NewModuleOwnerRepository(db)
	first := testModuleOwner(t, module, "00000000-0000-0000-0000-000000020002", domain.GovernanceSubjectTypeTeam, "platform", domain.ModuleOwnerRoleOwner)
	second := testModuleOwner(t, module, "00000000-0000-0000-0000-000000020003", domain.GovernanceSubjectTypeTeam, "platform", domain.ModuleOwnerRoleOwner)

	if err := repo.Add(ctx, first); err != nil {
		t.Fatalf("add first owner: %v", err)
	}
	if err := repo.Add(ctx, second); !errors.Is(err, domain.ErrDuplicate) {
		t.Fatalf("duplicate owner error = %v, want ErrDuplicate", err)
	}
}

func TestGovernanceEnumCheckConstraintsAllowValidValues(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	module := createTestModule(t, ctx, db, "00000000-0000-0000-0000-000000010012", "valid-governance-enums-api")
	now := time.Now().UTC()

	for i, subjectType := range []domain.GovernanceSubjectType{domain.GovernanceSubjectTypeUser, domain.GovernanceSubjectTypeTeam} {
		if _, err := db.executor(ctx).Exec(ctx, `
			INSERT INTO module_owners (id, module_id, subject_type, subject, role, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
		`,
			uuidForTest(20020+i),
			module.ID.String(),
			subjectType.String(),
			"subject-"+subjectType.String(),
			domain.ModuleOwnerRoleOwner.String(),
			now,
			now,
		); err != nil {
			t.Fatalf("insert valid owner subject type %q: %v", subjectType, err)
		}
	}
	for i, role := range []domain.ModuleOwnerRole{domain.ModuleOwnerRoleOwner, domain.ModuleOwnerRoleMaintainer} {
		if _, err := db.executor(ctx).Exec(ctx, `
			INSERT INTO module_owners (id, module_id, subject_type, subject, role, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
		`,
			uuidForTest(20030+i),
			module.ID.String(),
			domain.GovernanceSubjectTypeUser.String(),
			"role-subject-"+role.String(),
			role.String(),
			now,
			now,
		); err != nil {
			t.Fatalf("insert valid owner role %q: %v", role, err)
		}
	}

	requestStatuses := []domain.ApprovalRequestStatus{
		domain.ApprovalRequestStatusPending,
		domain.ApprovalRequestStatusApproved,
		domain.ApprovalRequestStatusRejected,
		domain.ApprovalRequestStatusCancelled,
		domain.ApprovalRequestStatusNotRequired,
	}
	for i, status := range requestStatuses {
		if _, err := db.executor(ctx).Exec(ctx, `
			INSERT INTO approval_requests (id, module_id, target_ref, status, required_approvals, received_approvals, created_at, updated_at)
			VALUES ($1, $2, $3, $4, 0, 0, $5, $6)
		`, uuidForTest(40020+i), module.ID.String(), "target", status.String(), now, now); err != nil {
			t.Fatalf("insert valid request status %q: %v", status, err)
		}
	}

	requirementTypes := []domain.ApprovalRequirementType{
		domain.ApprovalRequirementTypeModuleOwnerApproval,
		domain.ApprovalRequirementTypeAffectedConsumerApproval,
	}
	requirementStatuses := []domain.ApprovalRequirementStatus{
		domain.ApprovalRequirementStatusPending,
		domain.ApprovalRequirementStatusApproved,
		domain.ApprovalRequirementStatusRejected,
		domain.ApprovalRequirementStatusNotRequired,
	}
	for i, status := range requirementStatuses {
		requirementType := requirementTypes[i%len(requirementTypes)]
		if _, err := db.executor(ctx).Exec(ctx, `
			INSERT INTO approval_requirements (
				id, approval_request_id, requirement_type, target_module_id, target_module_name,
				required_role, status, reason, created_at, updated_at
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		`,
			uuidForTest(50020+i),
			uuidForTest(40020),
			requirementType.String(),
			module.ID.String(),
			module.Name.String(),
			domain.ModuleOwnerRoleOwner.String(),
			status.String(),
			"valid governance enum",
			now,
			now,
		); err != nil {
			t.Fatalf("insert valid requirement type=%q status=%q: %v", requirementType, status, err)
		}
	}

	for i, decision := range []domain.ApprovalDecisionValue{domain.ApprovalDecisionValueApproved, domain.ApprovalDecisionValueRejected} {
		if _, err := db.executor(ctx).Exec(ctx, `
			INSERT INTO approval_decisions (id, approval_request_id, requirement_id, decision, decided_by, comment, created_at)
			VALUES ($1, $2, $3, $4, $5, '', $6)
		`,
			uuidForTest(60020+i),
			uuidForTest(40020),
			uuidForTest(50020+i),
			decision.String(),
			"alice",
			now,
		); err != nil {
			t.Fatalf("insert valid decision %q: %v", decision, err)
		}
	}

	eventTypes := []domain.GovernanceAuditEventType{
		domain.GovernanceAuditEventTypeModuleOwnerAdded,
		domain.GovernanceAuditEventTypeModuleOwnerRemoved,
		domain.GovernanceAuditEventTypeApprovalRequestCreated,
		domain.GovernanceAuditEventTypeApprovalDecisionRecorded,
		domain.GovernanceAuditEventTypeApprovalRequestStatusChanged,
		domain.GovernanceAuditEventTypePolicyEvaluated,
	}
	for i, eventType := range eventTypes {
		if _, err := db.executor(ctx).Exec(ctx, `
			INSERT INTO governance_audit_events (id, event_type, actor, payload_json, created_at)
			VALUES ($1, $2, 'alice', '{}'::jsonb, $3)
		`, uuidForTest(70020+i), eventType.String(), now); err != nil {
			t.Fatalf("insert valid audit event type %q: %v", eventType, err)
		}
	}
}

func TestGovernanceEnumCheckConstraintsRejectInvalidValues(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	module := createTestModule(t, ctx, db, "00000000-0000-0000-0000-000000010013", "invalid-governance-enums-api")
	now := time.Now().UTC()

	requestID := uuidForTest(40080)
	if _, err := db.executor(ctx).Exec(ctx, `
		INSERT INTO approval_requests (id, module_id, target_ref, status, required_approvals, received_approvals, created_at, updated_at)
		VALUES ($1, $2, 'target', 'pending', 1, 0, $3, $4)
	`, requestID, module.ID.String(), now, now); err != nil {
		t.Fatalf("insert valid request fixture: %v", err)
	}
	requirementID := uuidForTest(50080)
	if _, err := db.executor(ctx).Exec(ctx, `
		INSERT INTO approval_requirements (
			id, approval_request_id, requirement_type, target_module_id, target_module_name,
			required_role, status, reason, created_at, updated_at
		)
		VALUES ($1, $2, 'module_owner_approval', $3, $4, 'owner', 'pending', 'fixture', $5, $6)
	`, requirementID, requestID, module.ID.String(), module.Name.String(), now, now); err != nil {
		t.Fatalf("insert valid requirement fixture: %v", err)
	}

	_, err := db.executor(ctx).Exec(ctx, `
		INSERT INTO module_owners (id, module_id, subject_type, subject, role, created_at, updated_at)
		VALUES ($1, $2, 'service', 'alice', 'owner', $3, $4)
	`, uuidForTest(20080), module.ID.String(), now, now)
	assertCheckViolation(t, err, "module_owners_subject_type_check")

	_, err = db.executor(ctx).Exec(ctx, `
		INSERT INTO module_owners (id, module_id, subject_type, subject, role, created_at, updated_at)
		VALUES ($1, $2, 'user', 'alice', 'admin', $3, $4)
	`, uuidForTest(20081), module.ID.String(), now, now)
	assertCheckViolation(t, err, "module_owners_role_check")

	_, err = db.executor(ctx).Exec(ctx, `
		INSERT INTO approval_requests (id, module_id, target_ref, status, required_approvals, received_approvals, created_at, updated_at)
		VALUES ($1, $2, 'target', 'done', 0, 0, $3, $4)
	`, uuidForTest(40081), module.ID.String(), now, now)
	assertCheckViolation(t, err, "approval_requests_status_check")

	_, err = db.executor(ctx).Exec(ctx, `
		INSERT INTO approval_requirements (
			id, approval_request_id, requirement_type, target_module_id, target_module_name,
			required_role, status, reason, created_at, updated_at
		)
		VALUES ($1, $2, 'manual_approval', $3, $4, 'owner', 'pending', 'invalid type', $5, $6)
	`, uuidForTest(50081), requestID, module.ID.String(), module.Name.String(), now, now)
	assertCheckViolation(t, err, "approval_requirements_type_check")

	_, err = db.executor(ctx).Exec(ctx, `
		INSERT INTO approval_requirements (
			id, approval_request_id, requirement_type, target_module_id, target_module_name,
			required_role, status, reason, created_at, updated_at
		)
		VALUES ($1, $2, 'module_owner_approval', $3, $4, 'owner', 'cancelled', 'invalid status', $5, $6)
	`, uuidForTest(50082), requestID, module.ID.String(), module.Name.String(), now, now)
	assertCheckViolation(t, err, "approval_requirements_status_check")

	_, err = db.executor(ctx).Exec(ctx, `
		INSERT INTO approval_decisions (id, approval_request_id, requirement_id, decision, decided_by, comment, created_at)
		VALUES ($1, $2, $3, 'pending', 'alice', '', $4)
	`, uuidForTest(60080), requestID, requirementID, now)
	assertCheckViolation(t, err, "approval_decisions_decision_check")

	_, err = db.executor(ctx).Exec(ctx, `
		INSERT INTO governance_audit_events (id, event_type, actor, payload_json, created_at)
		VALUES ($1, 'manual_override', 'alice', '{}'::jsonb, $2)
	`, uuidForTest(70080), now)
	assertCheckViolation(t, err, "governance_audit_events_type_check")
}

func TestGovernanceRepositoriesMapCheckViolationsToDomainErrors(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	moduleVersionID := createTestModuleVersion(t, ctx, db, "v10.3.0")
	report := createTestBreakingReport(t, ctx, db, moduleVersionID, "00000000-0000-0000-0000-000000030020")
	module := domain.Module{ID: report.ModuleID, Name: report.ModuleName}
	now := time.Now().UTC()

	ownerRepo := NewModuleOwnerRepository(db)
	invalidOwner := testModuleOwner(t, module, "00000000-0000-0000-0000-000000020020", domain.GovernanceSubjectTypeUser, "alice", domain.ModuleOwnerRoleOwner)
	invalidOwner.SubjectType = "service"
	if err := ownerRepo.Add(ctx, invalidOwner); !errors.Is(err, domain.ErrInvalidGovernanceSubjectType) {
		t.Fatalf("owner subject type error = %v, want ErrInvalidGovernanceSubjectType", err)
	}
	invalidOwner.SubjectType = domain.GovernanceSubjectTypeUser
	invalidOwner.Role = "admin"
	if err := ownerRepo.Add(ctx, invalidOwner); !errors.Is(err, domain.ErrInvalidModuleOwnerRole) {
		t.Fatalf("owner role error = %v, want ErrInvalidModuleOwnerRole", err)
	}

	approvalRepo := NewApprovalRepository(db)
	request := testApprovalRequest(t, report, "00000000-0000-0000-0000-000000040020")
	invalidRequest := request
	invalidRequest.Status = "done"
	if err := approvalRepo.CreateRequest(ctx, invalidRequest); !errors.Is(err, domain.ErrInvalidApprovalRequestStatus) {
		t.Fatalf("request status error = %v, want ErrInvalidApprovalRequestStatus", err)
	}

	requirement := testApprovalRequirement(t, request.ID, "00000000-0000-0000-0000-000000050020", domain.ApprovalRequirementTypeModuleOwnerApproval, request.ModuleID, request.ModuleName)
	request.Requirements = []domain.ApprovalRequirement{requirement}
	if err := approvalRepo.CreateRequest(ctx, request); err != nil {
		t.Fatalf("create valid request: %v", err)
	}

	invalidTypeRequest := testApprovalRequest(t, report, "00000000-0000-0000-0000-000000040021")
	invalidTypeRequest.BreakingReportID = nil
	invalidTypeRequirement := testApprovalRequirement(t, invalidTypeRequest.ID, "00000000-0000-0000-0000-000000050021", domain.ApprovalRequirementTypeModuleOwnerApproval, invalidTypeRequest.ModuleID, invalidTypeRequest.ModuleName)
	invalidTypeRequirement.RequirementType = "manual_approval"
	invalidTypeRequest.Requirements = []domain.ApprovalRequirement{invalidTypeRequirement}
	if err := approvalRepo.CreateRequest(ctx, invalidTypeRequest); !errors.Is(err, domain.ErrInvalidApprovalRequirementType) {
		t.Fatalf("requirement type error = %v, want ErrInvalidApprovalRequirementType", err)
	}

	invalidStatusRequest := testApprovalRequest(t, report, "00000000-0000-0000-0000-000000040022")
	invalidStatusRequest.BreakingReportID = nil
	invalidStatusRequirement := testApprovalRequirement(t, invalidStatusRequest.ID, "00000000-0000-0000-0000-000000050022", domain.ApprovalRequirementTypeModuleOwnerApproval, invalidStatusRequest.ModuleID, invalidStatusRequest.ModuleName)
	invalidStatusRequirement.Status = "cancelled"
	invalidStatusRequest.Requirements = []domain.ApprovalRequirement{invalidStatusRequirement}
	if err := approvalRepo.CreateRequest(ctx, invalidStatusRequest); !errors.Is(err, domain.ErrInvalidApprovalRequirementStatus) {
		t.Fatalf("requirement status error = %v, want ErrInvalidApprovalRequirementStatus", err)
	}

	invalidDecision := domain.ApprovalDecision{
		ID:                domain.NewApprovalDecisionID("00000000-0000-0000-0000-000000060020"),
		ApprovalRequestID: request.ID,
		RequirementID:     requirement.ID,
		Decision:          "pending",
		DecidedBy:         "alice",
		CreatedAt:         now,
	}
	if err := approvalRepo.AddDecision(ctx, invalidDecision); !errors.Is(err, domain.ErrInvalidApprovalDecision) {
		t.Fatalf("decision error = %v, want ErrInvalidApprovalDecision", err)
	}

	auditRepo := NewGovernanceAuditRepository(db)
	if err := auditRepo.Append(ctx, domain.GovernanceAuditEvent{
		ID:          domain.NewGovernanceAuditEventID("00000000-0000-0000-0000-000000070020"),
		EventType:   "manual_override",
		Actor:       "alice",
		PayloadJSON: json.RawMessage(`{}`),
		CreatedAt:   now,
	}); !errors.Is(err, domain.ErrInvalidGovernanceAuditEventType) {
		t.Fatalf("audit event type error = %v, want ErrInvalidGovernanceAuditEventType", err)
	}
}

func TestApprovalRepositoryCreateGetAndListRequestWithRequirements(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	moduleVersionID := createTestModuleVersion(t, ctx, db, "v10.0.0")
	report := createTestBreakingReport(t, ctx, db, moduleVersionID, "00000000-0000-0000-0000-000000030001")
	repo := NewApprovalRepository(db)
	request := testApprovalRequest(t, report, "00000000-0000-0000-0000-000000040001")
	request.Requirements = []domain.ApprovalRequirement{
		testApprovalRequirement(t, request.ID, "00000000-0000-0000-0000-000000050001", domain.ApprovalRequirementTypeModuleOwnerApproval, request.ModuleID, request.ModuleName),
	}

	if err := repo.CreateRequest(ctx, request); err != nil {
		t.Fatalf("create approval request: %v", err)
	}

	got, err := repo.GetRequestByID(ctx, request.ID)
	if err != nil {
		t.Fatalf("get request: %v", err)
	}
	assertApprovalRequest(t, got, request)
	if len(got.Requirements) != 1 {
		t.Fatalf("requirements = %d, want 1", len(got.Requirements))
	}
	if got.Requirements[0].RequirementType != domain.ApprovalRequirementTypeModuleOwnerApproval || got.Requirements[0].TargetModuleName != request.ModuleName {
		t.Fatalf("requirement = %#v", got.Requirements[0])
	}

	byReport, err := repo.GetRequestByBreakingReportID(ctx, report.ID)
	if err != nil {
		t.Fatalf("get by report: %v", err)
	}
	if byReport.ID != request.ID {
		t.Fatalf("by report id = %q, want %q", byReport.ID, request.ID)
	}

	listed, err := repo.ListRequestsByModule(ctx, request.ModuleID, 10, 0)
	if err != nil {
		t.Fatalf("list by module: %v", err)
	}
	if len(listed) != 1 || listed[0].ID != request.ID {
		t.Fatalf("listed requests = %#v", listed)
	}
}

func TestApprovalRepositoryDuplicateBreakingReportRequestConstraint(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	moduleVersionID := createTestModuleVersion(t, ctx, db, "v10.0.5")
	report := createTestBreakingReport(t, ctx, db, moduleVersionID, "00000000-0000-0000-0000-000000030006")
	repo := NewApprovalRepository(db)
	first := testApprovalRequest(t, report, "00000000-0000-0000-0000-000000040006")
	second := testApprovalRequest(t, report, "00000000-0000-0000-0000-000000040007")

	if err := repo.CreateRequest(ctx, first); err != nil {
		t.Fatalf("create first request: %v", err)
	}
	if err := repo.CreateRequest(ctx, second); !errors.Is(err, domain.ErrDuplicate) {
		t.Fatalf("duplicate request error = %v, want ErrDuplicate", err)
	}
}

func TestApprovalRepositoryConcurrentCreateRequestPersistsOneRequest(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	moduleVersionID := createTestModuleVersion(t, ctx, db, "v10.0.9")
	report := createTestBreakingReport(t, ctx, db, moduleVersionID, "00000000-0000-0000-0000-000000030010")
	repo := NewApprovalRepository(db)
	first := testApprovalRequest(t, report, "00000000-0000-0000-0000-000000040013")
	first.Requirements = []domain.ApprovalRequirement{
		testApprovalRequirement(t, first.ID, "00000000-0000-0000-0000-000000050007", domain.ApprovalRequirementTypeModuleOwnerApproval, first.ModuleID, first.ModuleName),
	}
	second := testApprovalRequest(t, report, "00000000-0000-0000-0000-000000040014")
	second.Requirements = []domain.ApprovalRequirement{
		testApprovalRequirement(t, second.ID, "00000000-0000-0000-0000-000000050008", domain.ApprovalRequirementTypeModuleOwnerApproval, second.ModuleID, second.ModuleName),
	}

	errs := runConcurrently(
		func() error { return repo.CreateRequest(ctx, first) },
		func() error { return repo.CreateRequest(ctx, second) },
	)
	assertOneSuccessOneDuplicate(t, errs)
	assertApprovalRequestCountByBreakingReport(t, ctx, db, report.ID, 1)
	assertTableCount(t, ctx, db, "approval_requirements", 1)
}

func TestApprovalRepositoryAllowsMultipleRequestsWithoutBreakingReport(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	moduleVersionID := createTestModuleVersion(t, ctx, db, "v10.0.6")
	report := createTestBreakingReport(t, ctx, db, moduleVersionID, "00000000-0000-0000-0000-000000030007")
	repo := NewApprovalRepository(db)
	first := testApprovalRequest(t, report, "00000000-0000-0000-0000-000000040008")
	second := testApprovalRequest(t, report, "00000000-0000-0000-0000-000000040009")
	first.BreakingReportID = nil
	second.BreakingReportID = nil

	if err := repo.CreateRequest(ctx, first); err != nil {
		t.Fatalf("create first request without breaking report: %v", err)
	}
	if err := repo.CreateRequest(ctx, second); err != nil {
		t.Fatalf("create second request without breaking report: %v", err)
	}
	assertTableCount(t, ctx, db, "approval_requests", 2)
}

func TestApprovalRepositoryDecisionAndStatusUpdates(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	moduleVersionID := createTestModuleVersion(t, ctx, db, "v10.0.1")
	report := createTestBreakingReport(t, ctx, db, moduleVersionID, "00000000-0000-0000-0000-000000030002")
	repo := NewApprovalRepository(db)
	request := testApprovalRequest(t, report, "00000000-0000-0000-0000-000000040002")
	requirement := testApprovalRequirement(t, request.ID, "00000000-0000-0000-0000-000000050002", domain.ApprovalRequirementTypeModuleOwnerApproval, request.ModuleID, request.ModuleName)
	request.Requirements = []domain.ApprovalRequirement{requirement}
	if err := repo.CreateRequest(ctx, request); err != nil {
		t.Fatalf("create request: %v", err)
	}

	decision := domain.ApprovalDecision{
		ID:                domain.NewApprovalDecisionID("00000000-0000-0000-0000-000000060001"),
		ApprovalRequestID: request.ID,
		RequirementID:     requirement.ID,
		Decision:          domain.ApprovalDecisionValueApproved,
		DecidedBy:         "alice",
		Comment:           "approved for rollout",
		CreatedAt:         time.Now().UTC(),
	}
	if err := repo.AddDecision(ctx, decision); err != nil {
		t.Fatalf("add decision: %v", err)
	}
	updatedAt := time.Now().UTC().Add(time.Second)
	if err := repo.UpdateRequirementStatus(ctx, requirement.ID, domain.ApprovalRequirementStatusApproved, updatedAt); err != nil {
		t.Fatalf("update requirement status: %v", err)
	}
	if err := repo.UpdateRequestStatus(ctx, request.ID, domain.ApprovalRequestStatusApproved, 1, 1, updatedAt); err != nil {
		t.Fatalf("update request status: %v", err)
	}

	got, err := repo.GetRequestByID(ctx, request.ID)
	if err != nil {
		t.Fatalf("get request: %v", err)
	}
	if got.Status != domain.ApprovalRequestStatusApproved || got.RequiredApprovals != 1 || got.ReceivedApprovals != 1 {
		t.Fatalf("request status/counts = %#v", got)
	}
	if len(got.Requirements) != 1 || got.Requirements[0].Status != domain.ApprovalRequirementStatusApproved {
		t.Fatalf("requirements = %#v", got.Requirements)
	}
	if len(got.Decisions) != 1 || got.Decisions[0].Decision != domain.ApprovalDecisionValueApproved || got.Decisions[0].Comment != "approved for rollout" {
		t.Fatalf("decisions = %#v", got.Decisions)
	}
}

func TestApprovalRepositoryDuplicateRequirementDecisionConstraint(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	moduleVersionID := createTestModuleVersion(t, ctx, db, "v10.0.7")
	report := createTestBreakingReport(t, ctx, db, moduleVersionID, "00000000-0000-0000-0000-000000030008")
	repo := NewApprovalRepository(db)
	request := testApprovalRequest(t, report, "00000000-0000-0000-0000-000000040010")
	requirement := testApprovalRequirement(t, request.ID, "00000000-0000-0000-0000-000000050006", domain.ApprovalRequirementTypeModuleOwnerApproval, request.ModuleID, request.ModuleName)
	request.Requirements = []domain.ApprovalRequirement{requirement}
	if err := repo.CreateRequest(ctx, request); err != nil {
		t.Fatalf("create request: %v", err)
	}

	first := domain.ApprovalDecision{
		ID:                domain.NewApprovalDecisionID("00000000-0000-0000-0000-000000060003"),
		ApprovalRequestID: request.ID,
		RequirementID:     requirement.ID,
		Decision:          domain.ApprovalDecisionValueApproved,
		DecidedBy:         "alice",
		CreatedAt:         time.Now().UTC(),
	}
	second := first
	second.ID = domain.NewApprovalDecisionID("00000000-0000-0000-0000-000000060004")
	second.Decision = domain.ApprovalDecisionValueRejected
	second.DecidedBy = "bob"

	if err := repo.AddDecision(ctx, first); err != nil {
		t.Fatalf("add first decision: %v", err)
	}
	if err := repo.AddDecision(ctx, second); !errors.Is(err, domain.ErrDuplicate) {
		t.Fatalf("duplicate decision error = %v, want ErrDuplicate", err)
	}
}

func TestApprovalRepositoryConcurrentAddDecisionPersistsOneDecision(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	moduleVersionID := createTestModuleVersion(t, ctx, db, "v10.1.0")
	report := createTestBreakingReport(t, ctx, db, moduleVersionID, "00000000-0000-0000-0000-000000030011")
	repo := NewApprovalRepository(db)
	request := testApprovalRequest(t, report, "00000000-0000-0000-0000-000000040015")
	requirement := testApprovalRequirement(t, request.ID, "00000000-0000-0000-0000-000000050009", domain.ApprovalRequirementTypeModuleOwnerApproval, request.ModuleID, request.ModuleName)
	request.Requirements = []domain.ApprovalRequirement{requirement}
	if err := repo.CreateRequest(ctx, request); err != nil {
		t.Fatalf("create request: %v", err)
	}
	first := domain.ApprovalDecision{
		ID:                domain.NewApprovalDecisionID("00000000-0000-0000-0000-000000060005"),
		ApprovalRequestID: request.ID,
		RequirementID:     requirement.ID,
		Decision:          domain.ApprovalDecisionValueApproved,
		DecidedBy:         "alice",
		CreatedAt:         time.Now().UTC(),
	}
	second := first
	second.ID = domain.NewApprovalDecisionID("00000000-0000-0000-0000-000000060006")
	second.Decision = domain.ApprovalDecisionValueRejected
	second.DecidedBy = "bob"

	errs := runConcurrently(
		func() error { return repo.AddDecision(ctx, first) },
		func() error { return repo.AddDecision(ctx, second) },
	)
	assertOneSuccessOneDuplicate(t, errs)
	assertDecisionCountByRequirement(t, ctx, db, requirement.ID, 1)
}

func TestGovernanceAuditRepositoryAppendAndList(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	moduleVersionID := createTestModuleVersion(t, ctx, db, "v10.0.2")
	report := createTestBreakingReport(t, ctx, db, moduleVersionID, "00000000-0000-0000-0000-000000030003")
	approvalRepo := NewApprovalRepository(db)
	auditRepo := NewGovernanceAuditRepository(db)
	request := testApprovalRequest(t, report, "00000000-0000-0000-0000-000000040003")
	if err := approvalRepo.CreateRequest(ctx, request); err != nil {
		t.Fatalf("create request: %v", err)
	}
	payload, err := json.Marshal(map[string]string{"status": "pending"})
	if err != nil {
		t.Fatalf("payload json: %v", err)
	}
	event := domain.GovernanceAuditEvent{
		ID:                domain.NewGovernanceAuditEventID("00000000-0000-0000-0000-000000070001"),
		EventType:         domain.GovernanceAuditEventTypeApprovalRequestCreated,
		Actor:             "alice",
		ModuleID:          &request.ModuleID,
		ModuleName:        request.ModuleName,
		ApprovalRequestID: &request.ID,
		BreakingReportID:  request.BreakingReportID,
		PayloadJSON:       payload,
		CreatedAt:         time.Now().UTC(),
	}
	if err := auditRepo.Append(ctx, event); err != nil {
		t.Fatalf("append audit event: %v", err)
	}

	byRequest, err := auditRepo.ListByApprovalRequest(ctx, request.ID, 10, 0)
	if err != nil {
		t.Fatalf("list by request: %v", err)
	}
	if len(byRequest) != 1 || byRequest[0].ID != event.ID || byRequest[0].Actor != "alice" {
		t.Fatalf("audit by request = %#v", byRequest)
	}
	if string(byRequest[0].PayloadJSON) != `{"status": "pending"}` && string(byRequest[0].PayloadJSON) != `{"status":"pending"}` {
		t.Fatalf("payload = %s", string(byRequest[0].PayloadJSON))
	}

	byModule, err := auditRepo.ListByModule(ctx, request.ModuleID, 10, 0)
	if err != nil {
		t.Fatalf("list by module: %v", err)
	}
	if len(byModule) != 1 || byModule[0].ApprovalRequestID == nil || *byModule[0].ApprovalRequestID != request.ID {
		t.Fatalf("audit by module = %#v", byModule)
	}
}

func TestGovernanceTransactionRollbackPreventsChangesAndAudit(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	module := createTestModule(t, ctx, db, "00000000-0000-0000-0000-000000010003", "rollback-governance-api")
	ownerRepo := NewModuleOwnerRepository(db)
	auditRepo := NewGovernanceAuditRepository(db)
	owner := testModuleOwner(t, module, "00000000-0000-0000-0000-000000020004", domain.GovernanceSubjectTypeUser, "alice", domain.ModuleOwnerRoleOwner)
	rollbackErr := errors.New("force rollback")

	err := db.WithinTransaction(ctx, func(txCtx context.Context) error {
		if err := ownerRepo.Add(txCtx, owner); err != nil {
			return err
		}
		if err := auditRepo.Append(txCtx, domain.GovernanceAuditEvent{
			ID:         domain.NewGovernanceAuditEventID("00000000-0000-0000-0000-000000070002"),
			EventType:  domain.GovernanceAuditEventTypeModuleOwnerAdded,
			Actor:      "alice",
			ModuleID:   &module.ID,
			ModuleName: module.Name,
			CreatedAt:  time.Now().UTC(),
		}); err != nil {
			return err
		}
		return rollbackErr
	})
	if !errors.Is(err, rollbackErr) {
		t.Fatalf("transaction error = %v", err)
	}
	assertTableCount(t, ctx, db, "module_owners", 0)
	assertTableCount(t, ctx, db, "governance_audit_events", 0)
}

func TestGovernanceTransactionRollbackOnApprovalRequestUniqueViolation(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	moduleVersionID := createTestModuleVersion(t, ctx, db, "v10.0.8")
	report := createTestBreakingReport(t, ctx, db, moduleVersionID, "00000000-0000-0000-0000-000000030009")
	approvalRepo := NewApprovalRepository(db)
	auditRepo := NewGovernanceAuditRepository(db)
	first := testApprovalRequest(t, report, "00000000-0000-0000-0000-000000040011")
	duplicate := testApprovalRequest(t, report, "00000000-0000-0000-0000-000000040012")
	if err := approvalRepo.CreateRequest(ctx, first); err != nil {
		t.Fatalf("create first request: %v", err)
	}

	err := db.WithinTransaction(ctx, func(txCtx context.Context) error {
		if err := auditRepo.Append(txCtx, domain.GovernanceAuditEvent{
			ID:                domain.NewGovernanceAuditEventID("00000000-0000-0000-0000-000000070004"),
			EventType:         domain.GovernanceAuditEventTypeApprovalRequestCreated,
			Actor:             "alice",
			ModuleID:          &duplicate.ModuleID,
			ModuleName:        duplicate.ModuleName,
			ApprovalRequestID: &first.ID,
			BreakingReportID:  duplicate.BreakingReportID,
			CreatedAt:         duplicate.CreatedAt,
		}); err != nil {
			return err
		}
		return approvalRepo.CreateRequest(txCtx, duplicate)
	})
	if !errors.Is(err, domain.ErrDuplicate) {
		t.Fatalf("transaction error = %v, want ErrDuplicate", err)
	}
	assertTableCount(t, ctx, db, "approval_requests", 1)
	assertTableCount(t, ctx, db, "governance_audit_events", 0)
}

func TestGovernanceTransactionCanShareOutbox(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	moduleVersionID := createTestModuleVersion(t, ctx, db, "v10.0.3")
	report := createTestBreakingReport(t, ctx, db, moduleVersionID, "00000000-0000-0000-0000-000000030004")
	approvalRepo := NewApprovalRepository(db)
	auditRepo := NewGovernanceAuditRepository(db)
	outboxWriter := NewOutboxWriter(db)
	request := testApprovalRequest(t, report, "00000000-0000-0000-0000-000000040004")
	payload, err := json.Marshal(map[string]string{"approval_request_id": request.ID.String()})
	if err != nil {
		t.Fatalf("payload json: %v", err)
	}

	err = db.WithinTransaction(ctx, func(txCtx context.Context) error {
		if err := approvalRepo.CreateRequest(txCtx, request); err != nil {
			return err
		}
		if err := auditRepo.Append(txCtx, domain.GovernanceAuditEvent{
			ID:                domain.NewGovernanceAuditEventID("00000000-0000-0000-0000-000000070003"),
			EventType:         domain.GovernanceAuditEventTypeApprovalRequestCreated,
			Actor:             "alice",
			ModuleID:          &request.ModuleID,
			ModuleName:        request.ModuleName,
			ApprovalRequestID: &request.ID,
			BreakingReportID:  request.BreakingReportID,
			CreatedAt:         request.CreatedAt,
		}); err != nil {
			return err
		}
		return outboxWriter.Create(txCtx, outbox.Record{
			EventType:     "protoradar.approval_request.created",
			AggregateType: "approval_request",
			AggregateID:   request.ID.String(),
			DedupKey:      "approval-request:" + request.ID.String() + ":created",
			Payload:       payload,
			OccurredAt:    request.CreatedAt,
		})
	})
	if err != nil {
		t.Fatalf("transaction: %v", err)
	}
	assertTableCount(t, ctx, db, "approval_requests", 1)
	assertTableCount(t, ctx, db, "governance_audit_events", 1)
	assertTableCount(t, ctx, db, "outbox_records", 1)
}

func TestGovernanceCascadeDeletesRequirementsDecisionsAndOwners(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	moduleVersionID := createTestModuleVersion(t, ctx, db, "v10.0.4")
	report := createTestBreakingReport(t, ctx, db, moduleVersionID, "00000000-0000-0000-0000-000000030005")
	ownerRepo := NewModuleOwnerRepository(db)
	approvalRepo := NewApprovalRepository(db)
	module := domain.Module{ID: report.ModuleID, Name: report.ModuleName}
	owner := testModuleOwner(t, module, "00000000-0000-0000-0000-000000020005", domain.GovernanceSubjectTypeUser, "alice", domain.ModuleOwnerRoleOwner)
	request := testApprovalRequest(t, report, "00000000-0000-0000-0000-000000040005")
	requirement := testApprovalRequirement(t, request.ID, "00000000-0000-0000-0000-000000050005", domain.ApprovalRequirementTypeModuleOwnerApproval, request.ModuleID, request.ModuleName)
	request.Requirements = []domain.ApprovalRequirement{requirement}
	if err := ownerRepo.Add(ctx, owner); err != nil {
		t.Fatalf("add owner: %v", err)
	}
	if err := approvalRepo.CreateRequest(ctx, request); err != nil {
		t.Fatalf("create request: %v", err)
	}
	if err := approvalRepo.AddDecision(ctx, domain.ApprovalDecision{
		ID:                domain.NewApprovalDecisionID("00000000-0000-0000-0000-000000060002"),
		ApprovalRequestID: request.ID,
		RequirementID:     requirement.ID,
		Decision:          domain.ApprovalDecisionValueApproved,
		DecidedBy:         "alice",
		CreatedAt:         time.Now().UTC(),
	}); err != nil {
		t.Fatalf("add decision: %v", err)
	}

	if _, err := db.executor(ctx).Exec(ctx, `DELETE FROM approval_requests WHERE id = $1`, request.ID.String()); err != nil {
		t.Fatalf("delete approval request: %v", err)
	}
	assertTableCount(t, ctx, db, "approval_requirements", 0)
	assertTableCount(t, ctx, db, "approval_decisions", 0)

	if _, err := db.executor(ctx).Exec(ctx, `DELETE FROM modules WHERE id = $1`, report.ModuleID.String()); err != nil {
		t.Fatalf("delete module: %v", err)
	}
	assertTableCount(t, ctx, db, "module_owners", 0)
}

func testModuleOwner(t *testing.T, module domain.Module, id string, subjectType domain.GovernanceSubjectType, subject string, role domain.ModuleOwnerRole) domain.ModuleOwner {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Microsecond)
	owner := domain.ModuleOwner{
		ID:          domain.NewModuleOwnerID(id),
		ModuleID:    module.ID,
		ModuleName:  module.Name,
		SubjectType: subjectType,
		Subject:     subject,
		Role:        role,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := owner.Validate(); err != nil {
		t.Fatalf("owner validation: %v", err)
	}
	return owner
}

func testApprovalRequest(t *testing.T, report domain.BreakingReport, id string) domain.ApprovalRequest {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Microsecond)
	return domain.ApprovalRequest{
		ID:                domain.NewApprovalRequestID(id),
		ModuleID:          report.ModuleID,
		ModuleName:        report.ModuleName,
		BreakingReportID:  &report.ID,
		TargetRef:         report.TargetRef,
		Status:            domain.ApprovalRequestStatusPending,
		RequiredApprovals: 1,
		ReceivedApprovals: 0,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
}

func testApprovalRequirement(t *testing.T, requestID domain.ApprovalRequestID, id string, requirementType domain.ApprovalRequirementType, targetModuleID domain.ModuleID, targetModuleName domain.ModuleName) domain.ApprovalRequirement {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Microsecond)
	return domain.ApprovalRequirement{
		ID:                domain.NewApprovalRequirementID(id),
		ApprovalRequestID: requestID,
		RequirementType:   requirementType,
		TargetModuleID:    &targetModuleID,
		TargetModuleName:  targetModuleName,
		RequiredRole:      domain.ModuleOwnerRoleOwner,
		Status:            domain.ApprovalRequirementStatusPending,
		Reason:            "breaking changes require owner approval",
		CreatedAt:         now,
		UpdatedAt:         now,
	}
}

func createTestBreakingReport(t *testing.T, ctx context.Context, db *DB, moduleVersionID domain.ModuleVersionID, id string) domain.BreakingReport {
	t.Helper()
	repo := NewBreakingReportRepository(db)
	report := testBreakingReport(t, moduleVersionID, id, domain.BreakingReportStatusBreaking, 1, time.Now().UTC().Truncate(time.Microsecond))
	if err := repo.Create(ctx, report, nil); err != nil {
		t.Fatalf("create breaking report: %v", err)
	}
	return report
}

func runConcurrently(operations ...func() error) []error {
	errs := make([]error, len(operations))
	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(len(operations))
	for i, operation := range operations {
		go func() {
			defer wg.Done()
			<-start
			errs[i] = operation()
		}()
	}
	close(start)
	wg.Wait()
	return errs
}

func assertOneSuccessOneDuplicate(t *testing.T, errs []error) {
	t.Helper()
	successes := 0
	duplicates := 0
	for _, err := range errs {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, domain.ErrDuplicate):
			duplicates++
		default:
			t.Fatalf("unexpected concurrent error: %v", err)
		}
	}
	if successes != 1 || duplicates != 1 {
		t.Fatalf("successes=%d duplicates=%d errors=%#v, want one success and one duplicate", successes, duplicates, errs)
	}
}

func assertApprovalRequestCountByBreakingReport(t *testing.T, ctx context.Context, db *DB, reportID domain.BreakingReportID, want int) {
	t.Helper()
	var got int
	if err := db.executor(ctx).QueryRow(ctx, `SELECT count(*) FROM approval_requests WHERE breaking_report_id = $1`, reportID.String()).Scan(&got); err != nil {
		t.Fatalf("count approval requests by breaking report: %v", err)
	}
	if got != want {
		t.Fatalf("approval request count = %d, want %d", got, want)
	}
}

func assertDecisionCountByRequirement(t *testing.T, ctx context.Context, db *DB, requirementID domain.ApprovalRequirementID, want int) {
	t.Helper()
	var got int
	if err := db.executor(ctx).QueryRow(ctx, `SELECT count(*) FROM approval_decisions WHERE requirement_id = $1`, requirementID.String()).Scan(&got); err != nil {
		t.Fatalf("count approval decisions by requirement: %v", err)
	}
	if got != want {
		t.Fatalf("approval decision count = %d, want %d", got, want)
	}
}

func assertModuleOwner(t *testing.T, got domain.ModuleOwner, want domain.ModuleOwner) {
	t.Helper()
	if got.ID != want.ID || got.ModuleID != want.ModuleID || got.ModuleName != want.ModuleName {
		t.Fatalf("owner identity = %#v, want %#v", got, want)
	}
	if got.SubjectType != want.SubjectType || got.Subject != want.Subject || got.Role != want.Role {
		t.Fatalf("owner subject/role = %#v, want %#v", got, want)
	}
}

func assertApprovalRequest(t *testing.T, got domain.ApprovalRequest, want domain.ApprovalRequest) {
	t.Helper()
	if got.ID != want.ID || got.ModuleID != want.ModuleID || got.ModuleName != want.ModuleName {
		t.Fatalf("request identity = %#v, want %#v", got, want)
	}
	if got.BreakingReportID == nil || want.BreakingReportID == nil || *got.BreakingReportID != *want.BreakingReportID {
		t.Fatalf("breaking report id = %#v, want %#v", got.BreakingReportID, want.BreakingReportID)
	}
	if got.TargetRef != want.TargetRef || got.Status != want.Status || got.RequiredApprovals != want.RequiredApprovals || got.ReceivedApprovals != want.ReceivedApprovals {
		t.Fatalf("request state = %#v, want %#v", got, want)
	}
}

func assertCheckViolation(t *testing.T, err error, constraintName string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected check violation %s, got nil", constraintName)
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		t.Fatalf("error = %T %[1]v, want PgError", err)
	}
	if pgErr.Code != "23514" || pgErr.ConstraintName != constraintName {
		t.Fatalf("pg error code=%s constraint=%s, want 23514 %s; error=%v", pgErr.Code, pgErr.ConstraintName, constraintName, err)
	}
}

func uuidForTest(value int) string {
	return fmt.Sprintf("00000000-0000-0000-0000-%012d", value)
}
