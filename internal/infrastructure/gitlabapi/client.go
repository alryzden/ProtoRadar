package gitlabapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/alryzden/ProtoRadar/internal/integration/gitlab"
)

var (
	ErrUnauthorized = errors.New("gitlab unauthorized")
	ErrForbidden    = errors.New("gitlab forbidden")
	ErrNotFound     = errors.New("gitlab resource not found")
	ErrServer       = errors.New("gitlab server error")
	ErrInvalidJSON  = errors.New("gitlab invalid json")
)

const defaultHTTPTimeout = 30 * time.Second

type Error struct {
	StatusCode int
	Status     string
	Body       string
}

func (err Error) Error() string {
	if err.Body == "" {
		return "gitlab returned " + err.Status
	}
	return "gitlab returned " + err.Status + ": " + err.Body
}

func (err Error) Unwrap() error {
	switch err.StatusCode {
	case http.StatusUnauthorized:
		return ErrUnauthorized
	case http.StatusForbidden:
		return ErrForbidden
	case http.StatusNotFound:
		return ErrNotFound
	default:
		if err.StatusCode >= 500 {
			return ErrServer
		}
		return nil
	}
}

type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

func NewClient(baseURL string, token string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultHTTPTimeout}
	}
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		token:      token,
		httpClient: httpClient,
	}
}

func (client *Client) GetMergeRequest(ctx context.Context, projectID int64, mergeRequestIID int64) (gitlab.MergeRequest, error) {
	req, err := client.newRequest(ctx, http.MethodGet, client.mergeRequestPath(projectID, mergeRequestIID), nil)
	if err != nil {
		return gitlab.MergeRequest{}, err
	}

	var response mergeRequestResponse
	if err := client.doJSON(req, &response); err != nil {
		return gitlab.MergeRequest{}, err
	}
	return response.toIntegration(projectID), nil
}

func (client *Client) ListMergeRequestNotes(ctx context.Context, projectID int64, mergeRequestIID int64) ([]gitlab.MergeRequestNote, error) {
	req, err := client.newRequest(ctx, http.MethodGet, client.mergeRequestPath(projectID, mergeRequestIID)+"/notes", nil)
	if err != nil {
		return nil, err
	}

	var response []mergeRequestNoteResponse
	if err := client.doJSON(req, &response); err != nil {
		return nil, err
	}
	notes := make([]gitlab.MergeRequestNote, 0, len(response))
	for _, note := range response {
		notes = append(notes, note.toIntegration())
	}
	return notes, nil
}

func (client *Client) CreateMergeRequestNote(ctx context.Context, projectID int64, mergeRequestIID int64, body string) (gitlab.MergeRequestNote, error) {
	var requestBody bytes.Buffer
	if err := json.NewEncoder(&requestBody).Encode(noteRequest{Body: body}); err != nil {
		return gitlab.MergeRequestNote{}, err
	}

	req, err := client.newRequest(ctx, http.MethodPost, client.mergeRequestPath(projectID, mergeRequestIID)+"/notes", &requestBody)
	if err != nil {
		return gitlab.MergeRequestNote{}, err
	}
	req.Header.Set("Content-Type", "application/json")

	var response mergeRequestNoteResponse
	if err := client.doJSON(req, &response); err != nil {
		return gitlab.MergeRequestNote{}, err
	}
	return response.toIntegration(), nil
}

func (client *Client) UpdateMergeRequestNote(ctx context.Context, projectID int64, mergeRequestIID int64, noteID int64, body string) (gitlab.MergeRequestNote, error) {
	var requestBody bytes.Buffer
	if err := json.NewEncoder(&requestBody).Encode(noteRequest{Body: body}); err != nil {
		return gitlab.MergeRequestNote{}, err
	}

	path := client.mergeRequestPath(projectID, mergeRequestIID) + "/notes/" + strconv.FormatInt(noteID, 10)
	req, err := client.newRequest(ctx, http.MethodPut, path, &requestBody)
	if err != nil {
		return gitlab.MergeRequestNote{}, err
	}
	req.Header.Set("Content-Type", "application/json")

	var response mergeRequestNoteResponse
	if err := client.doJSON(req, &response); err != nil {
		return gitlab.MergeRequestNote{}, err
	}
	return response.toIntegration(), nil
}

