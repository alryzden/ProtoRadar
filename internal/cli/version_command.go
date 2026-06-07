package cli

import (
	"fmt"

	"github.com/alryzden/ProtoRadar/internal/version"
)

func (app App) version() error {
	info := version.Info()
	fmt.Fprintf(app.output(), "version: %s\n", info.Version)
	fmt.Fprintf(app.output(), "commit: %s\n", info.Commit)
	fmt.Fprintf(app.output(), "build_date: %s\n", info.BuildDate)
	return nil
}
