package governance

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/alryzden/ProtoRadar/internal/audit"
	"github.com/alryzden/ProtoRadar/internal/domain"
	"github.com/alryzden/ProtoRadar/internal/integration/protoradarevents"
	"github.com/alryzden/ProtoRadar/internal/outbox"
)

const defaultApprovalListLimit = 100

type AffectedModulesProvider interface {
	ListAffectedModules(ctx context.Context, providerModuleID domain.ModuleID) ([]domain.AffectedModule, error)
}

type RuntimeImpactProvider interface {
	ListRuntimeImpactByModuleVersion(ctx context.Context, reportID domain.BreakingReportID, moduleVersionID domain.ModuleVersionID, limit int, offset int) ([]domain.RuntimeImpact, error)
}

type ApprovalService struct {
	modules      domain.ModuleRepository
	reports      domain.BreakingReportRepository
	owners       domain.ModuleOwnerRepository
	approvals    domain.ApprovalRepository
	audit        audit.AuditSink
	auditRead    domain.GovernanceAuditRepository
	dependencies AffectedModulesProvider
	runtime      RuntimeImpactProvider
	transactions domain.RegistryTransactionManager
	outbox       outbox.Writer
	policy       PolicyEvaluator
	policyConfig PolicyConfig
	clock        Clock
	ids          IDGenerator
}

func NewApprovalService(
	modules domain.ModuleRepository,
	reports domain.BreakingReportRepository,
	owners domain.ModuleOwnerRepository,
	approvals domain.ApprovalRepository,
	auditSink audit.AuditSink,
	auditRead domain.GovernanceAuditRepository,
	dependencies AffectedModulesProvider,
	runtime RuntimeImpactProvider,
	transactions domain.RegistryTransactionManager,
	outboxWriter outbox.Writer,
	policy PolicyEvaluator,
	policyConfig PolicyConfig,
	clock Clock,
	ids IDGenerator,
) *ApprovalService {
	if policy == nil {
		policy = DefaultPolicyEvaluator{}
	}
	return &ApprovalService{
		modules:      modules,
		reports:      reports,
		owners:       owners,
		approvals:    approvals,
		audit:        auditSink,
		auditRead:    auditRead,
		dependencies: dependencies,
		runtime:      runtime,
		transactions: transactions,
		outbox:       outboxWriter,
		policy:       policy,
		policyConfig: policyConfig,
		clock:        clock,
		ids:          ids,
	}
}

type CreateApprovalRequestForBreakingReportInput struct {
	BreakingReportID string
	Actor            string
	TargetRef        string
}

type GetApprovalStatusForBreakingReportInput struct {
	BreakingReportID string
}

type ApprovalDecisionInput struct {
	RequestID     string
	RequirementID string
	Actor         string
	Comment       string
}

type ListApprovalRequestAuditInput struct {
	RequestID string
	Limit     int
	Offset    int
}

func (svc *ApprovalService) CreateApprovalRequestForBreakingReport(ctx context.Context, input CreateApprovalRequestForBreakingReportInput) (domain.ApprovalRequest, error) {
	reportID := domain.NewBreakingReportID(input.BreakingReportID)
	if reportID == "" {
		return domain.ApprovalRequest{}, ErrInvalidBreakingReportID
	}
	actor, err := effectiveActorSubject(input.Actor)
	if err != nil {
		return domain.ApprovalRequest{}, err
	}
	if existing, err := svc.approvals.GetRequestByBreakingReportID(ctx, reportID); err == nil {
		return existing, nil
	} else if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return domain.ApprovalRequest{}, err
	}

	policyContext, err := svc.approvalPolicyContextForReport(ctx, reportID)
	if err != nil {
		return domain.ApprovalRequest{}, err
	}
	policyResult := svc.policy.Evaluate(EvaluatePolicyInput{
		Report:          policyContext.Report,
		ModuleOwners:    policyContext.ModuleOwners,
		AffectedModules: policyContext.AffectedModules,
		RuntimeImpact:   policyContext.RuntimeImpact,
		Config:          svc.policyConfig,
	})

	now := svc.clock.Now()
	request, err := svc.buildApprovalRequestFromPolicyDecision(policyContext, input.TargetRef, policyResult, now)
	if err != nil {
		return domain.ApprovalRequest{}, err
	}
	if err := svc.createApprovalRequestWithAuditAndOutbox(ctx, request, actor, policyResult, now); err != nil {
		if errors.Is(err, ErrApprovalRequestConflict) {
			existing, existingErr := svc.approvals.GetRequestByBreakingReportID(ctx, reportID)
			if existingErr == nil {
				return existing, nil
			}
		}
		return domain.ApprovalRequest{}, err
	}
	return request, nil
}

