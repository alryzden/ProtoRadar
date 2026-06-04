package bufcli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"

	"github.com/alryzden/ProtoRadar/internal/domain"
	"github.com/alryzden/ProtoRadar/internal/usecase/registry"
)

func TestInspectRunsBufBuildWithDescriptorSetArguments(t *testing.T) {
	runner := &fakeRunner{results: []commandResult{{stdout: descriptorFixture(t)}}}
	workflow := newWorkflow(testConfig(LintModeDisabled), runner)
	workdir := testWorkspace(t, true, false)

	result, err := workflow.Inspect(context.Background(), workdir, registry.BufWorkflowOptions{RequireBufYAML: true})
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}

	if len(runner.calls) != 1 {
		t.Fatalf("commands = %d, want 1", len(runner.calls))
	}
	call := runner.calls[0]
	if call.path != "/bin/buf" {
		t.Fatalf("path = %q", call.path)
	}
	if !slices.Equal(call.args, []string{"build", "--as-file-descriptor-set", "-o", "-"}) {
		t.Fatalf("args = %#v", call.args)
	}
	if call.dir != workdir {
		t.Fatalf("dir = %q", call.dir)
	}
	if result.BufImageDigest == "" || !strings.HasPrefix(result.BufImageDigest, "sha256:") {
		t.Fatalf("digest = %q", result.BufImageDigest)
	}
	if !result.ConfigInfo.BufYAMLPresent {
		t.Fatalf("buf.yaml should be present")
	}
}

func TestInspectRunsBufLintWhenRequested(t *testing.T) {
	runner := &fakeRunner{results: []commandResult{
		{stdout: descriptorFixture(t)},
		{stdout: []byte("lint ok\n")},
	}}
	workflow := newWorkflow(testConfig(LintModeWarn), runner)
	workdir := testWorkspace(t, true, true)

	result, err := workflow.Inspect(context.Background(), workdir, registry.BufWorkflowOptions{RunLint: true})
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}

	if len(runner.calls) != 2 {
		t.Fatalf("commands = %d, want 2", len(runner.calls))
	}
	if !slices.Equal(runner.calls[1].args, []string{"lint"}) {
		t.Fatalf("lint args = %#v", runner.calls[1].args)
	}
	if result.LintResult.Status != domain.BufLintStatusPassed {
		t.Fatalf("lint status = %q", result.LintResult.Status)
	}
	if !result.ConfigInfo.BufLockPresent {
		t.Fatalf("buf.lock should be present")
	}
	if result.ConfigInfo.BufYAMLDigest == "" || result.ConfigInfo.BufLockDigest == "" {
		t.Fatalf("config digests = %#v", result.ConfigInfo)
	}
}

func TestInspectBuildFailureReturnsStructuredError(t *testing.T) {
	runner := &fakeRunner{results: []commandResult{{stderr: []byte("build failed"), exitCode: 1}}}
	workflow := newWorkflow(testConfig(LintModeWarn), runner)
	workdir := testWorkspace(t, true, false)

	result, err := workflow.Inspect(context.Background(), workdir, registry.BufWorkflowOptions{})
	if err == nil || !strings.Contains(err.Error(), "buf build failed") {
		t.Fatalf("error = %v", err)
	}
	if !result.ConfigInfo.BufYAMLPresent {
		t.Fatalf("config info should be returned on build failure")
	}
}

func TestInspectLintWarnModeDoesNotFailWorkflow(t *testing.T) {
	runner := &fakeRunner{results: []commandResult{
		{stdout: descriptorFixture(t)},
		{stderr: []byte("lint warning"), exitCode: 100},
	}}
	workflow := newWorkflow(testConfig(LintModeWarn), runner)
	workdir := testWorkspace(t, true, false)

	result, err := workflow.Inspect(context.Background(), workdir, registry.BufWorkflowOptions{RunLint: true})
	if err != nil {
		t.Fatalf("inspect should not fail in warn mode: %v", err)
	}
	if result.LintResult.Status != domain.BufLintStatusWarning {
		t.Fatalf("lint status = %q", result.LintResult.Status)
	}
	if result.LintResult.Report != "lint warning" {
		t.Fatalf("lint report = %q", result.LintResult.Report)
	}
}

