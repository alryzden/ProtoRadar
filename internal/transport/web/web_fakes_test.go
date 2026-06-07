package web

import (
	"context"
	"strings"

	"github.com/alryzden/ProtoRadar/internal/domain"
	"github.com/alryzden/ProtoRadar/internal/usecase/uiquery"
)

type fakeQuery struct {
	edition              uiquery.EditionDetails
	modules              []uiquery.ModuleOverview
	version              uiquery.VersionOverview
	reports              []uiquery.BreakingReportSummary
	reportDetails        map[string]uiquery.BreakingReportDetails
	approvalDetails      map[string]uiquery.ApprovalRequestDetails
	dependencyGraphs     map[string]uiquery.ModuleDependencyGraph
	runtimeServices      []uiquery.RuntimeServiceSummary
	runtimeDetails       map[string]uiquery.RuntimeServiceDetails
	runtimeEnvironments  map[string]uiquery.RuntimeEnvironmentInventory
	moduleRuntimeUsages  map[string]uiquery.ModuleRuntimeUsages
	lastListInput        uiquery.ListModuleOverviewsInput
	lastReportListInput  uiquery.ListBreakingReportOverviewsInput
	lastRuntimeListInput uiquery.ListRuntimeServicesInput
}

func newFakeQuery() *fakeQuery {
	return &fakeQuery{
		edition: uiquery.EditionDetails{
			Edition:   "community",
			Version:   "v1.2.0",
			Commit:    "abc123",
			BuildDate: "2026-06-05T12:00:00Z",
			Capabilities: []uiquery.CapabilityStatus{
				{Name: "registry", Enabled: true},
				{Name: "breaking_checks", Enabled: true},
				{Name: "oidc_auth", Enabled: false},
				{Name: "advanced_rbac", Enabled: false},
			},
		},
		modules:       []uiquery.ModuleOverview{userModule(), billingModule()},
		version:       userVersionOverview(),
		reports:       []uiquery.BreakingReportSummary{userPassedReport(), userBreakingReport(), billingFailedReport()},
		reportDetails: map[string]uiquery.BreakingReportDetails{"report-2": userBreakingReportDetails()},
		approvalDetails: map[string]uiquery.ApprovalRequestDetails{
			"approval-1": approvalRequestDetails(),
		},
		dependencyGraphs: map[string]uiquery.ModuleDependencyGraph{
			"user-api": userDependencyGraph(),
		},
		runtimeServices: []uiquery.RuntimeServiceSummary{
			billingRuntimeSummary(),
		},
		runtimeDetails: map[string]uiquery.RuntimeServiceDetails{
			"billing-service": billingRuntimeDetails(),
		},
		runtimeEnvironments: map[string]uiquery.RuntimeEnvironmentInventory{
			"production": productionRuntimeInventory(),
		},
		moduleRuntimeUsages: map[string]uiquery.ModuleRuntimeUsages{
			"user-api": userAPIRuntimeUsages(),
		},
	}
}

func (query *fakeQuery) GetEdition(ctx context.Context, input uiquery.GetEditionInput) (uiquery.EditionDetails, error) {
	return query.edition, nil
}

func (query *fakeQuery) ListModuleOverviews(ctx context.Context, input uiquery.ListModuleOverviewsInput) ([]uiquery.ModuleOverview, error) {
	query.lastListInput = input
	if strings.TrimSpace(input.Query) == "" {
		return query.modules, nil
	}
	needle := strings.ToLower(strings.TrimSpace(input.Query))
	items := make([]uiquery.ModuleOverview, 0)
	for _, module := range query.modules {
		if strings.Contains(strings.ToLower(module.Module.Name), needle) || strings.Contains(strings.ToLower(module.Module.Description), needle) || strings.Contains(strings.ToLower(module.Module.RepositoryURL), needle) {
			items = append(items, module)
		}
	}
	return items, nil
}

func (query *fakeQuery) GetModuleOverview(ctx context.Context, input uiquery.GetModuleOverviewInput) (uiquery.ModuleOverview, error) {
	for _, module := range query.modules {
		if module.Module.Name == input.Module {
			return module, nil
		}
	}
	return uiquery.ModuleOverview{}, domain.ErrNotFound
}

