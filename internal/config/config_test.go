package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDefaults(t *testing.T) {
	cfg := Defaults()

	if cfg.Server.HTTPAddr != ":8080" {
		t.Fatalf("http addr = %q", cfg.Server.HTTPAddr)
	}
	if cfg.Server.HTTPReadHeaderTimeout != 5*time.Second {
		t.Fatalf("http read header timeout = %q", cfg.Server.HTTPReadHeaderTimeout)
	}
	if cfg.Server.HTTPReadTimeout != 30*time.Second {
		t.Fatalf("http read timeout = %q", cfg.Server.HTTPReadTimeout)
	}
	if cfg.Server.HTTPWriteTimeout != 60*time.Second {
		t.Fatalf("http write timeout = %q", cfg.Server.HTTPWriteTimeout)
	}
	if cfg.Server.HTTPIdleTimeout != 120*time.Second {
		t.Fatalf("http idle timeout = %q", cfg.Server.HTTPIdleTimeout)
	}
	if cfg.Server.HTTPMaxHeaderBytes != defaultHTTPMaxHeaderBytes {
		t.Fatalf("http max header bytes = %d", cfg.Server.HTTPMaxHeaderBytes)
	}
	if cfg.Server.MaxRequestBodyBytes != defaultMaxRequestBodyBytes {
		t.Fatalf("max request body bytes = %d", cfg.Server.MaxRequestBodyBytes)
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
	if cfg.Buf.BuildTimeout != 30*time.Second {
		t.Fatalf("buf build timeout = %q", cfg.Buf.BuildTimeout)
	}
	if cfg.Buf.LintTimeout != 30*time.Second {
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
	if !cfg.Governance.Enabled {
		t.Fatalf("governance enabled should default true")
	}
	if cfg.Governance.ProductionEnvironments != "production,prod" {
		t.Fatalf("governance production environments = %q", cfg.Governance.ProductionEnvironments)
	}
	if !cfg.Governance.AllowMaintainerApproval {
		t.Fatalf("governance allow maintainer approval should default true")
	}
	if cfg.Governance.ActorOverrideEnabled {
		t.Fatalf("governance actor override should default disabled")
	}
	if cfg.OutboxPublisher.Enabled {
		t.Fatalf("outbox publisher should default disabled")
	}
	if cfg.OutboxPublisher.BatchSize != defaultOutboxPublisherBatchSize {
		t.Fatalf("outbox publisher batch size = %d", cfg.OutboxPublisher.BatchSize)
	}
	if cfg.OutboxPublisher.PollInterval != 5*time.Second || cfg.OutboxPublisher.LeaseDuration != 30*time.Second {
		t.Fatalf("outbox publisher intervals = %q/%q", cfg.OutboxPublisher.PollInterval, cfg.OutboxPublisher.LeaseDuration)
	}
	if cfg.OutboxPublisher.MaxAttempts != defaultOutboxPublisherMaxAttempts {
		t.Fatalf("outbox publisher max attempts = %d", cfg.OutboxPublisher.MaxAttempts)
	}
	if cfg.OutboxPublisher.InitialRetryBackoff != 30*time.Second || cfg.OutboxPublisher.MaxRetryBackoff != 5*time.Minute {
		t.Fatalf("outbox publisher backoff = %q/%q", cfg.OutboxPublisher.InitialRetryBackoff, cfg.OutboxPublisher.MaxRetryBackoff)
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
	if cfg.Log.Level != LogLevelInfo {
		t.Fatalf("log level = %q", cfg.Log.Level)
	}
	if cfg.Log.Format != LogFormatJSON {
		t.Fatalf("log format = %q", cfg.Log.Format)
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
	cfg.Buf.BuildTimeout = 0
	want = "buf.build_timeout must be positive"
	if err := cfg.Validate(); err == nil || err.Error() != want {
		t.Fatalf("error = %v, want %q", err, want)
	}

	cfg.Buf.BuildTimeout = 30 * time.Second
	cfg.Buf.LintTimeout = 0
	want = "buf.lint_timeout must be positive"
	if err := cfg.Validate(); err == nil || err.Error() != want {
		t.Fatalf("error = %v, want %q", err, want)
	}

	cfg.Buf.LintTimeout = 30 * time.Second
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

func TestServerValidationFailures(t *testing.T) {
	tests := []struct {
		name string
		edit func(*Config)
		want string
	}{
		{
			name: "non-positive read header timeout",
			edit: func(cfg *Config) {
				cfg.Server.HTTPReadHeaderTimeout = 0
			},
			want: "server.http_read_header_timeout must be positive",
		},
		{
			name: "non-positive read timeout",
			edit: func(cfg *Config) {
				cfg.Server.HTTPReadTimeout = 0
			},
			want: "server.http_read_timeout must be positive",
		},
		{
			name: "non-positive write timeout",
			edit: func(cfg *Config) {
				cfg.Server.HTTPWriteTimeout = -time.Second
			},
			want: "server.http_write_timeout must be positive",
		},
		{
			name: "non-positive idle timeout",
			edit: func(cfg *Config) {
				cfg.Server.HTTPIdleTimeout = 0
			},
			want: "server.http_idle_timeout must be positive",
		},
		{
			name: "non-positive max header bytes",
			edit: func(cfg *Config) {
				cfg.Server.HTTPMaxHeaderBytes = 0
			},
			want: "server.http_max_header_bytes must be positive",
		},
		{
			name: "non-positive max request body bytes",
			edit: func(cfg *Config) {
				cfg.Server.MaxRequestBodyBytes = 0
			},
			want: "server.max_request_body_bytes must be positive",
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

func TestRuntimeMapping(t *testing.T) {
	cfg := Defaults()
	cfg.Server.HTTPAddr = ":9090"
	cfg.Server.HTTPReadHeaderTimeout = 6 * time.Second
	cfg.Server.HTTPReadTimeout = 31 * time.Second
	cfg.Server.HTTPWriteTimeout = 61 * time.Second
	cfg.Server.HTTPIdleTimeout = 121 * time.Second
	cfg.Server.HTTPMaxHeaderBytes = 2048
	cfg.Server.MaxRequestBodyBytes = 12345
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
	cfg.Buf.BuildTimeout = 45 * time.Second
	cfg.Buf.LintTimeout = 15 * time.Second
	cfg.Buf.LintMode = BufLintModeEnforce
	cfg.Buf.RequireConfig = false
	cfg.Buf.MaxReportBytes = 4096
	cfg.Breaking.MaxReportBytes = 32768
	cfg.Breaking.MaxChanges = 777
	cfg.Breaking.DefaultAgainst = "v1.0.0"
	cfg.Governance.Enabled = true
	cfg.Governance.ProductionEnvironments = "production, live"
	cfg.Governance.AllowMaintainerApproval = false
	cfg.Governance.ActorOverrideEnabled = true
	cfg.OutboxPublisher.Enabled = true
	cfg.OutboxPublisher.BatchSize = 12
	cfg.OutboxPublisher.PollInterval = 7 * time.Second
	cfg.OutboxPublisher.LeaseDuration = 45 * time.Second
	cfg.OutboxPublisher.MaxAttempts = 9
	cfg.OutboxPublisher.InitialRetryBackoff = 3 * time.Second
	cfg.OutboxPublisher.MaxRetryBackoff = 30 * time.Second
	cfg.UI.Enabled = false
	cfg.UI.BasePath = "/console/"
	cfg.UI.StaticPath = "/console/assets/"
	cfg.Log.Level = LogLevelDebug
	cfg.Log.Format = LogFormatText

	runtime, err := cfg.Runtime()
	if err != nil {
		t.Fatalf("runtime: %v", err)
	}

	if runtime.Server.HTTPAddr != ":9090" {
		t.Fatalf("http addr = %q", runtime.Server.HTTPAddr)
	}
	if runtime.Server.HTTPReadHeaderTimeout != 6*time.Second {
		t.Fatalf("http read header timeout = %s", runtime.Server.HTTPReadHeaderTimeout)
	}
	if runtime.Server.HTTPReadTimeout != 31*time.Second {
		t.Fatalf("http read timeout = %s", runtime.Server.HTTPReadTimeout)
	}
	if runtime.Server.HTTPWriteTimeout != 61*time.Second {
		t.Fatalf("http write timeout = %s", runtime.Server.HTTPWriteTimeout)
	}
	if runtime.Server.HTTPIdleTimeout != 121*time.Second {
		t.Fatalf("http idle timeout = %s", runtime.Server.HTTPIdleTimeout)
	}
	if runtime.Server.HTTPMaxHeaderBytes != 2048 {
		t.Fatalf("http max header bytes = %d", runtime.Server.HTTPMaxHeaderBytes)
	}
	if runtime.Server.MaxRequestBodyBytes != 12345 {
		t.Fatalf("max request body bytes = %d", runtime.Server.MaxRequestBodyBytes)
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
	if !runtime.Governance.Enabled {
		t.Fatalf("governance enabled should be true")
	}
	if len(runtime.Governance.ProductionEnvironments) != 2 || runtime.Governance.ProductionEnvironments[0] != "production" || runtime.Governance.ProductionEnvironments[1] != "live" {
		t.Fatalf("governance production environments = %#v", runtime.Governance.ProductionEnvironments)
	}
	if runtime.Governance.AllowMaintainerApproval {
		t.Fatalf("governance allow maintainer approval should be false")
	}
	if !runtime.Governance.ActorOverrideEnabled {
		t.Fatalf("governance actor override should be true")
	}
	if !runtime.OutboxPublisher.Enabled {
		t.Fatalf("outbox publisher enabled should be true")
	}
	if runtime.OutboxPublisher.BatchSize != 12 || runtime.OutboxPublisher.MaxAttempts != 9 {
		t.Fatalf("outbox publisher batch/max attempts = %d/%d", runtime.OutboxPublisher.BatchSize, runtime.OutboxPublisher.MaxAttempts)
	}
	if runtime.OutboxPublisher.PollInterval != 7*time.Second || runtime.OutboxPublisher.LeaseDuration != 45*time.Second {
		t.Fatalf("outbox publisher intervals = %s/%s", runtime.OutboxPublisher.PollInterval, runtime.OutboxPublisher.LeaseDuration)
	}
	if runtime.OutboxPublisher.InitialRetryBackoff != 3*time.Second || runtime.OutboxPublisher.MaxRetryBackoff != 30*time.Second {
		t.Fatalf("outbox publisher backoff = %s/%s", runtime.OutboxPublisher.InitialRetryBackoff, runtime.OutboxPublisher.MaxRetryBackoff)
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
	if runtime.Log.Level != LogLevelDebug {
		t.Fatalf("log level = %q", runtime.Log.Level)
	}
	if runtime.Log.Format != LogFormatText {
		t.Fatalf("log format = %q", runtime.Log.Format)
	}
}

func TestLoadFileEmptyPathUsesDefaultsAndEnv(t *testing.T) {
	t.Setenv("PROTORADAR_SERVER_HTTP_ADDR", ":9091")

	cfg, err := LoadFile("")
	if err != nil {
		t.Fatalf("load file: %v", err)
	}

	if cfg.Server.HTTPAddr != ":9091" {
		t.Fatalf("http addr = %q", cfg.Server.HTTPAddr)
	}
	if cfg.Buf.BinaryPath != "buf" {
		t.Fatalf("buf binary path = %q", cfg.Buf.BinaryPath)
	}
}

func TestYAMLFileOverridesDefaults(t *testing.T) {
	path := writeConfigFile(t, `
server:
  http_addr: :9090
  http_read_timeout: 35s
storage:
  s3:
    endpoint: http://localhost:9000
    bucket: yaml-bucket
    access_key: yaml-access
    secret_key: yaml-secret
auth:
  token_hash_secret: yaml-hash-secret
database:
  url: postgres://yaml
`)

	cfg, err := LoadFile(path)
	if err != nil {
		t.Fatalf("load file: %v", err)
	}

	if cfg.Server.HTTPAddr != ":9090" {
		t.Fatalf("http addr = %q", cfg.Server.HTTPAddr)
	}
	if cfg.Server.HTTPReadTimeout != 35*time.Second {
		t.Fatalf("http read timeout = %q", cfg.Server.HTTPReadTimeout)
	}
	if cfg.Storage.S3.Region != "us-east-1" {
		t.Fatalf("default region = %q", cfg.Storage.S3.Region)
	}
}

func TestLoadFileAndEnvOverrides(t *testing.T) {
	path := writeConfigFile(t, `
server:
  http_read_header_timeout: 6s
  http_read_timeout: 31s
  http_write_timeout: 61s
  http_idle_timeout: 121s
  http_max_header_bytes: 2048
  max_request_body_bytes: 256
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
governance:
  enabled: true
  production_environments: production, live
  allow_maintainer_approval: true
  actor_override_enabled: false
outbox_publisher:
  enabled: true
  batch_size: 25
  poll_interval: 11s
  lease_duration: 22s
  max_attempts: 6
  initial_backoff: 4s
  max_backoff: 44s
ui:
  enabled: false
  base_path: /console
  static_path: /console/assets
log:
  level: warn
  format: text
database:
  url: postgres://postgres:postgres@localhost:5432/protoradar
`)

	t.Setenv("PROTORADAR_STORAGE_S3_BUCKET", "env-bucket")
	t.Setenv("PROTORADAR_SERVER_HTTP_READ_HEADER_TIMEOUT", "7s")
	t.Setenv("PROTORADAR_SERVER_HTTP_READ_TIMEOUT", "32s")
	t.Setenv("PROTORADAR_SERVER_HTTP_WRITE_TIMEOUT", "62s")
	t.Setenv("PROTORADAR_SERVER_HTTP_IDLE_TIMEOUT", "122s")
	t.Setenv("PROTORADAR_SERVER_HTTP_MAX_HEADER_BYTES", "4096")
	t.Setenv("PROTORADAR_SERVER_MAX_REQUEST_BODY_BYTES", "512")
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
	t.Setenv("PROTORADAR_GOVERNANCE_ENABLED", "false")
	t.Setenv("PROTORADAR_GOVERNANCE_PRODUCTION_ENVIRONMENTS", "prod,live")
	t.Setenv("PROTORADAR_GOVERNANCE_ALLOW_MAINTAINER_APPROVAL", "false")
	t.Setenv("PROTORADAR_GOVERNANCE_ACTOR_OVERRIDE_ENABLED", "true")
	t.Setenv("PROTORADAR_OUTBOX_PUBLISHER_ENABLED", "false")
	t.Setenv("PROTORADAR_OUTBOX_PUBLISHER_BATCH_SIZE", "30")
	t.Setenv("PROTORADAR_OUTBOX_PUBLISHER_POLL_INTERVAL", "12s")
	t.Setenv("PROTORADAR_OUTBOX_PUBLISHER_LEASE_DURATION", "24s")
	t.Setenv("PROTORADAR_OUTBOX_PUBLISHER_MAX_ATTEMPTS", "7")
	t.Setenv("PROTORADAR_OUTBOX_PUBLISHER_INITIAL_BACKOFF", "5s")
	t.Setenv("PROTORADAR_OUTBOX_PUBLISHER_MAX_BACKOFF", "55s")
	t.Setenv("PROTORADAR_UI_ENABLED", "true")
	t.Setenv("PROTORADAR_UI_BASE_PATH", "/ui")
	t.Setenv("PROTORADAR_UI_STATIC_PATH", "/ui/static")
	t.Setenv("PROTORADAR_LOG_LEVEL", "debug")
	t.Setenv("PROTORADAR_LOG_FORMAT", "json")
	t.Setenv("PROTORADAR_DATABASE_URL", "postgres://env")

	cfg, err := LoadFile(path)
	if err != nil {
		t.Fatalf("load file: %v", err)
	}

	if cfg.Storage.S3.Bucket != "env-bucket" {
		t.Fatalf("bucket = %q", cfg.Storage.S3.Bucket)
	}
	if cfg.Server.HTTPReadHeaderTimeout != 7*time.Second {
		t.Fatalf("http read header timeout = %q", cfg.Server.HTTPReadHeaderTimeout)
	}
	if cfg.Server.HTTPReadTimeout != 32*time.Second {
		t.Fatalf("http read timeout = %q", cfg.Server.HTTPReadTimeout)
	}
	if cfg.Server.HTTPWriteTimeout != 62*time.Second {
		t.Fatalf("http write timeout = %q", cfg.Server.HTTPWriteTimeout)
	}
	if cfg.Server.HTTPIdleTimeout != 122*time.Second {
		t.Fatalf("http idle timeout = %q", cfg.Server.HTTPIdleTimeout)
	}
	if cfg.Server.HTTPMaxHeaderBytes != 4096 {
		t.Fatalf("http max header bytes = %d", cfg.Server.HTTPMaxHeaderBytes)
	}
	if cfg.Server.MaxRequestBodyBytes != 512 {
		t.Fatalf("max request body bytes = %d", cfg.Server.MaxRequestBodyBytes)
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
	if cfg.Buf.BuildTimeout != 40*time.Second {
		t.Fatalf("buf build timeout = %q", cfg.Buf.BuildTimeout)
	}
	if cfg.Buf.LintTimeout != 50*time.Second {
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
	if cfg.Governance.Enabled {
		t.Fatalf("governance enabled should be false")
	}
	if cfg.Governance.ProductionEnvironments != "prod,live" {
		t.Fatalf("governance production environments = %q", cfg.Governance.ProductionEnvironments)
	}
	if cfg.Governance.AllowMaintainerApproval {
		t.Fatalf("governance allow maintainer approval should be false")
	}
	if !cfg.Governance.ActorOverrideEnabled {
		t.Fatalf("governance actor override should be true")
	}
	if cfg.OutboxPublisher.Enabled {
		t.Fatalf("outbox publisher enabled should be false")
	}
	if cfg.OutboxPublisher.BatchSize != 30 || cfg.OutboxPublisher.MaxAttempts != 7 {
		t.Fatalf("outbox publisher batch/max attempts = %d/%d", cfg.OutboxPublisher.BatchSize, cfg.OutboxPublisher.MaxAttempts)
	}
	if cfg.OutboxPublisher.PollInterval != 12*time.Second || cfg.OutboxPublisher.LeaseDuration != 24*time.Second {
		t.Fatalf("outbox publisher intervals = %q/%q", cfg.OutboxPublisher.PollInterval, cfg.OutboxPublisher.LeaseDuration)
	}
	if cfg.OutboxPublisher.InitialRetryBackoff != 5*time.Second || cfg.OutboxPublisher.MaxRetryBackoff != 55*time.Second {
		t.Fatalf("outbox publisher backoff = %q/%q", cfg.OutboxPublisher.InitialRetryBackoff, cfg.OutboxPublisher.MaxRetryBackoff)
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
	if cfg.Log.Level != LogLevelDebug {
		t.Fatalf("log level = %q", cfg.Log.Level)
	}
	if cfg.Log.Format != LogFormatJSON {
		t.Fatalf("log format = %q", cfg.Log.Format)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
}

func TestYAMLDurationValuesDecodeIntoTypedFields(t *testing.T) {
	path := writeConfigFile(t, `
server:
  http_read_header_timeout: 8s
  http_read_timeout: 33s
buf:
  build_timeout: 1m
outbox_publisher:
  poll_interval: 9s
`)

	cfg, err := LoadFile(path)
	if err != nil {
		t.Fatalf("load file: %v", err)
	}

	if cfg.Server.HTTPReadHeaderTimeout != 8*time.Second {
		t.Fatalf("http read header timeout = %s", cfg.Server.HTTPReadHeaderTimeout)
	}
	if cfg.Server.HTTPReadTimeout != 33*time.Second {
		t.Fatalf("http read timeout = %s", cfg.Server.HTTPReadTimeout)
	}
	if cfg.Buf.BuildTimeout != time.Minute {
		t.Fatalf("buf build timeout = %s", cfg.Buf.BuildTimeout)
	}
	if cfg.OutboxPublisher.PollInterval != 9*time.Second {
		t.Fatalf("outbox poll interval = %s", cfg.OutboxPublisher.PollInterval)
	}
}

func TestEnvDurationValuesDecodeIntoTypedFields(t *testing.T) {
	t.Setenv("PROTORADAR_SERVER_HTTP_READ_HEADER_TIMEOUT", "8s")
	t.Setenv("PROTORADAR_SERVER_HTTP_READ_TIMEOUT", "33s")
	t.Setenv("PROTORADAR_BUF_BUILD_TIMEOUT", "1m")
	t.Setenv("PROTORADAR_OUTBOX_PUBLISHER_POLL_INTERVAL", "9s")

	cfg, err := LoadFile("")
	if err != nil {
		t.Fatalf("load file: %v", err)
	}

	if cfg.Server.HTTPReadHeaderTimeout != 8*time.Second {
		t.Fatalf("http read header timeout = %s", cfg.Server.HTTPReadHeaderTimeout)
	}
	if cfg.Server.HTTPReadTimeout != 33*time.Second {
		t.Fatalf("http read timeout = %s", cfg.Server.HTTPReadTimeout)
	}
	if cfg.Buf.BuildTimeout != time.Minute {
		t.Fatalf("buf build timeout = %s", cfg.Buf.BuildTimeout)
	}
	if cfg.OutboxPublisher.PollInterval != 9*time.Second {
		t.Fatalf("outbox poll interval = %s", cfg.OutboxPublisher.PollInterval)
	}
}

func TestInvalidDurationValuesReturnClearErrors(t *testing.T) {
	t.Run("yaml", func(t *testing.T) {
		path := writeConfigFile(t, `
server:
  http_read_timeout: soon
`)

		_, err := LoadFile(path)
		if err == nil || err.Error() != "server.http_read_timeout must be a valid duration" {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("env", func(t *testing.T) {
		t.Setenv("PROTORADAR_SERVER_HTTP_READ_TIMEOUT", "soon")

		_, err := LoadFile("")
		if err == nil || err.Error() != "server.http_read_timeout must be a valid duration" {
			t.Fatalf("error = %v", err)
		}
	})
}

func TestNumericSizeValuesDecodeFromYAMLAndEnv(t *testing.T) {
	path := writeConfigFile(t, `
server:
  http_max_header_bytes: 2048
  max_request_body_bytes: 4096
registry:
  max_artifact_size_bytes: 8192
`)
	t.Setenv("PROTORADAR_SERVER_MAX_REQUEST_BODY_BYTES", "16384")

	cfg, err := LoadFile(path)
	if err != nil {
		t.Fatalf("load file: %v", err)
	}

	if cfg.Server.HTTPMaxHeaderBytes != 2048 {
		t.Fatalf("http max header bytes = %d", cfg.Server.HTTPMaxHeaderBytes)
	}
	if cfg.Server.MaxRequestBodyBytes != 16384 {
		t.Fatalf("max request body bytes = %d", cfg.Server.MaxRequestBodyBytes)
	}
	if cfg.Registry.MaxArtifactSizeBytes != 8192 {
		t.Fatalf("max artifact size bytes = %d", cfg.Registry.MaxArtifactSizeBytes)
	}
}

func TestInvalidNumericValuesReturnClearErrors(t *testing.T) {
	t.Run("yaml", func(t *testing.T) {
		path := writeConfigFile(t, `
server:
  max_request_body_bytes: many
`)

		_, err := LoadFile(path)
		if err == nil || err.Error() != "server.max_request_body_bytes must be an integer" {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("env", func(t *testing.T) {
		t.Setenv("PROTORADAR_SERVER_MAX_REQUEST_BODY_BYTES", "many")

		_, err := LoadFile("")
		if err == nil || err.Error() != "server.max_request_body_bytes must be an integer" {
			t.Fatalf("error = %v", err)
		}
	})
}

func TestOldServerEnvNamesAreIgnored(t *testing.T) {
	t.Setenv("PROTORADAR_HTTP_ADDR", ":9999")
	t.Setenv("PROTORADAR_MAX_REQUEST_BODY_BYTES", "1")

	cfg, err := LoadFile("")
	if err != nil {
		t.Fatalf("load file: %v", err)
	}

	if cfg.Server.HTTPAddr != ":8080" {
		t.Fatalf("http addr = %q", cfg.Server.HTTPAddr)
	}
	if cfg.Server.MaxRequestBodyBytes != defaultMaxRequestBodyBytes {
		t.Fatalf("max request body bytes = %d", cfg.Server.MaxRequestBodyBytes)
	}
}

func TestUnknownYAMLKeyRejected(t *testing.T) {
	path := writeConfigFile(t, `
server:
  unsupported: true
`)

	_, err := LoadFile(path)
	if err == nil || err.Error() != "server.unsupported is not a supported config field" {
		t.Fatalf("error = %v", err)
	}
}

func TestInvalidYAMLRejected(t *testing.T) {
	path := writeConfigFile(t, `
server:
  http_addr: [
`)

	_, err := LoadFile(path)
	if err == nil || !strings.Contains(err.Error(), "load config file") {
		t.Fatalf("error = %v", err)
	}
}

func TestMissingConfigFileRejected(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.yaml")

	_, err := LoadFile(path)
	if err == nil || !strings.Contains(err.Error(), "load config file") || !strings.Contains(err.Error(), "missing.yaml") {
		t.Fatalf("error = %v", err)
	}
}

func TestEmptyYAMLFileIsValid(t *testing.T) {
	path := writeConfigFile(t, "")

	cfg, err := LoadFile(path)
	if err != nil {
		t.Fatalf("load file: %v", err)
	}
	if cfg.Server.HTTPAddr != ":8080" {
		t.Fatalf("http addr = %q", cfg.Server.HTTPAddr)
	}
}

func TestConfigErrorsDoNotPrintSecrets(t *testing.T) {
	t.Setenv("PROTORADAR_STORAGE_S3_SECRET_KEY", "super-secret-value")
	t.Setenv("PROTORADAR_STORAGE_S3_USE_PATH_STYLE", "definitely")

	_, err := LoadFile("")
	if err == nil {
		t.Fatalf("expected error")
	}
	if strings.Contains(err.Error(), "super-secret-value") {
		t.Fatalf("error leaked secret: %v", err)
	}
}

func TestLogValidationFailures(t *testing.T) {
	tests := []struct {
		name string
		edit func(*Config)
		want string
	}{
		{
			name: "invalid level",
			edit: func(cfg *Config) {
				cfg.Log.Level = "trace"
			},
			want: "log.level must be one of debug, info, warn, error",
		},
		{
			name: "invalid format",
			edit: func(cfg *Config) {
				cfg.Log.Format = "pretty"
			},
			want: "log.format must be one of json, text",
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
				cfg.Buf.BuildTimeout = -time.Second
			},
			want: "buf.build_timeout must be positive",
		},
		{
			name: "non-positive lint timeout",
			edit: func(cfg *Config) {
				cfg.Buf.LintTimeout = 0
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

func TestGovernanceValidationFailures(t *testing.T) {
	tests := []struct {
		name string
		edit func(*Config)
		want string
	}{
		{
			name: "missing production environments",
			edit: func(cfg *Config) {
				cfg.Governance.ProductionEnvironments = " , "
			},
			want: "governance.production_environments must contain at least one value",
		},
		{
			name: "disabled allows empty production environments",
			edit: func(cfg *Config) {
				cfg.Governance.Enabled = false
				cfg.Governance.ProductionEnvironments = " "
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			tt.edit(&cfg)

			err := cfg.Validate()
			if tt.want == "" {
				if err != nil {
					t.Fatalf("error = %v, want nil", err)
				}
				return
			}
			if err == nil || err.Error() != tt.want {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestGovernanceActorOverrideInvalidConfigRejected(t *testing.T) {
	t.Run("yaml", func(t *testing.T) {
		file, err := os.CreateTemp(t.TempDir(), "protoradar-*.yaml")
		if err != nil {
			t.Fatalf("create config: %v", err)
		}
		if _, err := file.WriteString("governance:\n  actor_override_enabled: definitely\n"); err != nil {
			t.Fatalf("write config: %v", err)
		}
		if err := file.Close(); err != nil {
			t.Fatalf("close config: %v", err)
		}

		_, err = LoadFile(file.Name())
		if err == nil || err.Error() != "governance.actor_override_enabled must be a boolean" {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("env", func(t *testing.T) {
		t.Setenv("PROTORADAR_GOVERNANCE_ACTOR_OVERRIDE_ENABLED", "definitely")

		_, err := LoadFile("")
		if err == nil || err.Error() != "governance.actor_override_enabled must be a boolean" {
			t.Fatalf("error = %v", err)
		}
	})
}

func TestOutboxPublisherValidationFailures(t *testing.T) {
	tests := []struct {
		name string
		edit func(*Config)
		want string
	}{
		{
			name: "non-positive batch size",
			edit: func(cfg *Config) {
				cfg.OutboxPublisher.BatchSize = 0
			},
			want: "outbox_publisher.batch_size must be positive",
		},
		{
			name: "non-positive poll interval",
			edit: func(cfg *Config) {
				cfg.OutboxPublisher.PollInterval = 0
			},
			want: "outbox_publisher.poll_interval must be positive",
		},
		{
			name: "non-positive lease duration",
			edit: func(cfg *Config) {
				cfg.OutboxPublisher.LeaseDuration = 0
			},
			want: "outbox_publisher.lease_duration must be positive",
		},
		{
			name: "non-positive max attempts",
			edit: func(cfg *Config) {
				cfg.OutboxPublisher.MaxAttempts = 0
			},
			want: "outbox_publisher.max_attempts must be positive",
		},
		{
			name: "non-positive initial backoff",
			edit: func(cfg *Config) {
				cfg.OutboxPublisher.InitialRetryBackoff = 0
			},
			want: "outbox_publisher.initial_backoff must be positive",
		},
		{
			name: "non-positive max backoff",
			edit: func(cfg *Config) {
				cfg.OutboxPublisher.MaxRetryBackoff = 0
			},
			want: "outbox_publisher.max_backoff must be positive",
		},
		{
			name: "initial backoff above max",
			edit: func(cfg *Config) {
				cfg.OutboxPublisher.InitialRetryBackoff = 10 * time.Second
				cfg.OutboxPublisher.MaxRetryBackoff = 5 * time.Second
			},
			want: "outbox_publisher.initial_backoff must be less than or equal to outbox_publisher.max_backoff",
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

func writeConfigFile(t *testing.T, body string) string {
	t.Helper()

	file, err := os.CreateTemp(t.TempDir(), "config-*.yaml")
	if err != nil {
		t.Fatalf("temp file: %v", err)
	}
	if _, err := file.WriteString(strings.TrimSpace(body)); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close config: %v", err)
	}
	return file.Name()
}