func (svc *ApprovalService) GetApprovalStatusForBreakingReport(ctx context.Context, input GetApprovalStatusForBreakingReportInput) (domain.ApprovalRequest, error) {
	reportID := domain.NewBreakingReportID(input.BreakingReportID)
	if reportID == "" {
		return domain.ApprovalRequest{}, ErrInvalidBreakingReportID
	}
	if _, _, err := svc.reports.GetByID(ctx, reportID); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return domain.ApprovalRequest{}, ErrBreakingReportNotFound
		}
		return domain.ApprovalRequest{}, err
	}
	request, err := svc.approvals.GetRequestByBreakingReportID(ctx, reportID)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.ApprovalRequest{}, ErrApprovalRequestNotFound
	}
	if err != nil {
		return domain.ApprovalRequest{}, err
	}
	return request, nil
}

func (svc *ApprovalService) ApproveRequirement(ctx context.Context, input ApprovalDecisionInput) (domain.ApprovalRequest, error) {
	return svc.recordDecision(ctx, input, domain.ApprovalDecisionValueApproved)
}

func (svc *ApprovalService) RejectRequirement(ctx context.Context, input ApprovalDecisionInput) (domain.ApprovalRequest, error) {
	return svc.recordDecision(ctx, input, domain.ApprovalDecisionValueRejected)
}

func (svc *ApprovalService) ListApprovalRequestAudit(ctx context.Context, input ListApprovalRequestAuditInput) ([]domain.GovernanceAuditEvent, error) {
	requestID := domain.NewApprovalRequestID(input.RequestID)
	if requestID == "" {
		return nil, ErrApprovalRequestNotFound
	}
	if _, err := svc.approvals.GetRequestByID(ctx, requestID); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, ErrApprovalRequestNotFound
		}
		return nil, err
	}
	limit := input.Limit
	if limit <= 0 {
		limit = defaultApprovalListLimit
	}
	events, err := svc.auditRead.ListByApprovalRequest(ctx, requestID, limit, input.Offset)
	if err != nil {
		return nil, err
	}
	return events, nil
}

