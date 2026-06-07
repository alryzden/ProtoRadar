package httptransport

import (
	"encoding/json"
	"time"

	"github.com/alryzden/ProtoRadar/internal/domain"
)

type addModuleOwnerRequest struct {
	SubjectType string `json:"subject_type"`
	Subject     string `json:"subject"`
	Role        string `json:"role"`
	Actor       string `json:"actor"`
}

type moduleOwnerDTO struct {
	ID          string    `json:"id"`
	ModuleID    string    `json:"module_id"`
	ModuleName  string    `json:"module_name"`
	SubjectType string    `json:"subject_type"`
	Subject     string    `json:"subject"`
	Role        string    `json:"role"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type listModuleOwnersResponse struct {
	Module string           `json:"module"`
	Owners []moduleOwnerDTO `json:"owners"`
}

type createApprovalRequestRequest struct {
	Actor     string `json:"actor"`
	TargetRef string `json:"target_ref"`
}

type approvalDecisionRequest struct {
	Actor   string `json:"actor"`
	Comment string `json:"comment"`
}

type approvalRequestDTO struct {
	ID                string                   `json:"id"`
	ModuleID          string                   `json:"module_id"`
	ModuleName        string                   `json:"module_name"`
	BreakingReportID  string                   `json:"breaking_report_id,omitempty"`
	TargetRef         string                   `json:"target_ref"`
	Status            string                   `json:"status"`
	RequiredApprovals int                      `json:"required_approvals"`
	ReceivedApprovals int                      `json:"received_approvals"`
	Requirements      []approvalRequirementDTO `json:"requirements"`
	Decisions         []approvalDecisionDTO    `json:"decisions"`
	CreatedAt         time.Time                `json:"created_at"`
	UpdatedAt         time.Time                `json:"updated_at"`
}

type approvalRequirementDTO struct {
	ID                string    `json:"id"`
	ApprovalRequestID string    `json:"approval_request_id"`
	RequirementType   string    `json:"requirement_type"`
	TargetModuleID    string    `json:"target_module_id,omitempty"`
	TargetModuleName  string    `json:"target_module_name"`
	RequiredRole      string    `json:"required_role"`
	Status            string    `json:"status"`
	Reason            string    `json:"reason"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type approvalDecisionDTO struct {
	ID                string    `json:"id"`
	ApprovalRequestID string    `json:"approval_request_id"`
	RequirementID     string    `json:"requirement_id"`
	Decision          string    `json:"decision"`
	DecidedBy         string    `json:"decided_by"`
	Comment           string    `json:"comment,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
}

type approvalAuditEventDTO struct {
	ID                string          `json:"id"`
	EventType         string          `json:"event_type"`
	Actor             string          `json:"actor,omitempty"`
	ModuleID          string          `json:"module_id,omitempty"`
	ModuleName        string          `json:"module_name,omitempty"`
	ApprovalRequestID string          `json:"approval_request_id,omitempty"`
	BreakingReportID  string          `json:"breaking_report_id,omitempty"`
	Payload           json.RawMessage `json:"payload"`
	CreatedAt         time.Time       `json:"created_at"`
}

type approvalAuditResponse struct {
	ApprovalRequestID string                  `json:"approval_request_id"`
	Events            []approvalAuditEventDTO `json:"events"`
}

func moduleOwnerResponse(owner domain.ModuleOwner) moduleOwnerDTO {
	return moduleOwnerDTO{
		ID:          owner.ID.String(),
		ModuleID:    owner.ModuleID.String(),
		ModuleName:  owner.ModuleName.String(),
		SubjectType: owner.SubjectType.String(),
		Subject:     owner.Subject,
		Role:        owner.Role.String(),
		CreatedAt:   owner.CreatedAt,
		UpdatedAt:   owner.UpdatedAt,
	}
}

func moduleOwnersResponse(module string, owners []domain.ModuleOwner) listModuleOwnersResponse {
	items := make([]moduleOwnerDTO, 0, len(owners))
	for _, owner := range owners {
		items = append(items, moduleOwnerResponse(owner))
	}
	return listModuleOwnersResponse{Module: module, Owners: items}
}

func approvalRequestResponse(request domain.ApprovalRequest) approvalRequestDTO {
	requirements := make([]approvalRequirementDTO, 0, len(request.Requirements))
	for _, requirement := range request.Requirements {
		requirements = append(requirements, approvalRequirementResponse(requirement))
	}
	decisions := make([]approvalDecisionDTO, 0, len(request.Decisions))
	for _, decision := range request.Decisions {
		decisions = append(decisions, approvalDecisionResponse(decision))
	}
	breakingReportID := ""
	if request.BreakingReportID != nil {
		breakingReportID = request.BreakingReportID.String()
	}
	return approvalRequestDTO{
		ID:                request.ID.String(),
		ModuleID:          request.ModuleID.String(),
		ModuleName:        request.ModuleName.String(),
		BreakingReportID:  breakingReportID,
		TargetRef:         request.TargetRef,
		Status:            request.Status.String(),
		RequiredApprovals: request.RequiredApprovals,
		ReceivedApprovals: request.ReceivedApprovals,
		Requirements:      requirements,
		Decisions:         decisions,
		CreatedAt:         request.CreatedAt,
		UpdatedAt:         request.UpdatedAt,
	}
}

func approvalRequirementResponse(requirement domain.ApprovalRequirement) approvalRequirementDTO {
	targetModuleID := ""
	if requirement.TargetModuleID != nil {
		targetModuleID = requirement.TargetModuleID.String()
	}
	return approvalRequirementDTO{
		ID:                requirement.ID.String(),
		ApprovalRequestID: requirement.ApprovalRequestID.String(),
		RequirementType:   requirement.RequirementType.String(),
		TargetModuleID:    targetModuleID,
		TargetModuleName:  requirement.TargetModuleName.String(),
		RequiredRole:      requirement.RequiredRole.String(),
		Status:            requirement.Status.String(),
		Reason:            requirement.Reason,
		CreatedAt:         requirement.CreatedAt,
		UpdatedAt:         requirement.UpdatedAt,
	}
}

func approvalDecisionResponse(decision domain.ApprovalDecision) approvalDecisionDTO {
	return approvalDecisionDTO{
		ID:                decision.ID.String(),
		ApprovalRequestID: decision.ApprovalRequestID.String(),
		RequirementID:     decision.RequirementID.String(),
		Decision:          decision.Decision.String(),
		DecidedBy:         decision.DecidedBy,
		Comment:           decision.Comment,
		CreatedAt:         decision.CreatedAt,
	}
}

func approvalAuditResponseForRequest(requestID string, events []domain.GovernanceAuditEvent) approvalAuditResponse {
	items := make([]approvalAuditEventDTO, 0, len(events))
	for _, event := range events {
		items = append(items, approvalAuditEventResponse(event))
	}
	return approvalAuditResponse{ApprovalRequestID: requestID, Events: items}
}

func approvalAuditEventResponse(event domain.GovernanceAuditEvent) approvalAuditEventDTO {
	payload := event.PayloadJSON
	if len(payload) == 0 {
		payload = json.RawMessage(`{}`)
	}
	dto := approvalAuditEventDTO{
		ID:         event.ID.String(),
		EventType:  event.EventType.String(),
		Actor:      event.Actor,
		ModuleName: event.ModuleName.String(),
		Payload:    payload,
		CreatedAt:  event.CreatedAt,
	}
	if event.ModuleID != nil {
		dto.ModuleID = event.ModuleID.String()
	}
	if event.ApprovalRequestID != nil {
		dto.ApprovalRequestID = event.ApprovalRequestID.String()
	}
	if event.BreakingReportID != nil {
		dto.BreakingReportID = event.BreakingReportID.String()
	}
	return dto
}
