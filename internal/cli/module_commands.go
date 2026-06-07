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
)

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
	case "owners":
		return app.moduleOwners(ctx, args[1:])
	case "version":
		return app.moduleVersion(ctx, args[1:])
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

func (app App) moduleVersion(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("module version requires a subcommand")
	}
	if isHelp(args[0]) {
		app.printHelp("module version")
		return nil
	}
	switch args[0] {
	case "deprecate":
		return app.deprecateModuleVersion(ctx, args[1:])
	default:
		return fmt.Errorf("unknown module version subcommand %q", args[0])
	}
}

func (app App) deprecateModuleVersion(ctx context.Context, args []string) error {
	if hasHelp(args) {
		app.printHelp("module version deprecate")
		return nil
	}
	flags := flag.NewFlagSet("module version deprecate", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	reason := flags.String("reason", "", "optional deprecation reason")
	if err := flags.Parse(flagsFirst(args, nil)); err != nil {
		return err
	}
	if flags.NArg() != 2 {
		return errors.New("module version deprecate requires a module name and version")
	}

	client, err := app.client()
	if err != nil {
		return err
	}
	version, err := client.DeprecateModuleVersion(ctx, flags.Arg(0), flags.Arg(1), api.DeprecateModuleVersionRequest{
		Reason: *reason,
	})
	if err != nil {
		if errors.Is(err, api.ErrUnauthorized) {
			return errors.New("module version deprecate failed: unauthorized")
		}
		if errors.Is(err, api.ErrForbidden) {
			return errors.New("module version deprecate failed: forbidden")
		}
		if errors.Is(err, api.ErrNotFound) {
			return fmt.Errorf("module version %s %s was not found", flags.Arg(0), flags.Arg(1))
		}
		return fmt.Errorf("module version deprecate failed: %w", err)
	}

	deprecatedAt := ""
	if version.DeprecatedAt != nil && !version.DeprecatedAt.IsZero() {
		deprecatedAt = version.DeprecatedAt.Format(timeFormat)
	}
	fmt.Fprintf(app.output(), "Deprecated %s %s\n", flags.Arg(0), version.Version)
	fmt.Fprintf(app.output(), "Deprecated at: %s\n", deprecatedAt)
	fmt.Fprintf(app.output(), "Deprecated by: %s\n", version.DeprecatedBy)
	fmt.Fprintf(app.output(), "Reason: %s\n", version.DeprecationReason)
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
