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
		INSERT INTO artifacts (id, module_version_id, kind, storage_key, size_bytes, checksum_sha256, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`,
		artifact.ID.String(),
		artifact.ModuleVersionID.String(),
		artifactKindOrDefault(artifact.Kind).String(),
		artifact.StorageKey,
		artifact.SizeBytes,
		artifact.ChecksumSHA256,
		artifact.CreatedAt,
	)
	return mapError(err)
}

func (repo *ArtifactRepository) GetByID(ctx context.Context, id domain.ArtifactID) (domain.Artifact, error) {
	return repo.getOne(ctx, `
		SELECT id, module_version_id, kind, storage_key, size_bytes, checksum_sha256, created_at
		FROM artifacts
		WHERE id = $1
	`, id.String())
}

func (repo *ArtifactRepository) GetByModuleVersion(ctx context.Context, moduleVersionID domain.ModuleVersionID) (domain.Artifact, error) {
	return repo.GetByModuleVersionAndKind(ctx, moduleVersionID, domain.ArtifactKindSourceArchive)
}

func (repo *ArtifactRepository) GetByModuleVersionAndKind(ctx context.Context, moduleVersionID domain.ModuleVersionID, kind domain.ArtifactKind) (domain.Artifact, error) {
	return repo.getOne(ctx, `
		SELECT id, module_version_id, kind, storage_key, size_bytes, checksum_sha256, created_at
		FROM artifacts
		WHERE module_version_id = $1 AND kind = $2
	`, moduleVersionID.String(), kind.String())
}

func (repo *ArtifactRepository) ListByModuleVersion(ctx context.Context, moduleVersionID domain.ModuleVersionID) ([]domain.Artifact, error) {
	rows, err := repo.db.executor(ctx).Query(ctx, `
		SELECT id, module_version_id, kind, storage_key, size_bytes, checksum_sha256, created_at
		FROM artifacts
		WHERE module_version_id = $1
		ORDER BY kind ASC
	`, moduleVersionID.String())
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	artifacts := make([]domain.Artifact, 0)
	for rows.Next() {
		artifact, err := scanArtifact(rows.Scan)
		if err != nil {
			return nil, err
		}
		artifacts = append(artifacts, artifact)
	}
	if err := rows.Err(); err != nil {
		return nil, mapError(err)
	}
	return artifacts, nil
}

func (repo *ArtifactRepository) getOne(ctx context.Context, query string, args ...any) (domain.Artifact, error) {
	artifact, err := scanArtifact(repo.db.executor(ctx).QueryRow(ctx, query, args...).Scan)
	if err != nil {
		return domain.Artifact{}, mapError(err)
	}
	return artifact, nil
}

func scanArtifact(scan func(dest ...any) error) (domain.Artifact, error) {
	var artifact domain.Artifact
	var id string
	var moduleVersionID string
	var kindValue string

	if err := scan(
		&id,
		&moduleVersionID,
		&kindValue,
		&artifact.StorageKey,
		&artifact.SizeBytes,
		&artifact.ChecksumSHA256,
		&artifact.CreatedAt,
	); err != nil {
		return domain.Artifact{}, err
	}

	kind, err := domain.NewArtifactKind(kindValue)
	if err != nil {
		return domain.Artifact{}, err
	}
	artifact.ID = domain.NewArtifactID(id)
	artifact.ModuleVersionID = domain.NewModuleVersionID(moduleVersionID)
	artifact.Kind = kind
	return artifact, nil
}

func artifactKindOrDefault(kind domain.ArtifactKind) domain.ArtifactKind {
	if kind.IsValid() {
		return kind
	}
	return domain.ArtifactKindSourceArchive
}

var _ domain.ArtifactRepository = (*ArtifactRepository)(nil)
var _ domain.ModuleVersionArtifactRepository = (*ArtifactRepository)(nil)
