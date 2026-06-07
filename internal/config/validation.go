package config

import (
	"fmt"
	"strings"
)

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
	if err := c.validateGovernance(); err != nil {
		return err
	}
	if err := c.validateOutboxPublisher(); err != nil {
		return err
	}
	if err := c.validateUI(); err != nil {
		return err
	}
	if err := c.validateLog(); err != nil {
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
	if _, err := parsePositiveDuration("server.http_read_header_timeout", c.Server.HTTPReadHeaderTimeout); err != nil {
		return err
	}
	if _, err := parsePositiveDuration("server.http_read_timeout", c.Server.HTTPReadTimeout); err != nil {
		return err
	}
	if _, err := parsePositiveDuration("server.http_write_timeout", c.Server.HTTPWriteTimeout); err != nil {
		return err
	}
	if _, err := parsePositiveDuration("server.http_idle_timeout", c.Server.HTTPIdleTimeout); err != nil {
		return err
	}
	if c.Server.HTTPMaxHeaderBytes <= 0 {
		return fmt.Errorf("server.http_max_header_bytes must be positive")
	}
	if c.Server.MaxRequestBodyBytes <= 0 {
		return fmt.Errorf("server.max_request_body_bytes must be positive")
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

func (c Config) validateGovernance() error {
	if !c.Governance.Enabled {
		return nil
	}
	if _, err := parseStringList("governance.production_environments", c.Governance.ProductionEnvironments); err != nil {
		return err
	}
	return nil
}

func (c Config) validateOutboxPublisher() error {
	if c.OutboxPublisher.BatchSize <= 0 {
		return fmt.Errorf("outbox_publisher.batch_size must be positive")
	}
	if _, err := parsePositiveDuration("outbox_publisher.poll_interval", c.OutboxPublisher.PollInterval); err != nil {
		return err
	}
	if _, err := parsePositiveDuration("outbox_publisher.lease_duration", c.OutboxPublisher.LeaseDuration); err != nil {
		return err
	}
	if c.OutboxPublisher.MaxAttempts <= 0 {
		return fmt.Errorf("outbox_publisher.max_attempts must be positive")
	}
	initialBackoff, err := parsePositiveDuration("outbox_publisher.initial_backoff", c.OutboxPublisher.InitialRetryBackoff)
	if err != nil {
		return err
	}
	maxBackoff, err := parsePositiveDuration("outbox_publisher.max_backoff", c.OutboxPublisher.MaxRetryBackoff)
	if err != nil {
		return err
	}
	if initialBackoff > maxBackoff {
		return fmt.Errorf("outbox_publisher.initial_backoff must be less than or equal to outbox_publisher.max_backoff")
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

func (c Config) validateLog() error {
	switch strings.TrimSpace(c.Log.Level) {
	case LogLevelDebug, LogLevelInfo, LogLevelWarn, LogLevelError:
	default:
		return fmt.Errorf("log.level must be one of debug, info, warn, error")
	}
	switch strings.TrimSpace(c.Log.Format) {
	case LogFormatJSON, LogFormatText:
	default:
		return fmt.Errorf("log.format must be one of json, text")
	}
	return nil
}
