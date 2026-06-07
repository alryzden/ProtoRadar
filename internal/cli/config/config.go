package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	appconfig "github.com/alryzden/ProtoRadar/internal/config"
)

const (
	envServerURL   = "PROTORADAR_SERVER_URL"
	envToken       = "PROTORADAR_TOKEN"
	envHTTPTimeout = "PROTORADAR_CLI_HTTP_TIMEOUT"
)

const DefaultHTTPTimeout = 30 * time.Second

type Config struct {
	ServerURL   string
	Token       string
	HTTPTimeout time.Duration
}

func DefaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "protoradar", "config.yaml"), nil
}

func Load(path string) (Config, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return Config{}, err
		}
	}

	cfg, err := parse(body)
	if err != nil {
		return Config{}, err
	}
	if err := cfg.ApplyEnv(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func Save(path string, cfg Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}

	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer closeConfigFile(file)

	if err := file.Chmod(0o600); err != nil {
		return err
	}

	_, err = fmt.Fprintf(file, "server_url: %q\ntoken: %q\n", cfg.ServerURL, cfg.Token)
	if err != nil {
		return err
	}
	if cfg.HTTPTimeout != 0 {
		_, err = fmt.Fprintf(file, "http_timeout: %q\n", cfg.HTTPTimeout.String())
	}
	return err
}

func closeConfigFile(file *os.File) {
	// Config save reports write/chmod/encode errors; deferred close is cleanup.
	_ = file.Close() //nolint:errcheck
}

func (cfg *Config) ApplyEnv() error {
	if value, ok := os.LookupEnv(envServerURL); ok {
		cfg.ServerURL = value
	}
	if value, ok := os.LookupEnv(envToken); ok {
		cfg.Token = value
	}
	if value, ok := os.LookupEnv(envHTTPTimeout); ok {
		parsed, err := appconfig.ParsePositiveDuration("cli.http_timeout", value)
		if err != nil {
			return err
		}
		cfg.HTTPTimeout = parsed
	}
	return nil
}

func (cfg Config) HTTPTimeoutDuration() (time.Duration, error) {
	if cfg.HTTPTimeout == 0 {
		return DefaultHTTPTimeout, nil
	}
	if cfg.HTTPTimeout < 0 {
		return 0, fmt.Errorf("cli.http_timeout must be positive")
	}
	return cfg.HTTPTimeout, nil
}

func parse(body []byte) (Config, error) {
	var cfg Config
	for lineNumber, rawLine := range strings.Split(string(body), "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, ok := strings.Cut(line, ":")
		if !ok {
			return Config{}, fmt.Errorf("config line %d must use key: value syntax", lineNumber+1)
		}
		value = strings.Trim(strings.TrimSpace(value), `"'`)

		switch strings.TrimSpace(key) {
		case "server_url":
			cfg.ServerURL = value
		case "token":
			cfg.Token = value
		case "http_timeout":
			parsed, err := appconfig.ParsePositiveDuration("cli.http_timeout", value)
			if err != nil {
				return Config{}, err
			}
			cfg.HTTPTimeout = parsed
		default:
			return Config{}, fmt.Errorf("%s is not a supported CLI config field", strings.TrimSpace(key))
		}
	}
	return cfg, nil
}
