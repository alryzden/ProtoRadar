package httptransport

import (
	"net/http"

	"github.com/alryzden/ProtoRadar/internal/domain"
	"github.com/alryzden/ProtoRadar/internal/identity"
	"github.com/alryzden/ProtoRadar/internal/usecase/governance"
)

const defaultGovernanceListLimit = 100

func (server *Server) listModuleOwners(w http.ResponseWriter, r *http.Request) {
	if server.governance == nil {
		writeInternalError(w)
		return
	}
	moduleName := r.PathValue("module")
	owners, err := server.governance.ListModuleOwners(r.Context(), governance.ListModuleOwnersInput{ModuleName: moduleName})
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, moduleOwnersResponse(moduleName, owners))
}

func (server *Server) addModuleOwner(w http.ResponseWriter, r *http.Request) {
	if server.governance == nil {
		writeInternalError(w)
		return
	}
	server.limitRequestBody(w, r)
	var req addModuleOwnerRequest
	if err := decodeJSON(r, &req); err != nil {
		if isRequestBodyTooLarge(err) {
			writePayloadTooLarge(w)
			return
		}
		writeError(w, http.StatusBadRequest, "bad_request", "Request body is invalid.")
		return
	}
	actor, err := server.resolveGovernanceActor(r, req.Actor)
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	owner, err := server.governance.AddModuleOwner(r.Context(), governance.AddModuleOwnerInput{
		ModuleName:  r.PathValue("module"),
		SubjectType: req.SubjectType,
		Subject:     req.Subject,
		Role:        req.Role,
		Actor:       actor,
	})
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, moduleOwnerResponse(owner))
}

func (server *Server) removeModuleOwner(w http.ResponseWriter, r *http.Request) {
	if server.governance == nil {
		writeInternalError(w)
		return
	}
	actor, err := server.resolveGovernanceActor(r, r.URL.Query().Get("actor"))
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	if err := server.governance.RemoveModuleOwner(r.Context(), governance.RemoveModuleOwnerInput{
		ModuleName: r.PathValue("module"),
		OwnerID:    r.PathValue("owner_id"),
		Actor:      actor,
	}); err != nil {
		writeUsecaseError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (server *Server) createApprovalRequest(w http.ResponseWriter, r *http.Request) {
	if server.governance == nil {
		writeInternalError(w)
		return
	}
	server.limitRequestBody(w, r)
	var req createApprovalRequestRequest
	if r.Body != nil && r.ContentLength != 0 {
		if err := decodeJSON(r, &req); err != nil {
			if isRequestBodyTooLarge(err) {
				writePayloadTooLarge(w)
				return
			}
			writeError(w, http.StatusBadRequest, "bad_request", "Request body is invalid.")
			return
		}
	}
	actor, err := server.resolveGovernanceActor(r, req.Actor)
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	request, err := server.governance.CreateApprovalRequestForBreakingReport(r.Context(), governance.CreateApprovalRequestForBreakingReportInput{
		BreakingReportID: r.PathValue("report_id"),
		Actor:            actor,
		TargetRef:        req.TargetRef,
	})
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, approvalRequestResponse(request))
}

func (server *Server) getApprovalStatus(w http.ResponseWriter, r *http.Request) {
	if server.governance == nil {
		writeInternalError(w)
		return
	}
	request, err := server.governance.GetApprovalStatusForBreakingReport(r.Context(), governance.GetApprovalStatusForBreakingReportInput{
		BreakingReportID: r.PathValue("report_id"),
	})
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, approvalRequestResponse(request))
}

func (server *Server) approveRequirement(w http.ResponseWriter, r *http.Request) {
	server.recordApprovalDecision(w, r, true)
}

func (server *Server) rejectRequirement(w http.ResponseWriter, r *http.Request) {
	server.recordApprovalDecision(w, r, false)
}

func (server *Server) recordApprovalDecision(w http.ResponseWriter, r *http.Request, approve bool) {
	if server.governance == nil {
		writeInternalError(w)
		return
	}
	server.limitRequestBody(w, r)
	var req approvalDecisionRequest
	if err := decodeJSON(r, &req); err != nil {
		if isRequestBodyTooLarge(err) {
			writePayloadTooLarge(w)
			return
		}
		writeError(w, http.StatusBadRequest, "bad_request", "Request body is invalid.")
		return
	}
	actor, err := server.resolveGovernanceActor(r, req.Actor)
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	input := governance.ApprovalDecisionInput{
		RequestID:     r.PathValue("request_id"),
		RequirementID: r.PathValue("requirement_id"),
		Actor:         actor,
		Comment:       req.Comment,
	}
	var (
		request     domain.ApprovalRequest
		decisionErr error
	)
	if approve {
		request, decisionErr = server.governance.ApproveRequirement(r.Context(), input)
	} else {
		request, decisionErr = server.governance.RejectRequirement(r.Context(), input)
	}
	if decisionErr != nil {
		writeUsecaseError(w, decisionErr)
		return
	}
	writeJSON(w, http.StatusOK, approvalRequestResponse(request))
}

func (server *Server) getApprovalRequestAudit(w http.ResponseWriter, r *http.Request) {
	if server.governance == nil {
		writeInternalError(w)
		return
	}
	requestID := r.PathValue("request_id")
	events, err := server.governance.ListApprovalRequestAudit(r.Context(), governance.ListApprovalRequestAuditInput{
		RequestID: requestID,
		Limit:     parsePositiveInt(r.URL.Query().Get("limit"), defaultGovernanceListLimit),
		Offset:    parsePositiveInt(r.URL.Query().Get("offset"), 0),
	})
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, approvalAuditResponseForRequest(requestID, events))
}

func (server *Server) resolveGovernanceActor(r *http.Request, requestedActor string) (string, error) {
	principal, ok := identity.PrincipalFromContext(r.Context())
	if !ok {
		return "", governance.ErrInvalidApprovalActor
	}
	actor, err := governance.ResolveGovernanceActor(principal, requestedActor, server.actorOverridePolicy)
	if err != nil {
		return "", err
	}
	return actor.Subject, nil
}