func (svc *ApprovalService) recordDecision(ctx context.Context, input ApprovalDecisionInput, decisionValue domain.ApprovalDecisionValue) (domain.ApprovalRequest, error) {
	requirementID := domain.NewApprovalRequirementID(input.RequirementID)
	if requirementID == "" {
		return domain.ApprovalRequest{}, ErrInvalidRequirementID
	}
	actor, err := effectiveActorSubject(input.Actor)
	if err != nil {
		return domain.ApprovalRequest{}, err
	}
	if len(input.Comment) > domain.MaxApprovalDecisionCommentLength {
		return domain.ApprovalRequest{}, ErrApprovalDecisionCommentTooLong
	}
	request, requirement, err := svc.getRequestByRequirement(ctx, requirementID)
	if err != nil {
		return domain.ApprovalRequest{}, err
	}
	if expectedRequestID := domain.NewApprovalRequestID(input.RequestID); expectedRequestID != "" && request.ID != expectedRequestID {
		return domain.ApprovalRequest{}, ErrApprovalRequestNotFound
	}
	if hasDecisionForRequirement(request.Decisions, requirement.ID) {
		return domain.ApprovalRequest{}, ErrApprovalRequirementConflict
	}
	if request.Status != domain.ApprovalRequestStatusPending {
		return domain.ApprovalRequest{}, ErrApprovalRequestConflict
	}
	if requirement.Status != domain.ApprovalRequirementStatusPending {
		return domain.ApprovalRequest{}, ErrApprovalRequirementConflict
	}
	if !svc.actorCanDecide(ctx, requirement, actor) {
		return domain.ApprovalRequest{}, ErrApprovalActorForbidden
	}

	now := svc.clock.Now()
	decisionID, err := svc.ids.NewApprovalDecisionID()
	if err != nil {
		return domain.ApprovalRequest{}, err
	}
	decision := domain.ApprovalDecision{
		ID:                decisionID,
		ApprovalRequestID: request.ID,
		RequirementID:     requirement.ID,
		Decision:          decisionValue,
		DecidedBy:         actor,
		Comment:           input.Comment,
		CreatedAt:         now,
	}
	if err := decision.Validate(); err != nil {
		return domain.ApprovalRequest{}, mapDecisionValidationError(err)
	}

	newRequirementStatus := domain.ApprovalRequirementStatusApproved
	newRequestStatus := domain.ApprovalRequestStatusPending
	if decisionValue == domain.ApprovalDecisionValueRejected {
		newRequirementStatus = domain.ApprovalRequirementStatusRejected
		newRequestStatus = domain.ApprovalRequestStatusRejected
	} else if allRequirementsApprovedAfter(request.Requirements, requirement.ID) {
		newRequestStatus = domain.ApprovalRequestStatusApproved
	}
	requiredApprovals := len(request.Requirements)
	receivedApprovals := receivedApprovalsAfter(request.Requirements, requirement.ID, newRequirementStatus)
	statusChanged := request.Status != newRequestStatus

	err = svc.transactions.WithinTransaction(ctx, func(txCtx context.Context) error {
		if err := svc.approvals.AddDecision(txCtx, decision); err != nil {
			if errors.Is(err, domain.ErrDuplicate) {
				return ErrApprovalRequirementConflict
			}
			return err
		}
		if err := svc.approvals.UpdateRequirementStatus(txCtx, requirement.ID, newRequirementStatus, now); err != nil {
			return err
		}
		if err := svc.approvals.UpdateRequestStatus(txCtx, request.ID, newRequestStatus, requiredApprovals, receivedApprovals, now); err != nil {
			return err
		}
		if err := svc.appendDecisionAudit(txCtx, request, decision, actor, now); err != nil {
			return err
		}
		if statusChanged {
			if err := svc.appendStatusChangedAudit(txCtx, request, newRequestStatus, actor, now); err != nil {
				return err
			}
		}
		record, err := protoradarevents.NewApprovalDecisionRecorded(protoradarevents.ApprovalDecisionRecorded{
			Decision:         decision,
			ModuleID:         request.ModuleID,
			ModuleName:       request.ModuleName,
			BreakingReportID: request.BreakingReportID,
			OccurredAt:       now,
		})
		if err != nil {
			return err
		}
		return svc.outbox.Create(txCtx, record)
	})
	if err != nil {
		return domain.ApprovalRequest{}, err
	}
	updated, err := svc.approvals.GetRequestByID(ctx, request.ID)
	if err != nil {
		return domain.ApprovalRequest{}, err
	}
	return updated, nil
}

func hasDecisionForRequirement(decisions []domain.ApprovalDecision, requirementID domain.ApprovalRequirementID) bool {
	for _, decision := range decisions {
		if decision.RequirementID == requirementID {
			return true
		}
	}
	return false
}

func (svc *ApprovalService) affectedModules(ctx context.Context, moduleID domain.ModuleID) ([]domain.AffectedModule, error) {
	if svc.dependencies == nil {
		return nil, nil
	}
	affected, err := svc.dependencies.ListAffectedModules(ctx, moduleID)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}
	return affected, nil
}

func (svc *ApprovalService) runtimeImpact(ctx context.Context, report domain.BreakingReport) ([]domain.RuntimeImpact, error) {
	if svc.runtime == nil || report.BaseVersionID == "" {
		return nil, nil
	}
	impact, err := svc.runtime.ListRuntimeImpactByModuleVersion(ctx, report.ID, report.BaseVersionID, defaultApprovalListLimit, 0)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}
	return impact, nil
}

type approvalPolicyContext struct {
	Report          domain.BreakingReport
	Module          domain.Module
	ModuleOwners    []domain.ModuleOwner
	AffectedModules []PolicyAffectedModule
	RuntimeImpact   []domain.RuntimeImpact
}

