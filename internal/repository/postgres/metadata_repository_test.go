package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/alryzden/ProtoRadar/internal/domain"
)

func TestArtifactRepositorySupportsMultipleKindsAndUniqueness(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	moduleVersionID := createTestModuleVersion(t, ctx, db, "v1.0.0")
	repo := NewArtifactRepository(db)
	now := time.Now().UTC()

	source := domain.Artifact{
		ID:              domain.NewArtifactID("00000000-0000-0000-0000-000000000101"),
		ModuleVersionID: moduleVersionID,
		Kind:            domain.ArtifactKindSourceArchive,
		StorageKey:      "modules/test/v1/source.tar.gz",
		ChecksumSHA256:  "source-checksum",
		SizeBytes:       10,
		CreatedAt:       now,
	}
	image := domain.Artifact{
		ID:              domain.NewArtifactID("00000000-0000-0000-0000-000000000102"),
		ModuleVersionID: moduleVersionID,
		Kind:            domain.ArtifactKindBufImage,
		StorageKey:      "modules/test/v1/image.bin",
		ChecksumSHA256:  "image-checksum",
		SizeBytes:       20,
		CreatedAt:       now,
	}
	if err := repo.Create(ctx, source); err != nil {
		t.Fatalf("create source artifact: %v", err)
	}
	if err := repo.Create(ctx, image); err != nil {
		t.Fatalf("create buf image artifact: %v", err)
	}

	artifacts, err := repo.ListByModuleVersion(ctx, moduleVersionID)
	if err != nil {
		t.Fatalf("list artifacts: %v", err)
	}
	if len(artifacts) != 2 {
		t.Fatalf("artifacts = %d, want 2", len(artifacts))
	}
	gotImage, err := repo.GetByModuleVersionAndKind(ctx, moduleVersionID, domain.ArtifactKindBufImage)
	if err != nil {
		t.Fatalf("get buf image artifact: %v", err)
	}
	if gotImage.ChecksumSHA256 != "image-checksum" || gotImage.Kind != domain.ArtifactKindBufImage {
		t.Fatalf("buf image artifact = %#v", gotImage)
	}

	duplicate := source
	duplicate.ID = domain.NewArtifactID("00000000-0000-0000-0000-000000000103")
	duplicate.StorageKey = "modules/test/v1/source-duplicate.tar.gz"
	if err := repo.Create(ctx, duplicate); !errors.Is(err, domain.ErrDuplicate) {
		t.Fatalf("duplicate artifact error = %v, want ErrDuplicate", err)
	}
}

func TestBufConfigRepositorySaveAndLoad(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	moduleVersionID := createTestModuleVersion(t, ctx, db, "v1.0.1")
	repo := NewBufConfigRepository(db)

	config := domain.BufConfigInfo{
		BufYAMLPresent:        true,
		BufLockPresent:        true,
		BufYAMLDigest:         "sha256:buf-yaml",
		BufLockDigest:         "sha256:buf-lock",
		ModulePaths:           []string{"proto", "vendor/proto"},
		Deps:                  []string{"buf.build/googleapis/googleapis"},
		LintEnabled:           true,
		BreakingConfigPresent: true,
	}
	if err := repo.Save(ctx, moduleVersionID, config); err != nil {
		t.Fatalf("save buf config: %v", err)
	}

	got, err := repo.GetByModuleVersion(ctx, moduleVersionID)
	if err != nil {
		t.Fatalf("get buf config: %v", err)
	}
	if got.BufYAMLDigest != config.BufYAMLDigest || got.BufLockDigest != config.BufLockDigest {
		t.Fatalf("config digests = %#v", got)
	}
	if strings.Join(got.ModulePaths, ",") != "proto,vendor/proto" {
		t.Fatalf("module paths = %#v", got.ModulePaths)
	}
	if strings.Join(got.Deps, ",") != "buf.build/googleapis/googleapis" {
		t.Fatalf("deps = %#v", got.Deps)
	}
	if !got.LintEnabled || !got.BreakingConfigPresent {
		t.Fatalf("config flags = %#v", got)
	}
}

