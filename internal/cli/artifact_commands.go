package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/alryzden/ProtoRadar/internal/cli/api"
)

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
	defer closePulledArtifactBody(artifact.Body)

	if err := extractArtifact(artifact.Body, *output, *force); err != nil {
		return err
	}
	fmt.Fprintf(app.output(), "Pulled %s %s to %s\n", flags.Arg(0), *version, *output)
	return nil
}

func closePulledArtifactBody(body io.Closer) {
	// Pull result is determined by download/extract; body close is cleanup.
	_ = body.Close() //nolint:errcheck
}
