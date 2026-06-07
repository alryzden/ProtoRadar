package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/alryzden/ProtoRadar/internal/cli/api"
	"github.com/alryzden/ProtoRadar/internal/cli/config"
	"github.com/alryzden/ProtoRadar/internal/infrastructure/gitlabapi"
	mrcheck "github.com/alryzden/ProtoRadar/internal/usecase/gitlabmr"
)

func (app App) gitlab(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("gitlab requires a subcommand")
	}
	if isHelp(args[0]) {
		app.printHelp("gitlab")
		return nil
	}
	switch args[0] {
	case "mr-check":
		return app.gitlabMRCheck(ctx, args[1:])
	default:
		return fmt.Errorf("unknown gitlab subcommand %q", args[0])
	}
}

func (app App) gitlabMRCheck(ctx context.Context, args []string) error {
	if hasHelp(args) {
		app.printHelp("gitlab mr-check")
		return nil
	}
	flags := flag.NewFlagSet("gitlab mr-check", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	module := flags.String("module", envDefault("PROTORADAR_MODULE", ""), "ProtoRadar module name")
	path := flags.String("path", envDefault("PROTORADAR_PROTO_PATH", "."), "Buf workspace root")
	against := flags.String("against", envDefault("PROTORADAR_AGAINST", "latest"), "baseline version or latest")
	targetRef := flags.String("target-ref", envDefault("CI_COMMIT_SHA", ""), "target reference")
	gitLabBaseURL := flags.String("gitlab-base-url", envDefault("CI_SERVER_URL", ""), "GitLab base URL")
	projectID := flags.String("project-id", envDefault("CI_PROJECT_ID", ""), "GitLab project ID")
	mergeRequestIID := flags.String("merge-request-iid", envDefault("CI_MERGE_REQUEST_IID", ""), "GitLab merge request IID")
	commitSHA := flags.String("commit-sha", envDefault("CI_COMMIT_SHA", ""), "GitLab commit SHA")
	gitLabToken := flags.String("gitlab-token", gitLabTokenEnvDefault(), "GitLab token")
	statusEnabled := flags.Bool("status", true, "set GitLab commit status")
	statusName := flags.String("status-name", mrcheck.DefaultStatusName, "GitLab commit status name")
	statusTargetURL := flags.String("status-target-url", envDefault("CI_JOB_URL", ""), "GitLab commit status target URL")
	reportFile := flags.String("report-file", "", "Markdown report file")
	governanceEnabled := flags.Bool("governance", false, "allow approved governance requests to pass breaking checks")
	governanceActor := flags.String("governance-actor", "", "deprecated: only sent when provided and only works when server actor override is enabled")
	if err := flags.Parse(flagsFirst(args, map[string]bool{"status": true})); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("gitlab mr-check does not accept positional arguments")
	}

	input, err := resolveGitLabMRCheckInput(gitLabMRCheckValues{
		Module:          *module,
		Path:            *path,
		Against:         *against,
		TargetRef:       *targetRef,
		GitLabBaseURL:   *gitLabBaseURL,
		ProjectID:       *projectID,
		MergeRequestIID: *mergeRequestIID,
		CommitSHA:       *commitSHA,
		GitLabToken:     *gitLabToken,
		StatusEnabled:   *statusEnabled,
		StatusName:      *statusName,
		StatusTargetURL: *statusTargetURL,
	})
	if err != nil {
		return err
	}

	artifact, err := createArtifact(input.Path)
	if err != nil {
		return err
	}
	configPath, err := app.configPath()
	if err != nil {
		return err
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}
	httpClient, err := app.httpClient(cfg)
	if err != nil {
		return err
	}
	protoRadarClient, err := newAPIClientFromConfig(cfg, httpClient)
	if err != nil {
		return err
	}
	runner := mrcheck.Runner{
		BreakingClient:        breakingClientAdapter{client: protoRadarClient},
		AffectedModulesClient: affectedModulesClientAdapter{client: protoRadarClient},
		RuntimeImpactClient:   runtimeImpactClientAdapter{client: protoRadarClient},
		GovernanceClient:      governanceStatusClientAdapter{client: protoRadarClient},
		GitLabClient:          gitlabapi.NewClient(input.GitLabBaseURL, input.GitLabToken, httpClient),
	}
	result, runErr := runner.Run(ctx, mrcheck.Input{
		Module:            input.Module,
		Against:           input.Against,
		TargetRef:         input.TargetRef,
		ArtifactName:      "source.tar.gz",
		Artifact:          artifact.Body,
		GitLabProjectID:   input.ProjectID,
		MergeRequestIID:   input.MergeRequestIID,
		CommitSHA:         input.CommitSHA,
		StatusEnabled:     input.StatusEnabled,
		StatusName:        input.StatusName,
		StatusTargetURL:   input.StatusTargetURL,
		GovernanceEnabled: *governanceEnabled,
		GovernanceActor:   optionalGitLabGovernanceActor(flags, *governanceActor),
	})
	if strings.TrimSpace(*reportFile) != "" && strings.TrimSpace(result.Markdown) != "" {
		if err := os.WriteFile(strings.TrimSpace(*reportFile), []byte(result.Markdown+"\n"), 0o600); err != nil {
			return ExitError{Code: mrcheck.ExitCodeError, Err: fmt.Errorf("write report file: %w", err)}
		}
	}
	if runErr != nil {
		return ExitError{Code: mrcheck.ExitCodeError, Err: runErr}
	}

	writeMRCheckResult(app.output(), input.Module, result)
	if result.ExitCode == mrcheck.ExitCodeBreaking {
		return ExitError{Code: mrcheck.ExitCodeBreaking}
	}
	if result.ExitCode == mrcheck.ExitCodeError {
		return ExitError{Code: mrcheck.ExitCodeError, Err: errors.New("gitlab mr-check failed")}
	}
	return nil
}

