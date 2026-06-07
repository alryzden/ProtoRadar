package gitlabmr

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/alryzden/ProtoRadar/internal/format/gitlabmr"
	"github.com/alryzden/ProtoRadar/internal/integration/gitlab"
)

func TestPassedCheckCreatesNewMRComment(t *testing.T) {
	runner, breaking, gitlabClient := newTestRunner()
	breaking.report = passedReport()

	result, err := runner.Run(context.Background(), validInput())
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if result.ExitCode != ExitCodePassed || result.CommentAction != "created" || result.NoteID != 1 {
		t.Fatalf("result = %#v", result)
	}
	if len(gitlabClient.createdBodies) != 1 {
		t.Fatalf("created bodies = %d", len(gitlabClient.createdBodies))
	}
	assertContains(t, gitlabClient.createdBodies[0], "## ✅ ProtoRadar Breaking Change Report")
	assertContains(t, gitlabClient.createdBodies[0], gitlabmr.BuildMarker("user-api"))
}

func TestPassedCheckUpdatesExistingMRComment(t *testing.T) {
	runner, breaking, gitlabClient := newTestRunner()
	breaking.report = passedReport()
	gitlabClient.notes = []gitlab.MergeRequestNote{{ID: 44, Body: gitlabmr.BuildMarker("user-api")}}

	result, err := runner.Run(context.Background(), validInput())
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if result.ExitCode != ExitCodePassed || result.CommentAction != "updated" || result.NoteID != 44 {
		t.Fatalf("result = %#v", result)
	}
	if len(gitlabClient.updatedBodies) != 1 || len(gitlabClient.createdBodies) != 0 {
		t.Fatalf("created=%d updated=%d", len(gitlabClient.createdBodies), len(gitlabClient.updatedBodies))
	}
}

func TestBreakingCheckCreatesNewMRComment(t *testing.T) {
	runner, breaking, gitlabClient := newTestRunner()
	breaking.report = breakingReport()
	runner.AffectedModulesClient = &fakeAffectedModulesClient{items: []AffectedModule{{
		Module:            "billing-api",
		LatestVersion:     "v1.4.0",
		DependencySources: []string{"import"},
		Reasons:           []string{"import_path"},
	}}}

	result, err := runner.Run(context.Background(), validInput())
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if result.ExitCode != ExitCodeBreaking || result.CommentAction != "created" {
		t.Fatalf("result = %#v", result)
	}
	assertContains(t, gitlabClient.createdBodies[0], "## ❌ ProtoRadar Breaking Change Report")
	assertContains(t, gitlabClient.createdBodies[0], "FIELD_SAME_TYPE")
	assertContains(t, gitlabClient.createdBodies[0], "| billing-api | v1.4.0 | import | import_path |")
}

func TestRunnerFetchesAffectedModulesAfterBreakingCheck(t *testing.T) {
	runner, breaking, gitlabClient := newTestRunner()
	breaking.report = breakingReport()
	affected := &fakeAffectedModulesClient{items: []AffectedModule{{Module: "billing-api", LatestVersion: "v1.4.0"}}}
	runner.AffectedModulesClient = affected

	result, err := runner.Run(context.Background(), validInput())
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if result.ExitCode != ExitCodeBreaking {
		t.Fatalf("result = %#v", result)
	}
	if breaking.calls != 1 || affected.calls != 1 || affected.modules[0] != "user-api" {
		t.Fatalf("breaking calls=%d affected calls=%d modules=%#v", breaking.calls, affected.calls, affected.modules)
	}
	assertContains(t, gitlabClient.createdBodies[0], "billing-api")
}

