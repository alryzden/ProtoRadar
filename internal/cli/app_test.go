package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
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

func TestHelpContainsKeyCommands(t *testing.T) {
	var output bytes.Buffer
	app := App{Out: &output}

	if err := app.Run(context.Background(), []string{"--help"}); err != nil {
		t.Fatalf("help: %v", err)
	}
	text := output.String()
	for _, want := range []string{"login", "version", "module create", "module list", "push", "pull", "check-breaking", "gitlab mr-check", "runtime report", "PROTORADAR_SERVER_URL", "PROTORADAR_TOKEN"} {
		if !strings.Contains(text, want) {
			t.Fatalf("help missing %q:\n%s", want, text)
		}
	}
}

func TestVersionCommandPrintsBuildMetadata(t *testing.T) {
	var output bytes.Buffer
	app := App{Out: &output}

	if err := app.Run(context.Background(), []string{"version"}); err != nil {
		t.Fatalf("version: %v", err)
	}
	text := output.String()
	for _, want := range []string{"version:", "commit:", "build_date:"} {
		if !strings.Contains(text, want) {
			t.Fatalf("version output missing %q: %q", want, text)
		}
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
				"module":     "user-api",
				"version":    publishedVersion,
				"status":     "published",
				"created_at": "2026-06-04T12:00:00Z",
				"source_artifact": map[string]any{
					"kind":            "source_archive",
					"checksum_sha256": "server-source-checksum",
					"size_bytes":      len(uploadedArtifact),
				},
				"buf_image_artifact": map[string]any{
					"kind":            "buf_image",
					"checksum_sha256": "server-buf-image-checksum",
					"size_bytes":      123,
				},
				"buf": map[string]any{
					"config_present": true,
					"lock_present":   true,
					"lint_status":    "warning",
				},
				"metadata_summary": map[string]any{
					"files":       2,
					"packages":    1,
					"services":    1,
					"methods":     2,
					"messages":    3,
					"fields":      4,
					"enums":       1,
					"enum_values": 2,
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
	writeFile(t, filepath.Join(protoDir, "buf.yaml"), "version: v2\n")
	writeFile(t, filepath.Join(protoDir, "buf.lock"), "deps: []\n")
	writeFile(t, filepath.Join(protoDir, "user.proto"), "syntax = \"proto3\";")
	output.Reset()
	if err := app.Run(context.Background(), []string{"push", "user-api", "--version", "v1.0.0", "--path", protoDir}); err != nil {
		t.Fatalf("push: %v", err)
	}
	if publishedVersion != "v1.0.0" {
		t.Fatalf("published version = %q", publishedVersion)
	}
	names := archiveNames(t, uploadedArtifact)
	if !strings.Contains(strings.Join(names, ","), "buf.yaml") || !strings.Contains(strings.Join(names, ","), "buf.lock") || !strings.Contains(strings.Join(names, ","), "user.proto") {
		t.Fatalf("uploaded artifact names = %#v", names)
	}
	if !strings.Contains(output.String(), "Lint status: warning") {
		t.Fatalf("push output missing lint status: %q", output.String())
	}
	if !strings.Contains(output.String(), "Metadata: files=2 packages=1 services=1 methods=2 messages=3 fields=4 enums=1 enum_values=2") {
		t.Fatalf("push output missing metadata summary: %q", output.String())
	}
	if !strings.Contains(output.String(), "Buf image checksum SHA-256: server-buf-image-checksum") {
		t.Fatalf("push output missing buf image checksum: %q", output.String())
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

func TestPushDisplaysAPIErrorsClearly(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer prr_token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.Method == http.MethodPost && r.URL.Path == "/api/v1/modules/user-api/versions" {
			writeJSON(t, w, http.StatusUnprocessableEntity, map[string]string{"error": "buf build failed"})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	saveCLIConfig(t, configPath, server.URL)
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "buf.yaml"), "version: v2\n")
	writeFile(t, filepath.Join(root, "user.proto"), "syntax = \"proto3\";")

	app := App{ConfigPath: configPath, HTTPClient: server.Client(), Out: &bytes.Buffer{}}
	err := app.Run(context.Background(), []string{"push", "user-api", "--version", "v1.0.0", "--path", root})
	if err == nil || !strings.Contains(err.Error(), "buf build failed") {
		t.Fatalf("error = %v", err)
	}
}

func TestCheckBreakingRequiresModuleAndPath(t *testing.T) {
	app := App{}

	err := app.Run(context.Background(), []string{"check-breaking", "--path", t.TempDir()})
	if err == nil || !strings.Contains(err.Error(), "module name") {
		t.Fatalf("missing module error = %v", err)
	}
	err = app.Run(context.Background(), []string{"check-breaking", "user-api"})
	if err == nil || !strings.Contains(err.Error(), "--path") {
		t.Fatalf("missing path error = %v", err)
	}
}

func TestCheckBreakingRequiresBufYAML(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "user.proto"), "syntax = \"proto3\";")
	app := App{ConfigPath: filepath.Join(t.TempDir(), "config.yaml")}

	err := app.Run(context.Background(), []string{"check-breaking", "user-api", "--path", root})
	if err == nil || !strings.Contains(err.Error(), "buf.yaml") {
		t.Fatalf("error = %v", err)
	}
}

func TestCheckBreakingPackagesWorkspaceAndDefaultsAgainstLatest(t *testing.T) {
	var gotAgainst string
	var gotTargetRef string
	var uploadedArtifact []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer prr_token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/modules/user-api/breaking-checks" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		gotAgainst = r.FormValue("against")
		gotTargetRef = r.FormValue("target_ref")
		file, _, err := r.FormFile("artifact")
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		defer file.Close()
		uploadedArtifact, _ = io.ReadAll(file)
		writeJSON(t, w, http.StatusOK, map[string]any{
			"id":            "report-1",
			"module":        "user-api",
			"against":       "v1.0.0",
			"target_ref":    "local",
			"status":        "passed",
			"change_count":  0,
			"human_summary": "No breaking changes found.",
			"changes":       []map[string]any{},
			"created_at":    "2026-06-04T12:00:00Z",
		})
	}))
	defer server.Close()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	saveCLIConfig(t, configPath, server.URL)
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "buf.yaml"), "version: v2\n")
	writeFile(t, filepath.Join(root, "buf.lock"), "deps: []\n")
	writeFile(t, filepath.Join(root, "proto", "user", "v1", "user.proto"), "syntax = \"proto3\";")
	var output bytes.Buffer
	app := App{ConfigPath: configPath, HTTPClient: server.Client(), Out: &output}

	err := app.Run(context.Background(), []string{"check-breaking", "user-api", "--path", root})
	if err != nil {
		t.Fatalf("check-breaking: %v", err)
	}
	if gotAgainst != "latest" {
		t.Fatalf("against = %q", gotAgainst)
	}
	if gotTargetRef != "" {
		t.Fatalf("target_ref = %q", gotTargetRef)
	}
	names := archiveNames(t, uploadedArtifact)
	for _, want := range []string{"buf.yaml", "buf.lock", "proto/user/v1/user.proto"} {
		if !slices.Contains(names, want) {
			t.Fatalf("missing %s in archive names %#v", want, names)
		}
	}
	if !strings.Contains(output.String(), "No breaking changes found.") {
		t.Fatalf("output = %q", output.String())
	}
}

func TestCheckBreakingRejectsNoProtoFiles(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "buf.yaml"), "version: v2\n")
	app := App{}

	err := app.Run(context.Background(), []string{"check-breaking", "user-api", "--path", root})
	if err == nil || !strings.Contains(err.Error(), "no .proto") {
		t.Fatalf("error = %v", err)
	}
}