type gitLabMRCheckValues struct {
	Module          string
	Path            string
	Against         string
	TargetRef       string
	GitLabBaseURL   string
	ProjectID       string
	MergeRequestIID string
	CommitSHA       string
	GitLabToken     string
	StatusEnabled   bool
	StatusName      string
	StatusTargetURL string
}

type gitLabMRCheckInput struct {
	Module          string
	Path            string
	Against         string
	TargetRef       string
	GitLabBaseURL   string
	ProjectID       int64
	MergeRequestIID int64
	CommitSHA       string
	GitLabToken     string
	StatusEnabled   bool
	StatusName      string
	StatusTargetURL string
}

func resolveGitLabMRCheckInput(values gitLabMRCheckValues) (gitLabMRCheckInput, error) {
	input := gitLabMRCheckInput{
		Module:          strings.TrimSpace(values.Module),
		Path:            strings.TrimSpace(values.Path),
		Against:         strings.TrimSpace(values.Against),
		TargetRef:       strings.TrimSpace(values.TargetRef),
		GitLabBaseURL:   strings.TrimSpace(values.GitLabBaseURL),
		CommitSHA:       strings.TrimSpace(values.CommitSHA),
		GitLabToken:     strings.TrimSpace(values.GitLabToken),
		StatusEnabled:   values.StatusEnabled,
		StatusName:      strings.TrimSpace(values.StatusName),
		StatusTargetURL: strings.TrimSpace(values.StatusTargetURL),
	}
	if input.Module == "" {
		return gitLabMRCheckInput{}, errors.New("gitlab mr-check requires --module")
	}
	if input.Path == "" {
		return gitLabMRCheckInput{}, errors.New("gitlab mr-check requires --path")
	}
	if input.Against == "" {
		return gitLabMRCheckInput{}, errors.New("gitlab mr-check requires --against")
	}
	if input.TargetRef == "" {
		input.TargetRef = input.CommitSHA
	}
	if input.GitLabBaseURL == "" {
		return gitLabMRCheckInput{}, errors.New("gitlab mr-check requires --gitlab-base-url")
	}
	if err := validateServerURL(input.GitLabBaseURL); err != nil {
		return gitLabMRCheckInput{}, fmt.Errorf("invalid GitLab base URL: %w", err)
	}
	if strings.TrimSpace(values.ProjectID) == "" {
		return gitLabMRCheckInput{}, errors.New("gitlab mr-check requires --project-id")
	}
	projectID, err := parsePositiveInt64(values.ProjectID, "--project-id")
	if err != nil {
		return gitLabMRCheckInput{}, err
	}
	input.ProjectID = projectID
	if strings.TrimSpace(values.MergeRequestIID) == "" {
		return gitLabMRCheckInput{}, errors.New("gitlab mr-check requires --merge-request-iid")
	}
	mergeRequestIID, err := parsePositiveInt64(values.MergeRequestIID, "--merge-request-iid")
	if err != nil {
		return gitLabMRCheckInput{}, err
	}
	input.MergeRequestIID = mergeRequestIID
	if input.CommitSHA == "" {
		return gitLabMRCheckInput{}, errors.New("gitlab mr-check requires --commit-sha")
	}
	if input.GitLabToken == "" {
		return gitLabMRCheckInput{}, errors.New("gitlab mr-check requires --gitlab-token, PROTORADAR_GITLAB_TOKEN, or GITLAB_TOKEN")
	}
	if input.StatusName == "" {
		input.StatusName = mrcheck.DefaultStatusName
	}
	return input, nil
}

func parsePositiveInt64(value string, name string) (int64, error) {
	parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("gitlab mr-check requires %s greater than zero", name)
	}
	return parsed, nil
}

func writeMRCheckResult(out io.Writer, module string, result mrcheck.Result) {
	switch result.ExitCode {
	case mrcheck.ExitCodePassed:
		fmt.Fprintf(out, "ProtoRadar MR check passed for %s\n", module)
	case mrcheck.ExitCodeBreaking:
		fmt.Fprintf(out, "ProtoRadar MR check found breaking changes for %s\n", module)
	default:
		fmt.Fprintf(out, "ProtoRadar MR check failed for %s\n", module)
	}
	if result.CommentAction != "" {
		fmt.Fprintf(out, "Merge request comment %s\n", result.CommentAction)
	}
}

func envDefault(name string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	return value
}

