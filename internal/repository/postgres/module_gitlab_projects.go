package postgres

import (
	"context"

	"github.com/alryzden/ProtoRadar/internal/domain"
)

type ModuleGitLabProjectRepository struct {
	db *DB
}

func NewModuleGitLabProjectRepository(db *DB) *ModuleGitLabProjectRepository {
	return &ModuleGitLabProjectRepository{db: db}
}

func (repo *ModuleGitLabProjectRepository) Upsert(ctx context.Context, mapping domain.ModuleGitLabProject) error {
	normalized, err := mapping.Normalized()
	if err != nil {
		return err
	}

	_, err = repo.db.executor(ctx).Exec(ctx, `
		INSERT INTO module_gitlab_projects (
			id,
			module_id,
			gitlab_base_url,
			gitlab_project_id,
			gitlab_project_path,
			created_at,
			updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (module_id) DO UPDATE SET
			gitlab_base_url = EXCLUDED.gitlab_base_url,
			gitlab_project_id = EXCLUDED.gitlab_project_id,
			gitlab_project_path = EXCLUDED.gitlab_project_path,
			updated_at = EXCLUDED.updated_at
	`,
		normalized.ID.String(),
		normalized.ModuleID.String(),
		normalized.GitLabBaseURL,
		normalized.GitLabProjectID,
		normalized.GitLabProjectPath,
		normalized.CreatedAt,
		normalized.UpdatedAt,
	)
	return mapError(err)
}

func (repo *ModuleGitLabProjectRepository) GetByModuleID(ctx context.Context, moduleID domain.ModuleID) (domain.ModuleGitLabProject, error) {
	return repo.getOne(ctx, `
		SELECT
			mgp.id,
			mgp.module_id,
			m.name,
			mgp.gitlab_base_url,
			mgp.gitlab_project_id,
			mgp.gitlab_project_path,
			mgp.created_at,
			mgp.updated_at
		FROM module_gitlab_projects mgp
		JOIN modules m ON m.id = mgp.module_id
		WHERE mgp.module_id = $1
	`, moduleID.String())
}

func (repo *ModuleGitLabProjectRepository) GetByGitLabProject(ctx context.Context, gitLabBaseURL string, gitLabProjectID int64) (domain.ModuleGitLabProject, error) {
	return repo.getOne(ctx, `
		SELECT
			mgp.id,
			mgp.module_id,
			m.name,
			mgp.gitlab_base_url,
			mgp.gitlab_project_id,
			mgp.gitlab_project_path,
			mgp.created_at,
			mgp.updated_at
		FROM module_gitlab_projects mgp
		JOIN modules m ON m.id = mgp.module_id
		WHERE mgp.gitlab_base_url = $1 AND mgp.gitlab_project_id = $2
	`, gitLabBaseURL, gitLabProjectID)
}

func (repo *ModuleGitLabProjectRepository) getOne(ctx context.Context, query string, args ...any) (domain.ModuleGitLabProject, error) {
	mapping, err := scanModuleGitLabProject(repo.db.executor(ctx).QueryRow(ctx, query, args...).Scan)
	if err != nil {
		return domain.ModuleGitLabProject{}, mapError(err)
	}
	return mapping, nil
}

func scanModuleGitLabProject(scan func(dest ...any) error) (domain.ModuleGitLabProject, error) {
	var mapping domain.ModuleGitLabProject
	var id string
	var moduleID string
	var moduleName string

	if err := scan(
		&id,
		&moduleID,
		&moduleName,
		&mapping.GitLabBaseURL,
		&mapping.GitLabProjectID,
		&mapping.GitLabProjectPath,
		&mapping.CreatedAt,
		&mapping.UpdatedAt,
	); err != nil {
		return domain.ModuleGitLabProject{}, err
	}

	parsedModuleName, err := domain.NewModuleName(moduleName)
	if err != nil {
		return domain.ModuleGitLabProject{}, err
	}
	mapping.ID = domain.NewModuleGitLabProjectID(id)
	mapping.ModuleID = domain.NewModuleID(moduleID)
	mapping.ModuleName = parsedModuleName
	return mapping, nil
}

var _ domain.ModuleGitLabProjectRepository = (*ModuleGitLabProjectRepository)(nil)
