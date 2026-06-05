package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var (
	ErrUnauthorized = errors.New("unauthorized")
	ErrConflict     = errors.New("conflict")
	ErrNotFound     = errors.New("not found")
	ErrTooLarge     = errors.New("artifact too large")
)

type Error struct {
	StatusCode int
	Status     string
	Code       string
	Body       string
}

func (err Error) Error() string {
	if err.Body == "" {
		return "server returned " + err.Status
	}
	return "server returned " + err.Status + ": " + err.Body
}

func (err Error) Unwrap() error {
	switch err.StatusCode {
	case http.StatusUnauthorized:
		return ErrUnauthorized
	case http.StatusConflict:
		return ErrConflict
	case http.StatusNotFound:
		return ErrNotFound
	case http.StatusRequestEntityTooLarge:
		return ErrTooLarge
	default:
		return nil
	}
}

type CreateModuleRequest struct {
	Name          string `json:"name"`
	Description   string `json:"description"`
	RepositoryURL string `json:"repository_url"`
}

type Module struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Description   string    `json:"description"`
	RepositoryURL string    `json:"repository_url"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type ListModulesResponse struct {
	Modules []Module `json:"modules"`
}

type ModuleVersion struct {
	ID          string     `json:"id"`
	ModuleID    string     `json:"module_id"`
	Version     string     `json:"version"`
	Digest      string     `json:"digest"`
	Status      string     `json:"status"`
	CreatedAt   time.Time  `json:"created_at"`
	PublishedAt *time.Time `json:"published_at,omitempty"`
}

type ListModuleVersionsResponse struct {
	Versions []ModuleVersion `json:"versions"`
}

type Artifact struct {
	ID              string    `json:"id"`
	ModuleVersionID string    `json:"module_version_id"`
	Kind            string    `json:"kind"`
	StorageKey      string    `json:"storage_key"`
	ChecksumSHA256  string    `json:"checksum_sha256"`
	SizeBytes       int64     `json:"size_bytes"`
	CreatedAt       time.Time `json:"created_at"`
}

type ArtifactSummary struct {
	Kind           string `json:"kind"`
	ChecksumSHA256 string `json:"checksum_sha256"`
	SizeBytes      int64  `json:"size_bytes"`
}

type BufInfo struct {
	ConfigPresent bool   `json:"config_present"`
	LockPresent   bool   `json:"lock_present"`
	LintStatus    string `json:"lint_status"`
	LintReport    string `json:"lint_report,omitempty"`
}

type MetadataSummary struct {
	Files      int `json:"files"`
	Packages   int `json:"packages"`
	Services   int `json:"services"`
	Methods    int `json:"methods"`
	Messages   int `json:"messages"`
	Fields     int `json:"fields"`
	Enums      int `json:"enums"`
	EnumValues int `json:"enum_values"`
}

type PublishModuleVersionResponse struct {
	Module           string          `json:"module"`
	Version          string          `json:"version"`
	Status           string          `json:"status"`
	SourceArtifact   ArtifactSummary `json:"source_artifact"`
	BufImageArtifact ArtifactSummary `json:"buf_image_artifact"`
	Buf              BufInfo         `json:"buf"`
	MetadataSummary  MetadataSummary `json:"metadata_summary"`
	CreatedAt        time.Time       `json:"created_at"`
}

type BreakingChange struct {
	Category    string `json:"category"`
	FilePath    string `json:"file_path"`
	PackageName string `json:"package_name"`
	Symbol      string `json:"symbol"`
	RuleID      string `json:"rule_id"`
	Message     string `json:"message"`
	Severity    string `json:"severity"`
}

type BreakingReport struct {
	ID           string           `json:"id"`
	Module       string           `json:"module"`
	Against      string           `json:"against"`
	TargetRef    string           `json:"target_ref"`
	Status       string           `json:"status"`
	ChangeCount  int              `json:"change_count"`
	Changes      []BreakingChange `json:"changes"`
	HumanSummary string           `json:"human_summary"`
	CreatedAt    time.Time        `json:"created_at"`
}

type LinkModuleGitLabProjectRequest struct {
	GitLabBaseURL     string `json:"gitlab_base_url"`
	GitLabProjectID   int64  `json:"gitlab_project_id"`
	GitLabProjectPath string `json:"gitlab_project_path"`
}

type ModuleGitLabProject struct {
	Module            string    `json:"module"`
	GitLabBaseURL     string    `json:"gitlab_base_url"`
	GitLabProjectID   int64     `json:"gitlab_project_id"`
	GitLabProjectPath string    `json:"gitlab_project_path"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type DependencyModule struct {
	Module            string   `json:"module"`
	LatestVersion     string   `json:"latest_version"`
	DependencySources []string `json:"dependency_sources"`
	Reasons           []string `json:"reasons"`
}

