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
	StorageKey      string    `json:"storage_key"`
	ChecksumSHA256  string    `json:"checksum_sha256"`
	SizeBytes       int64     `json:"size_bytes"`
	CreatedAt       time.Time `json:"created_at"`
}

type PublishModuleVersionResponse struct {
	Version  ModuleVersion `json:"version"`
	Artifact Artifact      `json:"artifact"`
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
		return responseError(res)
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
		return ArtifactDownload{}, responseError(res)
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
		return responseError(res)
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

func responseError(res *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(res.Body, 1024))
	text := strings.TrimSpace(string(body))
	return Error{
		StatusCode: res.StatusCode,
		Status:     res.Status,
		Body:       text,
	}
}
