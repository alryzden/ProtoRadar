package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/alryzden/ProtoRadar/internal/app/bootstrap"
	"github.com/alryzden/ProtoRadar/internal/config"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stderr, defaultRunDeps()))
}

type runDeps struct {
	loadConfig    func(string) (config.Config, error)
	runtimeConfig func(config.Config) (config.RuntimeConfig, error)
	newServer     func(context.Context, config.RuntimeConfig, fs.FS) (managedServer, error)
	signalContext func(context.Context) (context.Context, context.CancelFunc)
	migrations    fs.FS
}

type managedServer interface {
	httpServer() httpServer
	Close() error
}

type httpServer interface {
	ListenAndServe() error
	Shutdown(context.Context) error
}

type bootstrapServer struct {
	server *bootstrap.Server
}

func (server bootstrapServer) httpServer() httpServer {
	return server.server.HTTPServer
}

func (server bootstrapServer) Close() error {
	return server.server.Close()
}

func defaultRunDeps() runDeps {
	return runDeps{
		loadConfig: config.LoadFile,
		runtimeConfig: func(cfg config.Config) (config.RuntimeConfig, error) {
			return cfg.Runtime()
		},
		newServer: func(ctx context.Context, cfg config.RuntimeConfig, migrations fs.FS) (managedServer, error) {
			server, err := bootstrap.NewServer(ctx, cfg, migrations)
			if err != nil {
				return nil, err
			}
			return bootstrapServer{server: server}, nil
		},
		signalContext: func(ctx context.Context) (context.Context, context.CancelFunc) {
			return signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
		},
		migrations: os.DirFS("migrations"),
	}
}

func run(args []string, stderr io.Writer, deps runDeps) int {
	flags := flag.NewFlagSet("protoradar-server", flag.ContinueOnError)
	flags.SetOutput(stderr)
	configPath := flags.String("config", "", "path to server config file")
	if err := flags.Parse(args); err != nil {
		return 1
	}

	rawConfig, err := deps.loadConfig(*configPath)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	runtimeConfig, err := deps.runtimeConfig(rawConfig)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	ctx, stop := deps.signalContext(context.Background())
	defer stop()

	server, err := deps.newServer(ctx, runtimeConfig, deps.migrations)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return runServer(ctx, server, stderr)
}

func runServer(ctx context.Context, server managedServer, stderr io.Writer) int {
	errCh := make(chan error, 1)
	httpServer := server.httpServer()
	go func() {
		errCh <- httpServer.ListenAndServe()
	}()

	exitCode := 0
	select {
	case <-ctx.Done():
		// Use a fresh root so shutdown has its full timeout after the signal context is canceled.
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			fmt.Fprintln(stderr, err)
			exitCode = 1
		}
		cancel()
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			fmt.Fprintln(stderr, err)
			exitCode = 1
		}
	}
	if err := server.Close(); err != nil {
		fmt.Fprintln(stderr, err)
		exitCode = 1
	}
	return exitCode
}
