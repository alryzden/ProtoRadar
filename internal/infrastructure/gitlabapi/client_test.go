package gitlabapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alryzden/ProtoRadar/internal/integration/gitlab"
)

func TestNewClientUsesFiniteDefaultHTTPTimeout(t *testing.T) {
	client := NewClient("https://gitlab.example.com", "gitlab-token", nil)

	if client.httpClient == nil {
		t.Fatalf("http client was nil")
	}
	if client.httpClient.Timeout <= 0 {
		t.Fatalf("timeout = %s, want finite positive timeout", client.httpClient.Timeout)
	}
}

func TestNewClientPreservesInjectedHTTPClient(t *testing.T) {
	injected := &http.Client{Timeout: 7 * time.Second}

	client := NewClient("https://gitlab.example.com", "gitlab-token", injected)

	if client.httpClient != injected {
		t.Fatalf("injected client was not preserved")
	}
}

func TestClientConstructorDoesNotAssignHTTPDefaultClient(t *testing.T) {
	source := readClientSource(t)

	if strings.Contains(source, "http.DefaultClient") {
		t.Fatalf("client constructor must not assign http.DefaultClient directly")
	}
}

func TestGetMergeRequestSendsRequestAndParsesResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertRequest(t, r, http.MethodGet, "/api/v4/projects/123/merge_requests/7")
		assertToken(t, r)
		writeJSON(t, w, http.StatusOK, map[string]any{
			"project_id":    123,
			"iid":           7,
			"title":         "Update user API",
			"source_branch": "feature/user-api",
			"target_branch": "main",
			"web_url":       "https://gitlab.example.com/platform/user-api/-/merge_requests/7",
			"sha":           "abc123",
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "gitlab-token", server.Client())
	mr, err := client.GetMergeRequest(context.Background(), 123, 7)
	if err != nil {
		t.Fatalf("get merge request: %v", err)
	}

	if mr.ProjectID != 123 || mr.IID != 7 || mr.Title != "Update user API" || mr.SourceBranch != "feature/user-api" || mr.TargetBranch != "main" || mr.SHA != "abc123" {
		t.Fatalf("merge request = %#v", mr)
	}
	if mr.WebURL != "https://gitlab.example.com/platform/user-api/-/merge_requests/7" {
		t.Fatalf("web url = %q", mr.WebURL)
	}
}

func readClientSource(t *testing.T) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(".", "client.go"))
	if err != nil {
		t.Fatalf("read client source: %v", err)
	}
	return string(body)
}