type UnresolvedDependency struct {
	Module           string `json:"module"`
	Version          string `json:"version"`
	Source           string `json:"source"`
	ImportPath       string `json:"import_path"`
	ReferencedSymbol string `json:"referenced_symbol"`
	Reason           string `json:"reason"`
}

type ModuleDependencyGraph struct {
	Module     string                 `json:"module"`
	Upstream   []DependencyModule     `json:"upstream"`
	Downstream []DependencyModule     `json:"downstream"`
	Unresolved []UnresolvedDependency `json:"unresolved"`
}

type AffectedModules struct {
	Module          string             `json:"module"`
	AffectedModules []DependencyModule `json:"affected_modules"`
}

type RuntimeImpact struct {
	ServiceName  string    `json:"service_name"`
	Environment  string    `json:"environment"`
	UsedModule   string    `json:"used_module"`
	UsedVersion  string    `json:"used_version"`
	GitCommit    string    `json:"git_commit"`
	BuildVersion string    `json:"build_version"`
	ReportedAt   time.Time `json:"reported_at"`
	ImpactStatus string    `json:"impact_status"`
	Reason       string    `json:"reason"`
}

type BreakingReportRuntimeImpact struct {
	ReportID string          `json:"report_id"`
	Impacts  []RuntimeImpact `json:"impacts"`
}

type ReportRuntimeInventoryRequest struct {
	ServiceName  string                 `json:"service_name"`
	Environment  string                 `json:"environment"`
	GitCommit    string                 `json:"git_commit"`
	BuildVersion string                 `json:"build_version"`
	Modules      []RuntimeModuleRequest `json:"modules"`
}

type RuntimeModuleRequest struct {
	Module  string `json:"module"`
	Version string `json:"version"`
}

type RuntimeModuleUsage struct {
	Module        string `json:"module"`
	Version       string `json:"version"`
	LatestVersion string `json:"latest_version"`
	DriftStatus   string `json:"drift_status"`
	DriftReason   string `json:"drift_reason"`
}

type ReportRuntimeInventoryResponse struct {
	DeploymentID string               `json:"deployment_id"`
	ServiceName  string               `json:"service_name"`
	Environment  string               `json:"environment"`
	GitCommit    string               `json:"git_commit"`
	BuildVersion string               `json:"build_version"`
	ReportedAt   time.Time            `json:"reported_at"`
	Usages       []RuntimeModuleUsage `json:"usages"`
}

type ArtifactDownload struct {
	Body           io.ReadCloser
	ContentType    string
	Digest         string
	ChecksumSHA256 string
	SizeBytes      int64
}

type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

func NewClient(serverURL string, token string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{
		baseURL:    strings.TrimRight(serverURL, "/"),
		token:      token,
		httpClient: httpClient,
	}
}

func (client *Client) CheckAuth(ctx context.Context) error {
	req, err := client.newRequest(ctx, http.MethodGet, "/api/v1/modules", nil)
	if err != nil {
		return err
	}

	res, err := client.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return client.responseError(res)
	}
	return nil
}

