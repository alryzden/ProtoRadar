package config

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestDefaults(t *testing.T) {
	cfg := Defaults()

	if cfg.Server.HTTPAddr != ":8080" {
		t.Fatalf("http addr = %q", cfg.Server.HTTPAddr)
	}
	if cfg.Storage.S3.Region != "us-east-1" {
		t.Fatalf("region = %q", cfg.Storage.S3.Region)
	}
	if !cfg.Storage.S3.UsePathStyle {
		t.Fatalf("use_path_style should default true")
	}
	if cfg.Registry.MaxArtifactSizeBytes != defaultMaxArtifactSizeBytes {
		t.Fatalf("max artifact size = %d", cfg.Registry.MaxArtifactSizeBytes)
	}
	if cfg.Buf.BinaryPath != "buf" {
		t.Fatalf("buf binary path = %q", cfg.Buf.BinaryPath)
	}
	if cfg.Buf.BuildTimeout != "30s" {
		t.Fatalf("buf build timeout = %q", cfg.Buf.BuildTimeout)
	}
	if cfg.Buf.LintTimeout != "30s" {
		t.Fatalf("buf lint timeout = %q", cfg.Buf.LintTimeout)
	}
	if cfg.Buf.LintMode != BufLintModeWarn {
		t.Fatalf("buf lint mode = %q", cfg.Buf.LintMode)
	}
	if !cfg.Buf.RequireConfig {
		t.Fatalf("buf require config should default true")
	}
	if cfg.Buf.MaxReportBytes != defaultBufMaxReportBytes {
		t.Fatalf("buf max report bytes = %d", cfg.Buf.MaxReportBytes)
	}
	if cfg.Breaking.MaxReportBytes != defaultBreakingMaxReportBytes {
		t.Fatalf("breaking max report bytes = %d", cfg.Breaking.MaxReportBytes)
	}
	if cfg.Breaking.MaxChanges != defaultBreakingMaxChanges {
		t.Fatalf("breaking max changes = %d", cfg.Breaking.MaxChanges)
	}
	if cfg.Breaking.DefaultAgainst != "latest" {
		t.Fatalf("breaking default against = %q", cfg.Breaking.DefaultAgainst)
	}
	if !cfg.UI.Enabled {
		t.Fatalf("ui enabled should default true")
	}
	if cfg.UI.BasePath != "/ui" {
		t.Fatalf("ui base path = %q", cfg.UI.BasePath)
	}
	if cfg.UI.StaticPath != "/ui/static" {
		t.Fatalf("ui static path = %q", cfg.UI.StaticPath)
	}
}

