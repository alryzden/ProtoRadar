package config

type RuntimeConfig struct {
	Server   RuntimeServerConfig
	Database RuntimeDatabaseConfig
	Storage  RuntimeStorageConfig
	Auth     RuntimeAuthConfig
	Registry RuntimeRegistryConfig
}

type RuntimeServerConfig struct {
	HTTPAddr string
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