func TestDescriptorMetadataRepositorySaveLoadAndSummary(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	moduleVersionID := createTestModuleVersion(t, ctx, db, "v1.0.2")
	repo := NewDescriptorMetadataRepository(db)
	metadata := testDescriptorMetadata()

	if err := repo.Save(ctx, moduleVersionID, metadata); err != nil {
		t.Fatalf("save metadata: %v", err)
	}

	got, err := repo.GetByModuleVersion(ctx, moduleVersionID)
	if err != nil {
		t.Fatalf("get metadata: %v", err)
	}
	if len(got.Files) != 2 {
		t.Fatalf("files = %d", len(got.Files))
	}
	if got.Files[0].Path != "billing/v1/billing.proto" || got.Files[1].Path != "user/v1/user.proto" {
		t.Fatalf("files sorted by path = %#v", got.Files)
	}
	userFile := got.Files[1]
	if userFile.PackageName != "user.v1" || userFile.Syntax != "proto3" {
		t.Fatalf("user file = %#v", userFile)
	}
	if len(userFile.Imports) != 2 || !userFile.Imports[1].Public || !userFile.Imports[0].Weak {
		t.Fatalf("imports = %#v", userFile.Imports)
	}
	if len(userFile.Services) != 1 || len(userFile.Services[0].Methods) != 2 {
		t.Fatalf("services = %#v", userFile.Services)
	}
	if len(userFile.Messages) != 1 || len(userFile.Messages[0].Fields) != 2 || len(userFile.Messages[0].Messages) != 1 {
		t.Fatalf("messages = %#v", userFile.Messages)
	}
	if len(userFile.Enums) != 1 || len(userFile.Enums[0].Values) != 2 {
		t.Fatalf("enums = %#v", userFile.Enums)
	}

	summary, err := repo.GetSummaryByModuleVersion(ctx, moduleVersionID)
	if err != nil {
		t.Fatalf("get summary: %v", err)
	}
	want := metadata.Summary()
	if summary != want {
		t.Fatalf("summary = %#v, want %#v", summary, want)
	}
}

func TestDescriptorMetadataRepositoryConstraints(t *testing.T) {
	tests := []struct {
		name     string
		metadata domain.DescriptorMetadata
	}{
		{
			name: "proto file path unique per version",
			metadata: domain.DescriptorMetadata{Files: []domain.ProtoFile{
				{Path: "user.proto"},
				{Path: "user.proto"},
			}},
		},
		{
			name: "service full name unique per version",
			metadata: domain.DescriptorMetadata{Files: []domain.ProtoFile{
				{Path: "a.proto", Services: []domain.ProtoService{{Name: "Users", FullName: "example.Users"}}},
				{Path: "b.proto", Services: []domain.ProtoService{{Name: "Users", FullName: "example.Users"}}},
			}},
		},
		{
			name: "message full name unique per version",
			metadata: domain.DescriptorMetadata{Files: []domain.ProtoFile{
				{Path: "a.proto", Messages: []domain.ProtoMessage{{Name: "User", FullName: "example.User"}}},
				{Path: "b.proto", Messages: []domain.ProtoMessage{{Name: "User", FullName: "example.User"}}},
			}},
		},
		{
			name: "enum full name unique per version",
			metadata: domain.DescriptorMetadata{Files: []domain.ProtoFile{
				{Path: "a.proto", Enums: []domain.ProtoEnum{{Name: "Role", FullName: "example.Role"}}},
				{Path: "b.proto", Enums: []domain.ProtoEnum{{Name: "Role", FullName: "example.Role"}}},
			}},
		},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := newTestDB(t)
			ctx := context.Background()
			moduleVersionID := createTestModuleVersion(t, ctx, db, fmt.Sprintf("v1.1.%d", i))
			repo := NewDescriptorMetadataRepository(db)

			if err := repo.Save(ctx, moduleVersionID, tt.metadata); !errors.Is(err, domain.ErrDuplicate) {
				t.Fatalf("save error = %v, want ErrDuplicate", err)
			}
			if _, err := repo.GetByModuleVersion(ctx, moduleVersionID); !errors.Is(err, domain.ErrNotFound) {
				t.Fatalf("metadata should be rolled back after duplicate error, got %v", err)
			}
		})
	}
}

func TestDescriptorMetadataCascadeDeleteWhenModuleVersionDeleted(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	moduleVersionID := createTestModuleVersion(t, ctx, db, "v1.0.3")
	metadataRepo := NewDescriptorMetadataRepository(db)
	configRepo := NewBufConfigRepository(db)
	artifactRepo := NewArtifactRepository(db)
	now := time.Now().UTC()

	if err := metadataRepo.Save(ctx, moduleVersionID, testDescriptorMetadata()); err != nil {
		t.Fatalf("save metadata: %v", err)
	}
	if err := configRepo.Save(ctx, moduleVersionID, domain.BufConfigInfo{BufYAMLPresent: true}); err != nil {
		t.Fatalf("save buf config: %v", err)
	}
	if err := artifactRepo.Create(ctx, domain.Artifact{
		ID:              domain.NewArtifactID("00000000-0000-0000-0000-000000000201"),
		ModuleVersionID: moduleVersionID,
		Kind:            domain.ArtifactKindSourceArchive,
		StorageKey:      "cascade/source.tar.gz",
		ChecksumSHA256:  "source",
		SizeBytes:       1,
		CreatedAt:       now,
	}); err != nil {
		t.Fatalf("save artifact: %v", err)
	}

	if _, err := db.executor(ctx).Exec(ctx, `DELETE FROM module_versions WHERE id = $1`, moduleVersionID.String()); err != nil {
		t.Fatalf("delete module version: %v", err)
	}
	assertTableCount(t, ctx, db, "artifacts", 0)
	assertTableCount(t, ctx, db, "module_version_buf_configs", 0)
	assertTableCount(t, ctx, db, "proto_files", 0)
	assertTableCount(t, ctx, db, "proto_imports", 0)
	assertTableCount(t, ctx, db, "proto_services", 0)
	assertTableCount(t, ctx, db, "proto_methods", 0)
	assertTableCount(t, ctx, db, "proto_messages", 0)
	assertTableCount(t, ctx, db, "proto_fields", 0)
	assertTableCount(t, ctx, db, "proto_enums", 0)
	assertTableCount(t, ctx, db, "proto_enum_values", 0)
}

