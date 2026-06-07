package authorization

import (
	"context"
	"errors"
	"strings"

	"github.com/alryzden/ProtoRadar/internal/identity"
)

type Action string

const (
	ActionModuleCreate           Action = "module:create"
	ActionModuleRead             Action = "module:read"
	ActionModuleUpdate           Action = "module:update"
	ActionModuleDelete           Action = "module:delete"
	ActionModuleVersionPublish   Action = "module_version:publish"
	ActionModuleVersionRead      Action = "module_version:read"
	ActionModuleVersionDownload  Action = "module_version:download"
	ActionModuleVersionDeprecate Action = "module_version:deprecate"
	ActionGitLabMappingManage    Action = "gitlab_mapping:manage"
	ActionGitLabMappingRead      Action = "gitlab_mapping:read"
	ActionBreakingCheckRun       Action = "breaking_check:run"
	ActionBreakingReportRead     Action = "breaking_report:read"
	ActionDependencyGraphRead    Action = "dependency_graph:read"
	ActionRuntimeInventoryReport Action = "runtime_inventory:report"
	ActionRuntimeInventoryRead   Action = "runtime_inventory:read"
	ActionGovernanceOwnerManage  Action = "governance_owner:manage"
	ActionGovernanceOwnerRead    Action = "governance_owner:read"
	ActionApprovalRequestCreate  Action = "approval_request:create"
	ActionApprovalRequestRead    Action = "approval_request:read"
	ActionApprovalDecisionRecord Action = "approval_decision:record"
	ActionGovernanceAuditRead    Action = "governance_audit:read"
	ActionEditionRead            Action = "edition:read"
	ActionAdminRead              Action = "admin:read"
	ActionAdminWrite             Action = "admin:write"
)

func (action Action) String() string {
	return string(action)
}

type Resource struct {
	Type       string            `json:"type"`
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	Attributes map[string]string `json:"attributes"`
}

type Authorizer interface {
	Authorize(ctx context.Context, principal identity.Principal, action Action, resource Resource) error
}

var (
	ErrUnauthenticated = errors.New("unauthenticated")
	ErrForbidden       = errors.New("forbidden")
)

type CommunityAuthorizer struct{}

func (CommunityAuthorizer) Authorize(ctx context.Context, principal identity.Principal, action Action, resource Resource) error {
	if principal.IsZero() {
		return ErrUnauthenticated
	}
	if !communityActionAllowed(action) {
		return ErrForbidden
	}
	return nil
}

func communityActionAllowed(action Action) bool {
	switch action {
	case ActionModuleCreate,
		ActionModuleRead,
		ActionModuleUpdate,
		ActionModuleDelete,
		ActionModuleVersionPublish,
		ActionModuleVersionRead,
		ActionModuleVersionDownload,
		ActionModuleVersionDeprecate,
		ActionGitLabMappingManage,
		ActionGitLabMappingRead,
		ActionBreakingCheckRun,
		ActionBreakingReportRead,
		ActionDependencyGraphRead,
		ActionRuntimeInventoryReport,
		ActionRuntimeInventoryRead,
		ActionGovernanceOwnerManage,
		ActionGovernanceOwnerRead,
		ActionApprovalRequestCreate,
		ActionApprovalRequestRead,
		ActionApprovalDecisionRecord,
		ActionGovernanceAuditRead,
		ActionEditionRead:
		return true
	case ActionAdminRead, ActionAdminWrite:
		return false
	default:
		return false
	}
}

func ResourceWithName(resourceType string, name string) Resource {
	return Resource{Type: strings.TrimSpace(resourceType), Name: strings.TrimSpace(name)}
}

func ResourceWithID(resourceType string, id string) Resource {
	return Resource{Type: strings.TrimSpace(resourceType), ID: strings.TrimSpace(id)}
}

func ModuleResource(moduleName string) Resource {
	return ResourceWithName("module", moduleName)
}

func ModuleVersionResource(moduleName string, version string) Resource {
	moduleName = strings.TrimSpace(moduleName)
	version = strings.TrimSpace(version)
	return Resource{
		Type: "module_version",
		Name: moduleName + ":" + version,
		Attributes: map[string]string{
			"module":  moduleName,
			"version": version,
		},
	}
}

func BreakingReportResource(reportID string) Resource {
	return ResourceWithID("breaking_report", reportID)
}

func ApprovalRequestResource(requestID string) Resource {
	return ResourceWithID("approval_request", requestID)
}

func RuntimeServiceResource(serviceName string) Resource {
	return ResourceWithName("runtime_service", serviceName)
}

func RuntimeEnvironmentResource(environment string) Resource {
	return ResourceWithName("runtime_environment", environment)
}

func EditionResource() Resource {
	return Resource{Type: "edition"}
}