func TestInspectLintEnforceModeFailsWorkflow(t *testing.T) {
	runner := &fakeRunner{results: []commandResult{
		{stdout: descriptorFixture(t)},
		{stderr: []byte("lint failure"), exitCode: 100},
	}}
	workflow := newWorkflow(testConfig(LintModeEnforce), runner)
	workdir := testWorkspace(t, true, false)

	result, err := workflow.Inspect(context.Background(), workdir, registry.BufWorkflowOptions{RunLint: true})
	if err == nil || !strings.Contains(err.Error(), "buf lint failed") {
		t.Fatalf("error = %v", err)
	}
	if result.LintResult.Status != domain.BufLintStatusFailed {
		t.Fatalf("lint status = %q", result.LintResult.Status)
	}
}

func TestInspectTruncatesLintReport(t *testing.T) {
	runner := &fakeRunner{results: []commandResult{
		{stdout: descriptorFixture(t)},
		{stderr: []byte("1234567890"), exitCode: 100},
	}}
	cfg := testConfig(LintModeWarn)
	cfg.MaxReportBytes = 4
	workflow := newWorkflow(cfg, runner)
	workdir := testWorkspace(t, true, false)

	result, err := workflow.Inspect(context.Background(), workdir, registry.BufWorkflowOptions{RunLint: true})
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}
	if result.LintResult.Report != "1234" {
		t.Fatalf("report = %q", result.LintResult.Report)
	}
}

