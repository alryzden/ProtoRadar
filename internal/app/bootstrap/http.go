package bootstrap

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/alryzden/ProtoRadar/internal/config"
	httptransport "github.com/alryzden/ProtoRadar/internal/transport/http"
	"github.com/alryzden/ProtoRadar/internal/transport/web"
)

func NewHTTPHandler(registry httptransport.Registry, uiQuery web.Query, cfg config.RuntimeConfig, ready func(context.Context) error) (http.Handler, error) {
	apiHandler := httptransport.NewServer(registry, httptransport.Options{
		BootstrapToken: cfg.Auth.BootstrapToken,
		Ready:          ready,
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
