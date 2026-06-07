package config

import "time"

const defaultMaxArtifactSizeBytes int64 = 100 * 1024 * 1024
const defaultMaxRequestBodyBytes int64 = 110 * 1024 * 1024
const defaultBufMaxReportBytes = 16 * 1024
const defaultBreakingMaxReportBytes = 32 * 1024
const defaultBreakingMaxChanges = 1000
const defaultOutboxPublisherBatchSize = 50
const defaultOutboxPublisherMaxAttempts = 5
const defaultHTTPMaxHeaderBytes = 1 << 20

const (
	BufLintModeDisabled = "disabled"
	BufLintModeWarn     = "warn"
	BufLintModeEnforce  = "enforce"
)

const (
	LogLevelDebug = "debug"
	LogLevelInfo  = "info"
	LogLevelWarn  = "warn"
	LogLevelError = "error"
)

const (
	LogFormatJSON = "json"
	LogFormatText = "text"
)

type Config struct {
	Server          ServerConfig          `yaml:"server"`
	Database        DatabaseConfig        `yaml:"database"`
	Storage         StorageConfig         `yaml:"storage"`
	Auth            AuthConfig            `yaml:"auth"`
	Registry        RegistryConfig        `yaml:"registry"`
	Buf             BufConfig             `yaml:"buf"`
	Breaking        BreakingConfig        `yaml:"breaking"`
	Governance      GovernanceConfig      `yaml:"governance"`
	OutboxPublisher OutboxPublisherConfig `yaml:"outbox_publisher"`
	UI              UIConfig              `yaml:"ui"`
	Log             LogConfig             `yaml:"log"`
}

type ServerConfig struct {
	HTTPAddr              string        `yaml:"http_addr"`
	HTTPReadHeaderTimeout time.Duration `yaml:"http_read_header_timeout"`
	HTTPReadTimeout       time.Duration `yaml:"http_read_timeout"`
	HTTPWriteTimeout      time.Duration `yaml:"http_write_timeout"`
	HTTPIdleTimeout       time.Duration `yaml:"http_idle_timeout"`
	HTTPMaxHeaderBytes    int           `yaml:"http_max_header_bytes"`
	MaxRequestBodyBytes   int64         `yaml:"max_request_body_bytes"`
}

type DatabaseConfig struct {
	URL string `yaml:"url"`
}

type StorageConfig struct {
	S3 S3Config `yaml:"s3"`
}

type S3Config struct {
	Endpoint     string `yaml:"endpoint"`
	Region       string `yaml:"region"`
	Bucket       string `yaml:"bucket"`
	AccessKey    string `yaml:"access_key"`
	SecretKey    string `yaml:"secret_key"`
	UsePathStyle bool   `yaml:"use_path_style"`
}

type AuthConfig struct {
	TokenHashSecret string `yaml:"token_hash_secret"`
	BootstrapToken  string `yaml:"bootstrap_token"`
}

type RegistryConfig struct {
	MaxArtifactSizeBytes int64 `yaml:"max_artifact_size_bytes"`
}

type BufConfig struct {
	BinaryPath     string        `yaml:"binary_path"`
	BuildTimeout   time.Duration `yaml:"build_timeout"`
	LintTimeout    time.Duration `yaml:"lint_timeout"`
	LintMode       string        `yaml:"lint_mode"`
	RequireConfig  bool          `yaml:"require_config"`
	MaxReportBytes int           `yaml:"max_report_bytes"`
}

type BreakingConfig struct {
	MaxReportBytes int    `yaml:"max_report_bytes"`
	MaxChanges     int    `yaml:"max_changes"`
	DefaultAgainst string `yaml:"default_against"`
}

type GovernanceConfig struct {
	Enabled                 bool   `yaml:"enabled"`
	ProductionEnvironments  string `yaml:"production_environments"`
	AllowMaintainerApproval bool   `yaml:"allow_maintainer_approval"`
	ActorOverrideEnabled    bool   `yaml:"actor_override_enabled"`
}

type OutboxPublisherConfig struct {
	Enabled             bool          `yaml:"enabled"`
	BatchSize           int           `yaml:"batch_size"`
	PollInterval        time.Duration `yaml:"poll_interval"`
	LeaseDuration       time.Duration `yaml:"lease_duration"`
	MaxAttempts         int           `yaml:"max_attempts"`
	InitialRetryBackoff time.Duration `yaml:"initial_backoff"`
	MaxRetryBackoff     time.Duration `yaml:"max_backoff"`
}

type UIConfig struct {
	Enabled    bool   `yaml:"enabled"`
	BasePath   string `yaml:"base_path"`
	StaticPath string `yaml:"static_path"`
}

type LogConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

func Defaults() Config {
	return Config{
		Server: ServerConfig{
			HTTPAddr:              ":8080",
			HTTPReadHeaderTimeout: 5 * time.Second,
			HTTPReadTimeout:       30 * time.Second,
			HTTPWriteTimeout:      60 * time.Second,
			HTTPIdleTimeout:       120 * time.Second,
			HTTPMaxHeaderBytes:    defaultHTTPMaxHeaderBytes,
			MaxRequestBodyBytes:   defaultMaxRequestBodyBytes,
		},
		Storage: StorageConfig{
			S3: S3Config{
				Region:       "us-east-1",
				UsePathStyle: true,
			},
		},
		Registry: RegistryConfig{
			MaxArtifactSizeBytes: defaultMaxArtifactSizeBytes,
		},
		Buf: BufConfig{
			BinaryPath:     "buf",
			BuildTimeout:   30 * time.Second,
			LintTimeout:    30 * time.Second,
			LintMode:       BufLintModeWarn,
			RequireConfig:  true,
			MaxReportBytes: defaultBufMaxReportBytes,
		},
		Breaking: BreakingConfig{
			MaxReportBytes: defaultBreakingMaxReportBytes,
			MaxChanges:     defaultBreakingMaxChanges,
			DefaultAgainst: "latest",
		},
		Governance: GovernanceConfig{
			Enabled:                 true,
			ProductionEnvironments:  "production,prod",
			AllowMaintainerApproval: true,
			ActorOverrideEnabled:    false,
		},
		OutboxPublisher: OutboxPublisherConfig{
			Enabled:             false,
			BatchSize:           defaultOutboxPublisherBatchSize,
			PollInterval:        5 * time.Second,
			LeaseDuration:       30 * time.Second,
			MaxAttempts:         defaultOutboxPublisherMaxAttempts,
			InitialRetryBackoff: 30 * time.Second,
			MaxRetryBackoff:     5 * time.Minute,
		},
		UI: UIConfig{
			Enabled:    true,
			BasePath:   "/ui",
			StaticPath: "/ui/static",
		},
		Log: LogConfig{
			Level:  LogLevelInfo,
			Format: LogFormatJSON,
		},
	}
}