func TestCheckBreakingDisplaysHumanSummaryAndPassedExitCode(t *testing.T) {
	server := breakingServer(t, http.StatusOK, map[string]any{
		"id":            "report-1",
		"module":        "user-api",
		"against":       "v1.0.0",
		"target_ref":    "local",
		"status":        "passed",
		"change_count":  0,
		"human_summary": "ProtoRadar Breaking Change Report\nNo breaking changes found.",
		"changes":       []map[string]any{},
		"created_at":    "2026-06-04T12:00:00Z",
	})
	defer server.Close()

	output, err := runCheckBreakingAgainstServer(t, server, []string{"check-breaking", "user-api", "--path", validWorkspace(t), "--against", "v1.0.0"})
	if err != nil {
		t.Fatalf("check-breaking: %v", err)
	}
	if exitCode(err) != 0 {
		t.Fatalf("exit code = %d", exitCode(err))
	}
	if !strings.Contains(output, "No breaking changes found.") {
		t.Fatalf("output = %q", output)
	}
}

func TestCheckBreakingBreakingStatusMapsToExitCodeOne(t *testing.T) {
	server := breakingServer(t, http.StatusOK, map[string]any{
		"id":            "report-1",
		"module":        "user-api",
		"against":       "v1.0.0",
		"target_ref":    "local",
		"status":        "breaking",
		"change_count":  1,
		"human_summary": "ProtoRadar Breaking Change Report\nResult: breaking changes found.",
		"changes": []map[string]any{{
			"file_path": "user/v1/user.proto",
			"rule_id":   "FIELD_SAME_TYPE",
			"message":   "field changed",
			"severity":  "error",
		}},
		"created_at": "2026-06-04T12:00:00Z",
	})
	defer server.Close()

	output, err := runCheckBreakingAgainstServer(t, server, []string{"check-breaking", "user-api", "--path", validWorkspace(t), "--against", "v1.0.0", "--target-ref", "local"})
	if exitCode(err) != 1 {
		t.Fatalf("exit code = %d, err = %v", exitCode(err), err)
	}
	if !strings.Contains(output, "breaking changes found") {
		t.Fatalf("output = %q", output)
	}
}

func TestCheckBreakingFallbackSummary(t *testing.T) {
	server := breakingServer(t, http.StatusOK, map[string]any{
		"id":           "report-1",
		"module":       "user-api",
		"against":      "v1.0.0",
		"target_ref":   "local",
		"status":       "breaking",
		"change_count": 1,
		"changes": []map[string]any{{
			"file_path": "user/v1/user.proto",
			"rule_id":   "FIELD_SAME_TYPE",
			"symbol":    "user.v1.User.email",
			"message":   "field changed",
			"severity":  "error",
		}},
		"created_at": "2026-06-04T12:00:00Z",
	})
	defer server.Close()

	output, err := runCheckBreakingAgainstServer(t, server, []string{"check-breaking", "user-api", "--path", validWorkspace(t), "--against", "v1.0.0"})
	if exitCode(err) != 1 {
		t.Fatalf("exit code = %d", exitCode(err))
	}
	if !strings.Contains(output, "ProtoRadar Breaking Change Report") || !strings.Contains(output, "Rule: FIELD_SAME_TYPE") {
		t.Fatalf("output = %q", output)
	}
}

func TestCheckBreakingAPIErrorMapsToExitCodeTwoAndShowsMessage(t *testing.T) {
	server := breakingServer(t, http.StatusUnprocessableEntity, map[string]string{"error": "buf config not found"})
	defer server.Close()

	output, err := runCheckBreakingAgainstServer(t, server, []string{"check-breaking", "user-api", "--path", validWorkspace(t), "--against", "v1.0.0"})
	if exitCode(err) != 2 {
		t.Fatalf("exit code = %d, err = %v", exitCode(err), err)
	}
	if !strings.Contains(err.Error(), "buf config not found") {
		t.Fatalf("error = %v", err)
	}
	if strings.Contains(output, "prr_token") || strings.Contains(err.Error(), "prr_token") {
		t.Fatalf("raw token leaked: output=%q err=%v", output, err)
	}
}

func TestCommandsUseEnvServerURLAndToken(t *testing.T) {
	var called bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if r.Header.Get("Authorization") != "Bearer env_token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/modules" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		writeJSON(t, w, http.StatusOK, map[string]any{"modules": []map[string]any{}})
	}))
	defer server.Close()

	t.Setenv("PROTORADAR_SERVER_URL", server.URL)
	t.Setenv("PROTORADAR_TOKEN", "env_token")

	var output bytes.Buffer
	app := App{
		ConfigPath: filepath.Join(t.TempDir(), "missing-config.yaml"),
		HTTPClient: server.Client(),
		Out:        &output,
	}
	err := app.Run(context.Background(), []string{"list"})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if !called {
		t.Fatal("env-backed server was not called")
	}
	if strings.Contains(output.String(), "env_token") {
		t.Fatalf("raw token printed: %q", output.String())
	}
}

func TestCommandsEnvOverridesConfigValues(t *testing.T) {
	var envServerCalled bool
	envServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		envServerCalled = true
		if r.Header.Get("Authorization") != "Bearer env_token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		writeJSON(t, w, http.StatusOK, map[string]any{"modules": []map[string]any{}})
	}))
	defer envServer.Close()

	configServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("config server should not be called")
	}))
	defer configServer.Close()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	saveCLIConfig(t, configPath, configServer.URL)
	t.Setenv("PROTORADAR_SERVER_URL", envServer.URL)
	t.Setenv("PROTORADAR_TOKEN", "env_token")

	var output bytes.Buffer
	app := App{ConfigPath: configPath, HTTPClient: envServer.Client(), Out: &output}
	err := app.Run(context.Background(), []string{"list"})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if !envServerCalled {
		t.Fatal("env server was not called")
	}
	if strings.Contains(output.String(), "env_token") {
		t.Fatalf("raw token printed: %q", output.String())
	}
}

func TestCheckBreakingReportFileWritesPassedResult(t *testing.T) {
	server := breakingServer(t, http.StatusOK, map[string]any{
		"id":            "report-1",
		"module":        "user-api",
		"against":       "v1.0.0",
		"target_ref":    "local",
		"status":        "passed",
		"change_count":  0,
		"human_summary": "ProtoRadar Breaking Change Report\nNo breaking changes found.",
		"changes":       []map[string]any{},
		"created_at":    "2026-06-04T12:00:00Z",
	})
	defer server.Close()

	reportPath := filepath.Join(t.TempDir(), "breaking-report.txt")
	output, err := runCheckBreakingAgainstServer(t, server, []string{
		"check-breaking",
		"user-api",
		"--path",
		validWorkspace(t),
		"--against",
		"v1.0.0",
		"--report-file",
		reportPath,
	})
	if err != nil {
		t.Fatalf("check-breaking: %v", err)
	}
	body, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	if !strings.Contains(string(body), "No breaking changes found.") {
		t.Fatalf("report body = %q", string(body))
	}
	if !strings.Contains(output, "No breaking changes found.") {
		t.Fatalf("output = %q", output)
	}
}

func TestCheckBreakingReportFileWritesBreakingResult(t *testing.T) {
	server := breakingServer(t, http.StatusOK, map[string]any{
		"id":            "report-1",
		"module":        "user-api",
		"against":       "v1.0.0",
		"target_ref":    "local",
		"status":        "breaking",
		"change_count":  1,
		"human_summary": "ProtoRadar Breaking Change Report\nResult: breaking changes found.",
		"changes":       []map[string]any{},
		"created_at":    "2026-06-04T12:00:00Z",
	})
	defer server.Close()

	reportPath := filepath.Join(t.TempDir(), "breaking-report.txt")
	output, err := runCheckBreakingAgainstServer(t, server, []string{
		"check-breaking",
		"user-api",
		"--path",
		validWorkspace(t),
		"--against",
		"v1.0.0",
		"--report-file",
		reportPath,
	})
	if exitCode(err) != 1 {
		t.Fatalf("exit code = %d, err = %v", exitCode(err), err)
	}
	body, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	if !strings.Contains(string(body), "breaking changes found") {
		t.Fatalf("report body = %q", string(body))
	}
	if !strings.Contains(output, "breaking changes found") {
		t.Fatalf("output = %q", output)
	}
}

