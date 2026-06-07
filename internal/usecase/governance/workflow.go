package governance

import (
	"context"

	"github.com/alryzden/ProtoRadar/internal/domain"
)

type ApprovalWorkflow interface {
	CreateApprovalRequestForBreakingReport(ctx context.Context, input CreateApprovalRequestForBreakingReportInput) (domain.ApprovalRequest, error)
	GetApprovalStatusForBreakingReport(ctx context.Context, input GetApprovalStatusForBreakingReportInput) (domain.ApprovalRequest, error)
	ApproveRequirement(ctx context.Context, input ApprovalDecisionInput) (domain.ApprovalRequest, error)
	RejectRequirement(ctx context.Context, input ApprovalDecisionInput) (domain.ApprovalRequest, error)
}

type ApprovalAuditReader interface {
	ListApprovalRequestAudit(ctx context.Context, input ListApprovalRequestAuditInput) ([]domain.GovernanceAuditEvent, error)
}

type DefaultApprovalWorkflow struct {
	approvals *ApprovalService
}

func NewDefaultApprovalWorkflow(approvals *ApprovalService) *DefaultApprovalWorkflow {
	return &DefaultApprovalWorkflow{approvals: approvals}
}

func (workflow *DefaultApprovalWorkflow) CreateApprovalRequestForBreakingReport(ctx context.Context, input CreateApprovalRequestForBreakingReportInput) (domain.ApprovalRequest, error) {
	return workflow.approvals.CreateApprovalRequestForBreakingReport(ctx, input)
}

func (workflow *DefaultApprovalWorkflow) GetApprovalStatusForBreakingReport(ctx context.Context, input GetApprovalStatusForBreakingReportInput) (domain.ApprovalRequest, error) {
	return workflow.approvals.GetApprovalStatusForBreakingReport(ctx, input)
}

func (workflow *DefaultApprovalWorkflow) ApproveRequirement(ctx context.Context, input ApprovalDecisionInput) (domain.ApprovalRequest, error) {
	return workflow.approvals.ApproveRequirement(ctx, input)
}

func (workflow *DefaultApprovalWorkflow) RejectRequirement(ctx context.Context, input ApprovalDecisionInput) (domain.ApprovalRequest, error) {
	return workflow.approvals.RejectRequirement(ctx, input)
}

func (workflow *DefaultApprovalWorkflow) ListApprovalRequestAudit(ctx context.Context, input ListApprovalRequestAuditInput) ([]domain.GovernanceAuditEvent, error) {
	return workflow.approvals.ListApprovalRequestAudit(ctx, input)
}
