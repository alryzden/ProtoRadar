package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const defaultMaxArtifactSizeBytes int64 = 100 * 1024 * 1024
const defaultBufMaxReportBytes = 16 * 1024
const defaultBreakingMaxReportBytes = 32 * 1024
const defaultBreakingMaxChanges = 1000

const (
	BufLintModeDisabled = "disabled"
	BufLintModeWarn     = "warn"
	BufLintModeEnforce  = "enforce"
)

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	Storage  StorageConfig  `yaml:"storage"`
	Auth     AuthConfig     `yaml:"auth"`
	Registry RegistryConfig `yaml:"registry"`
	Buf      BufConfig      `yaml:"buf"`
	Breaking BreakingConfig `yaml:"breaking"`
	UI       UIConfig       `yaml:"ui"`
}

type ServerConfig struct {
	HTTPAddr string `yaml:"http_addr"`
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
	BinaryPath     string `yaml:"binary_path"`
	BuildTimeout   string `yaml:"build_timeout"`
	LintTimeout    string `yaml:"lint_timeout"`
	LintMode       string `yaml:"lint_mode"`
	RequireConfig  bool   `yaml:"require_config"`
	MaxReportBytes int    `yaml:"max_report_bytes"`
}

type BreakingConfig struct {
	MaxReportBytes int    `yaml:"max_report_bytes"`
	MaxChanges     int    `yaml:"max_changes"`
	DefaultAgainst string `yaml:"default_against"`
}

type UIConfig struct {
	Enabled    bool   `yaml:"enabled"`
	BasePath   string `yaml:"base_path"`
	StaticPath string `yaml:"static_path"`
}

func Defaults() Config {
	return Config{
		Server: ServerConfig{
			HTTPAddr: ":8080",
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
			BuildTimeout:   "30s",
			LintTimeout:    "30s",
			LintMode:       BufLintModeWarn,
			RequireConfig:  true,
			MaxReportBytes: defaultBufMaxReportBytes,
		},
		Breaking: BreakingConfig{
			MaxReportBytes: defaultBreakingMaxReportBytes,
			MaxChanges:     defaultBreakingMaxChanges,
			DefaultAgainst: "latest",
		},
		UI: UIConfig{
			Enabled:    true,
			BasePath:   "/ui",
			StaticPath: "/ui/static",
		},
	}
}

func LoadFile(path string) (Config, error) {
	cfg := Defaults()
	if path == "" {
		if err := cfg.ApplyEnv(); err != nil {
			return Config{}, err
		}
		return cfg, nil
	}

	body, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	if err := applyYAML(body, &cfg); err != nil {
		return Config{}, err
	}

	if err := cfg.ApplyEnv(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func applyYAML(body []byte, cfg *Config) error {
	section := ""
	subsection := ""

	for lineNumber, rawLine := range strings.Split(string(body), "\n") {
		line := strings.TrimRight(rawLine, " \t\r")
		if strings.TrimSpace(line) == "" || strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}

		indent := len(line) - len(strings.TrimLeft(line, " "))
		key, value, ok := strings.Cut(strings.TrimSpace(line), ":")
		if !ok {
			return fmt.Errorf("config line %d must use key: value syntax", lineNumber+1)
		}
		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"'`)

		if indent == 0 {
			section = key
			subsection = ""
			continue
		}
		if indent == 2 && value == "" {
			subsection = key
			continue
		}

		path := key
		if subsection != "" {
			path = section + "." + subsection + "." + key
		} else if section != "" {
			path = section + "." + key
		}

		if err := applyYAMLValue(cfg, path, value); err != nil {
			return err
		}
	}

	return nil
}

func applyYAMLValue(cfg *Config, path string, value string) error {
	switch path {
	case "server.http_addr":
		cfg.Server.HTTPAddr = value
	case "database.url":
		cfg.Database.URL = value
	case "storage.s3.endpoint":
		cfg.Storage.S3.Endpoint = value
	case "storage.s3.region":
		cfg.Storage.S3.Region = value
	case "storage.s3.bucket":
		cfg.Storage.S3.Bucket = value
	case "storage.s3.access_key":
		cfg.Storage.S3.AccessKey = value
	case "storage.s3.secret_key":
		cfg.Storage.S3.SecretKey = value
	case "storage.s3.use_path_style":
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("storage.s3.use_path_style must be a boolean")
		}
		cfg.Storage.S3.UsePathStyle = parsed
	case "auth.token_hash_secret":
		cfg.Auth.TokenHashSecret = value
	case "auth.bootstrap_token":
		cfg.Auth.BootstrapToken = value
	case "registry.max_artifact_size_bytes":
		parsed, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return fmt.Errorf("registry.max_artifact_size_bytes must be an integer")
		}
		cfg.Registry.MaxArtifactSizeBytes = parsed
	case "buf.binary_path":
		cfg.Buf.BinaryPath = value
	case "buf.build_timeout":
		cfg.Buf.BuildTimeout = value
	case "buf.lint_timeout":
		cfg.Buf.LintTimeout = value
	case "buf.lint_mode":
		cfg.Buf.LintMode = value
	case "buf.require_config":
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("buf.require_config must be a boolean")
		}
		cfg.Buf.RequireConfig = parsed
	case "buf.max_report_bytes":
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("buf.max_report_bytes must be an integer")
		}
		cfg.Buf.MaxReportBytes = parsed
	case "breaking.max_report_bytes":
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("breaking.max_report_bytes must be an integer")
		}
		cfg.Breaking.MaxReportBytes = parsed
	case "breaking.max_changes":
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("breaking.max_changes must be an integer")
		}
		cfg.Breaking.MaxChanges = parsed
	case "breaking.default_against":
		cfg.Breaking.DefaultAgainst = value
	case "ui.enabled":
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("ui.enabled must be a boolean")
		}
		cfg.UI.Enabled = parsed
	case "ui.base_path":
		cfg.UI.BasePath = value
	case "ui.static_path":
		cfg.UI.StaticPath = value
	default:
		return fmt.Errorf("%s is not a supported config field", path)
	}
	return nil
}

