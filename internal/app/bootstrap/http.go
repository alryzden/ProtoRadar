package bootstrap

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/alryzden/ProtoRadar/internal/config"
	httptransport "github.com/alryzden/ProtoRadar/internal/transport/http"
	"github.com/alryzden/ProtoRadar/internal/transport/web"
)

func NewHTTPHandler(registry httptransport.Registry, runtimeInventory httptransport.RuntimeInventory, uiQuery web.Query, cfg config.RuntimeConfig, ready func(context.Context) error) (http.Handler, error) {
	logger, err := httptransport.NewLogger(cfg.Log.Level, cfg.Log.Format, os.Stdout)
	if err != nil {
		return nil, fmt.Errorf("create logger: %w", err)
	}

	apiHandler := httptransport.NewServer(registry, httptransport.Options{
		BootstrapToken:      cfg.Auth.BootstrapToken,
		Runtime:             runtimeInventory,
		Ready:               ready,
		Logger:              logger,
		Metrics:             httptransport.NewMetrics(),
		MaxRequestBodyBytes: cfg.Server.MaxRequestBodyBytes,
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