func (client *Client) SetCommitStatus(ctx context.Context, projectID int64, sha string, status gitlab.CommitStatus) error {
	params := url.Values{}
	params.Set("state", string(status.State))
	if strings.TrimSpace(status.Name) != "" {
		params.Set("name", status.Name)
	}
	if strings.TrimSpace(status.TargetURL) != "" {
		params.Set("target_url", status.TargetURL)
	}
	if strings.TrimSpace(status.Description) != "" {
		params.Set("description", status.Description)
	}

	path := "/api/v4/projects/" + strconv.FormatInt(projectID, 10) + "/statuses/" + strings.TrimSpace(sha)
	req, err := client.newRequest(ctx, http.MethodPost, path, strings.NewReader(params.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	res, err := client.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("gitlab request failed: %w", err)
	}
	defer closeResponseBody(res.Body)
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return client.responseError(res)
	}
	if err := json.NewDecoder(res.Body).Decode(&commitStatusResponse{}); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidJSON, err)
	}
	return nil
}

func (client *Client) mergeRequestPath(projectID int64, mergeRequestIID int64) string {
	return "/api/v4/projects/" + strconv.FormatInt(projectID, 10) + "/merge_requests/" + strconv.FormatInt(mergeRequestIID, 10)
}

func (client *Client) doJSON(req *http.Request, dst any) error {
	res, err := client.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("gitlab request failed: %w", err)
	}
	defer closeResponseBody(res.Body)

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return client.responseError(res)
	}
	if err := json.NewDecoder(res.Body).Decode(dst); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidJSON, err)
	}
	return nil
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
		req.Header.Set("PRIVATE-TOKEN", client.token)
	}
	return req, nil
}

func (client *Client) responseError(res *http.Response) error {
	body := readLimitedResponseBody(res.Body)
	text := strings.TrimSpace(string(body))
	if client.token != "" {
		text = strings.ReplaceAll(text, client.token, "[redacted]")
	}
	return Error{
		StatusCode: res.StatusCode,
		Status:     res.Status,
		Body:       text,
	}
}

func closeResponseBody(body io.Closer) {
	// Client response bodies are read-only cleanup resources here; the GitLab
	// result is determined by the already-read status/body.
	_ = body.Close() //nolint:errcheck
}

func readLimitedResponseBody(reader io.Reader) []byte {
	body, err := io.ReadAll(io.LimitReader(reader, 1024))
	if err != nil {
		return body
	}
	return body
}

type mergeRequestResponse struct {
	ProjectID    int64  `json:"project_id"`
	IID          int64  `json:"iid"`
	Title        string `json:"title"`
	SourceBranch string `json:"source_branch"`
	TargetBranch string `json:"target_branch"`
	WebURL       string `json:"web_url"`
	SHA          string `json:"sha"`
}

func (response mergeRequestResponse) toIntegration(fallbackProjectID int64) gitlab.MergeRequest {
	projectID := response.ProjectID
	if projectID == 0 {
		projectID = fallbackProjectID
	}
	return gitlab.MergeRequest{
		ProjectID:    projectID,
		IID:          response.IID,
		Title:        response.Title,
		SourceBranch: response.SourceBranch,
		TargetBranch: response.TargetBranch,
		WebURL:       response.WebURL,
		SHA:          response.SHA,
	}
}

type mergeRequestNoteResponse struct {
	ID        int64          `json:"id"`
	Body      string         `json:"body"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	Author    noteAuthorInfo `json:"author"`
}

type noteAuthorInfo struct {
	Username string `json:"username"`
	Name     string `json:"name"`
}

func (response mergeRequestNoteResponse) toIntegration() gitlab.MergeRequestNote {
	return gitlab.MergeRequestNote{
		ID:        response.ID,
		Body:      response.Body,
		CreatedAt: response.CreatedAt,
		UpdatedAt: response.UpdatedAt,
		Author: gitlab.MergeRequestNoteAuthor{
			Username: response.Author.Username,
			Name:     response.Author.Name,
		},
	}
}

type noteRequest struct {
	Body string `json:"body"`
}

type commitStatusResponse struct{}

var _ gitlab.Client = (*Client)(nil)
