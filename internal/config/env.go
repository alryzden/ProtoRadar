package config

import (
	"os"
)

type envValueKind int

const (
	envString envValueKind = iota
	envBool
	envInt
	envInt64
	envDuration
)

type envBinding struct {
	name string
	path string
	kind envValueKind
}

func envBindings() []envBinding {
	return []envBinding{
		{name: "PROTORADAR_SERVER_HTTP_ADDR", path: "server.http_addr", kind: envString},
		{name: "PROTORADAR_SERVER_HTTP_READ_HEADER_TIMEOUT", path: "server.http_read_header_timeout", kind: envDuration},
		{name: "PROTORADAR_SERVER_HTTP_READ_TIMEOUT", path: "server.http_read_timeout", kind: envDuration},
		{name: "PROTORADAR_SERVER_HTTP_WRITE_TIMEOUT", path: "server.http_write_timeout", kind: envDuration},
		{name: "PROTORADAR_SERVER_HTTP_IDLE_TIMEOUT", path: "server.http_idle_timeout", kind: envDuration},
		{name: "PROTORADAR_SERVER_HTTP_MAX_HEADER_BYTES", path: "server.http_max_header_bytes", kind: envInt},
		{name: "PROTORADAR_SERVER_MAX_REQUEST_BODY_BYTES", path: "server.max_request_body_bytes", kind: envInt64},
		{name: "PROTORADAR_DATABASE_URL", path: "database.url", kind: envString},
		{name: "PROTORADAR_STORAGE_S3_ENDPOINT", path: "storage.s3.endpoint", kind: envString},
		{name: "PROTORADAR_STORAGE_S3_REGION", path: "storage.s3.region", kind: envString},
		{name: "PROTORADAR_STORAGE_S3_BUCKET", path: "storage.s3.bucket", kind: envString},
		{name: "PROTORADAR_STORAGE_S3_ACCESS_KEY", path: "storage.s3.access_key", kind: envString},
		{name: "PROTORADAR_STORAGE_S3_SECRET_KEY", path: "storage.s3.secret_key", kind: envString},
		{name: "PROTORADAR_STORAGE_S3_USE_PATH_STYLE", path: "storage.s3.use_path_style", kind: envBool},
		{name: "PROTORADAR_AUTH_TOKEN_HASH_SECRET", path: "auth.token_hash_secret", kind: envString},
		{name: "PROTORADAR_AUTH_BOOTSTRAP_TOKEN", path: "auth.bootstrap_token", kind: envString},
		{name: "PROTORADAR_REGISTRY_MAX_ARTIFACT_SIZE_BYTES", path: "registry.max_artifact_size_bytes", kind: envInt64},
		{name: "PROTORADAR_BUF_BINARY_PATH", path: "buf.binary_path", kind: envString},
		{name: "PROTORADAR_BUF_BUILD_TIMEOUT", path: "buf.build_timeout", kind: envDuration},
		{name: "PROTORADAR_BUF_LINT_TIMEOUT", path: "buf.lint_timeout", kind: envDuration},
		{name: "PROTORADAR_BUF_LINT_MODE", path: "buf.lint_mode", kind: envString},
		{name: "PROTORADAR_BUF_REQUIRE_CONFIG", path: "buf.require_config", kind: envBool},
		{name: "PROTORADAR_BUF_MAX_REPORT_BYTES", path: "buf.max_report_bytes", kind: envInt},
		{name: "PROTORADAR_BREAKING_MAX_REPORT_BYTES", path: "breaking.max_report_bytes", kind: envInt},
		{name: "PROTORADAR_BREAKING_MAX_CHANGES", path: "breaking.max_changes", kind: envInt},
		{name: "PROTORADAR_BREAKING_DEFAULT_AGAINST", path: "breaking.default_against", kind: envString},
		{name: "PROTORADAR_GOVERNANCE_ENABLED", path: "governance.enabled", kind: envBool},
		{name: "PROTORADAR_GOVERNANCE_PRODUCTION_ENVIRONMENTS", path: "governance.production_environments", kind: envString},
		{name: "PROTORADAR_GOVERNANCE_ALLOW_MAINTAINER_APPROVAL", path: "governance.allow_maintainer_approval", kind: envBool},
		{name: "PROTORADAR_GOVERNANCE_ACTOR_OVERRIDE_ENABLED", path: "governance.actor_override_enabled", kind: envBool},
		{name: "PROTORADAR_OUTBOX_PUBLISHER_ENABLED", path: "outbox_publisher.enabled", kind: envBool},
		{name: "PROTORADAR_OUTBOX_PUBLISHER_BATCH_SIZE", path: "outbox_publisher.batch_size", kind: envInt},
		{name: "PROTORADAR_OUTBOX_PUBLISHER_POLL_INTERVAL", path: "outbox_publisher.poll_interval", kind: envDuration},
		{name: "PROTORADAR_OUTBOX_PUBLISHER_LEASE_DURATION", path: "outbox_publisher.lease_duration", kind: envDuration},
		{name: "PROTORADAR_OUTBOX_PUBLISHER_MAX_ATTEMPTS", path: "outbox_publisher.max_attempts", kind: envInt},
		{name: "PROTORADAR_OUTBOX_PUBLISHER_INITIAL_BACKOFF", path: "outbox_publisher.initial_backoff", kind: envDuration},
		{name: "PROTORADAR_OUTBOX_PUBLISHER_MAX_BACKOFF", path: "outbox_publisher.max_backoff", kind: envDuration},
		{name: "PROTORADAR_UI_ENABLED", path: "ui.enabled", kind: envBool},
		{name: "PROTORADAR_UI_BASE_PATH", path: "ui.base_path", kind: envString},
		{name: "PROTORADAR_UI_STATIC_PATH", path: "ui.static_path", kind: envString},
		{name: "PROTORADAR_LOG_LEVEL", path: "log.level", kind: envString},
		{name: "PROTORADAR_LOG_FORMAT", path: "log.format", kind: envString},
	}
}

func validateConfigEnv() error {
	for _, binding := range envBindings() {
		value, ok := os.LookupEnv(binding.name)
		if !ok {
			continue
		}
		if err := validateEnvValue(binding, value); err != nil {
			return err
		}
	}
	return nil
}

func envPathForName(name string) string {
	for _, binding := range envBindings() {
		if binding.name == name {
			return binding.path
		}
	}
	return ""
}

func validateEnvValue(binding envBinding, value string) error {
	switch binding.kind {
	case envString:
		return nil
	case envBool:
		_, err := parseBool(binding.path, value)
		return err
	case envInt:
		_, err := parseInt(binding.path, value)
		return err
	case envInt64:
		_, err := parseInt64(binding.path, value)
		return err
	case envDuration:
		_, err := parseDuration(binding.path, value)
		return err
	default:
		return nil
	}
}