func gitLabTokenEnvDefault() string {
	if value := envDefault("PROTORADAR_GITLAB_TOKEN", ""); value != "" {
		return value
	}
	return envDefault("GITLAB_TOKEN", "")
}

type breakingClientAdapter struct {
	client *api.Client
}

func (adapter breakingClientAdapter) CheckBreaking(ctx context.Context, module string, against string, targetRef string, artifactName string, artifact []byte) (mrcheck.BreakingReport, error) {
	report, err := adapter.client.CheckBreaking(ctx, module, against, targetRef, artifactName, artifact)
	if err != nil {
		return mrcheck.BreakingReport{}, err
	}
	changes := make([]mrcheck.BreakingChange, 0, len(report.Changes))
	for _, change := range report.Changes {
		changes = append(changes, mrcheck.BreakingChange{
			Category:    change.Category,
			FilePath:    change.FilePath,
			PackageName: change.PackageName,
			Symbol:      change.Symbol,
			RuleID:      change.RuleID,
			Message:     change.Message,
			Severity:    change.Severity,
		})
	}
	return mrcheck.BreakingReport{
		ID:           report.ID,
		Module:       report.Module,
		Against:      report.Against,
		TargetRef:    report.TargetRef,
		Status:       report.Status,
		ChangeCount:  report.ChangeCount,
		Changes:      changes,
		HumanSummary: report.HumanSummary,
		CreatedAt:    report.CreatedAt,
	}, nil
}

type affectedModulesClientAdapter struct {
	client *api.Client
}

func (adapter affectedModulesClientAdapter) ListAffectedModules(ctx context.Context, module string) ([]mrcheck.AffectedModule, error) {
	affected, err := adapter.client.GetAffectedModules(ctx, module)
	if err != nil {
		return nil, err
	}
	items := make([]mrcheck.AffectedModule, 0, len(affected.AffectedModules))
	for _, item := range affected.AffectedModules {
		items = append(items, mrcheck.AffectedModule{
			Module:            item.Module,
			LatestVersion:     item.LatestVersion,
			DependencySources: append([]string(nil), item.DependencySources...),
			Reasons:           append([]string(nil), item.Reasons...),
		})
	}
	return items, nil
}

type runtimeImpactClientAdapter struct {
	client *api.Client
}

func (adapter runtimeImpactClientAdapter) ListRuntimeImpact(ctx context.Context, reportID string) ([]mrcheck.RuntimeImpact, error) {
	impact, err := adapter.client.GetBreakingReportRuntimeImpact(ctx, reportID)
	if err != nil {
		return nil, err
	}
	items := make([]mrcheck.RuntimeImpact, 0, len(impact.Impacts))
	for _, item := range impact.Impacts {
		items = append(items, mrcheck.RuntimeImpact{
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
	return items, nil
}

type governanceStatusClientAdapter struct {
	client *api.Client
}

func (adapter governanceStatusClientAdapter) GetApprovalStatus(ctx context.Context, reportID string) (mrcheck.GovernanceStatus, error) {
	request, err := adapter.client.GetApprovalStatus(ctx, reportID)
	if errors.Is(err, api.ErrNotFound) {
		return mrcheck.GovernanceStatus{}, mrcheck.ErrGovernanceNotFound
	}
	if err != nil {
		return mrcheck.GovernanceStatus{}, err
	}
	return governanceStatusFromAPI(request), nil
}

func (adapter governanceStatusClientAdapter) CreateApprovalRequest(ctx context.Context, reportID string, actor string) (mrcheck.GovernanceStatus, error) {
	request, err := adapter.client.CreateApprovalRequest(ctx, reportID, api.CreateApprovalRequestRequest{Actor: strings.TrimSpace(actor)})
	if err != nil {
		return mrcheck.GovernanceStatus{}, err
	}
	return governanceStatusFromAPI(request), nil
}

func optionalGitLabGovernanceActor(flags *flag.FlagSet, value string) string {
	if !flagProvided(flags, "governance-actor") {
		return ""
	}
	return strings.TrimSpace(value)
}

func governanceStatusFromAPI(request api.ApprovalRequest) mrcheck.GovernanceStatus {
	requirements := make([]mrcheck.GovernanceRequirement, 0, len(request.Requirements))
	for _, requirement := range request.Requirements {
		requirements = append(requirements, mrcheck.GovernanceRequirement{
			RequirementType:  requirement.RequirementType,
			TargetModuleName: requirement.TargetModuleName,
			Status:           requirement.Status,
			Reason:           requirement.Reason,
		})
	}
	decisions := make([]mrcheck.GovernanceDecision, 0, len(request.Decisions))
	for _, decision := range request.Decisions {
		decisions = append(decisions, mrcheck.GovernanceDecision{
			Decision:  decision.Decision,
			DecidedBy: decision.DecidedBy,
			Comment:   decision.Comment,
		})
	}
	return mrcheck.GovernanceStatus{
		ID:                request.ID,
		Status:            request.Status,
		Requirements:      requirements,
		Decisions:         decisions,
		RequiredApprovals: request.RequiredApprovals,
		ReceivedApprovals: request.ReceivedApprovals,
	}
}
