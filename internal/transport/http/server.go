package httptransport

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/alryzden/ProtoRadar/internal/domain"
	"github.com/alryzden/ProtoRadar/internal/storage"
	"github.com/alryzden/ProtoRadar/internal/usecase/registry"
	"github.com/alryzden/ProtoRadar/internal/usecase/runtimeinventory"
)

type Registry interface {
	CreateModule(ctx context.Context, req registry.CreateModuleRequest) (domain.Module, error)
	ListModules(ctx context.Context, limit int, offset int) ([]domain.Module, error)
	GetModule(ctx context.Context, name string) (domain.Module, error)
	LinkModuleGitLabProject(ctx context.Context, input registry.LinkModuleGitLabProjectInput) (registry.LinkModuleGitLabProjectOutput, error)
	GetModuleGitLabProject(ctx context.Context, moduleName string) (registry.GetModuleGitLabProjectOutput, error)
	PublishModuleVersion(ctx context.Context, req registry.PublishModuleVersionRequest) (registry.PublishModuleVersionResponse, error)
	ListModuleVersions(ctx context.Context, moduleName string, limit int, offset int) ([]domain.ModuleVersion, error)
	GetModuleVersion(ctx context.Context, moduleName string, version string) (domain.ModuleVersion, error)
	GetModuleVersionDetails(ctx context.Context, moduleName string, version string) (registry.ModuleVersionDetailsResponse, error)
	GetModuleVersionMetadata(ctx context.Context, moduleName string, version string) (domain.DescriptorMetadata, error)
	CheckBreaking(ctx context.Context, req registry.CheckBreakingRequest) (registry.CheckBreakingResponse, error)
	GetBreakingReport(ctx context.Context, reportID string) (registry.CheckBreakingResponse, error)
	ListBreakingReports(ctx context.Context, moduleName string, limit int, offset int) ([]domain.BreakingReport, error)
	GetModuleDependencyGraph(ctx context.Context, moduleName string) (registry.ModuleDependencyGraphResponse, error)
	ListAffectedModules(ctx context.Context, moduleName string) (registry.AffectedModulesResponse, error)
	GetBreakingReportAffectedModules(ctx context.Context, reportID string) (registry.BreakingReportAffectedModulesResponse, error)
	DownloadArtifact(ctx context.Context, moduleName string, version string) (storage.ArtifactObject, domain.Artifact, error)
	CreateAPIToken(ctx context.Context, req registry.CreateAPITokenRequest) (registry.CreateAPITokenResponse, error)
	AuthenticateToken(ctx context.Context, rawToken string) (registry.AuthSubject, error)
}

type RuntimeInventory interface {
	ReportRuntimeInventory(ctx context.Context, input runtimeinventory.ReportRuntimeInventoryInput) (runtimeinventory.ReportRuntimeInventoryOutput, error)
	ListRuntimeServices(ctx context.Context, limit int, offset int) ([]domain.RuntimeServiceSummary, error)
	GetRuntimeServiceDetails(ctx context.Context, serviceName string) (domain.RuntimeServiceDetails, error)
	GetEnvironmentInventory(ctx context.Context, environment string, limit int, offset int) (domain.RuntimeEnvironmentInventory, error)
	GetModuleRuntimeUsages(ctx context.Context, moduleName string, limit int, offset int) ([]domain.ModuleRuntimeUsage, error)
	GetBreakingReportRuntimeImpact(ctx context.Context, reportID domain.BreakingReportID, limit int, offset int) ([]domain.RuntimeImpact, error)
}

type Server struct {
	registry            Registry
	runtime             RuntimeInventory
	bootstrapToken      string
	readinessChecks     []ReadinessCheck
	logger              *slog.Logger
	metrics             *Metrics
	maxRequestBodyBytes int64
}