func TestValidateFailuresAreDeterministic(t *testing.T) {
	cfg := Defaults()
	cfg.Registry.MaxArtifactSizeBytes = 0

	want := "storage.s3.endpoint is required"
	if err := cfg.Validate(); err == nil || err.Error() != want {
		t.Fatalf("error = %v, want %q", err, want)
	}

	cfg.Storage.S3.Endpoint = "http://localhost:9000"
	want = "storage.s3.bucket is required"
	if err := cfg.Validate(); err == nil || err.Error() != want {
		t.Fatalf("error = %v, want %q", err, want)
	}

	cfg.Storage.S3.Bucket = "protoradar"
	want = "storage.s3.access_key is required"
	if err := cfg.Validate(); err == nil || err.Error() != want {
		t.Fatalf("error = %v, want %q", err, want)
	}

	cfg.Storage.S3.AccessKey = "minio"
	want = "storage.s3.secret_key is required"
	if err := cfg.Validate(); err == nil || err.Error() != want {
		t.Fatalf("error = %v, want %q", err, want)
	}

	cfg.Storage.S3.SecretKey = "password"
	want = "auth.token_hash_secret is required"
	if err := cfg.Validate(); err == nil || err.Error() != want {
		t.Fatalf("error = %v, want %q", err, want)
	}

	cfg.Auth.TokenHashSecret = "hash-secret"
	want = "registry.max_artifact_size_bytes must be positive"
	if err := cfg.Validate(); err == nil || err.Error() != want {
		t.Fatalf("error = %v, want %q", err, want)
	}

	cfg.Registry.MaxArtifactSizeBytes = 1
	cfg.Buf.BinaryPath = ""
	want = "buf.binary_path is required"
	if err := cfg.Validate(); err == nil || err.Error() != want {
		t.Fatalf("error = %v, want %q", err, want)
	}

	cfg.Buf.BinaryPath = "buf"
	cfg.Buf.BuildTimeout = "0s"
	want = "buf.build_timeout must be positive"
	if err := cfg.Validate(); err == nil || err.Error() != want {
		t.Fatalf("error = %v, want %q", err, want)
	}

	cfg.Buf.BuildTimeout = "30s"
	cfg.Buf.LintTimeout = "0s"
	want = "buf.lint_timeout must be positive"
	if err := cfg.Validate(); err == nil || err.Error() != want {
		t.Fatalf("error = %v, want %q", err, want)
	}

	cfg.Buf.LintTimeout = "30s"
	cfg.Buf.LintMode = "strict"
	want = "buf.lint_mode must be one of disabled, warn, enforce"
	if err := cfg.Validate(); err == nil || err.Error() != want {
		t.Fatalf("error = %v, want %q", err, want)
	}

	cfg.Buf.LintMode = BufLintModeWarn
	cfg.Buf.MaxReportBytes = 0
	want = "buf.max_report_bytes must be positive"
	if err := cfg.Validate(); err == nil || err.Error() != want {
		t.Fatalf("error = %v, want %q", err, want)
	}

	cfg.Buf.MaxReportBytes = 1
	cfg.Breaking.MaxReportBytes = 0
	want = "breaking.max_report_bytes must be positive"
	if err := cfg.Validate(); err == nil || err.Error() != want {
		t.Fatalf("error = %v, want %q", err, want)
	}

	cfg.Breaking.MaxReportBytes = 1
	cfg.Breaking.MaxChanges = 0
	want = "breaking.max_changes must be positive"
	if err := cfg.Validate(); err == nil || err.Error() != want {
		t.Fatalf("error = %v, want %q", err, want)
	}

	cfg.Breaking.MaxChanges = 1
	cfg.Breaking.DefaultAgainst = " "
	want = "breaking.default_against is required"
	if err := cfg.Validate(); err == nil || err.Error() != want {
		t.Fatalf("error = %v, want %q", err, want)
	}

	cfg.Breaking.DefaultAgainst = "latest"
	cfg.UI.BasePath = "ui"
	want = "ui.base_path must start with /"
	if err := cfg.Validate(); err == nil || err.Error() != want {
		t.Fatalf("error = %v, want %q", err, want)
	}

	cfg.UI.BasePath = "/ui"
	cfg.UI.StaticPath = "ui/static"
	want = "ui.static_path must start with /"
	if err := cfg.Validate(); err == nil || err.Error() != want {
		t.Fatalf("error = %v, want %q", err, want)
	}

	cfg.UI.StaticPath = "/assets"
	want = "ui.static_path must be under ui.base_path"
	if err := cfg.Validate(); err == nil || err.Error() != want {
		t.Fatalf("error = %v, want %q", err, want)
	}

	cfg.UI.StaticPath = "/ui/static"
	want = "database.url is required"
	if err := cfg.Validate(); err == nil || err.Error() != want {
		t.Fatalf("error = %v, want %q", err, want)
	}
}