func (client *Client) CreateModule(ctx context.Context, req CreateModuleRequest) (Module, error) {
	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(req); err != nil {
		return Module{}, err
	}

	httpReq, err := client.newRequest(ctx, http.MethodPost, "/api/v1/modules", &body)
	if err != nil {
		return Module{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	var module Module
	if err := client.doJSON(httpReq, &module); err != nil {
		return Module{}, err
	}
	return module, nil
}

func (client *Client) ListModules(ctx context.Context) ([]Module, error) {
	req, err := client.newRequest(ctx, http.MethodGet, "/api/v1/modules", nil)
	if err != nil {
		return nil, err
	}

	var response ListModulesResponse
	if err := client.doJSON(req, &response); err != nil {
		return nil, err
	}
	return response.Modules, nil
}

func (client *Client) ListModuleVersions(ctx context.Context, module string) ([]ModuleVersion, error) {
	req, err := client.newRequest(ctx, http.MethodGet, "/api/v1/modules/"+module+"/versions", nil)
	if err != nil {
		return nil, err
	}

	var response ListModuleVersionsResponse
	if err := client.doJSON(req, &response); err != nil {
		return nil, err
	}
	return response.Versions, nil
}

func (client *Client) PublishModuleVersion(ctx context.Context, module string, version string, artifactName string, artifact []byte) (PublishModuleVersionResponse, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("version", version); err != nil {
		return PublishModuleVersionResponse{}, err
	}
	part, err := writer.CreateFormFile("artifact", artifactName)
	if err != nil {
		return PublishModuleVersionResponse{}, err
	}
	if _, err := part.Write(artifact); err != nil {
		return PublishModuleVersionResponse{}, err
	}
	if err := writer.Close(); err != nil {
		return PublishModuleVersionResponse{}, err
	}

	req, err := client.newRequest(ctx, http.MethodPost, "/api/v1/modules/"+module+"/versions", &body)
	if err != nil {
		return PublishModuleVersionResponse{}, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	var response PublishModuleVersionResponse
	if err := client.doJSON(req, &response); err != nil {
		return PublishModuleVersionResponse{}, err
	}
	return response, nil
}

func (client *Client) CheckBreaking(ctx context.Context, module string, against string, targetRef string, artifactName string, artifact []byte) (BreakingReport, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("against", against); err != nil {
		return BreakingReport{}, err
	}
	if strings.TrimSpace(targetRef) != "" {
		if err := writer.WriteField("target_ref", targetRef); err != nil {
			return BreakingReport{}, err
		}
	}
	part, err := writer.CreateFormFile("artifact", artifactName)
	if err != nil {
		return BreakingReport{}, err
	}
	if _, err := part.Write(artifact); err != nil {
		return BreakingReport{}, err
	}
	if err := writer.Close(); err != nil {
		return BreakingReport{}, err
	}

	req, err := client.newRequest(ctx, http.MethodPost, "/api/v1/modules/"+module+"/breaking-checks", &body)
	if err != nil {
		return BreakingReport{}, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	var response BreakingReport
	if err := client.doJSON(req, &response); err != nil {
		return BreakingReport{}, err
	}
	return response, nil
}

func (client *Client) LinkModuleGitLabProject(ctx context.Context, module string, req LinkModuleGitLabProjectRequest) (ModuleGitLabProject, error) {
	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(req); err != nil {
		return ModuleGitLabProject{}, err
	}

	httpReq, err := client.newRequest(ctx, http.MethodPut, "/api/v1/modules/"+module+"/gitlab-project", &body)
	if err != nil {
		return ModuleGitLabProject{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	var response ModuleGitLabProject
	if err := client.doJSON(httpReq, &response); err != nil {
		return ModuleGitLabProject{}, err
	}
	return response, nil
}

func (client *Client) GetModuleDependencies(ctx context.Context, module string) (ModuleDependencyGraph, error) {
	req, err := client.newRequest(ctx, http.MethodGet, "/api/v1/modules/"+module+"/dependencies", nil)
	if err != nil {
		return ModuleDependencyGraph{}, err
	}

	var response ModuleDependencyGraph
	if err := client.doJSON(req, &response); err != nil {
		return ModuleDependencyGraph{}, err
	}
	return response, nil
}

func (client *Client) GetAffectedModules(ctx context.Context, module string) (AffectedModules, error) {
	req, err := client.newRequest(ctx, http.MethodGet, "/api/v1/modules/"+module+"/affected", nil)
	if err != nil {
		return AffectedModules{}, err
	}

	var response AffectedModules
	if err := client.doJSON(req, &response); err != nil {
		return AffectedModules{}, err
	}
	return response, nil
}

func (client *Client) GetBreakingReportRuntimeImpact(ctx context.Context, reportID string) (BreakingReportRuntimeImpact, error) {
	req, err := client.newRequest(ctx, http.MethodGet, "/api/v1/breaking-reports/"+reportID+"/runtime-impact", nil)
	if err != nil {
		return BreakingReportRuntimeImpact{}, err
	}

	var response BreakingReportRuntimeImpact
	if err := client.doJSON(req, &response); err != nil {
		return BreakingReportRuntimeImpact{}, err
	}
	return response, nil
}

func (client *Client) ReportRuntimeInventory(ctx context.Context, request ReportRuntimeInventoryRequest) (ReportRuntimeInventoryResponse, error) {
	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(request); err != nil {
		return ReportRuntimeInventoryResponse{}, err
	}

	req, err := client.newRequest(ctx, http.MethodPost, "/api/v1/runtime/reports", &body)
	if err != nil {
		return ReportRuntimeInventoryResponse{}, err
	}
	req.Header.Set("Content-Type", "application/json")

	var response ReportRuntimeInventoryResponse
	if err := client.doJSON(req, &response); err != nil {
		return ReportRuntimeInventoryResponse{}, err
	}
	return response, nil
}

func (client *Client) DownloadArtifact(ctx context.Context, module string, version string) (ArtifactDownload, error) {
	req, err := client.newRequest(ctx, http.MethodGet, "/api/v1/modules/"+module+"/versions/"+version+"/artifact", nil)
	if err != nil {
		return ArtifactDownload{}, err
	}

	res, err := client.httpClient.Do(req)
	if err != nil {
		return ArtifactDownload{}, fmt.Errorf("request failed: %w", err)
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		defer res.Body.Close()
		return ArtifactDownload{}, client.responseError(res)
	}

	return ArtifactDownload{
		Body:           res.Body,
		ContentType:    res.Header.Get("Content-Type"),
		Digest:         res.Header.Get("X-ProtoRadar-Digest"),
		ChecksumSHA256: res.Header.Get("X-ProtoRadar-Checksum-SHA256"),
		SizeBytes:      res.ContentLength,
	}, nil
}

func (client *Client) doJSON(req *http.Request, dst any) error {
	res, err := client.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return client.responseError(res)
	}
	return json.NewDecoder(res.Body).Decode(dst)
}

func (client *Client) newRequest(ctx context.Context, method string, path string, body io.Reader) (*http.Request, error) {
	endpoint, err := url.JoinPath(client.baseURL, path)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return nil, err
	}
	if client.token != "" {
		req.Header.Set("Authorization", "Bearer "+client.token)
	}
	return req, nil
}

func (client *Client) responseError(res *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(res.Body, 1024))
	code, text := parseErrorBody(body)
	if client.token != "" {
		text = strings.ReplaceAll(text, client.token, "[redacted]")
	}
	return Error{
		StatusCode: res.StatusCode,
		Status:     res.Status,
		Code:       code,
		Body:       text,
	}
}

func parseErrorBody(body []byte) (string, string) {
	text := strings.TrimSpace(string(body))
	if text == "" {
		return "", ""
	}

	var structured struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &structured); err == nil && structured.Error.Message != "" {
		return structured.Error.Code, structured.Error.Message
	}

	var legacy struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(body, &legacy); err == nil && legacy.Error != "" {
		return "", legacy.Error
	}

	return "", text
}
