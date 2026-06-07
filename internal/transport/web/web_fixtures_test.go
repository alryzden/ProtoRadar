package web

import (
	"time"

	"github.com/alryzden/ProtoRadar/internal/usecase/uiquery"
)

func userModule() uiquery.ModuleOverview {
	latest := uiquery.VersionSummary{Version: "v2.0.0", Status: "published", CreatedAt: testTime(4)}
	return uiquery.ModuleOverview{
		Module: uiquery.ModuleInfo{
			Name:          "user-api",
			Description:   "User service",
			RepositoryURL: "https://gitlab.example.com/platform/user-api",
			CreatedAt:     testTime(1),
			UpdatedAt:     testTime(5),
		},
		GitLabProject:       &uiquery.GitLabProjectInfo{BaseURL: "https://gitlab.example.com", ProjectPath: "platform/user-api", ProjectID: 12345},
		LatestVersion:       &latest,
		VersionCount:        2,
		LastPublishedOrSeen: testTime(7),
		BreakingReportCount: 2,
		LastBreakingStatus:  "breaking",
		Versions: []uiquery.VersionSummary{
			{
				Version:   "v2.0.0",
				Status:    "published",
				CreatedAt: testTime(4),
				Artifacts: []uiquery.ArtifactSummary{
					{Kind: "source_archive", ChecksumSHA256: "sha-source-1234567890abcdef"},
					{Kind: "buf_image", ChecksumSHA256: "sha-buf-image-1234567890"},
				},
				LintStatus:      "passed",
				MetadataSummary: uiquery.DescriptorMetadataSummary{Files: 2, Services: 1, Methods: 3, Messages: 4, Enums: 1},
			},
		},
		RecentReports: []uiquery.BreakingReportSummary{
			userBreakingReport(),
		},
		Owners: []uiquery.ModuleOwner{{
			ID:          "owner-1",
			ModuleID:    "user-api-id",
			ModuleName:  "user-api",
			SubjectType: "team",
			Subject:     "platform-team",
			Role:        "owner",
			CreatedAt:   testTime(4),
			UpdatedAt:   testTime(5),
		}},
	}
}

func billingModule() uiquery.ModuleOverview {
	latest := uiquery.VersionSummary{Version: "v1.0.0", Status: "published", CreatedAt: testTime(3)}
	return uiquery.ModuleOverview{
		Module:              uiquery.ModuleInfo{Name: "billing-api", Description: "Billing service", RepositoryURL: "https://gitlab.example.com/platform/billing-api", CreatedAt: testTime(2), UpdatedAt: testTime(3)},
		LatestVersion:       &latest,
		VersionCount:        1,
		LastPublishedOrSeen: testTime(3),
	}
}

func userVersionOverview() uiquery.VersionOverview {
	return uiquery.VersionOverview{
		Module: uiquery.ModuleInfo{
			Name:          "user-api",
			Description:   "User service",
			RepositoryURL: "https://gitlab.example.com/platform/user-api",
			CreatedAt:     testTime(1),
			UpdatedAt:     testTime(5),
		},
		Version: uiquery.VersionSummary{
			Version:   "v2.0.0",
			Status:    "published",
			CreatedAt: testTime(4),
			Artifacts: []uiquery.ArtifactSummary{
				{Kind: "source_archive", ChecksumSHA256: "sha-source-1234567890abcdef", SizeBytes: 42},
				{Kind: "buf_image", ChecksumSHA256: "sha-buf-image-1234567890", SizeBytes: 64},
			},
			LintStatus:      "passed",
			MetadataSummary: uiquery.DescriptorMetadataSummary{Files: 2, Packages: 1, Imports: 1, Services: 1, Methods: 1, Messages: 2, Fields: 2, Enums: 1, EnumValues: 2},
		},
		Artifacts: []uiquery.ArtifactSummary{
			{Kind: "source_archive", ChecksumSHA256: "sha-source-1234567890abcdef", SizeBytes: 42},
			{Kind: "buf_image", ChecksumSHA256: "sha-buf-image-1234567890", SizeBytes: 64},
		},
		BufConfig: uiquery.BufConfigSummary{
			ConfigPresent:         true,
			LockPresent:           true,
			ModulePaths:           []string{"proto"},
			Deps:                  []string{"buf.build/googleapis/googleapis"},
			LintEnabled:           true,
			LintStatus:            "passed",
			BreakingConfigPresent: true,
		},
		MetadataCounts: uiquery.DescriptorMetadataSummary{Files: 2, Packages: 1, Imports: 1, Services: 1, Methods: 1, Messages: 2, Fields: 2, Enums: 1, EnumValues: 2},
		Metadata: uiquery.DescriptorMetadata{Files: []uiquery.ProtoFile{{
			Path:        "proto/user/v1/user.proto",
			PackageName: "user.v1",
			Syntax:      "proto3",
			Imports: []uiquery.ProtoImport{
				{Path: "google/protobuf/timestamp.proto", Public: true, Weak: false},
			},
			Services: []uiquery.ProtoService{{
				Name:     "UserService",
				FullName: "user.v1.UserService",
				Methods: []uiquery.ProtoMethod{{
					Name:            "GetUser",
					InputType:       "user.v1.GetUserRequest",
					OutputType:      "user.v1.GetUserResponse",
					ClientStreaming: false,
					ServerStreaming: true,
				}},
			}},
			Messages: []uiquery.ProtoMessage{{
				Name:     "User",
				FullName: "user.v1.User",
				Fields: []uiquery.ProtoField{{
					Name:       "display_name",
					Number:     1,
					Type:       "TYPE_STRING",
					TypeName:   "string",
					Label:      "LABEL_OPTIONAL",
					IsRepeated: false,
					IsMap:      false,
				}},
			}},
			Enums: []uiquery.ProtoEnum{{
				Name:     "UserStatus",
				FullName: "user.v1.UserStatus",
				Values: []uiquery.ProtoEnumValue{
					{Name: "USER_STATUS_UNSPECIFIED", Number: 0},
					{Name: "USER_STATUS_ACTIVE", Number: 1},
				},
			}},
		}}},
		RelatedReports: []uiquery.BreakingReportSummary{
			userBreakingReport(),
		},
	}
}

