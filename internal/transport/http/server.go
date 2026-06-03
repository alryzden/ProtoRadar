package httptransport

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/alryzden/ProtoRadar/internal/domain"
	"github.com/alryzden/ProtoRadar/internal/storage"
	"github.com/alryzden/ProtoRadar/internal/usecase/registry"
)

type Registry interface {
	CreateModule(ctx context.Context, req registry.CreateModuleRequest) (domain.Module, error)
	ListModules(ctx context.Context, limit int, offset int) ([]domain.Module, error)
	GetModule(ctx context.Context, name string) (domain.Module, error)
	PublishModuleVersion(ctx context.Context, req registry.PublishModuleVersionRequest) (registry.PublishModuleVersionResponse, error)
	ListModuleVersions(ctx context.Context, moduleName string, limit int, offset int) ([]domain.ModuleVersion, error)
	GetModuleVersion(ctx context.Context, moduleName string, version string) (domain.ModuleVersion, error)
	GetModuleVersionDetails(ctx context.Context, moduleName string, version string) (registry.ModuleVersionDetailsResponse, error)
	GetModuleVersionMetadata(ctx context.Context, moduleName string, version string) (domain.DescriptorMetadata, error)
	DownloadArtifact(ctx context.Context, moduleName string, version string) (storage.ArtifactObject, domain.Artifact, error)
	CreateAPIToken(ctx context.Context, req registry.CreateAPITokenRequest) (registry.CreateAPITokenResponse, error)
	AuthenticateToken(ctx context.Context, rawToken string) (registry.AuthSubject, error)
}

type Server struct {
	registry       Registry
	bootstrapToken string
	ready          func(context.Context) error
}

type Options struct {
	BootstrapToken string
	Ready          func(context.Context) error
}

func NewServer(registry Registry, options Options) *Server {
	return &Server{
		registry:       registry,
		bootstrapToken: options.BootstrapToken,
		ready:          options.Ready,
	}
}

func (server *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", server.health)
	mux.HandleFunc("GET /readyz", server.readiness)
	mux.Handle("POST /api/v1/modules", server.requireBearer(http.HandlerFunc(server.createModule)))
	mux.Handle("GET /api/v1/modules", server.requireBearer(http.HandlerFunc(server.listModules)))
	mux.Handle("GET /api/v1/modules/{module}", server.requireBearer(http.HandlerFunc(server.getModule)))
	mux.Handle("POST /api/v1/modules/{module}/versions", server.requireBearer(http.HandlerFunc(server.publishModuleVersion)))
	mux.Handle("GET /api/v1/modules/{module}/versions", server.requireBearer(http.HandlerFunc(server.listModuleVersions)))
	mux.Handle("GET /api/v1/modules/{module}/versions/{version}", server.requireBearer(http.HandlerFunc(server.getModuleVersion)))
	mux.Handle("GET /api/v1/modules/{module}/versions/{version}/metadata", server.requireBearer(http.HandlerFunc(server.getModuleVersionMetadata)))
	mux.Handle("GET /api/v1/modules/{module}/versions/{version}/artifact", server.requireBearer(http.HandlerFunc(server.downloadArtifact)))
	mux.Handle("POST /api/v1/tokens", server.requireBootstrapToken(http.HandlerFunc(server.createAPIToken)))

	return mux
}

func (server *Server) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (server *Server) readiness(w http.ResponseWriter, r *http.Request) {
	if server.ready != nil {
		if err := server.ready(r.Context()); err != nil {
			writeError(w, http.StatusServiceUnavailable, "not ready")
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (server *Server) requireBearer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, ok := bearerToken(r.Header.Get("Authorization"))
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		if _, err := server.registry.AuthenticateToken(r.Context(), token); err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (server *Server) requireBootstrapToken(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, ok := bearerToken(r.Header.Get("Authorization"))
		if !ok || !sameToken(token, server.bootstrapToken) {
			writeError(w, http.StatusUnauthorized, "unauthorized")
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
		writeError(w, http.StatusBadRequest, "invalid request")
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

func (server *Server) publishModuleVersion(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}

	version := strings.TrimSpace(r.FormValue("version"))
	file, _, err := r.FormFile("artifact")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}
	defer file.Close()

	response, err := server.registry.PublishModuleVersion(r.Context(), registry.PublishModuleVersionRequest{
		ModuleName: r.PathValue("module"),
		Version:    version,
		Artifact:   file,
	})
	if err != nil {
		writeUsecaseError(w, err)
		return
	}

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
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}

	var expiresAt *time.Time
	if req.ExpiresAt != "" {
		parsed, err := time.Parse(time.RFC3339, req.ExpiresAt)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid request")
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

func writeUsecaseError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, registry.ErrInvalidModuleName), errors.Is(err, registry.ErrInvalidVersion):
		writeError(w, http.StatusBadRequest, "invalid request")
	case errors.Is(err, registry.ErrInvalidOrExpiredToken):
		writeError(w, http.StatusUnauthorized, "unauthorized")
	case errors.Is(err, registry.ErrModuleNotFound):
		writeError(w, http.StatusNotFound, "not found")
	case errors.Is(err, registry.ErrModuleAlreadyExists), errors.Is(err, registry.ErrModuleVersionAlreadyExists):
		writeError(w, http.StatusConflict, "conflict")
	case errors.Is(err, registry.ErrArtifactTooLarge):
		writeError(w, http.StatusRequestEntityTooLarge, "artifact too large")
	case errors.Is(err, registry.ErrUnsafeArchive):
		writeError(w, http.StatusBadRequest, "unsafe archive")
	case errors.Is(err, registry.ErrBufConfigNotFound):
		writeError(w, http.StatusUnprocessableEntity, "buf config not found")
	case errors.Is(err, registry.ErrBufBuildFailed):
		writeError(w, http.StatusUnprocessableEntity, "buf build failed")
	case errors.Is(err, registry.ErrBufLintFailed):
		writeError(w, http.StatusUnprocessableEntity, "buf lint failed")
	case errors.Is(err, registry.ErrDescriptorExtractionFailed):
		writeError(w, http.StatusUnprocessableEntity, "descriptor extraction failed")
	default:
		writeError(w, http.StatusInternalServerError, "internal error")
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, errorResponse{Error: message})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
