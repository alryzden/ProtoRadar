package web

import (
	"errors"
	"net/http"
	"strings"

	"github.com/alryzden/ProtoRadar/internal/domain"
	"github.com/alryzden/ProtoRadar/internal/usecase/uiquery"
)

func (server *Server) approvalRequestDetail(w http.ResponseWriter, r *http.Request) {
	if server.query == nil {
		server.RenderError(w, http.StatusInternalServerError, "")
		return
	}
	requestID := strings.TrimSpace(r.PathValue("request_id"))
	details, err := server.query.GetApprovalRequestDetails(r.Context(), uiquery.GetApprovalRequestDetailsInput{RequestID: requestID})
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			server.RenderError(w, http.StatusNotFound, "Approval request not found.")
			return
		}
		server.RenderError(w, http.StatusInternalServerError, "")
		return
	}
	view := server.approvalRequestDetailView(details)
	server.render(w, http.StatusOK, "approval_request_detail.html", pageData{
		Title:               "Approval Request " + view.Request.ID,
		Active:              "breaking-reports",
		Status:              view.Request.Status,
		Message:             "Inspect governance requirements, decisions, and audit trail.",
		ApprovalRequestView: &view,
	})
}
func (server *Server) approvalRequestDetailView(details uiquery.ApprovalRequestDetails) approvalRequestDetailView {
	view := approvalRequestDetailView{
		Request: server.approvalRequestView(details.Request),
	}
	for _, event := range details.AuditEvents {
		view.AuditTrail = append(view.AuditTrail, governanceAuditEventRow{
			ID:          event.ID,
			EventType:   event.EventType,
			Actor:       event.Actor,
			Module:      event.ModuleName,
			PayloadJSON: event.PayloadJSON,
			CreatedAt:   formatTime(event.CreatedAt),
		})
	}
	view.HasAudit = len(view.AuditTrail) > 0
	return view
}

func (server *Server) approvalRequestView(request uiquery.ApprovalRequestSummary) approvalRequestView {
	view := approvalRequestView{
		ID:                request.ID,
		Module:            request.ModuleName,
		ModuleURL:         server.basePath + "/modules/" + pathEscape(request.ModuleName),
		BreakingReportID:  request.BreakingReportID,
		TargetRef:         request.TargetRef,
		Status:            request.Status,
		RequiredApprovals: request.RequiredApprovals,
		ReceivedApprovals: request.ReceivedApprovals,
		CreatedAt:         formatTime(request.CreatedAt),
		UpdatedAt:         formatTime(request.UpdatedAt),
		RequestURL:        server.basePath + "/approval-requests/" + pathEscape(request.ID),
	}
	if strings.TrimSpace(request.BreakingReportID) != "" {
		view.BreakingReportURL = server.basePath + "/breaking-reports/" + pathEscape(request.BreakingReportID)
	}
	for _, requirement := range request.Requirements {
		row := approvalRequirementRow{
			ID:               requirement.ID,
			RequirementType:  requirement.RequirementType,
			TargetModuleName: requirement.TargetModuleName,
			TargetModuleURL:  server.basePath + "/modules/" + pathEscape(requirement.TargetModuleName),
			RequiredRole:     requirement.RequiredRole,
			Status:           requirement.Status,
			Reason:           requirement.Reason,
		}
		view.Requirements = append(view.Requirements, row)
		if strings.Contains(strings.ToLower(requirement.Reason), "no owners are configured") {
			target := strings.TrimSpace(requirement.TargetModuleName)
			if target == "" {
				target = "this module"
			}
			view.MissingWarnings = append(view.MissingWarnings, "Approval required, but no owners are configured for "+target+".")
		}
	}
	for _, decision := range request.Decisions {
		view.Decisions = append(view.Decisions, approvalDecisionRow{
			ID:            decision.ID,
			RequirementID: decision.RequirementID,
			Decision:      decision.Decision,
			DecidedBy:     decision.DecidedBy,
			Comment:       decision.Comment,
			CreatedAt:     formatTime(decision.CreatedAt),
		})
	}
	view.HasRequirements = len(view.Requirements) > 0
	view.HasDecisions = len(view.Decisions) > 0
	view.HasMissingWarnings = len(view.MissingWarnings) > 0
	return view
}
