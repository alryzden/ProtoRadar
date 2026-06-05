package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/alryzden/ProtoRadar/internal/cli/api"
)

type moduleFlags []string

func (flags *moduleFlags) String() string {
	return strings.Join(*flags, ",")
}

func (flags *moduleFlags) Set(value string) error {
	*flags = append(*flags, value)
	return nil
}

type runtimeReportFile struct {
	ServiceName  string                    `json:"service_name" yaml:"service_name"`
	Environment  string                    `json:"environment" yaml:"environment"`
	GitCommit    string                    `json:"git_commit" yaml:"git_commit"`
	BuildVersion string                    `json:"build_version" yaml:"build_version"`
	Modules      []runtimeReportFileModule `json:"modules" yaml:"modules"`
}

type runtimeReportFileModule struct {
	Module  string `json:"module" yaml:"module"`
	Version string `json:"version" yaml:"version"`
}

func (app App) runtime(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("runtime requires a subcommand")
	}
	if isHelp(args[0]) {
		app.printHelp("runtime")
		return nil
	}
	switch args[0] {
	case "report":
		return app.runtimeReport(ctx, args[1:])
	default:
		return fmt.Errorf("unknown runtime subcommand %q", args[0])
	}
}

func (app App) runtimeReport(ctx context.Context, args []string) error {
	if hasHelp(args) {
		app.printHelp("runtime report")
		return nil
	}
	flags := flag.NewFlagSet("runtime report", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	service := flags.String("service", "", "runtime service name; defaults to PROTORADAR_SERVICE or CI_PROJECT_NAME")
	environment := flags.String("environment", "", "runtime environment; defaults to PROTORADAR_ENVIRONMENT or CI_ENVIRONMENT_NAME")
	gitCommit := flags.String("git-commit", "", "git commit; defaults to CI_COMMIT_SHA")
	buildVersion := flags.String("build-version", "", "build version; defaults to PROTORADAR_BUILD_VERSION, CI_COMMIT_TAG or CI_COMMIT_SHORT_SHA")
	fromFile := flags.String("from-file", "", "runtime report YAML or JSON file")
	var moduleValues moduleFlags
	flags.Var(&moduleValues, "module", "reported module as module@version; repeatable. Explicit --module values replace file modules.")
	if err := flags.Parse(flagsFirst(args, nil)); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("runtime report does not accept positional arguments")
	}

	request, err := app.runtimeReportRequest(*fromFile)
	if err != nil {
		return ExitError{Code: 2, Err: err}
	}
	applyRuntimeReportDefaults(&request)
	if flagWasSet(flags, "service") {
		request.ServiceName = *service
	}
	if flagWasSet(flags, "environment") {
		request.Environment = *environment
	}
	if flagWasSet(flags, "git-commit") {
		request.GitCommit = *gitCommit
	}
	if flagWasSet(flags, "build-version") {
		request.BuildVersion = *buildVersion
	}
	if len(moduleValues) > 0 {
		request.Modules = nil
		for _, value := range moduleValues {
			module, err := parseRuntimeModuleFlag(value)
			if err != nil {
				return ExitError{Code: 2, Err: err}
			}
			request.Modules = append(request.Modules, module)
		}
	}
	if err := validateRuntimeReportRequest(request); err != nil {
		return ExitError{Code: 2, Err: err}
	}

	client, err := app.client()
	if err != nil {
		return ExitError{Code: 2, Err: err}
	}
	response, err := client.ReportRuntimeInventory(ctx, request)
	if err != nil {
		if errors.Is(err, api.ErrUnauthorized) {
			return ExitError{Code: 2, Err: errors.New("runtime report failed: unauthorized")}
		}
		return ExitError{Code: 2, Err: fmt.Errorf("runtime report failed: %w", err)}
	}

	printRuntimeReport(app.output(), response)
	return nil
}

func (app App) runtimeReportRequest(path string) (api.ReportRuntimeInventoryRequest, error) {
	if strings.TrimSpace(path) == "" {
		return api.ReportRuntimeInventoryRequest{}, nil
	}
	body, err := os.ReadFile(strings.TrimSpace(path))
	if err != nil {
		return api.ReportRuntimeInventoryRequest{}, fmt.Errorf("read runtime report file: %w", err)
	}
	var file runtimeReportFile
	if err := yaml.Unmarshal(body, &file); err != nil {
		if jsonErr := json.Unmarshal(body, &file); jsonErr != nil {
			return api.ReportRuntimeInventoryRequest{}, fmt.Errorf("parse runtime report file: %w", err)
		}
	}
	request := api.ReportRuntimeInventoryRequest{
		ServiceName:  file.ServiceName,
		Environment:  file.Environment,
		GitCommit:    file.GitCommit,
		BuildVersion: file.BuildVersion,
		Modules:      make([]api.RuntimeModuleRequest, 0, len(file.Modules)),
	}
	for _, module := range file.Modules {
		request.Modules = append(request.Modules, api.RuntimeModuleRequest{Module: module.Module, Version: module.Version})
	}
	return request, nil
}

