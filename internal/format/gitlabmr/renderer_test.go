package gitlabmr

import (
	"strings"
	"testing"
	"time"

	"github.com/alryzden/ProtoRadar/internal/integration/gitlab"
)

func TestBuildMarker(t *testing.T) {
	marker := BuildMarker("user-api")
	if marker != "<!-- protoradar:mr-check module=user-api -->" {
		t.Fatalf("marker = %q", marker)
	}
}

func TestContainsMarker(t *testing.T) {
	body := "before\n" + BuildMarker("user-api") + "\nafter"
	if !ContainsMarker(body, "user-api") {
		t.Fatalf("expected marker in %q", body)
	}
	if ContainsMarker(body, "billing-api") {
		t.Fatal("unexpected marker match for another module")
	}
}

func TestContainsMarkerIgnoresNotesWithoutMarker(t *testing.T) {
	if ContainsMarker("ordinary merge request comment", "user-api") {
		t.Fatal("unexpected marker match")
	}
}

func TestSelectExistingNoteNoMatch(t *testing.T) {
	_, ok := SelectExistingNote([]gitlab.MergeRequestNote{{ID: 1, Body: "ordinary note"}}, "user-api")
	if ok {
		t.Fatal("unexpected note match")
	}
}

func TestSelectExistingNoteWithMarker(t *testing.T) {
	note, ok := SelectExistingNote([]gitlab.MergeRequestNote{
		{ID: 1, Body: "ordinary note"},
		{ID: 2, Body: BuildMarker("user-api") + "\nreport"},
	}, "user-api")
	if !ok || note.ID != 2 {
		t.Fatalf("selected note = %#v, ok=%v", note, ok)
	}
}

func TestSelectExistingNoteChoosesLatestTimestamp(t *testing.T) {
	oldTime := time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC)
	newTime := oldTime.Add(time.Hour)
	note, ok := SelectExistingNote([]gitlab.MergeRequestNote{
		{ID: 1, Body: BuildMarker("user-api"), UpdatedAt: oldTime},
		{ID: 2, Body: BuildMarker("user-api"), UpdatedAt: newTime},
		{ID: 3, Body: BuildMarker("user-api"), UpdatedAt: oldTime.Add(-time.Hour)},
	}, "user-api")
	if !ok || note.ID != 2 {
		t.Fatalf("selected note = %#v, ok=%v", note, ok)
	}
}

func TestSelectExistingNoteChoosesLastWhenNoTimestamps(t *testing.T) {
	note, ok := SelectExistingNote([]gitlab.MergeRequestNote{
		{ID: 1, Body: BuildMarker("user-api")},
		{ID: 2, Body: "ordinary note"},
		{ID: 3, Body: BuildMarker("user-api")},
	}, "user-api")
	if !ok || note.ID != 3 {
		t.Fatalf("selected note = %#v, ok=%v", note, ok)
	}
}

func TestRenderPassedReport(t *testing.T) {
	output := RenderReport(Report{
		Module:      "user-api",
		Against:     "v1.0.0",
		TargetRef:   "abc123",
		Status:      "passed",
		ChangeCount: 0,
		ReportID:    "report-1",
		TargetURL:   "https://gitlab.example.com/job/1",
		CommitSHA:   "abc123",
	}, RenderOptions{})

	assertContains(t, output, BuildMarker("user-api"))
	assertContains(t, output, "## ✅ ProtoRadar Breaking Change Report")
	assertContains(t, output, "| Module | user-api |")
	assertContains(t, output, "| Base version / against | v1.0.0 |")
	assertContains(t, output, "| Target commit/ref | abc123 |")
	assertContains(t, output, "| Status | passed |")
	assertContains(t, output, "| Change count | 0 |")
	assertContains(t, output, "No breaking changes found.")
	assertContains(t, output, "Safe to merge from the protobuf compatibility perspective.")
	assertContains(t, output, "| Report ID | report-1 |")
	assertContains(t, output, "| Target URL | https://gitlab.example.com/job/1 |")
	assertContains(t, output, "| Commit SHA | abc123 |")
}

func TestRenderBreakingReportWithMultipleChanges(t *testing.T) {
	output := RenderReport(Report{
		Module:      "user-api",
		Against:     "v1.0.0",
		TargetRef:   "feature/user-api",
		Status:      "breaking",
		ChangeCount: 2,
		Changes: []Change{
			{FilePath: "user/v1/user.proto", Symbol: "user.v1.User.email", RuleID: "FIELD_SAME_TYPE", Message: "field changed type"},
			{FilePath: "user/v1/user.proto", Symbol: "user.v1.UserService.GetUser", RuleID: "RPC_NO_DELETE", Message: "rpc was deleted"},
		},
	}, RenderOptions{})

	assertContains(t, output, "## ❌ ProtoRadar Breaking Change Report")
	assertContains(t, output, "| File | Symbol | Rule | Message |")
	assertContains(t, output, "| user/v1/user.proto | user.v1.User.email | FIELD_SAME_TYPE | field changed type |")
	assertContains(t, output, "| user/v1/user.proto | user.v1.UserService.GetUser | RPC_NO_DELETE | rpc was deleted |")
	assertContains(t, output, "Restore compatibility, coordinate a major version, or complete the required governance approval workflow.")
}

