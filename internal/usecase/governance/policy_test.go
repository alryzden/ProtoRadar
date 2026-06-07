package governance

import (
	"slices"
	"testing"

	"github.com/alryzden/ProtoRadar/internal/domain"
)

func TestEvaluatePolicyPassedReportDoesNotRequireApproval(t *testing.T) {
	result := EvaluatePolicy(EvaluatePolicyInput{
		Report: breakingReport(domain.BreakingReportStatusPassed),
		AffectedModules: []PolicyAffectedModule{
			{ModuleID: moduleIDPtr("module-2"), ModuleName: mustModuleName(t, "billing-api"), ProductionUsed: true},
		},
		Config: DefaultPolicyConfig(),
	})

	if result.ApprovalRequired {
		t.Fatalf("approval required should be false")
	}
	if result.Status != domain.ApprovalRequestStatusNotRequired {
		t.Fatalf("status = %q", result.Status)
	}
	if result.ReasonText != policyReasonNoBreakingChanges {
		t.Fatalf("reason text = %q", result.ReasonText)
	}
	if len(result.Requirements) != 0 {
		t.Fatalf("requirements = %d, want 0", len(result.Requirements))
	}
}

func TestEvaluatePolicyBreakingReportRequiresModuleOwnerApproval(t *testing.T) {
	result := EvaluatePolicy(EvaluatePolicyInput{
		Report:       breakingReport(domain.BreakingReportStatusBreaking),
		ModuleOwners: []domain.ModuleOwner{owner("module-1", "user-api", domain.ModuleOwnerRoleOwner)},
		Config:       DefaultPolicyConfig(),
	})

	if !result.ApprovalRequired {
		t.Fatalf("approval required should be true")
	}
	if result.Status != domain.ApprovalRequestStatusPending {
		t.Fatalf("status = %q", result.Status)
	}
	if result.RequiredApprovals != 1 || len(result.Requirements) != 1 {
		t.Fatalf("required approvals = %d requirements = %d", result.RequiredApprovals, len(result.Requirements))
	}
	requirement := result.Requirements[0]
	if requirement.RequirementType != domain.ApprovalRequirementTypeModuleOwnerApproval {
		t.Fatalf("requirement type = %q", requirement.RequirementType)
	}
	if requirement.TargetModuleName.String() != "user-api" || requirement.RequiredRole != domain.ModuleOwnerRoleOwner {
		t.Fatalf("requirement = %#v", requirement)
	}
	if requirement.ReasonText != policyReasonModuleOwnerApproval {
		t.Fatalf("reason text = %q", requirement.ReasonText)
	}
	if len(requirement.Warnings) != 0 {
		t.Fatalf("warnings = %#v", requirement.Warnings)
	}
}

func TestEvaluatePolicyBreakingReportWithOwnerStillCreatesRequirement(t *testing.T) {
	result := EvaluatePolicy(EvaluatePolicyInput{
		Report:       breakingReport(domain.BreakingReportStatusBreaking),
		ModuleOwners: []domain.ModuleOwner{owner("module-1", "user-api", domain.ModuleOwnerRoleOwner)},
		Config:       DefaultPolicyConfig(),
	})

	if len(result.Requirements) != 1 {
		t.Fatalf("requirements = %d, want 1", len(result.Requirements))
	}
	if result.Requirements[0].Status != domain.ApprovalRequirementStatusPending {
		t.Fatalf("requirement status = %q", result.Requirements[0].Status)
	}
}

func TestEvaluatePolicyMissingOwnersCreatesPendingRequirementWithWarning(t *testing.T) {
	result := EvaluatePolicy(EvaluatePolicyInput{
		Report: breakingReport(domain.BreakingReportStatusBreaking),
		Config: DefaultPolicyConfig(),
	})

	if len(result.Requirements) != 1 {
		t.Fatalf("requirements = %d, want 1", len(result.Requirements))
	}
	requirement := result.Requirements[0]
	if requirement.Status != domain.ApprovalRequirementStatusPending {
		t.Fatalf("requirement status = %q", requirement.Status)
	}
	if len(requirement.Warnings) != 1 {
		t.Fatalf("requirement warnings = %#v", requirement.Warnings)
	}
	if requirement.Warnings[0].Message != policyWarningMissingOwners {
		t.Fatalf("warning = %#v", requirement.Warnings[0])
	}
	if !slices.Contains(result.Reasons, GovernancePolicyReasonMissingModuleOwner) {
		t.Fatalf("reasons = %#v", result.Reasons)
	}
}