func TestCheckBreakingReportFileWriteFailureMapsToExitCodeTwo(t *testing.T) {
	server := breakingServer(t, http.StatusOK, map[string]any{
		"id":            "report-1",
		"module":        "user-api",
		"against":       "v1.0.0",
		"target_ref":    "local",
		"status":        "passed",
		"change_count":  0,
		"human_summary": "No breaking changes found.",
		"changes":       []map[string]any{},
		"created_at":    "2026-06-04T12:00:00Z",
	})
	defer server.Close()

	_, err := runCheckBreakingAgainstServer(t, server, []string{
		"check-breaking",
		"user-api",
		"--path",
		validWorkspace(t),
		"--against",
		"v1.0.0",
		"--report-file",
		t.TempDir(),
	})
	if exitCode(err) != 2 {
		t.Fatalf("exit code = %d, err = %v", exitCode(err), err)
	}
}

func TestModuleLinkGitLabRequiresModuleArgument(t *testing.T) {
	app := App{}

	err := app.Run(context.Background(), []string{"module", "link-gitlab", "--project-id", "123", "--project-path", "platform/user-api", "--gitlab-base-url", "https://gitlab.example.com"})
	if err == nil || !strings.Contains(err.Error(), "module name") {
		t.Fatalf("error = %v", err)
	}
}

func TestModuleLinkGitLabAcceptsExplicitFlagsAndCallsAPI(t *testing.T) {
	var gotPath string
	var gotRequest map[string]any
	server := moduleLinkGitLabServer(t, func(r *http.Request) {
		gotPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&gotRequest); err != nil {
			t.Fatalf("decode request: %v", err)
		}
	}, http.StatusOK)
	defer server.Close()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	saveCLIConfig(t, configPath, server.URL)
	var output bytes.Buffer
	app := App{ConfigPath: configPath, HTTPClient: server.Client(), Out: &output}

	err := app.Run(context.Background(), []string{
		"module",
		"link-gitlab",
		"user-api",
		"--project-id",
		"12345",
		"--project-path",
		"platform/user-api",
		"--gitlab-base-url",
		"https://gitlab.example.com",
	})
	if err != nil {
		t.Fatalf("module link-gitlab: %v", err)
	}
	if gotPath != "/api/v1/modules/user-api/gitlab-project" {
		t.Fatalf("path = %q", gotPath)
	}
	if gotRequest["gitlab_base_url"] != "https://gitlab.example.com" {
		t.Fatalf("gitlab_base_url = %#v", gotRequest["gitlab_base_url"])
	}
	if gotRequest["gitlab_project_id"] != float64(12345) {
		t.Fatalf("gitlab_project_id = %#v", gotRequest["gitlab_project_id"])
	}
	if gotRequest["gitlab_project_path"] != "platform/user-api" {
		t.Fatalf("gitlab_project_path = %#v", gotRequest["gitlab_project_path"])
	}
	if !strings.Contains(output.String(), "Linked module user-api to GitLab project platform/user-api (12345)") {
		t.Fatalf("output = %q", output.String())
	}
}

func TestModuleLinkGitLabUsesCIEnvDefaults(t *testing.T) {
	var gotRequest map[string]any
	server := moduleLinkGitLabServer(t, func(r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotRequest); err != nil {
			t.Fatalf("decode request: %v", err)
		}
	}, http.StatusOK)
	defer server.Close()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	saveCLIConfig(t, configPath, server.URL)
	t.Setenv("CI_PROJECT_ID", "67890")
	t.Setenv("CI_PROJECT_PATH", "platform/billing-api")
	t.Setenv("CI_SERVER_URL", "https://gitlab.example.com")

	var output bytes.Buffer
	app := App{ConfigPath: configPath, HTTPClient: server.Client(), Out: &output}
	err := app.Run(context.Background(), []string{"module", "link-gitlab", "billing-api"})
	if err != nil {
		t.Fatalf("module link-gitlab: %v", err)
	}
	if gotRequest["gitlab_base_url"] != "https://gitlab.example.com" {
		t.Fatalf("gitlab_base_url = %#v", gotRequest["gitlab_base_url"])
	}
	if gotRequest["gitlab_project_id"] != float64(67890) {
		t.Fatalf("gitlab_project_id = %#v", gotRequest["gitlab_project_id"])
	}
	if gotRequest["gitlab_project_path"] != "platform/billing-api" {
		t.Fatalf("gitlab_project_path = %#v", gotRequest["gitlab_project_path"])
	}
	if !strings.Contains(output.String(), "Linked module billing-api to GitLab project platform/billing-api (67890)") {
		t.Fatalf("output = %q", output.String())
	}
}

func TestModuleLinkGitLabValidatesMissingValues(t *testing.T) {
	t.Setenv("CI_PROJECT_ID", "")
	t.Setenv("CI_PROJECT_PATH", "")
	t.Setenv("CI_SERVER_URL", "")

	app := App{}
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "project id",
			args: []string{"module", "link-gitlab", "user-api", "--project-path", "platform/user-api", "--gitlab-base-url", "https://gitlab.example.com"},
			want: "--project-id",
		},
		{
			name: "positive project id",
			args: []string{"module", "link-gitlab", "user-api", "--project-id", "0", "--project-path", "platform/user-api", "--gitlab-base-url", "https://gitlab.example.com"},
			want: "greater than zero",
		},
		{
			name: "project path",
			args: []string{"module", "link-gitlab", "user-api", "--project-id", "123", "--gitlab-base-url", "https://gitlab.example.com"},
			want: "--project-path",
		},
		{
			name: "base url",
			args: []string{"module", "link-gitlab", "user-api", "--project-id", "123", "--project-path", "platform/user-api"},
			want: "--gitlab-base-url",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := app.Run(context.Background(), tt.args)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestModuleLinkGitLabConflictMapsClearly(t *testing.T) {
	server := moduleLinkGitLabServer(t, nil, http.StatusConflict)
	defer server.Close()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	saveCLIConfig(t, configPath, server.URL)
	app := App{ConfigPath: configPath, HTTPClient: server.Client(), Out: &bytes.Buffer{}}

	err := app.Run(context.Background(), []string{
		"module",
		"link-gitlab",
		"user-api",
		"--project-id",
		"12345",
		"--project-path",
		"platform/user-api",
		"--gitlab-base-url",
		"https://gitlab.example.com",
	})
	if err == nil || !strings.Contains(err.Error(), "already linked") {
		t.Fatalf("error = %v", err)
	}
}

func TestModuleDependenciesRequiresModuleArgument(t *testing.T) {
	err := App{}.Run(context.Background(), []string{"module", "dependencies"})
	if err == nil || !strings.Contains(err.Error(), "module name") {
		t.Fatalf("error = %v", err)
	}
}

func TestModuleDependenciesCallsExpectedEndpointAndPrintsGraph(t *testing.T) {
	var gotPath string
	server := dependencyCommandServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/modules/user-api/dependencies" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		writeJSON(t, w, http.StatusOK, dependencyGraphResponse())
	})
	defer server.Close()

	output, err := runCLIAgainstServer(t, server, []string{"module", "dependencies", "user-api"})
	if err != nil {
		t.Fatalf("module dependencies: %v", err)
	}
	if gotPath != "/api/v1/modules/user-api/dependencies" {
		t.Fatalf("path = %q", gotPath)
	}
	for _, want := range []string{
		"Module: user-api",
		"Downstream consumers:",
		"- billing-api@v1.4.0",
		"sources: import, type_reference",
		"reason: imports user/v1/user.proto",
		"Upstream dependencies:",
		"- common-api@v1.0.0",
		"reason: imports common/v1/common.proto",
		"Unresolved dependencies:",
		"- google/type/date.proto",
		"reason: provider_not_found",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("output missing %q:\n%s", want, output)
		}
	}
}