type Options struct {
	BootstrapToken      string
	Runtime             RuntimeInventory
	Ready               func(context.Context) error
	ReadinessChecks     []ReadinessCheck
	Logger              *slog.Logger
	Metrics             *Metrics
	MaxRequestBodyBytes int64
}

type ReadinessCheck struct {
	Name  string
	Check func(context.Context) error
}

func NewServer(registry Registry, options Options) *Server {
	readiness := append([]ReadinessCheck{}, options.ReadinessChecks...)
	if options.Ready != nil {
		readiness = append(readiness, ReadinessCheck{Name: "database", Check: options.Ready})
	}
	return &Server{
		registry:            registry,
		runtime:             options.Runtime,
		bootstrapToken:      options.BootstrapToken,
		readinessChecks:     readiness,
		logger:              options.Logger,
		metrics:             options.Metrics,
		maxRequestBodyBytes: options.MaxRequestBodyBytes,
	}
}

func (server *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", server.health)
	mux.HandleFunc("GET /readyz", server.readiness)
	mux.Handle("POST /api/v1/modules", server.requireBearer(http.HandlerFunc(server.createModule)))
	mux.Handle("GET /api/v1/modules", server.requireBearer(http.HandlerFunc(server.listModules)))
	mux.Handle("GET /api/v1/modules/{module}", server.requireBearer(http.HandlerFunc(server.getModule)))
	mux.Handle("PUT /api/v1/modules/{module}/gitlab-project", server.requireBearer(http.HandlerFunc(server.linkModuleGitLabProject)))
	mux.Handle("GET /api/v1/modules/{module}/gitlab-project", server.requireBearer(http.HandlerFunc(server.getModuleGitLabProject)))
	mux.Handle("GET /api/v1/modules/{module}/dependencies", server.requireBearer(http.HandlerFunc(server.getModuleDependencies)))
	mux.Handle("GET /api/v1/modules/{module}/affected", server.requireBearer(http.HandlerFunc(server.getAffectedModules)))
	mux.Handle("GET /api/v1/modules/{module}/runtime-usages", server.requireBearer(http.HandlerFunc(server.getModuleRuntimeUsages)))
	mux.Handle("POST /api/v1/modules/{module}/versions", server.requireBearer(http.HandlerFunc(server.publishModuleVersion)))
	mux.Handle("GET /api/v1/modules/{module}/versions", server.requireBearer(http.HandlerFunc(server.listModuleVersions)))
	mux.Handle("GET /api/v1/modules/{module}/versions/{version}", server.requireBearer(http.HandlerFunc(server.getModuleVersion)))
	mux.Handle("GET /api/v1/modules/{module}/versions/{version}/metadata", server.requireBearer(http.HandlerFunc(server.getModuleVersionMetadata)))
	mux.Handle("GET /api/v1/modules/{module}/versions/{version}/artifact", server.requireBearer(http.HandlerFunc(server.downloadArtifact)))
	mux.Handle("POST /api/v1/modules/{module}/breaking-checks", server.requireBearer(http.HandlerFunc(server.createBreakingCheck)))
	mux.Handle("GET /api/v1/modules/{module}/breaking-reports", server.requireBearer(http.HandlerFunc(server.listBreakingReports)))
	mux.Handle("GET /api/v1/breaking-reports/{report_id}", server.requireBearer(http.HandlerFunc(server.getBreakingReport)))
	mux.Handle("GET /api/v1/breaking-reports/{report_id}/affected-modules", server.requireBearer(http.HandlerFunc(server.getBreakingReportAffectedModules)))
	mux.Handle("GET /api/v1/breaking-reports/{report_id}/runtime-impact", server.requireBearer(http.HandlerFunc(server.getBreakingReportRuntimeImpact)))
	mux.Handle("POST /api/v1/runtime/reports", server.requireBearer(http.HandlerFunc(server.reportRuntimeInventory)))
	mux.Handle("GET /api/v1/runtime/services", server.requireBearer(http.HandlerFunc(server.listRuntimeServices)))
	mux.Handle("GET /api/v1/runtime/services/{service}", server.requireBearer(http.HandlerFunc(server.getRuntimeServiceDetails)))
	mux.Handle("GET /api/v1/runtime/environments/{environment}", server.requireBearer(http.HandlerFunc(server.getEnvironmentInventory)))
	mux.Handle("POST /api/v1/tokens", server.requireBootstrapToken(http.HandlerFunc(server.createAPIToken)))
	mux.HandleFunc("GET /metrics", server.metricsHandler)

	return server.withRequestID(server.withRequestLogging(server.withHTTPMetrics(mux)))
}