func TestRenderErrorReport(t *testing.T) {
	output := RenderReport(Report{
		Module:    "user-api",
		Against:   "latest",
		TargetRef: "abc123",
		Status:    "error",
	}, RenderOptions{})

	assertContains(t, output, "## ⚠️ ProtoRadar Breaking Change Report")
	assertContains(t, output, "Check job logs and configuration.")
}

func TestRenderEscapesMarkdownTablePipesAndNormalizesNewlines(t *testing.T) {
	output := RenderReport(Report{
		Module:      "user|api",
		Against:     "v1.0.0",
		TargetRef:   "abc123",
		Status:      "breaking",
		ChangeCount: 1,
		Changes: []Change{{
			FilePath: "user/v1/user.proto",
			Symbol:   "user.v1.User|email",
			RuleID:   "FIELD|SAME|TYPE",
			Message:  "line one\nline two | pipe",
		}},
	}, RenderOptions{})

	assertContains(t, output, `user\|api`)
	assertContains(t, output, `user.v1.User\|email`)
	assertContains(t, output, `FIELD\|SAME\|TYPE`)
	assertContains(t, output, `line one line two \| pipe`)
}

func TestRenderCapsDisplayedChangesAndIncludesOverflowMessage(t *testing.T) {
	changes := []Change{
		{FilePath: "one.proto", Symbol: "one", RuleID: "RULE_ONE", Message: "one"},
		{FilePath: "two.proto", Symbol: "two", RuleID: "RULE_TWO", Message: "two"},
		{FilePath: "three.proto", Symbol: "three", RuleID: "RULE_THREE", Message: "three"},
	}
	output := RenderReport(Report{
		Module:      "user-api",
		Status:      "breaking",
		ChangeCount: len(changes),
		Changes:     changes,
	}, RenderOptions{MaxDisplayedChanges: 2})

	assertContains(t, output, "one.proto")
	assertContains(t, output, "two.proto")
	assertNotContains(t, output, "three.proto")
	assertContains(t, output, "_And 1 more changes. See the full CI artifact for details._")
}

func TestRenderIncludesAffectedModulesTable(t *testing.T) {
	output := RenderReport(Report{
		Module: "user-api",
		Status: "breaking",
		AffectedModules: []AffectedModule{{
			Module:            "billing-api",
			LatestVersion:     "v1.4.0",
			DependencySources: []string{"import", "type_reference"},
			Reasons:           []string{"import_path", "symbol"},
		}},
	}, RenderOptions{})

	assertContains(t, output, "### Potentially Affected Modules")
	assertContains(t, output, "| Module | Latest version | Dependency sources | Reason |")
	assertContains(t, output, "| billing-api | v1.4.0 | import, type_reference | import_path, symbol |")
}

func TestRenderIncludesEmptyAffectedModulesMessage(t *testing.T) {
	output := RenderReport(Report{Module: "user-api", Status: "passed"}, RenderOptions{})

	assertContains(t, output, "### Potentially Affected Modules")
	assertContains(t, output, "No downstream modules are currently known to depend on this module.")
}

func TestRenderCapsDisplayedAffectedModulesAndIncludesOverflowMessage(t *testing.T) {
	output := RenderReport(Report{
		Module: "user-api",
		Status: "breaking",
		AffectedModules: []AffectedModule{
			{Module: "one-api", LatestVersion: "v1.0.0"},
			{Module: "two-api", LatestVersion: "v1.0.0"},
			{Module: "three-api", LatestVersion: "v1.0.0"},
		},
	}, RenderOptions{MaxDisplayedAffectedModules: 2})

	assertContains(t, output, "one-api")
	assertContains(t, output, "two-api")
	assertNotContains(t, output, "three-api")
	assertContains(t, output, "_And 1 more modules. See ProtoRadar UI for the full dependency graph._")
}

func TestRenderIncludesRuntimeImpactTable(t *testing.T) {
	output := RenderReport(Report{
		Module:  "user-api",
		Against: "v1.2.0",
		Status:  "breaking",
		RuntimeImpacts: []RuntimeImpact{{
			ServiceName:  "billing-service",
			Environment:  "production",
			UsedModule:   "user-api",
			UsedVersion:  "v1.2.0",
			BuildVersion: "2026.06.04-15",
			GitCommit:    "abc1234",
			DriftStatus:  "deprecated_version",
			DriftReason:  "deprecated_version",
		}},
	}, RenderOptions{})

	assertContains(t, output, "### Runtime impact")
	assertContains(t, output, "| Service | Environment | Uses | Build | Commit | Runtime drift | Drift reason |")
	assertContains(t, output, "| `billing-service` | `production` | `user-api@v1.2.0` | `2026.06.04-15` | `abc1234` | `deprecated_version` | deprecated_version |")
}