func TestModuleDependenciesHandlesEmptyGraph(t *testing.T) {
	server := dependencyCommandServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/modules/user-api/dependencies" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		writeJSON(t, w, http.StatusOK, map[string]any{
			"module":     "user-api",
			"upstream":   []map[string]any{},
			"downstream": []map[string]any{},
			"unresolved": []map[string]any{},
		})
	})
	defer server.Close()

	output, err := runCLIAgainstServer(t, server, []string{"module", "dependencies", "user-api"})
	if err != nil {
		t.Fatalf("module dependencies: %v", err)
	}
	if !strings.Contains(output, "No downstream modules are currently known to depend on this module.") {
		t.Fatalf("output missing downstream empty message: %q", output)
	}
	if !strings.Contains(output, "No upstream dependencies are currently known for this module.") {
		t.Fatalf("output missing upstream empty message: %q", output)
	}
}

func TestModuleAffectedRequiresModuleArgument(t *testing.T) {
	err := App{}.Run(context.Background(), []string{"module", "affected"})
	if err == nil || !strings.Contains(err.Error(), "module name") {
		t.Fatalf("error = %v", err)
	}
}

func TestModuleAffectedCallsExpectedEndpointAndPrintsAffectedModules(t *testing.T) {
	var gotPath string
	server := dependencyCommandServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/modules/user-api/affected" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		writeJSON(t, w, http.StatusOK, affectedModulesResponse())
	})
	defer server.Close()

	output, err := runCLIAgainstServer(t, server, []string{"module", "affected", "user-api"})
	if err != nil {
		t.Fatalf("module affected: %v", err)
	}
	if gotPath != "/api/v1/modules/user-api/affected" {
		t.Fatalf("path = %q", gotPath)
	}
	for _, want := range []string{
		"Module: user-api",
		"Affected modules:",
		"- billing-api@v1.4.0",
		"sources: import",
		"reason: import_path",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("output missing %q:\n%s", want, output)
		}
	}
}

func TestModuleAffectedHandlesEmptyAffectedList(t *testing.T) {
	server := dependencyCommandServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/modules/user-api/affected" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		writeJSON(t, w, http.StatusOK, map[string]any{
			"module":           "user-api",
			"affected_modules": []map[string]any{},
		})
	})
	defer server.Close()

	output, err := runCLIAgainstServer(t, server, []string{"module", "affected", "user-api"})
	if err != nil {
		t.Fatalf("module affected: %v", err)
	}
	if !strings.Contains(output, "No downstream modules are currently known to depend on this module.") {
		t.Fatalf("output = %q", output)
	}
}

func TestModuleDependencyAPIErrorMapsToExitCodeTwoAndDoesNotPrintToken(t *testing.T) {
	server := dependencyCommandServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	})
	defer server.Close()

	output, err := runCLIAgainstServer(t, server, []string{"module", "dependencies", "user-api"})
	if exitCode(err) != 2 {
		t.Fatalf("exit code = %d, err = %v", exitCode(err), err)
	}
	if strings.Contains(output, "prr_token") || err != nil && strings.Contains(err.Error(), "prr_token") {
		t.Fatalf("raw token leaked: output=%q err=%v", output, err)
	}
}

func TestModuleDependencyNetworkErrorMapsToExitCodeTwo(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	saveCLIConfig(t, configPath, "http://protoradar.example.test")
	app := App{
		ConfigPath: configPath,
		HTTPClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, errors.New("network unavailable")
		})},
		Out: &bytes.Buffer{},
	}

	err := app.Run(context.Background(), []string{"module", "affected", "user-api"})
	if exitCode(err) != 2 {
		t.Fatalf("exit code = %d, err = %v", exitCode(err), err)
	}
}

func TestGitLabMRCheckAcceptsExplicitFlags(t *testing.T) {
	state := newMRCheckServerState(passedMRReport())
	server := gitLabMRCheckServer(t, state)
	defer server.Close()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	saveCLIConfig(t, configPath, server.URL)
	var output bytes.Buffer
	app := App{ConfigPath: configPath, HTTPClient: server.Client(), Out: &output}

	err := app.Run(context.Background(), []string{
		"gitlab", "mr-check",
		"--module", "user-api",
		"--path", validWorkspace(t),
		"--against", "v1.0.0",
		"--target-ref", "feature/user-api",
		"--gitlab-base-url", server.URL,
		"--project-id", "123",
		"--merge-request-iid", "7",
		"--commit-sha", "abc123",
		"--gitlab-token", "gitlab_secret",
	})
	if err != nil {
		t.Fatalf("gitlab mr-check: %v", err)
	}
	if state.gotAgainst != "v1.0.0" || state.gotTargetRef != "feature/user-api" || state.createdBodies == 0 {
		t.Fatalf("state = %#v", state)
	}
	if !strings.Contains(output.String(), "ProtoRadar MR check passed for user-api") {
		t.Fatalf("output = %q", output.String())
	}
}

func TestGitLabMRCheckUsesEnvDefaults(t *testing.T) {
	state := newMRCheckServerState(passedMRReport())
	server := gitLabMRCheckServer(t, state)
	defer server.Close()

	t.Setenv("PROTORADAR_SERVER_URL", server.URL)
	t.Setenv("PROTORADAR_TOKEN", "prr_token")
	t.Setenv("PROTORADAR_MODULE", "user-api")
	t.Setenv("PROTORADAR_PROTO_PATH", validWorkspace(t))
	t.Setenv("PROTORADAR_AGAINST", "latest")
	t.Setenv("CI_SERVER_URL", server.URL)
	t.Setenv("CI_PROJECT_ID", "123")
	t.Setenv("CI_MERGE_REQUEST_IID", "7")
	t.Setenv("CI_COMMIT_SHA", "abc123")
	t.Setenv("CI_JOB_URL", "https://gitlab.example.com/job/1")
	t.Setenv("PROTORADAR_GITLAB_TOKEN", "gitlab_secret")

	var output bytes.Buffer
	app := App{ConfigPath: filepath.Join(t.TempDir(), "missing.yaml"), HTTPClient: server.Client(), Out: &output}
	err := app.Run(context.Background(), []string{"gitlab", "mr-check"})
	if err != nil {
		t.Fatalf("gitlab mr-check: %v", err)
	}
	if state.gotAgainst != "latest" || state.statuses[1]["target_url"] != "https://gitlab.example.com/job/1" {
		t.Fatalf("state = %#v", state)
	}
}

func TestGitLabMRCheckRequiresInputs(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "module", args: []string{"gitlab", "mr-check", "--gitlab-base-url", "https://gitlab.example.com", "--project-id", "123", "--merge-request-iid", "7", "--commit-sha", "abc", "--gitlab-token", "token"}, want: "--module"},
		{name: "base url", args: []string{"gitlab", "mr-check", "--module", "user-api", "--project-id", "123", "--merge-request-iid", "7", "--commit-sha", "abc", "--gitlab-token", "token"}, want: "--gitlab-base-url"},
		{name: "project id", args: []string{"gitlab", "mr-check", "--module", "user-api", "--gitlab-base-url", "https://gitlab.example.com", "--merge-request-iid", "7", "--commit-sha", "abc", "--gitlab-token", "token"}, want: "--project-id"},
		{name: "mr iid", args: []string{"gitlab", "mr-check", "--module", "user-api", "--gitlab-base-url", "https://gitlab.example.com", "--project-id", "123", "--commit-sha", "abc", "--gitlab-token", "token"}, want: "--merge-request-iid"},
		{name: "commit sha", args: []string{"gitlab", "mr-check", "--module", "user-api", "--gitlab-base-url", "https://gitlab.example.com", "--project-id", "123", "--merge-request-iid", "7", "--gitlab-token", "token"}, want: "--commit-sha"},
		{name: "gitlab token", args: []string{"gitlab", "mr-check", "--module", "user-api", "--gitlab-base-url", "https://gitlab.example.com", "--project-id", "123", "--merge-request-iid", "7", "--commit-sha", "abc"}, want: "--gitlab-token"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := App{}.Run(context.Background(), tt.args)
			if exitCode(err) != 2 || err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, exit=%d", err, exitCode(err))
			}
		})
	}
}