func TestEvaluatePolicyProductionRuntimeImpactCreatesAffectedConsumerRequirement(t *testing.T) {
	result := EvaluatePolicy(EvaluatePolicyInput{
		Report:       breakingReport(domain.BreakingReportStatusBreaking),
		ModuleOwners: []domain.ModuleOwner{owner("module-1", "user-api", domain.ModuleOwnerRoleOwner)},
		AffectedModules: []PolicyAffectedModule{
			{
				ModuleID:   moduleIDPtr("module-2"),
				ModuleName: mustModuleName(t, "billing-api"),
				Owners:     []domain.ModuleOwner{owner("module-2", "billing-api", domain.ModuleOwnerRoleOwner)},
			},
		},
		RuntimeImpact: []domain.RuntimeImpact{runtimeImpact(t, "production", "billing-api")},
		Config:        DefaultPolicyConfig(),
	})

	if result.RequiredApprovals != 2 {
		t.Fatalf("required approvals = %d, want 2", result.RequiredApprovals)
	}
	affected := findRequirement(result.Requirements, domain.ApprovalRequirementTypeAffectedConsumerApproval, "billing-api")
	if affected == nil {
		t.Fatalf("affected consumer requirement not found: %#v", result.Requirements)
	}
	if affected.ReasonText != policyReasonAffectedConsumerApproval {
		t.Fatalf("reason text = %q", affected.ReasonText)
	}
}

func TestEvaluatePolicyNonProductionRuntimeImpactDoesNotCreateConsumerRequirement(t *testing.T) {
	result := EvaluatePolicy(EvaluatePolicyInput{
		Report:       breakingReport(domain.BreakingReportStatusBreaking),
		ModuleOwners: []domain.ModuleOwner{owner("module-1", "user-api", domain.ModuleOwnerRoleOwner)},
		AffectedModules: []PolicyAffectedModule{
			{ModuleID: moduleIDPtr("module-2"), ModuleName: mustModuleName(t, "billing-api")},
		},
		RuntimeImpact: []domain.RuntimeImpact{runtimeImpact(t, "staging", "billing-api")},
		Config:        DefaultPolicyConfig(),
	})

	if len(result.Requirements) != 1 {
		t.Fatalf("requirements = %d, want 1", len(result.Requirements))
	}
	if findRequirement(result.Requirements, domain.ApprovalRequirementTypeAffectedConsumerApproval, "billing-api") != nil {
		t.Fatalf("unexpected affected consumer requirement: %#v", result.Requirements)
	}
}

func TestEvaluatePolicyProductionEnvironmentAliasesFromConfigWork(t *testing.T) {
	config := DefaultPolicyConfig()
	config.ProductionEnvironments = []string{"live", "prod"}
	result := EvaluatePolicy(EvaluatePolicyInput{
		Report:       breakingReport(domain.BreakingReportStatusBreaking),
		ModuleOwners: []domain.ModuleOwner{owner("module-1", "user-api", domain.ModuleOwnerRoleOwner)},
		AffectedModules: []PolicyAffectedModule{
			{ModuleID: moduleIDPtr("module-2"), ModuleName: mustModuleName(t, "billing-api"), Owners: []domain.ModuleOwner{owner("module-2", "billing-api", domain.ModuleOwnerRoleOwner)}},
		},
		RuntimeImpact: []domain.RuntimeImpact{runtimeImpact(t, "live", "billing-api")},
		Config:        config,
	})

	if findRequirement(result.Requirements, domain.ApprovalRequirementTypeAffectedConsumerApproval, "billing-api") == nil {
		t.Fatalf("affected consumer requirement not found: %#v", result.Requirements)
	}
}