func (query *fakeQuery) GetVersionOverview(ctx context.Context, input uiquery.GetVersionOverviewInput) (uiquery.VersionOverview, error) {
	if input.Module != "user-api" || input.Version != "v2.0.0" {
		return uiquery.VersionOverview{}, domain.ErrNotFound
	}
	return query.version, nil
}

func (query *fakeQuery) GetModuleDependencyGraph(ctx context.Context, input uiquery.GetModuleDependencyGraphInput) (uiquery.ModuleDependencyGraph, error) {
	graph, ok := query.dependencyGraphs[input.Module]
	if !ok {
		return uiquery.ModuleDependencyGraph{}, domain.ErrNotFound
	}
	return graph, nil
}

func (query *fakeQuery) ListBreakingReportOverviews(ctx context.Context, input uiquery.ListBreakingReportOverviewsInput) ([]uiquery.BreakingReportSummary, error) {
	query.lastReportListInput = input
	items := make([]uiquery.BreakingReportSummary, 0, len(query.reports))
	for _, report := range query.reports {
		if input.Module != "" && report.Module != input.Module {
			continue
		}
		if input.Status != "" && report.Status != input.Status {
			continue
		}
		if input.Query != "" {
			needle := strings.ToLower(strings.TrimSpace(input.Query))
			if !strings.Contains(strings.ToLower(report.ID), needle) &&
				!strings.Contains(strings.ToLower(report.Module), needle) &&
				!strings.Contains(strings.ToLower(report.TargetRef), needle) &&
				!strings.Contains(strings.ToLower(report.BaseVersion), needle) {
				continue
			}
		}
		items = append(items, report)
	}
	return items, nil
}

func (query *fakeQuery) GetBreakingReportDetails(ctx context.Context, input uiquery.GetBreakingReportDetailsInput) (uiquery.BreakingReportDetails, error) {
	details, ok := query.reportDetails[input.ReportID]
	if !ok {
		return uiquery.BreakingReportDetails{}, domain.ErrNotFound
	}
	return details, nil
}

func (query *fakeQuery) GetApprovalRequestDetails(ctx context.Context, input uiquery.GetApprovalRequestDetailsInput) (uiquery.ApprovalRequestDetails, error) {
	details, ok := query.approvalDetails[input.RequestID]
	if !ok {
		return uiquery.ApprovalRequestDetails{}, domain.ErrNotFound
	}
	return details, nil
}

func (query *fakeQuery) ListRuntimeServices(ctx context.Context, input uiquery.ListRuntimeServicesInput) ([]uiquery.RuntimeServiceSummary, error) {
	query.lastRuntimeListInput = input
	items := make([]uiquery.RuntimeServiceSummary, 0, len(query.runtimeServices))
	for _, summary := range query.runtimeServices {
		if input.Query != "" && !strings.Contains(strings.ToLower(summary.ServiceName), strings.ToLower(input.Query)) {
			continue
		}
		if input.Environment != "" {
			found := false
			for _, environment := range summary.Environments {
				if environment == input.Environment {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		items = append(items, summary)
	}
	return items, nil
}

func (query *fakeQuery) GetRuntimeServiceDetails(ctx context.Context, input uiquery.GetRuntimeServiceDetailsInput) (uiquery.RuntimeServiceDetails, error) {
	details, ok := query.runtimeDetails[input.Service]
	if !ok {
		return uiquery.RuntimeServiceDetails{}, domain.ErrNotFound
	}
	return details, nil
}

func (query *fakeQuery) GetRuntimeEnvironmentInventory(ctx context.Context, input uiquery.GetRuntimeEnvironmentInventoryInput) (uiquery.RuntimeEnvironmentInventory, error) {
	inventory, ok := query.runtimeEnvironments[input.Environment]
	if !ok {
		return uiquery.RuntimeEnvironmentInventory{Environment: input.Environment}, nil
	}
	return inventory, nil
}

func (query *fakeQuery) GetModuleRuntimeUsages(ctx context.Context, input uiquery.GetModuleRuntimeUsagesInput) (uiquery.ModuleRuntimeUsages, error) {
	usages, ok := query.moduleRuntimeUsages[input.Module]
	if !ok {
		return uiquery.ModuleRuntimeUsages{}, domain.ErrNotFound
	}
	return usages, nil
}

var _ Query = (*fakeQuery)(nil)