func TestGitLabMRCheckInvalidIDsFailWithExitCodeTwo(t *testing.T) {
	baseArgs := []string{"gitlab", "mr-check", "--module", "user-api", "--gitlab-base-url", "https://gitlab.example.com", "--project-id", "123", "--merge-request-iid", "7", "--commit-sha", "abc", "--gitlab-token", "token"}
	args := slices.Clone(baseArgs)
	args[7] = "bad"
	if err := (App{}).Run(context.Background(), args); exitCode(err) != 2 {
		t.Fatalf("project id err = %v", err)
	}
	args = slices.Clone(baseArgs)
	args[9] = "0"
	if err := (App{}).Run(context.Background(), args); exitCode(err) != 2 {
		t.Fatalf("mr iid err = %v", err)
	}
}

func TestGitLabMRCheckPassedAndBreakingExitCodes(t *testing.T) {
	passedState := newMRCheckServerState(passedMRReport())
	passedServer := gitLabMRCheckServer(t, passedState)
	defer passedServer.Close()
	passedOutput, passedErr := runGitLabMRCheckAgainstServer(t, passedServer, []string{"gitlab", "mr-check", "--module", "user-api", "--path", validWorkspace(t), "--against", "latest", "--gitlab-base-url", passedServer.URL, "--project-id", "123", "--merge-request-iid", "7", "--commit-sha", "abc123", "--gitlab-token", "gitlab_secret"})
	if exitCode(passedErr) != 0 || !strings.Contains(passedOutput, "passed") {
		t.Fatalf("passed output=%q err=%v exit=%d", passedOutput, passedErr, exitCode(passedErr))
	}

	breakingState := newMRCheckServerState(breakingMRReport())
	breakingServer := gitLabMRCheckServer(t, breakingState)
	defer breakingServer.Close()
	breakingOutput, breakingErr := runGitLabMRCheckAgainstServer(t, breakingServer, []string{"gitlab", "mr-check", "--module", "user-api", "--path", validWorkspace(t), "--against", "latest", "--gitlab-base-url", breakingServer.URL, "--project-id", "123", "--merge-request-iid", "7", "--commit-sha", "abc123", "--gitlab-token", "gitlab_secret"})
	if exitCode(breakingErr) != 1 || !strings.Contains(breakingOutput, "breaking changes") {
		t.Fatalf("breaking output=%q err=%v exit=%d", breakingOutput, breakingErr, exitCode(breakingErr))
	}
}

func TestGitLabMRCheckFetchesRuntimeImpact(t *testing.T) {
	state := newMRCheckServerState(breakingMRReport())
	state.runtimeImpactBody = map[string]any{
		"report_id": "report-2",
		"impacts": []map[string]any{{
			"service_name":  "billing-service",
			"environment":   "production",
			"used_module":   "user-api",
			"used_version":  "v1.2.0",
			"git_commit":    "abc1234",
			"build_version": "2026.06.04-15",
			"impact_status": "potentially_affected_by_breaking_change",
			"reason":        "exact version match",
			"reported_at":   "2026-06-04T12:00:00Z",
		}},
	}
	server := gitLabMRCheckServer(t, state)
	defer server.Close()

	_, err := runGitLabMRCheckAgainstServer(t, server, []string{"gitlab", "mr-check", "--module", "user-api", "--path", validWorkspace(t), "--against", "latest", "--gitlab-base-url", server.URL, "--project-id", "123", "--merge-request-iid", "7", "--commit-sha", "abc123", "--gitlab-token", "gitlab_secret"})
	if exitCode(err) != 1 {
		t.Fatalf("err=%v exit=%d", err, exitCode(err))
	}
	if state.runtimeImpactCalls != 1 || state.runtimeImpactReportID != "report-2" {
		t.Fatalf("runtime impact calls=%d reportID=%q", state.runtimeImpactCalls, state.runtimeImpactReportID)
	}
	if len(state.createdNoteBodies) != 1 || !strings.Contains(state.createdNoteBodies[0], "| `billing-service` | `production` | `user-api@v1.2.0` | `2026.06.04-15` | `abc1234` |") {
		t.Fatalf("created note bodies = %#v", state.createdNoteBodies)
	}
}

func TestGitLabMRCheckToolOrGitLabErrorExitsTwo(t *testing.T) {
	toolState := newMRCheckServerState(map[string]string{"error": "buf failed"})
	toolState.breakingStatus = http.StatusInternalServerError
	toolServer := gitLabMRCheckServer(t, toolState)
	defer toolServer.Close()
	_, toolErr := runGitLabMRCheckAgainstServer(t, toolServer, []string{"gitlab", "mr-check", "--module", "user-api", "--path", validWorkspace(t), "--against", "latest", "--gitlab-base-url", toolServer.URL, "--project-id", "123", "--merge-request-iid", "7", "--commit-sha", "abc123", "--gitlab-token", "gitlab_secret"})
	if exitCode(toolErr) != 2 {
		t.Fatalf("tool err=%v exit=%d", toolErr, exitCode(toolErr))
	}

	gitlabState := newMRCheckServerState(passedMRReport())
	gitlabState.createNoteStatus = http.StatusInternalServerError
	gitlabServer := gitLabMRCheckServer(t, gitlabState)
	defer gitlabServer.Close()
	_, gitlabErr := runGitLabMRCheckAgainstServer(t, gitlabServer, []string{"gitlab", "mr-check", "--module", "user-api", "--path", validWorkspace(t), "--against", "latest", "--gitlab-base-url", gitlabServer.URL, "--project-id", "123", "--merge-request-iid", "7", "--commit-sha", "abc123", "--gitlab-token", "gitlab_secret"})
	if exitCode(gitlabErr) != 2 {
		t.Fatalf("gitlab err=%v exit=%d", gitlabErr, exitCode(gitlabErr))
	}
}

func TestGitLabMRCheckStatusFlags(t *testing.T) {
	state := newMRCheckServerState(passedMRReport())
	server := gitLabMRCheckServer(t, state)
	defer server.Close()
	_, err := runGitLabMRCheckAgainstServer(t, server, []string{"gitlab", "mr-check", "--module", "user-api", "--path", validWorkspace(t), "--against", "latest", "--gitlab-base-url", server.URL, "--project-id", "123", "--merge-request-iid", "7", "--commit-sha", "abc123", "--gitlab-token", "gitlab_secret", "--status=false"})
	if err != nil || len(state.statuses) != 0 {
		t.Fatalf("err=%v statuses=%#v", err, state.statuses)
	}

	state = newMRCheckServerState(passedMRReport())
	server = gitLabMRCheckServer(t, state)
	defer server.Close()
	_, err = runGitLabMRCheckAgainstServer(t, server, []string{"gitlab", "mr-check", "--module", "user-api", "--path", validWorkspace(t), "--against", "latest", "--gitlab-base-url", server.URL, "--project-id", "123", "--merge-request-iid", "7", "--commit-sha", "abc123", "--gitlab-token", "gitlab_secret", "--status-name", "custom/status", "--status-target-url", "https://gitlab.example.com/job/2"})
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if state.statuses[0]["name"] != "custom/status" || state.statuses[0]["target_url"] != "https://gitlab.example.com/job/2" {
		t.Fatalf("statuses=%#v", state.statuses)
	}
}

