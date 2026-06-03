package main

import (
	"context"
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
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
