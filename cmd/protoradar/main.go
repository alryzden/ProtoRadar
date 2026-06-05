package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/alryzden/ProtoRadar/internal/cli"
)

func main() {
	os.Exit(run(context.Background(), os.Args[1:], os.Stdout, os.Stderr))
}

func run(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	app := cli.App{
		Out: stdout,
		Err: stderr,
	}
	if err := app.Run(ctx, args); err != nil {
		if err.Error() != "" {
			fmt.Fprintln(stderr, err)
		}
		code := 2
		var exitErr cli.ExitError
		if errors.As(err, &exitErr) && exitErr.Code != 0 {
			code = exitErr.Code
		}
		return code
	}
	return 0
}
