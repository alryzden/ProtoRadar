package bootstrap

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/alryzden/ProtoRadar/internal/audit"
	"github.com/alryzden/ProtoRadar/internal/authorization"
	"github.com/alryzden/ProtoRadar/internal/config"
	"github.com/alryzden/ProtoRadar/internal/edition"
	"github.com/alryzden/ProtoRadar/internal/identity"
	"github.com/alryzden/ProtoRadar/internal/infrastructure/bufcli"
	"github.com/alryzden/ProtoRadar/internal/outbox"
	"github.com/alryzden/ProtoRadar/internal/repository/postgres"
	"github.com/alryzden/ProtoRadar/internal/transport/http"
	"github.com/alryzden/ProtoRadar/internal/usecase/dependencygraph"
	"github.com/alryzden/ProtoRadar/internal/usecase/governance"
	"github.com/alryzden/ProtoRadar/internal/usecase/registry"
	"github.com/alryzden/ProtoRadar/internal/usecase/runtimeinventory"
	"github.com/alryzden/ProtoRadar/internal/usecase/uiquery"
	"github.com/alryzden/ProtoRadar/internal/version"
)

type Server struct {
	HTTPServer   *http.Server
	DBPool       *pgxpool.Pool
	outboxCancel context.CancelFunc
	outboxDone   chan struct{}
}

const outboxShutdownTimeout = 5 * time.Second

func openDatabase(ctx context.Context, cfg config.RuntimeConfig, migrations fs.FS) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, cfg.Database.URL)
	if err != nil {
		return nil, err
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	if migrations != nil {
		if err := postgres.RunMigrations(ctx, pool, migrations); err != nil {
			pool.Close()
			return nil, err
		}
	}
	return pool, nil
}

