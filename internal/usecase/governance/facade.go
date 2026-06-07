package governance

import (
	"context"

	"github.com/alryzden/ProtoRadar/internal/domain"
)

type APIService struct {
	Owners        *Service
	Approvals     ApprovalWorkflow
	ApprovalAudit ApprovalAuditReader
}

func (svc APIService) AddModuleOwner(ctx context.Context, input AddModuleOwnerInput) (domain.ModuleOwner, error) {
	return svc.Owners.AddModuleOwner(ctx, input)
}

func (svc APIService) RemoveModuleOwner(ctx context.Context, input RemoveModuleOwnerInput) error {
	return svc.Owners.RemoveModuleOwner(ctx, input)
}

func (svc APIService) ListModuleOwners(ctx context.Context, input ListModuleOwnersInput) ([]domain.ModuleOwner, error) {
	return svc.Owners.ListModuleOwners(ctx, input)
}

func (svc APIService) CreateApprovalRequestForBreakingReport(ctx context.Context, input CreateApprovalRequestForBreakingReportInput) (domain.ApprovalRequest, error) {
	return svc.Approvals.CreateApprovalRequestForBreakingReport(ctx, input)
}

func (svc APIService) GetApprovalStatusForBreakingReport(ctx context.Context, input GetApprovalStatusForBreakingReportInput) (domain.ApprovalRequest, error) {
	return svc.Approvals.GetApprovalStatusForBreakingReport(ctx, input)
}

func (svc APIService) ApproveRequirement(ctx context.Context, input ApprovalDecisionInput) (domain.ApprovalRequest, error) {
	return svc.Approvals.ApproveRequirement(ctx, input)
}

func (svc APIService) RejectRequirement(ctx context.Context, input ApprovalDecisionInput) (domain.ApprovalRequest, error) {
	return svc.Approvals.RejectRequirement(ctx, input)
}

func (svc APIService) ListApprovalRequestAudit(ctx context.Context, input ListApprovalRequestAuditInput) ([]domain.GovernanceAuditEvent, error) {
	return svc.ApprovalAudit.ListApprovalRequestAudit(ctx, input)
}