func TestRenderIncludesEmptyRuntimeImpactMessage(t *testing.T) {
	output := RenderReport(Report{Module: "user-api", Status: "passed"}, RenderOptions{})

	assertContains(t, output, "### Runtime impact")
	assertContains(t, output, "No runtime services are currently known to use the affected module version.")
}

func TestRenderRuntimeImpactUnavailableWarning(t *testing.T) {
	output := RenderReport(Report{Module: "user-api", Status: "breaking", RuntimeImpactUnavailable: true}, RenderOptions{})

	assertContains(t, output, "### Runtime impact")
	assertContains(t, output, "Runtime impact could not be loaded. Check CI logs.")
}

func TestRenderCapsRuntimeImpactRowsAndIncludesOverflowMessage(t *testing.T) {
	output := RenderReport(Report{
		Module: "user-api",
		Status: "breaking",
		RuntimeImpacts: []RuntimeImpact{
			{ServiceName: "one-service", Environment: "production", UsedModule: "user-api", UsedVersion: "v1.0.0"},
			{ServiceName: "two-service", Environment: "production", UsedModule: "user-api", UsedVersion: "v1.0.0"},
			{ServiceName: "three-service", Environment: "production", UsedModule: "user-api", UsedVersion: "v1.0.0"},
		},
	}, RenderOptions{MaxDisplayedRuntimeImpacts: 2})

	assertContains(t, output, "one-service")
	assertContains(t, output, "two-service")
	assertNotContains(t, output, "three-service")
	assertContains(t, output, "_And 1 more runtime usages. See ProtoRadar UI for the full runtime inventory._")
}

func TestRenderEscapesRuntimeImpactTableValues(t *testing.T) {
	output := RenderReport(Report{
		Module: "user-api",
		Status: "breaking",
		RuntimeImpacts: []RuntimeImpact{{
			ServiceName:  "billing|service",
			Environment:  "prod|uction",
			UsedModule:   "user|api",
			UsedVersion:  "v1|2|0",
			BuildVersion: "build|15",
			GitCommit:    "abc|123",
			DriftStatus:  "deprecated|version",
			DriftReason:  "Use v1|3|0 instead",
		}},
	}, RenderOptions{})

	assertContains(t, output, `billing\|service`)
	assertContains(t, output, `prod\|uction`)
	assertContains(t, output, `user\|api@v1\|2\|0`)
	assertContains(t, output, `build\|15`)
	assertContains(t, output, `abc\|123`)
	assertContains(t, output, `deprecated\|version`)
	assertContains(t, output, `Use v1\|3\|0 instead`)
}

func TestRenderKeepsAffectedModulesWithRuntimeImpact(t *testing.T) {
	output := RenderReport(Report{
		Module: "user-api",
		Status: "breaking",
		AffectedModules: []AffectedModule{{
			Module:        "billing-api",
			LatestVersion: "v1.4.0",
		}},
		RuntimeImpacts: []RuntimeImpact{{
			ServiceName: "billing-service",
			Environment: "production",
			UsedModule:  "user-api",
			UsedVersion: "v1.2.0",
		}},
	}, RenderOptions{})

	assertContains(t, output, "### Potentially Affected Modules")
	assertContains(t, output, "billing-api")
	assertContains(t, output, "### Runtime impact")
	assertContains(t, output, "billing-service")
}

func TestRenderGovernancePending(t *testing.T) {
	output := RenderReport(Report{
		Module: "user-api",
		Status: "breaking",
		Governance: &Governance{
			Status: "pending",
			Requirements: []GovernanceRequirement{{
				RequirementType:  "module_owner_approval",
				TargetModuleName: "user-api",
				Status:           "pending",
				Reason:           "Breaking changes require approval from module owner",
			}},
		},
	}, RenderOptions{})

	assertContains(t, output, "### Governance")
	assertContains(t, output, "Status: ⏳ Approval required")
	assertContains(t, output, "| Requirement | Target | Status | Reason |")
	assertContains(t, output, "| Module owner approval | `user-api` | Pending | Breaking changes require approval from module owner |")
}

func TestRenderGovernanceApproved(t *testing.T) {
	output := RenderReport(Report{Module: "user-api", Status: "breaking", Governance: &Governance{Status: "approved"}}, RenderOptions{})

	assertContains(t, output, "Status: ✅ Approved")
}

