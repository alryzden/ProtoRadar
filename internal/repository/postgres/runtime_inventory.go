package postgres

import (
	"context"
	"database/sql"
	"time"

	"github.com/alryzden/ProtoRadar/internal/domain"
)

type RuntimeInventoryRepository struct {
	db *DB
}

func NewRuntimeInventoryRepository(db *DB) *RuntimeInventoryRepository {
	return &RuntimeInventoryRepository{db: db}
}

func (repo *RuntimeInventoryRepository) UpsertRuntimeServiceByName(ctx context.Context, service domain.RuntimeService) (domain.RuntimeService, error) {
	return repo.getRuntimeService(ctx, `
		INSERT INTO runtime_services (id, name, created_at, updated_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (name) DO UPDATE SET updated_at = EXCLUDED.updated_at
		RETURNING id, name, created_at, updated_at
	`, service.ID.String(), service.Name.String(), timeOrNow(service.CreatedAt), timeOrNow(service.UpdatedAt))
}

func (repo *RuntimeInventoryRepository) GetRuntimeServiceByName(ctx context.Context, serviceName domain.RuntimeServiceName) (domain.RuntimeService, error) {
	return repo.getRuntimeService(ctx, `
		SELECT id, name, created_at, updated_at
		FROM runtime_services
		WHERE name = $1
	`, serviceName.String())
}

func (repo *RuntimeInventoryRepository) CreateRuntimeDeployment(ctx context.Context, deployment domain.RuntimeDeployment) error {
	_, err := repo.db.executor(ctx).Exec(ctx, `
		INSERT INTO runtime_deployments (
			id,
			service_id,
			environment,
			git_commit,
			build_version,
			reported_at,
			created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`,
		deployment.ID.String(),
		deployment.ServiceID.String(),
		deployment.Environment.String(),
		deployment.GitCommit,
		deployment.BuildVersion,
		deployment.ReportedAt,
		timeOrNow(deployment.CreatedAt),
	)
	return mapError(err)
}

func (repo *RuntimeInventoryRepository) CreateRuntimeModuleUsages(ctx context.Context, usages []domain.RuntimeModuleUsage) error {
	return repo.db.WithinTransaction(ctx, func(txCtx context.Context) error {
		for _, usage := range usages {
			if err := repo.createRuntimeModuleUsage(txCtx, usage); err != nil {
				return err
			}
		}
		return nil
	})
}

func (repo *RuntimeInventoryRepository) ListRuntimeDeploymentsByService(ctx context.Context, serviceID domain.RuntimeServiceID, limit int, offset int) ([]domain.RuntimeDeployment, error) {
	return repo.listRuntimeDeployments(ctx, `
		WHERE rd.service_id = $1
		ORDER BY rd.reported_at DESC, rd.created_at DESC
		LIMIT $2 OFFSET $3
	`, serviceID.String(), limit, offset)
}

func (repo *RuntimeInventoryRepository) ListRuntimeModuleUsagesByDeployment(ctx context.Context, deploymentID domain.RuntimeDeploymentID) ([]domain.RuntimeModuleUsage, error) {
	return repo.listRuntimeModuleUsages(ctx, `
		WHERE rmu.deployment_id = $2
		ORDER BY rmu.module_name ASC, rmu.version ASC
	`, deploymentID.String())
}

func (repo *RuntimeInventoryRepository) ListLatestRuntimeUsagesByServiceEnvironment(ctx context.Context, serviceName domain.RuntimeServiceName, environment domain.RuntimeEnvironment) ([]domain.RuntimeModuleUsage, error) {
	return repo.listRuntimeModuleUsages(ctx, `
		JOIN runtime_deployments rd ON rd.id = rmu.deployment_id
		JOIN runtime_services rs ON rs.id = rd.service_id
		WHERE rd.id = (
			SELECT latest.id
			FROM runtime_deployments latest
			JOIN runtime_services latest_service ON latest_service.id = latest.service_id
			WHERE latest_service.name = $2 AND latest.environment = $3
			ORDER BY latest.reported_at DESC, latest.created_at DESC
			LIMIT 1
		)
		ORDER BY rmu.module_name ASC, rmu.version ASC
	`, serviceName.String(), environment.String())
}