func TestRuntimeMapping(t *testing.T) {
	cfg := Defaults()
	cfg.Server.HTTPAddr = ":9090"
	cfg.Database.URL = "postgres://postgres:postgres@localhost:5432/protoradar"
	cfg.Storage.S3.Endpoint = "http://localhost:9000"
	cfg.Storage.S3.Bucket = "protoradar"
	cfg.Storage.S3.AccessKey = "minio"
	cfg.Storage.S3.SecretKey = "password"
	cfg.Storage.S3.Region = "local"
	cfg.Storage.S3.UsePathStyle = false
	cfg.Auth.TokenHashSecret = "hash-secret"
	cfg.Auth.BootstrapToken = "bootstrap"
	cfg.Registry.MaxArtifactSizeBytes = 42
	cfg.Buf.BinaryPath = "/usr/local/bin/buf"
	cfg.Buf.BuildTimeout = "45s"
	cfg.Buf.LintTimeout = "15s"
	cfg.Buf.LintMode = BufLintModeEnforce
	cfg.Buf.RequireConfig = false
	cfg.Buf.MaxReportBytes = 4096
	cfg.Breaking.MaxReportBytes = 32768
	cfg.Breaking.MaxChanges = 777
	cfg.Breaking.DefaultAgainst = "v1.0.0"
	cfg.UI.Enabled = false
	cfg.UI.BasePath = "/console/"
	cfg.UI.StaticPath = "/console/assets/"

	runtime, err := cfg.Runtime()
	if err != nil {
		t.Fatalf("runtime: %v", err)
	}

	if runtime.Server.HTTPAddr != ":9090" {
		t.Fatalf("http addr = %q", runtime.Server.HTTPAddr)
	}
	if runtime.Database.URL != cfg.Database.URL {
		t.Fatalf("database url = %q", runtime.Database.URL)
	}
	if runtime.Storage.S3.Endpoint != cfg.Storage.S3.Endpoint {
		t.Fatalf("endpoint = %q", runtime.Storage.S3.Endpoint)
	}
	if runtime.Storage.S3.Bucket != cfg.Storage.S3.Bucket {
		t.Fatalf("bucket = %q", runtime.Storage.S3.Bucket)
	}
	if runtime.Storage.S3.UsePathStyle {
		t.Fatalf("use_path_style should be false")
	}
	if runtime.Auth.TokenHashSecret != "hash-secret" {
		t.Fatalf("token hash secret = %q", runtime.Auth.TokenHashSecret)
	}
	if runtime.Auth.BootstrapToken != "bootstrap" {
		t.Fatalf("bootstrap token = %q", runtime.Auth.BootstrapToken)
	}
	if runtime.Registry.MaxArtifactSizeBytes != 42 {
		t.Fatalf("max artifact size = %d", runtime.Registry.MaxArtifactSizeBytes)
	}
	if runtime.Buf.BinaryPath != "/usr/local/bin/buf" {
		t.Fatalf("buf binary path = %q", runtime.Buf.BinaryPath)
	}
	if runtime.Buf.BuildTimeout != 45*time.Second {
		t.Fatalf("buf build timeout = %s", runtime.Buf.BuildTimeout)
	}
	if runtime.Buf.LintTimeout != 15*time.Second {
		t.Fatalf("buf lint timeout = %s", runtime.Buf.LintTimeout)
	}
	if runtime.Buf.LintMode != BufLintModeEnforce {
		t.Fatalf("buf lint mode = %q", runtime.Buf.LintMode)
	}
	if runtime.Buf.RequireConfig {
		t.Fatalf("buf require config should be false")
	}
	if runtime.Buf.MaxReportBytes != 4096 {
		t.Fatalf("buf max report bytes = %d", runtime.Buf.MaxReportBytes)
	}
	if runtime.Breaking.MaxReportBytes != 32768 {
		t.Fatalf("breaking max report bytes = %d", runtime.Breaking.MaxReportBytes)
	}
	if runtime.Breaking.MaxChanges != 777 {
		t.Fatalf("breaking max changes = %d", runtime.Breaking.MaxChanges)
	}
	if runtime.Breaking.DefaultAgainst != "v1.0.0" {
		t.Fatalf("breaking default against = %q", runtime.Breaking.DefaultAgainst)
	}
	if runtime.UI.Enabled {
		t.Fatalf("ui enabled should be false")
	}
	if runtime.UI.BasePath != "/console" {
		t.Fatalf("ui base path = %q", runtime.UI.BasePath)
	}
	if runtime.UI.StaticPath != "/console/assets" {
		t.Fatalf("ui static path = %q", runtime.UI.StaticPath)
	}
}

