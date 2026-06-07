package governance

import (
	"strings"

	"github.com/alryzden/ProtoRadar/internal/domain"
)

const (
	policyReasonNoBreakingChanges        = "No breaking changes detected."
	policyReasonGovernanceDisabled       = "Governance is disabled."
	policyReasonModuleOwnerApproval      = "Breaking changes require approval from module owner."
	policyReasonAffectedConsumerApproval = "Production usage requires approval from affected consumer module owner."
	policyWarningMissingOwners           = "No owners are configured for this module."
)

type PolicyConfig struct {
	Enabled                 bool
	ProductionEnvironments  []string
	AllowMaintainerApproval bool
}

type PolicyEvaluator interface {
	Evaluate(input EvaluatePolicyInput) GovernancePolicyResult
}

type DefaultPolicyEvaluator struct{}

func (DefaultPolicyEvaluator) Evaluate(input EvaluatePolicyInput) GovernancePolicyResult {
	return EvaluatePolicy(input)
}

func DefaultPolicyConfig() PolicyConfig {
	return PolicyConfig{
		Enabled:                 true,
		ProductionEnvironments:  []string{"production", "prod"},
		AllowMaintainerApproval: true,
	}
}

type EvaluatePolicyInput struct {
	Report          domain.BreakingReport
	ModuleOwners    []domain.ModuleOwner
	AffectedModules []PolicyAffectedModule
	RuntimeImpact   []domain.RuntimeImpact
	Config          PolicyConfig
}

type PolicyAffectedModule struct {
	ModuleID       *domain.ModuleID
	ModuleName     domain.ModuleName
	Owners         []domain.ModuleOwner
	ProductionUsed bool
}

func EvaluatePolicy(input EvaluatePolicyInput) GovernancePolicyResult {
	config := normalizePolicyConfig(input.Config)
	if !config.Enabled {
		return GovernancePolicyResult{
			Status:     domain.ApprovalRequestStatusNotRequired,
			Reasons:    []GovernancePolicyReason{GovernancePolicyReasonGovernanceDisabled},
			ReasonText: policyReasonGovernanceDisabled,
		}
	}

	if input.Report.Status != domain.BreakingReportStatusBreaking {
		return GovernancePolicyResult{
			Status:     domain.ApprovalRequestStatusNotRequired,
			Reasons:    []GovernancePolicyReason{GovernancePolicyReasonNoBreakingChanges},
			ReasonText: policyReasonNoBreakingChanges,
		}
	}

	result := GovernancePolicyResult{
		ApprovalRequired: true,
		Status:           domain.ApprovalRequestStatusPending,
		ReasonText:       policyReasonModuleOwnerApproval,
		Reasons: []GovernancePolicyReason{
			GovernancePolicyReasonBreakingChanges,
			GovernancePolicyReasonModuleOwnerRequired,
		},
	}
	seen := make(map[string]struct{})
	changedModuleID := input.Report.ModuleID
	result.addRequirement(
		seen,
		GovernanceRequirementPlan{
			RequirementType:  domain.ApprovalRequirementTypeModuleOwnerApproval,
			TargetModuleID:   &changedModuleID,
			TargetModuleName: input.Report.ModuleName,
			RequiredRole:     domain.ModuleOwnerRoleOwner,
			AllowedRoles:     allowedPolicyRoles(config),
			Status:           domain.ApprovalRequirementStatusPending,
			Reason:           GovernancePolicyReasonModuleOwnerRequired,
			ReasonText:       policyReasonModuleOwnerApproval,
		},
		input.ModuleOwners,
		config,
	)

	productionModules := productionRuntimeModules(input.RuntimeImpact, config.ProductionEnvironments)
	for _, affected := range input.AffectedModules {
		if !affected.ProductionUsed {
			_, affected.ProductionUsed = productionModules[affected.ModuleName.String()]
		}
		if !affected.ProductionUsed {
			continue
		}

		result.addReason(GovernancePolicyReasonAffectedConsumerRequired)
		result.addReason(GovernancePolicyReasonProductionRuntimeConsumer)
		result.addRequirement(
			seen,
			GovernanceRequirementPlan{
				RequirementType:  domain.ApprovalRequirementTypeAffectedConsumerApproval,
				TargetModuleID:   affected.ModuleID,
				TargetModuleName: affected.ModuleName,
				RequiredRole:     domain.ModuleOwnerRoleOwner,
				AllowedRoles:     allowedPolicyRoles(config),
				Status:           domain.ApprovalRequirementStatusPending,
				Reason:           GovernancePolicyReasonProductionRuntimeConsumer,
				ReasonText:       policyReasonAffectedConsumerApproval,
			},
			affected.Owners,
			config,
		)
	}
	result.RequiredApprovals = len(result.Requirements)
	return result
}

