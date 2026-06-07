package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/alryzden/ProtoRadar/internal/cli/api"
)

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