func TestRunnerFetchesRuntimeImpactWhenReportIDExists(t *testing.T) {
	runner, breaking, gitlabClient := newTestRunner()
	breaking.report = breakingReport()
	runtimeImpact := &fakeRuntimeImpactClient{items: []RuntimeImpact{{
		ServiceName:  "billing-service",
		Environment:  "production",
		UsedModule:   "user-api",
		UsedVersion:  "v1.2.0",
		BuildVersion: "2026.06.04-15",
		GitCommit:    "abc1234",
		DriftStatus:  "deprecated_version",
		DriftReason:  "deprecated_version",
	}}}
	runner.RuntimeImpactClient = runtimeImpact

	result, err := runner.Run(context.Background(), validInput())
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if result.ExitCode != ExitCodeBreaking {
		t.Fatalf("result = %#v", result)
	}
	if breaking.calls != 1 || runtimeImpact.calls != 1 || runtimeImpact.reportIDs[0] != "report-2" {
		t.Fatalf("breaking calls=%d runtime calls=%d reportIDs=%#v", breaking.calls, runtimeImpact.calls, runtimeImpact.reportIDs)
	}
	assertContains(t, gitlabClient.createdBodies[0], "### Runtime impact")
	assertContains(t, gitlabClient.createdBodies[0], "| `billing-service` | `production` | `user-api@v1.2.0` | `2026.06.04-15` | `abc1234` | `deprecated_version` | deprecated_version |")
}

func TestRunnerRuntimeImpactFailureDoesNotFailBreakingCheck(t *testing.T) {
	runner, breaking, gitlabClient := newTestRunner()
	breaking.report = breakingReport()
	runner.RuntimeImpactClient = &fakeRuntimeImpactClient{err: errors.New("runtime impact API failed")}

	result, err := runner.Run(context.Background(), validInput())
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if result.ExitCode != ExitCodeBreaking {
		t.Fatalf("result = %#v", result)
	}
	assertContains(t, gitlabClient.createdBodies[0], "Runtime impact could not be loaded. Check CI logs.")
	assertStatusStates(t, gitlabClient.statuses, gitlab.CommitStatusStateRunning, gitlab.CommitStatusStateFailed)
}

func TestPassedReportWithoutIDDoesNotFetchRuntimeImpactOrCrash(t *testing.T) {
	runner, breaking, gitlabClient := newTestRunner()
	report := passedReport()
	report.ID = ""
	breaking.report = report
	runtimeImpact := &fakeRuntimeImpactClient{err: errors.New("should not be called")}
	runner.RuntimeImpactClient = runtimeImpact

	result, err := runner.Run(context.Background(), validInput())
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if result.ExitCode != ExitCodePassed {
		t.Fatalf("result = %#v", result)
	}
	if runtimeImpact.calls != 0 {
		t.Fatalf("runtime impact calls = %d", runtimeImpact.calls)
	}
	assertContains(t, gitlabClient.createdBodies[0], "No runtime services are currently known to use the affected module version.")
}

func TestBreakingCheckUpdatesExistingMRComment(t *testing.T) {
	runner, breaking, gitlabClient := newTestRunner()
	breaking.report = breakingReport()
	gitlabClient.notes = []gitlab.MergeRequestNote{{ID: 9, Body: gitlabmr.BuildMarker("user-api")}}

	result, err := runner.Run(context.Background(), validInput())
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if result.ExitCode != ExitCodeBreaking || result.CommentAction != "updated" || result.NoteID != 9 {
		t.Fatalf("result = %#v", result)
	}
	if len(gitlabClient.updatedBodies) != 1 || len(gitlabClient.createdBodies) != 0 {
		t.Fatalf("created=%d updated=%d", len(gitlabClient.createdBodies), len(gitlabClient.updatedBodies))
	}
}

func TestExistingMarkerPreventsDuplicateComment(t *testing.T) {
	runner, breaking, gitlabClient := newTestRunner()
	breaking.report = passedReport()
	gitlabClient.notes = []gitlab.MergeRequestNote{
		{ID: 1, Body: gitlabmr.BuildMarker("user-api"), UpdatedAt: time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC)},
		{ID: 2, Body: gitlabmr.BuildMarker("user-api"), UpdatedAt: time.Date(2026, 6, 4, 13, 0, 0, 0, time.UTC)},
	}

	result, err := runner.Run(context.Background(), validInput())
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if result.CommentAction != "updated" || gitlabClient.updatedNoteIDs[0] != 2 || len(gitlabClient.createdBodies) != 0 {
		t.Fatalf("result=%#v updated=%#v created=%d", result, gitlabClient.updatedNoteIDs, len(gitlabClient.createdBodies))
	}
}

