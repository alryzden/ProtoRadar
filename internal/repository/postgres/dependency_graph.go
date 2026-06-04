package postgres

import (
	"context"
	"time"

	"github.com/alryzden/ProtoRadar/internal/domain"
	"github.com/alryzden/ProtoRadar/internal/usecase/registry"
)

type ModuleDependencyRepository struct {
	db *DB
}

func NewModuleDependencyRepository(db *DB) *ModuleDependencyRepository {
	return &ModuleDependencyRepository{db: db}
}

func (repo *ModuleDependencyRepository) ReplaceByConsumerModuleVersion(ctx context.Context, consumerModuleVersionID domain.ModuleVersionID, dependencies []domain.ModuleDependency, unresolved []domain.UnresolvedProtoDependency) error {
	return repo.db.WithinTransaction(ctx, func(txCtx context.Context) error {
		if _, err := repo.db.executor(txCtx).Exec(txCtx, `DELETE FROM module_dependencies WHERE consumer_module_version_id = $1`, consumerModuleVersionID.String()); err != nil {
			return mapError(err)
		}
		if _, err := repo.db.executor(txCtx).Exec(txCtx, `DELETE FROM unresolved_proto_dependencies WHERE module_version_id = $1`, consumerModuleVersionID.String()); err != nil {
			return mapError(err)
		}
		for _, dependency := range dependencies {
			if err := repo.createDependency(txCtx, dependency); err != nil {
				return err
			}
		}
		for _, dependency := range unresolved {
			if err := repo.createUnresolvedDependency(txCtx, dependency); err != nil {
				return err
			}
		}
		return nil
	})
}

func (repo *ModuleDependencyRepository) ListUpstreamByModule(ctx context.Context, moduleID domain.ModuleID) ([]domain.ModuleDependency, error) {
	return repo.listDependencies(ctx, `
		WHERE md.consumer_module_id = $1
		ORDER BY cv.created_at DESC, pm.name ASC, pv.version ASC, md.source ASC, md.import_path ASC, md.referenced_symbol ASC
	`, moduleID.String())
}

func (repo *ModuleDependencyRepository) ListUpstreamByModuleVersion(ctx context.Context, moduleVersionID domain.ModuleVersionID) ([]domain.ModuleDependency, error) {
	return repo.listDependencies(ctx, `
		WHERE md.consumer_module_version_id = $1
		ORDER BY pm.name ASC, pv.version ASC, md.source ASC, md.import_path ASC, md.referenced_symbol ASC
	`, moduleVersionID.String())
}

func (repo *ModuleDependencyRepository) ListDownstreamByModule(ctx context.Context, moduleID domain.ModuleID) ([]domain.ModuleDependency, error) {
	return repo.listDependencies(ctx, `
		WHERE md.provider_module_id = $1
		ORDER BY cm.name ASC, cv.version ASC, md.source ASC, md.import_path ASC, md.referenced_symbol ASC
	`, moduleID.String())
}