func TestEvaluatePolicyDeduplicatesAffectedModuleRequirements(t *testing.T) {
	result := EvaluatePolicy(EvaluatePolicyInput{
		Report:       breakingReport(domain.BreakingReportStatusBreaking),
		ModuleOwners: []domain.ModuleOwner{owner("module-1", "user-api", domain.ModuleOwnerRoleOwner)},
		AffectedModules: []PolicyAffectedModule{
			{ModuleID: moduleIDPtr("module-2"), ModuleName: mustModuleName(t, "billing-api"), ProductionUsed: true},
			{ModuleID: moduleIDPtr("module-2"), ModuleName: mustModuleName(t, "billing-api"), ProductionUsed: true},
		},
		Config: DefaultPolicyConfig(),
	})

	count := 0
	for _, requirement := range result.Requirements {
		if requirement.RequirementType == domain.ApprovalRequirementTypeAffectedConsumerApproval {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("affected consumer requirements = %d, want 1: %#v", count, result.Requirements)
	}
}

func TestEvaluatePolicyDisabledGovernanceDoesNotRequireApproval(t *testing.T) {
	config := DefaultPolicyConfig()
	config.Enabled = false
	result := EvaluatePolicy(EvaluatePolicyInput{
		Report: breakingReport(domain.BreakingReportStatusBreaking),
		Config: config,
	})

	if result.ApprovalRequired {
		t.Fatalf("approval required should be false")
	}
	if result.Status != domain.ApprovalRequestStatusNotRequired {
		t.Fatalf("status = %q", result.Status)
	}
	if result.ReasonText != policyReasonGovernanceDisabled {
		t.Fatalf("reason text = %q", result.ReasonText)
	}
}

func TestEvaluatePolicyAllowMaintainerApprovalIsReflectedInAllowedRoles(t *testing.T) {
	config := DefaultPolicyConfig()
	result := EvaluatePolicy(EvaluatePolicyInput{
		Report:       breakingReport(domain.BreakingReportStatusBreaking),
		ModuleOwners: []domain.ModuleOwner{owner("module-1", "user-api", domain.ModuleOwnerRoleMaintainer)},
		Config:       config,
	})

	requirement := result.Requirements[0]
	if !slices.Contains(requirement.AllowedRoles, domain.ModuleOwnerRoleMaintainer) {
		t.Fatalf("allowed roles = %#v", requirement.AllowedRoles)
	}
	if len(requirement.Warnings) != 0 {
		t.Fatalf("maintainer should satisfy missing-owner check when allowed: %#v", requirement.Warnings)
	}

	config.AllowMaintainerApproval = false
	result = EvaluatePolicy(EvaluatePolicyInput{
		Report:       breakingReport(domain.BreakingReportStatusBreaking),
		ModuleOwners: []domain.ModuleOwner{owner("module-1", "user-api", domain.ModuleOwnerRoleMaintainer)},
		Config:       config,
	})
	requirement = result.Requirements[0]
	if slices.Contains(requirement.AllowedRoles, domain.ModuleOwnerRoleMaintainer) {
		t.Fatalf("allowed roles = %#v", requirement.AllowedRoles)
	}
	if len(requirement.Warnings) != 1 {
		t.Fatalf("maintainer should not satisfy missing-owner check when disallowed: %#v", requirement.Warnings)
	}
}

func breakingReport(status domain.BreakingReportStatus) domain.BreakingReport {
	moduleName, _ := domain.NewModuleName("user-api")
	version, _ := domain.NewVersion("v1.0.0")
	return domain.BreakingReport{
		ID:          domain.NewBreakingReportID("report-1"),
		ModuleID:    domain.NewModuleID("module-1"),
		ModuleName:  moduleName,
		BaseVersion: version,
		TargetRef:   "feature/governance",
		Status:      status,
		ChangeCount: 1,
	}
}

func owner(moduleID string, moduleName string, role domain.ModuleOwnerRole) domain.ModuleOwner {
	name, _ := domain.NewModuleName(moduleName)
	return domain.ModuleOwner{
		ID:          domain.NewModuleOwnerID(moduleID + "-" + role.String()),
		ModuleID:    domain.NewModuleID(moduleID),
		ModuleName:  name,
		SubjectType: domain.GovernanceSubjectTypeUser,
		Subject:     "alice",
		Role:        role,
	}
}

func runtimeImpact(t *testing.T, environment string, moduleName string) domain.RuntimeImpact {
	t.Helper()
	env, err := domain.NewRuntimeEnvironment(environment)
	if err != nil {
		t.Fatalf("runtime environment: %v", err)
	}
	name, err := domain.NewModuleName(moduleName)
	if err != nil {
		t.Fatalf("module name: %v", err)
	}
	return domain.RuntimeImpact{
		Environment: env,
		UsedModule:  name,
	}
}

func mustModuleName(t *testing.T, value string) domain.ModuleName {
	t.Helper()
	name, err := domain.NewModuleName(value)
	if err != nil {
		t.Fatalf("module name: %v", err)
	}
	return name
}

func moduleIDPtr(value string) *domain.ModuleID {
	id := domain.NewModuleID(value)
	return &id
}

func findRequirement(requirements []GovernanceRequirementPlan, requirementType domain.ApprovalRequirementType, moduleName string) *GovernanceRequirementPlan {
	for i := range requirements {
		if requirements[i].RequirementType == requirementType && requirements[i].TargetModuleName.String() == moduleName {
			return &requirements[i]
		}
	}
	return nil
}
