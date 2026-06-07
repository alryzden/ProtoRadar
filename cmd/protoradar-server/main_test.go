package main

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/alryzden/ProtoRadar/internal/config"
)

func TestShutdownErrorStillClosesServer(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	server := &fakeManagedServer{http: &fakeHTTPServer{listenBlock: make(chan struct{}), shutdownErr: errors.New("shutdown failed")}}
	var stderr bytes.Buffer

	code := run([]string{}, &stderr, testRunDeps(ctx, server, nil))

	if code == 0 {
		t.Fatalf("exit code = 0, want non-zero")
	}
	if !server.closed {
		t.Fatalf("server was not closed")
	}
	if !strings.Contains(stderr.String(), "shutdown failed") {
		t.Fatalf("stderr = %q, want shutdown error", stderr.String())
	}
}

func TestUnexpectedListenAndServeErrorStillClosesServer(t *testing.T) {
	ctx := context.Background()
	server := &fakeManagedServer{http: &fakeHTTPServer{listenErr: errors.New("listen failed")}}
	var stderr bytes.Buffer

	code := run([]string{}, &stderr, testRunDeps(ctx, server, nil))

	if code == 0 {
		t.Fatalf("exit code = 0, want non-zero")
	}
	if !server.closed {
		t.Fatalf("server was not closed")
	}
	if !strings.Contains(stderr.String(), "listen failed") {
		t.Fatalf("stderr = %q, want listen error", stderr.String())
	}
}

func TestNormalSignalShutdownCallsServerClose(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	server := &fakeManagedServer{http: &fakeHTTPServer{listenBlock: make(chan struct{})}}
	var stderr bytes.Buffer

	code := run([]string{}, &stderr, testRunDeps(ctx, server, nil))

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	if !server.closed {
		t.Fatalf("server was not closed")
	}
	if server.http.shutdownCalls != 1 {
		t.Fatalf("shutdown calls = %d, want 1", server.http.shutdownCalls)
	}
}

func TestHTTPServerClosedDoesNotReturnNonZero(t *testing.T) {
	ctx := context.Background()
	server := &fakeManagedServer{http: &fakeHTTPServer{listenErr: http.ErrServerClosed}}
	var stderr bytes.Buffer

	code := run([]string{}, &stderr, testRunDeps(ctx, server, nil))

	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	if !server.closed {
		t.Fatalf("server was not closed")
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestCloseErrorReturnsNonZero(t *testing.T) {
	ctx := context.Background()
	server := &fakeManagedServer{http: &fakeHTTPServer{listenErr: http.ErrServerClosed}, closeErr: errors.New("close failed")}
	var stderr bytes.Buffer

	code := run([]string{}, &stderr, testRunDeps(ctx, server, nil))

	if code == 0 {
		t.Fatalf("exit code = 0, want non-zero")
	}
	if !server.closed {
		t.Fatalf("server was not closed")
	}
	if !strings.Contains(stderr.String(), "close failed") {
		t.Fatalf("stderr = %q, want close error", stderr.String())
	}
}

func TestBootstrapFailureBeforeServerCreationDoesNotCloseServer(t *testing.T) {
	ctx := context.Background()
	server := &fakeManagedServer{http: &fakeHTTPServer{}}
	var stderr bytes.Buffer

	code := run([]string{}, &stderr, testRunDeps(ctx, server, errors.New("bootstrap failed")))

	if code == 0 {
		t.Fatalf("exit code = 0, want non-zero")
	}
	if server.closed {
		t.Fatalf("server was closed before successful creation")
	}
	if !strings.Contains(stderr.String(), "bootstrap failed") {
		t.Fatalf("stderr = %q, want bootstrap error", stderr.String())
	}
}

func TestOSExitOnlyAppearsInMain(t *testing.T) {
	body, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("read main.go: %v", err)
	}
	text := string(body)
	if strings.Count(text, "os.Exit") != 1 {
		t.Fatalf("os.Exit count = %d, want 1", strings.Count(text, "os.Exit"))
	}
	if !strings.Contains(text, "func main() {\n\tos.Exit(run(") {
		t.Fatalf("os.Exit should be called only by outer main")
	}
}

func testRunDeps(ctx context.Context, server managedServer, newServerErr error) runDeps {
	return runDeps{
		loadConfig: func(string) (config.Config, error) {
			return config.Config{}, nil
		},
		runtimeConfig: func(config.Config) (config.RuntimeConfig, error) {
			return config.RuntimeConfig{}, nil
		},
		newServer: func(context.Context, config.RuntimeConfig, fs.FS) (managedServer, error) {
			if newServerErr != nil {
				return nil, newServerErr
			}
			return server, nil
		},
		signalContext: func(context.Context) (context.Context, context.CancelFunc) {
			return ctx, func() {}
		},
	}
}

type fakeManagedServer struct {
	http     *fakeHTTPServer
	closed   bool
	closeErr error
}

func (server *fakeManagedServer) httpServer() httpServer {
	return server.http
}

func (server *fakeManagedServer) Close() error {
	server.closed = true
	return server.closeErr
}

type fakeHTTPServer struct {
	listenBlock   <-chan struct{}
	listenErr     error
	shutdownErr   error
	shutdownCalls int
}

func (server *fakeHTTPServer) ListenAndServe() error {
	if server.listenBlock != nil {
		<-server.listenBlock
	}
	return server.listenErr
}

func (server *fakeHTTPServer) Shutdown(context.Context) error {
	server.shutdownCalls++
	return server.shutdownErr
}