func TestLoadFileAndEnvOverrides(t *testing.T) {
	file, err := os.CreateTemp(t.TempDir(), "config-*.yaml")
	if err != nil {
		t.Fatalf("temp file: %v", err)
	}
	_, err = file.WriteString(strings.TrimSpace(`
storage:
  s3:
    endpoint: http://localhost:9000
    region: local
    bucket: yaml-bucket
    access_key: yaml-access
    secret_key: yaml-secret
    use_path_style: true
auth:
  token_hash_secret: yaml-secret
registry:
  max_artifact_size_bytes: 128
buf:
  binary_path: /yaml/bin/buf
  build_timeout: 20s
  lint_timeout: 25s
  lint_mode: disabled
  require_config: false
  max_report_bytes: 8192
breaking:
  max_report_bytes: 32768
  max_changes: 200
  default_against: v1.0.0
ui:
  enabled: false
  base_path: /console
  static_path: /console/assets
database:
  url: postgres://postgres:postgres@localhost:5432/protoradar
`))
	if err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close config: %v", err)
	}

	t.Setenv("PROTORADAR_STORAGE_S3_BUCKET", "env-bucket")
	t.Setenv("PROTORADAR_REGISTRY_MAX_ARTIFACT_SIZE_BYTES", "256")
	t.Setenv("PROTORADAR_BUF_BINARY_PATH", "/env/bin/buf")
	t.Setenv("PROTORADAR_BUF_BUILD_TIMEOUT", "40s")
	t.Setenv("PROTORADAR_BUF_LINT_TIMEOUT", "50s")
	t.Setenv("PROTORADAR_BUF_LINT_MODE", "enforce")
	t.Setenv("PROTORADAR_BUF_REQUIRE_CONFIG", "true")
	t.Setenv("PROTORADAR_BUF_MAX_REPORT_BYTES", "12345")
	t.Setenv("PROTORADAR_BREAKING_MAX_REPORT_BYTES", "54321")
	t.Setenv("PROTORADAR_BREAKING_MAX_CHANGES", "321")
	t.Setenv("PROTORADAR_BREAKING_DEFAULT_AGAINST", "latest")
	t.Setenv("PROTORADAR_UI_ENABLED", "true")
	t.Setenv("PROTORADAR_UI_BASE_PATH", "/ui")
	t.Setenv("PROTORADAR_UI_STATIC_PATH", "/ui/static")
	t.Setenv("PROTORADAR_DATABASE_URL", "postgres://env")

	cfg, err := LoadFile(file.Name())
	if err != nil {
		t.Fatalf("load file: %v", err)
	}

	if cfg.Storage.S3.Bucket != "env-bucket" {
		t.Fatalf("bucket = %q", cfg.Storage.S3.Bucket)
	}
	if cfg.Registry.MaxArtifactSizeBytes != 256 {
		t.Fatalf("max artifact size = %d", cfg.Registry.MaxArtifactSizeBytes)
	}
	if cfg.Database.URL != "postgres://env" {
		t.Fatalf("database url = %q", cfg.Database.URL)
	}
	if cfg.Buf.BinaryPath != "/env/bin/buf" {
		t.Fatalf("buf binary path = %q", cfg.Buf.BinaryPath)
	}
	if cfg.Buf.BuildTimeout != "40s" {
		t.Fatalf("buf build timeout = %q", cfg.Buf.BuildTimeout)
	}
	if cfg.Buf.LintTimeout != "50s" {
		t.Fatalf("buf lint timeout = %q", cfg.Buf.LintTimeout)
	}
	if cfg.Buf.LintMode != BufLintModeEnforce {
		t.Fatalf("buf lint mode = %q", cfg.Buf.LintMode)
	}
	if !cfg.Buf.RequireConfig {
		t.Fatalf("buf require config should be true")
	}
	if cfg.Buf.MaxReportBytes != 12345 {
		t.Fatalf("buf max report bytes = %d", cfg.Buf.MaxReportBytes)
	}
	if cfg.Breaking.MaxReportBytes != 54321 {
		t.Fatalf("breaking max report bytes = %d", cfg.Breaking.MaxReportBytes)
	}
	if cfg.Breaking.MaxChanges != 321 {
		t.Fatalf("breaking max changes = %d", cfg.Breaking.MaxChanges)
	}
	if cfg.Breaking.DefaultAgainst != "latest" {
		t.Fatalf("breaking default against = %q", cfg.Breaking.DefaultAgainst)
	}
	if !cfg.UI.Enabled {
		t.Fatalf("ui enabled should be true")
	}
	if cfg.UI.BasePath != "/ui" {
		t.Fatalf("ui base path = %q", cfg.UI.BasePath)
	}
	if cfg.UI.StaticPath != "/ui/static" {
		t.Fatalf("ui static path = %q", cfg.UI.StaticPath)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
}