func applyRuntimeReportDefaults(request *api.ReportRuntimeInventoryRequest) {
	if strings.TrimSpace(request.ServiceName) == "" {
		request.ServiceName = firstEnv("PROTORADAR_SERVICE", "CI_PROJECT_NAME")
	}
	if strings.TrimSpace(request.Environment) == "" {
		request.Environment = firstEnv("PROTORADAR_ENVIRONMENT", "CI_ENVIRONMENT_NAME")
	}
	if strings.TrimSpace(request.GitCommit) == "" {
		request.GitCommit = firstEnv("CI_COMMIT_SHA")
	}
	if strings.TrimSpace(request.BuildVersion) == "" {
		request.BuildVersion = firstEnv("PROTORADAR_BUILD_VERSION", "CI_COMMIT_TAG", "CI_COMMIT_SHORT_SHA")
	}
}

func firstEnv(names ...string) string {
	for _, name := range names {
		if value := strings.TrimSpace(os.Getenv(name)); value != "" {
			return value
		}
	}
	return ""
}

func parseRuntimeModuleFlag(value string) (api.RuntimeModuleRequest, error) {
	trimmed := strings.TrimSpace(value)
	module, version, ok := strings.Cut(trimmed, "@")
	if !ok || strings.Contains(version, "@") || strings.TrimSpace(module) == "" || strings.TrimSpace(version) == "" {
		return api.RuntimeModuleRequest{}, fmt.Errorf("invalid --module %q; expected module@version", value)
	}
	if strings.TrimSpace(module) != module || strings.TrimSpace(version) != version {
		return api.RuntimeModuleRequest{}, fmt.Errorf("invalid --module %q; expected module@version", value)
	}
	return api.RuntimeModuleRequest{Module: module, Version: version}, nil
}

func validateRuntimeReportRequest(request api.ReportRuntimeInventoryRequest) error {
	if strings.TrimSpace(request.ServiceName) == "" {
		return errors.New("runtime report requires --service, PROTORADAR_SERVICE, CI_PROJECT_NAME or service_name in --from-file")
	}
	if strings.TrimSpace(request.Environment) == "" {
		return errors.New("runtime report requires --environment, PROTORADAR_ENVIRONMENT, CI_ENVIRONMENT_NAME or environment in --from-file")
	}
	if strings.TrimSpace(request.GitCommit) == "" {
		return errors.New("runtime report requires --git-commit, CI_COMMIT_SHA or git_commit in --from-file")
	}
	if strings.TrimSpace(request.BuildVersion) == "" {
		return errors.New("runtime report requires --build-version, PROTORADAR_BUILD_VERSION, CI_COMMIT_TAG, CI_COMMIT_SHORT_SHA or build_version in --from-file")
	}
	if len(request.Modules) == 0 {
		return errors.New("runtime report requires at least one --module or modules in --from-file")
	}
	for _, module := range request.Modules {
		if strings.TrimSpace(module.Module) == "" || strings.TrimSpace(module.Version) == "" {
			return errors.New("runtime report modules require module and version")
		}
	}
	return nil
}

func flagWasSet(flags *flag.FlagSet, name string) bool {
	wasSet := false
	flags.Visit(func(flag *flag.Flag) {
		if flag.Name == name {
			wasSet = true
		}
	})
	return wasSet
}

func printRuntimeReport(w io.Writer, response api.ReportRuntimeInventoryResponse) {
	fmt.Fprintln(w, "Runtime inventory reported")
	fmt.Fprintf(w, "Service: %s\n", response.ServiceName)
	fmt.Fprintf(w, "Environment: %s\n", response.Environment)
	fmt.Fprintf(w, "Build: %s\n", response.BuildVersion)
	fmt.Fprintf(w, "Commit: %s\n", response.GitCommit)
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Modules:")
	for _, usage := range response.Usages {
		fmt.Fprintf(w, "- %s@%s", usage.Module, usage.Version)
		status := runtimeStatusText(usage)
		if status != "" {
			fmt.Fprintf(w, " - %s", status)
		}
		fmt.Fprintln(w)
	}
}

func runtimeStatusText(usage api.RuntimeModuleUsage) string {
	switch usage.DriftStatus {
	case "up_to_date":
		return "up to date"
	case "behind_latest":
		if usage.LatestVersion != "" {
			return "behind latest (latest: " + usage.LatestVersion + ")"
		}
		return "behind latest"
	case "unknown_version":
		if usage.DriftReason != "" {
			return "unknown version (" + usage.DriftReason + ")"
		}
		return "unknown version"
	case "deprecated_version":
		return "deprecated version"
	default:
		return usage.DriftStatus
	}
}
