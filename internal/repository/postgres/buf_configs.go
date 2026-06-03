package postgres

import (
	"context"
	"encoding/json"
	"time"

	"github.com/alryzden/ProtoRadar/internal/domain"
)

type BufConfigRepository struct {
	db *DB
}

func NewBufConfigRepository(db *DB) *BufConfigRepository {
	return &BufConfigRepository{db: db}
}

func (repo *BufConfigRepository) Save(ctx context.Context, moduleVersionID domain.ModuleVersionID, config domain.BufConfigInfo) error {
	modulePaths, err := json.Marshal(config.ModulePaths)
	if err != nil {
		return err
	}
	deps, err := json.Marshal(config.Deps)
	if err != nil {
		return err
	}

	_, err = repo.db.executor(ctx).Exec(ctx, `
		INSERT INTO module_version_buf_configs (
			id,
			module_version_id,
			buf_yaml_present,
			buf_lock_present,
			buf_yaml_digest,
			buf_lock_digest,
			module_paths,
			deps,
			lint_enabled,
			breaking_config_present,
			created_at
		)
		VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (module_version_id) DO UPDATE SET
			buf_yaml_present = EXCLUDED.buf_yaml_present,
			buf_lock_present = EXCLUDED.buf_lock_present,
			buf_yaml_digest = EXCLUDED.buf_yaml_digest,
			buf_lock_digest = EXCLUDED.buf_lock_digest,
			module_paths = EXCLUDED.module_paths,
			deps = EXCLUDED.deps,
			lint_enabled = EXCLUDED.lint_enabled,
			breaking_config_present = EXCLUDED.breaking_config_present
	`,
		moduleVersionID.String(),
		config.BufYAMLPresent,
		config.BufLockPresent,
		config.BufYAMLDigest,
		config.BufLockDigest,
		modulePaths,
		deps,
		config.LintEnabled,
		config.BreakingConfigPresent,
		time.Now().UTC(),
	)
	return mapError(err)
}

func (repo *BufConfigRepository) GetByModuleVersion(ctx context.Context, moduleVersionID domain.ModuleVersionID) (domain.BufConfigInfo, error) {
	var config domain.BufConfigInfo
	var modulePaths []byte
	var deps []byte

	err := repo.db.executor(ctx).QueryRow(ctx, `
		SELECT
			buf_yaml_present,
			buf_lock_present,
			buf_yaml_digest,
			buf_lock_digest,
			module_paths,
			deps,
			lint_enabled,
			breaking_config_present
		FROM module_version_buf_configs
		WHERE module_version_id = $1
	`, moduleVersionID.String()).Scan(
		&config.BufYAMLPresent,
		&config.BufLockPresent,
		&config.BufYAMLDigest,
		&config.BufLockDigest,
		&modulePaths,
		&deps,
		&config.LintEnabled,
		&config.BreakingConfigPresent,
	)
	if err != nil {
		return domain.BufConfigInfo{}, mapError(err)
	}
	if err := json.Unmarshal(modulePaths, &config.ModulePaths); err != nil {
		return domain.BufConfigInfo{}, err
	}
	if err := json.Unmarshal(deps, &config.Deps); err != nil {
		return domain.BufConfigInfo{}, err
	}
	return config, nil
}

var _ domain.BufConfigRepository = (*BufConfigRepository)(nil)