func TestInspectReturnsContextCancellation(t *testing.T) {
	runner := &fakeRunner{err: context.Canceled}
	workflow := newWorkflow(testConfig(LintModeDisabled), runner)
	workdir := testWorkspace(t, true, false)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := workflow.Inspect(ctx, workdir, registry.BufWorkflowOptions{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}

func TestInspectRequiresBufYAML(t *testing.T) {
	runner := &fakeRunner{results: []commandResult{{stdout: descriptorFixture(t)}}}
	workflow := newWorkflow(testConfig(LintModeDisabled), runner)
	workdir := testWorkspace(t, false, false)

	_, err := workflow.Inspect(context.Background(), workdir, registry.BufWorkflowOptions{RequireBufYAML: true})
	if err == nil || !strings.Contains(err.Error(), "buf.yaml is required") {
		t.Fatalf("error = %v", err)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("commands should not run when buf.yaml is missing")
	}
}

func TestExtractDescriptorMetadata(t *testing.T) {
	metadata, err := ExtractDescriptorMetadata(descriptorFixture(t))
	if err != nil {
		t.Fatalf("extract metadata: %v", err)
	}
	if len(metadata.Files) != 1 {
		t.Fatalf("files = %d", len(metadata.Files))
	}
	file := metadata.Files[0]
	if file.Path != "user/v1/user.proto" || file.PackageName != "user.v1" || file.Syntax != "proto3" {
		t.Fatalf("file = %#v", file)
	}
	if len(file.Imports) != 2 || !file.Imports[0].Public || !file.Imports[1].Weak {
		t.Fatalf("imports = %#v", file.Imports)
	}
	if len(file.Services) != 1 || file.Services[0].FullName != "user.v1.UserService" {
		t.Fatalf("services = %#v", file.Services)
	}
	if len(file.Services[0].Methods) != 1 || !file.Services[0].Methods[0].ServerStreaming {
		t.Fatalf("methods = %#v", file.Services[0].Methods)
	}
	if len(file.Messages) != 1 || file.Messages[0].FullName != "user.v1.User" {
		t.Fatalf("messages = %#v", file.Messages)
	}
	if len(file.Messages[0].Fields) != 3 {
		t.Fatalf("fields = %#v", file.Messages[0].Fields)
	}
	if !file.Messages[0].Fields[1].IsRepeated {
		t.Fatalf("tags field should be repeated")
	}
	if !file.Messages[0].Fields[2].IsMap {
		t.Fatalf("attributes field should be detected as map")
	}
	if len(file.Messages[0].Enums) != 1 || file.Messages[0].Enums[0].Values[1].Number != 1 {
		t.Fatalf("nested enums = %#v", file.Messages[0].Enums)
	}
	if len(file.Enums) != 1 || file.Enums[0].FullName != "user.v1.Role" {
		t.Fatalf("file enums = %#v", file.Enums)
	}
}

func TestCheckBreakingRunsBufBreakingWithBaselineImage(t *testing.T) {
	runner := &fakeRunner{results: []commandResult{{stdout: []byte("ok")}}}
	workflow := newWorkflow(testConfig(LintModeDisabled), runner)
	workdir := testWorkspace(t, true, false)

	result, err := workflow.CheckBreaking(context.Background(), registry.BufBreakingCheckInput{
		Workdir:       workdir,
		BaselineImage: []byte("baseline image"),
		TargetRef:     "local",
	})
	if err != nil {
		t.Fatalf("check breaking: %v", err)
	}
	if result.Status != domain.BreakingReportStatusPassed {
		t.Fatalf("status = %q", result.Status)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("commands = %d, want 1", len(runner.calls))
	}
	call := runner.calls[0]
	if call.path != "/bin/buf" {
		t.Fatalf("path = %q", call.path)
	}
	if len(call.args) != 4 || call.args[0] != "breaking" || call.args[1] != workdir || call.args[2] != "--against" || call.args[3] == "" {
		t.Fatalf("args = %#v", call.args)
	}
	if call.dir != workdir {
		t.Fatalf("dir = %q", call.dir)
	}
}

func TestCheckBreakingExitCodeZeroMapsToPassed(t *testing.T) {
	runner := &fakeRunner{results: []commandResult{{stdout: []byte("Success: no breaking changes")}}}
	workflow := newWorkflow(testConfig(LintModeDisabled), runner)

	result, err := workflow.CheckBreaking(context.Background(), registry.BufBreakingCheckInput{
		Workdir:       testWorkspace(t, true, false),
		BaselineImage: []byte("baseline image"),
	})
	if err != nil {
		t.Fatalf("check breaking: %v", err)
	}
	if result.Status != domain.BreakingReportStatusPassed {
		t.Fatalf("status = %q", result.Status)
	}
	if result.HumanSummary != "No breaking changes found." {
		t.Fatalf("summary = %q", result.HumanSummary)
	}
}

func TestCheckBreakingDiagnosticsMapToBreaking(t *testing.T) {
	report := "user/v1/user.proto:12:5:FIELD_SAME_TYPE: Field \"email\" changed type from string to bytes.\n" +
		"user/v1/user.proto:RPC_NO_DELETE: RPC user.v1.UserService.GetUser was deleted.\n"
	runner := &fakeRunner{results: []commandResult{{stderr: []byte(report), exitCode: 100}}}
	workflow := newWorkflow(testConfig(LintModeDisabled), runner)

	result, err := workflow.CheckBreaking(context.Background(), registry.BufBreakingCheckInput{
		Workdir:       testWorkspace(t, true, false),
		BaselineImage: []byte("baseline image"),
	})
	if err != nil {
		t.Fatalf("breaking diagnostics should not be internal error: %v", err)
	}
	if result.Status != domain.BreakingReportStatusBreaking {
		t.Fatalf("status = %q", result.Status)
	}
	if len(result.Changes) != 2 {
		t.Fatalf("changes = %d, want 2", len(result.Changes))
	}
	first := result.Changes[0]
	if first.FilePath != "user/v1/user.proto" {
		t.Fatalf("file path = %q", first.FilePath)
	}
	if first.RuleID != "FIELD_SAME_TYPE" {
		t.Fatalf("rule id = %q", first.RuleID)
	}
	if !strings.Contains(first.Message, "email") {
		t.Fatalf("message = %q", first.Message)
	}
	if first.Category != "field" {
		t.Fatalf("category = %q", first.Category)
	}
}

func TestCheckBreakingCommandFailureMapsToFailed(t *testing.T) {
	runner := &fakeRunner{err: errors.New("exec failed")}
	workflow := newWorkflow(testConfig(LintModeDisabled), runner)

	result, err := workflow.CheckBreaking(context.Background(), registry.BufBreakingCheckInput{
		Workdir:       testWorkspace(t, true, false),
		BaselineImage: []byte("baseline image"),
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	if result.Status != domain.BreakingReportStatusFailed {
		t.Fatalf("status = %q", result.Status)
	}
}

func TestCheckBreakingNonDiagnosticFailureMapsToFailed(t *testing.T) {
	runner := &fakeRunner{results: []commandResult{{stderr: []byte("failed to parse configuration"), exitCode: 1}}}
	workflow := newWorkflow(testConfig(LintModeDisabled), runner)

	result, err := workflow.CheckBreaking(context.Background(), registry.BufBreakingCheckInput{
		Workdir:       testWorkspace(t, true, false),
		BaselineImage: []byte("baseline image"),
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	if result.Status != domain.BreakingReportStatusFailed {
		t.Fatalf("status = %q", result.Status)
	}
}

func TestCheckBreakingCapturesStdoutAndStderr(t *testing.T) {
	runner := &fakeRunner{results: []commandResult{{
		stdout:   []byte("user/v1/user.proto:FIELD_NO_DELETE: field was deleted"),
		stderr:   []byte("user/v1/user.proto:RPC_NO_DELETE: rpc was deleted"),
		exitCode: 100,
	}}}
	workflow := newWorkflow(testConfig(LintModeDisabled), runner)

	result, err := workflow.CheckBreaking(context.Background(), registry.BufBreakingCheckInput{
		Workdir:       testWorkspace(t, true, false),
		BaselineImage: []byte("baseline image"),
	})
	if err != nil {
		t.Fatalf("check breaking: %v", err)
	}
	if !strings.Contains(result.RawOutput, "FIELD_NO_DELETE") || !strings.Contains(result.RawOutput, "RPC_NO_DELETE") {
		t.Fatalf("raw output = %q", result.RawOutput)
	}
}

func TestCheckBreakingTruncatesRawOutput(t *testing.T) {
	runner := &fakeRunner{results: []commandResult{{
		stderr:   []byte("user/v1/user.proto:FIELD_SAME_TYPE: 1234567890"),
		exitCode: 100,
	}}}
	cfg := testConfig(LintModeDisabled)
	cfg.MaxReportBytes = 24
	workflow := newWorkflow(cfg, runner)

	result, err := workflow.CheckBreaking(context.Background(), registry.BufBreakingCheckInput{
		Workdir:       testWorkspace(t, true, false),
		BaselineImage: []byte("baseline image"),
	})
	if err != nil {
		t.Fatalf("check breaking: %v", err)
	}
	if len(result.RawOutput) != 24 {
		t.Fatalf("raw output length = %d, want 24: %q", len(result.RawOutput), result.RawOutput)
	}
}

func TestCheckBreakingReturnsContextCancellation(t *testing.T) {
	runner := &fakeRunner{err: context.Canceled}
	workflow := newWorkflow(testConfig(LintModeDisabled), runner)

	result, err := workflow.CheckBreaking(context.Background(), registry.BufBreakingCheckInput{
		Workdir:       testWorkspace(t, true, false),
		BaselineImage: []byte("baseline image"),
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	if result.Status != domain.BreakingReportStatusFailed {
		t.Fatalf("status = %q", result.Status)
	}
}

type fakeRunner struct {
	calls   []commandSpec
	results []commandResult
	err     error
}

func (runner *fakeRunner) Run(ctx context.Context, spec commandSpec) (commandResult, error) {
	runner.calls = append(runner.calls, spec)
	if runner.err != nil {
		return commandResult{}, runner.err
	}
	if len(runner.results) == 0 {
		return commandResult{}, nil
	}
	result := runner.results[0]
	runner.results = runner.results[1:]
	return result, nil
}

func testConfig(lintMode string) Config {
	return Config{
		BinaryPath:     "/bin/buf",
		BuildTimeout:   time.Second,
		LintTimeout:    time.Second,
		LintMode:       lintMode,
		MaxReportBytes: 1024,
	}
}

func testWorkspace(t *testing.T, withBufYAML bool, withBufLock bool) string {
	t.Helper()
	workdir := t.TempDir()
	if withBufYAML {
		if err := os.WriteFile(filepath.Join(workdir, "buf.yaml"), []byte("version: v2\n"), 0o600); err != nil {
			t.Fatalf("write buf.yaml: %v", err)
		}
	}
	if withBufLock {
		if err := os.WriteFile(filepath.Join(workdir, "buf.lock"), []byte("deps: []\n"), 0o600); err != nil {
			t.Fatalf("write buf.lock: %v", err)
		}
	}
	return workdir
}

func descriptorFixture(t *testing.T) []byte {
	t.Helper()
	oneofIndex := int32(0)
	body, err := proto.Marshal(&descriptorpb.FileDescriptorSet{File: []*descriptorpb.FileDescriptorProto{
		{
			Name:             proto.String("user/v1/user.proto"),
			Package:          proto.String("user.v1"),
			Syntax:           proto.String("proto3"),
			Dependency:       []string{"google/protobuf/timestamp.proto", "common/v1/common.proto"},
			PublicDependency: []int32{0},
			WeakDependency:   []int32{1},
			MessageType: []*descriptorpb.DescriptorProto{
				{
					Name: proto.String("User"),
					Field: []*descriptorpb.FieldDescriptorProto{
						{
							Name:     proto.String("id"),
							JsonName: proto.String("id"),
							Number:   proto.Int32(1),
							Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
							Type:     descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
						},
						{
							Name:     proto.String("tags"),
							JsonName: proto.String("tags"),
							Number:   proto.Int32(2),
							Label:    descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum(),
							Type:     descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
						},
						{
							Name:       proto.String("attributes"),
							JsonName:   proto.String("attributes"),
							Number:     proto.Int32(3),
							Label:      descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum(),
							Type:       descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
							TypeName:   proto.String(".user.v1.User.AttributesEntry"),
							OneofIndex: &oneofIndex,
						},
					},
					OneofDecl: []*descriptorpb.OneofDescriptorProto{{Name: proto.String("identity")}},
					NestedType: []*descriptorpb.DescriptorProto{
						{
							Name:    proto.String("AttributesEntry"),
							Options: &descriptorpb.MessageOptions{MapEntry: proto.Bool(true)},
						},
					},
					EnumType: []*descriptorpb.EnumDescriptorProto{
						{
							Name: proto.String("State"),
							Value: []*descriptorpb.EnumValueDescriptorProto{
								{Name: proto.String("STATE_UNSPECIFIED"), Number: proto.Int32(0)},
								{Name: proto.String("STATE_ACTIVE"), Number: proto.Int32(1)},
							},
						},
					},
				},
			},
			EnumType: []*descriptorpb.EnumDescriptorProto{
				{
					Name: proto.String("Role"),
					Value: []*descriptorpb.EnumValueDescriptorProto{
						{Name: proto.String("ROLE_UNSPECIFIED"), Number: proto.Int32(0)},
					},
				},
			},
			Service: []*descriptorpb.ServiceDescriptorProto{
				{
					Name: proto.String("UserService"),
					Method: []*descriptorpb.MethodDescriptorProto{
						{
							Name:            proto.String("WatchUsers"),
							InputType:       proto.String(".user.v1.WatchUsersRequest"),
							OutputType:      proto.String(".user.v1.User"),
							ServerStreaming: proto.Bool(true),
						},
					},
				},
			},
		},
	}})
	if err != nil {
		t.Fatalf("marshal descriptor fixture: %v", err)
	}
	return body
}