func (c *Config) ApplyEnv() error {
	if value, ok := os.LookupEnv("PROTORADAR_HTTP_ADDR"); ok {
		c.Server.HTTPAddr = value
	}
	if value, ok := os.LookupEnv("PROTORADAR_DATABASE_URL"); ok {
		c.Database.URL = value
	}
	if value, ok := os.LookupEnv("PROTORADAR_STORAGE_S3_ENDPOINT"); ok {
		c.Storage.S3.Endpoint = value
	}
	if value, ok := os.LookupEnv("PROTORADAR_STORAGE_S3_REGION"); ok {
		c.Storage.S3.Region = value
	}
	if value, ok := os.LookupEnv("PROTORADAR_STORAGE_S3_BUCKET"); ok {
		c.Storage.S3.Bucket = value
	}
	if value, ok := os.LookupEnv("PROTORADAR_STORAGE_S3_ACCESS_KEY"); ok {
		c.Storage.S3.AccessKey = value
	}
	if value, ok := os.LookupEnv("PROTORADAR_STORAGE_S3_SECRET_KEY"); ok {
		c.Storage.S3.SecretKey = value
	}
	if value, ok := os.LookupEnv("PROTORADAR_STORAGE_S3_USE_PATH_STYLE"); ok {
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("storage.s3.use_path_style must be a boolean")
		}
		c.Storage.S3.UsePathStyle = parsed
	}
	if value, ok := os.LookupEnv("PROTORADAR_AUTH_TOKEN_HASH_SECRET"); ok {
		c.Auth.TokenHashSecret = value
	}
	if value, ok := os.LookupEnv("PROTORADAR_AUTH_BOOTSTRAP_TOKEN"); ok {
		c.Auth.BootstrapToken = value
	}
	if value, ok := os.LookupEnv("PROTORADAR_REGISTRY_MAX_ARTIFACT_SIZE_BYTES"); ok {
		parsed, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return fmt.Errorf("registry.max_artifact_size_bytes must be an integer")
		}
		c.Registry.MaxArtifactSizeBytes = parsed
	}
	if value, ok := os.LookupEnv("PROTORADAR_BUF_BINARY_PATH"); ok {
		c.Buf.BinaryPath = value
	}
	if value, ok := os.LookupEnv("PROTORADAR_BUF_BUILD_TIMEOUT"); ok {
		c.Buf.BuildTimeout = value
	}
	if value, ok := os.LookupEnv("PROTORADAR_BUF_LINT_TIMEOUT"); ok {
		c.Buf.LintTimeout = value
	}
	if value, ok := os.LookupEnv("PROTORADAR_BUF_LINT_MODE"); ok {
		c.Buf.LintMode = value
	}
	if value, ok := os.LookupEnv("PROTORADAR_BUF_REQUIRE_CONFIG"); ok {
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("buf.require_config must be a boolean")
		}
		c.Buf.RequireConfig = parsed
	}
	if value, ok := os.LookupEnv("PROTORADAR_BUF_MAX_REPORT_BYTES"); ok {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("buf.max_report_bytes must be an integer")
		}
		c.Buf.MaxReportBytes = parsed
	}
	if value, ok := os.LookupEnv("PROTORADAR_BREAKING_MAX_REPORT_BYTES"); ok {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("breaking.max_report_bytes must be an integer")
		}
		c.Breaking.MaxReportBytes = parsed
	}
	if value, ok := os.LookupEnv("PROTORADAR_BREAKING_MAX_CHANGES"); ok {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("breaking.max_changes must be an integer")
		}
		c.Breaking.MaxChanges = parsed
	}
	if value, ok := os.LookupEnv("PROTORADAR_BREAKING_DEFAULT_AGAINST"); ok {
		c.Breaking.DefaultAgainst = value
	}
	if value, ok := os.LookupEnv("PROTORADAR_UI_ENABLED"); ok {
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("ui.enabled must be a boolean")
		}
		c.UI.Enabled = parsed
	}
	if value, ok := os.LookupEnv("PROTORADAR_UI_BASE_PATH"); ok {
		c.UI.BasePath = value
	}
	if value, ok := os.LookupEnv("PROTORADAR_UI_STATIC_PATH"); ok {
		c.UI.StaticPath = value
	}
	return nil
}

