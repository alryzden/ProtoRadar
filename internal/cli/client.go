package cli

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/alryzden/ProtoRadar/internal/cli/api"
	"github.com/alryzden/ProtoRadar/internal/cli/config"
)

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
	httpClient, err := app.httpClient(cfg)
	if err != nil {
		return nil, err
	}
	return newAPIClientFromConfig(cfg, httpClient)
}

func (app App) httpClient(cfg config.Config) (*http.Client, error) {
	if app.HTTPClient != nil {
		return app.HTTPClient, nil
	}
	timeout, err := cfg.HTTPTimeoutDuration()
	if err != nil {
		return nil, err
	}
	return newDefaultHTTPClient(timeout), nil
}

func newDefaultHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout}
}

func newAPIClientFromConfig(cfg config.Config, httpClient *http.Client) (*api.Client, error) {
	if strings.TrimSpace(cfg.ServerURL) == "" {
		return nil, errors.New("server_url is not configured; run protoradar login")
	}
	if strings.TrimSpace(cfg.Token) == "" {
		return nil, errors.New("token is not configured; run protoradar login")
	}
	if err := validateServerURL(cfg.ServerURL); err != nil {
		return nil, err
	}
	return api.NewClient(cfg.ServerURL, cfg.Token, httpClient), nil
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