func (svc *ApprovalService) approvalPolicyContextForReport(ctx context.Context, reportID domain.BreakingReportID) (approvalPolicyContext, error) {
	report, _, err := svc.reports.GetByID(ctx, reportID)
	if errors.Is(err, domain.ErrNotFound) {
		return approvalPolicyContext{}, ErrBreakingReportNotFound
	}
	if err != nil {
		return approvalPolicyContext{}, err
	}
	module, err := svc.modules.GetByID(ctx, report.ModuleID)
	if errors.Is(err, domain.ErrNotFound) {
		return approvalPolicyContext{}, ErrModuleNotFound
	}
	if err != nil {
		return approvalPolicyContext{}, err
	}
	moduleOwners, err := svc.owners.ListByModule(ctx, module.ID)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return approvalPolicyContext{}, err
	}
	affected, err := svc.affectedModules(ctx, report.ModuleID)
	if err != nil {
		return approvalPolicyContext{}, err
	}
	runtimeImpact, err := svc.runtimeImpact(ctx, report)
	if err != nil {
		return approvalPolicyContext{}, err
	}
	policyAffected, err := svc.policyAffectedModules(ctx, affected)
	if err != nil {
		return approvalPolicyContext{}, err
	}
	return approvalPolicyContext{
		Report:          report,
		Module:          module,
		ModuleOwners:    moduleOwners,
		AffectedModules: policyAffected,
		RuntimeImpact:   runtimeImpact,
	}, nil
}

func (svc *ApprovalService) policyAffectedModules(ctx context.Context, affected []domain.AffectedModule) ([]PolicyAffectedModule, error) {
	items := make([]PolicyAffectedModule, 0, len(affected))
	for _, item := range affected {
		module, err := svc.modules.GetByName(ctx, item.ModuleName)
		if errors.Is(err, domain.ErrNotFound) {
			items = append(items, PolicyAffectedModule{ModuleName: item.ModuleName})
			continue
		}
		if err != nil {
			return nil, err
		}
		owners, err := svc.owners.ListByModule(ctx, module.ID)
		if err != nil && !errors.Is(err, domain.ErrNotFound) {
			return nil, err
		}
		moduleID := module.ID
		items = append(items, PolicyAffectedModule{
			ModuleID:   &moduleID,
			ModuleName: module.Name,
			Owners:     owners,
		})
	}
	return items, nil
}

func (svc *ApprovalService) buildApprovalRequestFromPolicyDecision(policyContext approvalPolicyContext, targetRefOverride string, policyResult GovernancePolicyResult, now time.Time) (domain.ApprovalRequest, error) {
	requestID, err := svc.ids.NewApprovalRequestID()
	if err != nil {
		return domain.ApprovalRequest{}, err
	}
	status := domain.ApprovalRequestStatusNotRequired
	if policyResult.ApprovalRequired {
		status = domain.ApprovalRequestStatusPending
	}
	targetRef := strings.TrimSpace(targetRefOverride)
	if targetRef == "" {
		targetRef = policyContext.Report.TargetRef
	}
	reportID := policyContext.Report.ID
	request := domain.ApprovalRequest{
		ID:                requestID,
		ModuleID:          policyContext.Module.ID,
		ModuleName:        policyContext.Module.Name,
		BreakingReportID:  &reportID,
		TargetRef:         targetRef,
		Status:            status,
		RequiredApprovals: len(policyResult.Requirements),
		ReceivedApprovals: 0,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	request.Requirements = make([]domain.ApprovalRequirement, 0, len(policyResult.Requirements))
	for _, plan := range policyResult.Requirements {
		requirementID, err := svc.ids.NewApprovalRequirementID()
		if err != nil {
			return domain.ApprovalRequest{}, err
		}
		request.Requirements = append(request.Requirements, domain.ApprovalRequirement{
			ID:                requirementID,
			ApprovalRequestID: request.ID,
			RequirementType:   plan.RequirementType,
			TargetModuleID:    plan.TargetModuleID,
			TargetModuleName:  plan.TargetModuleName,
			RequiredRole:      plan.RequiredRole,
			Status:            plan.Status,
			Reason:            requirementReason(plan),
			CreatedAt:         now,
			UpdatedAt:         now,
		})
	}
	return request, nil
}

func (svc *ApprovalService) createApprovalRequestWithAuditAndOutbox(ctx context.Context, request domain.ApprovalRequest, actor string, policyResult GovernancePolicyResult, occurredAt time.Time) error {
	return svc.transactions.WithinTransaction(ctx, func(txCtx context.Context) error {
		if err := svc.approvals.CreateRequest(txCtx, request); err != nil {
			if errors.Is(err, domain.ErrDuplicate) {
				return ErrApprovalRequestConflict
			}
			return err
		}
		if err := svc.appendApprovalRequestCreatedAudit(txCtx, request, actor, policyResult, occurredAt); err != nil {
			return err
		}
		record, err := protoradarevents.NewApprovalRequestCreated(protoradarevents.ApprovalRequestCreated{
			Request:    request,
			Actor:      actor,
			OccurredAt: occurredAt,
		})
		if err != nil {
			return err
		}
		return svc.outbox.Create(txCtx, record)
	})
}

func (svc *ApprovalService) getRequestByRequirement(ctx context.Context, requirementID domain.ApprovalRequirementID) (domain.ApprovalRequest, domain.ApprovalRequirement, error) {
	request, err := svc.approvals.GetRequestByRequirementID(ctx, requirementID)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.ApprovalRequest{}, domain.ApprovalRequirement{}, ErrApprovalRequirementNotFound
	}
	if err != nil {
		return domain.ApprovalRequest{}, domain.ApprovalRequirement{}, err
	}
	for _, requirement := range request.Requirements {
		if requirement.ID == requirementID {
			return request, requirement, nil
		}
	}
	return domain.ApprovalRequest{}, domain.ApprovalRequirement{}, ErrApprovalRequirementNotFound
}

