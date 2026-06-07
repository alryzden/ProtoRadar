package registry

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"io"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/alryzden/ProtoRadar/internal/domain"
	"github.com/alryzden/ProtoRadar/internal/integration/protoradarevents"
	"github.com/alryzden/ProtoRadar/internal/outbox"
	"github.com/alryzden/ProtoRadar/internal/storage"
)

func TestCreateModuleCreatesModuleAndOutboxInTransaction(t *testing.T) {
	fixture := newFixture()

	module, err := fixture.service.CreateModule(context.Background(), CreateModuleRequest{
		Name:          "billing-api",
		Description:   "Billing contracts",
		RepositoryURL: "https://gitlab.example.com/platform/billing-api",
	})
	if err != nil {
		t.Fatalf("create module: %v", err)
	}

	if module.ID.String() != "module-1" {
		t.Fatalf("module id = %q", module.ID)
	}
	if len(fixture.modules.byName) != 1 {
		t.Fatalf("modules stored = %d", len(fixture.modules.byName))
	}
	if len(fixture.outbox.records) != 1 {
		t.Fatalf("outbox records = %d", len(fixture.outbox.records))
	}
	record := fixture.outbox.records[0]
	if record.EventType != "protoradar.module.created" {
		t.Fatalf("event type = %q", record.EventType)
	}
	if record.DedupKey != "module:module-1:created" {
		t.Fatalf("dedup key = %q", record.DedupKey)
	}
}

func TestCreateModuleRollbackPreventsModuleAndOutbox(t *testing.T) {
	fixture := newFixture()
	fixture.outbox.createErr = errors.New("outbox failed")

	_, err := fixture.service.CreateModule(context.Background(), CreateModuleRequest{Name: "billing-api"})
	if err == nil {
		t.Fatalf("expected error")
	}
	if len(fixture.modules.byName) != 0 {
		t.Fatalf("module was not rolled back")
	}
	if len(fixture.outbox.records) != 0 {
		t.Fatalf("outbox record was not rolled back")
	}
}

func TestCreateModuleRejectsDuplicateModule(t *testing.T) {
	fixture := newFixture()

	if _, err := fixture.service.CreateModule(context.Background(), CreateModuleRequest{Name: "billing-api"}); err != nil {
		t.Fatalf("create module: %v", err)
	}
	_, err := fixture.service.CreateModule(context.Background(), CreateModuleRequest{Name: "billing-api"})
	if !errors.Is(err, ErrModuleAlreadyExists) {
		t.Fatalf("error = %v, want ErrModuleAlreadyExists", err)
	}
}

func TestLinkModuleGitLabProjectLinksModule(t *testing.T) {
	fixture := newFixture()
	module := fixture.addModule(t, "billing-api")

	response, err := fixture.service.LinkModuleGitLabProject(context.Background(), LinkModuleGitLabProjectInput{
		ModuleName:        "billing-api",
		GitLabBaseURL:     "https://gitlab.example.com/",
		GitLabProjectID:   123,
		GitLabProjectPath: "platform/billing-api",
	})
	if err != nil {
		t.Fatalf("link gitlab project: %v", err)
	}

	if response.Mapping.ID.String() != "module-gitlab-project-1" {
		t.Fatalf("mapping id = %q", response.Mapping.ID)
	}
	if response.Mapping.ModuleID != module.ID || response.Mapping.ModuleName != module.Name {
		t.Fatalf("mapping module = %#v, want %#v", response.Mapping, module)
	}
	if response.Mapping.GitLabBaseURL != "https://gitlab.example.com" {
		t.Fatalf("gitlab base url = %q", response.Mapping.GitLabBaseURL)
	}
	if response.Mapping.GitLabProjectID != 123 || response.Mapping.GitLabProjectPath != "platform/billing-api" {
		t.Fatalf("gitlab mapping = %#v", response.Mapping)
	}
	if len(fixture.gitLabProjects.byModuleID) != 1 {
		t.Fatalf("gitlab mappings = %d", len(fixture.gitLabProjects.byModuleID))
	}
}

func TestLinkModuleGitLabProjectUpdatesExistingMapping(t *testing.T) {
	fixture := newFixture()
	module := fixture.addModule(t, "billing-api")

	first, err := fixture.service.LinkModuleGitLabProject(context.Background(), LinkModuleGitLabProjectInput{
		ModuleName:        "billing-api",
		GitLabBaseURL:     "https://gitlab.example.com",
		GitLabProjectID:   123,
		GitLabProjectPath: "platform/billing-api",
	})
	if err != nil {
		t.Fatalf("link first gitlab project: %v", err)
	}
	fixture.clock.now = fixture.clock.now.Add(time.Hour)
	second, err := fixture.service.LinkModuleGitLabProject(context.Background(), LinkModuleGitLabProjectInput{
		ModuleName:        "billing-api",
		GitLabBaseURL:     "https://gitlab.example.org",
		GitLabProjectID:   456,
		GitLabProjectPath: "platform/billing-api-v2",
	})
	if err != nil {
		t.Fatalf("link updated gitlab project: %v", err)
	}

	if second.Mapping.ID != first.Mapping.ID {
		t.Fatalf("mapping id = %q, want existing %q", second.Mapping.ID, first.Mapping.ID)
	}
	if second.Mapping.ModuleID != module.ID {
		t.Fatalf("module id = %q, want %q", second.Mapping.ModuleID, module.ID)
	}
	if second.Mapping.GitLabBaseURL != "https://gitlab.example.org" || second.Mapping.GitLabProjectID != 456 || second.Mapping.GitLabProjectPath != "platform/billing-api-v2" {
		t.Fatalf("mapping was not updated: %#v", second.Mapping)
	}
	if len(fixture.gitLabProjects.byModuleID) != 1 {
		t.Fatalf("gitlab mappings = %d", len(fixture.gitLabProjects.byModuleID))
	}
	if len(fixture.outbox.records) != 2 {
		t.Fatalf("outbox records = %d", len(fixture.outbox.records))
	}
}

func TestLinkModuleGitLabProjectRejectsInvalidGitLabBaseURL(t *testing.T) {
	fixture := newFixture()
	fixture.addModule(t, "billing-api")

	_, err := fixture.service.LinkModuleGitLabProject(context.Background(), LinkModuleGitLabProjectInput{
		ModuleName:        "billing-api",
		GitLabBaseURL:     "://not-a-url",
		GitLabProjectID:   123,
		GitLabProjectPath: "platform/billing-api",
	})
	if !errors.Is(err, ErrInvalidGitLabBaseURL) {
		t.Fatalf("error = %v, want ErrInvalidGitLabBaseURL", err)
	}
}

func TestLinkModuleGitLabProjectRejectsNonHTTPGitLabBaseURL(t *testing.T) {
	fixture := newFixture()
	fixture.addModule(t, "billing-api")

	_, err := fixture.service.LinkModuleGitLabProject(context.Background(), LinkModuleGitLabProjectInput{
		ModuleName:        "billing-api",
		GitLabBaseURL:     "ssh://gitlab.example.com",
		GitLabProjectID:   123,
		GitLabProjectPath: "platform/billing-api",
	})
	if !errors.Is(err, ErrInvalidGitLabBaseURL) {
		t.Fatalf("error = %v, want ErrInvalidGitLabBaseURL", err)
	}
}

func TestLinkModuleGitLabProjectRejectsZeroOrNegativeProjectID(t *testing.T) {
	fixture := newFixture()
	fixture.addModule(t, "billing-api")

	for _, projectID := range []int64{0, -1} {
		_, err := fixture.service.LinkModuleGitLabProject(context.Background(), LinkModuleGitLabProjectInput{
			ModuleName:        "billing-api",
			GitLabBaseURL:     "https://gitlab.example.com",
			GitLabProjectID:   projectID,
			GitLabProjectPath: "platform/billing-api",
		})
		if !errors.Is(err, ErrInvalidGitLabProjectID) {
			t.Fatalf("project id %d error = %v, want ErrInvalidGitLabProjectID", projectID, err)
		}
	}
}

func TestLinkModuleGitLabProjectRejectsEmptyProjectPath(t *testing.T) {
	fixture := newFixture()
	fixture.addModule(t, "billing-api")

	_, err := fixture.service.LinkModuleGitLabProject(context.Background(), LinkModuleGitLabProjectInput{
		ModuleName:        "billing-api",
		GitLabBaseURL:     "https://gitlab.example.com",
		GitLabProjectID:   123,
		GitLabProjectPath: " ",
	})
	if !errors.Is(err, ErrInvalidGitLabProjectPath) {
		t.Fatalf("error = %v, want ErrInvalidGitLabProjectPath", err)
	}
}

func TestLinkModuleGitLabProjectReturnsNotFoundForUnknownModule(t *testing.T) {
	fixture := newFixture()

	_, err := fixture.service.LinkModuleGitLabProject(context.Background(), LinkModuleGitLabProjectInput{
		ModuleName:        "missing-api",
		GitLabBaseURL:     "https://gitlab.example.com",
		GitLabProjectID:   123,
		GitLabProjectPath: "platform/missing-api",
	})
	if !errors.Is(err, ErrModuleNotFound) {
		t.Fatalf("error = %v, want ErrModuleNotFound", err)
	}
}

func TestLinkModuleGitLabProjectReturnsConflictWhenGitLabProjectAlreadyLinked(t *testing.T) {
	fixture := newFixture()
	fixture.addModule(t, "billing-api")
	fixture.addModule(t, "orders-api")

	if _, err := fixture.service.LinkModuleGitLabProject(context.Background(), LinkModuleGitLabProjectInput{
		ModuleName:        "billing-api",
		GitLabBaseURL:     "https://gitlab.example.com",
		GitLabProjectID:   123,
		GitLabProjectPath: "platform/billing-api",
	}); err != nil {
		t.Fatalf("link first project: %v", err)
	}
	_, err := fixture.service.LinkModuleGitLabProject(context.Background(), LinkModuleGitLabProjectInput{
		ModuleName:        "orders-api",
		GitLabBaseURL:     "https://gitlab.example.com",
		GitLabProjectID:   123,
		GitLabProjectPath: "platform/billing-api",
	})
	if !errors.Is(err, ErrGitLabProjectAlreadyLinked) {
		t.Fatalf("error = %v, want ErrGitLabProjectAlreadyLinked", err)
	}
	if len(fixture.gitLabProjects.byModuleID) != 1 {
		t.Fatalf("gitlab mappings = %d, want first mapping only", len(fixture.gitLabProjects.byModuleID))
	}
}

func TestLinkModuleGitLabProjectWritesOutboxInMappingTransaction(t *testing.T) {
	fixture := newFixture()
	module := fixture.addModule(t, "billing-api")

	_, err := fixture.service.LinkModuleGitLabProject(context.Background(), LinkModuleGitLabProjectInput{
		ModuleName:        "billing-api",
		GitLabBaseURL:     "https://gitlab.example.com",
		GitLabProjectID:   123,
		GitLabProjectPath: "platform/billing-api",
	})
	if err != nil {
		t.Fatalf("link gitlab project: %v", err)
	}

	if len(fixture.gitLabProjects.byModuleID) != 1 {
		t.Fatalf("gitlab mappings = %d", len(fixture.gitLabProjects.byModuleID))
	}
	if len(fixture.outbox.records) != 1 {
		t.Fatalf("outbox records = %d", len(fixture.outbox.records))
	}
	record := fixture.outbox.records[0]
	if record.EventType != "protoradar.module_gitlab_project.linked" {
		t.Fatalf("event type = %q", record.EventType)
	}
	if record.DedupKey != "module:"+module.ID.String()+":gitlab-project:https://gitlab.example.com:123:linked" {
		t.Fatalf("dedup key = %q", record.DedupKey)
	}
	if record.AggregateID != module.ID.String() {
		t.Fatalf("aggregate id = %q", record.AggregateID)
	}
}