func (repo *ModuleDependencyRepository) ListAffectedModules(ctx context.Context, providerModuleID domain.ModuleID) ([]domain.AffectedModule, error) {
	rows, err := repo.db.executor(ctx).Query(ctx, `
		WITH latest_consumer_versions AS (
			SELECT DISTINCT ON (module_id) id, module_id, version, created_at
			FROM module_versions
			WHERE status = $2
			ORDER BY module_id, created_at DESC
		)
		SELECT
			cm.name,
			lcv.version,
			array_agg(DISTINCT md.source ORDER BY md.source),
			COALESCE(array_agg(DISTINCT md.reason ORDER BY md.reason) FILTER (WHERE md.reason <> ''), '{}'::text[])
		FROM module_dependencies md
		JOIN latest_consumer_versions lcv ON lcv.id = md.consumer_module_version_id
		JOIN modules cm ON cm.id = md.consumer_module_id
		WHERE md.provider_module_id = $1
		GROUP BY cm.name, lcv.version
		ORDER BY cm.name ASC
	`, providerModuleID.String(), domain.ModuleVersionStatusPublished.String())
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	items := make([]domain.AffectedModule, 0)
	for rows.Next() {
		var moduleNameValue string
		var versionValue string
		var sourceValues []string
		var reasonValues []string
		if err := rows.Scan(&moduleNameValue, &versionValue, &sourceValues, &reasonValues); err != nil {
			return nil, err
		}
		moduleName, err := domain.NewModuleName(moduleNameValue)
		if err != nil {
			return nil, err
		}
		version, err := domain.NewVersion(versionValue)
		if err != nil {
			return nil, err
		}
		items = append(items, domain.AffectedModule{
			ModuleName:        moduleName,
			LatestVersion:     version,
			DependencySources: dependencySources(sourceValues),
			Reasons:           dependencyResolutionReasons(reasonValues),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, mapError(err)
	}
	return items, nil
}

func (repo *ModuleDependencyRepository) ListUnresolvedByModule(ctx context.Context, moduleID domain.ModuleID) ([]domain.UnresolvedProtoDependency, error) {
	return repo.listUnresolvedDependencies(ctx, `
		WHERE upd.module_id = $1
		ORDER BY mv.created_at DESC, upd.source ASC, upd.import_path ASC, upd.referenced_symbol ASC
	`, moduleID.String())
}

func (repo *ModuleDependencyRepository) ListUnresolvedByModuleVersion(ctx context.Context, moduleVersionID domain.ModuleVersionID) ([]domain.UnresolvedProtoDependency, error) {
	return repo.listUnresolvedDependencies(ctx, `
		WHERE upd.module_version_id = $1
		ORDER BY upd.source ASC, upd.import_path ASC, upd.referenced_symbol ASC
	`, moduleVersionID.String())
}

func (repo *ModuleDependencyRepository) ListLatestPublishedModuleVersions(ctx context.Context) ([]domain.ModuleVersion, error) {
	rows, err := repo.db.executor(ctx).Query(ctx, `
		SELECT DISTINCT ON (module_id) id, module_id, version, digest, status, created_at
		FROM module_versions
		WHERE status = $1
		ORDER BY module_id, created_at DESC
	`, domain.ModuleVersionStatusPublished.String())
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

func (repo *ModuleDependencyRepository) ListProviderFiles(ctx context.Context, moduleVersionIDs []domain.ModuleVersionID) ([]registry.ProviderFile, error) {
	ids := moduleVersionIDStrings(moduleVersionIDs)
	if len(ids) == 0 {
		return []registry.ProviderFile{}, nil
	}
	rows, err := repo.db.executor(ctx).Query(ctx, `
		SELECT pf.path, m.id, m.name, mv.id, mv.version
		FROM proto_files pf
		JOIN module_versions mv ON mv.id = pf.module_version_id
		JOIN modules m ON m.id = mv.module_id
		WHERE pf.module_version_id = ANY($1::uuid[])
		ORDER BY pf.path ASC, m.name ASC
	`, ids)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	items := make([]registry.ProviderFile, 0)
	for rows.Next() {
		file, err := scanProviderFile(rows.Scan)
		if err != nil {
			return nil, err
		}
		items = append(items, file)
	}
	if err := rows.Err(); err != nil {
		return nil, mapError(err)
	}
	return items, nil
}

func (repo *ModuleDependencyRepository) ListProviderSymbols(ctx context.Context, moduleVersionIDs []domain.ModuleVersionID) ([]registry.ProviderSymbol, error) {
	ids := moduleVersionIDStrings(moduleVersionIDs)
	if len(ids) == 0 {
		return []registry.ProviderSymbol{}, nil
	}
	rows, err := repo.db.executor(ctx).Query(ctx, `
		SELECT full_name, package_name, module_id, module_name, module_version_id, version
		FROM (
			SELECT pm.full_name, pm.package_name, m.id AS module_id, m.name AS module_name, mv.id AS module_version_id, mv.version
			FROM proto_messages pm
			JOIN module_versions mv ON mv.id = pm.module_version_id
			JOIN modules m ON m.id = mv.module_id
			WHERE pm.module_version_id = ANY($1::uuid[])
			UNION ALL
			SELECT pe.full_name, pe.package_name, m.id AS module_id, m.name AS module_name, mv.id AS module_version_id, mv.version
			FROM proto_enums pe
			JOIN module_versions mv ON mv.id = pe.module_version_id
			JOIN modules m ON m.id = mv.module_id
			WHERE pe.module_version_id = ANY($1::uuid[])
			UNION ALL
			SELECT ps.full_name, ps.package_name, m.id AS module_id, m.name AS module_name, mv.id AS module_version_id, mv.version
			FROM proto_services ps
			JOIN module_versions mv ON mv.id = ps.module_version_id
			JOIN modules m ON m.id = mv.module_id
			WHERE ps.module_version_id = ANY($1::uuid[])
		) symbols
		ORDER BY full_name ASC, module_name ASC
	`, ids)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	items := make([]registry.ProviderSymbol, 0)
	for rows.Next() {
		symbol, err := scanProviderSymbol(rows.Scan)
		if err != nil {
			return nil, err
		}
		items = append(items, symbol)
	}
	if err := rows.Err(); err != nil {
		return nil, mapError(err)
	}
	return items, nil
}

func (repo *ModuleDependencyRepository) createDependency(ctx context.Context, dependency domain.ModuleDependency) error {
	_, err := repo.db.executor(ctx).Exec(ctx, `
		INSERT INTO module_dependencies (
			id,
			consumer_module_id,
			consumer_module_version_id,
			provider_module_id,
			provider_module_version_id,
			source,
			reason,
			import_path,
			referenced_package,
			referenced_symbol,
			created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`,
		dependency.ID.String(),
		dependency.ConsumerModuleID.String(),
		dependency.ConsumerModuleVersionID.String(),
		dependency.ProviderModuleID.String(),
		dependency.ProviderModuleVersionID.String(),
		dependency.Source.String(),
		dependency.Reason.String(),
		dependency.ImportPath,
		dependency.ReferencedPackage,
		dependency.ReferencedSymbol,
		timeOrNow(dependency.CreatedAt),
	)
	return mapError(err)
}

func (repo *ModuleDependencyRepository) createUnresolvedDependency(ctx context.Context, dependency domain.UnresolvedProtoDependency) error {
	_, err := repo.db.executor(ctx).Exec(ctx, `
		INSERT INTO unresolved_proto_dependencies (
			id,
			module_id,
			module_version_id,
			source,
			import_path,
			referenced_package,
			referenced_symbol,
			reason,
			created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`,
		dependency.ID.String(),
		dependency.ModuleID.String(),
		dependency.ModuleVersionID.String(),
		dependency.Source.String(),
		dependency.ImportPath,
		dependency.ReferencedPackage,
		dependency.ReferencedSymbol,
		dependency.Reason.String(),
		timeOrNow(dependency.CreatedAt),
	)
	return mapError(err)
}

func (repo *ModuleDependencyRepository) listDependencies(ctx context.Context, where string, args ...any) ([]domain.ModuleDependency, error) {
	rows, err := repo.db.executor(ctx).Query(ctx, `
		SELECT
			md.id,
			md.consumer_module_id,
			cm.name,
			md.consumer_module_version_id,
			cv.version,
			md.provider_module_id,
			pm.name,
			md.provider_module_version_id,
			pv.version,
			md.source,
			md.reason,
			md.import_path,
			md.referenced_package,
			md.referenced_symbol,
			md.created_at
		FROM module_dependencies md
		JOIN modules cm ON cm.id = md.consumer_module_id
		JOIN module_versions cv ON cv.id = md.consumer_module_version_id
		JOIN modules pm ON pm.id = md.provider_module_id
		JOIN module_versions pv ON pv.id = md.provider_module_version_id
		`+where, args...)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	dependencies := make([]domain.ModuleDependency, 0)
	for rows.Next() {
		dependency, err := scanModuleDependency(rows.Scan)
		if err != nil {
			return nil, err
		}
		dependencies = append(dependencies, dependency)
	}
	if err := rows.Err(); err != nil {
		return nil, mapError(err)
	}
	return dependencies, nil
}

func (repo *ModuleDependencyRepository) listUnresolvedDependencies(ctx context.Context, where string, args ...any) ([]domain.UnresolvedProtoDependency, error) {
	rows, err := repo.db.executor(ctx).Query(ctx, `
		SELECT
			upd.id,
			upd.module_id,
			m.name,
			upd.module_version_id,
			mv.version,
			upd.source,
			upd.import_path,
			upd.referenced_package,
			upd.referenced_symbol,
			upd.reason,
			upd.created_at
		FROM unresolved_proto_dependencies upd
		JOIN modules m ON m.id = upd.module_id
		JOIN module_versions mv ON mv.id = upd.module_version_id
		`+where, args...)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	dependencies := make([]domain.UnresolvedProtoDependency, 0)
	for rows.Next() {
		dependency, err := scanUnresolvedProtoDependency(rows.Scan)
		if err != nil {
			return nil, err
		}
		dependencies = append(dependencies, dependency)
	}
	if err := rows.Err(); err != nil {
		return nil, mapError(err)
	}
	return dependencies, nil
}

func scanModuleDependency(scan func(dest ...any) error) (domain.ModuleDependency, error) {
	var dependency domain.ModuleDependency
	var id string
	var consumerModuleID string
	var consumerModuleNameValue string
	var consumerModuleVersionID string
	var consumerVersionValue string
	var providerModuleID string
	var providerModuleNameValue string
	var providerModuleVersionID string
	var providerVersionValue string
	var sourceValue string
	var reasonValue string

	if err := scan(
		&id,
		&consumerModuleID,
		&consumerModuleNameValue,
		&consumerModuleVersionID,
		&consumerVersionValue,
		&providerModuleID,
		&providerModuleNameValue,
		&providerModuleVersionID,
		&providerVersionValue,
		&sourceValue,
		&reasonValue,
		&dependency.ImportPath,
		&dependency.ReferencedPackage,
		&dependency.ReferencedSymbol,
		&dependency.CreatedAt,
	); err != nil {
		return domain.ModuleDependency{}, err
	}

	consumerModuleName, err := domain.NewModuleName(consumerModuleNameValue)
	if err != nil {
		return domain.ModuleDependency{}, err
	}
	consumerVersion, err := domain.NewVersion(consumerVersionValue)
	if err != nil {
		return domain.ModuleDependency{}, err
	}
	providerModuleName, err := domain.NewModuleName(providerModuleNameValue)
	if err != nil {
		return domain.ModuleDependency{}, err
	}
	providerVersion, err := domain.NewVersion(providerVersionValue)
	if err != nil {
		return domain.ModuleDependency{}, err
	}

	dependency.ID = domain.NewModuleDependencyID(id)
	dependency.ConsumerModuleID = domain.NewModuleID(consumerModuleID)
	dependency.ConsumerModuleName = consumerModuleName
	dependency.ConsumerModuleVersionID = domain.NewModuleVersionID(consumerModuleVersionID)
	dependency.ConsumerVersion = consumerVersion
	dependency.ProviderModuleID = domain.NewModuleID(providerModuleID)
	dependency.ProviderModuleName = providerModuleName
	dependency.ProviderModuleVersionID = domain.NewModuleVersionID(providerModuleVersionID)
	dependency.ProviderVersion = providerVersion
	dependency.Source = domain.DependencySource(sourceValue)
	dependency.Reason = domain.DependencyResolutionReason(reasonValue)
	return dependency, nil
}

func scanUnresolvedProtoDependency(scan func(dest ...any) error) (domain.UnresolvedProtoDependency, error) {
	var dependency domain.UnresolvedProtoDependency
	var id string
	var moduleID string
	var moduleNameValue string
	var moduleVersionID string
	var versionValue string
	var sourceValue string
	var reasonValue string

	if err := scan(
		&id,
		&moduleID,
		&moduleNameValue,
		&moduleVersionID,
		&versionValue,
		&sourceValue,
		&dependency.ImportPath,
		&dependency.ReferencedPackage,
		&dependency.ReferencedSymbol,
		&reasonValue,
		&dependency.CreatedAt,
	); err != nil {
		return domain.UnresolvedProtoDependency{}, err
	}

	moduleName, err := domain.NewModuleName(moduleNameValue)
	if err != nil {
		return domain.UnresolvedProtoDependency{}, err
	}
	version, err := domain.NewVersion(versionValue)
	if err != nil {
		return domain.UnresolvedProtoDependency{}, err
	}

	dependency.ID = domain.NewUnresolvedProtoDependencyID(id)
	dependency.ModuleID = domain.NewModuleID(moduleID)
	dependency.ModuleName = moduleName
	dependency.ModuleVersionID = domain.NewModuleVersionID(moduleVersionID)
	dependency.Version = version
	dependency.Source = domain.DependencySource(sourceValue)
	dependency.Reason = domain.UnresolvedDependencyReason(reasonValue)
	return dependency, nil
}

func scanProviderFile(scan func(dest ...any) error) (registry.ProviderFile, error) {
	var file registry.ProviderFile
	var moduleID string
	var moduleNameValue string
	var moduleVersionID string
	var versionValue string
	if err := scan(&file.Path, &moduleID, &moduleNameValue, &moduleVersionID, &versionValue); err != nil {
		return registry.ProviderFile{}, err
	}
	moduleName, err := domain.NewModuleName(moduleNameValue)
	if err != nil {
		return registry.ProviderFile{}, err
	}
	version, err := domain.NewVersion(versionValue)
	if err != nil {
		return registry.ProviderFile{}, err
	}
	file.ModuleID = domain.NewModuleID(moduleID)
	file.ModuleName = moduleName
	file.ModuleVersionID = domain.NewModuleVersionID(moduleVersionID)
	file.Version = version
	return file, nil
}

func scanProviderSymbol(scan func(dest ...any) error) (registry.ProviderSymbol, error) {
	var symbol registry.ProviderSymbol
	var moduleID string
	var moduleNameValue string
	var moduleVersionID string
	var versionValue string
	if err := scan(&symbol.FullName, &symbol.PackageName, &moduleID, &moduleNameValue, &moduleVersionID, &versionValue); err != nil {
		return registry.ProviderSymbol{}, err
	}
	moduleName, err := domain.NewModuleName(moduleNameValue)
	if err != nil {
		return registry.ProviderSymbol{}, err
	}
	version, err := domain.NewVersion(versionValue)
	if err != nil {
		return registry.ProviderSymbol{}, err
	}
	symbol.ModuleID = domain.NewModuleID(moduleID)
	symbol.ModuleName = moduleName
	symbol.ModuleVersionID = domain.NewModuleVersionID(moduleVersionID)
	symbol.Version = version
	return symbol, nil
}

func dependencySources(values []string) []domain.DependencySource {
	items := make([]domain.DependencySource, 0, len(values))
	for _, value := range values {
		items = append(items, domain.DependencySource(value))
	}
	return items
}

func dependencyResolutionReasons(values []string) []domain.DependencyResolutionReason {
	items := make([]domain.DependencyResolutionReason, 0, len(values))
	for _, value := range values {
		items = append(items, domain.DependencyResolutionReason(value))
	}
	return items
}

func moduleVersionIDStrings(ids []domain.ModuleVersionID) []string {
	values := make([]string, 0, len(ids))
	for _, id := range ids {
		values = append(values, id.String())
	}
	return values
}

func timeOrNow(value time.Time) time.Time {
	if value.IsZero() {
		return time.Now().UTC()
	}
	return value
}

var _ domain.ModuleDependencyRepository = (*ModuleDependencyRepository)(nil)
var _ registry.DependencyProviderIndexRepository = (*ModuleDependencyRepository)(nil)
