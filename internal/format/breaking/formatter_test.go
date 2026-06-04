package breaking

import (
	"strings"
	"testing"

	"github.com/alryzden/ProtoRadar/internal/domain"
)

func TestFormatReportPassed(t *testing.T) {
	moduleName, err := domain.NewModuleName("user-api")
	if err != nil {
		t.Fatalf("module name: %v", err)
	}
	version, err := domain.NewVersion("v1.0.0")
	if err != nil {
		t.Fatalf("version: %v", err)
	}

	output := FormatReport(domain.BreakingReport{
		ModuleName:  moduleName,
		BaseVersion: version,
		TargetRef:   "local",
		Status:      domain.BreakingReportStatusPassed,
		ChangeCount: 0,
	}, nil)

	assertContains(t, output, "ProtoRadar Breaking Change Report")
	assertContains(t, output, "Module: user-api")
	assertContains(t, output, "Against: v1.0.0")
	assertContains(t, output, "Status: passed")
	assertContains(t, output, "No breaking changes found.")
	assertContains(t, output, "Result: no breaking changes found.")
}

func TestFormatReportBreakingWithMultipleChanges(t *testing.T) {
	moduleName, err := domain.NewModuleName("user-api")
	if err != nil {
		t.Fatalf("module name: %v", err)
	}
	version, err := domain.NewVersion("v1.0.0")
	if err != nil {
		t.Fatalf("version: %v", err)
	}

	output := FormatReport(domain.BreakingReport{
		ModuleName:  moduleName,
		BaseVersion: version,
		TargetRef:   "local",
		Status:      domain.BreakingReportStatusBreaking,
		ChangeCount: 2,
	}, []domain.BreakingChange{
		{
			FilePath:    "user/v1/user.proto",
			RuleID:      "FIELD_SAME_TYPE",
			Symbol:      "user.v1.User.email",
			Message:     `Field "email" changed type from string to bytes.`,
			Severity:    "breaking",
			PackageName: "user.v1",
		},
		{
			FilePath: "user/v1/user.proto",
			RuleID:   "RPC_NO_DELETE",
			Symbol:   "user.v1.UserService.GetUser",
			Message:  "RPC was deleted.",
		},
	})

	assertContains(t, output, "Breaking changes:")
	assertContains(t, output, "1. user/v1/user.proto")
	assertContains(t, output, "Rule: FIELD_SAME_TYPE")
	assertContains(t, output, "Symbol: user.v1.User.email")
	assertContains(t, output, `Message: Field "email" changed type from string to bytes.`)
	assertContains(t, output, "2. user/v1/user.proto")
	assertContains(t, output, "Rule: RPC_NO_DELETE")
	assertContains(t, output, "Result: breaking changes found.")
}

func TestFormatReportWithNoOptionalFields(t *testing.T) {
	output := FormatReport(domain.BreakingReport{
		Status:      domain.BreakingReportStatusBreaking,
		ChangeCount: 1,
	}, []domain.BreakingChange{{Message: "A breaking change was detected."}})

	assertContains(t, output, "Target: unspecified")
	assertContains(t, output, "1. (unknown file)")
	assertContains(t, output, "Message: A breaking change was detected.")
	if strings.Contains(output, "Rule:") {
		t.Fatalf("unexpected empty rule line: %s", output)
	}
	if strings.Contains(output, "Symbol:") {
		t.Fatalf("unexpected empty symbol line: %s", output)
	}
}

func assertContains(t *testing.T, output string, want string) {
	t.Helper()
	if !strings.Contains(output, want) {
		t.Fatalf("output missing %q:\n%s", want, output)
	}
}