func userPassedReport() uiquery.BreakingReportSummary {
	return uiquery.BreakingReportSummary{
		ID:          "report-1",
		Module:      "user-api",
		BaseVersion: "v2.0.0",
		TargetRef:   "main",
		Status:      "passed",
		ChangeCount: 0,
		CreatedAt:   testTime(6),
	}
}

func userBreakingReport() uiquery.BreakingReportSummary {
	return uiquery.BreakingReportSummary{
		ID:          "report-2",
		Module:      "user-api",
		BaseVersion: "v2.0.0",
		TargetRef:   "feature/remove-field",
		Status:      "breaking",
		ChangeCount: 1,
		CreatedAt:   testTime(7),
	}
}

func billingFailedReport() uiquery.BreakingReportSummary {
	return uiquery.BreakingReportSummary{
		ID:          "report-3",
		Module:      "billing-api",
		BaseVersion: "v1.0.0",
		TargetRef:   "feature/billing-change",
		Status:      "failed",
		ChangeCount: 0,
		CreatedAt:   testTime(8),
	}
}

func userBreakingReportDetails() uiquery.BreakingReportDetails {
	return uiquery.BreakingReportDetails{
		Report:  userBreakingReport(),
		Summary: "Removed field from user response.\nCoordinate compatibility before merging.",
		Changes: []uiquery.BreakingChangeSummary{{
			FilePath:    "proto/user/v1/user.proto",
			PackageName: "user.v1",
			Symbol:      "user.v1.User.display_name",
			RuleID:      "FIELD_NO_DELETE",
			Message:     "Field was removed",
			Severity:    "breaking",
			CreatedAt:   testTime(7),
		}},
		AffectedModules: []uiquery.DependencyModule{{
			Module:            "billing-api",
			Version:           "v1.0.0",
			DependencySources: []string{"import"},
			Reasons:           []string{"import_path"},
		}},
		RuntimeImpact: []uiquery.RuntimeImpact{{
			ServiceName:  "billing-service",
			Environment:  "production",
			UsedModule:   "user-api",
			UsedVersion:  "v2.0.0",
			GitCommit:    "abc1234",
			BuildVersion: "2026.06.04-15",
			ReportedAt:   testTime(9),
			ImpactStatus: "potentially_affected_by_breaking_change",
			Reason:       "service uses base module version",
			DriftStatus:  "deprecated_version",
			DriftReason:  "deprecated_version",
		}},
		Approval: approvalRequestSummary(),
	}
}

func approvalRequestSummary() *uiquery.ApprovalRequestSummary {
	summary := uiquery.ApprovalRequestSummary{
		ID:                "approval-1",
		ModuleID:          "user-api-id",
		ModuleName:        "user-api",
		BreakingReportID:  "report-2",
		TargetRef:         "feature/remove-field",
		Status:            "pending",
		RequiredApprovals: 1,
		ReceivedApprovals: 0,
		Requirements: []uiquery.ApprovalRequirementSummary{{
			ID:                "requirement-1",
			ApprovalRequestID: "approval-1",
			RequirementType:   "module_owner_approval",
			TargetModuleID:    "user-api-id",
			TargetModuleName:  "user-api",
			RequiredRole:      "owner",
			Status:            "pending",
			Reason:            "No owners are configured for this module.",
			CreatedAt:         testTime(7),
			UpdatedAt:         testTime(7),
		}},
		Decisions: []uiquery.ApprovalDecisionSummary{{
			ID:                "decision-1",
			ApprovalRequestID: "approval-1",
			RequirementID:     "requirement-1",
			Decision:          "approved",
			DecidedBy:         "alice",
			Comment:           "looks good",
			CreatedAt:         testTime(8),
		}},
		CreatedAt: testTime(7),
		UpdatedAt: testTime(8),
	}
	return &summary
}