func TestDescriptorMetadataSaveUsesTransactionRollback(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()
	moduleVersionID := createTestModuleVersion(t, ctx, db, "v1.0.4")
	repo := NewDescriptorMetadataRepository(db)
	rollbackErr := errors.New("force rollback")

	err := db.WithinTransaction(ctx, func(txCtx context.Context) error {
		if err := repo.Save(txCtx, moduleVersionID, testDescriptorMetadata()); err != nil {
			return err
		}
		return rollbackErr
	})
	if !errors.Is(err, rollbackErr) {
		t.Fatalf("transaction error = %v", err)
	}
	if _, err := repo.GetByModuleVersion(ctx, moduleVersionID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("metadata should not persist after rollback, got %v", err)
	}
}

func createTestModuleVersion(t *testing.T, ctx context.Context, db *DB, versionValue string) domain.ModuleVersionID {
	t.Helper()
	moduleRepo := NewModuleRepository(db)
	versionRepo := NewModuleVersionRepository(db)
	now := time.Now().UTC()
	moduleID := domain.NewModuleID("00000000-0000-0000-0000-000000000001")
	moduleName, err := domain.NewModuleName("test-module")
	if err != nil {
		t.Fatalf("module name: %v", err)
	}
	if err := moduleRepo.Create(ctx, domain.Module{
		ID:        moduleID,
		Name:      moduleName,
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("create module: %v", err)
	}
	version, err := domain.NewVersion(versionValue)
	if err != nil {
		t.Fatalf("version: %v", err)
	}
	moduleVersionID := domain.NewModuleVersionID("00000000-0000-0000-0000-000000000002")
	if err := versionRepo.Create(ctx, domain.ModuleVersion{
		ID:        moduleVersionID,
		ModuleID:  moduleID,
		Version:   version,
		Status:    domain.ModuleVersionStatusPublished,
		Digest:    "sha256:test",
		CreatedAt: now,
	}); err != nil {
		t.Fatalf("create module version: %v", err)
	}
	return moduleVersionID
}

func testDescriptorMetadata() domain.DescriptorMetadata {
	return domain.DescriptorMetadata{Files: []domain.ProtoFile{
		{
			Path:        "user/v1/user.proto",
			PackageName: "user.v1",
			Syntax:      "proto3",
			Imports: []domain.ProtoImport{
				{Path: "common/v1/common.proto", Weak: true},
				{Path: "google/protobuf/timestamp.proto", Public: true},
			},
			Services: []domain.ProtoService{
				{
					Name:     "UserService",
					FullName: "user.v1.UserService",
					Methods: []domain.ProtoMethod{
						{Name: "GetUser", InputType: ".user.v1.GetUserRequest", OutputType: ".user.v1.User"},
						{Name: "WatchUsers", InputType: ".user.v1.WatchUsersRequest", OutputType: ".user.v1.User", ServerStreaming: true},
					},
				},
			},
			Messages: []domain.ProtoMessage{
				{
					Name:     "User",
					FullName: "user.v1.User",
					Fields: []domain.ProtoField{
						{Name: "id", Number: 1, Type: "string", Label: "optional", JSONName: "id"},
						{Name: "tags", Number: 2, Type: "string", Label: "repeated", JSONName: "tags", IsRepeated: true},
					},
					Messages: []domain.ProtoMessage{
						{
							Name:     "Profile",
							FullName: "user.v1.User.Profile",
							Fields: []domain.ProtoField{
								{Name: "display_name", Number: 1, Type: "string", JSONName: "displayName"},
							},
						},
					},
				},
			},
			Enums: []domain.ProtoEnum{
				{
					Name:     "Role",
					FullName: "user.v1.Role",
					Values: []domain.ProtoEnumValue{
						{Name: "ROLE_UNSPECIFIED", Number: 0},
						{Name: "ROLE_ADMIN", Number: 1},
					},
				},
			},
		},
		{
			Path:        "billing/v1/billing.proto",
			PackageName: "billing.v1",
			Syntax:      "proto3",
		},
	}}
}

func assertTableCount(t *testing.T, ctx context.Context, db *DB, table string, want int) {
	t.Helper()
	var got int
	if err := db.executor(ctx).QueryRow(ctx, `SELECT count(*) FROM `+table).Scan(&got); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	if got != want {
		t.Fatalf("%s count = %d, want %d", table, got, want)
	}
}

func quoteIdent(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}
