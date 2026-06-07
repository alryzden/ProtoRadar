package bootstrap

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/alryzden/ProtoRadar/internal/authorization"
	"github.com/alryzden/ProtoRadar/internal/config"
	"github.com/alryzden/ProtoRadar/internal/edition"
	"github.com/alryzden/ProtoRadar/internal/identity"
	httptransport "github.com/alryzden/ProtoRadar/internal/transport/http"
	"github.com/alryzden/ProtoRadar/internal/transport/web"
	"github.com/alryzden/ProtoRadar/internal/version"
)

func NewHTTPHandler(registry httptransport.Registry, authProvider identity.AuthProvider, authorizer authorization.Authorizer, capabilityChecker edition.CapabilityChecker, runtimeInventory httptransport.RuntimeInventory, governance httptransport.Governance, uiQuery web.Query, cfg config.RuntimeConfig, ready func(context.Context) error) (http.Handler, error) {
	logger, err := httptransport.NewLogger(cfg.Log.Level, cfg.Log.Format, os.Stdout)
	if err != nil {
		return nil, fmt.Errorf("create logger: %w", err)
	}
	return NewHTTPHandlerWithObservability(registry, authProvider, authorizer, capabilityChecker, runtimeInventory, governance, uiQuery, cfg, ready, logger, httptransport.NewMetrics())
}

func NewHTTPHandlerWithObservability(registry httptransport.Registry, authProvider identity.AuthProvider, authorizer authorization.Authorizer, capabilityChecker edition.CapabilityChecker, runtimeInventory httptransport.RuntimeInventory, governance httptransport.Governance, uiQuery web.Query, cfg config.RuntimeConfig, ready func(context.Context) error, logger *slog.Logger, metrics *httptransport.Metrics) (http.Handler, error) {
	apiHandler := httptransport.NewServer(registry, httptransport.Options{
		BootstrapToken:                 cfg.Auth.BootstrapToken,
		AuthProvider:                   authProvider,
		Authorizer:                     authorizer,
		CapabilityChecker:              capabilityChecker,
		BuildInfo:                      version.Info(),
		Runtime:                        runtimeInventory,
		Governance:                     governance,
		GovernanceActorOverrideEnabled: cfg.Governance.ActorOverrideEnabled,
		Ready:                          ready,
		Logger:                         logger,
		Metrics:                        metrics,
		MaxRequestBodyBytes:            cfg.Server.MaxRequestBodyBytes,
	}).Handler()

	if !cfg.UI.Enabled {
		return apiHandler, nil
	}

	webServer, err := web.NewServer(web.Options{
		BasePath:   cfg.UI.BasePath,
		StaticPath: cfg.UI.StaticPath,
		Query:      uiQuery,
	})
	if err != nil {
		return nil, fmt.Errorf("create web ui: %w", err)
	}

	mux := http.NewServeMux()
	basePath := strings.TrimRight(cfg.UI.BasePath, "/")
	if basePath == "" {
		basePath = "/"
	}
	webHandler := webServer.Handler()
	if basePath == "/" {
		mux.Handle("/", webHandler)
	} else {
		mux.Handle(basePath, webHandler)
		mux.Handle(basePath+"/", webHandler)
		mux.Handle("/", apiHandler)
	}
	return mux, nil
}