func TestGitLabMRCheckReportFileAndWriteFailure(t *testing.T) {
	state := newMRCheckServerState(passedMRReport())
	server := gitLabMRCheckServer(t, state)
	defer server.Close()
	reportPath := filepath.Join(t.TempDir(), "mr-report.md")
	_, err := runGitLabMRCheckAgainstServer(t, server, []string{"gitlab", "mr-check", "--module", "user-api", "--path", validWorkspace(t), "--against", "latest", "--gitlab-base-url", server.URL, "--project-id", "123", "--merge-request-iid", "7", "--commit-sha", "abc123", "--gitlab-token", "gitlab_secret", "--report-file", reportPath})
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	body, err := os.ReadFile(reportPath)
	if err != nil || !strings.Contains(string(body), "ProtoRadar Breaking Change Report") {
		t.Fatalf("body=%q err=%v", string(body), err)
	}

	state = newMRCheckServerState(passedMRReport())
	server = gitLabMRCheckServer(t, state)
	defer server.Close()
	_, err = runGitLabMRCheckAgainstServer(t, server, []string{"gitlab", "mr-check", "--module", "user-api", "--path", validWorkspace(t), "--against", "latest", "--gitlab-base-url", server.URL, "--project-id", "123", "--merge-request-iid", "7", "--commit-sha", "abc123", "--gitlab-token", "gitlab_secret", "--report-file", t.TempDir()})
	if exitCode(err) != 2 {
		t.Fatalf("err=%v exit=%d", err, exitCode(err))
	}
}

func TestGitLabMRCheckDoesNotPrintRawTokens(t *testing.T) {
	state := newMRCheckServerState(passedMRReport())
	server := gitLabMRCheckServer(t, state)
	defer server.Close()
	output, err := runGitLabMRCheckAgainstServer(t, server, []string{"gitlab", "mr-check", "--module", "user-api", "--path", validWorkspace(t), "--against", "latest", "--gitlab-base-url", server.URL, "--project-id", "123", "--merge-request-iid", "7", "--commit-sha", "abc123", "--gitlab-token", "gitlab_secret"})
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if strings.Contains(output, "gitlab_secret") || strings.Contains(output, "prr_token") {
		t.Fatalf("output leaked token: %q", output)
	}
}

type mrCheckServerState struct {
	breakingStatus        int
	breakingBody          any
	runtimeImpactStatus   int
	runtimeImpactBody     any
	runtimeImpactCalls    int
	runtimeImpactReportID string
	createNoteStatus      int
	notes                 []map[string]any
	gotAgainst            string
	gotTargetRef          string
	createdBodies         int
	createdNoteBodies     []string
	updatedBodies         int
	statuses              []map[string]string
}

func newMRCheckServerState(body any) *mrCheckServerState {
	return &mrCheckServerState{
		breakingStatus:      http.StatusOK,
		breakingBody:        body,
		runtimeImpactStatus: http.StatusOK,
		runtimeImpactBody:   map[string]any{"report_id": "report-1", "impacts": []map[string]any{}},
		createNoteStatus:    http.StatusCreated,
		notes:               []map[string]any{},
	}
}

func passedMRReport() map[string]any {
	return map[string]any{
		"id":           "report-1",
		"module":       "user-api",
		"against":      "v1.0.0",
		"target_ref":   "abc123",
		"status":       "passed",
		"change_count": 0,
		"changes":      []map[string]any{},
		"created_at":   "2026-06-04T12:00:00Z",
	}
}

func breakingMRReport() map[string]any {
	return map[string]any{
		"id":           "report-2",
		"module":       "user-api",
		"against":      "v1.0.0",
		"target_ref":   "abc123",
		"status":       "breaking",
		"change_count": 1,
		"changes": []map[string]any{{
			"file_path": "user/v1/user.proto",
			"symbol":    "user.v1.User.email",
			"rule_id":   "FIELD_SAME_TYPE",
			"message":   "field changed type",
		}},
		"created_at": "2026-06-04T12:00:00Z",
	}
}

func gitLabMRCheckServer(t *testing.T, state *mrCheckServerState) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/v1/"):
			handleProtoRadarMRCheckRequest(t, state, w, r)
		case strings.HasPrefix(r.URL.Path, "/api/v4/"):
			handleGitLabMRCheckRequest(t, state, w, r)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func handleProtoRadarMRCheckRequest(t *testing.T, state *mrCheckServerState, w http.ResponseWriter, r *http.Request) {
	t.Helper()
	if r.Header.Get("Authorization") != "Bearer prr_token" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/v1/breaking-reports/") && strings.HasSuffix(r.URL.Path, "/runtime-impact") {
		parts := strings.Split(r.URL.Path, "/")
		if len(parts) >= 5 {
			state.runtimeImpactReportID = parts[4]
		}
		state.runtimeImpactCalls++
		writeJSON(t, w, state.runtimeImpactStatus, state.runtimeImpactBody)
		return
	}
	if r.Method != http.MethodPost || r.URL.Path != "/api/v1/modules/user-api/breaking-checks" {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	state.gotAgainst = r.FormValue("against")
	state.gotTargetRef = r.FormValue("target_ref")
	if _, _, err := r.FormFile("artifact"); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	writeJSON(t, w, state.breakingStatus, state.breakingBody)
}

func handleGitLabMRCheckRequest(t *testing.T, state *mrCheckServerState, w http.ResponseWriter, r *http.Request) {
	t.Helper()
	if r.Header.Get("PRIVATE-TOKEN") != "gitlab_secret" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/api/v4/projects/123/merge_requests/7/notes":
		writeJSON(t, w, http.StatusOK, state.notes)
	case r.Method == http.MethodPost && r.URL.Path == "/api/v4/projects/123/merge_requests/7/notes":
		if state.createNoteStatus < 200 || state.createNoteStatus >= 300 {
			writeJSON(t, w, state.createNoteStatus, map[string]string{"error": "note failed"})
			return
		}
		var req map[string]string
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req["body"] == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		state.createdBodies++
		state.createdNoteBodies = append(state.createdNoteBodies, req["body"])
		writeJSON(t, w, http.StatusCreated, map[string]any{"id": state.createdBodies, "body": req["body"]})
	case r.Method == http.MethodPut && r.URL.Path == "/api/v4/projects/123/merge_requests/7/notes/44":
		var req map[string]string
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req["body"] == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		state.updatedBodies++
		writeJSON(t, w, http.StatusOK, map[string]any{"id": 44, "body": req["body"]})
	case r.Method == http.MethodPost && r.URL.Path == "/api/v4/projects/123/statuses/abc123":
		if err := r.ParseForm(); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		state.statuses = append(state.statuses, map[string]string{
			"state":       r.Form.Get("state"),
			"name":        r.Form.Get("name"),
			"target_url":  r.Form.Get("target_url"),
			"description": r.Form.Get("description"),
		})
		writeJSON(t, w, http.StatusCreated, map[string]string{"status": "ok"})
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func runGitLabMRCheckAgainstServer(t *testing.T, server *httptest.Server, args []string) (string, error) {
	t.Helper()
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	saveCLIConfig(t, configPath, server.URL)
	var output bytes.Buffer
	app := App{ConfigPath: configPath, HTTPClient: server.Client(), Out: &output}
	err := app.Run(context.Background(), args)
	return output.String(), err
}

func runCLIAgainstServer(t *testing.T, server *httptest.Server, args []string) (string, error) {
	t.Helper()
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	saveCLIConfig(t, configPath, server.URL)
	var output bytes.Buffer
	app := App{ConfigPath: configPath, HTTPClient: server.Client(), Out: &output}
	err := app.Run(context.Background(), args)
	return output.String(), err
}

func dependencyCommandServer(t *testing.T, handle func(http.ResponseWriter, *http.Request)) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer prr_token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		handle(w, r)
	}))
}

