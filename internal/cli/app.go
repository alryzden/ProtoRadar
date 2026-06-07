package cli

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
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
	case "edition":
		return app.edition(ctx)
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
	case "approvals":
		return app.approvals(ctx, args[1:])
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}
