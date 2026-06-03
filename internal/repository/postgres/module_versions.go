package postgres

import (
	"context"
	"time"

	"github.com/alryzden/ProtoRadar/internal/domain"
)

type ModuleVersionRepository struct {
	db *DB
}

func NewModuleVersionRepository(db *DB) *ModuleVersionRepository {
	return &ModuleVersionRepository{db: db}
}

func (repo *ModuleVersionRepository) Create(ctx context.Context, version domain.ModuleVersion) error {
	_, err := repo.db.executor(ctx).Exec(ctx, `
		INSERT INTO module_versions (id, module_id, version, digest, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`,
		version.ID.String(),
		version.ModuleID.String(),
		version.Version.String(),
		version.Digest,
		version.Status.String(),
		version.CreatedAt,
	)
	return mapError(err)
}

func (repo *ModuleVersionRepository) GetByID(ctx context.Context, id domain.ModuleVersionID) (domain.ModuleVersion, error) {
	return repo.getOne(ctx, `
		SELECT id, module_id, version, digest, status, created_at
		FROM module_versions
		WHERE id = $1
	`, id.String())
}

func (repo *ModuleVersionRepository) GetByModuleAndVersion(ctx context.Context, moduleID domain.ModuleID, version domain.Version) (domain.ModuleVersion, error) {
	return repo.getOne(ctx, `
		SELECT id, module_id, version, digest, status, created_at
		FROM module_versions
		WHERE module_id = $1 AND version = $2
	`, moduleID.String(), version.String())
}

func (repo *ModuleVersionRepository) GetLatestByModule(ctx context.Context, moduleID domain.ModuleID) (domain.ModuleVersion, error) {
	return repo.getOne(ctx, `
		SELECT id, module_id, version, digest, status, created_at
		FROM module_versions
		WHERE module_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`, moduleID.String())
}

func (repo *ModuleVersionRepository) ListByModule(ctx context.Context, moduleID domain.ModuleID, limit int, offset int) ([]domain.ModuleVersion, error) {
	rows, err := repo.db.executor(ctx).Query(ctx, `
		SELECT id, module_id, version, digest, status, created_at
		FROM module_versions
		WHERE module_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`, moduleID.String(), limit, offset)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	versions := make([]domain.ModuleVersion, 0)
	for rows.Next() {
		version, err := scanModuleVersion(rows.Scan)
		if err != nil {
			return nil, err
		}
		versions = append(versions, version)
	}

	if err := rows.Err(); err != nil {
		return nil, mapError(err)
	}
	return versions, nil
}

func (repo *ModuleVersionRepository) getOne(ctx context.Context, query string, args ...any) (domain.ModuleVersion, error) {
	version, err := scanModuleVersion(repo.db.executor(ctx).QueryRow(ctx, query, args...).Scan)
	if err != nil {
		return domain.ModuleVersion{}, mapError(err)
	}
	return version, nil
}

func scanModuleVersion(scan func(dest ...any) error) (domain.ModuleVersion, error) {
	var id string
	var moduleID string
	var versionValue string
	var statusValue string
	var createdAt time.Time
	var digest string

	if err := scan(&id, &moduleID, &versionValue, &digest, &statusValue, &createdAt); err != nil {
		return domain.ModuleVersion{}, err
	}

	version, err := domain.NewVersion(versionValue)
	if err != nil {
		return domain.ModuleVersion{}, err
	}
	status, err := domain.NewModuleVersionStatus(statusValue)
	if err != nil {
		return domain.ModuleVersion{}, err
	}

	return domain.ModuleVersion{
		ID:        domain.NewModuleVersionID(id),
		ModuleID:  domain.NewModuleID(moduleID),
		Version:   version,
		Status:    status,
		Digest:    digest,
		CreatedAt: createdAt,
	}, nil
}

var _ domain.ModuleVersionRepository = (*ModuleVersionRepository)(nil)
