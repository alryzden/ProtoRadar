package gitlabmr

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/alryzden/ProtoRadar/internal/format/gitlabmr"
	"github.com/alryzden/ProtoRadar/internal/integration/gitlab"
)

const (
	ExitCodePassed   = 0
	ExitCodeBreaking = 1
	ExitCodeError    = 2

	DefaultStatusName = "protoradar/breaking-check"
)

var ErrGovernanceNotFound = errors.New("governance approval request not found")

var secretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(private[-_ ]?token|access[-_ ]?token|protoradar[-_ ]?token|gitlab[-_ ]?token|authorization|bearer|secret|password)(=|:|\s+)\s*[^\s|]+`),
	regexp.MustCompile(`glpat-[A-Za-z0-9_-]+`),
	regexp.MustCompile(`prr_[A-Za-z0-9_-]+`),
}

type BreakingClient interface {
	CheckBreaking(ctx context.Context, module string, against string, targetRef string, artifactName string, artifact []byte) (BreakingReport, error)
}

type AffectedModulesClient interface {
	ListAffectedModules(ctx context.Context, module string) ([]AffectedModule, error)
}

type RuntimeImpactClient interface {
	ListRuntimeImpact(ctx context.Context, reportID string) ([]RuntimeImpact, error)
}

type GovernanceClient interface {
	GetApprovalStatus(ctx context.Context, reportID string) (GovernanceStatus, error)
	CreateApprovalRequest(ctx context.Context, reportID string, actor string) (GovernanceStatus, error)
}

type GitLabClient interface {
	ListMergeRequestNotes(ctx context.Context, projectID int64, mergeRequestIID int64) ([]gitlab.MergeRequestNote, error)
	CreateMergeRequestNote(ctx context.Context, projectID int64, mergeRequestIID int64, body string) (gitlab.MergeRequestNote, error)
	UpdateMergeRequestNote(ctx context.Context, projectID int64, mergeRequestIID int64, noteID int64, body string) (gitlab.MergeRequestNote, error)
	SetCommitStatus(ctx context.Context, projectID int64, sha string, status gitlab.CommitStatus) error
}

type BreakingReport struct {
	ID           string
	Module       string
	Against      string
	TargetRef    string
	Status       string
	ChangeCount  int
	Changes      []BreakingChange
	HumanSummary string
	CreatedAt    time.Time
}

type BreakingChange struct {
	Category    string
	FilePath    string
	PackageName string
	Symbol      string
	RuleID      string
	Message     string
	Severity    string
}

type AffectedModule struct {
	Module            string
	LatestVersion     string
	DependencySources []string
	Reasons           []string
}

type RuntimeImpact struct {
	ServiceName  string
	Environment  string
	UsedModule   string
	UsedVersion  string
	BuildVersion string
	GitCommit    string
	DriftStatus  string
	DriftReason  string
}

type GovernanceStatus struct {
	ID                string
	Status            string
	Requirements      []GovernanceRequirement
	Decisions         []GovernanceDecision
	RequiredApprovals int
	ReceivedApprovals int
}

type GovernanceRequirement struct {
	RequirementType  string
	TargetModuleName string
	Status           string
	Reason           string
}

type GovernanceDecision struct {
	Decision  string
	DecidedBy string
	Comment   string
}

type Runner struct {
	BreakingClient        BreakingClient
	AffectedModulesClient AffectedModulesClient
	RuntimeImpactClient   RuntimeImpactClient
	GovernanceClient      GovernanceClient
	GitLabClient          GitLabClient
}

type Input struct {
	Module              string
	Against             string
	TargetRef           string
	ArtifactName        string
	Artifact            []byte
	GitLabProjectID     int64
	MergeRequestIID     int64
	CommitSHA           string
	StatusEnabled       bool
	StatusName          string
	StatusTargetURL     string
	MaxDisplayedChanges int
	GovernanceEnabled   bool
	GovernanceActor     string
}

type Result struct {
	ExitCode       int
	BreakingStatus string
	Markdown       string
	NoteID         int64
	CommentAction  string
}

func (runner Runner) Run(ctx context.Context, input Input) (Result, error) {
	if err := runner.validate(input); err != nil {
		return Result{ExitCode: ExitCodeError}, err
	}

	statusName := strings.TrimSpace(input.StatusName)
	if statusName == "" {
		statusName = DefaultStatusName
	}

	if input.StatusEnabled {
		if err := runner.setStatus(ctx, input, statusName, gitlab.CommitStatusStateRunning, "ProtoRadar breaking check is running"); err != nil {
			return Result{ExitCode: ExitCodeError}, fmt.Errorf("set running commit status: %w", sanitizeError(err))
		}
	}

	report, err := runner.BreakingClient.CheckBreaking(ctx, strings.TrimSpace(input.Module), strings.TrimSpace(input.Against), strings.TrimSpace(input.TargetRef), artifactName(input), input.Artifact)
	if err != nil {
		return runner.handleError(ctx, input, statusName, fmt.Errorf("breaking check failed: %w", sanitizeError(err)))
	}

	affected := runner.bestEffortAffectedModules(ctx, input)
	runtimeImpacts, runtimeImpactUnavailable := runner.fetchRuntimeImpact(ctx, report)
	governanceStatus, err := runner.fetchGovernanceStatus(ctx, input, report)
	if err != nil {
		return runner.handleError(ctx, input, statusName, fmt.Errorf("governance approval status failed: %w", sanitizeError(err)))
	}

	markdown := gitlabmr.RenderReport(renderReportFromBreaking(input, report, affected, runtimeImpacts, runtimeImpactUnavailable, governanceStatus), gitlabmr.RenderOptions{MaxDisplayedChanges: input.MaxDisplayedChanges})
	note, action, err := runner.upsertComment(ctx, input, markdown)
	if err != nil {
		if input.StatusEnabled {
			runner.bestEffortSetStatus(ctx, input, statusName, gitlab.CommitStatusStateFailed, "ProtoRadar failed to publish the merge request report")
		}
		return Result{ExitCode: ExitCodeError, BreakingStatus: report.Status, Markdown: markdown}, sanitizeError(err)
	}

	state, description, exitCode := runner.finalState(input, report, governanceStatus)
	if input.StatusEnabled {
		if err := runner.setStatus(ctx, input, statusName, state, description); err != nil {
			return Result{ExitCode: ExitCodeError, BreakingStatus: report.Status, Markdown: markdown, NoteID: note.ID, CommentAction: action}, fmt.Errorf("set final commit status: %w", sanitizeError(err))
		}
	}

	return Result{ExitCode: exitCode, BreakingStatus: report.Status, Markdown: markdown, NoteID: note.ID, CommentAction: action}, nil
}

func (runner Runner) validate(input Input) error {
	if runner.BreakingClient == nil {
		return errors.New("gitlab mr runner requires a breaking client")
	}
	if runner.GitLabClient == nil {
		return errors.New("gitlab mr runner requires a gitlab client")
	}
	if strings.TrimSpace(input.Module) == "" {
		return errors.New("gitlab mr runner requires module")
	}
	if strings.TrimSpace(input.Against) == "" {
		return errors.New("gitlab mr runner requires against")
	}
	if len(input.Artifact) == 0 {
		return errors.New("gitlab mr runner requires artifact")
	}
	if input.GitLabProjectID <= 0 {
		return errors.New("gitlab mr runner requires gitlab project id")
	}
	if input.MergeRequestIID <= 0 {
		return errors.New("gitlab mr runner requires merge request iid")
	}
	if input.StatusEnabled && strings.TrimSpace(input.CommitSHA) == "" {
		return errors.New("gitlab mr runner requires commit sha when status is enabled")
	}
	return nil
}

func (runner Runner) handleError(ctx context.Context, input Input, statusName string, cause error) (Result, error) {
	markdown := gitlabmr.RenderReport(gitlabmr.Report{
		Module:      input.Module,
		Against:     input.Against,
		TargetRef:   input.TargetRef,
		Status:      "check failed",
		ChangeCount: 0,
		Changes: []gitlabmr.Change{{
			Message: cause.Error(),
		}},
		TargetURL: input.StatusTargetURL,
		CommitSHA: input.CommitSHA,
	}, gitlabmr.RenderOptions{MaxDisplayedChanges: input.MaxDisplayedChanges})

	result := Result{ExitCode: ExitCodeError, BreakingStatus: "error", Markdown: markdown}
	note, action, noteErr := runner.upsertComment(ctx, input, markdown)
	if noteErr == nil {
		result.NoteID = note.ID
		result.CommentAction = action
	}
	if input.StatusEnabled {
		runner.bestEffortSetStatus(ctx, input, statusName, gitlab.CommitStatusStateFailed, "ProtoRadar breaking check failed")
	}
	if noteErr != nil {
		return result, fmt.Errorf("%w; error report comment failed: %w", cause, sanitizeError(noteErr))
	}
	return result, cause
}

func (runner Runner) upsertComment(ctx context.Context, input Input, markdown string) (gitlab.MergeRequestNote, string, error) {
	notes, err := runner.GitLabClient.ListMergeRequestNotes(ctx, input.GitLabProjectID, input.MergeRequestIID)
	if err != nil {
		return gitlab.MergeRequestNote{}, "", fmt.Errorf("list merge request notes: %w", sanitizeError(err))
	}
	if existing, ok := gitlabmr.SelectExistingNote(notes, input.Module); ok {
		note, err := runner.GitLabClient.UpdateMergeRequestNote(ctx, input.GitLabProjectID, input.MergeRequestIID, existing.ID, markdown)
		if err != nil {
			return gitlab.MergeRequestNote{}, "", fmt.Errorf("update merge request note: %w", sanitizeError(err))
		}
		return note, "updated", nil
	}
	note, err := runner.GitLabClient.CreateMergeRequestNote(ctx, input.GitLabProjectID, input.MergeRequestIID, markdown)
	if err != nil {
		return gitlab.MergeRequestNote{}, "", fmt.Errorf("create merge request note: %w", sanitizeError(err))
	}
	return note, "created", nil
}

func (runner Runner) setStatus(ctx context.Context, input Input, statusName string, state gitlab.CommitStatusState, description string) error {
	return runner.GitLabClient.SetCommitStatus(ctx, input.GitLabProjectID, strings.TrimSpace(input.CommitSHA), gitlab.CommitStatus{
		State:       state,
		Name:        statusName,
		TargetURL:   strings.TrimSpace(input.StatusTargetURL),
		Description: description,
	})
}

func (runner Runner) fetchAffectedModules(ctx context.Context, input Input) ([]AffectedModule, error) {
	if runner.AffectedModulesClient == nil {
		return []AffectedModule{}, nil
	}
	return runner.AffectedModulesClient.ListAffectedModules(ctx, strings.TrimSpace(input.Module))
}

func (runner Runner) bestEffortAffectedModules(ctx context.Context, input Input) []AffectedModule {
	affected, err := runner.fetchAffectedModules(ctx, input)
	if err != nil {
		return []AffectedModule{}
	}
	return affected
}

func (runner Runner) bestEffortSetStatus(ctx context.Context, input Input, statusName string, state gitlab.CommitStatusState, description string) {
	// Status updates on failure paths are best-effort; the primary MR check
	// result/comment error is returned to the caller.
	_ = runner.setStatus(ctx, input, statusName, state, description) //nolint:errcheck
}

func (runner Runner) fetchRuntimeImpact(ctx context.Context, report BreakingReport) ([]RuntimeImpact, bool) {
	if runner.RuntimeImpactClient == nil || strings.TrimSpace(report.ID) == "" {
		return []RuntimeImpact{}, false
	}
	impacts, err := runner.RuntimeImpactClient.ListRuntimeImpact(ctx, strings.TrimSpace(report.ID))
	if err != nil {
		return []RuntimeImpact{}, true
	}
	return impacts, false
}

func (runner Runner) fetchGovernanceStatus(ctx context.Context, input Input, report BreakingReport) (*GovernanceStatus, error) {
	if runner.GovernanceClient == nil || strings.TrimSpace(report.ID) == "" {
		return governanceStatusUnavailable()
	}
	status, err := runner.GovernanceClient.GetApprovalStatus(ctx, strings.TrimSpace(report.ID))
	if errors.Is(err, ErrGovernanceNotFound) {
		if !input.GovernanceEnabled {
			return governanceStatusUnavailable()
		}
		status, err = runner.GovernanceClient.CreateApprovalRequest(ctx, strings.TrimSpace(report.ID), strings.TrimSpace(input.GovernanceActor))
	}
	if err != nil {
		if !input.GovernanceEnabled {
			return governanceStatusUnavailable()
		}
		if strings.TrimSpace(input.GovernanceActor) != "" {
			return nil, fmt.Errorf("server rejected governance actor override; omit --governance-actor so the server uses the authenticated ProtoRadar principal, or enable server actor override for migration compatibility: %w", err)
		}
		return nil, err
	}
	return &status, nil
}

func governanceStatusUnavailable() (*GovernanceStatus, error) {
	// Nil status with nil error means governance is optional or disabled for this
	// MR check, so rendering should omit the governance section.
	return nil, nil //nolint:nilnil
}

func (runner Runner) finalState(input Input, report BreakingReport, governance *GovernanceStatus) (gitlab.CommitStatusState, string, int) {
	if !input.GovernanceEnabled {
		if normalizedStatus(report.Status) == "breaking" {
			return gitlab.CommitStatusStateFailed, "ProtoRadar found protobuf breaking changes", ExitCodeBreaking
		}
		return gitlab.CommitStatusStateSuccess, "No protobuf breaking changes found", ExitCodePassed
	}
	switch normalizedGovernanceStatus(governance) {
	case "approved":
		return gitlab.CommitStatusStateSuccess, "ProtoRadar breaking changes approved", ExitCodePassed
	case "pending":
		return gitlab.CommitStatusStateFailed, "ProtoRadar breaking changes require governance approval", ExitCodeBreaking
	case "rejected":
		return gitlab.CommitStatusStateFailed, "ProtoRadar breaking changes were rejected", ExitCodeBreaking
	case "not_required":
		return gitlab.CommitStatusStateSuccess, "Governance approval not required", ExitCodePassed
	default:
		if normalizedStatus(report.Status) == "breaking" {
			return gitlab.CommitStatusStateFailed, "ProtoRadar found protobuf breaking changes", ExitCodeBreaking
		}
		return gitlab.CommitStatusStateSuccess, "No protobuf breaking changes found", ExitCodePassed
	}
}

func renderReportFromBreaking(input Input, report BreakingReport, affected []AffectedModule, runtimeImpacts []RuntimeImpact, runtimeImpactUnavailable bool, governance *GovernanceStatus) gitlabmr.Report {
	module := report.Module
	if strings.TrimSpace(module) == "" {
		module = input.Module
	}
	against := report.Against
	if strings.TrimSpace(against) == "" {
		against = input.Against
	}
	targetRef := report.TargetRef
	if strings.TrimSpace(targetRef) == "" {
		targetRef = input.TargetRef
	}
	changes := make([]gitlabmr.Change, 0, len(report.Changes))
	for _, change := range report.Changes {
		changes = append(changes, gitlabmr.Change{
			FilePath: change.FilePath,
			Symbol:   change.Symbol,
			RuleID:   change.RuleID,
			Message:  change.Message,
		})
	}
	affectedModules := make([]gitlabmr.AffectedModule, 0, len(affected))
	for _, item := range affected {
		affectedModules = append(affectedModules, gitlabmr.AffectedModule{
			Module:            item.Module,
			LatestVersion:     item.LatestVersion,
			DependencySources: append([]string(nil), item.DependencySources...),
			Reasons:           append([]string(nil), item.Reasons...),
		})
	}
	renderedRuntimeImpacts := make([]gitlabmr.RuntimeImpact, 0, len(runtimeImpacts))
	for _, item := range runtimeImpacts {
		renderedRuntimeImpacts = append(renderedRuntimeImpacts, gitlabmr.RuntimeImpact{
			ServiceName:  item.ServiceName,
			Environment:  item.Environment,
			UsedModule:   item.UsedModule,
			UsedVersion:  item.UsedVersion,
			BuildVersion: item.BuildVersion,
			GitCommit:    item.GitCommit,
			DriftStatus:  item.DriftStatus,
			DriftReason:  item.DriftReason,
		})
	}
	return gitlabmr.Report{
		Module:                   module,
		Against:                  against,
		TargetRef:                targetRef,
		Status:                   report.Status,
		ChangeCount:              report.ChangeCount,
		Changes:                  changes,
		AffectedModules:          affectedModules,
		RuntimeImpacts:           renderedRuntimeImpacts,
		RuntimeImpactUnavailable: runtimeImpactUnavailable,
		Governance:               renderGovernance(governance),
		ReportID:                 report.ID,
		TargetURL:                input.StatusTargetURL,
		CommitSHA:                input.CommitSHA,
	}
}

func renderGovernance(governance *GovernanceStatus) *gitlabmr.Governance {
	if governance == nil {
		return nil
	}
	requirements := make([]gitlabmr.GovernanceRequirement, 0, len(governance.Requirements))
	warnings := make([]string, 0)
	for _, requirement := range governance.Requirements {
		requirements = append(requirements, gitlabmr.GovernanceRequirement{
			RequirementType:  requirement.RequirementType,
			TargetModuleName: requirement.TargetModuleName,
			Status:           requirement.Status,
			Reason:           requirement.Reason,
		})
		if strings.Contains(strings.ToLower(requirement.Reason), "no owners are configured") {
			target := strings.TrimSpace(requirement.TargetModuleName)
			if target == "" {
				target = "this module"
			}
			warnings = append(warnings, "Approval required, but no owners are configured for `"+target+"`.")
		}
	}
	decisions := make([]gitlabmr.GovernanceDecision, 0, len(governance.Decisions))
	for _, decision := range governance.Decisions {
		decisions = append(decisions, gitlabmr.GovernanceDecision{
			Decision:  decision.Decision,
			DecidedBy: decision.DecidedBy,
			Comment:   decision.Comment,
		})
	}
	return &gitlabmr.Governance{
		Status:               governance.Status,
		ApprovalRequestID:    governance.ID,
		Requirements:         requirements,
		Decisions:            decisions,
		MissingOwnerWarnings: warnings,
	}
}

func normalizedGovernanceStatus(governance *GovernanceStatus) string {
	if governance == nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(governance.Status))
}

func artifactName(input Input) string {
	if strings.TrimSpace(input.ArtifactName) == "" {
		return "source.tar.gz"
	}
	return input.ArtifactName
}

func normalizedStatus(status string) string {
	return strings.ToLower(strings.TrimSpace(status))
}

func sanitizeError(err error) error {
	if err == nil {
		return nil
	}
	return errors.New(redactSecrets(err.Error()))
}

func redactSecrets(value string) string {
	for _, pattern := range secretPatterns {
		value = pattern.ReplaceAllString(value, "$1[redacted]")
	}
	return value
}