func TestPassedCheckSetsCommitStatusSuccess(t *testing.T) {
	runner, breaking, gitlabClient := newTestRunner()
	breaking.report = passedReport()

	result, err := runner.Run(context.Background(), validInput())
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if result.ExitCode != ExitCodePassed {
		t.Fatalf("result = %#v", result)
	}
	assertStatusStates(t, gitlabClient.statuses, gitlab.CommitStatusStateRunning, gitlab.CommitStatusStateSuccess)
}

func TestBreakingCheckSetsCommitStatusFailed(t *testing.T) {
	runner, breaking, gitlabClient := newTestRunner()
	breaking.report = breakingReport()

	result, err := runner.Run(context.Background(), validInput())
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if result.ExitCode != ExitCodeBreaking {
		t.Fatalf("result = %#v", result)
	}
	assertStatusStates(t, gitlabClient.statuses, gitlab.CommitStatusStateRunning, gitlab.CommitStatusStateFailed)
}

func TestGovernanceModeCreatesApprovalRequestWhenStatusMissing(t *testing.T) {
	runner, breaking, gitlabClient := newTestRunner()
	breaking.report = breakingReport()
	governance := &fakeGovernanceClient{
		getErr: ErrGovernanceNotFound,
		createStatus: GovernanceStatus{
			ID:     "approval-1",
			Status: "pending",
			Requirements: []GovernanceRequirement{{
				RequirementType:  "module_owner_approval",
				TargetModuleName: "user-api",
				Status:           "pending",
				Reason:           "Breaking changes require approval from module owner",
			}},
		},
	}
	runner.GovernanceClient = governance
	input := validInput()
	input.GovernanceEnabled = true

	result, err := runner.Run(context.Background(), input)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if result.ExitCode != ExitCodeBreaking || governance.getCalls != 1 || governance.createCalls != 1 {
		t.Fatalf("result=%#v governance=%#v", result, governance)
	}
	if governance.createActors[0] != "" {
		t.Fatalf("approval actor should be empty by default: %#v", governance.createActors)
	}
	assertContains(t, gitlabClient.createdBodies[0], "### Governance")
	assertContains(t, gitlabClient.createdBodies[0], "Status: ⏳ Approval required")
	assertStatusStates(t, gitlabClient.statuses, gitlab.CommitStatusStateRunning, gitlab.CommitStatusStateFailed)
}

func TestGovernanceModeSendsActorOnlyWhenExplicitlyConfigured(t *testing.T) {
	runner, breaking, _ := newTestRunner()
	breaking.report = breakingReport()
	governance := &fakeGovernanceClient{
		getErr:       ErrGovernanceNotFound,
		createStatus: GovernanceStatus{ID: "approval-1", Status: "pending"},
	}
	runner.GovernanceClient = governance
	input := validInput()
	input.GovernanceEnabled = true
	input.GovernanceActor = "ci-override"

	_, err := runner.Run(context.Background(), input)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(governance.createActors) != 1 || governance.createActors[0] != "ci-override" {
		t.Fatalf("create actors = %#v", governance.createActors)
	}
}

