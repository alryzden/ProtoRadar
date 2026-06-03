package postgres

import (
	"context"

	"github.com/alryzden/ProtoRadar/internal/domain"
)

type ArtifactRepository struct {
	db *DB
}

func NewArtifactRepository(db *DB) *ArtifactRepository {
	return &ArtifactRepository{db: db}
}

func (repo *ArtifactRepository) Create(ctx context.Context, artifact domain.Artifact) error {
	_, err := repo.db.executor(ctx).Exec(ctx, `
		INSERT INTO artifacts (id, module_version_id, storage_key, size_bytes, checksum_sha256, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`,
		artifact.ID.String(),
		artifact.ModuleVersionID.String(),
		artifact.StorageKey,
		artifact.SizeBytes,
		artifact.ChecksumSHA256,
		artifact.CreatedAt,
	)
	return mapError(err)
}

func (repo *ArtifactRepository) GetByID(ctx context.Context, id domain.ArtifactID) (domain.Artifact, error) {
	return repo.getOne(ctx, `
		SELECT id, module_version_id, storage_key, size_bytes, checksum_sha256, created_at
		FROM artifacts
		WHERE id = $1
	`, id.String())
}

func (repo *ArtifactRepository) GetByModuleVersion(ctx context.Context, moduleVersionID domain.ModuleVersionID) (domain.Artifact, error) {
	return repo.getOne(ctx, `
		SELECT id, module_version_id, storage_key, size_bytes, checksum_sha256, created_at
		FROM artifacts
		WHERE module_version_id = $1
	`, moduleVersionID.String())
}

func (repo *ArtifactRepository) getOne(ctx context.Context, query string, args ...any) (domain.Artifact, error) {
	var artifact domain.Artifact
	var id string
	var moduleVersionID string

	err := repo.db.executor(ctx).QueryRow(ctx, query, args...).Scan(
		&id,
		&moduleVersionID,
		&artifact.StorageKey,
		&artifact.SizeBytes,
		&artifact.ChecksumSHA256,
		&artifact.CreatedAt,
	)
	if err != nil {
		return domain.Artifact{}, mapError(err)
	}

	artifact.ID = domain.NewArtifactID(id)
	artifact.ModuleVersionID = domain.NewModuleVersionID(moduleVersionID)
	return artifact, nil
}

var _ domain.ArtifactRepository = (*ArtifactRepository)(nil)