func dependencyGraphResponse() map[string]any {
	return map[string]any{
		"module": "user-api",
		"downstream": []map[string]any{{
			"module":             "billing-api",
			"latest_version":     "v1.4.0",
			"dependency_sources": []string{"import", "type_reference"},
			"reasons":            []string{"imports user/v1/user.proto"},
		}},
		"upstream": []map[string]any{{
			"module":             "common-api",
			"latest_version":     "v1.0.0",
			"dependency_sources": []string{"import"},
			"reasons":            []string{"imports common/v1/common.proto"},
		}},
		"unresolved": []map[string]any{{
			"module":      "user-api",
			"version":     "v1.0.0",
			"source":      "import",
			"import_path": "google/type/date.proto",
			"reason":      "provider_not_found",
		}},
	}
}

func affectedModulesResponse() map[string]any {
	return map[string]any{
		"module": "user-api",
		"affected_modules": []map[string]any{{
			"module":             "billing-api",
			"latest_version":     "v1.4.0",
			"dependency_sources": []string{"import"},
			"reasons":            []string{"import_path"},
		}},
	}
}

func saveCLIConfig(t *testing.T, path string, serverURL string) {
	t.Helper()
	saveCLIConfigWithToken(t, path, serverURL, "prr_token")
}

func saveCLIConfigWithToken(t *testing.T, path string, serverURL string, token string) {
	t.Helper()
	if err := config.Save(path, config.Config{ServerURL: serverURL, Token: token}); err != nil {
		t.Fatalf("save config: %v", err)
	}
}

func runtimeReportServer(t *testing.T, handler func(http.ResponseWriter, *http.Request)) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer prr_token" && r.Header.Get("Authorization") != "Bearer prr_super_secret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/runtime/reports" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		handler(w, r)
	}))
}

func runRuntimeReportAgainstServer(t *testing.T, server *httptest.Server, args []string) (string, error) {
	t.Helper()
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	saveCLIConfig(t, configPath, server.URL)
	var output bytes.Buffer
	app := App{ConfigPath: configPath, HTTPClient: server.Client(), Out: &output}
	err := app.Run(context.Background(), args)
	return output.String(), err
}

func writeRuntimeReportResponse(t *testing.T, w http.ResponseWriter, request map[string]any, usages []map[string]any) {
	t.Helper()
	writeJSON(t, w, http.StatusCreated, map[string]any{
		"deployment_id": "deployment-1",
		"service_name":  request["service_name"],
		"environment":   request["environment"],
		"git_commit":    request["git_commit"],
		"build_version": request["build_version"],
		"reported_at":   "2026-06-04T12:00:00Z",
		"usages":        usages,
	})
}

func TestRuntimeReportRequiresService(t *testing.T) {
	app := App{ConfigPath: filepath.Join(t.TempDir(), "missing.yaml")}
	err := app.Run(context.Background(), []string{"runtime", "report", "--environment", "production", "--git-commit", "abc123", "--build-version", "build-1", "--module", "user-api@v1.0.0"})
	if exitCode(err) != 2 || err == nil || !strings.Contains(err.Error(), "--service") {
		t.Fatalf("error = %v", err)
	}
}

func TestRuntimeReportRequiresEnvironment(t *testing.T) {
	app := App{ConfigPath: filepath.Join(t.TempDir(), "missing.yaml")}
	err := app.Run(context.Background(), []string{"runtime", "report", "--service", "billing-service", "--git-commit", "abc123", "--build-version", "build-1", "--module", "user-api@v1.0.0"})
	if exitCode(err) != 2 || err == nil || !strings.Contains(err.Error(), "--environment") {
		t.Fatalf("error = %v", err)
	}
}

func TestRuntimeReportRequiresModule(t *testing.T) {
	app := App{ConfigPath: filepath.Join(t.TempDir(), "missing.yaml")}
	err := app.Run(context.Background(), []string{"runtime", "report", "--service", "billing-service", "--environment", "production", "--git-commit", "abc123", "--build-version", "build-1"})
	if exitCode(err) != 2 || err == nil || !strings.Contains(err.Error(), "module") {
		t.Fatalf("error = %v", err)
	}
}

func TestRuntimeReportInvalidModuleFormatExitsTwo(t *testing.T) {
	app := App{ConfigPath: filepath.Join(t.TempDir(), "missing.yaml")}
	err := app.Run(context.Background(), []string{"runtime", "report", "--service", "billing-service", "--environment", "production", "--git-commit", "abc123", "--build-version", "build-1", "--module", "user-api"})
	if exitCode(err) != 2 || err == nil || !strings.Contains(err.Error(), "module@version") {
		t.Fatalf("error = %v", err)
	}
}

func TestRuntimeReportCallsExpectedAPIEndpointAndPayload(t *testing.T) {
	var gotPath string
	var gotAuth string
	var payload map[string]any
	server := runtimeReportServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		writeRuntimeReportResponse(t, w, payload, []map[string]any{
			{"module": "user-api", "version": "v1.2.0", "latest_version": "v1.5.0", "drift_status": "behind_latest", "drift_reason": "behind_latest: latest version is v1.5.0"},
			{"module": "billing-api", "version": "v1.4.0", "drift_status": "up_to_date", "drift_reason": "up_to_date"},
		})
	})
	defer server.Close()

	output, err := runRuntimeReportAgainstServer(t, server, []string{
		"runtime", "report",
		"--service", "billing-service",
		"--environment", "production",
		"--git-commit", "abc1234",
		"--build-version", "2026.06.04-15",
		"--module", "user-api@v1.2.0",
		"--module", "billing-api@v1.4.0",
	})
	if err != nil {
		t.Fatalf("runtime report: %v", err)
	}
	if gotPath != "/api/v1/runtime/reports" || gotAuth != "Bearer prr_token" {
		t.Fatalf("path/auth = %s/%s", gotPath, gotAuth)
	}
	if payload["service_name"] != "billing-service" || payload["environment"] != "production" || payload["git_commit"] != "abc1234" || payload["build_version"] != "2026.06.04-15" {
		t.Fatalf("payload = %#v", payload)
	}
	modules, ok := payload["modules"].([]any)
	if !ok || len(modules) != 2 {
		t.Fatalf("modules = %#v", payload["modules"])
	}
	if !strings.Contains(output, "Runtime inventory reported") || !strings.Contains(output, "user-api@v1.2.0 - behind latest (latest: v1.5.0)") || !strings.Contains(output, "billing-api@v1.4.0 - up to date") {
		t.Fatalf("output = %q", output)
	}
}

func TestRuntimeReportUsesGitLabEnvDefaults(t *testing.T) {
	var payload map[string]any
	server := runtimeReportServer(t, func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		writeRuntimeReportResponse(t, w, payload, []map[string]any{{"module": "user-api", "version": "v1.0.0", "drift_status": "up_to_date", "drift_reason": "up_to_date"}})
	})
	defer server.Close()
	t.Setenv("PROTORADAR_SERVER_URL", server.URL)
	t.Setenv("PROTORADAR_TOKEN", "prr_token")
	t.Setenv("CI_PROJECT_NAME", "billing-service")
	t.Setenv("CI_ENVIRONMENT_NAME", "production")
	t.Setenv("CI_COMMIT_SHA", "abc1234")
	t.Setenv("CI_COMMIT_SHORT_SHA", "abc1234")

	var output bytes.Buffer
	app := App{ConfigPath: filepath.Join(t.TempDir(), "missing.yaml"), HTTPClient: server.Client(), Out: &output}
	err := app.Run(context.Background(), []string{"runtime", "report", "--module", "user-api@v1.0.0"})
	if err != nil {
		t.Fatalf("runtime report: %v", err)
	}
	if payload["service_name"] != "billing-service" || payload["environment"] != "production" || payload["git_commit"] != "abc1234" || payload["build_version"] != "abc1234" {
		t.Fatalf("payload = %#v", payload)
	}
}