func (repo *RuntimeInventoryRepository) ListRuntimeServices(ctx context.Context, limit int, offset int) ([]domain.RuntimeServiceSummary, error) {
	rows, err := repo.db.executor(ctx).Query(ctx, `
		WITH latest_deployments AS (
			SELECT DISTINCT ON (service_id, environment) id, service_id, environment, reported_at
			FROM runtime_deployments
			ORDER BY service_id, environment, reported_at DESC, created_at DESC
		), service_deployments AS (
			SELECT service_id, count(*) AS deployment_count, max(reported_at) AS latest_reported_at
			FROM runtime_deployments
			GROUP BY service_id
		)
		SELECT
			rs.id,
			rs.name,
			rs.created_at,
			rs.updated_at,
			COALESCE(array_agg(DISTINCT ld.environment ORDER BY ld.environment) FILTER (WHERE ld.environment IS NOT NULL), '{}'::text[]),
			count(DISTINCT ld.environment),
			COALESCE(sd.deployment_count, 0),
			count(rmu.id) FILTER (WHERE rmu.drift_status = $3),
			count(rmu.id) FILTER (WHERE rmu.drift_status = $4),
			count(rmu.id) FILTER (WHERE rmu.drift_status = $5),
			count(rmu.id) FILTER (WHERE rmu.drift_status = $6),
			sd.latest_reported_at
		FROM runtime_services rs
		LEFT JOIN service_deployments sd ON sd.service_id = rs.id
		LEFT JOIN latest_deployments ld ON ld.service_id = rs.id
		LEFT JOIN runtime_module_usages rmu ON rmu.deployment_id = ld.id
		GROUP BY rs.id, rs.name, rs.created_at, rs.updated_at, sd.deployment_count, sd.latest_reported_at
		ORDER BY rs.name ASC
		LIMIT $1 OFFSET $2
	`,
		limit,
		offset,
		domain.RuntimeDriftStatusUpToDate.String(),
		domain.RuntimeDriftStatusBehindLatest.String(),
		domain.RuntimeDriftStatusUnknownVersion.String(),
		domain.RuntimeDriftStatusDeprecatedVersion.String(),
	)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	summaries := make([]domain.RuntimeServiceSummary, 0)
	for rows.Next() {
		summary, err := scanRuntimeServiceSummary(rows.Scan)
		if err != nil {
			return nil, err
		}
		summaries = append(summaries, summary)
	}
	if err := rows.Err(); err != nil {
		return nil, mapError(err)
	}
	return summaries, nil
}

func (repo *RuntimeInventoryRepository) GetRuntimeServiceDetails(ctx context.Context, serviceName domain.RuntimeServiceName) (domain.RuntimeServiceDetails, error) {
	service, err := repo.GetRuntimeServiceByName(ctx, serviceName)
	if err != nil {
		return domain.RuntimeServiceDetails{}, err
	}
	deployments, err := repo.ListRuntimeDeploymentsByService(ctx, service.ID, 1000, 0)
	if err != nil {
		return domain.RuntimeServiceDetails{}, err
	}
	usages := make([]domain.RuntimeModuleUsage, 0)
	for _, deployment := range deployments {
		deploymentUsages, err := repo.ListRuntimeModuleUsagesByDeployment(ctx, deployment.ID)
		if err != nil {
			return domain.RuntimeServiceDetails{}, err
		}
		usages = append(usages, deploymentUsages...)
	}
	return domain.RuntimeServiceDetails{Service: service, Deployments: deployments, Usages: usages}, nil
}