func (svc *ApprovalService) actorCanDecide(ctx context.Context, requirement domain.ApprovalRequirement, actor string) bool {
	targetModuleID := requirement.TargetModuleID
	if targetModuleID == nil || targetModuleID.String() == "" {
		return false
	}
	roles := []domain.ModuleOwnerRole{domain.ModuleOwnerRoleOwner}
	if svc.policyConfig.AllowMaintainerApproval {
		roles = append(roles, domain.ModuleOwnerRoleMaintainer)
	}
	for _, subjectType := range []domain.GovernanceSubjectType{domain.GovernanceSubjectTypeUser, domain.GovernanceSubjectTypeTeam} {
		ok, err := svc.owners.HasRole(ctx, *targetModuleID, subjectType, actor, roles)
		if err == nil && ok {
			return true
		}
	}
	return false
}

func allRequirementsApprovedAfter(requirements []domain.ApprovalRequirement, decidedRequirementID domain.ApprovalRequirementID) bool {
	for _, requirement := range requirements {
		if requirement.ID == decidedRequirementID {
			continue
		}
		if requirement.Status != domain.ApprovalRequirementStatusApproved && requirement.Status != domain.ApprovalRequirementStatusNotRequired {
			return false
		}
	}
	return true
}

func receivedApprovalsAfter(requirements []domain.ApprovalRequirement, decidedRequirementID domain.ApprovalRequirementID, decidedStatus domain.ApprovalRequirementStatus) int {
	count := 0
	for _, requirement := range requirements {
		status := requirement.Status
		if requirement.ID == decidedRequirementID {
			status = decidedStatus
		}
		if status == domain.ApprovalRequirementStatusApproved {
			count++
		}
	}
	return count
}

func requirementReason(plan GovernanceRequirementPlan) string {
	if strings.TrimSpace(plan.ReasonText) != "" {
		if len(plan.Warnings) == 0 {
			return plan.ReasonText
		}
		messages := make([]string, 0, len(plan.Warnings)+1)
		messages = append(messages, plan.ReasonText)
		for _, warning := range plan.Warnings {
			messages = append(messages, warning.Message)
		}
		return strings.Join(messages, " ")
	}
	return plan.Reason.String()
}

