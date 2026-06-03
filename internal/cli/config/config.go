package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	envServerURL = "PROTORADAR_SERVER_URL"
	envToken     = "PROTORADAR_TOKEN"
)

type Config struct {
	ServerURL string
	Token     string
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
	cfg.ApplyEnv()
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
	defer file.Close()

	if err := file.Chmod(0o600); err != nil {
		return err
	}

	_, err = fmt.Fprintf(file, "server_url: %q\ntoken: %q\n", cfg.ServerURL, cfg.Token)
	return err
}

func (cfg *Config) ApplyEnv() {
	if value, ok := os.LookupEnv(envServerURL); ok {
		cfg.ServerURL = value
	}
	if value, ok := os.LookupEnv(envToken); ok {
		cfg.Token = value
	}
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
		default:
			return Config{}, fmt.Errorf("%s is not a supported CLI config field", strings.TrimSpace(key))
		}
	}
	return cfg, nil
}