func (repo *RuntimeInventoryRepository) ListRuntimeEnvironmentInventory(ctx context.Context, environment domain.RuntimeEnvironment, limit int, offset int) (domain.RuntimeEnvironmentInventory, error) {
	deployments, err := repo.listRuntimeDeployments(ctx, `
		WHERE rd.environment = $1
		ORDER BY rd.reported_at DESC, rd.created_at DESC
		LIMIT $2 OFFSET $3
	`, environment.String(), limit, offset)
	if err != nil {
		return domain.RuntimeEnvironmentInventory{}, err
	}
	usages := make([]domain.RuntimeModuleUsage, 0)
	for _, deployment := range deployments {
		deploymentUsages, err := repo.ListRuntimeModuleUsagesByDeployment(ctx, deployment.ID)
		if err != nil {
			return domain.RuntimeEnvironmentInventory{}, err
		}
		usages = append(usages, deploymentUsages...)
	}
	return domain.RuntimeEnvironmentInventory{Environment: environment, Deployments: deployments, Usages: usages}, nil
}

func (repo *RuntimeInventoryRepository) ListModuleRuntimeUsages(ctx context.Context, moduleID domain.ModuleID, limit int, offset int) ([]domain.ModuleRuntimeUsage, error) {
	return repo.listModuleRuntimeUsages(ctx, `
		WHERE rmu.module_id = $1
		ORDER BY rd.reported_at DESC, rs.name ASC, rd.environment ASC
		LIMIT $2 OFFSET $3
	`, moduleID.String(), limit, offset)
}

func (repo *RuntimeInventoryRepository) ListModuleRuntimeUsagesByModuleName(ctx context.Context, moduleName domain.ModuleName, limit int, offset int) ([]domain.ModuleRuntimeUsage, error) {
	return repo.listModuleRuntimeUsages(ctx, `
		WHERE rmu.module_name = $1
		ORDER BY rd.reported_at DESC, rs.name ASC, rd.environment ASC
		LIMIT $2 OFFSET $3
	`, moduleName.String(), limit, offset)
}

func (repo *RuntimeInventoryRepository) ListRuntimeModuleUsagesByDriftStatus(ctx context.Context, status domain.RuntimeDriftStatus, limit int, offset int) ([]domain.RuntimeModuleUsage, error) {
	return repo.listRuntimeModuleUsages(ctx, `
		WHERE rmu.drift_status = $2
		ORDER BY rmu.created_at DESC, rmu.module_name ASC
		LIMIT $3 OFFSET $4
	`, status.String(), limit, offset)
}

func (repo *RuntimeInventoryRepository) ListRuntimeImpactByModuleVersion(ctx context.Context, reportID domain.BreakingReportID, moduleVersionID domain.ModuleVersionID, limit int, offset int) ([]domain.RuntimeImpact, error) {
	_ = reportID
	rows, err := repo.db.executor(ctx).Query(ctx, `
		SELECT
			rs.name,
			rd.environment,
			rmu.module_name,
			rmu.version,
			rd.git_commit,
			rd.build_version,
			rd.reported_at,
			$4 AS impact_status,
			'exact runtime module version matches breaking report base version' AS reason
		FROM runtime_module_usages rmu
		JOIN runtime_deployments rd ON rd.id = rmu.deployment_id
		JOIN runtime_services rs ON rs.id = rd.service_id
		WHERE rmu.module_version_id = $1
		ORDER BY rd.reported_at DESC, rs.name ASC, rd.environment ASC
		LIMIT $2 OFFSET $3
	`, moduleVersionID.String(), limit, offset, domain.RuntimeImpactStatusPotentiallyAffectedByBreakingChange.String())
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	items := make([]domain.RuntimeImpact, 0)
	for rows.Next() {
		impact, err := scanRuntimeImpact(rows.Scan)
		if err != nil {
			return nil, err
		}
		items = append(items, impact)
	}
	if err := rows.Err(); err != nil {
		return nil, mapError(err)
	}
	return items, nil
}

func (repo *RuntimeInventoryRepository) createRuntimeModuleUsage(ctx context.Context, usage domain.RuntimeModuleUsage) error {
	_, err := repo.db.executor(ctx).Exec(ctx, `
		INSERT INTO runtime_module_usages (
			id,
			deployment_id,
			module_id,
			module_name,
			module_version_id,
			version,
			drift_status,
			drift_reason,
			created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`,
		usage.ID.String(),
		usage.DeploymentID.String(),
		nullableModuleID(usage.ModuleID),
		usage.ModuleName.String(),
		nullableModuleVersionID(usage.ModuleVersionID),
		usage.Version.String(),
		usage.DriftStatus.String(),
		usage.DriftReason,
		timeOrNow(usage.CreatedAt),
	)
	return mapError(err)
}

