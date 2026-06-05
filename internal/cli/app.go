package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/alryzden/ProtoRadar/internal/cli/api"
	"github.com/alryzden/ProtoRadar/internal/cli/config"
	"github.com/alryzden/ProtoRadar/internal/version"
)

type App struct {
	ConfigPath string
	HTTPClient *http.Client
	Out        io.Writer
	Err        io.Writer
}

func (app App) Run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return app.usage()
	}
	if isHelp(args[0]) {
		app.printHelp("root")
		return nil
	}

	switch args[0] {
	case "help":
		topic := "root"
		if len(args) > 1 {
			topic = strings.Join(args[1:], " ")
		}
		app.printHelp(topic)
		return nil
	case "login":
		return app.login(ctx, args[1:])
	case "version":
		return app.version()
	case "module":
		return app.module(ctx, args[1:])
	case "push":
		return app.push(ctx, args[1:])
	case "check-breaking":
		return app.checkBreaking(ctx, args[1:])
	case "gitlab":
		return app.gitlab(ctx, args[1:])
	case "pull":
		return app.pull(ctx, args[1:])
	case "list":
		return app.list(ctx, args[1:])
	case "runtime":
		return app.runtime(ctx, args[1:])
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func (app App) login(ctx context.Context, args []string) error {
	if hasHelp(args) {
		app.printHelp("login")
		return nil
	}
	flags := flag.NewFlagSet("login", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	serverURL := flags.String("server", "", "ProtoRadar server URL")
	token := flags.String("token", "", "API token")
	if err := flags.Parse(flagsFirst(args, nil)); err != nil {
		return err
	}

	if strings.TrimSpace(*serverURL) == "" {
		return errors.New("login requires --server")
	}
	if strings.TrimSpace(*token) == "" {
		return errors.New("login requires --token")
	}
	if err := validateServerURL(*serverURL); err != nil {
		return err
	}

	client := api.NewClient(*serverURL, *token, app.HTTPClient)
	if err := client.CheckAuth(ctx); err != nil {
		return fmt.Errorf("login failed: %w", err)
	}

	path, err := app.configPath()
	if err != nil {
		return err
	}
	if err := config.Save(path, config.Config{ServerURL: *serverURL, Token: *token}); err != nil {
		return err
	}

	fmt.Fprintln(app.output(), "Logged in to "+strings.TrimRight(*serverURL, "/"))
	return nil
}

func (app App) module(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("module requires a subcommand")
	}
	if isHelp(args[0]) {
		app.printHelp("module")
		return nil
	}
	switch args[0] {
	case "create":
		return app.createModule(ctx, args[1:])
	case "list":
		return app.list(ctx, args[1:])
	case "link-gitlab":
		return app.linkModuleGitLab(ctx, args[1:])
	case "dependencies":
		return app.moduleDependencies(ctx, args[1:])
	case "affected":
		return app.moduleAffected(ctx, args[1:])
	default:
		return fmt.Errorf("unknown module subcommand %q", args[0])
	}
}

func (app App) createModule(ctx context.Context, args []string) error {
	if hasHelp(args) {
		app.printHelp("module create")
		return nil
	}
	flags := flag.NewFlagSet("module create", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	description := flags.String("description", "", "module description")
	repositoryURL := flags.String("repository-url", "", "module repository URL")
	if err := flags.Parse(flagsFirst(args, nil)); err != nil {
		return err
	}
	if flags.NArg() != 1 {
		return errors.New("module create requires a module name")
	}

	client, err := app.client()
	if err != nil {
		return err
	}
	module, err := client.CreateModule(ctx, api.CreateModuleRequest{
		Name:          flags.Arg(0),
		Description:   *description,
		RepositoryURL: *repositoryURL,
	})
	if err != nil {
		if errors.Is(err, api.ErrConflict) {
			return fmt.Errorf("module %q already exists", flags.Arg(0))
		}
		return err
	}

	fmt.Fprintf(app.output(), "Created module %s\n", module.Name)
	return nil
}

func (app App) linkModuleGitLab(ctx context.Context, args []string) error {
	if hasHelp(args) {
		app.printHelp("module link-gitlab")
		return nil
	}
	flags := flag.NewFlagSet("module link-gitlab", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	projectID := flags.String("project-id", "", "GitLab project ID")
	projectPath := flags.String("project-path", "", "GitLab project path")
	gitLabBaseURL := flags.String("gitlab-base-url", "", "GitLab base URL")
	if err := flags.Parse(flagsFirst(args, nil)); err != nil {
		return err
	}
	if flags.NArg() != 1 {
		return errors.New("module link-gitlab requires a module name")
	}

	projectIDValue := strings.TrimSpace(*projectID)
	if projectIDValue == "" {
		projectIDValue = strings.TrimSpace(os.Getenv("CI_PROJECT_ID"))
	}
	projectPathValue := strings.TrimSpace(*projectPath)
	if projectPathValue == "" {
		projectPathValue = strings.TrimSpace(os.Getenv("CI_PROJECT_PATH"))
	}
	gitLabBaseURLValue := strings.TrimSpace(*gitLabBaseURL)
	if gitLabBaseURLValue == "" {
		gitLabBaseURLValue = strings.TrimSpace(os.Getenv("CI_SERVER_URL"))
	}

	if projectIDValue == "" {
		return errors.New("module link-gitlab requires --project-id")
	}
	parsedProjectID, err := strconv.ParseInt(projectIDValue, 10, 64)
	if err != nil || parsedProjectID <= 0 {
		return errors.New("module link-gitlab requires --project-id greater than zero")
	}
	if projectPathValue == "" {
		return errors.New("module link-gitlab requires --project-path")
	}
	if gitLabBaseURLValue == "" {
		return errors.New("module link-gitlab requires --gitlab-base-url")
	}
	if err := validateServerURL(gitLabBaseURLValue); err != nil {
		return fmt.Errorf("invalid GitLab base URL: %w", err)
	}

	client, err := app.client()
	if err != nil {
		return err
	}
	mapping, err := client.LinkModuleGitLabProject(ctx, flags.Arg(0), api.LinkModuleGitLabProjectRequest{
		GitLabBaseURL:     gitLabBaseURLValue,
		GitLabProjectID:   parsedProjectID,
		GitLabProjectPath: projectPathValue,
	})
	if err != nil {
		if errors.Is(err, api.ErrUnauthorized) {
			return errors.New("module link-gitlab failed: unauthorized")
		}
		if errors.Is(err, api.ErrNotFound) {
			return fmt.Errorf("module %q was not found", flags.Arg(0))
		}
		if errors.Is(err, api.ErrConflict) {
			return fmt.Errorf("GitLab project %s (%d) is already linked to another module", projectPathValue, parsedProjectID)
		}
		return fmt.Errorf("module link-gitlab failed: %w", err)
	}

	moduleName := mapping.Module
	if moduleName == "" {
		moduleName = flags.Arg(0)
	}
	responseProjectPath := mapping.GitLabProjectPath
	if responseProjectPath == "" {
		responseProjectPath = projectPathValue
	}
	responseProjectID := mapping.GitLabProjectID
	if responseProjectID == 0 {
		responseProjectID = parsedProjectID
	}
	fmt.Fprintf(app.output(), "Linked module %s to GitLab project %s (%d)\n", moduleName, responseProjectPath, responseProjectID)
	return nil
}

func (app App) moduleDependencies(ctx context.Context, args []string) error {
	if hasHelp(args) {
		app.printHelp("module dependencies")
		return nil
	}
	flags := flag.NewFlagSet("module dependencies", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	if err := flags.Parse(flagsFirst(args, nil)); err != nil {
		return err
	}
	if flags.NArg() != 1 {
		return errors.New("module dependencies requires a module name")
	}

	client, err := app.client()
	if err != nil {
		return err
	}
	graph, err := client.GetModuleDependencies(ctx, flags.Arg(0))
	if err != nil {
		if errors.Is(err, api.ErrUnauthorized) {
			return errors.New("module dependencies failed: unauthorized")
		}
		if errors.Is(err, api.ErrNotFound) {
			return fmt.Errorf("module %q was not found", flags.Arg(0))
		}
		return fmt.Errorf("module dependencies failed: %w", err)
	}

	printModuleDependencies(app.output(), graph)
	return nil
}

func (app App) moduleAffected(ctx context.Context, args []string) error {
	if hasHelp(args) {
		app.printHelp("module affected")
		return nil
	}
	flags := flag.NewFlagSet("module affected", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	if err := flags.Parse(flagsFirst(args, nil)); err != nil {
		return err
	}
	if flags.NArg() != 1 {
		return errors.New("module affected requires a module name")
	}

	client, err := app.client()
	if err != nil {
		return err
	}
	affected, err := client.GetAffectedModules(ctx, flags.Arg(0))
	if err != nil {
		if errors.Is(err, api.ErrUnauthorized) {
			return errors.New("module affected failed: unauthorized")
		}
		if errors.Is(err, api.ErrNotFound) {
			return fmt.Errorf("module %q was not found", flags.Arg(0))
		}
		return fmt.Errorf("module affected failed: %w", err)
	}

	printAffectedModules(app.output(), affected)
	return nil
}

func (app App) list(ctx context.Context, args []string) error {
	if hasHelp(args) {
		app.printHelp("list")
		return nil
	}
	if len(args) != 0 {
		return errors.New("list does not accept arguments")
	}
	client, err := app.client()
	if err != nil {
		return err
	}
	modules, err := client.ListModules(ctx)
	if err != nil {
		return err
	}

	fmt.Fprintln(app.output(), "MODULE\tLATEST VERSION\tUPDATED")
	for _, module := range modules {
		latest := ""
		versions, err := client.ListModuleVersions(ctx, module.Name)
		if err == nil && len(versions) > 0 {
			latest = versions[0].Version
		}
		updated := ""
		if !module.UpdatedAt.IsZero() {
			updated = module.UpdatedAt.Format(timeFormat)
		}
		fmt.Fprintf(app.output(), "%s\t%s\t%s\n", module.Name, latest, updated)
	}
	return nil
}

func (app App) push(ctx context.Context, args []string) error {
	if hasHelp(args) {
		app.printHelp("push")
		return nil
	}
	flags := flag.NewFlagSet("push", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	version := flags.String("version", "", "module version")
	path := flags.String("path", "", "directory containing proto files")
	if err := flags.Parse(flagsFirst(args, nil)); err != nil {
		return err
	}
	if flags.NArg() != 1 {
		return errors.New("push requires a module name")
	}
	if strings.TrimSpace(*version) == "" {
		return errors.New("push requires --version")
	}
	if strings.TrimSpace(*path) == "" {
		return errors.New("push requires --path")
	}

	artifact, err := createArtifact(*path)
	if err != nil {
		return err
	}
	client, err := app.client()
	if err != nil {
		return err
	}
	response, err := client.PublishModuleVersion(ctx, flags.Arg(0), *version, "artifact.tar.gz", artifact.Body)
	if err != nil {
		if errors.Is(err, api.ErrConflict) {
			return fmt.Errorf("module %s version %s already exists", flags.Arg(0), *version)
		}
		if errors.Is(err, api.ErrNotFound) {
			return fmt.Errorf("module %q was not found", flags.Arg(0))
		}
		return err
	}

	checksum := response.SourceArtifact.ChecksumSHA256
	if checksum == "" {
		checksum = artifact.ChecksumSHA256
	}
	size := response.SourceArtifact.SizeBytes
	if size == 0 {
		size = artifact.SizeBytes
	}
	versionValue := response.Version
	if versionValue == "" {
		versionValue = *version
	}
	moduleName := response.Module
	if moduleName == "" {
		moduleName = flags.Arg(0)
	}

	fmt.Fprintf(app.output(), "Published %s %s\n", moduleName, versionValue)
	fmt.Fprintf(app.output(), "Source checksum SHA-256: %s\n", checksum)
	fmt.Fprintf(app.output(), "Source size: %d bytes\n", size)
	if response.BufImageArtifact.ChecksumSHA256 != "" || response.BufImageArtifact.SizeBytes > 0 {
		fmt.Fprintf(app.output(), "Buf image checksum SHA-256: %s\n", response.BufImageArtifact.ChecksumSHA256)
		fmt.Fprintf(app.output(), "Buf image size: %d bytes\n", response.BufImageArtifact.SizeBytes)
	}
	if response.Buf.LintStatus != "" {
		fmt.Fprintf(app.output(), "Lint status: %s\n", response.Buf.LintStatus)
	}
	if response.MetadataSummary.Files > 0 || response.MetadataSummary.Services > 0 || response.MetadataSummary.Messages > 0 || response.MetadataSummary.Enums > 0 {
		fmt.Fprintf(app.output(), "Metadata: files=%d packages=%d services=%d methods=%d messages=%d fields=%d enums=%d enum_values=%d\n",
			response.MetadataSummary.Files,
			response.MetadataSummary.Packages,
			response.MetadataSummary.Services,
			response.MetadataSummary.Methods,
			response.MetadataSummary.Messages,
			response.MetadataSummary.Fields,
			response.MetadataSummary.Enums,
			response.MetadataSummary.EnumValues,
		)
	}
	return nil
}

func (app App) checkBreaking(ctx context.Context, args []string) error {
	if hasHelp(args) {
		app.printHelp("check-breaking")
		return nil
	}
	flags := flag.NewFlagSet("check-breaking", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	path := flags.String("path", "", "Buf workspace root")
	against := flags.String("against", "latest", "baseline version or latest")
	targetRef := flags.String("target-ref", "", "target reference")
	reportFile := flags.String("report-file", "", "human-readable report file")
	if err := flags.Parse(flagsFirst(args, nil)); err != nil {
		return err
	}
	if flags.NArg() != 1 {
		return errors.New("check-breaking requires a module name")
	}
	if strings.TrimSpace(*path) == "" {
		return errors.New("check-breaking requires --path")
	}
	if strings.TrimSpace(*against) == "" {
		return errors.New("check-breaking requires --against")
	}

	artifact, err := createArtifact(*path)
	if err != nil {
		return err
	}
	client, err := app.client()
	if err != nil {
		return err
	}
	report, err := client.CheckBreaking(ctx, flags.Arg(0), strings.TrimSpace(*against), strings.TrimSpace(*targetRef), "source.tar.gz", artifact.Body)
	if err != nil {
		if errors.Is(err, api.ErrUnauthorized) {
			return fmt.Errorf("check-breaking failed: unauthorized")
		}
		if errors.Is(err, api.ErrNotFound) {
			return fmt.Errorf("check-breaking failed: module or baseline was not found")
		}
		if errors.Is(err, api.ErrConflict) {
			return fmt.Errorf("check-breaking failed: baseline is not Buf-compatible")
		}
		if errors.Is(err, api.ErrTooLarge) {
			return fmt.Errorf("check-breaking failed: artifact too large")
		}
		return fmt.Errorf("check-breaking failed: %w", err)
	}

	summary := strings.TrimSpace(report.HumanSummary)
	if summary == "" {
		summary = fallbackBreakingSummary(report)
	}
	if strings.TrimSpace(*reportFile) != "" {
		if err := os.WriteFile(strings.TrimSpace(*reportFile), []byte(summary+"\n"), 0o600); err != nil {
			return fmt.Errorf("write report file: %w", err)
		}
	}
	fmt.Fprintln(app.output(), summary)
	if report.Status == "breaking" {
		return ExitError{Code: 1}
	}
	return nil
}

func (app App) pull(ctx context.Context, args []string) error {
	if hasHelp(args) {
		app.printHelp("pull")
		return nil
	}
	flags := flag.NewFlagSet("pull", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	version := flags.String("version", "", "module version")
	output := flags.String("output", "", "output directory")
	force := flags.Bool("force", false, "overwrite non-empty output directory")
	if err := flags.Parse(flagsFirst(args, map[string]bool{"force": true})); err != nil {
		return err
	}
	if flags.NArg() != 1 {
		return errors.New("pull requires a module name")
	}
	if strings.TrimSpace(*version) == "" {
		return errors.New("pull requires --version")
	}
	if strings.TrimSpace(*output) == "" {
		return errors.New("pull requires --output")
	}

	client, err := app.client()
	if err != nil {
		return err
	}
	artifact, err := client.DownloadArtifact(ctx, flags.Arg(0), *version)
	if err != nil {
		if errors.Is(err, api.ErrNotFound) {
			return fmt.Errorf("artifact for %s %s was not found", flags.Arg(0), *version)
		}
		return err
	}
	defer artifact.Body.Close()

	if err := extractArtifact(artifact.Body, *output, *force); err != nil {
		return err
	}
	fmt.Fprintf(app.output(), "Pulled %s %s to %s\n", flags.Arg(0), *version, *output)
	return nil
}

func (app App) version() error {
	info := version.Info()
	fmt.Fprintf(app.output(), "version: %s\n", info.Version)
	fmt.Fprintf(app.output(), "commit: %s\n", info.Commit)
	fmt.Fprintf(app.output(), "build_date: %s\n", info.BuildDate)
	return nil
}

func (app App) usage() error {
	return errors.New("usage: protoradar <login|version|module|push|check-breaking|gitlab|pull|list|runtime>")
}

func (app App) printHelp(topic string) {
	fmt.Fprint(app.output(), commandHelp(strings.TrimSpace(topic)))
}

func hasHelp(args []string) bool {
	return len(args) > 0 && isHelp(args[0])
}

func isHelp(arg string) bool {
	return arg == "-h" || arg == "--help" || arg == "help"
}

func commandHelp(topic string) string {
	switch topic {
	case "login":
		return `Usage:
  protoradar login --server <url> --token <token>

Stores credentials for later CLI commands. The token is never printed.
`
	case "module":
		return `Usage:
  protoradar module <create|list|link-gitlab|dependencies|affected> [flags]
`
	case "module create":
		return `Usage:
  protoradar module create <module> [--description <text>] [--repository-url <url>]

Example:
  protoradar module create user-api --description "User API contracts"
`
	case "module link-gitlab":
		return `Usage:
  protoradar module link-gitlab <module> --project-id <id> --project-path <path> --gitlab-base-url <url>
`
	case "module dependencies":
		return `Usage:
  protoradar module dependencies <module>

Shows direct upstream, downstream, and unresolved protobuf dependencies.
`
	case "module affected":
		return `Usage:
  protoradar module affected <module>

Shows direct downstream modules currently known to depend on the module.
`
	case "list":
		return `Usage:
  protoradar list
  protoradar module list
`
	case "push":
		return `Usage:
  protoradar push <module> --version <version> --path <buf-workspace>
`
	case "pull":
		return `Usage:
  protoradar pull <module> --version <version> --output <directory> [--force]
`
	case "check-breaking":
		return `Usage:
  protoradar check-breaking <module> --path <buf-workspace> [--against latest|<version>] [--target-ref <ref>] [--report-file <path>]

Exit codes:
  0 no breaking changes
  1 breaking changes found
  2 input, auth, network, server, config, or internal error
`
	case "gitlab":
		return `Usage:
  protoradar gitlab mr-check [flags]
`
	case "gitlab mr-check":
		return `Usage:
  protoradar gitlab mr-check --module <module> --path <buf-workspace> --gitlab-base-url <url> --project-id <id> --merge-request-iid <iid> --commit-sha <sha> --gitlab-token <token>

In GitLab CI, PROTORADAR_SERVER_URL, PROTORADAR_TOKEN, and PROTORADAR_GITLAB_TOKEN are supported.
Exit code 1 means breaking changes were found, not a tool failure.
`
	case "runtime":
		return `Usage:
  protoradar runtime report [--from-file <path>] [--module <module@version>]...
`
	case "runtime report":
		return `Usage:
  protoradar runtime report --service <service> --environment <env> --git-commit <sha> --build-version <version> --module <module@version>
  protoradar runtime report --from-file protoradar-runtime.yaml

Drift statuses such as behind_latest or unknown_version are successful inventory results and exit 0.
`
	default:
		return `ProtoRadar CLI

Usage:
  protoradar <command> [flags]

Commands:
  login                  Store server URL and API token.
  version                Print version, commit, and build date.
  module create          Create a protobuf module.
  module list            List modules. Alias: protoradar list.
  module link-gitlab     Link a module to a GitLab project.
  module dependencies    Show direct upstream/downstream protobuf dependencies.
  module affected        Show direct downstream modules affected by a module.
  push                   Publish a Buf-compatible module version.
  pull                   Download a published source artifact.
  check-breaking         Run a server-side breaking-change check.
  gitlab mr-check        Run GitLab MR bot check/comment/status flow.
  runtime report         Report deployed module versions.

Authentication:
  Use protoradar login, or set PROTORADAR_SERVER_URL and PROTORADAR_TOKEN in CI.

Exit codes:
  0 success or no breaking changes
  1 breaking changes found by check-breaking or gitlab mr-check
  2 invalid input, auth, network, server, config, or internal error

Examples:
  protoradar login --server http://localhost:8080 --token <token>
  protoradar module create user-api --description "User API contracts"
  protoradar push user-api --version v1.0.0 --path examples/repos/user-api
  protoradar check-breaking user-api --path . --against latest
`
	}
}

func (app App) configPath() (string, error) {
	if app.ConfigPath != "" {
		return app.ConfigPath, nil
	}
	return config.DefaultPath()
}

func (app App) client() (*api.Client, error) {
	path, err := app.configPath()
	if err != nil {
		return nil, err
	}
	cfg, err := config.Load(path)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(cfg.ServerURL) == "" {
		return nil, errors.New("server_url is not configured; run protoradar login")
	}
	if strings.TrimSpace(cfg.Token) == "" {
		return nil, errors.New("token is not configured; run protoradar login")
	}
	if err := validateServerURL(cfg.ServerURL); err != nil {
		return nil, err
	}
	return api.NewClient(cfg.ServerURL, cfg.Token, app.HTTPClient), nil
}

func (app App) output() io.Writer {
	if app.Out == nil {
		return io.Discard
	}
	return app.Out
}

func validateServerURL(value string) error {
	parsed, err := url.Parse(value)
	if err != nil {
		return err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return errors.New("server URL must use http or https")
	}
	if parsed.Host == "" {
		return errors.New("server URL must include a host")
	}
	return nil
}

func errNotImplemented(command string) error {
	return fmt.Errorf("%s is not implemented yet", command)
}

const timeFormat = "2006-01-02T15:04:05Z07:00"

type ExitError struct {
	Code int
	Err  error
}

func (err ExitError) Error() string {
	if err.Err == nil {
		return ""
	}
	return err.Err.Error()
}

func (err ExitError) Unwrap() error {
	return err.Err
}

func fallbackBreakingSummary(report api.BreakingReport) string {
	var builder strings.Builder
	builder.WriteString("ProtoRadar Breaking Change Report\n\n")
	if report.Module != "" {
		fmt.Fprintf(&builder, "Module: %s\n", report.Module)
	}
	if report.Against != "" {
		fmt.Fprintf(&builder, "Against: %s\n", report.Against)
	}
	target := report.TargetRef
	if strings.TrimSpace(target) == "" {
		target = "local"
	}
	fmt.Fprintf(&builder, "Target: %s\n", target)
	fmt.Fprintf(&builder, "Status: %s\n", report.Status)
	fmt.Fprintf(&builder, "Changes: %d\n", report.ChangeCount)
	if report.Status == "breaking" {
		builder.WriteString("\nBreaking changes:\n")
		for index, change := range report.Changes {
			filePath := change.FilePath
			if strings.TrimSpace(filePath) == "" {
				filePath = "(unknown file)"
			}
			fmt.Fprintf(&builder, "%d. %s\n", index+1, filePath)
			writeFallbackLine(&builder, "Rule", change.RuleID)
			writeFallbackLine(&builder, "Symbol", change.Symbol)
			writeFallbackLine(&builder, "Package", change.PackageName)
			writeFallbackLine(&builder, "Category", change.Category)
			writeFallbackLine(&builder, "Severity", change.Severity)
			writeFallbackLine(&builder, "Message", change.Message)
		}
		builder.WriteString("\nResult: breaking changes found.")
		return builder.String()
	}
	builder.WriteString("\nNo breaking changes found.\n\nResult: no breaking changes found.")
	return builder.String()
}

func printModuleDependencies(w io.Writer, graph api.ModuleDependencyGraph) {
	fmt.Fprintf(w, "Module: %s\n\n", graph.Module)
	fmt.Fprintln(w, "Downstream consumers:")
	printDependencyModules(w, graph.Downstream, "No downstream modules are currently known to depend on this module.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Upstream dependencies:")
	printDependencyModules(w, graph.Upstream, "No upstream dependencies are currently known for this module.")
	if len(graph.Unresolved) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Unresolved dependencies:")
		for _, dependency := range graph.Unresolved {
			target := strings.TrimSpace(dependency.ImportPath)
			if target == "" {
				target = strings.TrimSpace(dependency.ReferencedSymbol)
			}
			if target == "" {
				target = "(unknown dependency)"
			}
			fmt.Fprintf(w, "- %s\n", target)
			writeOutputLine(w, "  reason", dependency.Reason)
		}
	}
}

func printAffectedModules(w io.Writer, affected api.AffectedModules) {
	fmt.Fprintf(w, "Module: %s\n\n", affected.Module)
	fmt.Fprintln(w, "Affected modules:")
	printDependencyModules(w, affected.AffectedModules, "No downstream modules are currently known to depend on this module.")
}

func printDependencyModules(w io.Writer, modules []api.DependencyModule, emptyMessage string) {
	if len(modules) == 0 {
		fmt.Fprintf(w, "%s\n", emptyMessage)
		return
	}
	for _, module := range modules {
		version := strings.TrimSpace(module.LatestVersion)
		if version == "" {
			version = "unknown"
		}
		fmt.Fprintf(w, "- %s@%s\n", module.Module, version)
		if len(module.DependencySources) > 0 {
			fmt.Fprintf(w, "  sources: %s\n", strings.Join(module.DependencySources, ", "))
		}
		for _, reason := range module.Reasons {
			writeOutputLine(w, "  reason", reason)
		}
	}
}

func writeOutputLine(w io.Writer, label string, value string) {
	if strings.TrimSpace(value) == "" {
		return
	}
	fmt.Fprintf(w, "%s: %s\n", label, value)
}

func writeFallbackLine(builder *strings.Builder, label string, value string) {
	if strings.TrimSpace(value) == "" {
		return
	}
	fmt.Fprintf(builder, "   %s: %s\n", label, value)
}

func flagsFirst(args []string, boolFlags map[string]bool) []string {
	var flags []string
	var positional []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			positional = append(positional, arg)
			continue
		}

		flags = append(flags, arg)
		name := strings.TrimLeft(arg, "-")
		if before, _, ok := strings.Cut(name, "="); ok {
			name = before
		}
		if strings.Contains(arg, "=") || boolFlags[name] {
			continue
		}
		if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
			flags = append(flags, args[i+1])
			i++
		}
	}
	return append(flags, positional...)
}