func (svc *ApprovalService) appendApprovalRequestCreatedAudit(ctx context.Context, request domain.ApprovalRequest, actor string, policyResult GovernancePolicyResult, occurredAt time.Time) error {
	payload, err := json.Marshal(approvalRequestCreatedAuditPayload{
		PolicyReasons:    policyReasons(policyResult.Reasons),
		RequirementCount: len(request.Requirements),
		ApprovalRequired: policyResult.ApprovalRequired,
	})
	if err != nil {
		return err
	}
	return svc.appendApprovalAudit(ctx, domain.GovernanceAuditEventTypeApprovalRequestCreated, request, strings.TrimSpace(actor), payload, occurredAt)
}

func (svc *ApprovalService) appendDecisionAudit(ctx context.Context, request domain.ApprovalRequest, decision domain.ApprovalDecision, actor string, occurredAt time.Time) error {
	payload, err := json.Marshal(approvalDecisionAuditPayload{
		DecisionID:    decision.ID.String(),
		RequirementID: decision.RequirementID.String(),
		Decision:      decision.Decision.String(),
		DecidedBy:     decision.DecidedBy,
		Comment:       decision.Comment,
	})
	if err != nil {
		return err
	}
	return svc.appendApprovalAudit(ctx, domain.GovernanceAuditEventTypeApprovalDecisionRecorded, request, strings.TrimSpace(actor), payload, occurredAt)
}

func (svc *ApprovalService) appendStatusChangedAudit(ctx context.Context, request domain.ApprovalRequest, status domain.ApprovalRequestStatus, actor string, occurredAt time.Time) error {
	payload, err := json.Marshal(approvalStatusChangedAuditPayload{
		FromStatus: request.Status.String(),
		ToStatus:   status.String(),
	})
	if err != nil {
		return err
	}
	return svc.appendApprovalAudit(ctx, domain.GovernanceAuditEventTypeApprovalRequestStatusChanged, request, strings.TrimSpace(actor), payload, occurredAt)
}

func (svc *ApprovalService) appendApprovalAudit(ctx context.Context, eventType domain.GovernanceAuditEventType, request domain.ApprovalRequest, actor string, payload json.RawMessage, occurredAt time.Time) error {
	auditID, err := svc.ids.NewGovernanceAuditEventID()
	if err != nil {
		return err
	}
	moduleID := request.ModuleID
	requestID := request.ID
	related := []audit.AuditResource{
		{Type: "module", ID: moduleID.String(), Name: request.ModuleName.String()},
	}
	if request.BreakingReportID != nil {
		related = append(related, audit.AuditResource{Type: "breaking_report", ID: request.BreakingReportID.String()})
	}
	return svc.audit.Record(ctx, audit.AuditEvent{
		ID:               auditID.String(),
		Type:             eventType.String(),
		Actor:            actor,
		Action:           eventType.String(),
		Outcome:          audit.OutcomeSuccess,
		Resource:         audit.AuditResource{Type: "approval_request", ID: requestID.String()},
		RelatedResources: related,
		Payload:          payload,
		OccurredAt:       occurredAt,
	})
}

func policyReasons(reasons []GovernancePolicyReason) []string {
	values := make([]string, 0, len(reasons))
	for _, reason := range reasons {
		values = append(values, reason.String())
	}
	return values
}

func mapDecisionValidationError(err error) error {
	switch {
	case errors.Is(err, domain.ErrInvalidApprovalDecision):
		return ErrApprovalRequirementConflict
	case errors.Is(err, domain.ErrInvalidApprovalActor):
		return ErrInvalidApprovalActor
	case errors.Is(err, domain.ErrApprovalDecisionCommentTooLong):
		return ErrApprovalDecisionCommentTooLong
	default:
		return err
	}
}

type approvalRequestCreatedAuditPayload struct {
	PolicyReasons    []string `json:"policy_reasons"`
	RequirementCount int      `json:"requirement_count"`
	ApprovalRequired bool     `json:"approval_required"`
}

type approvalDecisionAuditPayload struct {
	DecisionID    string `json:"decision_id"`
	RequirementID string `json:"requirement_id"`
	Decision      string `json:"decision"`
	DecidedBy     string `json:"decided_by"`
	Comment       string `json:"comment,omitempty"`
}

type approvalStatusChangedAuditPayload struct {
	FromStatus string `json:"from_status"`
	ToStatus   string `json:"to_status"`
}