func (result *GovernancePolicyResult) addRequirement(seen map[string]struct{}, requirement GovernanceRequirementPlan, owners []domain.ModuleOwner, config PolicyConfig) {
	key := requirementKey(requirement)
	if _, ok := seen[key]; ok {
		return
	}
	seen[key] = struct{}{}
	if !hasEligibleOwner(owners, config) {
		warning := GovernancePolicyWarning{
			RequirementType:  requirement.RequirementType,
			TargetModuleID:   requirement.TargetModuleID,
			TargetModuleName: requirement.TargetModuleName,
			Reason:           GovernancePolicyReasonMissingModuleOwner,
			Message:          policyWarningMissingOwners,
		}
		requirement.Warnings = append(requirement.Warnings, warning)
		result.Warnings = append(result.Warnings, warning)
		result.addReason(GovernancePolicyReasonMissingModuleOwner)
	}
	result.Requirements = append(result.Requirements, requirement)
}

func (result *GovernancePolicyResult) addReason(reason GovernancePolicyReason) {
	for _, existing := range result.Reasons {
		if existing == reason {
			return
		}
	}
	result.Reasons = append(result.Reasons, reason)
}

func requirementKey(requirement GovernanceRequirementPlan) string {
	target := requirement.TargetModuleName.String()
	if requirement.TargetModuleID != nil && requirement.TargetModuleID.String() != "" {
		target = requirement.TargetModuleID.String()
	}
	return requirement.RequirementType.String() + ":" + target
}

func hasEligibleOwner(owners []domain.ModuleOwner, config PolicyConfig) bool {
	for _, owner := range owners {
		if owner.Role == domain.ModuleOwnerRoleOwner {
			return true
		}
		if config.AllowMaintainerApproval && owner.Role == domain.ModuleOwnerRoleMaintainer {
			return true
		}
	}
	return false
}

func allowedPolicyRoles(config PolicyConfig) []domain.ModuleOwnerRole {
	roles := []domain.ModuleOwnerRole{domain.ModuleOwnerRoleOwner}
	if config.AllowMaintainerApproval {
		roles = append(roles, domain.ModuleOwnerRoleMaintainer)
	}
	return roles
}

func productionRuntimeModules(impact []domain.RuntimeImpact, productionEnvironments []string) map[string]struct{} {
	production := make(map[string]struct{}, len(productionEnvironments))
	for _, environment := range productionEnvironments {
		trimmed := strings.ToLower(strings.TrimSpace(environment))
		if trimmed != "" {
			production[trimmed] = struct{}{}
		}
	}
	modules := make(map[string]struct{})
	for _, item := range impact {
		if _, ok := production[strings.ToLower(strings.TrimSpace(item.Environment.String()))]; !ok {
			continue
		}
		modules[item.UsedModule.String()] = struct{}{}
	}
	return modules
}

func normalizePolicyConfig(config PolicyConfig) PolicyConfig {
	if len(config.ProductionEnvironments) == 0 {
		config.ProductionEnvironments = DefaultPolicyConfig().ProductionEnvironments
	}
	return config
}
