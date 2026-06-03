package postgres

import (
	"context"

	"github.com/alryzden/ProtoRadar/internal/domain"
)

type ModuleRepository struct {
	db *DB
}

func NewModuleRepository(db *DB) *ModuleRepository {
	return &ModuleRepository{db: db}
}

func (repo *ModuleRepository) Create(ctx context.Context, module domain.Module) error {
	_, err := repo.db.executor(ctx).Exec(ctx, `
		INSERT INTO modules (id, name, description, repository_url, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`,
		module.ID.String(),
		module.Name.String(),
		module.Description,
		module.RepositoryURL,
		module.CreatedAt,
		module.UpdatedAt,
	)
	return mapError(err)
}

func (repo *ModuleRepository) GetByID(ctx context.Context, id domain.ModuleID) (domain.Module, error) {
	var module domain.Module
	var moduleID string
	var name string

	err := repo.db.executor(ctx).QueryRow(ctx, `
		SELECT id, name, description, repository_url, created_at, updated_at
		FROM modules
		WHERE id = $1
	`, id.String()).Scan(&moduleID, &name, &module.Description, &module.RepositoryURL, &module.CreatedAt, &module.UpdatedAt)
	if err != nil {
		return domain.Module{}, mapError(err)
	}

	module.ID = domain.NewModuleID(moduleID)
	module.Name, err = domain.NewModuleName(name)
	if err != nil {
		return domain.Module{}, err
	}
	return module, nil
}

func (repo *ModuleRepository) GetByName(ctx context.Context, name domain.ModuleName) (domain.Module, error) {
	var module domain.Module
	var moduleID string
	var storedName string

	err := repo.db.executor(ctx).QueryRow(ctx, `
		SELECT id, name, description, repository_url, created_at, updated_at
		FROM modules
		WHERE name = $1
	`, name.String()).Scan(&moduleID, &storedName, &module.Description, &module.RepositoryURL, &module.CreatedAt, &module.UpdatedAt)
	if err != nil {
		return domain.Module{}, mapError(err)
	}

	module.ID = domain.NewModuleID(moduleID)
	module.Name, err = domain.NewModuleName(storedName)
	if err != nil {
		return domain.Module{}, err
	}
	return module, nil
}

func (repo *ModuleRepository) List(ctx context.Context, limit int, offset int) ([]domain.Module, error) {
	rows, err := repo.db.executor(ctx).Query(ctx, `
		SELECT id, name, description, repository_url, created_at, updated_at
		FROM modules
		ORDER BY name ASC
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	modules := make([]domain.Module, 0)
	for rows.Next() {
		var module domain.Module
		var moduleID string
		var name string
		if err := rows.Scan(&moduleID, &name, &module.Description, &module.RepositoryURL, &module.CreatedAt, &module.UpdatedAt); err != nil {
			return nil, err
		}
		moduleName, err := domain.NewModuleName(name)
		if err != nil {
			return nil, err
		}
		module.ID = domain.NewModuleID(moduleID)
		module.Name = moduleName
		modules = append(modules, module)
	}

	if err := rows.Err(); err != nil {
		return nil, mapError(err)
	}
	return modules, nil
}

var _ domain.ModuleRepository = (*ModuleRepository)(nil)
