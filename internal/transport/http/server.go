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

	"github.com/alryzden/ProtoRadar/internal/authorization"
	"github.com/alryzden/ProtoRadar/internal/domain"
	"github.com/alryzden/ProtoRadar/internal/edition"
	"github.com/alryzden/ProtoRadar/internal/identity"
	"github.com/alryzden/ProtoRadar/internal/storage"
	"github.com/alryzden/ProtoRadar/internal/usecase/governance"
	"github.com/alryzden/ProtoRadar/internal/usecase/registry"
	"github.com/alryzden/ProtoRadar/internal/usecase/runtimeinventory"
	"github.com/alryzden/ProtoRadar/internal/version"
)

type Registry interface {
	CreateModule(ctx context.Context, req registry.CreateModuleRequest) (domain.Module, error)
	ListModules(ctx context.Context, limit int, offset int) ([]domain.Module, error)
	GetModule(ctx context.Context, name string) (domain.Module, error)
	LinkModuleGitLabProject(ctx context.Context, input registry.LinkModuleGitLabProjectInput) (registry.LinkModuleGitLabProjectOutput, error)
	GetModuleGitLabProject(ctx context.Context, moduleName string) (registry.GetModuleGitLabProjectOutput, error)
	PublishModuleVersion(ctx context.Context, req registry.PublishModuleVersionRequest) (registry.PublishModuleVersionResponse, error)
	DeprecateModuleVersion(ctx context.Context, input registry.DeprecateModuleVersionInput) (registry.DeprecateModuleVersionOutput, error)
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

type Governance interface {
	AddModuleOwner(ctx context.Context, input governance.AddModuleOwnerInput) (domain.ModuleOwner, error)
	RemoveModuleOwner(ctx context.Context, input governance.RemoveModuleOwnerInput) error
	ListModuleOwners(ctx context.Context, input governance.ListModuleOwnersInput) ([]domain.ModuleOwner, error)
	governance.ApprovalWorkflow
	governance.ApprovalAuditReader
}

type Server struct {
	registry            Registry
	auth                identity.AuthProvider
	authorizer          authorization.Authorizer
	capabilities        edition.CapabilityChecker
	buildInfo           version.BuildInfo
	runtime             RuntimeInventory
	governance          Governance
	actorOverridePolicy governance.ActorOverridePolicy
	bootstrapToken      string
	readinessChecks     []ReadinessCheck
	logger              *slog.Logger
	metrics             *Metrics
	maxRequestBodyBytes int64
}

type Options struct {
	BootstrapToken                 string
	AuthProvider                   identity.AuthProvider
	Authorizer                     authorization.Authorizer
	CapabilityChecker              edition.CapabilityChecker
	BuildInfo                      version.BuildInfo
	Runtime                        RuntimeInventory
	Governance                     Governance
	GovernanceActorOverrideEnabled bool
	Ready                          func(context.Context) error
	ReadinessChecks                []ReadinessCheck
	Logger                         *slog.Logger
	Metrics                        *Metrics
	MaxRequestBodyBytes            int64
}

type ReadinessCheck struct {
	Name  string
	Check func(context.Context) error
}

func NewServer(registryService Registry, options Options) *Server {
	readiness := append([]ReadinessCheck{}, options.ReadinessChecks...)
	if options.Ready != nil {
		readiness = append(readiness, ReadinessCheck{Name: "database", Check: options.Ready})
	}
	authProvider := options.AuthProvider
	if authProvider == nil && registryService != nil {
		authProvider = registryAuthProvider{registry: registryService}
	}
	authorizer := options.Authorizer
	if authorizer == nil {
		authorizer = authorization.CommunityAuthorizer{}
	}
	capabilityChecker := options.CapabilityChecker
	if capabilityChecker == nil {
		capabilityChecker = edition.NewCommunityCapabilityChecker()
	}
	buildInfo := options.BuildInfo
	if buildInfo.Version == "" {
		buildInfo = version.Info()
	}
	return &Server{
		registry:            registryService,
		auth:                authProvider,
		authorizer:          authorizer,
		capabilities:        capabilityChecker,
		buildInfo:           buildInfo,
		runtime:             options.Runtime,
		governance:          options.Governance,
		actorOverridePolicy: governance.ActorOverridePolicy{Enabled: options.GovernanceActorOverrideEnabled},
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
	mux.Handle("GET /api/v1/edition", server.protected(authorization.ActionEditionRead, staticResource("edition"), http.HandlerFunc(server.getEdition)))
	mux.Handle("POST /api/v1/modules", server.protected(authorization.ActionModuleCreate, staticResource("module_collection"), http.HandlerFunc(server.createModule)))
	mux.Handle("GET /api/v1/modules", server.protected(authorization.ActionModuleRead, staticResource("module_collection"), http.HandlerFunc(server.listModules)))
	mux.Handle("GET /api/v1/modules/{module}", server.protected(authorization.ActionModuleRead, pathResource("module", "module"), http.HandlerFunc(server.getModule)))
	mux.Handle("PUT /api/v1/modules/{module}/gitlab-project", server.protected(authorization.ActionGitLabMappingManage, pathResource("module", "module"), http.HandlerFunc(server.linkModuleGitLabProject)))
	mux.Handle("GET /api/v1/modules/{module}/gitlab-project", server.protected(authorization.ActionGitLabMappingRead, pathResource("module", "module"), http.HandlerFunc(server.getModuleGitLabProject)))
	mux.Handle("GET /api/v1/modules/{module}/dependencies", server.protected(authorization.ActionDependencyGraphRead, pathResource("module", "module"), http.HandlerFunc(server.getModuleDependencies)))
	mux.Handle("GET /api/v1/modules/{module}/affected", server.protected(authorization.ActionDependencyGraphRead, pathResource("module", "module"), http.HandlerFunc(server.getAffectedModules)))
	mux.Handle("GET /api/v1/modules/{module}/owners", server.protected(authorization.ActionGovernanceOwnerRead, pathResource("module", "module"), http.HandlerFunc(server.listModuleOwners)))
	mux.Handle("POST /api/v1/modules/{module}/owners", server.protected(authorization.ActionGovernanceOwnerManage, pathResource("module", "module"), http.HandlerFunc(server.addModuleOwner)))
	mux.Handle("DELETE /api/v1/modules/{module}/owners/{owner_id}", server.protected(authorization.ActionGovernanceOwnerManage, pathIDResource("module_owner", "owner_id"), http.HandlerFunc(server.removeModuleOwner)))
	mux.Handle("GET /api/v1/modules/{module}/runtime-usages", server.protected(authorization.ActionRuntimeInventoryRead, pathResource("module", "module"), http.HandlerFunc(server.getModuleRuntimeUsages)))
	mux.Handle("POST /api/v1/modules/{module}/versions", server.protected(authorization.ActionModuleVersionPublish, pathResource("module", "module"), http.HandlerFunc(server.publishModuleVersion)))
	mux.Handle("GET /api/v1/modules/{module}/versions", server.protected(authorization.ActionModuleVersionRead, pathResource("module", "module"), http.HandlerFunc(server.listModuleVersions)))
	mux.Handle("GET /api/v1/modules/{module}/versions/{version}", server.protected(authorization.ActionModuleVersionRead, moduleVersionResource("module", "version"), http.HandlerFunc(server.getModuleVersion)))
	mux.Handle("POST /api/v1/modules/{module}/versions/{version}/deprecate", server.protected(authorization.ActionModuleVersionDeprecate, moduleVersionResource("module", "version"), http.HandlerFunc(server.deprecateModuleVersion)))
	mux.Handle("GET /api/v1/modules/{module}/versions/{version}/metadata", server.protected(authorization.ActionModuleVersionRead, moduleVersionResource("module", "version"), http.HandlerFunc(server.getModuleVersionMetadata)))
	mux.Handle("GET /api/v1/modules/{module}/versions/{version}/artifact", server.protected(authorization.ActionModuleVersionDownload, moduleVersionResource("module", "version"), http.HandlerFunc(server.downloadArtifact)))
	mux.Handle("POST /api/v1/modules/{module}/breaking-checks", server.protected(authorization.ActionBreakingCheckRun, pathResource("module", "module"), http.HandlerFunc(server.createBreakingCheck)))
	mux.Handle("GET /api/v1/modules/{module}/breaking-reports", server.protected(authorization.ActionBreakingReportRead, pathResource("module", "module"), http.HandlerFunc(server.listBreakingReports)))
	mux.Handle("GET /api/v1/breaking-reports/{report_id}", server.protected(authorization.ActionBreakingReportRead, pathIDResource("breaking_report", "report_id"), http.HandlerFunc(server.getBreakingReport)))
	mux.Handle("GET /api/v1/breaking-reports/{report_id}/affected-modules", server.protected(authorization.ActionBreakingReportRead, pathIDResource("breaking_report", "report_id"), http.HandlerFunc(server.getBreakingReportAffectedModules)))
	mux.Handle("GET /api/v1/breaking-reports/{report_id}/runtime-impact", server.protected(authorization.ActionBreakingReportRead, pathIDResource("breaking_report", "report_id"), http.HandlerFunc(server.getBreakingReportRuntimeImpact)))
	mux.Handle("POST /api/v1/breaking-reports/{report_id}/approval-request", server.protected(authorization.ActionApprovalRequestCreate, pathIDResource("breaking_report", "report_id"), http.HandlerFunc(server.createApprovalRequest)))
	mux.Handle("GET /api/v1/breaking-reports/{report_id}/approval-status", server.protected(authorization.ActionApprovalRequestRead, pathIDResource("breaking_report", "report_id"), http.HandlerFunc(server.getApprovalStatus)))
	mux.Handle("POST /api/v1/approval-requests/{request_id}/requirements/{requirement_id}/approve", server.protected(authorization.ActionApprovalDecisionRecord, pathIDResource("approval_request", "request_id"), http.HandlerFunc(server.approveRequirement)))
	mux.Handle("POST /api/v1/approval-requests/{request_id}/requirements/{requirement_id}/reject", server.protected(authorization.ActionApprovalDecisionRecord, pathIDResource("approval_request", "request_id"), http.HandlerFunc(server.rejectRequirement)))
	mux.Handle("GET /api/v1/approval-requests/{request_id}/audit", server.protected(authorization.ActionGovernanceAuditRead, pathIDResource("approval_request", "request_id"), http.HandlerFunc(server.getApprovalRequestAudit)))
	mux.Handle("POST /api/v1/runtime/reports", server.protected(authorization.ActionRuntimeInventoryReport, staticResource("runtime_inventory"), http.HandlerFunc(server.reportRuntimeInventory)))
	mux.Handle("GET /api/v1/runtime/services", server.protected(authorization.ActionRuntimeInventoryRead, staticResource("runtime_inventory"), http.HandlerFunc(server.listRuntimeServices)))
	mux.Handle("GET /api/v1/runtime/services/{service}", server.protected(authorization.ActionRuntimeInventoryRead, pathResource("runtime_service", "service"), http.HandlerFunc(server.getRuntimeServiceDetails)))
	mux.Handle("GET /api/v1/runtime/environments/{environment}", server.protected(authorization.ActionRuntimeInventoryRead, pathResource("runtime_environment", "environment"), http.HandlerFunc(server.getEnvironmentInventory)))
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

func (server *Server) getEdition(w http.ResponseWriter, r *http.Request) {
	model := edition.NewCommunityEdition(r.Context(), server.buildInfo, server.capabilities)
	writeJSON(w, http.StatusOK, editionDTO(model))
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
		if server.auth == nil {
			writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication is required.")
			return
		}
		principal, err := server.auth.Authenticate(r.Context(), identity.AuthRequest{
			AuthorizationHeader: r.Header.Get("Authorization"),
			RequestID:           requestIDFromContext(r.Context()),
			RemoteAddr:          r.RemoteAddr,
		})
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication is required.")
			return
		}
		next.ServeHTTP(w, r.WithContext(identity.ContextWithPrincipal(r.Context(), principal)))
	})
}