func TestListMergeRequestNotesSendsRequestAndParsesNotes(t *testing.T) {
	created := time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC)
	updated := created.Add(time.Minute)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertRequest(t, r, http.MethodGet, "/api/v4/projects/123/merge_requests/7/notes")
		assertToken(t, r)
		writeJSON(t, w, http.StatusOK, []map[string]any{
			{
				"id":         11,
				"body":       "existing note",
				"created_at": created.Format(time.RFC3339),
				"updated_at": updated.Format(time.RFC3339),
				"author": map[string]any{
					"username": "protoradar-bot",
					"name":     "ProtoRadar Bot",
				},
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "gitlab-token", server.Client())
	notes, err := client.ListMergeRequestNotes(context.Background(), 123, 7)
	if err != nil {
		t.Fatalf("list notes: %v", err)
	}
	if len(notes) != 1 {
		t.Fatalf("notes length = %d", len(notes))
	}
	if notes[0].ID != 11 || notes[0].Body != "existing note" || notes[0].Author.Username != "protoradar-bot" || notes[0].Author.Name != "ProtoRadar Bot" {
		t.Fatalf("note = %#v", notes[0])
	}
	if !notes[0].CreatedAt.Equal(created) || !notes[0].UpdatedAt.Equal(updated) {
		t.Fatalf("note timestamps = %s %s", notes[0].CreatedAt, notes[0].UpdatedAt)
	}
}

func TestCreateMergeRequestNoteSendsBodyAndParsesResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertRequest(t, r, http.MethodPost, "/api/v4/projects/123/merge_requests/7/notes")
		assertToken(t, r)
		if r.Header.Get("Content-Type") != "application/json" {
			t.Fatalf("content type = %q", r.Header.Get("Content-Type"))
		}
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if body["body"] != "new note" {
			t.Fatalf("body = %#v", body)
		}
		writeJSON(t, w, http.StatusCreated, map[string]any{
			"id":   12,
			"body": body["body"],
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "gitlab-token", server.Client())
	note, err := client.CreateMergeRequestNote(context.Background(), 123, 7, "new note")
	if err != nil {
		t.Fatalf("create note: %v", err)
	}
	if note.ID != 12 || note.Body != "new note" {
		t.Fatalf("note = %#v", note)
	}
}

func TestUpdateMergeRequestNoteSendsUpdatedBodyAndParsesResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertRequest(t, r, http.MethodPut, "/api/v4/projects/123/merge_requests/7/notes/12")
		assertToken(t, r)
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if body["body"] != "updated note" {
			t.Fatalf("body = %#v", body)
		}
		writeJSON(t, w, http.StatusOK, map[string]any{
			"id":   12,
			"body": body["body"],
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "gitlab-token", server.Client())
	note, err := client.UpdateMergeRequestNote(context.Background(), 123, 7, 12, "updated note")
	if err != nil {
		t.Fatalf("update note: %v", err)
	}
	if note.ID != 12 || note.Body != "updated note" {
		t.Fatalf("note = %#v", note)
	}
}

func TestSetCommitStatusSendsStatusFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assertRequest(t, r, http.MethodPost, "/api/v4/projects/123/statuses/abc123")
		assertToken(t, r)
		if r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
			t.Fatalf("content type = %q", r.Header.Get("Content-Type"))
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		if r.Form.Get("state") != "success" || r.Form.Get("name") != "protoradar/breaking-check" || r.Form.Get("target_url") != "https://gitlab.example.com/job/1" || r.Form.Get("description") != "No breaking changes found" {
			t.Fatalf("form = %#v", r.Form)
		}
		writeJSON(t, w, http.StatusCreated, map[string]any{"status": "ok"})
	}))
	defer server.Close()

	client := NewClient(server.URL, "gitlab-token", server.Client())
	err := client.SetCommitStatus(context.Background(), 123, "abc123", gitlab.CommitStatus{
		State:       gitlab.CommitStatusStateSuccess,
		Name:        "protoradar/breaking-check",
		TargetURL:   "https://gitlab.example.com/job/1",
		Description: "No breaking changes found",
	})
	if err != nil {
		t.Fatalf("set commit status: %v", err)
	}
}

func TestErrorsMapStatusCodesAndDoNotExposeToken(t *testing.T) {
	tests := []struct {
		name string
		code int
		want error
	}{
		{name: "unauthorized", code: http.StatusUnauthorized, want: ErrUnauthorized},
		{name: "forbidden", code: http.StatusForbidden, want: ErrForbidden},
		{name: "not found", code: http.StatusNotFound, want: ErrNotFound},
		{name: "server", code: http.StatusBadGateway, want: ErrServer},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "request failed for gitlab-token", tt.code)
			}))
			defer server.Close()

			client := NewClient(server.URL, "gitlab-token", server.Client())
			err := client.SetCommitStatus(context.Background(), 123, "abc123", gitlab.CommitStatus{State: gitlab.CommitStatusStateFailed})
			if !errors.Is(err, tt.want) {
				t.Fatalf("error = %v, want %v", err, tt.want)
			}
			if strings.Contains(err.Error(), "gitlab-token") {
				t.Fatalf("token leaked in error: %v", err)
			}
		})
	}
}

func TestInvalidJSONMapsToParseError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "gitlab-token", server.Client())
	_, err := client.GetMergeRequest(context.Background(), 123, 7)
	if !errors.Is(err, ErrInvalidJSON) {
		t.Fatalf("error = %v, want ErrInvalidJSON", err)
	}
}

func TestContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("request should not be sent with a canceled context")
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	client := NewClient(server.URL, "gitlab-token", server.Client())
	_, err := client.GetMergeRequest(ctx, 123, 7)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}

func assertRequest(t *testing.T, r *http.Request, method string, path string) {
	t.Helper()
	if r.Method != method {
		t.Fatalf("method = %q, want %q", r.Method, method)
	}
	if r.URL.Path != path {
		t.Fatalf("path = %q, want %q", r.URL.Path, path)
	}
}

func assertToken(t *testing.T, r *http.Request) {
	t.Helper()
	if r.Header.Get("PRIVATE-TOKEN") != "gitlab-token" {
		t.Fatalf("private token header = %q", r.Header.Get("PRIVATE-TOKEN"))
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
