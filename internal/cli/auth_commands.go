package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/alryzden/ProtoRadar/internal/cli/api"
	"github.com/alryzden/ProtoRadar/internal/cli/config"
)

func (app App) login(ctx context.Context, args []string) error {
	if hasHelp(args) {
		app.printHelp("login")
		return nil
	}
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

	path, err := app.configPath()
	if err != nil {
		return err
	}
	cfg, err := config.Load(path)
	if err != nil {
		return err
	}
	httpClient, err := app.httpClient(cfg)
	if err != nil {
		return err
	}

	client := api.NewClient(*serverURL, *token, httpClient)
	if err := client.CheckAuth(ctx); err != nil {
		return fmt.Errorf("login failed: %w", err)
	}

	if err := config.Save(path, config.Config{ServerURL: *serverURL, Token: *token, HTTPTimeout: cfg.HTTPTimeout}); err != nil {
		return err
	}

	fmt.Fprintln(app.output(), "Logged in to "+strings.TrimRight(*serverURL, "/"))
	return nil
}
