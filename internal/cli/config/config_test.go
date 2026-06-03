package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
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
}

func TestSaveUses0600FileMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file permission bits are not portable on windows")
	}

	path := filepath.Join(t.TempDir(), "protoradar", "config.yaml")
	if err := Save(path, Config{ServerURL: "http://localhost:8080", Token: "prr_token"}); err != nil {
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