func TestBufValidationFailures(t *testing.T) {
	tests := []struct {
		name string
		edit func(*Config)
		want string
	}{
		{
			name: "invalid lint mode",
			edit: func(cfg *Config) {
				cfg.Buf.LintMode = "strict"
			},
			want: "buf.lint_mode must be one of disabled, warn, enforce",
		},
		{
			name: "missing binary path",
			edit: func(cfg *Config) {
				cfg.Buf.BinaryPath = " "
			},
			want: "buf.binary_path is required",
		},
		{
			name: "non-positive build timeout",
			edit: func(cfg *Config) {
				cfg.Buf.BuildTimeout = "-1s"
			},
			want: "buf.build_timeout must be positive",
		},
		{
			name: "non-positive lint timeout",
			edit: func(cfg *Config) {
				cfg.Buf.LintTimeout = "0s"
			},
			want: "buf.lint_timeout must be positive",
		},
		{
			name: "non-positive max report bytes",
			edit: func(cfg *Config) {
				cfg.Buf.MaxReportBytes = 0
			},
			want: "buf.max_report_bytes must be positive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			tt.edit(&cfg)

			if err := cfg.Validate(); err == nil || err.Error() != tt.want {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestBreakingValidationFailures(t *testing.T) {
	tests := []struct {
		name string
		edit func(*Config)
		want string
	}{
		{
			name: "non-positive max report bytes",
			edit: func(cfg *Config) {
				cfg.Breaking.MaxReportBytes = 0
			},
			want: "breaking.max_report_bytes must be positive",
		},
		{
			name: "non-positive max changes",
			edit: func(cfg *Config) {
				cfg.Breaking.MaxChanges = 0
			},
			want: "breaking.max_changes must be positive",
		},
		{
			name: "missing default against",
			edit: func(cfg *Config) {
				cfg.Breaking.DefaultAgainst = " "
			},
			want: "breaking.default_against is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			tt.edit(&cfg)

			if err := cfg.Validate(); err == nil || err.Error() != tt.want {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestUIValidationFailures(t *testing.T) {
	tests := []struct {
		name string
		edit func(*Config)
		want string
	}{
		{
			name: "base path must be absolute",
			edit: func(cfg *Config) {
				cfg.UI.BasePath = "ui"
			},
			want: "ui.base_path must start with /",
		},
		{
			name: "static path must be absolute",
			edit: func(cfg *Config) {
				cfg.UI.StaticPath = "ui/static"
			},
			want: "ui.static_path must start with /",
		},
		{
			name: "static path must be under base path",
			edit: func(cfg *Config) {
				cfg.UI.BasePath = "/ui"
				cfg.UI.StaticPath = "/assets"
			},
			want: "ui.static_path must be under ui.base_path",
		},
		{
			name: "static path must not equal base path",
			edit: func(cfg *Config) {
				cfg.UI.BasePath = "/ui"
				cfg.UI.StaticPath = "/ui"
			},
			want: "ui.static_path must be under ui.base_path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			tt.edit(&cfg)

			if err := cfg.Validate(); err == nil || err.Error() != tt.want {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
		})
	}
}

func validConfig() Config {
	cfg := Defaults()
	cfg.Database.URL = "postgres://postgres:postgres@localhost:5432/protoradar"
	cfg.Storage.S3.Endpoint = "http://localhost:9000"
	cfg.Storage.S3.Bucket = "protoradar"
	cfg.Storage.S3.AccessKey = "minio"
	cfg.Storage.S3.SecretKey = "password"
	cfg.Auth.TokenHashSecret = "hash-secret"
	return cfg
}