func (repo *RuntimeInventoryRepository) getRuntimeService(ctx context.Context, query string, args ...any) (domain.RuntimeService, error) {
	service, err := scanRuntimeService(repo.db.executor(ctx).QueryRow(ctx, query, args...).Scan)
	if err != nil {
		return domain.RuntimeService{}, mapError(err)
	}
	return service, nil
}

func (repo *RuntimeInventoryRepository) listRuntimeDeployments(ctx context.Context, where string, args ...any) ([]domain.RuntimeDeployment, error) {
	rows, err := repo.db.executor(ctx).Query(ctx, `
		SELECT
			rd.id,
			rd.service_id,
			rs.name,
			rd.environment,
			rd.git_commit,
			rd.build_version,
			rd.reported_at,
			rd.created_at
		FROM runtime_deployments rd
		JOIN runtime_services rs ON rs.id = rd.service_id
		`+where, args...)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	deployments := make([]domain.RuntimeDeployment, 0)
	for rows.Next() {
		deployment, err := scanRuntimeDeployment(rows.Scan)
		if err != nil {
			return nil, err
		}
		deployments = append(deployments, deployment)
	}
	if err := rows.Err(); err != nil {
		return nil, mapError(err)
	}
	return deployments, nil
}

func (repo *RuntimeInventoryRepository) listRuntimeModuleUsages(ctx context.Context, clause string, args ...any) ([]domain.RuntimeModuleUsage, error) {
	rows, err := repo.db.executor(ctx).Query(ctx, `
		SELECT
			rmu.id,
			rmu.deployment_id,
			rmu.module_id,
			rmu.module_name,
			rmu.module_version_id,
			rmu.version,
			latest.version,
			rmu.drift_status,
			rmu.drift_reason,
			rmu.created_at
		FROM runtime_module_usages rmu
		LEFT JOIN LATERAL (
			SELECT mv.version
			FROM module_versions mv
			WHERE mv.module_id = rmu.module_id AND mv.status = $1
			ORDER BY mv.created_at DESC
			LIMIT 1
		) latest ON true
		`+clause, append([]any{domain.ModuleVersionStatusPublished.String()}, args...)...)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	usages := make([]domain.RuntimeModuleUsage, 0)
	for rows.Next() {
		usage, err := scanRuntimeModuleUsage(rows.Scan)
		if err != nil {
			return nil, err
		}
		usages = append(usages, usage)
	}
	if err := rows.Err(); err != nil {
		return nil, mapError(err)
	}
	return usages, nil
}

func (repo *RuntimeInventoryRepository) listModuleRuntimeUsages(ctx context.Context, where string, args ...any) ([]domain.ModuleRuntimeUsage, error) {
	rows, err := repo.db.executor(ctx).Query(ctx, `
		SELECT
			rs.name,
			rd.environment,
			rd.id,
			rmu.module_name,
			rmu.version,
			rd.git_commit,
			rd.build_version,
			rd.reported_at,
			rmu.drift_status,
			rmu.drift_reason
		FROM runtime_module_usages rmu
		JOIN runtime_deployments rd ON rd.id = rmu.deployment_id
		JOIN runtime_services rs ON rs.id = rd.service_id
		`+where, args...)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	items := make([]domain.ModuleRuntimeUsage, 0)
	for rows.Next() {
		item, err := scanModuleRuntimeUsage(rows.Scan)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, mapError(err)
	}
	return items, nil
}

func scanRuntimeService(scan func(dest ...any) error) (domain.RuntimeService, error) {
	var id string
	var nameValue string
	var createdAt time.Time
	var updatedAt time.Time
	if err := scan(&id, &nameValue, &createdAt, &updatedAt); err != nil {
		return domain.RuntimeService{}, err
	}
	name, err := domain.NewRuntimeServiceName(nameValue)
	if err != nil {
		return domain.RuntimeService{}, err
	}
	return domain.RuntimeService{ID: domain.NewRuntimeServiceID(id), Name: name, CreatedAt: createdAt, UpdatedAt: updatedAt}, nil
}

