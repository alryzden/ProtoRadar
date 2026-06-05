package config

import "time"

type RuntimeConfig struct {
	Server   RuntimeServerConfig
	Database RuntimeDatabaseConfig
	Storage  RuntimeStorageConfig
	Auth     RuntimeAuthConfig
	Registry RuntimeRegistryConfig
	Buf      RuntimeBufConfig
	Breaking RuntimeBreakingConfig
	UI       RuntimeUIConfig
	Log      RuntimeLogConfig
}

type RuntimeServerConfig struct {
	HTTPAddr            string
	MaxRequestBodyBytes int64
}

type RuntimeDatabaseConfig struct {
	URL string
}

type RuntimeStorageConfig struct {
	S3 RuntimeS3Config
}

type RuntimeS3Config struct {
	Endpoint     string
	Region       string
	Bucket       string
	AccessKey    string
	SecretKey    string
	UsePathStyle bool
}

type RuntimeAuthConfig struct {
	TokenHashSecret string
	BootstrapToken  string
}

type RuntimeRegistryConfig struct {
	MaxArtifactSizeBytes int64
}

type RuntimeBufConfig struct {
	BinaryPath     string
	BuildTimeout   time.Duration
	LintTimeout    time.Duration
	LintMode       string
	RequireConfig  bool
	MaxReportBytes int
}

type RuntimeBreakingConfig struct {
	MaxReportBytes int
	MaxChanges     int
	DefaultAgainst string
}

type RuntimeUIConfig struct {
	Enabled    bool
	BasePath   string
	StaticPath string
}

type RuntimeLogConfig struct {
	Level  string
	Format string
}