func (server *Server) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (server *Server) readiness(w http.ResponseWriter, r *http.Request) {
	checks := make(map[string]string, len(server.readinessChecks))
	ready := true
	for _, check := range server.readinessChecks {
		name := strings.TrimSpace(check.Name)
		if name == "" || check.Check == nil {
			continue
		}
		checks[name] = "ok"
		if err := check.Check(r.Context()); err != nil {
			checks[name] = "error"
			ready = false
		}
	}
	status := http.StatusOK
	bodyStatus := "ok"
	if !ready {
		status = http.StatusServiceUnavailable
		bodyStatus = "error"
	}
	writeJSON(w, status, readinessResponse{Status: bodyStatus, Checks: checks})
}

func (server *Server) metricsHandler(w http.ResponseWriter, r *http.Request) {
	if server.metrics == nil {
		w.Header().Set("Content-Type", prometheusContentType)
		w.WriteHeader(http.StatusOK)
		return
	}
	server.metrics.WritePrometheus(w)
}

func (server *Server) requireBearer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, ok := bearerToken(r.Header.Get("Authorization"))
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication is required.")
			return
		}
		if _, err := server.registry.AuthenticateToken(r.Context(), token); err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication is required.")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (server *Server) requireBootstrapToken(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, ok := bearerToken(r.Header.Get("Authorization"))
		if !ok || !sameToken(token, server.bootstrapToken) {
			writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication is required.")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func sameToken(got string, want string) bool {
	if got == "" || want == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}

func bearerToken(header string) (string, bool) {
	prefix := "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return "", false
	}
	token := strings.TrimSpace(strings.TrimPrefix(header, prefix))
	return token, token != ""
}

func (server *Server) createModule(w http.ResponseWriter, r *http.Request) {
	var req createModuleRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Request body is invalid.")
		return
	}

	module, err := server.registry.CreateModule(r.Context(), registry.CreateModuleRequest{
		Name:          req.Name,
		Description:   req.Description,
		RepositoryURL: req.RepositoryURL,
	})
	if err != nil {
		writeUsecaseError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, moduleResponse(module))
}

func (server *Server) listModules(w http.ResponseWriter, r *http.Request) {
	modules, err := server.registry.ListModules(r.Context(), 100, 0)
	if err != nil {
		writeUsecaseError(w, err)
		return
	}

	items := make([]moduleDTO, 0, len(modules))
	for _, module := range modules {
		items = append(items, moduleResponse(module))
	}
	writeJSON(w, http.StatusOK, listModulesResponse{Modules: items})
}

func (server *Server) getModule(w http.ResponseWriter, r *http.Request) {
	module, err := server.registry.GetModule(r.Context(), r.PathValue("module"))
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, moduleResponse(module))
}

func (server *Server) linkModuleGitLabProject(w http.ResponseWriter, r *http.Request) {
	var req linkModuleGitLabProjectRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Request body is invalid.")
		return
	}

	response, err := server.registry.LinkModuleGitLabProject(r.Context(), registry.LinkModuleGitLabProjectInput{
		ModuleName:        r.PathValue("module"),
		GitLabBaseURL:     req.GitLabBaseURL,
		GitLabProjectID:   req.GitLabProjectID,
		GitLabProjectPath: req.GitLabProjectPath,
	})
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, moduleGitLabProjectResponse(response.Mapping))
}

