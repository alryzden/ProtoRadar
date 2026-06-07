package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/alryzden/ProtoRadar/internal/domain"
)

type ModuleOwnerRepository struct {
	db *DB
}

func NewModuleOwnerRepository(db *DB) *ModuleOwnerRepository {
	return &ModuleOwnerRepository{db: db}
}

func (repo *ModuleOwnerRepository) Add(ctx context.Context, owner domain.ModuleOwner) error {
	_, err := repo.db.executor(ctx).Exec(ctx, `
		INSERT INTO module_owners (id, module_id, subject_type, subject, role, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`,
		owner.ID.String(),
		owner.ModuleID.String(),
		owner.SubjectType.String(),
		owner.Subject,
		owner.Role.String(),
		owner.CreatedAt,
		owner.UpdatedAt,
	)
	return mapError(err)
}

func (repo *ModuleOwnerRepository) Remove(ctx context.Context, id domain.ModuleOwnerID, _ time.Time) error {
	tag, err := repo.db.executor(ctx).Exec(ctx, `DELETE FROM module_owners WHERE id = $1`, id.String())
	if err != nil {
		return mapError(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (repo *ModuleOwnerRepository) GetByID(ctx context.Context, id domain.ModuleOwnerID) (domain.ModuleOwner, error) {
	owner, err := scanModuleOwner(repo.db.executor(ctx).QueryRow(ctx, `
		SELECT mo.id, mo.module_id, m.name, mo.subject_type, mo.subject, mo.role, mo.created_at, mo.updated_at
		FROM module_owners mo
		JOIN modules m ON m.id = mo.module_id
		WHERE mo.id = $1
	`, id.String()).Scan)
	if err != nil {
		return domain.ModuleOwner{}, mapError(err)
	}
	return owner, nil
}

func (repo *ModuleOwnerRepository) ListByModule(ctx context.Context, moduleID domain.ModuleID) ([]domain.ModuleOwner, error) {
	rows, err := repo.db.executor(ctx).Query(ctx, `
		SELECT mo.id, mo.module_id, m.name, mo.subject_type, mo.subject, mo.role, mo.created_at, mo.updated_at
		FROM module_owners mo
		JOIN modules m ON m.id = mo.module_id
		WHERE mo.module_id = $1
		ORDER BY mo.role ASC, mo.subject_type ASC, mo.subject ASC
	`, moduleID.String())
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	owners := make([]domain.ModuleOwner, 0)
	for rows.Next() {
		owner, err := scanModuleOwner(rows.Scan)
		if err != nil {
			return nil, err
		}
		owners = append(owners, owner)
	}
	if err := rows.Err(); err != nil {
		return nil, mapError(err)
	}
	return owners, nil
}

func (repo *ModuleOwnerRepository) HasRole(ctx context.Context, moduleID domain.ModuleID, subjectType domain.GovernanceSubjectType, subject string, roles []domain.ModuleOwnerRole) (bool, error) {
	rows, err := repo.db.executor(ctx).Query(ctx, `
		SELECT role
		FROM module_owners
		WHERE module_id = $1 AND subject_type = $2 AND subject = $3
	`, moduleID.String(), subjectType.String(), subject)
	if err != nil {
		return false, mapError(err)
	}
	defer rows.Close()

	allowed := make(map[domain.ModuleOwnerRole]struct{}, len(roles))
	for _, role := range roles {
		allowed[role] = struct{}{}
	}
	for rows.Next() {
		var roleValue string
		if err := rows.Scan(&roleValue); err != nil {
			return false, err
		}
		role, err := domain.NewModuleOwnerRole(roleValue)
		if err != nil {
			return false, err
		}
		if _, ok := allowed[role]; ok {
			return true, nil
		}
	}
	if err := rows.Err(); err != nil {
		return false, mapError(err)
	}
	return false, nil
}

type ApprovalRepository struct {
	db *DB
}

func NewApprovalRepository(db *DB) *ApprovalRepository {
	return &ApprovalRepository{db: db}
}

func (repo *ApprovalRepository) CreateRequest(ctx context.Context, request domain.ApprovalRequest) error {
	return repo.db.WithinTransaction(ctx, func(txCtx context.Context) error {
		_, err := repo.db.executor(txCtx).Exec(txCtx, `
			INSERT INTO approval_requests (
				id,
				module_id,
				breaking_report_id,
				target_ref,
				status,
				required_approvals,
				received_approvals,
				created_at,
				updated_at
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		`,
			request.ID.String(),
			request.ModuleID.String(),
			nullableBreakingReportID(request.BreakingReportID),
			request.TargetRef,
			request.Status.String(),
			request.RequiredApprovals,
			request.ReceivedApprovals,
			request.CreatedAt,
			request.UpdatedAt,
		)
		if err != nil {
			return mapError(err)
		}
		for _, requirement := range request.Requirements {
			if err := repo.createRequirement(txCtx, request.ID, requirement); err != nil {
				return err
			}
		}
		return nil
	})
}

func (repo *ApprovalRepository) GetRequestByID(ctx context.Context, id domain.ApprovalRequestID) (domain.ApprovalRequest, error) {
	request, err := repo.getRequest(ctx, `
		SELECT ar.id, ar.module_id, m.name, ar.breaking_report_id, ar.target_ref, ar.status,
			ar.required_approvals, ar.received_approvals, ar.created_at, ar.updated_at
		FROM approval_requests ar
		JOIN modules m ON m.id = ar.module_id
		WHERE ar.id = $1
	`, id.String())
	if err != nil {
		return domain.ApprovalRequest{}, err
	}
	return repo.populateRequest(ctx, request)
}

func (repo *ApprovalRepository) GetRequestByBreakingReportID(ctx context.Context, reportID domain.BreakingReportID) (domain.ApprovalRequest, error) {
	request, err := repo.getRequest(ctx, `
		SELECT ar.id, ar.module_id, m.name, ar.breaking_report_id, ar.target_ref, ar.status,
			ar.required_approvals, ar.received_approvals, ar.created_at, ar.updated_at
		FROM approval_requests ar
		JOIN modules m ON m.id = ar.module_id
		WHERE ar.breaking_report_id = $1
		ORDER BY ar.created_at DESC
		LIMIT 1
	`, reportID.String())
	if err != nil {
		return domain.ApprovalRequest{}, err
	}
	return repo.populateRequest(ctx, request)
}

func (repo *ApprovalRepository) GetRequestByRequirementID(ctx context.Context, requirementID domain.ApprovalRequirementID) (domain.ApprovalRequest, error) {
	request, err := repo.getRequest(ctx, `
		SELECT ar.id, ar.module_id, m.name, ar.breaking_report_id, ar.target_ref, ar.status,
			ar.required_approvals, ar.received_approvals, ar.created_at, ar.updated_at
		FROM approval_requests ar
		JOIN modules m ON m.id = ar.module_id
		JOIN approval_requirements req ON req.approval_request_id = ar.id
		WHERE req.id = $1
	`, requirementID.String())
	if err != nil {
		return domain.ApprovalRequest{}, err
	}
	return repo.populateRequest(ctx, request)
}

func (repo *ApprovalRepository) AddDecision(ctx context.Context, decision domain.ApprovalDecision) error {
	_, err := repo.db.executor(ctx).Exec(ctx, `
		INSERT INTO approval_decisions (id, approval_request_id, requirement_id, decision, decided_by, comment, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`,
		decision.ID.String(),
		decision.ApprovalRequestID.String(),
		decision.RequirementID.String(),
		decision.Decision.String(),
		decision.DecidedBy,
		decision.Comment,
		decision.CreatedAt,
	)
	return mapError(err)
}

func (repo *ApprovalRepository) UpdateRequirementStatus(ctx context.Context, id domain.ApprovalRequirementID, status domain.ApprovalRequirementStatus, updatedAt time.Time) error {
	tag, err := repo.db.executor(ctx).Exec(ctx, `
		UPDATE approval_requirements
		SET status = $2, updated_at = $3
		WHERE id = $1
	`, id.String(), status.String(), updatedAt)
	if err != nil {
		return mapError(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (repo *ApprovalRepository) UpdateRequestStatus(ctx context.Context, id domain.ApprovalRequestID, status domain.ApprovalRequestStatus, requiredApprovals int, receivedApprovals int, updatedAt time.Time) error {
	tag, err := repo.db.executor(ctx).Exec(ctx, `
		UPDATE approval_requests
		SET status = $2, required_approvals = $3, received_approvals = $4, updated_at = $5
		WHERE id = $1
	`, id.String(), status.String(), requiredApprovals, receivedApprovals, updatedAt)
	if err != nil {
		return mapError(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (repo *ApprovalRepository) ListRequestsByModule(ctx context.Context, moduleID domain.ModuleID, limit int, offset int) ([]domain.ApprovalRequest, error) {
	rows, err := repo.db.executor(ctx).Query(ctx, `
		SELECT ar.id, ar.module_id, m.name, ar.breaking_report_id, ar.target_ref, ar.status,
			ar.required_approvals, ar.received_approvals, ar.created_at, ar.updated_at
		FROM approval_requests ar
		JOIN modules m ON m.id = ar.module_id
		WHERE ar.module_id = $1
		ORDER BY ar.created_at DESC
		LIMIT $2 OFFSET $3
	`, moduleID.String(), limit, offset)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	requests := make([]domain.ApprovalRequest, 0)
	for rows.Next() {
		request, err := scanApprovalRequest(rows.Scan)
		if err != nil {
			return nil, err
		}
		requests = append(requests, request)
	}
	if err := rows.Err(); err != nil {
		return nil, mapError(err)
	}
	return requests, nil
}

func (repo *ApprovalRepository) createRequirement(ctx context.Context, requestID domain.ApprovalRequestID, requirement domain.ApprovalRequirement) error {
	approvalRequestID := requirement.ApprovalRequestID
	if approvalRequestID == "" {
		approvalRequestID = requestID
	}
	_, err := repo.db.executor(ctx).Exec(ctx, `
		INSERT INTO approval_requirements (
			id,
			approval_request_id,
			requirement_type,
			target_module_id,
			target_module_name,
			required_role,
			status,
			reason,
			created_at,
			updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`,
		requirement.ID.String(),
		approvalRequestID.String(),
		requirement.RequirementType.String(),
		nullableModuleID(requirement.TargetModuleID),
		requirement.TargetModuleName.String(),
		requirement.RequiredRole.String(),
		requirement.Status.String(),
		requirement.Reason,
		requirement.CreatedAt,
		requirement.UpdatedAt,
	)
	return mapError(err)
}

func (repo *ApprovalRepository) getRequest(ctx context.Context, query string, args ...any) (domain.ApprovalRequest, error) {
	request, err := scanApprovalRequest(repo.db.executor(ctx).QueryRow(ctx, query, args...).Scan)
	if err != nil {
		return domain.ApprovalRequest{}, mapError(err)
	}
	return request, nil
}

func (repo *ApprovalRepository) populateRequest(ctx context.Context, request domain.ApprovalRequest) (domain.ApprovalRequest, error) {
	requirements, err := repo.listRequirements(ctx, request.ID)
	if err != nil {
		return domain.ApprovalRequest{}, err
	}
	decisions, err := repo.listDecisions(ctx, request.ID)
	if err != nil {
		return domain.ApprovalRequest{}, err
	}
	request.Requirements = requirements
	request.Decisions = decisions
	return request, nil
}

func (repo *ApprovalRepository) listRequirements(ctx context.Context, requestID domain.ApprovalRequestID) ([]domain.ApprovalRequirement, error) {
	rows, err := repo.db.executor(ctx).Query(ctx, `
		SELECT id, approval_request_id, requirement_type, target_module_id, target_module_name,
			required_role, status, reason, created_at, updated_at
		FROM approval_requirements
		WHERE approval_request_id = $1
		ORDER BY created_at ASC, id ASC
	`, requestID.String())
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	requirements := make([]domain.ApprovalRequirement, 0)
	for rows.Next() {
		requirement, err := scanApprovalRequirement(rows.Scan)
		if err != nil {
			return nil, err
		}
		requirements = append(requirements, requirement)
	}
	if err := rows.Err(); err != nil {
		return nil, mapError(err)
	}
	return requirements, nil
}

func (repo *ApprovalRepository) listDecisions(ctx context.Context, requestID domain.ApprovalRequestID) ([]domain.ApprovalDecision, error) {
	rows, err := repo.db.executor(ctx).Query(ctx, `
		SELECT id, approval_request_id, requirement_id, decision, decided_by, comment, created_at
		FROM approval_decisions
		WHERE approval_request_id = $1
		ORDER BY created_at ASC, id ASC
	`, requestID.String())
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	decisions := make([]domain.ApprovalDecision, 0)
	for rows.Next() {
		decision, err := scanApprovalDecision(rows.Scan)
		if err != nil {
			return nil, err
		}
		decisions = append(decisions, decision)
	}
	if err := rows.Err(); err != nil {
		return nil, mapError(err)
	}
	return decisions, nil
}

type GovernanceAuditRepository struct {
	db *DB
}

func NewGovernanceAuditRepository(db *DB) *GovernanceAuditRepository {
	return &GovernanceAuditRepository{db: db}
}

func (repo *GovernanceAuditRepository) Append(ctx context.Context, event domain.GovernanceAuditEvent) error {
	payload := event.PayloadJSON
	if len(payload) == 0 {
		payload = json.RawMessage(`{}`)
	}
	_, err := repo.db.executor(ctx).Exec(ctx, `
		INSERT INTO governance_audit_events (
			id,
			event_type,
			actor,
			module_id,
			module_name,
			approval_request_id,
			breaking_report_id,
			payload_json,
			created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`,
		event.ID.String(),
		event.EventType.String(),
		event.Actor,
		nullableModuleID(event.ModuleID),
		event.ModuleName.String(),
		nullableApprovalRequestID(event.ApprovalRequestID),
		nullableBreakingReportID(event.BreakingReportID),
		payload,
		event.CreatedAt,
	)
	return mapError(err)
}

func (repo *GovernanceAuditRepository) ListByApprovalRequest(ctx context.Context, approvalRequestID domain.ApprovalRequestID, limit int, offset int) ([]domain.GovernanceAuditEvent, error) {
	return repo.list(ctx, `
		SELECT id, event_type, actor, module_id, module_name, approval_request_id, breaking_report_id, payload_json, created_at
		FROM governance_audit_events
		WHERE approval_request_id = $1
		ORDER BY created_at ASC, id ASC
		LIMIT $2 OFFSET $3
	`, approvalRequestID.String(), limit, offset)
}

func (repo *GovernanceAuditRepository) ListByModule(ctx context.Context, moduleID domain.ModuleID, limit int, offset int) ([]domain.GovernanceAuditEvent, error) {
	return repo.list(ctx, `
		SELECT id, event_type, actor, module_id, module_name, approval_request_id, breaking_report_id, payload_json, created_at
		FROM governance_audit_events
		WHERE module_id = $1
		ORDER BY created_at ASC, id ASC
		LIMIT $2 OFFSET $3
	`, moduleID.String(), limit, offset)
}

func (repo *GovernanceAuditRepository) list(ctx context.Context, query string, args ...any) ([]domain.GovernanceAuditEvent, error) {
	rows, err := repo.db.executor(ctx).Query(ctx, query, args...)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	events := make([]domain.GovernanceAuditEvent, 0)
	for rows.Next() {
		event, err := scanGovernanceAuditEvent(rows.Scan)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, mapError(err)
	}
	return events, nil
}

func scanModuleOwner(scan func(dest ...any) error) (domain.ModuleOwner, error) {
	var owner domain.ModuleOwner
	var id string
	var moduleID string
	var moduleNameValue string
	var subjectTypeValue string
	var roleValue string
	if err := scan(&id, &moduleID, &moduleNameValue, &subjectTypeValue, &owner.Subject, &roleValue, &owner.CreatedAt, &owner.UpdatedAt); err != nil {
		return domain.ModuleOwner{}, err
	}
	moduleName, err := domain.NewModuleName(moduleNameValue)
	if err != nil {
		return domain.ModuleOwner{}, err
	}
	subjectType, err := domain.NewGovernanceSubjectType(subjectTypeValue)
	if err != nil {
		return domain.ModuleOwner{}, err
	}
	role, err := domain.NewModuleOwnerRole(roleValue)
	if err != nil {
		return domain.ModuleOwner{}, err
	}
	owner.ID = domain.NewModuleOwnerID(id)
	owner.ModuleID = domain.NewModuleID(moduleID)
	owner.ModuleName = moduleName
	owner.SubjectType = subjectType
	owner.Role = role
	return owner, nil
}

func scanApprovalRequest(scan func(dest ...any) error) (domain.ApprovalRequest, error) {
	var request domain.ApprovalRequest
	var id string
	var moduleID string
	var moduleNameValue string
	var breakingReportID sql.NullString
	var statusValue string
	if err := scan(
		&id,
		&moduleID,
		&moduleNameValue,
		&breakingReportID,
		&request.TargetRef,
		&statusValue,
		&request.RequiredApprovals,
		&request.ReceivedApprovals,
		&request.CreatedAt,
		&request.UpdatedAt,
	); err != nil {
		return domain.ApprovalRequest{}, err
	}
	moduleName, err := domain.NewModuleName(moduleNameValue)
	if err != nil {
		return domain.ApprovalRequest{}, err
	}
	status, err := domain.NewApprovalRequestStatus(statusValue)
	if err != nil {
		return domain.ApprovalRequest{}, err
	}
	request.ID = domain.NewApprovalRequestID(id)
	request.ModuleID = domain.NewModuleID(moduleID)
	request.ModuleName = moduleName
	request.Status = status
	if breakingReportID.Valid {
		id := domain.NewBreakingReportID(breakingReportID.String)
		request.BreakingReportID = &id
	}
	return request, nil
}

func scanApprovalRequirement(scan func(dest ...any) error) (domain.ApprovalRequirement, error) {
	var requirement domain.ApprovalRequirement
	var id string
	var approvalRequestID string
	var requirementTypeValue string
	var targetModuleID sql.NullString
	var targetModuleNameValue string
	var requiredRoleValue string
	var statusValue string
	if err := scan(
		&id,
		&approvalRequestID,
		&requirementTypeValue,
		&targetModuleID,
		&targetModuleNameValue,
		&requiredRoleValue,
		&statusValue,
		&requirement.Reason,
		&requirement.CreatedAt,
		&requirement.UpdatedAt,
	); err != nil {
		return domain.ApprovalRequirement{}, err
	}
	requirementType, err := domain.NewApprovalRequirementType(requirementTypeValue)
	if err != nil {
		return domain.ApprovalRequirement{}, err
	}
	status, err := domain.NewApprovalRequirementStatus(statusValue)
	if err != nil {
		return domain.ApprovalRequirement{}, err
	}
	requirement.ID = domain.NewApprovalRequirementID(id)
	requirement.ApprovalRequestID = domain.NewApprovalRequestID(approvalRequestID)
	requirement.RequirementType = requirementType
	if targetModuleID.Valid {
		id := domain.NewModuleID(targetModuleID.String)
		requirement.TargetModuleID = &id
	}
	if targetModuleNameValue != "" {
		moduleName, err := domain.NewModuleName(targetModuleNameValue)
		if err != nil {
			return domain.ApprovalRequirement{}, err
		}
		requirement.TargetModuleName = moduleName
	}
	if requiredRoleValue != "" {
		role, err := domain.NewModuleOwnerRole(requiredRoleValue)
		if err != nil {
			return domain.ApprovalRequirement{}, err
		}
		requirement.RequiredRole = role
	}
	requirement.Status = status
	return requirement, nil
}

func scanApprovalDecision(scan func(dest ...any) error) (domain.ApprovalDecision, error) {
	var decision domain.ApprovalDecision
	var id string
	var approvalRequestID string
	var requirementID string
	var decisionValue string
	if err := scan(&id, &approvalRequestID, &requirementID, &decisionValue, &decision.DecidedBy, &decision.Comment, &decision.CreatedAt); err != nil {
		return domain.ApprovalDecision{}, err
	}
	value, err := domain.NewApprovalDecisionValue(decisionValue)
	if err != nil {
		return domain.ApprovalDecision{}, err
	}
	decision.ID = domain.NewApprovalDecisionID(id)
	decision.ApprovalRequestID = domain.NewApprovalRequestID(approvalRequestID)
	decision.RequirementID = domain.NewApprovalRequirementID(requirementID)
	decision.Decision = value
	return decision, nil
}

func scanGovernanceAuditEvent(scan func(dest ...any) error) (domain.GovernanceAuditEvent, error) {
	var event domain.GovernanceAuditEvent
	var id string
	var eventType string
	var moduleID sql.NullString
	var moduleNameValue string
	var approvalRequestID sql.NullString
	var breakingReportID sql.NullString
	if err := scan(
		&id,
		&eventType,
		&event.Actor,
		&moduleID,
		&moduleNameValue,
		&approvalRequestID,
		&breakingReportID,
		&event.PayloadJSON,
		&event.CreatedAt,
	); err != nil {
		return domain.GovernanceAuditEvent{}, err
	}
	event.ID = domain.NewGovernanceAuditEventID(id)
	event.EventType = domain.GovernanceAuditEventType(eventType)
	if moduleNameValue != "" {
		moduleName, err := domain.NewModuleName(moduleNameValue)
		if err != nil {
			return domain.GovernanceAuditEvent{}, err
		}
		event.ModuleName = moduleName
	}
	if moduleID.Valid {
		id := domain.NewModuleID(moduleID.String)
		event.ModuleID = &id
	}
	if approvalRequestID.Valid {
		id := domain.NewApprovalRequestID(approvalRequestID.String)
		event.ApprovalRequestID = &id
	}
	if breakingReportID.Valid {
		id := domain.NewBreakingReportID(breakingReportID.String)
		event.BreakingReportID = &id
	}
	return event, nil
}

func nullableApprovalRequestID(id *domain.ApprovalRequestID) any {
	if id == nil {
		return nil
	}
	return id.String()
}

func nullableBreakingReportID(id *domain.BreakingReportID) any {
	if id == nil {
		return nil
	}
	return id.String()
}

var _ domain.ModuleOwnerRepository = (*ModuleOwnerRepository)(nil)
var _ domain.ApprovalRepository = (*ApprovalRepository)(nil)
var _ domain.GovernanceAuditRepository = (*GovernanceAuditRepository)(nil)