func NewServer(ctx context.Context, cfg config.RuntimeConfig, migrations fs.FS) (*Server, error) {
	// Config/runtime prerequisites are already validated and parsed by internal/config.
	// Bootstrap only adapts typed runtime config into concrete implementations.

	// Database and migrations.
	pool, err := openDatabase(ctx, cfg, migrations)
	if err != nil {
		return nil, err
	}
	db := postgres.New(pool)

	// Observability.
	logger, err := httptransport.NewLogger(cfg.Log.Level, cfg.Log.Format, os.Stdout)
	if err != nil {
		pool.Close()
		return nil, err
	}
	metrics := httptransport.NewMetrics()

	// Storage and external adapters.
	artifactStore, err := NewArtifactStore(cfg)
	if err != nil {
		pool.Close()
		return nil, err
	}
	bufWorkflow, err := bufcli.NewWorkflow(bufcli.Config{
		BinaryPath:     cfg.Buf.BinaryPath,
		BuildTimeout:   cfg.Buf.BuildTimeout,
		LintTimeout:    cfg.Buf.LintTimeout,
		LintMode:       cfg.Buf.LintMode,
		RequireConfig:  cfg.Buf.RequireConfig,
		MaxReportBytes: cfg.Buf.MaxReportBytes,
	})
	if err != nil {
		pool.Close()
		return nil, err
	}
	bufBreaking, err := bufcli.NewWorkflow(bufcli.Config{
		BinaryPath:     cfg.Buf.BinaryPath,
		BuildTimeout:   cfg.Buf.BuildTimeout,
		LintTimeout:    cfg.Buf.LintTimeout,
		LintMode:       cfg.Buf.LintMode,
		RequireConfig:  cfg.Buf.RequireConfig,
		MaxReportBytes: cfg.Breaking.MaxReportBytes,
	})
	if err != nil {
		pool.Close()
		return nil, err
	}

	// Repositories and durable outbox writer.
	moduleRepo := postgres.NewModuleRepository(db)
	gitLabProjectRepo := postgres.NewModuleGitLabProjectRepository(db)
	versionRepo := postgres.NewModuleVersionRepository(db)
	artifactRepo := postgres.NewArtifactRepository(db)
	bufConfigRepo := postgres.NewBufConfigRepository(db)
	metadataRepo := postgres.NewDescriptorMetadataRepository(db)
	breakingReportRepo := postgres.NewBreakingReportRepository(db)
	dependencyRepo := postgres.NewModuleDependencyRepository(db)
	runtimeInventoryRepo := postgres.NewRuntimeInventoryRepository(db)
	tokenRepo := postgres.NewAPITokenRepository(db)
	moduleOwnerRepo := postgres.NewModuleOwnerRepository(db)
	approvalRepo := postgres.NewApprovalRepository(db)
	governanceAuditRepo := postgres.NewGovernanceAuditRepository(db)
	auditSink := audit.NewCommunityAuditSink(governanceAuditRepo)
	outboxWriter := postgres.NewOutboxWriter(db)

	// Identity, authorization, capabilities, and shared generators.
	clock := registry.SystemClock{}
	authProvider := identity.NewAPITokenAuthProviderWithObserver(tokenRepo, cfg.Auth.TokenHashSecret, clock, identityTokenUsageLogger{logger: logger})
	authorizer := authorization.CommunityAuthorizer{}
	capabilityChecker := edition.NewCommunityCapabilityChecker()
	ids := registry.RandomIDGenerator{}
	governanceClock := governance.SystemClock{}
	governanceIDs := governance.RandomIDGenerator{}

	// Usecases and query services. Usecases receive repository ports and the
	// transactional outbox writer; they do not publish directly.
	dependencyGraphService := dependencygraph.NewService(
		moduleRepo,
		versionRepo,
		metadataRepo,
		dependencyRepo,
		dependencyRepo,
		db,
		outboxWriter,
		clock,
		ids,
	)

	registryService := registry.NewService(
		moduleRepo,
		gitLabProjectRepo,
		versionRepo,
		artifactRepo,
		bufConfigRepo,
		metadataRepo,
		breakingReportRepo,
		tokenRepo,
		dependencyGraphService,
		dependencyRepo,
		db,
		outboxWriter,
		artifactStore,
		bufWorkflow,
		bufBreaking,
		clock,
		ids,
		registry.RandomTokenGenerator{},
		registry.Options{
			MaxArtifactSizeBytes:           cfg.Registry.MaxArtifactSizeBytes,
			MaxSourceUncompressedSizeBytes: cfg.Registry.MaxArtifactSizeBytes,
			TokenHashSecret:                cfg.Auth.TokenHashSecret,
			BufRequireConfig:               cfg.Buf.RequireConfig,
			BufLintMode:                    cfg.Buf.LintMode,
			BreakingMaxChanges:             cfg.Breaking.MaxChanges,
			BreakingDefaultAgainst:         cfg.Breaking.DefaultAgainst,
			ArtifactCleanupObserver:        registryArtifactCleanupLogger{logger: logger},
			TokenUsageObserver:             registryTokenUsageLogger{logger: logger},
		},
	)
	runtimeInventoryService := runtimeinventory.NewService(
		moduleRepo,
		versionRepo,
		runtimeInventoryRepo,
		breakingReportRepo,
		db,
		outboxWriter,
		clock,
		ids,
	)
	moduleOwnerService := governance.NewService(
		moduleRepo,
		moduleOwnerRepo,
		auditSink,
		db,
		outboxWriter,
		governanceClock,
		governanceIDs,
	)
	approvalService := governance.NewApprovalService(
		moduleRepo,
		breakingReportRepo,
		moduleOwnerRepo,
		approvalRepo,
		auditSink,
		governanceAuditRepo,
		dependencyRepo,
		runtimeInventoryRepo,
		db,
		outboxWriter,
		governance.DefaultPolicyEvaluator{},
		governance.PolicyConfig{
			Enabled:                 cfg.Governance.Enabled,
			ProductionEnvironments:  cfg.Governance.ProductionEnvironments,
			AllowMaintainerApproval: cfg.Governance.AllowMaintainerApproval,
		},
		governanceClock,
		governanceIDs,
	)
	approvalWorkflow := governance.NewDefaultApprovalWorkflow(approvalService)
	governanceService := governance.APIService{
		Owners:        moduleOwnerService,
		Approvals:     approvalWorkflow,
		ApprovalAudit: approvalWorkflow,
	}
	uiQueryService := uiquery.NewService(
		moduleRepo,
		gitLabProjectRepo,
		versionRepo,
		artifactRepo,
		bufConfigRepo,
		metadataRepo,
		breakingReportRepo,
		dependencyRepo,
		runtimeInventoryRepo,
		uiquery.GovernanceRepositories{
			Owners:    moduleOwnerRepo,
			Approvals: approvalRepo,
			Audit:     governanceAuditRepo,
		},
		capabilityChecker,
		version.Info(),
	)

	// Transports.
	handler, err := NewHTTPHandlerWithObservability(registryService, authProvider, authorizer, capabilityChecker, runtimeInventoryService, governanceService, uiQueryService, cfg, pool.Ping, logger, metrics)
	if err != nil {
		pool.Close()
		return nil, err
	}
	server := &Server{
		HTTPServer: newHTTPServer(cfg, handler),
		DBPool:     pool,
	}

	// Outbox lifecycle. The server owns the publisher goroutine and cancels it
	// before closing the database pool in Close.
	if cfg.OutboxPublisher.Enabled {
		cancel, done, err := startOutboxPublisher(ctx, cfg, outboxWriter, logger, metrics)
		if err != nil {
			pool.Close()
			return nil, err
		}
		server.outboxCancel = cancel
		server.outboxDone = done
	}
	return server, nil
}

