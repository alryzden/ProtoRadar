package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alryzden/ProtoRadar/internal/cli/config"
)

func TestLoginRequiresServerAndToken(t *testing.T) {
	app := App{}

	if err := app.Run(context.Background(), []string{"login", "--token", "prr_token"}); err == nil || !strings.Contains(err.Error(), "--server") {
		t.Fatalf("missing server error = %v", err)
	}
	if err := app.Run(context.Background(), []string{"login", "--server", "http://localhost:8080"}); err == nil || !strings.Contains(err.Error(), "--token") {
		t.Fatalf("missing token error = %v", err)
	}
}

func TestLoginRejectsInvalidServerURL(t *testing.T) {
	app := App{}

	err := app.Run(context.Background(), []string{"login", "--server", "localhost:8080", "--token", "prr_token"})
	if err == nil || !strings.Contains(err.Error(), "http or https") {
		t.Fatalf("invalid server error = %v", err)
	}
}

func TestLoginSavesConfigAndDoesNotPrintToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer prr_secret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	var output bytes.Buffer
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	app := App{
		ConfigPath: configPath,
		HTTPClient: server.Client(),
		Out:        &output,
	}

	err := app.Run(context.Background(), []string{"login", "--server", server.URL, "--token", "prr_secret"})
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.ServerURL != server.URL {
		t.Fatalf("server url = %q", cfg.ServerURL)
	}
	if cfg.Token != "prr_secret" {
		t.Fatalf("token was not saved")
	}
	if strings.Contains(output.String(), "prr_secret") {
		t.Fatalf("token printed in output: %s", output.String())
	}
	if !strings.Contains(output.String(), "Logged in") {
		t.Fatalf("success output = %q", output.String())
	}
}

func TestLoginRejectsInvalidToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	app := App{
		ConfigPath: filepath.Join(t.TempDir(), "config.yaml"),
		HTTPClient: server.Client(),
		Out:        &bytes.Buffer{},
	}

	err := app.Run(context.Background(), []string{"login", "--server", server.URL, "--token", "bad-token"})
	if err == nil || !strings.Contains(err.Error(), "login failed") {
		t.Fatalf("error = %v", err)
	}
}

func TestCommandsUseHTTPAPI(t *testing.T) {
	var createdModule string
	var publishedVersion string
	var uploadedArtifact []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer prr_token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/modules":
			var req map[string]string
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			createdModule = req["name"]
			writeJSON(t, w, http.StatusCreated, map[string]any{
				"id":             createdModule,
				"name":           createdModule,
				"description":    req["description"],
				"repository_url": req["repository_url"],
				"created_at":     "2026-06-04T12:00:00Z",
				"updated_at":     "2026-06-04T12:00:00Z",
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/modules":
			writeJSON(t, w, http.StatusOK, map[string]any{
				"modules": []map[string]any{{
					"id":         "user-api",
					"name":       "user-api",
					"updated_at": "2026-06-04T12:00:00Z",
				}},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/modules/user-api/versions":
			writeJSON(t, w, http.StatusOK, map[string]any{
				"versions": []map[string]any{{
					"id":         "version-1",
					"module_id":  "user-api",
					"version":    "v1.0.0",
					"digest":     "sha256:server-digest",
					"status":     "published",
					"created_at": "2026-06-04T12:00:00Z",
				}},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/modules/user-api/versions":
			if err := r.ParseMultipartForm(32 << 20); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			publishedVersion = r.FormValue("version")
			file, _, err := r.FormFile("artifact")
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			defer file.Close()
			uploadedArtifact, _ = io.ReadAll(file)
			writeJSON(t, w, http.StatusCreated, map[string]any{
				"version": map[string]any{
					"id":         "version-1",
					"module_id":  "user-api",
					"version":    publishedVersion,
					"digest":     "sha256:server-digest",
					"status":     "published",
					"created_at": "2026-06-04T12:00:00Z",
				},
				"artifact": map[string]any{
					"id":                "artifact-1",
					"module_version_id": "version-1",
					"checksum_sha256":   "server-checksum",
					"size_bytes":        len(uploadedArtifact),
					"created_at":        "2026-06-04T12:00:00Z",
				},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/modules/user-api/versions/v1.0.0/artifact":
			w.Header().Set("Content-Type", "application/gzip")
			w.Header().Set("X-ProtoRadar-Digest", "sha256:server-digest")
			w.Header().Set("X-ProtoRadar-Checksum-SHA256", "server-checksum")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(buildArchive(t, "user.proto", "syntax = \"proto3\";"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	saveCLIConfig(t, configPath, server.URL)
	var output bytes.Buffer
	app := App{ConfigPath: configPath, HTTPClient: server.Client(), Out: &output}

	if err := app.Run(context.Background(), []string{"module", "create", "user-api", "--description", "User service protobuf contracts", "--repository-url", "https://gitlab.example.com/platform/user-api"}); err != nil {
		t.Fatalf("module create: %v", err)
	}
	if createdModule != "user-api" {
		t.Fatalf("created module = %q", createdModule)
	}

	output.Reset()
	if err := app.Run(context.Background(), []string{"list"}); err != nil {
		t.Fatalf("list: %v", err)
	}
	if !strings.Contains(output.String(), "user-api") || !strings.Contains(output.String(), "v1.0.0") {
		t.Fatalf("list output = %q", output.String())
	}

	protoDir := t.TempDir()
	writeFile(t, filepath.Join(protoDir, "user.proto"), "syntax = \"proto3\";")
	output.Reset()
	if err := app.Run(context.Background(), []string{"push", "user-api", "--version", "v1.0.0", "--path", protoDir}); err != nil {
		t.Fatalf("push: %v", err)
	}
	if publishedVersion != "v1.0.0" {
		t.Fatalf("published version = %q", publishedVersion)
	}
	if len(archiveNames(t, uploadedArtifact)) != 1 {
		t.Fatalf("uploaded artifact did not contain one proto")
	}

	outputDir := filepath.Join(t.TempDir(), "downloaded")
	output.Reset()
	if err := app.Run(context.Background(), []string{"pull", "user-api", "--version", "v1.0.0", "--output", outputDir}); err != nil {
		t.Fatalf("pull: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outputDir, "user.proto")); err != nil {
		t.Fatalf("pulled proto: %v", err)
	}
}

func saveCLIConfig(t *testing.T, path string, serverURL string) {
	t.Helper()
	if err := config.Save(path, config.Config{ServerURL: serverURL, Token: "prr_token"}); err != nil {
		t.Fatalf("save config: %v", err)
	}
}

func writeJSON(t *testing.T, w http.ResponseWriter, status int, body any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		t.Fatalf("write json: %v", err)
	}
}
