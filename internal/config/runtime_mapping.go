package config

import "strings"

func (c Config) Runtime() (RuntimeConfig, error) {
	if err := c.Validate(); err != nil {
		return RuntimeConfig{}, err
	}

	productionEnvironments := []string(nil)
	var err error
	if c.Governance.Enabled || strings.TrimSpace(c.Governance.ProductionEnvironments) != "" {
		productionEnvironments, err = parseStringList("governance.production_environments", c.Governance.ProductionEnvironments)
		if err != nil {
			return RuntimeConfig{}, err
		}
	}

	return RuntimeConfig{
		Server: RuntimeServerConfig{
			HTTPAddr:              c.Server.HTTPAddr,
			HTTPReadHeaderTimeout: c.Server.HTTPReadHeaderTimeout,
			HTTPReadTimeout:       c.Server.HTTPReadTimeout,
			HTTPWriteTimeout:      c.Server.HTTPWriteTimeout,
			HTTPIdleTimeout:       c.Server.HTTPIdleTimeout,
			HTTPMaxHeaderBytes:    c.Server.HTTPMaxHeaderBytes,
			MaxRequestBodyBytes:   c.Server.MaxRequestBodyBytes,
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
			BuildTimeout:   c.Buf.BuildTimeout,
			LintTimeout:    c.Buf.LintTimeout,
			LintMode:       c.Buf.LintMode,
			RequireConfig:  c.Buf.RequireConfig,
			MaxReportBytes: c.Buf.MaxReportBytes,
		},
		Breaking: RuntimeBreakingConfig{
			MaxReportBytes: c.Breaking.MaxReportBytes,
			MaxChanges:     c.Breaking.MaxChanges,
			DefaultAgainst: strings.TrimSpace(c.Breaking.DefaultAgainst),
		},
		Governance: RuntimeGovernanceConfig{
			Enabled:                 c.Governance.Enabled,
			ProductionEnvironments:  productionEnvironments,
			AllowMaintainerApproval: c.Governance.AllowMaintainerApproval,
			ActorOverrideEnabled:    c.Governance.ActorOverrideEnabled,
		},
		OutboxPublisher: RuntimeOutboxPublisherConfig{
			Enabled:             c.OutboxPublisher.Enabled,
			BatchSize:           c.OutboxPublisher.BatchSize,
			PollInterval:        c.OutboxPublisher.PollInterval,
			LeaseDuration:       c.OutboxPublisher.LeaseDuration,
			MaxAttempts:         c.OutboxPublisher.MaxAttempts,
			InitialRetryBackoff: c.OutboxPublisher.InitialRetryBackoff,
			MaxRetryBackoff:     c.OutboxPublisher.MaxRetryBackoff,
		},
		UI: RuntimeUIConfig{
			Enabled:    c.UI.Enabled,
			BasePath:   normalizePath(c.UI.BasePath),
			StaticPath: normalizePath(c.UI.StaticPath),
		},
		Log: RuntimeLogConfig{
			Level:  strings.TrimSpace(c.Log.Level),
			Format: strings.TrimSpace(c.Log.Format),
		},
	}, nil
}
