package config

import (
	"os"
	"strings"
	"testing"
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
	if err := cfg.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
}
