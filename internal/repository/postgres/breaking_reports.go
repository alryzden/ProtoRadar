package postgres

import (
	"context"

	"github.com/alryzden/ProtoRadar/internal/domain"
)

type BreakingReportRepository struct {
	db *DB
}

func NewBreakingReportRepository(db *DB) *BreakingReportRepository {
	return &BreakingReportRepository{db: db}
}

func (repo *BreakingReportRepository) Create(ctx context.Context, report domain.BreakingReport, changes []domain.BreakingChange) error {
	return repo.db.WithinTransaction(ctx, func(txCtx context.Context) error {
		_, err := repo.db.executor(txCtx).Exec(txCtx, `
			INSERT INTO breaking_reports (
				id,
				module_id,
				base_version_id,
				target_ref,
				status,
				change_count,
				raw_output,
				human_summary,
				created_at
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		`,
			report.ID.String(),
			report.ModuleID.String(),
			report.BaseVersionID.String(),
			report.TargetRef,
			report.Status.String(),
			report.ChangeCount,
			report.RawOutput,
			report.HumanSummary,
			report.CreatedAt,
		)
		if err != nil {
			return mapError(err)
		}
		for _, change := range changes {
			if err := repo.createChange(txCtx, report.ID, change); err != nil {
				return err
			}
		}
		return nil
	})
}

func (repo *BreakingReportRepository) GetByID(ctx context.Context, id domain.BreakingReportID) (domain.BreakingReport, []domain.BreakingChange, error) {
	report, err := repo.getOne(ctx, `
		SELECT
			br.id,
			br.module_id,
			m.name,
			br.base_version_id,
			mv.version,
			br.target_ref,
			br.status,
			br.change_count,
			br.raw_output,
			br.human_summary,
			br.created_at
		FROM breaking_reports br
		JOIN modules m ON m.id = br.module_id
		JOIN module_versions mv ON mv.id = br.base_version_id
		WHERE br.id = $1
	`, id.String())
	if err != nil {
		return domain.BreakingReport{}, nil, err
	}
	changes, err := repo.ListChangesByReport(ctx, id, 1000, 0)
	if err != nil {
		return domain.BreakingReport{}, nil, err
	}
	return report, changes, nil
}

func (repo *BreakingReportRepository) ListByModule(ctx context.Context, moduleID domain.ModuleID, limit int, offset int) ([]domain.BreakingReport, error) {
	rows, err := repo.db.executor(ctx).Query(ctx, `
		SELECT
			br.id,
			br.module_id,
			m.name,
			br.base_version_id,
			mv.version,
			br.target_ref,
			br.status,
			br.change_count,
			br.raw_output,
			br.human_summary,
			br.created_at
		FROM breaking_reports br
		JOIN modules m ON m.id = br.module_id
		JOIN module_versions mv ON mv.id = br.base_version_id
		WHERE br.module_id = $1
		ORDER BY br.created_at DESC
		LIMIT $2 OFFSET $3
	`, moduleID.String(), limit, offset)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	reports := make([]domain.BreakingReport, 0)
	for rows.Next() {
		report, err := scanBreakingReport(rows.Scan)
		if err != nil {
			return nil, err
		}
		reports = append(reports, report)
	}
	if err := rows.Err(); err != nil {
		return nil, mapError(err)
	}
	return reports, nil
}

func (repo *BreakingReportRepository) CountChangesByReport(ctx context.Context, reportID domain.BreakingReportID) (int, error) {
	var count int
	err := repo.db.executor(ctx).QueryRow(ctx, `
		SELECT count(*)
		FROM breaking_changes
		WHERE report_id = $1
	`, reportID.String()).Scan(&count)
	if err != nil {
		return 0, mapError(err)
	}
	return count, nil
}

func (repo *BreakingReportRepository) ListChangesByReport(ctx context.Context, reportID domain.BreakingReportID, limit int, offset int) ([]domain.BreakingChange, error) {
	rows, err := repo.db.executor(ctx).Query(ctx, `
		SELECT
			id,
			report_id,
			category,
			file_path,
			package_name,
			symbol,
			rule_id,
			message,
			severity,
			created_at
		FROM breaking_changes
		WHERE report_id = $1
		ORDER BY created_at ASC, id ASC
		LIMIT $2 OFFSET $3
	`, reportID.String(), limit, offset)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	changes := make([]domain.BreakingChange, 0)
	for rows.Next() {
		change, err := scanBreakingChange(rows.Scan)
		if err != nil {
			return nil, err
		}
		changes = append(changes, change)
	}
	if err := rows.Err(); err != nil {
		return nil, mapError(err)
	}
	return changes, nil
}

func (repo *BreakingReportRepository) createChange(ctx context.Context, reportID domain.BreakingReportID, change domain.BreakingChange) error {
	_, err := repo.db.executor(ctx).Exec(ctx, `
		INSERT INTO breaking_changes (
			id,
			report_id,
			category,
			file_path,
			package_name,
			symbol,
			rule_id,
			message,
			severity,
			created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`,
		change.ID.String(),
		reportID.String(),
		change.Category,
		change.FilePath,
		change.PackageName,
		change.Symbol,
		change.RuleID,
		change.Message,
		change.Severity,
		change.CreatedAt,
	)
	return mapError(err)
}

func (repo *BreakingReportRepository) getOne(ctx context.Context, query string, args ...any) (domain.BreakingReport, error) {
	report, err := scanBreakingReport(repo.db.executor(ctx).QueryRow(ctx, query, args...).Scan)
	if err != nil {
		return domain.BreakingReport{}, mapError(err)
	}
	return report, nil
}

func scanBreakingReport(scan func(dest ...any) error) (domain.BreakingReport, error) {
	var report domain.BreakingReport
	var id string
	var moduleID string
	var moduleNameValue string
	var baseVersionID string
	var baseVersionValue string
	var statusValue string

	if err := scan(
		&id,
		&moduleID,
		&moduleNameValue,
		&baseVersionID,
		&baseVersionValue,
		&report.TargetRef,
		&statusValue,
		&report.ChangeCount,
		&report.RawOutput,
		&report.HumanSummary,
		&report.CreatedAt,
	); err != nil {
		return domain.BreakingReport{}, err
	}

	moduleName, err := domain.NewModuleName(moduleNameValue)
	if err != nil {
		return domain.BreakingReport{}, err
	}
	baseVersion, err := domain.NewVersion(baseVersionValue)
	if err != nil {
		return domain.BreakingReport{}, err
	}
	status, err := domain.NewBreakingReportStatus(statusValue)
	if err != nil {
		return domain.BreakingReport{}, err
	}

	report.ID = domain.NewBreakingReportID(id)
	report.ModuleID = domain.NewModuleID(moduleID)
	report.ModuleName = moduleName
	report.BaseVersionID = domain.NewModuleVersionID(baseVersionID)
	report.BaseVersion = baseVersion
	report.Status = status
	return report, nil
}

func scanBreakingChange(scan func(dest ...any) error) (domain.BreakingChange, error) {
	var change domain.BreakingChange
	var id string
	var reportID string

	if err := scan(
		&id,
		&reportID,
		&change.Category,
		&change.FilePath,
		&change.PackageName,
		&change.Symbol,
		&change.RuleID,
		&change.Message,
		&change.Severity,
		&change.CreatedAt,
	); err != nil {
		return domain.BreakingChange{}, err
	}

	change.ID = domain.NewBreakingChangeID(id)
	change.ReportID = domain.NewBreakingReportID(reportID)
	return change, nil
}

var _ domain.BreakingReportRepository = (*BreakingReportRepository)(nil)
