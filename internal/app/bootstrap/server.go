package bootstrap

import (
	"context"
	"io/fs"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/alryzden/ProtoRadar/internal/config"
	"github.com/alryzden/ProtoRadar/internal/infrastructure/bufcli"
	"github.com/alryzden/ProtoRadar/internal/repository/postgres"
	"github.com/alryzden/ProtoRadar/internal/usecase/dependencygraph"
	"github.com/alryzden/ProtoRadar/internal/usecase/registry"
	"github.com/alryzden/ProtoRadar/internal/usecase/runtimeinventory"
	"github.com/alryzden/ProtoRadar/internal/usecase/uiquery"
)

type Server struct {
	HTTPServer *http.Server
	DBPool     *pgxpool.Pool
}

func NewServer(ctx context.Context, cfg config.RuntimeConfig, migrations fs.FS) (*Server, error) {
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

	db := postgres.New(pool)
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
	outboxWriter := postgres.NewOutboxWriter(db)
	clock := registry.SystemClock{}
	ids := registry.RandomIDGenerator{}

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
	)

	handler, err := NewHTTPHandler(registryService, runtimeInventoryService, uiQueryService, cfg, pool.Ping)
	if err != nil {
		pool.Close()
		return nil, err
	}
	return &Server{
		HTTPServer: &http.Server{
			Addr:    cfg.Server.HTTPAddr,
			Handler: handler,
		},
		DBPool: pool,
	}, nil
}

func (server *Server) Close() {
	if server.DBPool != nil {
		server.DBPool.Close()
	}
}