func TestLinkModuleGitLabProjectRollbackPreventsMappingAndOutbox(t *testing.T) {
	fixture := newFixture()
	fixture.addModule(t, "billing-api")
	fixture.outbox.createErr = errors.New("outbox failed")

	_, err := fixture.service.LinkModuleGitLabProject(context.Background(), LinkModuleGitLabProjectInput{
		ModuleName:        "billing-api",
		GitLabBaseURL:     "https://gitlab.example.com",
		GitLabProjectID:   123,
		GitLabProjectPath: "platform/billing-api",
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	if len(fixture.gitLabProjects.byModuleID) != 0 {
		t.Fatalf("mapping was not rolled back")
	}
	if len(fixture.outbox.records) != 0 {
		t.Fatalf("outbox was not rolled back")
	}
}

func TestGetModuleGitLabProjectReturnsExistingMapping(t *testing.T) {
	fixture := newFixture()
	fixture.addModule(t, "billing-api")
	linked, err := fixture.service.LinkModuleGitLabProject(context.Background(), LinkModuleGitLabProjectInput{
		ModuleName:        "billing-api",
		GitLabBaseURL:     "https://gitlab.example.com",
		GitLabProjectID:   123,
		GitLabProjectPath: "platform/billing-api",
	})
	if err != nil {
		t.Fatalf("link gitlab project: %v", err)
	}

	got, err := fixture.service.GetModuleGitLabProject(context.Background(), "billing-api")
	if err != nil {
		t.Fatalf("get gitlab project: %v", err)
	}
	if got.Mapping != linked.Mapping {
		t.Fatalf("mapping = %#v, want %#v", got.Mapping, linked.Mapping)
	}
}

func TestGetModuleGitLabProjectReturnsNotFoundWhenMappingMissing(t *testing.T) {
	fixture := newFixture()
	fixture.addModule(t, "billing-api")

	_, err := fixture.service.GetModuleGitLabProject(context.Background(), "billing-api")
	if !errors.Is(err, ErrModuleGitLabProjectNotFound) {
		t.Fatalf("error = %v, want ErrModuleGitLabProjectNotFound", err)
	}
}

func TestLinkModuleGitLabProjectEventPayloadDoesNotIncludeTokensOrSecrets(t *testing.T) {
	fixture := newFixture()
	fixture.addModule(t, "billing-api")

	_, err := fixture.service.LinkModuleGitLabProject(context.Background(), LinkModuleGitLabProjectInput{
		ModuleName:        "billing-api",
		GitLabBaseURL:     "https://gitlab.example.com",
		GitLabProjectID:   123,
		GitLabProjectPath: "platform/billing-api",
	})
	if err != nil {
		t.Fatalf("link gitlab project: %v", err)
	}
	if len(fixture.outbox.records) != 1 {
		t.Fatalf("outbox records = %d", len(fixture.outbox.records))
	}

	payload := strings.ToLower(string(fixture.outbox.records[0].Payload))
	for _, forbidden := range []string{"token", "secret", "password", "bearer", "raw_token"} {
		if strings.Contains(payload, forbidden) {
			t.Fatalf("payload contains forbidden content %q: %s", forbidden, payload)
		}
	}
	var event map[string]any
	if err := json.Unmarshal(fixture.outbox.records[0].Payload, &event); err != nil {
		t.Fatalf("payload json: %v", err)
	}
	if _, exists := event["gitlab_project_path"]; !exists {
		t.Fatalf("payload missing gitlab_project_path: %#v", event)
	}
}

func TestPublishModuleVersionUploadsAndStoresMetadata(t *testing.T) {
	fixture := newFixture()
	module := fixture.addModule(t, "billing-api")

	response, err := fixture.service.PublishModuleVersion(context.Background(), PublishModuleVersionRequest{
		ModuleName: "billing-api",
		Version:    "v1.0.0",
		Artifact:   bytes.NewReader(validSourceArchive(t)),
	})
	if err != nil {
		t.Fatalf("publish version: %v", err)
	}

	if response.Version.ModuleID != module.ID {
		t.Fatalf("module id = %q", response.Version.ModuleID)
	}
	if response.Version.Version.String() != "v1.0.0" {
		t.Fatalf("version = %q", response.Version.Version)
	}
	if response.SourceArtifact.ModuleVersionID != response.Version.ID {
		t.Fatalf("source artifact module version id = %q", response.SourceArtifact.ModuleVersionID)
	}
	if response.SourceArtifact.Kind != domain.ArtifactKindSourceArchive {
		t.Fatalf("source artifact kind = %q", response.SourceArtifact.Kind)
	}
	if response.BufImageArtifact.Kind != domain.ArtifactKindBufImage {
		t.Fatalf("buf image artifact kind = %q", response.BufImageArtifact.Kind)
	}
	if response.SourceArtifact.StorageKey == "" || response.BufImageArtifact.StorageKey == "" {
		t.Fatalf("artifact keys should be set: %#v %#v", response.SourceArtifact, response.BufImageArtifact)
	}
	if len(fixture.versions.byModuleVersion) != 1 {
		t.Fatalf("versions stored = %d", len(fixture.versions.byModuleVersion))
	}
	if len(fixture.artifacts.byVersionKind) != 2 {
		t.Fatalf("artifacts stored = %d", len(fixture.artifacts.byVersionKind))
	}
}

func TestPublishModuleVersionWritesOutboxInMetadataTransaction(t *testing.T) {
	fixture := newFixture()
	fixture.addModule(t, "billing-api")

	_, err := fixture.service.PublishModuleVersion(context.Background(), PublishModuleVersionRequest{
		ModuleName: "billing-api",
		Version:    "v1.0.0",
		Artifact:   bytes.NewReader(validSourceArchive(t)),
	})
	if err != nil {
		t.Fatalf("publish version: %v", err)
	}

	if len(fixture.outbox.records) != 2 {
		t.Fatalf("outbox records = %d", len(fixture.outbox.records))
	}
	record := requireOutboxRecord(t, fixture.outbox.records, "protoradar.module_version.published")
	if record.EventType != "protoradar.module_version.published" {
		t.Fatalf("event type = %q", record.EventType)
	}
	if record.DedupKey != "module:module-1:version:v1.0.0:published" {
		t.Fatalf("dedup key = %q", record.DedupKey)
	}
	dependenciesRecord := requireOutboxRecord(t, fixture.outbox.records, protoradarevents.EventTypeModuleDependenciesUpdated)
	if dependenciesRecord.DedupKey != "module:module-1:version:v1.0.0:dependencies-updated" {
		t.Fatalf("dependency dedup key = %q", dependenciesRecord.DedupKey)
	}
	if len(fixture.versions.byModuleVersion) != 1 || len(fixture.artifacts.byVersionKind) != 2 || !fixture.dependencies.called {
		t.Fatalf("metadata was not stored with outbox")
	}
}

func TestPublishModuleVersionRejectsUnknownModule(t *testing.T) {
	fixture := newFixture()

	_, err := fixture.service.PublishModuleVersion(context.Background(), PublishModuleVersionRequest{
		ModuleName: "billing-api",
		Version:    "v1.0.0",
		Artifact:   bytes.NewReader(validSourceArchive(t)),
	})
	if !errors.Is(err, ErrModuleNotFound) {
		t.Fatalf("error = %v, want ErrModuleNotFound", err)
	}
}

func TestPublishModuleVersionRejectsDuplicateVersion(t *testing.T) {
	fixture := newFixture()
	fixture.addModule(t, "billing-api")

	req := PublishModuleVersionRequest{
		ModuleName: "billing-api",
		Version:    "v1.0.0",
		Artifact:   bytes.NewReader(validSourceArchive(t)),
	}
	if _, err := fixture.service.PublishModuleVersion(context.Background(), req); err != nil {
		t.Fatalf("publish version: %v", err)
	}
	req.Artifact = bytes.NewReader(validSourceArchive(t))
	_, err := fixture.service.PublishModuleVersion(context.Background(), req)
	if !errors.Is(err, ErrModuleVersionAlreadyExists) {
		t.Fatalf("error = %v, want ErrModuleVersionAlreadyExists", err)
	}
}

func TestDeprecateModuleVersionMarksExistingVersionAndWritesOutbox(t *testing.T) {
	fixture := newFixture()
	module := fixture.addModule(t, "billing-api")
	moduleVersion := fixture.addPublishedVersion(t, module, "v1.0.0", fixture.clock.now.Add(-time.Hour), false)
	fixture.clock.now = fixture.clock.now.Add(time.Hour)

	response, err := fixture.service.DeprecateModuleVersion(context.Background(), DeprecateModuleVersionInput{
		ModuleName: "billing-api",
		Version:    "v1.0.0",
		Actor:      " maintainer@example.com ",
		Reason:     " Use v1.1.0 instead. ",
	})
	if err != nil {
		t.Fatalf("deprecate version: %v", err)
	}

	if response.Version.ID != moduleVersion.ID {
		t.Fatalf("version id = %q, want %q", response.Version.ID, moduleVersion.ID)
	}
	if response.Version.DeprecatedAt == nil || !response.Version.DeprecatedAt.Equal(fixture.clock.now) {
		t.Fatalf("deprecated at = %#v, want %s", response.Version.DeprecatedAt, fixture.clock.now)
	}
	if response.Version.DeprecatedBy != "maintainer@example.com" {
		t.Fatalf("deprecated by = %q", response.Version.DeprecatedBy)
	}
	if response.Version.DeprecationReason != "Use v1.1.0 instead." {
		t.Fatalf("reason = %q", response.Version.DeprecationReason)
	}
	stored, err := fixture.versions.GetByID(context.Background(), moduleVersion.ID)
	if err != nil {
		t.Fatalf("load stored version: %v", err)
	}
	if !stored.IsDeprecated() || stored.DeprecatedBy != response.Version.DeprecatedBy || stored.DeprecationReason != response.Version.DeprecationReason {
		t.Fatalf("stored deprecation = %#v", stored)
	}
	record := requireOutboxRecord(t, fixture.outbox.records, protoradarevents.EventTypeModuleVersionDeprecated)
	if record.DedupKey != "module:"+module.ID.String()+":version:v1.0.0:deprecated" {
		t.Fatalf("dedup key = %q", record.DedupKey)
	}
	var payload protoradarevents.ModuleVersionDeprecatedPayload
	if err := json.Unmarshal(record.Payload, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload.DeprecatedBy != "maintainer@example.com" || payload.DeprecationReason != "Use v1.1.0 instead." || payload.ModuleVersionID != moduleVersion.ID.String() {
		t.Fatalf("payload = %#v", payload)
	}
}

func TestDeprecateModuleVersionReturnsNotFoundForUnknownModuleOrVersion(t *testing.T) {
	fixture := newFixture()
	module := fixture.addModule(t, "billing-api")
	fixture.addPublishedVersion(t, module, "v1.0.0", fixture.clock.now, false)

	_, err := fixture.service.DeprecateModuleVersion(context.Background(), DeprecateModuleVersionInput{
		ModuleName: "missing-api",
		Version:    "v1.0.0",
		Actor:      "ci",
	})
	if !errors.Is(err, ErrModuleNotFound) {
		t.Fatalf("unknown module error = %v, want ErrModuleNotFound", err)
	}

	_, err = fixture.service.DeprecateModuleVersion(context.Background(), DeprecateModuleVersionInput{
		ModuleName: "billing-api",
		Version:    "v9.9.9",
		Actor:      "ci",
	})
	if !errors.Is(err, ErrModuleNotFound) {
		t.Fatalf("unknown version error = %v, want ErrModuleNotFound", err)
	}
}

func TestDeprecateModuleVersionRejectsInvalidInput(t *testing.T) {
	fixture := newFixture()

	tests := []struct {
		name  string
		input DeprecateModuleVersionInput
		want  error
	}{
		{name: "module", input: DeprecateModuleVersionInput{ModuleName: " ", Version: "v1.0.0", Actor: "ci"}, want: ErrInvalidModuleName},
		{name: "version", input: DeprecateModuleVersionInput{ModuleName: "billing-api", Version: " ", Actor: "ci"}, want: ErrInvalidVersion},
		{name: "actor", input: DeprecateModuleVersionInput{ModuleName: "billing-api", Version: "v1.0.0", Actor: " "}, want: ErrInvalidActor},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := fixture.service.DeprecateModuleVersion(context.Background(), tt.input)
			if !errors.Is(err, tt.want) {
				t.Fatalf("error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestDeprecateModuleVersionIsIdempotentWhenAlreadyDeprecated(t *testing.T) {
	fixture := newFixture()
	module := fixture.addModule(t, "billing-api")
	version := fixture.addPublishedVersion(t, module, "v1.0.0", fixture.clock.now, false)
	deprecatedAt := fixture.clock.now.Add(-time.Hour)
	if err := fixture.versions.UpdateDeprecation(context.Background(), version.ID, &deprecatedAt, "first-actor", "first reason"); err != nil {
		t.Fatalf("seed deprecation: %v", err)
	}

	response, err := fixture.service.DeprecateModuleVersion(context.Background(), DeprecateModuleVersionInput{
		ModuleName: "billing-api",
		Version:    "v1.0.0",
		Actor:      "second-actor",
		Reason:     "second reason",
	})
	if err != nil {
		t.Fatalf("deprecate already deprecated: %v", err)
	}
	if response.Version.DeprecatedAt == nil || !response.Version.DeprecatedAt.Equal(deprecatedAt) {
		t.Fatalf("deprecated at = %#v, want existing %s", response.Version.DeprecatedAt, deprecatedAt)
	}
	if response.Version.DeprecatedBy != "first-actor" || response.Version.DeprecationReason != "first reason" {
		t.Fatalf("deprecation was changed: %#v", response.Version)
	}
	if len(fixture.outbox.records) != 0 {
		t.Fatalf("outbox records = %d, want none for idempotent repeat", len(fixture.outbox.records))
	}
}

func TestDeprecateModuleVersionRollbackPreventsPartialDeprecationAndOutbox(t *testing.T) {
	fixture := newFixture()
	module := fixture.addModule(t, "billing-api")
	version := fixture.addPublishedVersion(t, module, "v1.0.0", fixture.clock.now, false)
	fixture.outbox.createErr = errors.New("outbox failed")

	_, err := fixture.service.DeprecateModuleVersion(context.Background(), DeprecateModuleVersionInput{
		ModuleName: "billing-api",
		Version:    "v1.0.0",
		Actor:      "ci",
		Reason:     "Use v1.1.0 instead.",
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	stored, err := fixture.versions.GetByID(context.Background(), version.ID)
	if err != nil {
		t.Fatalf("load stored version: %v", err)
	}
	if stored.IsDeprecated() || len(fixture.outbox.records) != 0 {
		t.Fatalf("rollback failed: stored=%#v outbox=%#v", stored, fixture.outbox.records)
	}
}

func TestPublishModuleVersionRejectsArtifactLargerThanMaxSize(t *testing.T) {
	fixture := newFixture()
	fixture.service.options.MaxArtifactSizeBytes = 4
	fixture.addModule(t, "billing-api")

	_, err := fixture.service.PublishModuleVersion(context.Background(), PublishModuleVersionRequest{
		ModuleName: "billing-api",
		Version:    "v1.0.0",
		Artifact:   bytes.NewReader([]byte("too large")),
	})
	if !errors.Is(err, ErrArtifactTooLarge) {
		t.Fatalf("error = %v, want ErrArtifactTooLarge", err)
	}
	if fixture.store.putKey != "" {
		t.Fatalf("artifact should not have been uploaded")
	}
}

func TestPublishModuleVersionCleansUpStorageWhenTransactionFailsAfterUpload(t *testing.T) {
	fixture := newFixture()
	fixture.addModule(t, "billing-api")
	fixture.outbox.createErr = errors.New("outbox failed")

	_, err := fixture.service.PublishModuleVersion(context.Background(), PublishModuleVersionRequest{
		ModuleName: "billing-api",
		Version:    "v1.0.0",
		Artifact:   bytes.NewReader(validSourceArchive(t)),
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	if len(fixture.store.putKeys) != 2 {
		t.Fatalf("put keys = %#v, want 2 uploads", fixture.store.putKeys)
	}
	if len(fixture.store.deletedKeys) != 2 {
		t.Fatalf("deleted keys = %#v, want cleanup of both uploads", fixture.store.deletedKeys)
	}
	if len(fixture.versions.byModuleVersion) != 0 {
		t.Fatalf("version metadata was not rolled back")
	}
	if len(fixture.artifacts.byVersionKind) != 0 {
		t.Fatalf("artifact metadata was not rolled back")
	}
	if len(fixture.bufConfigs.byVersion) != 0 {
		t.Fatalf("buf config metadata was not rolled back")
	}
	if len(fixture.metadata.byVersion) != 0 {
		t.Fatalf("descriptor metadata was not rolled back")
	}
}

func TestPublishModuleVersionReportsCleanupDeleteFailuresWithoutHidingPrimaryError(t *testing.T) {
	fixture := newFixture()
	fixture.addModule(t, "billing-api")
	primaryErr := errors.New("outbox failed")
	cleanupErr := errors.New("delete failed")
	fixture.outbox.createErr = primaryErr
	fixture.store.deleteErr = cleanupErr

	_, err := fixture.service.PublishModuleVersion(context.Background(), PublishModuleVersionRequest{
		ModuleName: "billing-api",
		Version:    "v1.0.0",
		Artifact:   bytes.NewReader(validSourceArchive(t)),
	})
	if !errors.Is(err, primaryErr) {
		t.Fatalf("error = %v, want primary publish error", err)
	}
	if len(fixture.store.putKeys) != 2 {
		t.Fatalf("put keys = %#v, want 2 uploads", fixture.store.putKeys)
	}
	if len(fixture.store.deletedKeys) != 2 {
		t.Fatalf("deleted keys = %#v, want cleanup attempts for both uploads", fixture.store.deletedKeys)
	}
	if len(fixture.cleanupObserver.failures) != 2 {
		t.Fatalf("cleanup failures = %#v, want 2", fixture.cleanupObserver.failures)
	}
	for _, failure := range fixture.cleanupObserver.failures {
		if failure.StorageKey == "" {
			t.Fatalf("cleanup failure missing storage key: %#v", failure)
		}
		if !errors.Is(failure.Error, cleanupErr) {
			t.Fatalf("cleanup failure error = %v, want cleanup error", failure.Error)
		}
	}
	if len(fixture.store.objects) != 2 {
		t.Fatalf("stored objects = %d, want orphan residue to remain observable", len(fixture.store.objects))
	}
	if len(fixture.versions.byModuleVersion) != 0 || len(fixture.artifacts.byVersionKind) != 0 {
		t.Fatalf("publish metadata was committed despite primary failure")
	}
}

func TestPublishModuleVersionReportsSourceCleanupFailureWhenBufImageUploadFails(t *testing.T) {
	fixture := newFixture()
	fixture.addModule(t, "billing-api")
	cleanupErr := errors.New("delete failed")
	fixture.store.putErrOnCall = 2
	fixture.store.putErr = errors.New("buf image upload failed")
	fixture.store.deleteErr = cleanupErr

	_, err := fixture.service.PublishModuleVersion(context.Background(), PublishModuleVersionRequest{
		ModuleName: "billing-api",
		Version:    "v1.0.0",
		Artifact:   bytes.NewReader(validSourceArchive(t)),
	})
	if !errors.Is(err, ErrStorageFailure) {
		t.Fatalf("error = %v, want ErrStorageFailure", err)
	}
	if len(fixture.store.putKeys) != 2 {
		t.Fatalf("put keys = %#v, want source and failed buf image attempts", fixture.store.putKeys)
	}
	if len(fixture.store.deletedKeys) != 1 {
		t.Fatalf("deleted keys = %#v, want source cleanup attempt", fixture.store.deletedKeys)
	}
	if len(fixture.cleanupObserver.failures) != 1 {
		t.Fatalf("cleanup failures = %#v, want 1", fixture.cleanupObserver.failures)
	}
	if !errors.Is(fixture.cleanupObserver.failures[0].Error, cleanupErr) {
		t.Fatalf("cleanup failure error = %v, want cleanup error", fixture.cleanupObserver.failures[0].Error)
	}
	if len(fixture.versions.byModuleVersion) != 0 || len(fixture.artifacts.byVersionKind) != 0 {
		t.Fatalf("publish metadata was committed despite storage failure")
	}
}

func TestPublishModuleVersionRejectsMissingBufYAMLWhenRequired(t *testing.T) {
	fixture := newFixture()
	fixture.addModule(t, "billing-api")

	_, err := fixture.service.PublishModuleVersion(context.Background(), PublishModuleVersionRequest{
		ModuleName: "billing-api",
		Version:    "v1.0.0",
		Artifact:   bytes.NewReader(sourceArchiveWithoutBufYAML(t)),
	})
	if !errors.Is(err, ErrBufConfigNotFound) {
		t.Fatalf("error = %v, want ErrBufConfigNotFound", err)
	}
	if len(fixture.versions.byModuleVersion) != 0 || len(fixture.outbox.records) != 0 {
		t.Fatalf("publish should not persist version or outbox")
	}
}

func TestPublishModuleVersionRejectsBufBuildFailure(t *testing.T) {
	fixture := newFixture()
	fixture.addModule(t, "billing-api")
	fixture.bufWorkflow.err = errors.New("build failed")
	fixture.bufWorkflow.result = BufWorkflowResult{}

	_, err := fixture.service.PublishModuleVersion(context.Background(), PublishModuleVersionRequest{
		ModuleName: "billing-api",
		Version:    "v1.0.0",
		Artifact:   bytes.NewReader(validSourceArchive(t)),
	})
	if !errors.Is(err, ErrBufBuildFailed) {
		t.Fatalf("error = %v, want ErrBufBuildFailed", err)
	}
	if len(fixture.versions.byModuleVersion) != 0 || len(fixture.outbox.records) != 0 {
		t.Fatalf("publish should not persist version or outbox")
	}
}

func TestPublishModuleVersionAllowsLintWarning(t *testing.T) {
	fixture := newFixture()
	fixture.addModule(t, "billing-api")
	fixture.bufWorkflow.result.LintResult = domain.BufLintResult{Status: domain.BufLintStatusWarning, Report: "lint warning"}

	response, err := fixture.service.PublishModuleVersion(context.Background(), PublishModuleVersionRequest{
		ModuleName: "billing-api",
		Version:    "v1.0.0",
		Artifact:   bytes.NewReader(validSourceArchive(t)),
	})
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	if response.LintResult.Status != domain.BufLintStatusWarning || response.LintResult.Report != "lint warning" {
		t.Fatalf("lint result = %#v", response.LintResult)
	}
	if len(fixture.versions.byModuleVersion) != 1 || len(fixture.outbox.records) != 2 {
		t.Fatalf("publish should persist with lint warning")
	}
}

func TestPublishModuleVersionRejectsLintFailureInEnforceMode(t *testing.T) {
	fixture := newFixture()
	fixture.addModule(t, "billing-api")
	fixture.service.options.BufLintMode = BufLintModeEnforce
	fixture.bufWorkflow.result.LintResult = domain.BufLintResult{Status: domain.BufLintStatusFailed, Report: "lint failed"}
	fixture.bufWorkflow.err = errors.New("lint failed")

	_, err := fixture.service.PublishModuleVersion(context.Background(), PublishModuleVersionRequest{
		ModuleName: "billing-api",
		Version:    "v1.0.0",
		Artifact:   bytes.NewReader(validSourceArchive(t)),
	})
	if !errors.Is(err, ErrBufLintFailed) {
		t.Fatalf("error = %v, want ErrBufLintFailed", err)
	}
	if len(fixture.versions.byModuleVersion) != 0 || len(fixture.outbox.records) != 0 {
		t.Fatalf("publish should not persist version or outbox")
	}
}

func TestPublishModuleVersionStoresDescriptorMetadataAndBufConfig(t *testing.T) {
	fixture := newFixture()
	fixture.addModule(t, "billing-api")

	response, err := fixture.service.PublishModuleVersion(context.Background(), PublishModuleVersionRequest{
		ModuleName: "billing-api",
		Version:    "v1.0.0",
		Artifact:   bytes.NewReader(validSourceArchive(t)),
	})
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	if _, exists := fixture.bufConfigs.byVersion[response.Version.ID.String()]; !exists {
		t.Fatalf("buf config metadata was not saved")
	}
	metadata, exists := fixture.metadata.byVersion[response.Version.ID.String()]
	if !exists {
		t.Fatalf("descriptor metadata was not saved")
	}
	if response.MetadataSummary != metadata.Summary() {
		t.Fatalf("metadata summary = %#v, want %#v", response.MetadataSummary, metadata.Summary())
	}
}

func TestPublishModuleVersionUploadsSourceArchiveAndBufImage(t *testing.T) {
	fixture := newFixture()
	fixture.addModule(t, "billing-api")

	response, err := fixture.service.PublishModuleVersion(context.Background(), PublishModuleVersionRequest{
		ModuleName: "billing-api",
		Version:    "v1.0.0",
		Artifact:   bytes.NewReader(validSourceArchive(t)),
	})
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	if len(fixture.store.putKeys) != 2 {
		t.Fatalf("put keys = %#v, want source and buf image", fixture.store.putKeys)
	}
	if !strings.Contains(response.SourceArtifact.StorageKey, "/source/sha256-") || !strings.HasSuffix(response.SourceArtifact.StorageKey, ".tar.gz") {
		t.Fatalf("source key = %q", response.SourceArtifact.StorageKey)
	}
	if !strings.Contains(response.BufImageArtifact.StorageKey, "/buf-image/sha256-") || !strings.HasSuffix(response.BufImageArtifact.StorageKey, ".binpb") {
		t.Fatalf("buf image key = %q", response.BufImageArtifact.StorageKey)
	}
}

func TestPublishModuleVersionRejectsUnsafeArchive(t *testing.T) {
	fixture := newFixture()
	fixture.addModule(t, "billing-api")

	_, err := fixture.service.PublishModuleVersion(context.Background(), PublishModuleVersionRequest{
		ModuleName: "billing-api",
		Version:    "v1.0.0",
		Artifact:   bytes.NewReader(buildSourceArchive(t, map[string]string{"../evil.proto": "evil"})),
	})
	if !errors.Is(err, ErrUnsafeArchive) {
		t.Fatalf("error = %v, want ErrUnsafeArchive", err)
	}
}

func TestPublishModuleVersionEventDoesNotContainRawToken(t *testing.T) {
	fixture := newFixture()
	fixture.addModule(t, "billing-api")

	if _, err := fixture.service.PublishModuleVersion(context.Background(), PublishModuleVersionRequest{
		ModuleName: "billing-api",
		Version:    "v1.0.0",
		Artifact:   bytes.NewReader(validSourceArchive(t)),
	}); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if len(fixture.outbox.records) != 2 {
		t.Fatalf("outbox records = %d", len(fixture.outbox.records))
	}
	payload := string(requireOutboxRecord(t, fixture.outbox.records, "protoradar.module_version.published").Payload)
	if strings.Contains(payload, "raw-token") || strings.Contains(strings.ToLower(payload), "token") {
		t.Fatalf("event payload contains token data: %s", payload)
	}
	var event map[string]any
	if err := json.Unmarshal(requireOutboxRecord(t, fixture.outbox.records, "protoradar.module_version.published").Payload, &event); err != nil {
		t.Fatalf("event payload json: %v", err)
	}
	if event["lint_status"] != string(domain.BufLintStatusPassed) {
		t.Fatalf("lint_status = %#v", event["lint_status"])
	}
}

func TestPublishModuleVersionComputesImportDependency(t *testing.T) {
	fixture := newFixture()
	fixture.addModule(t, "billing-api")
	provider := fixture.addModule(t, "user-api")
	providerVersion := fixture.addPublishedVersion(t, provider, "v1.0.0", fixture.clock.now.Add(-time.Hour), true)
	fixture.bufWorkflow.result.DescriptorMetadata = domain.DescriptorMetadata{Files: []domain.ProtoFile{{
		Path:    "billing/v1/billing.proto",
		Imports: []domain.ProtoImport{{Path: "user/v1/user.proto"}},
	}}}
	fixture.dependencies.providerFiles = []ProviderFile{{Path: "user/v1/user.proto", ModuleID: provider.ID, ModuleName: provider.Name, ModuleVersionID: providerVersion.ID, Version: providerVersion.Version}}

	if _, err := fixture.service.PublishModuleVersion(context.Background(), PublishModuleVersionRequest{
		ModuleName: "billing-api",
		Version:    "v1.0.0",
		Artifact:   bytes.NewReader(validSourceArchive(t)),
	}); err != nil {
		t.Fatalf("publish: %v", err)
	}

	if len(fixture.dependencies.dependencies) != 1 {
		t.Fatalf("dependencies = %#v", fixture.dependencies.dependencies)
	}
	dependency := fixture.dependencies.dependencies[0]
	if dependency.ProviderModuleID != provider.ID || dependency.Source != domain.DependencySourceImport || dependency.Reason != domain.DependencyResolutionReasonImportPath || dependency.ImportPath != "user/v1/user.proto" {
		t.Fatalf("dependency = %#v", dependency)
	}
}

func TestPublishModuleVersionComputesTypeReferenceDependency(t *testing.T) {
	fixture := newFixture()
	fixture.addModule(t, "billing-api")
	provider := fixture.addModule(t, "user-api")
	providerVersion := fixture.addPublishedVersion(t, provider, "v1.0.0", fixture.clock.now.Add(-time.Hour), true)
	fixture.bufWorkflow.result.DescriptorMetadata = domain.DescriptorMetadata{Files: []domain.ProtoFile{{
		Path: "billing/v1/billing.proto",
		Messages: []domain.ProtoMessage{{
			Name:     "Invoice",
			FullName: "billing.v1.Invoice",
			Fields:   []domain.ProtoField{{Name: "user", Number: 1, TypeName: ".user.v1.User"}},
		}},
	}}}
	fixture.dependencies.symbols = []ProviderSymbol{{FullName: "user.v1.User", PackageName: "user.v1", ModuleID: provider.ID, ModuleName: provider.Name, ModuleVersionID: providerVersion.ID, Version: providerVersion.Version}}

	if _, err := fixture.service.PublishModuleVersion(context.Background(), PublishModuleVersionRequest{
		ModuleName: "billing-api",
		Version:    "v1.0.0",
		Artifact:   bytes.NewReader(validSourceArchive(t)),
	}); err != nil {
		t.Fatalf("publish: %v", err)
	}

	if len(fixture.dependencies.dependencies) != 1 {
		t.Fatalf("dependencies = %#v", fixture.dependencies.dependencies)
	}
	dependency := fixture.dependencies.dependencies[0]
	if dependency.ProviderModuleID != provider.ID || dependency.Source != domain.DependencySourceTypeReference || dependency.Reason != domain.DependencyResolutionReasonSymbol || dependency.ReferencedSymbol != "user.v1.User" {
		t.Fatalf("dependency = %#v", dependency)
	}
}

func TestPublishModuleVersionStoresUnresolvedDependenciesForUnknownImports(t *testing.T) {
	fixture := newFixture()
	fixture.addModule(t, "billing-api")
	fixture.bufWorkflow.result.DescriptorMetadata = domain.DescriptorMetadata{Files: []domain.ProtoFile{{
		Path:    "billing/v1/billing.proto",
		Imports: []domain.ProtoImport{{Path: "missing/v1/missing.proto"}},
	}}}

	if _, err := fixture.service.PublishModuleVersion(context.Background(), PublishModuleVersionRequest{
		ModuleName: "billing-api",
		Version:    "v1.0.0",
		Artifact:   bytes.NewReader(validSourceArchive(t)),
	}); err != nil {
		t.Fatalf("publish with unresolved import: %v", err)
	}

	if len(fixture.dependencies.dependencies) != 0 || len(fixture.dependencies.unresolved) != 1 {
		t.Fatalf("dependencies=%#v unresolved=%#v", fixture.dependencies.dependencies, fixture.dependencies.unresolved)
	}
	if fixture.dependencies.unresolved[0].ImportPath != "missing/v1/missing.proto" || fixture.dependencies.unresolved[0].Reason != domain.UnresolvedDependencyReasonProviderNotFound {
		t.Fatalf("unresolved = %#v", fixture.dependencies.unresolved[0])
	}
}

func TestPublishModuleVersionIgnoresSelfDependencies(t *testing.T) {
	fixture := newFixture()
	fixture.addModule(t, "billing-api")
	fixture.bufWorkflow.result.DescriptorMetadata = domain.DescriptorMetadata{Files: []domain.ProtoFile{
		{Path: "billing/v1/common.proto", Messages: []domain.ProtoMessage{{Name: "Money", FullName: "billing.v1.Money"}}},
		{Path: "billing/v1/billing.proto", Imports: []domain.ProtoImport{{Path: "billing/v1/common.proto"}}, Messages: []domain.ProtoMessage{{Name: "Invoice", FullName: "billing.v1.Invoice", Fields: []domain.ProtoField{{Name: "money", Number: 1, TypeName: ".billing.v1.Money"}}}}},
	}}

	if _, err := fixture.service.PublishModuleVersion(context.Background(), PublishModuleVersionRequest{
		ModuleName: "billing-api",
		Version:    "v1.0.0",
		Artifact:   bytes.NewReader(validSourceArchive(t)),
	}); err != nil {
		t.Fatalf("publish: %v", err)
	}

	if len(fixture.dependencies.dependencies) != 0 || len(fixture.dependencies.unresolved) != 0 {
		t.Fatalf("self dependencies should be ignored, dependencies=%#v unresolved=%#v", fixture.dependencies.dependencies, fixture.dependencies.unresolved)
	}
}

func TestPublishModuleVersionWritesModuleDependenciesUpdatedOutboxRecord(t *testing.T) {
	fixture := newFixture()
	fixture.addModule(t, "billing-api")

	if _, err := fixture.service.PublishModuleVersion(context.Background(), PublishModuleVersionRequest{
		ModuleName: "billing-api",
		Version:    "v1.0.0",
		Artifact:   bytes.NewReader(validSourceArchive(t)),
	}); err != nil {
		t.Fatalf("publish: %v", err)
	}

	record := requireOutboxRecord(t, fixture.outbox.records, protoradarevents.EventTypeModuleDependenciesUpdated)
	if record.DedupKey != "module:module-1:version:v1.0.0:dependencies-updated" {
		t.Fatalf("dedup key = %q", record.DedupKey)
	}
}

func TestPublishModuleVersionTransactionIncludesDependenciesAndOutboxRecords(t *testing.T) {
	fixture := newFixture()
	fixture.addModule(t, "billing-api")

	response, err := fixture.service.PublishModuleVersion(context.Background(), PublishModuleVersionRequest{
		ModuleName: "billing-api",
		Version:    "v1.0.0",
		Artifact:   bytes.NewReader(validSourceArchive(t)),
	})
	if err != nil {
		t.Fatalf("publish: %v", err)
	}

	if _, exists := fixture.versions.byID[response.Version.ID.String()]; !exists {
		t.Fatalf("module version was not stored")
	}
	if _, exists := fixture.metadata.byVersion[response.Version.ID.String()]; !exists {
		t.Fatalf("descriptor metadata was not stored")
	}
	if !fixture.dependencies.called || len(fixture.dependencies.inputs) != 1 {
		t.Fatalf("dependency rebuild was not called")
	}
	if fixture.dependencies.inputs[0].Version.ID != response.Version.ID || fixture.dependencies.inputs[0].Metadata.Summary() != response.MetadataSummary {
		t.Fatalf("dependency rebuild input = %#v", fixture.dependencies.inputs[0])
	}
	requireOutboxRecord(t, fixture.outbox.records, protoradarevents.EventTypeModuleDependenciesUpdated)
	requireOutboxRecord(t, fixture.outbox.records, protoradarevents.EventTypeModuleVersionPublished)
}

func TestPublishModuleVersionRollbackPreventsDependenciesAndOutboxRecords(t *testing.T) {
	fixture := newFixture()
	fixture.addModule(t, "billing-api")
	fixture.outbox.createErr = errors.New("outbox failed")

	_, err := fixture.service.PublishModuleVersion(context.Background(), PublishModuleVersionRequest{
		ModuleName: "billing-api",
		Version:    "v1.0.0",
		Artifact:   bytes.NewReader(validSourceArchive(t)),
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	if len(fixture.versions.byModuleVersion) != 0 || len(fixture.metadata.byVersion) != 0 || len(fixture.dependencies.dependencies) != 0 || len(fixture.dependencies.unresolved) != 0 || len(fixture.outbox.records) != 0 {
		t.Fatalf("rollback failed: versions=%#v metadata=%#v dependencies=%#v unresolved=%#v outbox=%#v", fixture.versions.byModuleVersion, fixture.metadata.byVersion, fixture.dependencies.dependencies, fixture.dependencies.unresolved, fixture.outbox.records)
	}
}

func TestPublishModuleVersionResolverInternalErrorFailsAndRollsBack(t *testing.T) {
	fixture := newFixture()
	fixture.addModule(t, "billing-api")
	fixture.dependencies.err = errors.New("resolver repository failed")

	_, err := fixture.service.PublishModuleVersion(context.Background(), PublishModuleVersionRequest{
		ModuleName: "billing-api",
		Version:    "v1.0.0",
		Artifact:   bytes.NewReader(validSourceArchive(t)),
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	if len(fixture.versions.byModuleVersion) != 0 || len(fixture.metadata.byVersion) != 0 || len(fixture.dependencies.dependencies) != 0 || len(fixture.dependencies.unresolved) != 0 || len(fixture.outbox.records) != 0 {
		t.Fatalf("rollback failed after resolver error")
	}
}

func TestCheckBreakingPassedStoresReportAndOutbox(t *testing.T) {
	fixture := newFixture()
	module := fixture.addModule(t, "billing-api")
	baseline := fixture.addPublishedVersion(t, module, "v1.0.0", fixture.clock.now.Add(-time.Hour), true)

	response, err := fixture.service.CheckBreaking(context.Background(), CheckBreakingRequest{
		ModuleName:            "billing-api",
		Against:               "v1.0.0",
		TargetRef:             "local",
		ProposedSourceArchive: bytes.NewReader(validSourceArchive(t)),
	})
	if err != nil {
		t.Fatalf("check breaking: %v", err)
	}

	if response.Report.Status != domain.BreakingReportStatusPassed {
		t.Fatalf("status = %q", response.Report.Status)
	}
	if response.Report.BaseVersionID != baseline.ID {
		t.Fatalf("base version id = %q, want %q", response.Report.BaseVersionID, baseline.ID)
	}
	if len(fixture.reports.byID) != 1 {
		t.Fatalf("reports stored = %d", len(fixture.reports.byID))
	}
	if len(fixture.outbox.records) != 1 {
		t.Fatalf("outbox records = %d", len(fixture.outbox.records))
	}
	record := fixture.outbox.records[0]
	if record.EventType != "protoradar.breaking_report.created" {
		t.Fatalf("event type = %q", record.EventType)
	}
	if record.DedupKey != "breaking-report:breaking-report-1:created" {
		t.Fatalf("dedup key = %q", record.DedupKey)
	}
}

func TestCheckBreakingBreakingStoresReportAndChanges(t *testing.T) {
	fixture := newFixture()
	module := fixture.addModule(t, "billing-api")
	fixture.addPublishedVersion(t, module, "v1.0.0", fixture.clock.now.Add(-time.Hour), true)
	fixture.bufBreaking.result = BufBreakingCheckResult{
		Status:    domain.BreakingReportStatusBreaking,
		RawOutput: "user/v1/user.proto:FIELD_SAME_TYPE: field type changed",
		Changes: []domain.BreakingChange{
			{FilePath: "user/v1/user.proto", RuleID: "FIELD_SAME_TYPE", Message: "field type changed"},
		},
	}

	response, err := fixture.service.CheckBreaking(context.Background(), CheckBreakingRequest{
		ModuleName:            "billing-api",
		Against:               "v1.0.0",
		ProposedSourceArchive: bytes.NewReader(validSourceArchive(t)),
	})
	if err != nil {
		t.Fatalf("breaking result should not be internal error: %v", err)
	}
	if response.Report.Status != domain.BreakingReportStatusBreaking {
		t.Fatalf("status = %q", response.Report.Status)
	}
	if response.Report.ChangeCount != 1 {
		t.Fatalf("change count = %d", response.Report.ChangeCount)
	}
	if len(response.Changes) != 1 {
		t.Fatalf("changes = %d", len(response.Changes))
	}
	change := response.Changes[0]
	if change.ID == "" || change.ReportID != response.Report.ID || change.Severity != "error" {
		t.Fatalf("change metadata = %#v", change)
	}
}

func TestCheckBreakingLatestResolvesLatestPublishedVersion(t *testing.T) {
	fixture := newFixture()
	module := fixture.addModule(t, "billing-api")
	fixture.addPublishedVersion(t, module, "v1.0.0", fixture.clock.now.Add(-2*time.Hour), true)
	latest := fixture.addPublishedVersion(t, module, "v1.1.0", fixture.clock.now.Add(-time.Hour), true)

	response, err := fixture.service.CheckBreaking(context.Background(), CheckBreakingRequest{
		ModuleName:            "billing-api",
		Against:               "latest",
		ProposedSourceArchive: bytes.NewReader(validSourceArchive(t)),
	})
	if err != nil {
		t.Fatalf("check breaking: %v", err)
	}
	if response.Report.BaseVersionID != latest.ID {
		t.Fatalf("base version id = %q, want latest %q", response.Report.BaseVersionID, latest.ID)
	}
	if len(fixture.bufBreaking.calls) != 1 || !bytes.Equal(fixture.bufBreaking.calls[0].BaselineImage, []byte("baseline image v1.1.0")) {
		t.Fatalf("baseline image call = %#v", fixture.bufBreaking.calls)
	}
}

func TestCheckBreakingExplicitVersionResolvesExactVersion(t *testing.T) {
	fixture := newFixture()
	module := fixture.addModule(t, "billing-api")
	exact := fixture.addPublishedVersion(t, module, "v1.0.0", fixture.clock.now.Add(-2*time.Hour), true)
	fixture.addPublishedVersion(t, module, "v1.1.0", fixture.clock.now.Add(-time.Hour), true)

	response, err := fixture.service.CheckBreaking(context.Background(), CheckBreakingRequest{
		ModuleName:            "billing-api",
		Against:               "v1.0.0",
		ProposedSourceArchive: bytes.NewReader(validSourceArchive(t)),
	})
	if err != nil {
		t.Fatalf("check breaking: %v", err)
	}
	if response.Report.BaseVersionID != exact.ID {
		t.Fatalf("base version id = %q, want exact %q", response.Report.BaseVersionID, exact.ID)
	}
}

func TestCheckBreakingRejectsUnknownModule(t *testing.T) {
	fixture := newFixture()

	_, err := fixture.service.CheckBreaking(context.Background(), CheckBreakingRequest{
		ModuleName:            "billing-api",
		Against:               "latest",
		ProposedSourceArchive: bytes.NewReader(validSourceArchive(t)),
	})
	if !errors.Is(err, ErrModuleNotFound) {
		t.Fatalf("error = %v, want ErrModuleNotFound", err)
	}
}

func TestCheckBreakingRejectsInvalidAgainstValue(t *testing.T) {
	fixture := newFixture()
	fixture.service.options.BreakingDefaultAgainst = ""

	_, err := fixture.service.CheckBreaking(context.Background(), CheckBreakingRequest{
		ModuleName:            "billing-api",
		Against:               " ",
		ProposedSourceArchive: bytes.NewReader(validSourceArchive(t)),
	})
	if !errors.Is(err, ErrInvalidAgainst) {
		t.Fatalf("error = %v, want ErrInvalidAgainst", err)
	}
}

func TestCheckBreakingRejectsMissingArchive(t *testing.T) {
	fixture := newFixture()

	_, err := fixture.service.CheckBreaking(context.Background(), CheckBreakingRequest{
		ModuleName: "billing-api",
		Against:    "latest",
	})
	if !errors.Is(err, ErrArtifactRequired) {
		t.Fatalf("error = %v, want ErrArtifactRequired", err)
	}
}

func TestCheckBreakingRejectsUnknownBaseline(t *testing.T) {
	fixture := newFixture()
	fixture.addModule(t, "billing-api")

	_, err := fixture.service.CheckBreaking(context.Background(), CheckBreakingRequest{
		ModuleName:            "billing-api",
		Against:               "latest",
		ProposedSourceArchive: bytes.NewReader(validSourceArchive(t)),
	})
	if !errors.Is(err, ErrBaselineVersionNotFound) {
		t.Fatalf("error = %v, want ErrBaselineVersionNotFound", err)
	}
}

func TestCheckBreakingRejectsBaselineWithoutBufImage(t *testing.T) {
	fixture := newFixture()
	module := fixture.addModule(t, "billing-api")
	fixture.addPublishedVersion(t, module, "v1.0.0", fixture.clock.now.Add(-time.Hour), false)

	_, err := fixture.service.CheckBreaking(context.Background(), CheckBreakingRequest{
		ModuleName:            "billing-api",
		Against:               "v1.0.0",
		ProposedSourceArchive: bytes.NewReader(validSourceArchive(t)),
	})
	if !errors.Is(err, ErrBaselineBufImageMissing) {
		t.Fatalf("error = %v, want ErrBaselineBufImageMissing", err)
	}
}

func TestCheckBreakingRejectsUnsafeArchive(t *testing.T) {
	fixture := newFixture()
	module := fixture.addModule(t, "billing-api")
	fixture.addPublishedVersion(t, module, "v1.0.0", fixture.clock.now.Add(-time.Hour), true)

	_, err := fixture.service.CheckBreaking(context.Background(), CheckBreakingRequest{
		ModuleName:            "billing-api",
		Against:               "v1.0.0",
		ProposedSourceArchive: bytes.NewReader(buildSourceArchive(t, map[string]string{"../evil.proto": "evil"})),
	})
	if !errors.Is(err, ErrUnsafeArchive) {
		t.Fatalf("error = %v, want ErrUnsafeArchive", err)
	}
}

func TestCheckBreakingRejectsMissingBufYAML(t *testing.T) {
	fixture := newFixture()
	module := fixture.addModule(t, "billing-api")
	fixture.addPublishedVersion(t, module, "v1.0.0", fixture.clock.now.Add(-time.Hour), true)

	_, err := fixture.service.CheckBreaking(context.Background(), CheckBreakingRequest{
		ModuleName:            "billing-api",
		Against:               "v1.0.0",
		ProposedSourceArchive: bytes.NewReader(sourceArchiveWithoutBufYAML(t)),
	})
	if !errors.Is(err, ErrBufConfigNotFound) {
		t.Fatalf("error = %v, want ErrBufConfigNotFound", err)
	}
}

func TestCheckBreakingRejectsArtifactTooLarge(t *testing.T) {
	fixture := newFixture()
	module := fixture.addModule(t, "billing-api")
	fixture.addPublishedVersion(t, module, "v1.0.0", fixture.clock.now.Add(-time.Hour), true)
	fixture.service.options.MaxArtifactSizeBytes = 4

	_, err := fixture.service.CheckBreaking(context.Background(), CheckBreakingRequest{
		ModuleName:            "billing-api",
		Against:               "v1.0.0",
		ArchiveSizeBytes:      5,
		ProposedSourceArchive: bytes.NewReader(validSourceArchive(t)),
	})
	if !errors.Is(err, ErrArtifactTooLarge) {
		t.Fatalf("error = %v, want ErrArtifactTooLarge", err)
	}
}

func TestCheckBreakingToolFailureDoesNotStoreReport(t *testing.T) {
	fixture := newFixture()
	module := fixture.addModule(t, "billing-api")
	fixture.addPublishedVersion(t, module, "v1.0.0", fixture.clock.now.Add(-time.Hour), true)
	fixture.bufBreaking.err = errors.New("buf failed")
	fixture.bufBreaking.result = BufBreakingCheckResult{Status: domain.BreakingReportStatusFailed}

	_, err := fixture.service.CheckBreaking(context.Background(), CheckBreakingRequest{
		ModuleName:            "billing-api",
		Against:               "v1.0.0",
		ProposedSourceArchive: bytes.NewReader(validSourceArchive(t)),
	})
	if !errors.Is(err, ErrBufBreakingFailed) {
		t.Fatalf("error = %v, want ErrBufBreakingFailed", err)
	}
	if len(fixture.reports.byID) != 0 || len(fixture.outbox.records) != 0 {
		t.Fatalf("tool failure should not store report or outbox")
	}
}

func TestCheckBreakingRollbackPreventsReportChangesAndOutbox(t *testing.T) {
	fixture := newFixture()
	module := fixture.addModule(t, "billing-api")
	fixture.addPublishedVersion(t, module, "v1.0.0", fixture.clock.now.Add(-time.Hour), true)
	fixture.outbox.createErr = errors.New("outbox failed")
	fixture.bufBreaking.result = BufBreakingCheckResult{
		Status:  domain.BreakingReportStatusBreaking,
		Changes: []domain.BreakingChange{{FilePath: "user.proto", RuleID: "FIELD_NO_DELETE", Message: "field deleted"}},
	}

	_, err := fixture.service.CheckBreaking(context.Background(), CheckBreakingRequest{
		ModuleName:            "billing-api",
		Against:               "v1.0.0",
		ProposedSourceArchive: bytes.NewReader(validSourceArchive(t)),
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	if len(fixture.reports.byID) != 0 {
		t.Fatalf("report was not rolled back")
	}
	if len(fixture.reports.changes) != 0 {
		t.Fatalf("changes were not rolled back")
	}
	if len(fixture.outbox.records) != 0 {
		t.Fatalf("outbox was not rolled back")
	}
}

func TestCheckBreakingEventDoesNotContainRawTokenOrArchiveContents(t *testing.T) {
	fixture := newFixture()
	module := fixture.addModule(t, "billing-api")
	fixture.addPublishedVersion(t, module, "v1.0.0", fixture.clock.now.Add(-time.Hour), true)
	archive := buildSourceArchive(t, map[string]string{
		"buf.yaml":     "version: v2\n",
		"secret.proto": "syntax = \"proto3\"; // raw-token archive-secret",
	})

	if _, err := fixture.service.CheckBreaking(context.Background(), CheckBreakingRequest{
		ModuleName:            "billing-api",
		Against:               "v1.0.0",
		TargetRef:             "local",
		ProposedSourceArchive: bytes.NewReader(archive),
	}); err != nil {
		t.Fatalf("check breaking: %v", err)
	}
	payload := string(fixture.outbox.records[0].Payload)
	if strings.Contains(payload, "archive-secret") {
		t.Fatalf("event payload contains archive contents: %s", payload)
	}
	if strings.Contains(payload, "raw-token") {
		t.Fatalf("event payload contains raw token data: %s", payload)
	}
}

func TestDownloadArtifactReturnsStreamAndMetadata(t *testing.T) {
	fixture := newFixture()
	fixture.addModule(t, "billing-api")
	archive := validSourceArchive(t)
	response, err := fixture.service.PublishModuleVersion(context.Background(), PublishModuleVersionRequest{
		ModuleName: "billing-api",
		Version:    "v1.0.0",
		Artifact:   bytes.NewReader(archive),
	})
	if err != nil {
		t.Fatalf("publish version: %v", err)
	}

	object, gotArtifact, err := fixture.service.DownloadArtifact(context.Background(), "billing-api", "v1.0.0")
	if err != nil {
		t.Fatalf("download: %v", err)
	}
	defer object.Body.Close()

	if gotArtifact.ID != response.SourceArtifact.ID {
		t.Fatalf("artifact id = %q, want %q", gotArtifact.ID, response.SourceArtifact.ID)
	}
	body, err := io.ReadAll(object.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if !bytes.Equal(body, archive) {
		t.Fatalf("downloaded source archive did not match uploaded archive")
	}
}

func TestCreateAPITokenReturnsRawTokenOnceAndStoresOnlyHash(t *testing.T) {
	fixture := newFixture()
	fixture.tokenGenerator.next = "raw-token"

	response, err := fixture.service.CreateAPIToken(context.Background(), CreateAPITokenRequest{Name: "ci"})
	if err != nil {
		t.Fatalf("create token: %v", err)
	}

	if response.RawToken != "raw-token" {
		t.Fatalf("raw token = %q", response.RawToken)
	}
	if response.Token.TokenHash == "raw-token" {
		t.Fatalf("stored raw token instead of hash")
	}
	stored := fixture.tokens.byHash[response.Token.TokenHash]
	if stored.TokenHash == "" {
		t.Fatalf("token hash was not stored")
	}
	if slices.Contains([]string{stored.TokenHash, response.Token.TokenHash}, "raw-token") {
		t.Fatalf("raw token leaked into stored token")
	}
}

func TestAuthenticateTokenAcceptsValidAndRejectsInvalidOrExpiredToken(t *testing.T) {
	fixture := newFixture()
	fixture.tokenGenerator.next = "raw-token"
	response, err := fixture.service.CreateAPIToken(context.Background(), CreateAPITokenRequest{Name: "ci"})
	if err != nil {
		t.Fatalf("create token: %v", err)
	}

	subject, err := fixture.service.AuthenticateToken(context.Background(), "raw-token")
	if err != nil {
		t.Fatalf("authenticate: %v", err)
	}
	if subject.TokenID != response.Token.ID {
		t.Fatalf("subject token id = %q", subject.TokenID)
	}
	if fixture.tokens.lastUsed[response.Token.ID.String()].IsZero() {
		t.Fatalf("last_used_at was not updated")
	}

	if _, err := fixture.service.AuthenticateToken(context.Background(), "wrong-token"); !errors.Is(err, ErrInvalidOrExpiredToken) {
		t.Fatalf("invalid token error = %v", err)
	}

	expiredAt := fixture.clock.now.Add(-time.Hour)
	fixture.tokenGenerator.next = "expired-token"
	if _, err := fixture.service.CreateAPIToken(context.Background(), CreateAPITokenRequest{Name: "expired", ExpiresAt: &expiredAt}); err != nil {
		t.Fatalf("create expired token: %v", err)
	}
	if _, err := fixture.service.AuthenticateToken(context.Background(), "expired-token"); !errors.Is(err, ErrInvalidOrExpiredToken) {
		t.Fatalf("expired token error = %v", err)
	}
}

func TestAuthenticateTokenSucceedsWhenMarkUsedFailsAndReportsFailure(t *testing.T) {
	fixture := newFixture()
	fixture.tokenGenerator.next = "raw-token"
	response, err := fixture.service.CreateAPIToken(context.Background(), CreateAPITokenRequest{Name: "ci"})
	if err != nil {
		t.Fatalf("create token: %v", err)
	}
	markErr := errors.New("mark used failed")
	fixture.tokens.markUsedErr = markErr

	subject, err := fixture.service.AuthenticateToken(context.Background(), "raw-token")
	if err != nil {
		t.Fatalf("authenticate: %v", err)
	}
	if subject.TokenID != response.Token.ID || subject.Name != "ci" {
		t.Fatalf("subject = %#v", subject)
	}
	if len(fixture.tokenUsageObserver.failures) != 1 {
		t.Fatalf("token usage failures = %#v, want 1", fixture.tokenUsageObserver.failures)
	}
	failure := fixture.tokenUsageObserver.failures[0]
	if failure.TokenID != response.Token.ID.String() {
		t.Fatalf("token id = %q, want %q", failure.TokenID, response.Token.ID)
	}
	if !errors.Is(failure.Error, markErr) {
		t.Fatalf("failure error = %v, want mark error", failure.Error)
	}
	if strings.Contains(failure.TokenID, "raw-token") {
		t.Fatalf("observer leaked raw token: %#v", failure)
	}
}

type fixture struct {
	service            *Service
	modules            *fakeModules
	gitLabProjects     *fakeGitLabProjects
	versions           *fakeVersions
	artifacts          *fakeArtifacts
	bufConfigs         *fakeBufConfigs
	metadata           *fakeMetadata
	reports            *fakeReports
	tokens             *fakeTokens
	dependencies       *fakeDependencyRebuilder
	transactions       *fakeTransactions
	outbox             *fakeOutbox
	store              *fakeArtifactStore
	bufWorkflow        *fakeBufWorkflow
	bufBreaking        *fakeBufBreaking
	clock              *fakeClock
	ids                *fakeIDs
	tokenGenerator     *fakeTokenGenerator
	cleanupObserver    *fakeArtifactCleanupObserver
	tokenUsageObserver *fakeTokenUsageObserver
}

func newFixture() *fixture {
	modules := newFakeModules()
	gitLabProjects := newFakeGitLabProjects()
	versions := newFakeVersions()
	artifacts := newFakeArtifacts()
	bufConfigs := newFakeBufConfigs()
	metadata := newFakeMetadata()
	reports := newFakeReports()
	tokens := newFakeTokens()
	outboxWriter := &fakeOutbox{}
	store := &fakeArtifactStore{objects: map[string][]byte{}}
	cleanupObserver := &fakeArtifactCleanupObserver{}
	tokenUsageObserver := &fakeTokenUsageObserver{}
	bufWorkflow := &fakeBufWorkflow{result: successfulBufWorkflowResult()}
	bufBreaking := &fakeBufBreaking{result: BufBreakingCheckResult{Status: domain.BreakingReportStatusPassed, RawOutput: "no breaking changes"}}
	clock := &fakeClock{now: time.Date(2026, 6, 4, 12, 0, 0, 0, time.UTC)}
	dependencies := &fakeDependencyRebuilder{outbox: outboxWriter, clock: clock}
	ids := &fakeIDs{}
	tokenGenerator := &fakeTokenGenerator{next: "raw-token"}
	transactions := &fakeTransactions{
		modules:        modules,
		gitLabProjects: gitLabProjects,
		versions:       versions,
		artifacts:      artifacts,
		bufConfigs:     bufConfigs,
		metadata:       metadata,
		reports:        reports,
		tokens:         tokens,
		dependencies:   dependencies,
		outbox:         outboxWriter,
	}

	return &fixture{
		service: NewService(
			modules,
			gitLabProjects,
			versions,
			artifacts,
			bufConfigs,
			metadata,
			reports,
			tokens,
			dependencies,
			nil,
			transactions,
			outboxWriter,
			store,
			bufWorkflow,
			bufBreaking,
			clock,
			ids,
			tokenGenerator,
			Options{MaxArtifactSizeBytes: 4096, MaxSourceUncompressedSizeBytes: 4096, TokenHashSecret: "hash-secret", BufRequireConfig: true, BufLintMode: BufLintModeWarn, BreakingMaxChanges: 1000, BreakingDefaultAgainst: "latest", ArtifactCleanupObserver: cleanupObserver, TokenUsageObserver: tokenUsageObserver},
		),
		modules:            modules,
		gitLabProjects:     gitLabProjects,
		versions:           versions,
		artifacts:          artifacts,
		bufConfigs:         bufConfigs,
		metadata:           metadata,
		reports:            reports,
		tokens:             tokens,
		dependencies:       dependencies,
		transactions:       transactions,
		outbox:             outboxWriter,
		store:              store,
		bufWorkflow:        bufWorkflow,
		bufBreaking:        bufBreaking,
		clock:              clock,
		ids:                ids,
		tokenGenerator:     tokenGenerator,
		cleanupObserver:    cleanupObserver,
		tokenUsageObserver: tokenUsageObserver,
	}
}

func (fixture *fixture) addModule(t *testing.T, nameValue string) domain.Module {
	t.Helper()

	name, err := domain.NewModuleName(nameValue)
	if err != nil {
		t.Fatalf("module name: %v", err)
	}
	module := domain.Module{
		ID:        fixture.nextModuleID(t),
		Name:      name,
		CreatedAt: fixture.clock.now,
		UpdatedAt: fixture.clock.now,
	}
	if err := fixture.modules.Create(context.Background(), module); err != nil {
		t.Fatalf("create fake module: %v", err)
	}
	return module
}

func (fixture *fixture) addPublishedVersion(t *testing.T, module domain.Module, versionValue string, createdAt time.Time, withBufImage bool) domain.ModuleVersion {
	t.Helper()

	version, err := domain.NewVersion(versionValue)
	if err != nil {
		t.Fatalf("version: %v", err)
	}
	moduleVersion := domain.ModuleVersion{
		ID:        domain.NewModuleVersionID("module-version-" + versionValue),
		ModuleID:  module.ID,
		Version:   version,
		Status:    domain.ModuleVersionStatusPublished,
		Digest:    "sha256:" + versionValue,
		CreatedAt: createdAt,
	}
	if err := fixture.versions.Create(context.Background(), moduleVersion); err != nil {
		t.Fatalf("create fake version: %v", err)
	}
	if withBufImage {
		key := "modules/" + module.Name.String() + "/versions/" + versionValue + "/buf-image/sha256-baseline.binpb"
		artifact := domain.Artifact{
			ID:              domain.NewArtifactID("artifact-" + versionValue),
			ModuleVersionID: moduleVersion.ID,
			Kind:            domain.ArtifactKindBufImage,
			StorageKey:      key,
			ChecksumSHA256:  "baseline-" + versionValue,
			SizeBytes:       int64(len("baseline image " + versionValue)),
			CreatedAt:       createdAt,
		}
		if err := fixture.artifacts.Create(context.Background(), artifact); err != nil {
			t.Fatalf("create fake artifact: %v", err)
		}
		fixture.store.objects[key] = []byte("baseline image " + versionValue)
	}
	return moduleVersion
}

type fakeClock struct {
	now time.Time
}

func (clock *fakeClock) Now() time.Time {
	return clock.now
}

type fakeIDs struct {
	moduleID              int
	moduleGitLabProjectID int
	moduleVersionID       int
	artifactID            int
	apiTokenID            int
	reportID              int
	changeID              int
}

func (fixture *fixture) nextModuleID(t *testing.T) domain.ModuleID {
	t.Helper()
	id, err := fixture.ids.NewModuleID()
	if err != nil {
		t.Fatalf("module id: %v", err)
	}
	return id
}

func (ids *fakeIDs) NewModuleID() (domain.ModuleID, error) {
	ids.moduleID++
	return domain.NewModuleID("module-" + strconv.Itoa(ids.moduleID)), nil
}

func (ids *fakeIDs) NewModuleGitLabProjectID() (domain.ModuleGitLabProjectID, error) {
	ids.moduleGitLabProjectID++
	return domain.NewModuleGitLabProjectID("module-gitlab-project-" + strconv.Itoa(ids.moduleGitLabProjectID)), nil
}

func (ids *fakeIDs) NewModuleVersionID() (domain.ModuleVersionID, error) {
	ids.moduleVersionID++
	return domain.NewModuleVersionID("module-version-" + strconv.Itoa(ids.moduleVersionID)), nil
}

func (ids *fakeIDs) NewArtifactID() (domain.ArtifactID, error) {
	ids.artifactID++
	return domain.NewArtifactID("artifact-" + strconv.Itoa(ids.artifactID)), nil
}

func (ids *fakeIDs) NewAPITokenID() (domain.APITokenID, error) {
	ids.apiTokenID++
	return domain.NewAPITokenID("api-token-" + strconv.Itoa(ids.apiTokenID)), nil
}

func (ids *fakeIDs) NewBreakingReportID() (domain.BreakingReportID, error) {
	ids.reportID++
	return domain.NewBreakingReportID("breaking-report-" + strconv.Itoa(ids.reportID)), nil
}

func (ids *fakeIDs) NewBreakingChangeID() (domain.BreakingChangeID, error) {
	ids.changeID++
	return domain.NewBreakingChangeID("breaking-change-" + strconv.Itoa(ids.changeID)), nil
}

type fakeTokenGenerator struct {
	next string
}

func (generator *fakeTokenGenerator) NewToken() (string, error) {
	return generator.next, nil
}

type fakeTransactions struct {
	modules        *fakeModules
	gitLabProjects *fakeGitLabProjects
	versions       *fakeVersions
	artifacts      *fakeArtifacts
	bufConfigs     *fakeBufConfigs
	metadata       *fakeMetadata
	reports        *fakeReports
	tokens         *fakeTokens
	dependencies   *fakeDependencyRebuilder
	outbox         *fakeOutbox
}

func (tx *fakeTransactions) WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	modules := tx.modules.snapshot()
	gitLabProjects := tx.gitLabProjects.snapshot()
	versions := tx.versions.snapshot()
	artifacts := tx.artifacts.snapshot()
	bufConfigs := tx.bufConfigs.snapshot()
	metadata := tx.metadata.snapshot()
	reports := tx.reports.snapshot()
	tokens := tx.tokens.snapshot()
	dependencies := tx.dependencies.snapshot()
	outboxRecords := slices.Clone(tx.outbox.records)

	if err := fn(ctx); err != nil {
		tx.modules.restore(modules)
		tx.gitLabProjects.restore(gitLabProjects)
		tx.versions.restore(versions)
		tx.artifacts.restore(artifacts)
		tx.bufConfigs.restore(bufConfigs)
		tx.metadata.restore(metadata)
		tx.reports.restore(reports)
		tx.tokens.restore(tokens)
		tx.dependencies.restore(dependencies)
		tx.outbox.records = outboxRecords
		return err
	}
	return nil
}

type fakeModules struct {
	byID   map[string]domain.Module
	byName map[string]domain.Module
}

func newFakeModules() *fakeModules {
	return &fakeModules{
		byID:   map[string]domain.Module{},
		byName: map[string]domain.Module{},
	}
}

func (repo *fakeModules) Create(ctx context.Context, module domain.Module) error {
	if _, exists := repo.byName[module.Name.String()]; exists {
		return domain.ErrDuplicate
	}
	repo.byID[module.ID.String()] = module
	repo.byName[module.Name.String()] = module
	return nil
}

func (repo *fakeModules) GetByID(ctx context.Context, id domain.ModuleID) (domain.Module, error) {
	module, exists := repo.byID[id.String()]
	if !exists {
		return domain.Module{}, domain.ErrNotFound
	}
	return module, nil
}

func (repo *fakeModules) GetByName(ctx context.Context, name domain.ModuleName) (domain.Module, error) {
	module, exists := repo.byName[name.String()]
	if !exists {
		return domain.Module{}, domain.ErrNotFound
	}
	return module, nil
}

func (repo *fakeModules) List(ctx context.Context, limit int, offset int) ([]domain.Module, error) {
	modules := make([]domain.Module, 0, len(repo.byName))
	for _, module := range repo.byName {
		modules = append(modules, module)
	}
	return modules, nil
}

func (repo *fakeModules) snapshot() *fakeModules {
	copy := newFakeModules()
	for key, value := range repo.byID {
		copy.byID[key] = value
	}
	for key, value := range repo.byName {
		copy.byName[key] = value
	}
	return copy
}

func (repo *fakeModules) restore(snapshot *fakeModules) {
	repo.byID = snapshot.byID
	repo.byName = snapshot.byName
}

type fakeGitLabProjects struct {
	byModuleID      map[string]domain.ModuleGitLabProject
	byGitLabProject map[string]domain.ModuleGitLabProject
}

func newFakeGitLabProjects() *fakeGitLabProjects {
	return &fakeGitLabProjects{
		byModuleID:      map[string]domain.ModuleGitLabProject{},
		byGitLabProject: map[string]domain.ModuleGitLabProject{},
	}
}

func (repo *fakeGitLabProjects) Upsert(ctx context.Context, mapping domain.ModuleGitLabProject) error {
	normalized, err := mapping.Normalized()
	if err != nil {
		return err
	}

	gitLabKey := gitLabProjectKey(normalized.GitLabBaseURL, normalized.GitLabProjectID)
	if existing, exists := repo.byGitLabProject[gitLabKey]; exists && existing.ModuleID != normalized.ModuleID {
		return domain.ErrDuplicate
	}

	moduleKey := normalized.ModuleID.String()
	if existing, exists := repo.byModuleID[moduleKey]; exists {
		delete(repo.byGitLabProject, gitLabProjectKey(existing.GitLabBaseURL, existing.GitLabProjectID))
		normalized.ID = existing.ID
		normalized.CreatedAt = existing.CreatedAt
	}

	repo.byModuleID[moduleKey] = normalized
	repo.byGitLabProject[gitLabKey] = normalized
	return nil
}

func (repo *fakeGitLabProjects) GetByModuleID(ctx context.Context, moduleID domain.ModuleID) (domain.ModuleGitLabProject, error) {
	mapping, exists := repo.byModuleID[moduleID.String()]
	if !exists {
		return domain.ModuleGitLabProject{}, domain.ErrNotFound
	}
	return mapping, nil
}

func (repo *fakeGitLabProjects) GetByGitLabProject(ctx context.Context, gitLabBaseURL string, gitLabProjectID int64) (domain.ModuleGitLabProject, error) {
	mapping, exists := repo.byGitLabProject[gitLabProjectKey(gitLabBaseURL, gitLabProjectID)]
	if !exists {
		return domain.ModuleGitLabProject{}, domain.ErrNotFound
	}
	return mapping, nil
}

func (repo *fakeGitLabProjects) snapshot() *fakeGitLabProjects {
	copy := newFakeGitLabProjects()
	for key, value := range repo.byModuleID {
		copy.byModuleID[key] = value
	}
	for key, value := range repo.byGitLabProject {
		copy.byGitLabProject[key] = value
	}
	return copy
}

func (repo *fakeGitLabProjects) restore(snapshot *fakeGitLabProjects) {
	repo.byModuleID = snapshot.byModuleID
	repo.byGitLabProject = snapshot.byGitLabProject
}

func gitLabProjectKey(baseURL string, projectID int64) string {
	return baseURL + ":" + strconv.FormatInt(projectID, 10)
}

type fakeVersions struct {
	byID            map[string]domain.ModuleVersion
	byModuleVersion map[string]domain.ModuleVersion
}

func newFakeVersions() *fakeVersions {
	return &fakeVersions{
		byID:            map[string]domain.ModuleVersion{},
		byModuleVersion: map[string]domain.ModuleVersion{},
	}
}

func (repo *fakeVersions) Create(ctx context.Context, version domain.ModuleVersion) error {
	key := moduleVersionKey(version.ModuleID, version.Version)
	if _, exists := repo.byModuleVersion[key]; exists {
		return domain.ErrDuplicate
	}
	repo.byID[version.ID.String()] = version
	repo.byModuleVersion[key] = version
	return nil
}

func (repo *fakeVersions) UpdateDeprecation(ctx context.Context, id domain.ModuleVersionID, deprecatedAt *time.Time, deprecatedBy string, deprecationReason string) error {
	version, exists := repo.byID[id.String()]
	if !exists {
		return domain.ErrNotFound
	}
	version.DeprecatedAt = deprecatedAt
	version.DeprecatedBy = deprecatedBy
	version.DeprecationReason = deprecationReason
	repo.byID[id.String()] = version
	repo.byModuleVersion[moduleVersionKey(version.ModuleID, version.Version)] = version
	return nil
}

func (repo *fakeVersions) GetByID(ctx context.Context, id domain.ModuleVersionID) (domain.ModuleVersion, error) {
	version, exists := repo.byID[id.String()]
	if !exists {
		return domain.ModuleVersion{}, domain.ErrNotFound
	}
	return version, nil
}

func (repo *fakeVersions) GetByModuleAndVersion(ctx context.Context, moduleID domain.ModuleID, version domain.Version) (domain.ModuleVersion, error) {
	moduleVersion, exists := repo.byModuleVersion[moduleVersionKey(moduleID, version)]
	if !exists {
		return domain.ModuleVersion{}, domain.ErrNotFound
	}
	return moduleVersion, nil
}

func (repo *fakeVersions) GetLatestByModule(ctx context.Context, moduleID domain.ModuleID) (domain.ModuleVersion, error) {
	var latest domain.ModuleVersion
	for _, version := range repo.byModuleVersion {
		if version.ModuleID == moduleID && (latest.ID == "" || version.CreatedAt.After(latest.CreatedAt)) {
			latest = version
		}
	}
	if latest.ID == "" {
		return domain.ModuleVersion{}, domain.ErrNotFound
	}
	return latest, nil
}

func (repo *fakeVersions) ListByModule(ctx context.Context, moduleID domain.ModuleID, limit int, offset int) ([]domain.ModuleVersion, error) {
	versions := make([]domain.ModuleVersion, 0)
	for _, version := range repo.byModuleVersion {
		if version.ModuleID == moduleID {
			versions = append(versions, version)
		}
	}
	return versions, nil
}

func (repo *fakeVersions) snapshot() *fakeVersions {
	copy := newFakeVersions()
	for key, value := range repo.byID {
		copy.byID[key] = value
	}
	for key, value := range repo.byModuleVersion {
		copy.byModuleVersion[key] = value
	}
	return copy
}

func (repo *fakeVersions) restore(snapshot *fakeVersions) {
	repo.byID = snapshot.byID
	repo.byModuleVersion = snapshot.byModuleVersion
}

func moduleVersionKey(moduleID domain.ModuleID, version domain.Version) string {
	return moduleID.String() + ":" + version.String()
}

type fakeArtifacts struct {
	byID          map[string]domain.Artifact
	byVersionKind map[string]domain.Artifact
}

func newFakeArtifacts() *fakeArtifacts {
	return &fakeArtifacts{
		byID:          map[string]domain.Artifact{},
		byVersionKind: map[string]domain.Artifact{},
	}
}

func (repo *fakeArtifacts) Create(ctx context.Context, artifact domain.Artifact) error {
	key := artifactVersionKindKey(artifact.ModuleVersionID, artifact.Kind)
	if _, exists := repo.byVersionKind[key]; exists {
		return domain.ErrDuplicate
	}
	repo.byID[artifact.ID.String()] = artifact
	repo.byVersionKind[key] = artifact
	return nil
}

func (repo *fakeArtifacts) GetByID(ctx context.Context, id domain.ArtifactID) (domain.Artifact, error) {
	artifact, exists := repo.byID[id.String()]
	if !exists {
		return domain.Artifact{}, domain.ErrNotFound
	}
	return artifact, nil
}

func (repo *fakeArtifacts) GetByModuleVersion(ctx context.Context, moduleVersionID domain.ModuleVersionID) (domain.Artifact, error) {
	return repo.GetByModuleVersionAndKind(ctx, moduleVersionID, domain.ArtifactKindSourceArchive)
}

func (repo *fakeArtifacts) GetByModuleVersionAndKind(ctx context.Context, moduleVersionID domain.ModuleVersionID, kind domain.ArtifactKind) (domain.Artifact, error) {
	artifact, exists := repo.byVersionKind[artifactVersionKindKey(moduleVersionID, kind)]
	if !exists {
		return domain.Artifact{}, domain.ErrNotFound
	}
	return artifact, nil
}

func (repo *fakeArtifacts) ListByModuleVersion(ctx context.Context, moduleVersionID domain.ModuleVersionID) ([]domain.Artifact, error) {
	artifacts := make([]domain.Artifact, 0)
	for _, artifact := range repo.byVersionKind {
		if artifact.ModuleVersionID == moduleVersionID {
			artifacts = append(artifacts, artifact)
		}
	}
	return artifacts, nil
}

func (repo *fakeArtifacts) snapshot() *fakeArtifacts {
	copy := newFakeArtifacts()
	for key, value := range repo.byID {
		copy.byID[key] = value
	}
	for key, value := range repo.byVersionKind {
		copy.byVersionKind[key] = value
	}
	return copy
}

func (repo *fakeArtifacts) restore(snapshot *fakeArtifacts) {
	repo.byID = snapshot.byID
	repo.byVersionKind = snapshot.byVersionKind
}

func artifactVersionKindKey(moduleVersionID domain.ModuleVersionID, kind domain.ArtifactKind) string {
	return moduleVersionID.String() + ":" + kind.String()
}

type fakeBufConfigs struct {
	byVersion map[string]domain.BufConfigInfo
}

func newFakeBufConfigs() *fakeBufConfigs {
	return &fakeBufConfigs{byVersion: map[string]domain.BufConfigInfo{}}
}

func (repo *fakeBufConfigs) Save(ctx context.Context, moduleVersionID domain.ModuleVersionID, config domain.BufConfigInfo) error {
	repo.byVersion[moduleVersionID.String()] = config
	return nil
}

func (repo *fakeBufConfigs) GetByModuleVersion(ctx context.Context, moduleVersionID domain.ModuleVersionID) (domain.BufConfigInfo, error) {
	config, exists := repo.byVersion[moduleVersionID.String()]
	if !exists {
		return domain.BufConfigInfo{}, domain.ErrNotFound
	}
	return config, nil
}

func (repo *fakeBufConfigs) snapshot() *fakeBufConfigs {
	copy := newFakeBufConfigs()
	for key, value := range repo.byVersion {
		copy.byVersion[key] = value
	}
	return copy
}

func (repo *fakeBufConfigs) restore(snapshot *fakeBufConfigs) {
	repo.byVersion = snapshot.byVersion
}

type fakeMetadata struct {
	byVersion map[string]domain.DescriptorMetadata
	saveErr   error
}

func newFakeMetadata() *fakeMetadata {
	return &fakeMetadata{byVersion: map[string]domain.DescriptorMetadata{}}
}

func (repo *fakeMetadata) Save(ctx context.Context, moduleVersionID domain.ModuleVersionID, metadata domain.DescriptorMetadata) error {
	if repo.saveErr != nil {
		return repo.saveErr
	}
	repo.byVersion[moduleVersionID.String()] = metadata
	return nil
}

func (repo *fakeMetadata) GetByModuleVersion(ctx context.Context, moduleVersionID domain.ModuleVersionID) (domain.DescriptorMetadata, error) {
	metadata, exists := repo.byVersion[moduleVersionID.String()]
	if !exists {
		return domain.DescriptorMetadata{}, domain.ErrNotFound
	}
	return metadata, nil
}

func (repo *fakeMetadata) GetSummaryByModuleVersion(ctx context.Context, moduleVersionID domain.ModuleVersionID) (domain.DescriptorMetadataSummary, error) {
	metadata, err := repo.GetByModuleVersion(ctx, moduleVersionID)
	if err != nil {
		return domain.DescriptorMetadataSummary{}, err
	}
	return metadata.Summary(), nil
}

func (repo *fakeMetadata) snapshot() *fakeMetadata {
	copy := newFakeMetadata()
	copy.saveErr = repo.saveErr
	for key, value := range repo.byVersion {
		copy.byVersion[key] = value
	}
	return copy
}

func (repo *fakeMetadata) restore(snapshot *fakeMetadata) {
	repo.byVersion = snapshot.byVersion
	repo.saveErr = snapshot.saveErr
}

type fakeDependencyRebuilder struct {
	outbox        *fakeOutbox
	clock         *fakeClock
	called        bool
	inputs        []PublishedModuleVersionDependencyRebuildInput
	providerFiles []ProviderFile
	symbols       []ProviderSymbol
	dependencies  []domain.ModuleDependency
	unresolved    []domain.UnresolvedProtoDependency
	err           error
	dependencyID  int
	unresolvedID  int
}

func (rebuilder *fakeDependencyRebuilder) RebuildPublishedModuleVersionDependencies(ctx context.Context, input PublishedModuleVersionDependencyRebuildInput) (PublishedModuleVersionDependencyRebuildOutput, error) {
	rebuilder.called = true
	rebuilder.inputs = append(rebuilder.inputs, input)
	if rebuilder.err != nil {
		return PublishedModuleVersionDependencyRebuildOutput{}, rebuilder.err
	}

	selfFiles := map[string]struct{}{}
	selfSymbols := map[string]struct{}{}
	for _, file := range input.Metadata.Files {
		selfFiles[file.Path] = struct{}{}
		for _, message := range file.Messages {
			collectRegistryTestMessageSymbols(selfSymbols, message)
		}
		for _, service := range file.Services {
			selfSymbols[strings.TrimPrefix(service.FullName, ".")] = struct{}{}
		}
	}

	dependencies := make([]domain.ModuleDependency, 0)
	unresolved := make([]domain.UnresolvedProtoDependency, 0)
	for _, file := range input.Metadata.Files {
		for _, protoImport := range file.Imports {
			if IsWellKnownProtoImport(protoImport.Path) {
				continue
			}
			if _, self := selfFiles[protoImport.Path]; self {
				continue
			}
			matches := make([]ProviderFile, 0)
			for _, provider := range rebuilder.providerFiles {
				if provider.Path == protoImport.Path {
					matches = append(matches, provider)
				}
			}
			if len(matches) == 0 {
				unresolved = append(unresolved, rebuilder.newUnresolved(input, domain.DependencySourceImport, protoImport.Path, "", ""))
				continue
			}
			if matches[0].ModuleID == input.Module.ID {
				continue
			}
			dependencies = append(dependencies, rebuilder.newDependency(input, matches[0].ModuleID, matches[0].ModuleName, matches[0].ModuleVersionID, matches[0].Version, domain.DependencySourceImport, domain.DependencyResolutionReasonImportPath, protoImport.Path, "", ""))
		}
		for _, message := range file.Messages {
			rebuilder.resolveMessageReferences(input, message, selfSymbols, &dependencies, &unresolved)
		}
	}

	record, err := protoradarevents.NewModuleDependenciesUpdated(protoradarevents.ModuleDependenciesUpdated{
		Module:                    input.Module,
		Version:                   input.Version,
		DependencyCount:           len(dependencies),
		UnresolvedDependencyCount: len(unresolved),
		OccurredAt:                rebuilder.clock.Now(),
	})
	if err != nil {
		return PublishedModuleVersionDependencyRebuildOutput{}, err
	}
	if err := rebuilder.outbox.Create(ctx, record); err != nil {
		return PublishedModuleVersionDependencyRebuildOutput{}, err
	}

	rebuilder.dependencies = dependencies
	rebuilder.unresolved = unresolved
	return PublishedModuleVersionDependencyRebuildOutput{Dependencies: dependencies, Unresolved: unresolved}, nil
}

func (rebuilder *fakeDependencyRebuilder) resolveMessageReferences(input PublishedModuleVersionDependencyRebuildInput, message domain.ProtoMessage, selfSymbols map[string]struct{}, dependencies *[]domain.ModuleDependency, unresolved *[]domain.UnresolvedProtoDependency) {
	for _, field := range message.Fields {
		symbol := strings.TrimPrefix(strings.TrimSpace(field.TypeName), ".")
		if symbol == "" || IsWellKnownProtoSymbol(symbol) {
			continue
		}
		if _, self := selfSymbols[symbol]; self {
			continue
		}
		var matched *ProviderSymbol
		for index := range rebuilder.symbols {
			if strings.TrimPrefix(rebuilder.symbols[index].FullName, ".") == symbol {
				matched = &rebuilder.symbols[index]
				break
			}
		}
		if matched == nil {
			*unresolved = append(*unresolved, rebuilder.newUnresolved(input, domain.DependencySourceTypeReference, "", packageFromRegistryTestSymbol(symbol), symbol))
			continue
		}
		if matched.ModuleID == input.Module.ID {
			continue
		}
		*dependencies = append(*dependencies, rebuilder.newDependency(input, matched.ModuleID, matched.ModuleName, matched.ModuleVersionID, matched.Version, domain.DependencySourceTypeReference, domain.DependencyResolutionReasonSymbol, "", matched.PackageName, symbol))
	}
	for _, nested := range message.Messages {
		rebuilder.resolveMessageReferences(input, nested, selfSymbols, dependencies, unresolved)
	}
}

func (rebuilder *fakeDependencyRebuilder) newDependency(input PublishedModuleVersionDependencyRebuildInput, providerModuleID domain.ModuleID, providerModuleName domain.ModuleName, providerVersionID domain.ModuleVersionID, providerVersion domain.Version, source domain.DependencySource, reason domain.DependencyResolutionReason, importPath string, referencedPackage string, referencedSymbol string) domain.ModuleDependency {
	rebuilder.dependencyID++
	return domain.ModuleDependency{
		ID:                      domain.NewModuleDependencyID("dependency-" + strconv.Itoa(rebuilder.dependencyID)),
		ConsumerModuleID:        input.Module.ID,
		ConsumerModuleName:      input.Module.Name,
		ConsumerModuleVersionID: input.Version.ID,
		ConsumerVersion:         input.Version.Version,
		ProviderModuleID:        providerModuleID,
		ProviderModuleName:      providerModuleName,
		ProviderModuleVersionID: providerVersionID,
		ProviderVersion:         providerVersion,
		Source:                  source,
		Reason:                  reason,
		ImportPath:              importPath,
		ReferencedPackage:       referencedPackage,
		ReferencedSymbol:        referencedSymbol,
		CreatedAt:               rebuilder.clock.Now(),
	}
}

func (rebuilder *fakeDependencyRebuilder) newUnresolved(input PublishedModuleVersionDependencyRebuildInput, source domain.DependencySource, importPath string, referencedPackage string, referencedSymbol string) domain.UnresolvedProtoDependency {
	rebuilder.unresolvedID++
	return domain.UnresolvedProtoDependency{
		ID:                domain.NewUnresolvedProtoDependencyID("unresolved-" + strconv.Itoa(rebuilder.unresolvedID)),
		ModuleID:          input.Module.ID,
		ModuleName:        input.Module.Name,
		ModuleVersionID:   input.Version.ID,
		Version:           input.Version.Version,
		Source:            source,
		ImportPath:        importPath,
		ReferencedPackage: referencedPackage,
		ReferencedSymbol:  referencedSymbol,
		Reason:            domain.UnresolvedDependencyReasonProviderNotFound,
		CreatedAt:         rebuilder.clock.Now(),
	}
}

func (rebuilder *fakeDependencyRebuilder) snapshot() *fakeDependencyRebuilder {
	return &fakeDependencyRebuilder{
		outbox:        rebuilder.outbox,
		clock:         rebuilder.clock,
		called:        rebuilder.called,
		inputs:        slices.Clone(rebuilder.inputs),
		providerFiles: slices.Clone(rebuilder.providerFiles),
		symbols:       slices.Clone(rebuilder.symbols),
		dependencies:  slices.Clone(rebuilder.dependencies),
		unresolved:    slices.Clone(rebuilder.unresolved),
		err:           rebuilder.err,
		dependencyID:  rebuilder.dependencyID,
		unresolvedID:  rebuilder.unresolvedID,
	}
}

func (rebuilder *fakeDependencyRebuilder) restore(snapshot *fakeDependencyRebuilder) {
	rebuilder.called = snapshot.called
	rebuilder.inputs = snapshot.inputs
	rebuilder.providerFiles = snapshot.providerFiles
	rebuilder.symbols = snapshot.symbols
	rebuilder.dependencies = snapshot.dependencies
	rebuilder.unresolved = snapshot.unresolved
	rebuilder.err = snapshot.err
	rebuilder.dependencyID = snapshot.dependencyID
	rebuilder.unresolvedID = snapshot.unresolvedID
}

func collectRegistryTestMessageSymbols(symbols map[string]struct{}, message domain.ProtoMessage) {
	symbols[strings.TrimPrefix(message.FullName, ".")] = struct{}{}
	for _, nested := range message.Messages {
		collectRegistryTestMessageSymbols(symbols, nested)
	}
}

func packageFromRegistryTestSymbol(symbol string) string {
	index := strings.LastIndex(symbol, ".")
	if index <= 0 {
		return ""
	}
	return symbol[:index]
}

type fakeReports struct {
	byID      map[string]domain.BreakingReport
	changes   map[string][]domain.BreakingChange
	createErr error
}

func newFakeReports() *fakeReports {
	return &fakeReports{
		byID:    map[string]domain.BreakingReport{},
		changes: map[string][]domain.BreakingChange{},
	}
}

func (repo *fakeReports) Create(ctx context.Context, report domain.BreakingReport, changes []domain.BreakingChange) error {
	if repo.createErr != nil {
		return repo.createErr
	}
	if _, exists := repo.byID[report.ID.String()]; exists {
		return domain.ErrDuplicate
	}
	repo.byID[report.ID.String()] = report
	repo.changes[report.ID.String()] = slices.Clone(changes)
	return nil
}

func (repo *fakeReports) GetByID(ctx context.Context, id domain.BreakingReportID) (domain.BreakingReport, []domain.BreakingChange, error) {
	report, exists := repo.byID[id.String()]
	if !exists {
		return domain.BreakingReport{}, nil, domain.ErrNotFound
	}
	return report, slices.Clone(repo.changes[id.String()]), nil
}

func (repo *fakeReports) ListByModule(ctx context.Context, moduleID domain.ModuleID, limit int, offset int) ([]domain.BreakingReport, error) {
	reports := make([]domain.BreakingReport, 0)
	for _, report := range repo.byID {
		if report.ModuleID == moduleID {
			reports = append(reports, report)
		}
	}
	return reports, nil
}

func (repo *fakeReports) CountChangesByReport(ctx context.Context, reportID domain.BreakingReportID) (int, error) {
	if _, exists := repo.byID[reportID.String()]; !exists {
		return 0, domain.ErrNotFound
	}
	return len(repo.changes[reportID.String()]), nil
}

func (repo *fakeReports) ListChangesByReport(ctx context.Context, reportID domain.BreakingReportID, limit int, offset int) ([]domain.BreakingChange, error) {
	if _, exists := repo.byID[reportID.String()]; !exists {
		return nil, domain.ErrNotFound
	}
	return slices.Clone(repo.changes[reportID.String()]), nil
}

func (repo *fakeReports) snapshot() *fakeReports {
	copy := newFakeReports()
	copy.createErr = repo.createErr
	for key, value := range repo.byID {
		copy.byID[key] = value
	}
	for key, value := range repo.changes {
		copy.changes[key] = slices.Clone(value)
	}
	return copy
}

func (repo *fakeReports) restore(snapshot *fakeReports) {
	repo.byID = snapshot.byID
	repo.changes = snapshot.changes
	repo.createErr = snapshot.createErr
}

type fakeTokens struct {
	byID        map[string]domain.APIToken
	byHash      map[string]domain.APIToken
	lastUsed    map[string]time.Time
	markUsedErr error
}

func newFakeTokens() *fakeTokens {
	return &fakeTokens{
		byID:     map[string]domain.APIToken{},
		byHash:   map[string]domain.APIToken{},
		lastUsed: map[string]time.Time{},
	}
}

func (repo *fakeTokens) Create(ctx context.Context, token domain.APIToken) error {
	if _, exists := repo.byHash[token.TokenHash]; exists {
		return domain.ErrDuplicate
	}
	repo.byID[token.ID.String()] = token
	repo.byHash[token.TokenHash] = token
	return nil
}

func (repo *fakeTokens) GetByID(ctx context.Context, id domain.APITokenID) (domain.APIToken, error) {
	token, exists := repo.byID[id.String()]
	if !exists {
		return domain.APIToken{}, domain.ErrNotFound
	}
	return token, nil
}

func (repo *fakeTokens) GetByHash(ctx context.Context, tokenHash string) (domain.APIToken, error) {
	token, exists := repo.byHash[tokenHash]
	if !exists {
		return domain.APIToken{}, domain.ErrNotFound
	}
	return token, nil
}

func (repo *fakeTokens) MarkUsed(ctx context.Context, id domain.APITokenID, usedAt time.Time) error {
	if repo.markUsedErr != nil {
		return repo.markUsedErr
	}
	token, exists := repo.byID[id.String()]
	if !exists {
		return domain.ErrNotFound
	}
	token.LastUsedAt = &usedAt
	repo.byID[id.String()] = token
	repo.byHash[token.TokenHash] = token
	repo.lastUsed[id.String()] = usedAt
	return nil
}

type fakeTokenUsageObserver struct {
	failures []TokenUsageFailure
}

func (observer *fakeTokenUsageObserver) RecordTokenUsageFailure(ctx context.Context, failure TokenUsageFailure) {
	observer.failures = append(observer.failures, failure)
}

func (repo *fakeTokens) snapshot() *fakeTokens {
	copy := newFakeTokens()
	for key, value := range repo.byID {
		copy.byID[key] = value
	}
	for key, value := range repo.byHash {
		copy.byHash[key] = value
	}
	for key, value := range repo.lastUsed {
		copy.lastUsed[key] = value
	}
	return copy
}

func (repo *fakeTokens) restore(snapshot *fakeTokens) {
	repo.byID = snapshot.byID
	repo.byHash = snapshot.byHash
	repo.lastUsed = snapshot.lastUsed
}

type fakeOutbox struct {
	records   []outbox.Record
	createErr error
}

func (writer *fakeOutbox) Create(ctx context.Context, record outbox.Record) error {
	if writer.createErr != nil {
		return writer.createErr
	}
	writer.records = append(writer.records, record)
	return nil
}

type fakeArtifactStore struct {
	objects      map[string][]byte
	putKey       string
	deletedKey   string
	putKeys      []string
	deletedKeys  []string
	putCalls     int
	putErrOnCall int
	putErr       error
	deleteErr    error
}

func (store *fakeArtifactStore) Put(ctx context.Context, key string, body io.Reader, sizeBytes int64) (storage.ArtifactObject, error) {
	store.putCalls++
	store.putKey = key
	store.putKeys = append(store.putKeys, key)
	if store.putErrOnCall == store.putCalls && store.putErr != nil {
		return storage.ArtifactObject{}, store.putErr
	}

	data, err := io.ReadAll(body)
	if err != nil {
		return storage.ArtifactObject{}, err
	}
	store.objects[key] = data
	return storage.ArtifactObject{
		Key:         key,
		ContentType: "application/gzip",
		SizeBytes:   sizeBytes,
	}, nil
}

func (store *fakeArtifactStore) Get(ctx context.Context, key string) (storage.ArtifactObject, error) {
	data, exists := store.objects[key]
	if !exists {
		return storage.ArtifactObject{}, domain.ErrNotFound
	}
	return storage.ArtifactObject{
		Key:         key,
		ContentType: "application/gzip",
		SizeBytes:   int64(len(data)),
		Body:        io.NopCloser(bytes.NewReader(data)),
	}, nil
}

func (store *fakeArtifactStore) Delete(ctx context.Context, key string) error {
	store.deletedKey = key
	store.deletedKeys = append(store.deletedKeys, key)
	if store.deleteErr != nil {
		return store.deleteErr
	}
	delete(store.objects, key)
	return nil
}

type fakeArtifactCleanupObserver struct {
	failures []ArtifactCleanupFailure
}

func (observer *fakeArtifactCleanupObserver) RecordArtifactCleanupFailure(ctx context.Context, failure ArtifactCleanupFailure) {
	observer.failures = append(observer.failures, failure)
}

type fakeBufWorkflow struct {
	result registryBufWorkflowResultAlias
	err    error
	calls  []bufWorkflowCall
}

type registryBufWorkflowResultAlias = BufWorkflowResult

type bufWorkflowCall struct {
	workdir string
	options BufWorkflowOptions
}

func (workflow *fakeBufWorkflow) Inspect(ctx context.Context, workdir string, options BufWorkflowOptions) (BufWorkflowResult, error) {
	workflow.calls = append(workflow.calls, bufWorkflowCall{workdir: workdir, options: options})
	return workflow.result, workflow.err
}

type fakeBufBreaking struct {
	result BufBreakingCheckResult
	err    error
	calls  []BufBreakingCheckInput
}

func (checker *fakeBufBreaking) CheckBreaking(ctx context.Context, input BufBreakingCheckInput) (BufBreakingCheckResult, error) {
	checker.calls = append(checker.calls, input)
	return checker.result, checker.err
}

func successfulBufWorkflowResult() BufWorkflowResult {
	metadata := domain.DescriptorMetadata{Files: []domain.ProtoFile{
		{
			Path:        "user.proto",
			PackageName: "user.v1",
			Syntax:      "proto3",
			Messages: []domain.ProtoMessage{
				{
					Name:     "User",
					FullName: "user.v1.User",
					Fields: []domain.ProtoField{
						{Name: "id", Number: 1, Type: "TYPE_STRING", Label: "LABEL_OPTIONAL", JSONName: "id"},
					},
				},
			},
		},
	}}
	return BufWorkflowResult{
		ConfigInfo: domain.BufConfigInfo{
			BufYAMLPresent: true,
			BufLockPresent: true,
			BufYAMLDigest:  "sha256:buf-yaml",
			BufLockDigest:  "sha256:buf-lock",
			LintEnabled:    true,
		},
		BufImage:           []byte("buf image"),
		BufImageDigest:     "sha256:buf-image",
		LintResult:         domain.BufLintResult{Status: domain.BufLintStatusPassed},
		DescriptorMetadata: metadata,
	}
}

func validSourceArchive(t *testing.T) []byte {
	t.Helper()
	return buildSourceArchive(t, map[string]string{
		"buf.yaml":   "version: v2\n",
		"user.proto": "syntax = \"proto3\";",
	})
}

func sourceArchiveWithoutBufYAML(t *testing.T) []byte {
	t.Helper()
	return buildSourceArchive(t, map[string]string{
		"user.proto": "syntax = \"proto3\";",
	})
}

func requireOutboxRecord(t *testing.T, records []outbox.Record, eventType string) outbox.Record {
	t.Helper()
	for _, record := range records {
		if record.EventType == eventType {
			return record
		}
	}
	t.Fatalf("outbox record %q not found in %#v", eventType, records)
	return outbox.Record{}
}

func buildSourceArchive(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	gzipWriter := gzip.NewWriter(&buffer)
	tarWriter := tar.NewWriter(gzipWriter)
	for name, body := range files {
		if err := tarWriter.WriteHeader(&tar.Header{Name: name, Mode: 0o600, Size: int64(len(body))}); err != nil {
			t.Fatalf("write tar header: %v", err)
		}
		if _, err := tarWriter.Write([]byte(body)); err != nil {
			t.Fatalf("write tar body: %v", err)
		}
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatalf("close gzip: %v", err)
	}
	return buffer.Bytes()
}