func TestRenderGovernanceRejected(t *testing.T) {
	output := RenderReport(Report{Module: "user-api", Status: "breaking", Governance: &Governance{Status: "rejected"}}, RenderOptions{})

	assertContains(t, output, "Status: ❌ Rejected")
}

func TestRenderGovernanceNotRequired(t *testing.T) {
	output := RenderReport(Report{Module: "user-api", Status: "passed", Governance: &Governance{Status: "not_required"}}, RenderOptions{})

	assertContains(t, output, "Status: ✅ Approval not required")
}

func TestRenderGovernanceMissingOwnerWarning(t *testing.T) {
	output := RenderReport(Report{
		Module: "user-api",
		Status: "breaking",
		Governance: &Governance{
			Status:               "pending",
			MissingOwnerWarnings: []string{"Approval required, but no owners are configured for `user-api`."},
		},
	}, RenderOptions{})

	assertContains(t, output, "Approval required, but no owners are configured for `user-api`.")
}

func TestRenderEscapesGovernanceValues(t *testing.T) {
	output := RenderReport(Report{
		Module: "user-api",
		Status: "breaking",
		Governance: &Governance{
			Status: "pending",
			Requirements: []GovernanceRequirement{{
				RequirementType:  "module_owner_approval",
				TargetModuleName: "user|api",
				Status:           "pending",
				Reason:           "reason | with pipe",
			}},
			Decisions: []GovernanceDecision{{
				Decision:  "approved",
				DecidedBy: "alice|platform",
				Comment:   "comment | pipe",
			}},
		},
	}, RenderOptions{})

	assertContains(t, output, `user\|api`)
	assertContains(t, output, `reason \| with pipe`)
	assertContains(t, output, `alice\|platform`)
	assertContains(t, output, `comment \| pipe`)
}

func TestRenderGovernanceDecisionsUseReturnedDecidedBy(t *testing.T) {
	output := RenderReport(Report{
		Module: "user-api",
		Status: "breaking",
		Governance: &Governance{
			Status: "approved",
			Decisions: []GovernanceDecision{{
				Decision:  "approved",
				DecidedBy: "ci-protoradar-token",
				Comment:   "approved by policy owner",
			}},
		},
	}, RenderOptions{})

	assertContains(t, output, "| Decision | Decided by | Comment |")
	assertContains(t, output, "| Approved | ci-protoradar-token | approved by policy owner |")
	assertNotContains(t, output, "| Decision | Actor | Comment |")
	assertNotContains(t, output, "GitLab user")
	assertNotContains(t, output, "gitlab")
}

func TestRenderIncludesHiddenMarker(t *testing.T) {
	output := RenderReport(Report{Module: "user-api", Status: "passed"}, RenderOptions{})
	if !strings.HasPrefix(output, BuildMarker("user-api")+"\n") {
		t.Fatalf("output does not start with marker:\n%s", output)
	}
}

func TestRenderRedactsTokensAndSecrets(t *testing.T) {
	output := RenderReport(Report{
		Module:    "user-api",
		Against:   "latest",
		TargetRef: "abc123",
		Status:    "breaking",
		TargetURL: "https://gitlab.example.com/job/1?private_token=glpat-secret-token",
		Changes: []Change{{
			FilePath: "user.proto",
			Symbol:   "user.v1.User",
			RuleID:   "RULE",
			Message:  "failed with PROTORADAR_TOKEN=prr_supersecret and password: hunter2",
		}},
		AffectedModules: []AffectedModule{{
			Module:            "billing|api",
			LatestVersion:     "v1.0.0|secret",
			DependencySources: []string{"import|type"},
			Reasons:           []string{"private_token=glpat-another-secret"},
		}},
		RuntimeImpacts: []RuntimeImpact{{
			ServiceName:  "billing-service",
			Environment:  "production",
			UsedModule:   "user-api",
			UsedVersion:  "v1.2.0",
			BuildVersion: "build-private_token=glpat-runtime-secret",
			GitCommit:    "abc123",
		}},
	}, RenderOptions{})

	assertNotContains(t, output, "glpat-secret-token")
	assertNotContains(t, output, "prr_supersecret")
	assertNotContains(t, output, "hunter2")
	assertNotContains(t, output, "glpat-another-secret")
	assertNotContains(t, output, "glpat-runtime-secret")
	assertContains(t, output, `billing\|api`)
	assertContains(t, output, `v1.0.0\|secret`)
	assertContains(t, output, `import\|type`)
	assertContains(t, output, "[redacted]")
}

func assertContains(t *testing.T, output string, want string) {
	t.Helper()
	if !strings.Contains(output, want) {
		t.Fatalf("output missing %q:\n%s", want, output)
	}
}

func assertNotContains(t *testing.T, output string, want string) {
	t.Helper()
	if strings.Contains(output, want) {
		t.Fatalf("output unexpectedly contains %q:\n%s", want, output)
	}
}