func (server *Server) getModuleGitLabProject(w http.ResponseWriter, r *http.Request) {
	response, err := server.registry.GetModuleGitLabProject(r.Context(), r.PathValue("module"))
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, moduleGitLabProjectResponse(response.Mapping))
}

func (server *Server) getModuleDependencies(w http.ResponseWriter, r *http.Request) {
	response, err := server.registry.GetModuleDependencyGraph(r.Context(), r.PathValue("module"))
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	if server.metrics != nil {
		server.metrics.SetDependencyTotals(len(response.Upstream)+len(response.Downstream), len(response.Unresolved))
	}
	writeJSON(w, http.StatusOK, moduleDependencyGraphResponse(response))
}

func (server *Server) getAffectedModules(w http.ResponseWriter, r *http.Request) {
	response, err := server.registry.ListAffectedModules(r.Context(), r.PathValue("module"))
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, affectedModulesResponse(response))
}

func (server *Server) publishModuleVersion(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	server.limitRequestBody(w, r)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		server.recordPublishMetric("error", started)
		if isRequestBodyTooLarge(err) {
			writePayloadTooLarge(w)
			return
		}
		writeError(w, http.StatusBadRequest, "bad_request", "Multipart request is invalid.")
		return
	}

	version := strings.TrimSpace(r.FormValue("version"))
	file, _, err := r.FormFile("artifact")
	if err != nil {
		server.recordPublishMetric("error", started)
		writeError(w, http.StatusBadRequest, "bad_request", "Artifact file is required.")
		return
	}
	defer file.Close()

	response, err := server.registry.PublishModuleVersion(r.Context(), registry.PublishModuleVersionRequest{
		ModuleName: r.PathValue("module"),
		Version:    version,
		Artifact:   file,
	})
	if err != nil {
		server.recordPublishMetric("error", started)
		writeUsecaseError(w, err)
		return
	}
	server.recordPublishMetric("success", started)

	writeJSON(w, http.StatusCreated, publishModuleVersionResponse{
		Module:           r.PathValue("module"),
		Version:          response.Version.Version.String(),
		Status:           response.Version.Status.String(),
		SourceArtifact:   artifactSummaryResponse(response.SourceArtifact),
		BufImageArtifact: artifactSummaryResponse(response.BufImageArtifact),
		Buf:              bufInfoResponse(response.BufConfig, response.LintResult),
		MetadataSummary:  metadataSummaryResponse(response.MetadataSummary),
		CreatedAt:        response.Version.CreatedAt,
	})
}

func (server *Server) listModuleVersions(w http.ResponseWriter, r *http.Request) {
	versions, err := server.registry.ListModuleVersions(r.Context(), r.PathValue("module"), 100, 0)
	if err != nil {
		writeUsecaseError(w, err)
		return
	}

	items := make([]moduleVersionDTO, 0, len(versions))
	for _, version := range versions {
		items = append(items, moduleVersionResponse(version))
	}
	writeJSON(w, http.StatusOK, listModuleVersionsResponse{Versions: items})
}

func (server *Server) getModuleVersion(w http.ResponseWriter, r *http.Request) {
	details, err := server.registry.GetModuleVersionDetails(r.Context(), r.PathValue("module"), r.PathValue("version"))
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, moduleVersionDetailsResponse(r.PathValue("module"), details))
}

func (server *Server) getModuleVersionMetadata(w http.ResponseWriter, r *http.Request) {
	metadata, err := server.registry.GetModuleVersionMetadata(r.Context(), r.PathValue("module"), r.PathValue("version"))
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, descriptorMetadataResponse(metadata))
}

