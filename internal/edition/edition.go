package edition

import (
	"context"
	"errors"
	"sort"

	"github.com/alryzden/ProtoRadar/internal/version"
)

type Capability string

const (
	CapabilityRegistry                  Capability = "registry"
	CapabilityBufWorkflow               Capability = "buf_workflow"
	CapabilityBreakingChecks            Capability = "breaking_checks"
	CapabilityGitLabCI                  Capability = "gitlab_ci"
	CapabilityGitLabMRBot               Capability = "gitlab_mr_bot"
	CapabilityWebUI                     Capability = "web_ui"
	CapabilityDependencyGraph           Capability = "dependency_graph"
	CapabilityRuntimeInventory          Capability = "runtime_inventory"
	CapabilityCommunityGovernance       Capability = "community_governance"
	CapabilityAPITokenAuth              Capability = "api_token_auth" // #nosec G101 -- capability vocabulary, not a credential.
	CapabilityBasicAudit                Capability = "basic_audit"
	CapabilityPrometheusMetrics         Capability = "prometheus_metrics"
	CapabilityStructuredLogs            Capability = "structured_logs"
	CapabilityDockerComposeQuickstart   Capability = "docker_compose_quickstart"
	CapabilityOIDCAuth                  Capability = "oidc_auth"
	CapabilityLDAPAuth                  Capability = "ldap_auth"
	CapabilityAdvancedRBAC              Capability = "advanced_rbac"
	CapabilityGitLabGroupSync           Capability = "gitlab_group_sync"
	CapabilityAdvancedAudit             Capability = "advanced_audit"
	CapabilityAdvancedApprovalWorkflows Capability = "advanced_approval_workflows"
	CapabilityEnterpriseDependencyGraph Capability = "enterprise_dependency_graph"
	CapabilityRuntimeAlerts             Capability = "runtime_alerts"
	CapabilityHelmHA                    Capability = "helm_ha"
	CapabilityAirGapped                 Capability = "air_gapped"
	CapabilityLicenseManagement         Capability = "license_management"
)

const NameCommunity = "community"

var ErrCapabilityNotAvailable = errors.New("capability_not_available")

func (capability Capability) String() string {
	return string(capability)
}

type CapabilityStatus struct {
	Capability Capability
	Enabled    bool
}

type CapabilityChecker interface {
	IsEnabled(ctx context.Context, capability Capability) bool
	Require(ctx context.Context, capability Capability) error
	List(ctx context.Context) []CapabilityStatus
}

type CommunityCapabilityChecker struct{}

func NewCommunityCapabilityChecker() CommunityCapabilityChecker {
	return CommunityCapabilityChecker{}
}

func (CommunityCapabilityChecker) IsEnabled(ctx context.Context, capability Capability) bool {
	_, ok := communityCapabilitySet()[capability]
	return ok
}

func (checker CommunityCapabilityChecker) Require(ctx context.Context, capability Capability) error {
	if checker.IsEnabled(ctx, capability) {
		return nil
	}
	return ErrCapabilityNotAvailable
}

func (checker CommunityCapabilityChecker) List(ctx context.Context) []CapabilityStatus {
	capabilities := AllCapabilities()
	statuses := make([]CapabilityStatus, 0, len(capabilities))
	for _, capability := range capabilities {
		statuses = append(statuses, CapabilityStatus{
			Capability: capability,
			Enabled:    checker.IsEnabled(ctx, capability),
		})
	}
	return statuses
}

type Edition struct {
	Name         string
	Version      version.BuildInfo
	Capabilities []CapabilityStatus
}

func NewCommunityEdition(ctx context.Context, build version.BuildInfo, checker CapabilityChecker) Edition {
	if checker == nil {
		checker = NewCommunityCapabilityChecker()
	}
	return Edition{
		Name:         NameCommunity,
		Version:      build,
		Capabilities: checker.List(ctx),
	}
}

func CommunityCapabilities() []Capability {
	return []Capability{
		CapabilityRegistry,
		CapabilityBufWorkflow,
		CapabilityBreakingChecks,
		CapabilityGitLabCI,
		CapabilityGitLabMRBot,
		CapabilityWebUI,
		CapabilityDependencyGraph,
		CapabilityRuntimeInventory,
		CapabilityCommunityGovernance,
		CapabilityAPITokenAuth,
		CapabilityBasicAudit,
		CapabilityPrometheusMetrics,
		CapabilityStructuredLogs,
		CapabilityDockerComposeQuickstart,
	}
}

func EnterpriseCapabilities() []Capability {
	return []Capability{
		CapabilityOIDCAuth,
		CapabilityLDAPAuth,
		CapabilityAdvancedRBAC,
		CapabilityGitLabGroupSync,
		CapabilityAdvancedAudit,
		CapabilityAdvancedApprovalWorkflows,
		CapabilityEnterpriseDependencyGraph,
		CapabilityRuntimeAlerts,
		CapabilityHelmHA,
		CapabilityAirGapped,
		CapabilityLicenseManagement,
	}
}

func AllCapabilities() []Capability {
	capabilities := append(CommunityCapabilities(), EnterpriseCapabilities()...)
	sort.Slice(capabilities, func(i, j int) bool {
		return capabilities[i] < capabilities[j]
	})
	return capabilities
}

func communityCapabilitySet() map[Capability]struct{} {
	enabled := CommunityCapabilities()
	items := make(map[Capability]struct{}, len(enabled))
	for _, capability := range enabled {
		items[capability] = struct{}{}
	}
	return items
}
