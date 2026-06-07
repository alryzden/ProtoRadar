package governance

import "errors"

var (
	ErrInvalidModuleName                = errors.New("invalid module name")
	ErrModuleNotFound                   = errors.New("module not found")
	ErrInvalidSubjectType               = errors.New("invalid governance subject type")
	ErrInvalidSubject                   = errors.New("invalid governance subject")
	ErrInvalidModuleOwnerRole           = errors.New("invalid module owner role")
	ErrInvalidModuleOwnerID             = errors.New("invalid module owner id")
	ErrModuleOwnerNotFound              = errors.New("module owner not found")
	ErrModuleOwnerAlreadyExists         = errors.New("module owner already exists")
	ErrInvalidBreakingReportID          = errors.New("invalid breaking report id")
	ErrBreakingReportNotFound           = errors.New("breaking report not found")
	ErrApprovalRequestNotFound          = errors.New("approval request not found")
	ErrInvalidRequirementID             = errors.New("invalid approval requirement id")
	ErrApprovalRequirementNotFound      = errors.New("approval requirement not found")
	ErrInvalidApprovalActor             = errors.New("invalid approval actor")
	ErrApprovalDecisionCommentTooLong   = errors.New("approval decision comment too long")
	ErrApprovalRequestConflict          = errors.New("approval request conflict")
	ErrApprovalRequirementConflict      = errors.New("approval requirement conflict")
	ErrApprovalActorForbidden           = errors.New("approval actor forbidden")
	ErrGovernanceActorOverrideForbidden = errors.New("governance actor override forbidden")
)