func scanRuntimeDeployment(scan func(dest ...any) error) (domain.RuntimeDeployment, error) {
	var id string
	var serviceID string
	var serviceNameValue string
	var environmentValue string
	var deployment domain.RuntimeDeployment
	if err := scan(&id, &serviceID, &serviceNameValue, &environmentValue, &deployment.GitCommit, &deployment.BuildVersion, &deployment.ReportedAt, &deployment.CreatedAt); err != nil {
		return domain.RuntimeDeployment{}, err
	}
	serviceName, err := domain.NewRuntimeServiceName(serviceNameValue)
	if err != nil {
		return domain.RuntimeDeployment{}, err
	}
	environment, err := domain.NewRuntimeEnvironment(environmentValue)
	if err != nil {
		return domain.RuntimeDeployment{}, err
	}
	deployment.ID = domain.NewRuntimeDeploymentID(id)
	deployment.ServiceID = domain.NewRuntimeServiceID(serviceID)
	deployment.ServiceName = serviceName
	deployment.Environment = environment
	return deployment, nil
}

func scanRuntimeModuleUsage(scan func(dest ...any) error) (domain.RuntimeModuleUsage, error) {
	var id string
	var deploymentID string
	var moduleID sql.NullString
	var moduleNameValue string
	var moduleVersionID sql.NullString
	var versionValue string
	var latestVersionValue sql.NullString
	var driftStatusValue string
	var usage domain.RuntimeModuleUsage
	if err := scan(&id, &deploymentID, &moduleID, &moduleNameValue, &moduleVersionID, &versionValue, &latestVersionValue, &driftStatusValue, &usage.DriftReason, &usage.CreatedAt); err != nil {
		return domain.RuntimeModuleUsage{}, err
	}
	moduleName, err := domain.NewModuleName(moduleNameValue)
	if err != nil {
		return domain.RuntimeModuleUsage{}, err
	}
	version, err := domain.NewVersion(versionValue)
	if err != nil {
		return domain.RuntimeModuleUsage{}, err
	}
	status, err := domain.NewRuntimeDriftStatus(driftStatusValue)
	if err != nil {
		return domain.RuntimeModuleUsage{}, err
	}
	usage.ID = domain.NewRuntimeModuleUsageID(id)
	usage.DeploymentID = domain.NewRuntimeDeploymentID(deploymentID)
	usage.ModuleID = runtimeModuleID(moduleID)
	usage.ModuleName = moduleName
	usage.ModuleVersionID = runtimeModuleVersionID(moduleVersionID)
	usage.Version = version
	usage.LatestVersion = runtimeVersion(latestVersionValue)
	usage.DriftStatus = status
	return usage, nil
}

func scanRuntimeServiceSummary(scan func(dest ...any) error) (domain.RuntimeServiceSummary, error) {
	var id string
	var nameValue string
	var createdAt time.Time
	var updatedAt time.Time
	var environmentValues []string
	var latestReportedAt sql.NullTime
	var summary domain.RuntimeServiceSummary
	if err := scan(
		&id,
		&nameValue,
		&createdAt,
		&updatedAt,
		&environmentValues,
		&summary.EnvironmentCount,
		&summary.DeploymentCount,
		&summary.UpToDateCount,
		&summary.BehindLatestCount,
		&summary.UnknownVersionCount,
		&summary.DeprecatedCount,
		&latestReportedAt,
	); err != nil {
		return domain.RuntimeServiceSummary{}, err
	}
	name, err := domain.NewRuntimeServiceName(nameValue)
	if err != nil {
		return domain.RuntimeServiceSummary{}, err
	}
	summary.Service = domain.RuntimeService{ID: domain.NewRuntimeServiceID(id), Name: name, CreatedAt: createdAt, UpdatedAt: updatedAt}
	summary.Environments = make([]domain.RuntimeEnvironment, 0, len(environmentValues))
	for _, value := range environmentValues {
		environment, err := domain.NewRuntimeEnvironment(value)
		if err != nil {
			return domain.RuntimeServiceSummary{}, err
		}
		summary.Environments = append(summary.Environments, environment)
	}
	if latestReportedAt.Valid {
		summary.LatestReportedAt = &latestReportedAt.Time
	}
	return summary, nil
}