func newHTTPServer(cfg config.RuntimeConfig, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              cfg.Server.HTTPAddr,
		Handler:           handler,
		ReadHeaderTimeout: cfg.Server.HTTPReadHeaderTimeout,
		ReadTimeout:       cfg.Server.HTTPReadTimeout,
		WriteTimeout:      cfg.Server.HTTPWriteTimeout,
		IdleTimeout:       cfg.Server.HTTPIdleTimeout,
		MaxHeaderBytes:    cfg.Server.HTTPMaxHeaderBytes,
	}
}

type registryArtifactCleanupLogger struct {
	logger *slog.Logger
}

func (observer registryArtifactCleanupLogger) RecordArtifactCleanupFailure(ctx context.Context, failure registry.ArtifactCleanupFailure) {
	if observer.logger == nil || failure.Error == nil {
		return
	}
	observer.logger.WarnContext(ctx, "artifact_cleanup_delete_failed",
		slog.String("storage_key", failure.StorageKey),
		slog.String("error", failure.Error.Error()),
	)
}

type identityTokenUsageLogger struct {
	logger *slog.Logger
}

func (observer identityTokenUsageLogger) RecordTokenUsageFailure(ctx context.Context, failure identity.TokenUsageFailure) {
	if observer.logger == nil || failure.Error == nil {
		return
	}
	observer.logger.WarnContext(ctx, "api_token_mark_used_failed",
		slog.String("api_token_id", failure.TokenID),
		slog.String("error", failure.Error.Error()),
	)
}

type registryTokenUsageLogger struct {
	logger *slog.Logger
}

func (observer registryTokenUsageLogger) RecordTokenUsageFailure(ctx context.Context, failure registry.TokenUsageFailure) {
	if observer.logger == nil || failure.Error == nil {
		return
	}
	observer.logger.WarnContext(ctx, "api_token_mark_used_failed",
		slog.String("api_token_id", failure.TokenID),
		slog.String("error", failure.Error.Error()),
	)
}

func startOutboxPublisher(ctx context.Context, cfg config.RuntimeConfig, repository outbox.Repository, logger *slog.Logger, metrics *httptransport.Metrics) (context.CancelFunc, chan struct{}, error) {
	dispatcher, err := outbox.NewLoggingDispatcher(logger)
	if err != nil {
		return nil, nil, err
	}
	publisher, err := outbox.NewPublisher(repository, dispatcher, outbox.PublisherOptions{
		Enabled:             cfg.OutboxPublisher.Enabled,
		BatchSize:           cfg.OutboxPublisher.BatchSize,
		PollInterval:        cfg.OutboxPublisher.PollInterval,
		LeaseDuration:       cfg.OutboxPublisher.LeaseDuration,
		MaxAttempts:         cfg.OutboxPublisher.MaxAttempts,
		InitialRetryBackoff: cfg.OutboxPublisher.InitialRetryBackoff,
		MaxRetryBackoff:     cfg.OutboxPublisher.MaxRetryBackoff,
		Logger:              logger,
		Observer:            metrics,
	})
	if err != nil {
		return nil, nil, err
	}
	publisherCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := publisher.Run(publisherCtx); err != nil && logger != nil {
			logger.ErrorContext(publisherCtx, "outbox_publisher_stopped_with_error", slog.String("error", err.Error()))
		}
	}()
	return cancel, done, nil
}

func (server *Server) Close() error {
	// Cleanup ownership: stop the bootstrap-owned publisher before closing the
	// database pool it depends on. Preserve the bounded wait from hardening.
	if server.outboxCancel != nil {
		server.outboxCancel()
	}
	if server.outboxDone != nil {
		if err := waitOutboxShutdown(server.outboxDone, outboxShutdownTimeout); err != nil {
			return err
		}
	}
	if server.DBPool != nil {
		server.DBPool.Close()
	}
	return nil
}

func waitOutboxShutdown(done <-chan struct{}, timeout time.Duration) error {
	if done == nil {
		return nil
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-done:
		return nil
	case <-timer.C:
		return fmt.Errorf("outbox publisher shutdown timed out after %s", timeout)
	}
}
