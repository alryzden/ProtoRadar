package bootstrap

import (
	"context"
	"net/http"

	"github.com/alryzden/ProtoRadar/internal/config"
	httptransport "github.com/alryzden/ProtoRadar/internal/transport/http"
)

func NewHTTPHandler(registry httptransport.Registry, cfg config.RuntimeConfig, ready func(context.Context) error) http.Handler {
	return httptransport.NewServer(registry, httptransport.Options{
		BootstrapToken: cfg.Auth.BootstrapToken,
		Ready:          ready,
	}).Handler()
}