func (server *Server) createBreakingCheck(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	server.limitRequestBody(w, r)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		server.recordBreakingCheckMetric("error", started)
		if isRequestBodyTooLarge(err) {
			writePayloadTooLarge(w)
			return
		}
		writeError(w, http.StatusBadRequest, "bad_request", "Multipart request is invalid.")
		return
	}

	file, header, err := r.FormFile("artifact")
	if err != nil {
		server.recordBreakingCheckMetric("error", started)
		writeError(w, http.StatusBadRequest, "bad_request", "Artifact file is required.")
		return
	}
	defer file.Close()

	var sizeBytes int64
	var archiveName string
	if header != nil {
		sizeBytes = header.Size
		archiveName = header.Filename
	}
	response, err := server.registry.CheckBreaking(r.Context(), registry.CheckBreakingRequest{
		ModuleName:            r.PathValue("module"),
		Against:               strings.TrimSpace(r.FormValue("against")),
		TargetRef:             strings.TrimSpace(r.FormValue("target_ref")),
		ProposedSourceArchive: file,
		ArchiveName:           archiveName,
		ArchiveSizeBytes:      sizeBytes,
	})
	if err != nil {
		server.recordBreakingCheckMetric("error", started)
		writeUsecaseError(w, err)
		return
	}
	server.recordBreakingCheckMetric(response.Report.Status.String(), started)
	writeJSON(w, http.StatusOK, breakingReportResponse(response.Report, response.Changes))
}

func (server *Server) getBreakingReport(w http.ResponseWriter, r *http.Request) {
	response, err := server.registry.GetBreakingReport(r.Context(), r.PathValue("report_id"))
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, breakingReportResponse(response.Report, response.Changes))
}

func (server *Server) getBreakingReportAffectedModules(w http.ResponseWriter, r *http.Request) {
	response, err := server.registry.GetBreakingReportAffectedModules(r.Context(), r.PathValue("report_id"))
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, breakingReportAffectedModulesResponse(response))
}

func (server *Server) reportRuntimeInventory(w http.ResponseWriter, r *http.Request) {
	if server.runtime == nil {
		server.recordRuntimeReportMetric("error")
		writeInternalError(w)
		return
	}
	server.limitRequestBody(w, r)
	var req reportRuntimeInventoryRequest
	if err := decodeJSON(r, &req); err != nil {
		server.recordRuntimeReportMetric("error")
		if isRequestBodyTooLarge(err) {
			writePayloadTooLarge(w)
			return
		}
		writeError(w, http.StatusBadRequest, "bad_request", "Request body is invalid.")
		return
	}
	modules := make([]runtimeinventory.ReportedModuleInput, 0, len(req.Modules))
	for _, module := range req.Modules {
		modules = append(modules, runtimeinventory.ReportedModuleInput{
			Module:  module.Module,
			Version: module.Version,
		})
	}
	output, err := server.runtime.ReportRuntimeInventory(r.Context(), runtimeinventory.ReportRuntimeInventoryInput{
		ServiceName:  req.ServiceName,
		Environment:  req.Environment,
		GitCommit:    req.GitCommit,
		BuildVersion: req.BuildVersion,
		Modules:      modules,
	})
	if err != nil {
		server.recordRuntimeReportMetric("error")
		writeUsecaseError(w, err)
		return
	}
	server.recordRuntimeReportMetric("success")
	writeJSON(w, http.StatusCreated, reportRuntimeInventoryResponse(output))
}

func (server *Server) listRuntimeServices(w http.ResponseWriter, r *http.Request) {
	if server.runtime == nil {
		writeInternalError(w)
		return
	}
	summaries, err := server.runtime.ListRuntimeServices(r.Context(), 100, 0)
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, runtimeServicesResponse(summaries))
}

func (server *Server) getRuntimeServiceDetails(w http.ResponseWriter, r *http.Request) {
	if server.runtime == nil {
		writeInternalError(w)
		return
	}
	details, err := server.runtime.GetRuntimeServiceDetails(r.Context(), r.PathValue("service"))
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, runtimeServiceDetailsResponse(details))
}

