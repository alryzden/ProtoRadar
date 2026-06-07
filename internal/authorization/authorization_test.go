package authorization

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/alryzden/ProtoRadar/internal/identity"
)

func TestActionStringValues(t *testing.T) {
	tests := []struct {
		name   string
		action Action
		want   string
	}{
		{name: "module create", action: ActionModuleCreate, want: "module:create"},
		{name: "module read", action: ActionModuleRead, want: "module:read"},
		{name: "module update", action: ActionModuleUpdate, want: "module:update"},
		{name: "module delete", action: ActionModuleDelete, want: "module:delete"},
		{name: "module version publish", action: ActionModuleVersionPublish, want: "module_version:publish"},
		{name: "module version read", action: ActionModuleVersionRead, want: "module_version:read"},
		{name: "module version download", action: ActionModuleVersionDownload, want: "module_version:download"},
		{name: "module version deprecate", action: ActionModuleVersionDeprecate, want: "module_version:deprecate"},
		{name: "gitlab mapping manage", action: ActionGitLabMappingManage, want: "gitlab_mapping:manage"},
		{name: "gitlab mapping read", action: ActionGitLabMappingRead, want: "gitlab_mapping:read"},
		{name: "breaking check run", action: ActionBreakingCheckRun, want: "breaking_check:run"},
		{name: "breaking report read", action: ActionBreakingReportRead, want: "breaking_report:read"},
		{name: "dependency graph read", action: ActionDependencyGraphRead, want: "dependency_graph:read"},
		{name: "runtime inventory report", action: ActionRuntimeInventoryReport, want: "runtime_inventory:report"},
		{name: "runtime inventory read", action: ActionRuntimeInventoryRead, want: "runtime_inventory:read"},
		{name: "governance owner manage", action: ActionGovernanceOwnerManage, want: "governance_owner:manage"},
		{name: "governance owner read", action: ActionGovernanceOwnerRead, want: "governance_owner:read"},
		{name: "approval request create", action: ActionApprovalRequestCreate, want: "approval_request:create"},
		{name: "approval request read", action: ActionApprovalRequestRead, want: "approval_request:read"},
		{name: "approval decision record", action: ActionApprovalDecisionRecord, want: "approval_decision:record"},
		{name: "governance audit read", action: ActionGovernanceAuditRead, want: "governance_audit:read"},
		{name: "edition read", action: ActionEditionRead, want: "edition:read"},
		{name: "admin read", action: ActionAdminRead, want: "admin:read"},
		{name: "admin write", action: ActionAdminWrite, want: "admin:write"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.action.String(); got != tt.want {
				t.Fatalf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResourceHelpers(t *testing.T) {
	tests := []struct {
		name string
		got  Resource
		want Resource
	}{
		{
			name: "resource with name trims input",
			got:  ResourceWithName(" module ", " user-api "),
			want: Resource{Type: "module", Name: "user-api"},
		},
		{
			name: "resource with id trims input",
			got:  ResourceWithID(" breaking_report ", " report-1 "),
			want: Resource{Type: "breaking_report", ID: "report-1"},
		},
		{
			name: "module",
			got:  ModuleResource(" user-api "),
			want: Resource{Type: "module", Name: "user-api"},
		},
		{
			name: "module version",
			got:  ModuleVersionResource(" user-api ", " v1.0.0 "),
			want: Resource{
				Type: "module_version",
				Name: "user-api:v1.0.0",
				Attributes: map[string]string{
					"module":  "user-api",
					"version": "v1.0.0",
				},
			},
		},
		{
			name: "breaking report",
			got:  BreakingReportResource(" report-1 "),
			want: Resource{Type: "breaking_report", ID: "report-1"},
		},
		{
			name: "approval request",
			got:  ApprovalRequestResource(" request-1 "),
			want: Resource{Type: "approval_request", ID: "request-1"},
		},
		{
			name: "runtime service",
			got:  RuntimeServiceResource(" billing-service "),
			want: Resource{Type: "runtime_service", Name: "billing-service"},
		},
		{
			name: "runtime environment",
			got:  RuntimeEnvironmentResource(" production "),
			want: Resource{Type: "runtime_environment", Name: "production"},
		},
		{
			name: "edition",
			got:  EditionResource(),
			want: Resource{Type: "edition"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !reflect.DeepEqual(tt.got, tt.want) {
				t.Fatalf("resource = %#v, want %#v", tt.got, tt.want)
			}
		})
	}
}

func TestCommunityAuthorizerAllowsCurrentCommunityActionsForAuthenticatedPrincipal(t *testing.T) {
	authorizer := CommunityAuthorizer{}
	principal := identity.Principal{Subject: "ci", Type: identity.PrincipalTypeAPIToken}
	actions := []Action{
		ActionModuleCreate,
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
		ActionEditionRead,
	}

	for _, action := range actions {
		if err := authorizer.Authorize(context.Background(), principal, action, ResourceWithName("module", "user-api")); err != nil {
			t.Fatalf("action %s error = %v", action, err)
		}
	}
}

func TestCommunityAuthorizerRejectsMissingPrincipal(t *testing.T) {
	err := CommunityAuthorizer{}.Authorize(context.Background(), identity.Principal{}, ActionModuleRead, ResourceWithName("module", "user-api"))
	if !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("error = %v, want ErrUnauthenticated", err)
	}
}

func TestCommunityAuthorizerRejectsAdminAndUnknownActions(t *testing.T) {
	authorizer := CommunityAuthorizer{}
	principal := identity.Principal{Subject: "ci", Type: identity.PrincipalTypeAPIToken}
	for _, action := range []Action{ActionAdminRead, ActionAdminWrite, Action("enterprise:unknown")} {
		err := authorizer.Authorize(context.Background(), principal, action, Resource{})
		if !errors.Is(err, ErrForbidden) {
			t.Fatalf("action %s error = %v, want ErrForbidden", action, err)
		}
	}
}
