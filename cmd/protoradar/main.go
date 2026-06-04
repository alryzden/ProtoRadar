package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/alryzden/ProtoRadar/internal/cli"
)

func main() {
	app := cli.App{
		Out: os.Stdout,
		Err: os.Stderr,
	}
	if err := app.Run(context.Background(), os.Args[1:]); err != nil {
		if err.Error() != "" {
			fmt.Fprintln(os.Stderr, err)
		}
		code := 2
		var exitErr cli.ExitError
		if errors.As(err, &exitErr) && exitErr.Code != 0 {
			code = exitErr.Code
		}
		os.Exit(code)
	}
}