func (server *Server) getEnvironmentInventory(w http.ResponseWriter, r *http.Request) {
	if server.runtime == nil {
		writeInternalError(w)
		return
	}
	inventory, err := server.runtime.GetEnvironmentInventory(r.Context(), r.PathValue("environment"), 100, 0)
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, runtimeEnvironmentInventoryResponse(inventory))
}

func (server *Server) getModuleRuntimeUsages(w http.ResponseWriter, r *http.Request) {
	if server.runtime == nil {
		writeInternalError(w)
		return
	}
	usages, err := server.runtime.GetModuleRuntimeUsages(r.Context(), r.PathValue("module"), 100, 0)
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, moduleRuntimeUsagesResponse(r.PathValue("module"), usages))
}

func (server *Server) getBreakingReportRuntimeImpact(w http.ResponseWriter, r *http.Request) {
	if server.runtime == nil {
		writeInternalError(w)
		return
	}
	impacts, err := server.runtime.GetBreakingReportRuntimeImpact(r.Context(), domain.NewBreakingReportID(r.PathValue("report_id")), 100, 0)
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, breakingReportRuntimeImpactResponse(r.PathValue("report_id"), impacts))
}

func (server *Server) listBreakingReports(w http.ResponseWriter, r *http.Request) {
	limit := parsePositiveInt(r.URL.Query().Get("limit"), 100)
	reports, err := server.registry.ListBreakingReports(r.Context(), r.PathValue("module"), limit, 0)
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	items := make([]breakingReportSummaryDTO, 0, len(reports))
	for _, report := range reports {
		items = append(items, breakingReportSummaryResponse(report))
	}
	writeJSON(w, http.StatusOK, listBreakingReportsResponse{Reports: items})
}

func (server *Server) downloadArtifact(w http.ResponseWriter, r *http.Request) {
	moduleName := r.PathValue("module")
	versionValue := r.PathValue("version")

	version, err := server.registry.GetModuleVersion(r.Context(), moduleName, versionValue)
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	object, artifact, err := server.registry.DownloadArtifact(r.Context(), moduleName, versionValue)
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	defer object.Body.Close()

	contentType := object.ContentType
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s-%s.tar.gz"`, moduleName, versionValue))
	w.Header().Set("X-ProtoRadar-Digest", version.Digest)
	if artifact.ChecksumSHA256 != "" {
		w.Header().Set("X-ProtoRadar-Checksum-SHA256", artifact.ChecksumSHA256)
	}
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, object.Body)
}

func (server *Server) createAPIToken(w http.ResponseWriter, r *http.Request) {
	var req createAPITokenRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "Request body is invalid.")
		return
	}

	var expiresAt *time.Time
	if req.ExpiresAt != "" {
		parsed, err := time.Parse(time.RFC3339, req.ExpiresAt)
		if err != nil {
			writeError(w, http.StatusBadRequest, "validation_error", "expires_at must use RFC3339 format.")
			return
		}
		expiresAt = &parsed
	}

	token, err := server.registry.CreateAPIToken(r.Context(), registry.CreateAPITokenRequest{
		Name:      req.Name,
		ExpiresAt: expiresAt,
	})
	if err != nil {
		writeUsecaseError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, createAPITokenResponse{
		ID:        token.Token.ID.String(),
		Name:      token.Token.Name,
		Token:     token.RawToken,
		CreatedAt: token.Token.CreatedAt,
		ExpiresAt: token.Token.ExpiresAt,
	})
}

func decodeJSON(r *http.Request, dst any) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(dst)
}

func (server *Server) limitRequestBody(w http.ResponseWriter, r *http.Request) {
	if server.maxRequestBodyBytes <= 0 {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, server.maxRequestBodyBytes)
}

func isRequestBodyTooLarge(err error) bool {
	var maxBytesErr *http.MaxBytesError
	return errors.As(err, &maxBytesErr)
}