func TestGovernanceModeActorOverrideRejectionIsClearAndRedacted(t *testing.T) {
	runner, breaking, _ := newTestRunner()
	breaking.report = breakingReport()
	governance := &fakeGovernanceClient{
		getErr:    ErrGovernanceNotFound,
		createErr: errors.New("Governance actor is not allowed for PROTORADAR_TOKEN=prr_supersecret"),
	}
	runner.GovernanceClient = governance
	input := validInput()
	input.GovernanceEnabled = true
	input.GovernanceActor = "alice"

	result, err := runner.Run(context.Background(), input)
	if err == nil || result.ExitCode != ExitCodeError {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	if !strings.Contains(err.Error(), "server rejected governance actor override") || !strings.Contains(err.Error(), "omit --governance-actor") {
		t.Fatalf("error = %v", err)
	}
	if strings.Contains(err.Error(), "prr_supersecret") || strings.Contains(result.Markdown, "prr_supersecret") {
		t.Fatalf("token leaked: result=%#v err=%v", result, err)
	}
}

func TestGovernanceModeApprovedBreakingChangeExitsZero(t *testing.T) {
	runner, breaking, gitlabClient := newTestRunner()
	breaking.report = breakingReport()
	runner.GovernanceClient = &fakeGovernanceClient{getStatus: GovernanceStatus{ID: "approval-1", Status: "approved"}}
	input := validInput()
	input.GovernanceEnabled = true

	result, err := runner.Run(context.Background(), input)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if result.ExitCode != ExitCodePassed {
		t.Fatalf("result = %#v", result)
	}
	assertContains(t, gitlabClient.createdBodies[0], "Status: ✅ Approved")
	assertStatusStates(t, gitlabClient.statuses, gitlab.CommitStatusStateRunning, gitlab.CommitStatusStateSuccess)
}

func TestGovernanceModeRejectedBreakingChangeExitsOne(t *testing.T) {
	runner, breaking, gitlabClient := newTestRunner()
	breaking.report = breakingReport()
	runner.GovernanceClient = &fakeGovernanceClient{getStatus: GovernanceStatus{ID: "approval-1", Status: "rejected"}}
	input := validInput()
	input.GovernanceEnabled = true

	result, err := runner.Run(context.Background(), input)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if result.ExitCode != ExitCodeBreaking {
		t.Fatalf("result = %#v", result)
	}
	assertContains(t, gitlabClient.createdBodies[0], "Status: ❌ Rejected")
	assertStatusStates(t, gitlabClient.statuses, gitlab.CommitStatusStateRunning, gitlab.CommitStatusStateFailed)
}

func TestGovernanceFalseKeepsOldBreakingExitBehavior(t *testing.T) {
	runner, breaking, _ := newTestRunner()
	breaking.report = breakingReport()
	runner.GovernanceClient = &fakeGovernanceClient{getStatus: GovernanceStatus{ID: "approval-1", Status: "approved"}}

	result, err := runner.Run(context.Background(), validInput())
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if result.ExitCode != ExitCodeBreaking {
		t.Fatalf("result = %#v", result)
	}
}

func TestGovernanceModeErrorReturnsExitCodeTwo(t *testing.T) {
	runner, breaking, _ := newTestRunner()
	breaking.report = breakingReport()
	runner.GovernanceClient = &fakeGovernanceClient{getErr: errors.New("governance API failed")}
	input := validInput()
	input.GovernanceEnabled = true

	result, err := runner.Run(context.Background(), input)
	if err == nil || result.ExitCode != ExitCodeError {
		t.Fatalf("result=%#v err=%v", result, err)
	}
}

func TestStatusFalseSkipsCommitStatusCalls(t *testing.T) {
	runner, breaking, gitlabClient := newTestRunner()
	breaking.report = passedReport()
	input := validInput()
	input.StatusEnabled = false
	input.CommitSHA = ""

	result, err := runner.Run(context.Background(), input)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if result.ExitCode != ExitCodePassed {
		t.Fatalf("result = %#v", result)
	}
	if len(gitlabClient.statuses) != 0 {
		t.Fatalf("statuses = %#v", gitlabClient.statuses)
	}
}

func TestGitLabNoteCreationFailureReturnsExitCodeTwo(t *testing.T) {
	runner, breaking, gitlabClient := newTestRunner()
	breaking.report = passedReport()
	gitlabClient.createErr = errors.New("create failed")

	result, err := runner.Run(context.Background(), validInput())
	if err == nil || result.ExitCode != ExitCodeError {
		t.Fatalf("result=%#v err=%v", result, err)
	}
}

func TestGitLabNoteUpdateFailureReturnsExitCodeTwo(t *testing.T) {
	runner, breaking, gitlabClient := newTestRunner()
	breaking.report = passedReport()
	gitlabClient.notes = []gitlab.MergeRequestNote{{ID: 4, Body: gitlabmr.BuildMarker("user-api")}}
	gitlabClient.updateErr = errors.New("update failed")

	result, err := runner.Run(context.Background(), validInput())
	if err == nil || result.ExitCode != ExitCodeError {
		t.Fatalf("result=%#v err=%v", result, err)
	}
}

func TestCommitStatusFailureReturnsExitCodeTwoWhenEnabled(t *testing.T) {
	runner, breaking, gitlabClient := newTestRunner()
	breaking.report = passedReport()
	gitlabClient.failStatusOnCall = 2

	result, err := runner.Run(context.Background(), validInput())
	if err == nil || result.ExitCode != ExitCodeError {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	if len(gitlabClient.createdBodies) != 1 {
		t.Fatalf("expected report comment before final status failure")
	}
}

func TestBreakingCheckToolErrorReturnsExitCodeTwo(t *testing.T) {
	runner, breaking, _ := newTestRunner()
	breaking.err = errors.New("buf failed")

	result, err := runner.Run(context.Background(), validInput())
	if err == nil || result.ExitCode != ExitCodeError || result.BreakingStatus != "error" {
		t.Fatalf("result=%#v err=%v", result, err)
	}
}

func TestRunnerAttemptsErrorCommentAndStatusOnBreakingCheckError(t *testing.T) {
	runner, breaking, gitlabClient := newTestRunner()
	breaking.err = errors.New("buf failed")

	result, err := runner.Run(context.Background(), validInput())
	if err == nil || result.ExitCode != ExitCodeError {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	if len(gitlabClient.createdBodies) != 1 {
		t.Fatalf("created bodies = %d", len(gitlabClient.createdBodies))
	}
	assertContains(t, gitlabClient.createdBodies[0], "## ⚠️ ProtoRadar Breaking Change Report")
	assertContains(t, gitlabClient.createdBodies[0], "| Status | check failed |")
	assertContains(t, gitlabClient.createdBodies[0], "breaking check failed: buf failed")
	assertStatusStates(t, gitlabClient.statuses, gitlab.CommitStatusStateRunning, gitlab.CommitStatusStateFailed)
}

func TestRunnerUpdatesErrorCommentWhenMarkerExists(t *testing.T) {
	runner, breaking, gitlabClient := newTestRunner()
	breaking.err = errors.New("buf failed")
	gitlabClient.notes = []gitlab.MergeRequestNote{{ID: 8, Body: gitlabmr.BuildMarker("user-api") + "\nold report"}}

	result, err := runner.Run(context.Background(), validInput())
	if err == nil || result.ExitCode != ExitCodeError {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	if len(gitlabClient.createdBodies) != 0 || len(gitlabClient.updatedBodies) != 1 || gitlabClient.updatedNoteIDs[0] != 8 {
		t.Fatalf("created=%d updated=%d ids=%#v", len(gitlabClient.createdBodies), len(gitlabClient.updatedBodies), gitlabClient.updatedNoteIDs)
	}
	assertContains(t, gitlabClient.updatedBodies[0], "## ⚠️ ProtoRadar Breaking Change Report")
}

func TestRunnerDoesNotCreateDuplicateCommentsOnRepeatedFailures(t *testing.T) {
	runner, breaking, gitlabClient := newTestRunner()
	breaking.err = errors.New("buf failed")

	first, err := runner.Run(context.Background(), validInput())
	if err == nil || first.NoteID == 0 {
		t.Fatalf("first result=%#v err=%v", first, err)
	}
	gitlabClient.notes = []gitlab.MergeRequestNote{{ID: first.NoteID, Body: gitlabClient.createdBodies[0]}}

	second, err := runner.Run(context.Background(), validInput())
	if err == nil || second.CommentAction != "updated" || len(gitlabClient.createdBodies) != 1 || len(gitlabClient.updatedBodies) != 1 {
		t.Fatalf("second=%#v err=%v created=%d updated=%d", second, err, len(gitlabClient.createdBodies), len(gitlabClient.updatedBodies))
	}
}

func TestGitLabNoteFailureDoesNotGetHiddenByStatusFailure(t *testing.T) {
	runner, breaking, gitlabClient := newTestRunner()
	breaking.report = passedReport()
	gitlabClient.createErr = errors.New("create failed")
	gitlabClient.failStatusOnCall = 2

	result, err := runner.Run(context.Background(), validInput())
	if err == nil || result.ExitCode != ExitCodeError {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	if !strings.Contains(err.Error(), "create merge request note") || strings.Contains(err.Error(), "status failed") {
		t.Fatalf("error precedence wrong: %v", err)
	}
	assertStatusStates(t, gitlabClient.statuses, gitlab.CommitStatusStateRunning, gitlab.CommitStatusStateFailed)
}

func TestErrorCommentFailurePreservesOriginalToolError(t *testing.T) {
	runner, breaking, gitlabClient := newTestRunner()
	breaking.err = errors.New("buf failed")
	gitlabClient.createErr = errors.New("create failed")

	result, err := runner.Run(context.Background(), validInput())
	if err == nil || result.ExitCode != ExitCodeError {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	if !strings.Contains(err.Error(), "breaking check failed: buf failed") || !strings.Contains(err.Error(), "error report comment failed") {
		t.Fatalf("error = %v", err)
	}
}

func TestTokensAreNotPresentInRenderedMarkdownOrErrors(t *testing.T) {
	runner, breaking, gitlabClient := newTestRunner()
	breaking.err = errors.New("failed with PROTORADAR_TOKEN=prr_supersecret and glpat-secret-token")
	gitlabClient.createErr = errors.New("gitlab token glpat-other-secret")

	result, err := runner.Run(context.Background(), validInput())
	if err == nil || result.ExitCode != ExitCodeError {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	for _, forbidden := range []string{"prr_supersecret", "glpat-secret-token", "glpat-other-secret"} {
		if strings.Contains(result.Markdown, forbidden) {
			t.Fatalf("markdown leaked %q:\n%s", forbidden, result.Markdown)
		}
		if strings.Contains(err.Error(), forbidden) {
			t.Fatalf("error leaked %q: %v", forbidden, err)
		}
	}
}

func newTestRunner() (Runner, *fakeBreakingClient, *fakeGitLabClient) {
	breaking := &fakeBreakingClient{}
	gitlabClient := &fakeGitLabClient{}
	return Runner{BreakingClient: breaking, GitLabClient: gitlabClient}, breaking, gitlabClient
}

func validInput() Input {
	return Input{
		Module:              "user-api",
		Against:             "latest",
		TargetRef:           "abc123",
		ArtifactName:        "source.tar.gz",
		Artifact:            []byte("artifact"),
		GitLabProjectID:     123,
		MergeRequestIID:     7,
		CommitSHA:           "abc123",
		StatusEnabled:       true,
		StatusName:          "protoradar/breaking-check",
		StatusTargetURL:     "https://gitlab.example.com/job/1",
		MaxDisplayedChanges: 50,
	}
}

func passedReport() BreakingReport {
	return BreakingReport{
		ID:          "report-1",
		Module:      "user-api",
		Against:     "v1.0.0",
		TargetRef:   "abc123",
		Status:      "passed",
		ChangeCount: 0,
	}
}

func breakingReport() BreakingReport {
	return BreakingReport{
		ID:          "report-2",
		Module:      "user-api",
		Against:     "v1.0.0",
		TargetRef:   "abc123",
		Status:      "breaking",
		ChangeCount: 1,
		Changes: []BreakingChange{{
			FilePath: "user/v1/user.proto",
			Symbol:   "user.v1.User.email",
			RuleID:   "FIELD_SAME_TYPE",
			Message:  "field changed type",
		}},
	}
}

type fakeBreakingClient struct {
	report BreakingReport
	err    error
	calls  int
}

func (fake *fakeBreakingClient) CheckBreaking(ctx context.Context, module string, against string, targetRef string, artifactName string, artifact []byte) (BreakingReport, error) {
	fake.calls++
	if fake.err != nil {
		return BreakingReport{}, fake.err
	}
	return fake.report, nil
}

type fakeAffectedModulesClient struct {
	items   []AffectedModule
	err     error
	calls   int
	modules []string
}

func (fake *fakeAffectedModulesClient) ListAffectedModules(ctx context.Context, module string) ([]AffectedModule, error) {
	fake.calls++
	fake.modules = append(fake.modules, module)
	if fake.err != nil {
		return nil, fake.err
	}
	return fake.items, nil
}

type fakeRuntimeImpactClient struct {
	items     []RuntimeImpact
	err       error
	calls     int
	reportIDs []string
}

func (fake *fakeRuntimeImpactClient) ListRuntimeImpact(ctx context.Context, reportID string) ([]RuntimeImpact, error) {
	fake.calls++
	fake.reportIDs = append(fake.reportIDs, reportID)
	if fake.err != nil {
		return nil, fake.err
	}
	return fake.items, nil
}

type fakeGovernanceClient struct {
	getStatus    GovernanceStatus
	createStatus GovernanceStatus
	getErr       error
	createErr    error
	getCalls     int
	createCalls  int
	reportIDs    []string
	createActors []string
}

func (fake *fakeGovernanceClient) GetApprovalStatus(ctx context.Context, reportID string) (GovernanceStatus, error) {
	fake.getCalls++
	fake.reportIDs = append(fake.reportIDs, reportID)
	if fake.getErr != nil {
		return GovernanceStatus{}, fake.getErr
	}
	return fake.getStatus, nil
}

func (fake *fakeGovernanceClient) CreateApprovalRequest(ctx context.Context, reportID string, actor string) (GovernanceStatus, error) {
	fake.createCalls++
	fake.reportIDs = append(fake.reportIDs, reportID)
	fake.createActors = append(fake.createActors, actor)
	if fake.createErr != nil {
		return GovernanceStatus{}, fake.createErr
	}
	return fake.createStatus, nil
}

type fakeGitLabClient struct {
	notes            []gitlab.MergeRequestNote
	createdBodies    []string
	updatedBodies    []string
	updatedNoteIDs   []int64
	statuses         []gitlab.CommitStatus
	createErr        error
	updateErr        error
	listErr          error
	statusErr        error
	failStatusOnCall int
}

func (fake *fakeGitLabClient) ListMergeRequestNotes(ctx context.Context, projectID int64, mergeRequestIID int64) ([]gitlab.MergeRequestNote, error) {
	if fake.listErr != nil {
		return nil, fake.listErr
	}
	return fake.notes, nil
}

func (fake *fakeGitLabClient) CreateMergeRequestNote(ctx context.Context, projectID int64, mergeRequestIID int64, body string) (gitlab.MergeRequestNote, error) {
	if fake.createErr != nil {
		return gitlab.MergeRequestNote{}, fake.createErr
	}
	fake.createdBodies = append(fake.createdBodies, body)
	note := gitlab.MergeRequestNote{ID: int64(len(fake.createdBodies)), Body: body}
	return note, nil
}

func (fake *fakeGitLabClient) UpdateMergeRequestNote(ctx context.Context, projectID int64, mergeRequestIID int64, noteID int64, body string) (gitlab.MergeRequestNote, error) {
	if fake.updateErr != nil {
		return gitlab.MergeRequestNote{}, fake.updateErr
	}
	fake.updatedNoteIDs = append(fake.updatedNoteIDs, noteID)
	fake.updatedBodies = append(fake.updatedBodies, body)
	return gitlab.MergeRequestNote{ID: noteID, Body: body}, nil
}

func (fake *fakeGitLabClient) SetCommitStatus(ctx context.Context, projectID int64, sha string, status gitlab.CommitStatus) error {
	fake.statuses = append(fake.statuses, status)
	if fake.statusErr != nil {
		return fake.statusErr
	}
	if fake.failStatusOnCall > 0 && len(fake.statuses) == fake.failStatusOnCall {
		return errors.New("status failed")
	}
	return nil
}

func assertStatusStates(t *testing.T, statuses []gitlab.CommitStatus, want ...gitlab.CommitStatusState) {
	t.Helper()
	if len(statuses) != len(want) {
		t.Fatalf("statuses = %#v, want %d entries", statuses, len(want))
	}
	for index, status := range statuses {
		if status.State != want[index] {
			t.Fatalf("status[%d] = %q, want %q", index, status.State, want[index])
		}
	}
}

func assertContains(t *testing.T, output string, want string) {
	t.Helper()
	if !strings.Contains(output, want) {
		t.Fatalf("output missing %q:\n%s", want, output)
	}
}