func approvalRequestDetails() uiquery.ApprovalRequestDetails {
	return uiquery.ApprovalRequestDetails{
		Request: *approvalRequestSummary(),
		AuditEvents: []uiquery.GovernanceAuditEventSummary{{
			ID:                "audit-1",
			EventType:         "approval_request_created",
			Actor:             "alice",
			ModuleID:          "user-api-id",
			ModuleName:        "user-api",
			ApprovalRequestID: "approval-1",
			BreakingReportID:  "report-2",
			PayloadJSON:       `{"requirement_count":1}`,
			CreatedAt:         testTime(7),
		}},
	}
}

func userDependencyGraph() uiquery.ModuleDependencyGraph {
	return uiquery.ModuleDependencyGraph{
		Module: uiquery.ModuleInfo{Name: "user-api"},
		Downstream: []uiquery.DependencyModule{{
			Module:            "billing-api",
			Version:           "v1.0.0",
			DependencySources: []string{"import"},
			Reasons:           []string{"import_path"},
		}},
		Upstream: []uiquery.DependencyModule{{
			Module:            "common-api",
			Version:           "v1.0.0",
			DependencySources: []string{"type_reference"},
			Reasons:           []string{"symbol"},
		}},
		Unresolved: []uiquery.UnresolvedDependency{{
			Source:           "import",
			ImportPath:       "missing/v1/missing.proto",
			ReferencedSymbol: "missing.v1.Missing",
			Reason:           "provider_not_found",
		}},
	}
}

func billingRuntimeSummary() uiquery.RuntimeServiceSummary {
	reportedAt := testTime(9)
	return uiquery.RuntimeServiceSummary{
		ServiceName:         "billing-service",
		Environments:        []string{"production"},
		LastReportedAt:      &reportedAt,
		UpToDateCount:       1,
		BehindLatestCount:   1,
		UnknownVersionCount: 1,
		DeprecatedCount:     0,
	}
}

func billingRuntimeDetails() uiquery.RuntimeServiceDetails {
	return uiquery.RuntimeServiceDetails{
		ServiceName: "billing-service",
		Deployments: []uiquery.RuntimeDeployment{{
			ID:           "deployment-1",
			ServiceName:  "billing-service",
			Environment:  "production",
			GitCommit:    "abc1234",
			BuildVersion: "2026.06.04-15",
			ReportedAt:   testTime(9),
			CreatedAt:    testTime(9),
		}},
		Usages: []uiquery.RuntimeModuleUsage{{
			DeploymentID:  "deployment-1",
			Module:        "user-api",
			Version:       "v1.2.0",
			LatestVersion: "v2.0.0",
			DriftStatus:   "behind_latest",
			DriftReason:   "latest version is v2.0.0",
		}, {
			DeploymentID:  "deployment-1",
			Module:        "billing-api",
			Version:       "v1.0.0",
			LatestVersion: "v1.0.0",
			DriftStatus:   "up_to_date",
			DriftReason:   "up_to_date",
		}},
	}
}

func productionRuntimeInventory() uiquery.RuntimeEnvironmentInventory {
	details := billingRuntimeDetails()
	return uiquery.RuntimeEnvironmentInventory{
		Environment: "production",
		Deployments: details.Deployments,
		Usages:      details.Usages,
	}
}

func userAPIRuntimeUsages() uiquery.ModuleRuntimeUsages {
	return uiquery.ModuleRuntimeUsages{
		Module: "user-api",
		Usages: []uiquery.ModuleRuntimeUsage{{
			ServiceName:   "billing-service",
			Environment:   "production",
			Module:        "user-api",
			Version:       "v1.2.0",
			LatestVersion: "v2.0.0",
			GitCommit:     "abc1234",
			BuildVersion:  "2026.06.04-15",
			ReportedAt:    testTime(9),
			DriftStatus:   "behind_latest",
			DriftReason:   "latest version is v2.0.0",
		}},
	}
}

func testTime(hour int) time.Time {
	return time.Date(2026, 6, 4, hour, 0, 0, 0, time.UTC)
}
