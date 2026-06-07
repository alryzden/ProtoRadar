package governance

import "github.com/alryzden/ProtoRadar/internal/domain"

type GovernancePolicyReason string

const (
	GovernancePolicyReasonGovernanceDisabled        GovernancePolicyReason = "governance_disabled"
	GovernancePolicyReasonNoBreakingChanges         GovernancePolicyReason = "no_breaking_changes"
	GovernancePolicyReasonBreakingChanges           GovernancePolicyReason = "breaking_changes"
	GovernancePolicyReasonModuleOwnerRequired       GovernancePolicyReason = "module_owner_required"
	GovernancePolicyReasonAffectedConsumerRequired  GovernancePolicyReason = "affected_consumer_required"
	GovernancePolicyReasonMissingModuleOwner        GovernancePolicyReason = "missing_module_owner"
	GovernancePolicyReasonProductionRuntimeConsumer GovernancePolicyReason = "production_runtime_consumer"
)

func (reason GovernancePolicyReason) String() string {
	return string(reason)
}

type GovernancePolicyResult struct {
	ApprovalRequired  bool
	Status            domain.ApprovalRequestStatus
	RequiredApprovals int
	Requirements      []GovernanceRequirementPlan
	Reasons           []GovernancePolicyReason
	ReasonText        string
	Warnings          []GovernancePolicyWarning
}

type GovernanceRequirementPlan struct {
	RequirementType  domain.ApprovalRequirementType
	TargetModuleID   *domain.ModuleID
	TargetModuleName domain.ModuleName
	RequiredRole     domain.ModuleOwnerRole
	AllowedRoles     []domain.ModuleOwnerRole
	Status           domain.ApprovalRequirementStatus
	Reason           GovernancePolicyReason
	ReasonText       string
	Warnings         []GovernancePolicyWarning
}

type GovernancePolicyWarning struct {
	RequirementType  domain.ApprovalRequirementType
	TargetModuleID   *domain.ModuleID
	TargetModuleName domain.ModuleName
	Reason           GovernancePolicyReason
	Message          string
}