type resourceBuilder func(*http.Request) authorization.Resource

func (server *Server) protected(action authorization.Action, resource resourceBuilder, next http.Handler) http.Handler {
	return server.requireBearer(server.requireAuthorization(action, resource, next))
}

func (server *Server) requireAuthorization(action authorization.Action, resource resourceBuilder, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := identity.PrincipalFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication is required.")
			return
		}
		target := authorization.Resource{}
		if resource != nil {
			target = resource(r)
		}
		if err := server.authorizer.Authorize(r.Context(), principal, action, target); err != nil {
			writeAuthorizationError(w, err)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func staticResource(resourceType string) resourceBuilder {
	return func(r *http.Request) authorization.Resource {
		return authorization.Resource{Type: resourceType}
	}
}

func pathResource(resourceType string, pathValue string) resourceBuilder {
	return func(r *http.Request) authorization.Resource {
		return authorization.Resource{Type: resourceType, Name: r.PathValue(pathValue)}
	}
}

func pathIDResource(resourceType string, pathValue string) resourceBuilder {
	return func(r *http.Request) authorization.Resource {
		return authorization.Resource{Type: resourceType, ID: r.PathValue(pathValue)}
	}
}

func moduleVersionResource(modulePathValue string, versionPathValue string) resourceBuilder {
	return func(r *http.Request) authorization.Resource {
		return authorization.ModuleVersionResource(r.PathValue(modulePathValue), r.PathValue(versionPathValue))
	}
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

type registryAuthProvider struct {
	registry Registry
}

func (provider registryAuthProvider) Authenticate(ctx context.Context, req identity.AuthRequest) (identity.Principal, error) {
	token, ok := req.Bearer()
	if !ok {
		return identity.Principal{}, registry.ErrInvalidOrExpiredToken
	}
	subject, err := provider.registry.AuthenticateToken(ctx, token)
	if err != nil {
		return identity.Principal{}, err
	}
	principalSubject := strings.TrimSpace(subject.Name)
	if principalSubject == "" {
		principalSubject = subject.TokenID.String()
	}
	return identity.Principal{
		Subject:     principalSubject,
		DisplayName: principalSubject,
		Type:        identity.PrincipalTypeAPIToken,
		Metadata: map[string]string{
			"api_token_id": subject.TokenID.String(),
		},
	}, nil
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
	// #nosec G120 -- limitRequestBody wraps r.Body with http.MaxBytesReader before multipart parsing.
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		server.recordPublishMetric("error", started)
		if isRequestBodyTooLarge(err) {
			writePayloadTooLarge(w)
			return
		}
		writeError(w, http.StatusBadRequest, "bad_request", "Multipart request is invalid.")
		return
	}

	versionValue := strings.TrimSpace(r.FormValue("version"))
	file, _, err := r.FormFile("artifact")
	if err != nil {
		server.recordPublishMetric("error", started)
		writeError(w, http.StatusBadRequest, "bad_request", "Artifact file is required.")
		return
	}
	defer closeReadCloser(file)

	response, err := server.registry.PublishModuleVersion(r.Context(), registry.PublishModuleVersionRequest{
		ModuleName: r.PathValue("module"),
		Version:    versionValue,
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

func (server *Server) deprecateModuleVersion(w http.ResponseWriter, r *http.Request) {
	server.limitRequestBody(w, r)
	var req deprecateModuleVersionRequest
	if r.Body != nil && r.ContentLength != 0 {
		if err := decodeJSON(r, &req); err != nil {
			if isRequestBodyTooLarge(err) {
				writePayloadTooLarge(w)
				return
			}
			writeError(w, http.StatusBadRequest, "bad_request", "Request body is invalid.")
			return
		}
	}
	principal, ok := identity.PrincipalFromContext(r.Context())
	if !ok || strings.TrimSpace(principal.Subject) == "" {
		writeUsecaseError(w, registry.ErrInvalidActor)
		return
	}
	response, err := server.registry.DeprecateModuleVersion(r.Context(), registry.DeprecateModuleVersionInput{
		ModuleName: r.PathValue("module"),
		Version:    r.PathValue("version"),
		Actor:      principal.Subject,
		Reason:     req.Reason,
	})
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, moduleVersionResponse(response.Version))
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
	// #nosec G120 -- limitRequestBody wraps r.Body with http.MaxBytesReader before multipart parsing.
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
	defer closeReadCloser(file)

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

	moduleVersion, err := server.registry.GetModuleVersion(r.Context(), moduleName, versionValue)
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	object, artifact, err := server.registry.DownloadArtifact(r.Context(), moduleName, versionValue)
	if err != nil {
		writeUsecaseError(w, err)
		return
	}
	defer closeReadCloser(object.Body)

	contentType := object.ContentType
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s-%s.tar.gz"`, moduleName, versionValue))
	w.Header().Set("X-ProtoRadar-Digest", moduleVersion.Digest)
	if artifact.ChecksumSHA256 != "" {
		w.Header().Set("X-ProtoRadar-Checksum-SHA256", artifact.ChecksumSHA256)
	}
	w.WriteHeader(http.StatusOK)
	bytesCopied, copyErr := io.Copy(w, object.Body)
	if copyErr != nil {
		if server.metrics != nil {
			server.metrics.RecordArtifactStreamError()
		}
		if server.logger != nil {
			server.logger.ErrorContext(r.Context(), "artifact_download_stream_error",
				slog.String("module", moduleName),
				slog.String("version", versionValue),
				slog.String("artifact_kind", artifact.Kind.String()),
				slog.Int64("artifact_size_bytes", artifact.SizeBytes),
				slog.Int64("bytes_copied", bytesCopied),
				slog.String("error_type", fmt.Sprintf("%T", copyErr)),
				slog.String("request_id", requestIDFromContext(r.Context())),
			)
		}
	}
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
	case errors.Is(err, registry.ErrInvalidModuleName), errors.Is(err, registry.ErrInvalidVersion), errors.Is(err, registry.ErrInvalidActor), errors.Is(err, registry.ErrInvalidAgainst), errors.Is(err, registry.ErrInvalidTargetRef), errors.Is(err, registry.ErrArtifactRequired), errors.Is(err, registry.ErrInvalidGitLabBaseURL), errors.Is(err, registry.ErrInvalidGitLabProjectID), errors.Is(err, registry.ErrInvalidGitLabProjectPath), errors.Is(err, runtimeinventory.ErrRuntimeModulesRequired), errors.Is(err, domain.ErrInvalidRuntimeServiceName), errors.Is(err, domain.ErrInvalidRuntimeEnvironment), errors.Is(err, domain.ErrInvalidRuntimeGitCommit), errors.Is(err, domain.ErrInvalidRuntimeBuildVersion), errors.Is(err, domain.ErrInvalidModuleName), errors.Is(err, domain.ErrInvalidVersion), errors.Is(err, domain.ErrInvalidGovernanceSubjectType), errors.Is(err, domain.ErrInvalidModuleOwnerRole), errors.Is(err, domain.ErrInvalidApprovalRequestStatus), errors.Is(err, domain.ErrInvalidApprovalRequirementType), errors.Is(err, domain.ErrInvalidApprovalRequirementStatus), errors.Is(err, domain.ErrInvalidApprovalDecision), errors.Is(err, domain.ErrInvalidGovernanceAuditEventType), errors.Is(err, governance.ErrInvalidModuleName), errors.Is(err, governance.ErrInvalidSubjectType), errors.Is(err, governance.ErrInvalidSubject), errors.Is(err, governance.ErrInvalidModuleOwnerRole), errors.Is(err, governance.ErrInvalidModuleOwnerID), errors.Is(err, governance.ErrInvalidBreakingReportID), errors.Is(err, governance.ErrInvalidRequirementID), errors.Is(err, governance.ErrInvalidApprovalActor), errors.Is(err, governance.ErrApprovalDecisionCommentTooLong):
		writeError(w, http.StatusBadRequest, "validation_error", "Request validation failed.")
	case errors.Is(err, registry.ErrInvalidOrExpiredToken):
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication is required.")
	case errors.Is(err, registry.ErrModuleNotFound), errors.Is(err, registry.ErrBaselineVersionNotFound), errors.Is(err, registry.ErrBreakingReportNotFound), errors.Is(err, registry.ErrModuleGitLabProjectNotFound), errors.Is(err, domain.ErrNotFound), errors.Is(err, governance.ErrModuleNotFound), errors.Is(err, governance.ErrModuleOwnerNotFound), errors.Is(err, governance.ErrBreakingReportNotFound), errors.Is(err, governance.ErrApprovalRequestNotFound), errors.Is(err, governance.ErrApprovalRequirementNotFound):
		writeError(w, http.StatusNotFound, "not_found", "Requested resource was not found.")
	case errors.Is(err, registry.ErrModuleAlreadyExists), errors.Is(err, registry.ErrModuleVersionAlreadyExists), errors.Is(err, registry.ErrGitLabProjectAlreadyLinked), errors.Is(err, governance.ErrModuleOwnerAlreadyExists), errors.Is(err, governance.ErrApprovalRequestConflict), errors.Is(err, governance.ErrApprovalRequirementConflict):
		writeError(w, http.StatusConflict, "conflict", "Requested operation conflicts with existing state.")
	case errors.Is(err, governance.ErrApprovalActorForbidden), errors.Is(err, governance.ErrGovernanceActorOverrideForbidden):
		writeError(w, http.StatusForbidden, "forbidden", "Governance actor is not allowed.")
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

func writeAuthorizationError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, authorization.ErrUnauthenticated):
		writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication is required.")
	default:
		writeError(w, http.StatusForbidden, "forbidden", "Principal is not allowed to perform this action.")
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
	if err := json.NewEncoder(w).Encode(body); err != nil {
		return
	}
}

func closeReadCloser(body io.Closer) {
	// Close failures here are cleanup-only; request handlers already returned
	// their primary response or stream error.
	_ = body.Close() //nolint:errcheck
}
