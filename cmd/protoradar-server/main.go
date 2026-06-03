package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/alryzden/ProtoRadar/internal/app/bootstrap"
	"github.com/alryzden/ProtoRadar/internal/config"
)

func main() {
	configPath := flag.String("config", "", "path to server config file")
	flag.Parse()

	rawConfig, err := config.LoadFile(*configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	runtimeConfig, err := rawConfig.Runtime()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	server, err := bootstrap.NewServer(ctx, runtimeConfig, os.DirFS("migrations"))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer server.Close()

	errCh := make(chan error, 1)
	go func() {
		errCh <- server.HTTPServer.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.HTTPServer.Shutdown(shutdownCtx); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
}