func TestRuntimeReportReadsFromFile(t *testing.T) {
	var payload map[string]any
	server := runtimeReportServer(t, func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		writeRuntimeReportResponse(t, w, payload, []map[string]any{{"module": "user-api", "version": "v1.2.0", "drift_status": "up_to_date", "drift_reason": "up_to_date"}})
	})
	defer server.Close()

	file := filepath.Join(t.TempDir(), "protoradar-runtime.yaml")
	writeFile(t, file, "service_name: billing-service\nenvironment: production\ngit_commit: abc1234\nbuild_version: build-1\nmodules:\n  - module: user-api\n    version: v1.2.0\n")
	_, err := runRuntimeReportAgainstServer(t, server, []string{"runtime", "report", "--from-file", file})
	if err != nil {
		t.Fatalf("runtime report: %v", err)
	}
	if payload["service_name"] != "billing-service" || payload["environment"] != "production" {
		t.Fatalf("payload = %#v", payload)
	}
}

func TestRuntimeReportFlagsOverrideFileAndModulesReplaceFileModules(t *testing.T) {
	var payload map[string]any
	server := runtimeReportServer(t, func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		writeRuntimeReportResponse(t, w, payload, []map[string]any{{"module": "billing-api", "version": "v1.4.0", "drift_status": "up_to_date", "drift_reason": "up_to_date"}})
	})
	defer server.Close()

	file := filepath.Join(t.TempDir(), "protoradar-runtime.yaml")
	writeFile(t, file, "service_name: file-service\nenvironment: file-env\ngit_commit: file-commit\nbuild_version: file-build\nmodules:\n  - module: user-api\n    version: v1.2.0\n")
	_, err := runRuntimeReportAgainstServer(t, server, []string{"runtime", "report", "--from-file", file, "--service", "billing-service", "--environment", "production", "--git-commit", "abc1234", "--build-version", "build-1", "--module", "billing-api@v1.4.0"})
	if err != nil {
		t.Fatalf("runtime report: %v", err)
	}
	if payload["service_name"] != "billing-service" || payload["environment"] != "production" || payload["git_commit"] != "abc1234" || payload["build_version"] != "build-1" {
		t.Fatalf("payload = %#v", payload)
	}
	modules := payload["modules"].([]any)
	if len(modules) != 1 || modules[0].(map[string]any)["module"] != "billing-api" {
		t.Fatalf("modules = %#v", modules)
	}
}

func TestRuntimeReportBehindLatestAndUnknownVersionExitZero(t *testing.T) {
	server := runtimeReportServer(t, func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		writeRuntimeReportResponse(t, w, payload, []map[string]any{
			{"module": "user-api", "version": "v1.2.0", "latest_version": "v1.5.0", "drift_status": "behind_latest", "drift_reason": "behind_latest: latest version is v1.5.0"},
			{"module": "missing-api", "version": "v9.9.9", "drift_status": "unknown_version", "drift_reason": "module_not_found"},
		})
	})
	defer server.Close()

	output, err := runRuntimeReportAgainstServer(t, server, []string{"runtime", "report", "--service", "billing-service", "--environment", "production", "--git-commit", "abc1234", "--build-version", "build-1", "--module", "user-api@v1.2.0", "--module", "missing-api@v9.9.9"})
	if exitCode(err) != 0 {
		t.Fatalf("error = %v", err)
	}
	if !strings.Contains(output, "behind latest") || !strings.Contains(output, "unknown version") {
		t.Fatalf("output = %q", output)
	}
}

func TestRuntimeReportAuthNetworkAndServerErrorsExitTwo(t *testing.T) {
	for _, tt := range []struct {
		name   string
		status int
	}{
		{name: "auth", status: http.StatusUnauthorized},
		{name: "server", status: http.StatusInternalServerError},
	} {
		t.Run(tt.name, func(t *testing.T) {
			server := runtimeReportServer(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
			})
			defer server.Close()
			_, err := runRuntimeReportAgainstServer(t, server, []string{"runtime", "report", "--service", "billing-service", "--environment", "production", "--git-commit", "abc1234", "--build-version", "build-1", "--module", "user-api@v1.0.0"})
			if exitCode(err) != 2 {
				t.Fatalf("error = %v", err)
			}
		})
	}

	app := App{ConfigPath: filepath.Join(t.TempDir(), "missing.yaml")}
	err := app.Run(context.Background(), []string{"runtime", "report", "--service", "billing-service", "--environment", "production", "--git-commit", "abc1234", "--build-version", "build-1", "--module", "user-api@v1.0.0"})
	if exitCode(err) != 2 {
		t.Fatalf("network/config error = %v", err)
	}
}

func TestRuntimeReportDoesNotPrintRawToken(t *testing.T) {
	server := runtimeReportServer(t, func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		writeRuntimeReportResponse(t, w, payload, []map[string]any{{"module": "user-api", "version": "v1.0.0", "drift_status": "up_to_date", "drift_reason": "up_to_date"}})
	})
	defer server.Close()

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	saveCLIConfigWithToken(t, configPath, server.URL, "prr_super_secret")
	var output bytes.Buffer
	app := App{ConfigPath: configPath, HTTPClient: server.Client(), Out: &output}
	err := app.Run(context.Background(), []string{"runtime", "report", "--service", "billing-service", "--environment", "production", "--git-commit", "abc1234", "--build-version", "build-1", "--module", "user-api@v1.0.0"})
	if err != nil {
		t.Fatalf("runtime report: %v", err)
	}
	if strings.Contains(output.String(), "prr_super_secret") {
		t.Fatalf("raw token printed: %q", output.String())
	}
}

func validWorkspace(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "buf.yaml"), "version: v2\n")
	writeFile(t, filepath.Join(root, "user.proto"), "syntax = \"proto3\";")
	return root
}

func breakingServer(t *testing.T, status int, body any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer prr_token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/modules/user-api/breaking-checks" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if r.FormValue("against") == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if _, _, err := r.FormFile("artifact"); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		writeJSON(t, w, status, body)
	}))
}

func moduleLinkGitLabServer(t *testing.T, inspect func(*http.Request), status int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer prr_token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.Method != http.MethodPut || r.URL.Path != "/api/v1/modules/user-api/gitlab-project" && r.URL.Path != "/api/v1/modules/billing-api/gitlab-project" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if inspect != nil {
			inspect(r)
		}
		if status < 200 || status >= 300 {
			writeJSON(t, w, status, map[string]string{"error": "linked elsewhere"})
			return
		}
		module := strings.TrimPrefix(r.URL.Path, "/api/v1/modules/")
		module = strings.TrimSuffix(module, "/gitlab-project")
		writeJSON(t, w, status, map[string]any{
			"module":              module,
			"gitlab_base_url":     "https://gitlab.example.com",
			"gitlab_project_id":   projectIDFromModule(module),
			"gitlab_project_path": projectPathFromModule(module),
			"created_at":          "2026-06-04T12:00:00Z",
			"updated_at":          "2026-06-04T12:00:00Z",
		})
	}))
}

func projectIDFromModule(module string) int64 {
	if module == "billing-api" {
		return 67890
	}
	return 12345
}

func projectPathFromModule(module string) string {
	if module == "billing-api" {
		return "platform/billing-api"
	}
	return "platform/user-api"
}

func runCheckBreakingAgainstServer(t *testing.T, server *httptest.Server, args []string) (string, error) {
	t.Helper()
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	saveCLIConfig(t, configPath, server.URL)
	var output bytes.Buffer
	app := App{ConfigPath: configPath, HTTPClient: server.Client(), Out: &output}
	err := app.Run(context.Background(), args)
	return output.String(), err
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	var exitErr ExitError
	if errors.As(err, &exitErr) {
		return exitErr.Code
	}
	return 2
}

func writeJSON(t *testing.T, w http.ResponseWriter, status int, body any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		t.Fatalf("write json: %v", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}