func scanModuleRuntimeUsage(scan func(dest ...any) error) (domain.ModuleRuntimeUsage, error) {
	var serviceNameValue string
	var environmentValue string
	var deploymentID string
	var moduleNameValue string
	var versionValue string
	var statusValue string
	var item domain.ModuleRuntimeUsage
	if err := scan(&serviceNameValue, &environmentValue, &deploymentID, &moduleNameValue, &versionValue, &item.GitCommit, &item.BuildVersion, &item.ReportedAt, &statusValue, &item.DriftReason); err != nil {
		return domain.ModuleRuntimeUsage{}, err
	}
	serviceName, err := domain.NewRuntimeServiceName(serviceNameValue)
	if err != nil {
		return domain.ModuleRuntimeUsage{}, err
	}
	environment, err := domain.NewRuntimeEnvironment(environmentValue)
	if err != nil {
		return domain.ModuleRuntimeUsage{}, err
	}
	moduleName, err := domain.NewModuleName(moduleNameValue)
	if err != nil {
		return domain.ModuleRuntimeUsage{}, err
	}
	version, err := domain.NewVersion(versionValue)
	if err != nil {
		return domain.ModuleRuntimeUsage{}, err
	}
	status, err := domain.NewRuntimeDriftStatus(statusValue)
	if err != nil {
		return domain.ModuleRuntimeUsage{}, err
	}
	item.ServiceName = serviceName
	item.Environment = environment
	item.DeploymentID = domain.NewRuntimeDeploymentID(deploymentID)
	item.ModuleName = moduleName
	item.Version = version
	item.DriftStatus = status
	return item, nil
}

func scanRuntimeImpact(scan func(dest ...any) error) (domain.RuntimeImpact, error) {
	var serviceNameValue string
	var environmentValue string
	var moduleNameValue string
	var versionValue string
	var impactStatusValue string
	var item domain.RuntimeImpact
	if err := scan(&serviceNameValue, &environmentValue, &moduleNameValue, &versionValue, &item.GitCommit, &item.BuildVersion, &item.ReportedAt, &impactStatusValue, &item.Reason); err != nil {
		return domain.RuntimeImpact{}, err
	}
	serviceName, err := domain.NewRuntimeServiceName(serviceNameValue)
	if err != nil {
		return domain.RuntimeImpact{}, err
	}
	environment, err := domain.NewRuntimeEnvironment(environmentValue)
	if err != nil {
		return domain.RuntimeImpact{}, err
	}
	moduleName, err := domain.NewModuleName(moduleNameValue)
	if err != nil {
		return domain.RuntimeImpact{}, err
	}
	version, err := domain.NewVersion(versionValue)
	if err != nil {
		return domain.RuntimeImpact{}, err
	}
	item.ServiceName = serviceName
	item.Environment = environment
	item.UsedModule = moduleName
	item.UsedVersion = version
	item.ImpactStatus = domain.RuntimeImpactStatus(impactStatusValue)
	return item, nil
}

func nullableModuleID(id *domain.ModuleID) any {
	if id == nil {
		return nil
	}
	return id.String()
}

func nullableModuleVersionID(id *domain.ModuleVersionID) any {
	if id == nil {
		return nil
	}
	return id.String()
}

func runtimeModuleID(value sql.NullString) *domain.ModuleID {
	if !value.Valid {
		return nil
	}
	id := domain.NewModuleID(value.String)
	return &id
}

func runtimeModuleVersionID(value sql.NullString) *domain.ModuleVersionID {
	if !value.Valid {
		return nil
	}
	id := domain.NewModuleVersionID(value.String)
	return &id
}

func runtimeVersion(value sql.NullString) *domain.Version {
	if !value.Valid {
		return nil
	}
	version, err := domain.NewVersion(value.String)
	if err != nil {
		return nil
	}
	return &version
}

var _ domain.RuntimeInventoryRepository = (*RuntimeInventoryRepository)(nil)