func parsePositiveInt(value string, fallback int) int {
	if value == "" {
		return fallback
	}
	var parsed int
	if _, err := fmt.Sscanf(value, "%d", &parsed); err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func writeUsecaseError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, registry.ErrInvalidModuleName), errors.Is(err, registry.ErrInvalidVersion), errors.Is(err, registry.ErrInvalidAgainst), errors.Is(err, registry.ErrInvalidTargetRef), errors.Is(err, registry.ErrArtifactRequired), errors.Is(err, registry.ErrInvalidGitLabBaseURL), errors.Is(err, registry.ErrInvalidGitLabProjectID), errors.Is(err, registry.ErrInvalidGitLabProjectPath), errors.Is(err, runtimeinventory.ErrRuntimeModulesRequired), errors.Is(err, domain.ErrInvalidRuntimeServiceName), errors.Is(err, domain.ErrInvalidRuntimeEnvironment), errors.Is(err, domain.ErrInvalidRuntimeGitCommit), errors.Is(err, domain.ErrInvalidRuntimeBuildVersion), errors.Is(err, domain.ErrInvalidModuleName), errors.Is(err, domain.ErrInvalidVersion):
		writeError(w, http.StatusBadRequest, "validation_error", "Request validation failed.")
	case errors.Is(err, registry.ErrInvalidOrExpiredToken):
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication is required.")
	case errors.Is(err, registry.ErrModuleNotFound), errors.Is(err, registry.ErrBaselineVersionNotFound), errors.Is(err, registry.ErrBreakingReportNotFound), errors.Is(err, registry.ErrModuleGitLabProjectNotFound), errors.Is(err, domain.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "Requested resource was not found.")
	case errors.Is(err, registry.ErrModuleAlreadyExists), errors.Is(err, registry.ErrModuleVersionAlreadyExists), errors.Is(err, registry.ErrGitLabProjectAlreadyLinked):
		writeError(w, http.StatusConflict, "conflict", "Requested operation conflicts with existing state.")
	case errors.Is(err, registry.ErrBaselineBufImageMissing):
		writeError(w, http.StatusConflict, "conflict", "Baseline Buf image is missing.")
	case errors.Is(err, registry.ErrArtifactTooLarge):
		writeError(w, http.StatusRequestEntityTooLarge, "payload_too_large", "Artifact is too large.")
	case errors.Is(err, registry.ErrUnsafeArchive):
		writeError(w, http.StatusBadRequest, "validation_error", "Archive is unsafe.")
	case errors.Is(err, registry.ErrBufConfigNotFound):
		writeError(w, http.StatusUnprocessableEntity, "unprocessable_entity", "Buf config was not found.")
	case errors.Is(err, registry.ErrBufBuildFailed):
		writeError(w, http.StatusUnprocessableEntity, "unprocessable_entity", "Buf build failed.")
	case errors.Is(err, registry.ErrBufLintFailed):
		writeError(w, http.StatusUnprocessableEntity, "unprocessable_entity", "Buf lint failed.")
	case errors.Is(err, registry.ErrDescriptorExtractionFailed):
		writeError(w, http.StatusUnprocessableEntity, "unprocessable_entity", "Descriptor extraction failed.")
	case errors.Is(err, registry.ErrBufBreakingFailed):
		writeError(w, http.StatusInternalServerError, "internal_error", "Buf breaking failed.")
	default:
		writeInternalError(w)
	}
}

func writeInternalError(w http.ResponseWriter) {
	writeError(w, http.StatusInternalServerError, "internal_error", "Internal server error.")
}

func writePayloadTooLarge(w http.ResponseWriter) {
	writeError(w, http.StatusRequestEntityTooLarge, "payload_too_large", "Request body is too large.")
}

func writeError(w http.ResponseWriter, status int, code string, message string) {
	writeJSON(w, status, errorResponse{Error: apiErrorDTO{Code: code, Message: message}})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
