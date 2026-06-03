package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/alryzden/ProtoRadar/internal/cli/api"
	"github.com/alryzden/ProtoRadar/internal/cli/config"
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

	switch args[0] {
	case "login":
		return app.login(ctx, args[1:])
	case "module":
		return app.module(ctx, args[1:])
	case "push":
		return app.push(ctx, args[1:])
	case "pull":
		return app.pull(ctx, args[1:])
	case "list":
		return app.list(ctx, args[1:])
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func (app App) login(ctx context.Context, args []string) error {
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
	switch args[0] {
	case "create":
		return app.createModule(ctx, args[1:])
	default:
		return fmt.Errorf("unknown module subcommand %q", args[0])
	}
}

func (app App) createModule(ctx context.Context, args []string) error {
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

func (app App) list(ctx context.Context, args []string) error {
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

func (app App) pull(ctx context.Context, args []string) error {
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

func (app App) usage() error {
	return errors.New("usage: protoradar <login|module|push|pull|list>")
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
