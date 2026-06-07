package config

import "fmt"

func rejectUnknownYAMLKeys(keys []string) error {
	for _, key := range keys {
		if !supportedYAMLKey(key) {
			return fmt.Errorf("%s is not a supported config field", key)
		}
	}
	return nil
}

func supportedYAMLKey(key string) bool {
	switch key {
	case "server",
		"server.http_addr",
		"server.http_read_header_timeout",
		"server.http_read_timeout",
		"server.http_write_timeout",
		"server.http_idle_timeout",
		"server.http_max_header_bytes",
		"server.max_request_body_bytes",
		"database",
		"database.url",
		"storage",
		"storage.s3",
		"storage.s3.endpoint",
		"storage.s3.region",
		"storage.s3.bucket",
		"storage.s3.access_key",
		"storage.s3.secret_key",
		"storage.s3.use_path_style",
		"auth",
		"auth.token_hash_secret",
		"auth.bootstrap_token",
		"registry",
		"registry.max_artifact_size_bytes",
		"buf",
		"buf.binary_path",
		"buf.build_timeout",
		"buf.lint_timeout",
		"buf.lint_mode",
		"buf.require_config",
		"buf.max_report_bytes",
		"breaking",
		"breaking.max_report_bytes",
		"breaking.max_changes",
		"breaking.default_against",
		"governance",
		"governance.enabled",
		"governance.production_environments",
		"governance.allow_maintainer_approval",
		"governance.actor_override_enabled",
		"outbox_publisher",
		"outbox_publisher.enabled",
		"outbox_publisher.batch_size",
		"outbox_publisher.poll_interval",
		"outbox_publisher.lease_duration",
		"outbox_publisher.max_attempts",
		"outbox_publisher.initial_backoff",
		"outbox_publisher.max_backoff",
		"ui",
		"ui.enabled",
		"ui.base_path",
		"ui.static_path",
		"log",
		"log.level",
		"log.format":
		return true
	default:
		return false
	}
}
