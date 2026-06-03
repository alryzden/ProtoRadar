package bootstrap

import (
	"context"
	"io/fs"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/alryzden/ProtoRadar/internal/config"
	"github.com/alryzden/ProtoRadar/internal/infrastructure/bufcli"
	"github.com/alryzden/ProtoRadar/internal/repository/postgres"
	"github.com/alryzden/ProtoRadar/internal/usecase/registry"
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

	registryService := registry.NewService(
		postgres.NewModuleRepository(db),
		postgres.NewModuleVersionRepository(db),
		postgres.NewArtifactRepository(db),
		postgres.NewBufConfigRepository(db),
		postgres.NewDescriptorMetadataRepository(db),
		postgres.NewAPITokenRepository(db),
		db,
		postgres.NewOutboxWriter(db),
		artifactStore,
		bufWorkflow,
		registry.SystemClock{},
		registry.RandomIDGenerator{},
		registry.RandomTokenGenerator{},
		registry.Options{
			MaxArtifactSizeBytes:           cfg.Registry.MaxArtifactSizeBytes,
			MaxSourceUncompressedSizeBytes: cfg.Registry.MaxArtifactSizeBytes,
			TokenHashSecret:                cfg.Auth.TokenHashSecret,
			BufRequireConfig:               cfg.Buf.RequireConfig,
			BufLintMode:                    cfg.Buf.LintMode,
		},
	)

	handler := NewHTTPHandler(registryService, cfg, pool.Ping)
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
