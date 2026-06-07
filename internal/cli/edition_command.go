package cli

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/alryzden/ProtoRadar/internal/cli/api"
)

func (app App) edition(ctx context.Context) error {
	client, err := app.client()
	if err != nil {
		return ExitError{Code: 2, Err: err}
	}
	response, err := client.GetEdition(ctx)
	if err != nil {
		return ExitError{Code: 2, Err: fmt.Errorf("edition failed: %w", err)}
	}
	printEdition(app.output(), response)
	return nil
}
func printEdition(w io.Writer, response api.EditionResponse) {
	fmt.Fprintf(w, "Edition: %s\n", response.Edition)
	fmt.Fprintf(w, "Version: %s\n", response.Version)
	writeOutputLine(w, "Commit", response.Commit)
	writeOutputLine(w, "Build date", response.BuildDate)

	var enabled []string
	var unavailable []string
	for _, capability := range response.Capabilities {
		name := strings.TrimSpace(capability.Name)
		if name == "" {
			continue
		}
		if capability.Enabled {
			enabled = append(enabled, name)
		} else {
			unavailable = append(unavailable, name)
		}
	}
	if len(enabled) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Enabled capabilities:")
		for _, name := range enabled {
			fmt.Fprintf(w, "- %s\n", name)
		}
	}
	if len(unavailable) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Unavailable enterprise capabilities:")
		for _, name := range unavailable {
			fmt.Fprintf(w, "- %s\n", name)
		}
	}
}
