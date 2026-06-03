package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

const defaultMaxArtifactSizeBytes int64 = 100 * 1024 * 1024

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	Storage  StorageConfig  `yaml:"storage"`
	Auth     AuthConfig     `yaml:"auth"`
	Registry RegistryConfig `yaml:"registry"`
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

func (c Config) Runtime() (RuntimeConfig, error) {
	if err := c.Validate(); err != nil {
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
	}, nil
}
