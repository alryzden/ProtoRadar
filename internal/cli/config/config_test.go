package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestSaveLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "protoradar", "config.yaml")

	err := Save(path, Config{
		ServerURL: "http://localhost:8080",
		Token:     "prr_token",
	})
	if err != nil {
		t.Fatalf("save: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.ServerURL != "http://localhost:8080" {
		t.Fatalf("server url = %q", cfg.ServerURL)
	}
	if cfg.Token != "prr_token" {
		t.Fatalf("token = %q", cfg.Token)
	}
	if cfg.HTTPTimeout != 0 {
		t.Fatalf("http timeout = %s", cfg.HTTPTimeout)
	}
}

func TestSaveUses0600FileMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file permission bits are not portable on windows")
	}

	path := filepath.Join(t.TempDir(), "protoradar", "config.yaml")
	if err := Save(path, Config{ServerURL: "http://localhost:8080", Token: "prr_token", HTTPTimeout: 45 * time.Second}); err != nil {
		t.Fatalf("save: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Fatalf("mode = %o, want 0600", mode)
	}
}

func TestLoadAppliesEnvOverrides(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := Save(path, Config{ServerURL: "http://file", Token: "file-token"}); err != nil {
		t.Fatalf("save: %v", err)
	}

	t.Setenv(envServerURL, "http://env")
	t.Setenv(envToken, "env-token")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.ServerURL != "http://env" {
		t.Fatalf("server url = %q", cfg.ServerURL)
	}
	if cfg.Token != "env-token" {
		t.Fatalf("token = %q", cfg.Token)
	}
}

func TestLoadAppliesHTTPTimeoutEnvOverride(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := Save(path, Config{ServerURL: "http://file", Token: "file-token", HTTPTimeout: 45 * time.Second}); err != nil {
		t.Fatalf("save: %v", err)
	}

	fileCfg, err := Load(path)
	if err != nil {
		t.Fatalf("load file config: %v", err)
	}
	if fileCfg.HTTPTimeout != 45*time.Second {
		t.Fatalf("file http timeout = %s", fileCfg.HTTPTimeout)
	}

	t.Setenv(envHTTPTimeout, "10s")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.HTTPTimeout != 10*time.Second {
		t.Fatalf("http timeout = %s", cfg.HTTPTimeout)
	}
	timeout, err := cfg.HTTPTimeoutDuration()
	if err != nil {
		t.Fatalf("timeout duration: %v", err)
	}
	if timeout != 10*time.Second {
		t.Fatalf("timeout = %s", timeout)
	}
}

func TestLoadRejectsInvalidHTTPTimeout(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("http_timeout: invalid\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := Load(path)
	if err == nil || err.Error() != "cli.http_timeout must be a valid duration" {
		t.Fatalf("error = %v", err)
	}
}

func TestLoadRejectsInvalidHTTPTimeoutEnvOverride(t *testing.T) {
	t.Setenv(envHTTPTimeout, "invalid")

	_, err := Load(filepath.Join(t.TempDir(), "missing.yaml"))
	if err == nil || err.Error() != "cli.http_timeout must be a valid duration" {
		t.Fatalf("error = %v", err)
	}
}

func TestHTTPTimeoutDurationValidation(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
		want time.Duration
		err  string
	}{
		{name: "default", cfg: Config{}, want: DefaultHTTPTimeout},
		{name: "explicit", cfg: Config{HTTPTimeout: 45 * time.Second}, want: 45 * time.Second},
		{name: "non-positive", cfg: Config{HTTPTimeout: -time.Second}, err: "cli.http_timeout must be positive"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.cfg.HTTPTimeoutDuration()
			if tt.err != "" {
				if err == nil || err.Error() != tt.err {
					t.Fatalf("error = %v, want %q", err, tt.err)
				}
				return
			}
			if err != nil {
				t.Fatalf("timeout duration: %v", err)
			}
			if got != tt.want {
				t.Fatalf("timeout = %s, want %s", got, tt.want)
			}
		})
	}
}