func (c Config) Validate() error {
	if err := c.validateServer(); err != nil {
		return err
	}
	if err := c.validateStorage(); err != nil {
		return err
	}
	if err := c.validateAuth(); err != nil {
		return err
	}
	if err := c.validateRegistry(); err != nil {
		return err
	}
	if err := c.validateBuf(); err != nil {
		return err
	}
	if err := c.validateBreaking(); err != nil {
		return err
	}
	if err := c.validateUI(); err != nil {
		return err
	}
	if err := c.validateDatabase(); err != nil {
		return err
	}
	return nil
}

func (c Config) validateServer() error {
	if strings.TrimSpace(c.Server.HTTPAddr) == "" {
		return fmt.Errorf("server.http_addr is required")
	}
	return nil
}

func (c Config) validateDatabase() error {
	if strings.TrimSpace(c.Database.URL) == "" {
		return fmt.Errorf("database.url is required")
	}
	return nil
}

func (c Config) validateStorage() error {
	required := []struct {
		path  string
		value string
	}{
		{path: "storage.s3.endpoint", value: c.Storage.S3.Endpoint},
		{path: "storage.s3.bucket", value: c.Storage.S3.Bucket},
		{path: "storage.s3.access_key", value: c.Storage.S3.AccessKey},
		{path: "storage.s3.secret_key", value: c.Storage.S3.SecretKey},
	}

	for _, field := range required {
		if strings.TrimSpace(field.value) == "" {
			return fmt.Errorf("%s is required", field.path)
		}
	}
	return nil
}

func (c Config) validateAuth() error {
	if strings.TrimSpace(c.Auth.TokenHashSecret) == "" {
		return fmt.Errorf("auth.token_hash_secret is required")
	}
	return nil
}

func (c Config) validateRegistry() error {
	if c.Registry.MaxArtifactSizeBytes <= 0 {
		return fmt.Errorf("registry.max_artifact_size_bytes must be positive")
	}
	return nil
}

func (c Config) validateBuf() error {
	if strings.TrimSpace(c.Buf.BinaryPath) == "" {
		return fmt.Errorf("buf.binary_path is required")
	}
	if _, err := parsePositiveDuration("buf.build_timeout", c.Buf.BuildTimeout); err != nil {
		return err
	}
	if _, err := parsePositiveDuration("buf.lint_timeout", c.Buf.LintTimeout); err != nil {
		return err
	}
	switch c.Buf.LintMode {
	case BufLintModeDisabled, BufLintModeWarn, BufLintModeEnforce:
	default:
		return fmt.Errorf("buf.lint_mode must be one of disabled, warn, enforce")
	}
	if c.Buf.MaxReportBytes <= 0 {
		return fmt.Errorf("buf.max_report_bytes must be positive")
	}
	return nil
}

