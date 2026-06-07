package governance

import (
	"context"
	"testing"

	"github.com/alryzden/ProtoRadar/internal/domain"
)

func TestAPIServiceUsesApprovalWorkflowBoundary(t *testing.T) {
	requestID := domain.NewApprovalRequestID("request-1")
	workflow := &fakeApprovalWorkflowBoundary{
		request: domain.ApprovalRequest{ID: requestID, Status: domain.ApprovalRequestStatusPending},
	}
	svc := APIService{
		Approvals:     workflow,
		ApprovalAudit: workflow,
	}

	request, err := svc.CreateApprovalRequestForBreakingReport(context.Background(), CreateApprovalRequestForBreakingReportInput{BreakingReportID: "report-1"})
	if err != nil {
		t.Fatalf("create approval request: %v", err)
	}
	if request.ID != requestID || !workflow.createCalled {
		t.Fatalf("workflow create request = %#v called=%v", request, workflow.createCalled)
	}

	events, err := svc.ListApprovalRequestAudit(context.Background(), ListApprovalRequestAuditInput{RequestID: requestID.String()})
	if err != nil {
		t.Fatalf("list approval audit: %v", err)
	}
	if len(events) != 1 || !workflow.auditCalled {
		t.Fatalf("audit events = %#v called=%v", events, workflow.auditCalled)
	}
}

type fakeApprovalWorkflowBoundary struct {
	request      domain.ApprovalRequest
	createCalled bool
	auditCalled  bool
}

func (fake *fakeApprovalWorkflowBoundary) CreateApprovalRequestForBreakingReport(ctx context.Context, input CreateApprovalRequestForBreakingReportInput) (domain.ApprovalRequest, error) {
	fake.createCalled = true
	return fake.request, nil
}

func (fake *fakeApprovalWorkflowBoundary) GetApprovalStatusForBreakingReport(ctx context.Context, input GetApprovalStatusForBreakingReportInput) (domain.ApprovalRequest, error) {
	return fake.request, nil
}

func (fake *fakeApprovalWorkflowBoundary) ApproveRequirement(ctx context.Context, input ApprovalDecisionInput) (domain.ApprovalRequest, error) {
	return fake.request, nil
}

func (fake *fakeApprovalWorkflowBoundary) RejectRequirement(ctx context.Context, input ApprovalDecisionInput) (domain.ApprovalRequest, error) {
	return fake.request, nil
}

func (fake *fakeApprovalWorkflowBoundary) ListApprovalRequestAudit(ctx context.Context, input ListApprovalRequestAuditInput) ([]domain.GovernanceAuditEvent, error) {
	fake.auditCalled = true
	return []domain.GovernanceAuditEvent{{ID: domain.NewGovernanceAuditEventID("audit-1")}}, nil
}