func (c Config) validateBreaking() error {
	if c.Breaking.MaxReportBytes <= 0 {
		return fmt.Errorf("breaking.max_report_bytes must be positive")
	}
	if c.Breaking.MaxChanges <= 0 {
		return fmt.Errorf("breaking.max_changes must be positive")
	}
	if strings.TrimSpace(c.Breaking.DefaultAgainst) == "" {
		return fmt.Errorf("breaking.default_against is required")
	}
	return nil
}

func (c Config) validateUI() error {
	basePath := strings.TrimSpace(c.UI.BasePath)
	staticPath := strings.TrimSpace(c.UI.StaticPath)
	if !strings.HasPrefix(basePath, "/") {
		return fmt.Errorf("ui.base_path must start with /")
	}
	if !strings.HasPrefix(staticPath, "/") {
		return fmt.Errorf("ui.static_path must start with /")
	}
	if staticPath == basePath {
		return fmt.Errorf("ui.static_path must be under ui.base_path")
	}
	basePrefix := strings.TrimRight(basePath, "/")
	if basePrefix != "" && !strings.HasPrefix(staticPath, basePrefix+"/") {
		return fmt.Errorf("ui.static_path must be under ui.base_path")
	}
	return nil
}

func (c Config) Runtime() (RuntimeConfig, error) {
	if err := c.Validate(); err != nil {
		return RuntimeConfig{}, err
	}

	buildTimeout, err := parsePositiveDuration("buf.build_timeout", c.Buf.BuildTimeout)
	if err != nil {
		return RuntimeConfig{}, err
	}
	lintTimeout, err := parsePositiveDuration("buf.lint_timeout", c.Buf.LintTimeout)
	if err != nil {
		return RuntimeConfig{}, err
	}

	return RuntimeConfig{
		Server: RuntimeServerConfig{
			HTTPAddr: c.Server.HTTPAddr,
		},
		Database: RuntimeDatabaseConfig{
			URL: c.Database.URL,
		},
		Storage: RuntimeStorageConfig{
			S3: RuntimeS3Config{
				Endpoint:     c.Storage.S3.Endpoint,
				Region:       c.Storage.S3.Region,
				Bucket:       c.Storage.S3.Bucket,
				AccessKey:    c.Storage.S3.AccessKey,
				SecretKey:    c.Storage.S3.SecretKey,
				UsePathStyle: c.Storage.S3.UsePathStyle,
			},
		},
		Auth: RuntimeAuthConfig{
			TokenHashSecret: c.Auth.TokenHashSecret,
			BootstrapToken:  c.Auth.BootstrapToken,
		},
		Registry: RuntimeRegistryConfig{
			MaxArtifactSizeBytes: c.Registry.MaxArtifactSizeBytes,
		},
		Buf: RuntimeBufConfig{
			BinaryPath:     c.Buf.BinaryPath,
			BuildTimeout:   buildTimeout,
			LintTimeout:    lintTimeout,
			LintMode:       c.Buf.LintMode,
			RequireConfig:  c.Buf.RequireConfig,
			MaxReportBytes: c.Buf.MaxReportBytes,
		},
		Breaking: RuntimeBreakingConfig{
			MaxReportBytes: c.Breaking.MaxReportBytes,
			MaxChanges:     c.Breaking.MaxChanges,
			DefaultAgainst: strings.TrimSpace(c.Breaking.DefaultAgainst),
		},
		UI: RuntimeUIConfig{
			Enabled:    c.UI.Enabled,
			BasePath:   normalizePath(c.UI.BasePath),
			StaticPath: normalizePath(c.UI.StaticPath),
		},
	}, nil
}

func normalizePath(value string) string {
	path := strings.TrimRight(strings.TrimSpace(value), "/")
	if path == "" {
		return "/"
	}
	return path
}

func parsePositiveDuration(path string, value string) (time.Duration, error) {
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be a valid duration", path)
	}
	if parsed <= 0 {
		return 0, fmt.Errorf("%s must be positive", path)
	}
	return parsed, nil
}
