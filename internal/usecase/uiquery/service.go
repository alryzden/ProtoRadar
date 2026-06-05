package uiquery

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/alryzden/ProtoRadar/internal/domain"
)

const (
	defaultListLimit     = 1000
	defaultVersionsLimit = 100
	defaultReportsLimit  = 100
)

type Service struct {
	modules        domain.ModuleRepository
	gitLabProjects domain.ModuleGitLabProjectRepository
	versions       domain.ModuleVersionRepository
	artifacts      domain.ModuleVersionArtifactRepository
	bufConfigs     domain.BufConfigRepository
	metadata       domain.DescriptorMetadataRepository
	reports        domain.BreakingReportRepository
	dependencies   domain.ModuleDependencyRepository
	runtime        domain.RuntimeInventoryRepository
}

func NewService(
	modules domain.ModuleRepository,
	gitLabProjects domain.ModuleGitLabProjectRepository,
	versions domain.ModuleVersionRepository,
	artifacts domain.ModuleVersionArtifactRepository,
	bufConfigs domain.BufConfigRepository,
	metadata domain.DescriptorMetadataRepository,
	reports domain.BreakingReportRepository,
	dependencies domain.ModuleDependencyRepository,
	runtime domain.RuntimeInventoryRepository,
) *Service {
	return &Service{
		modules:        modules,
		gitLabProjects: gitLabProjects,
		versions:       versions,
		artifacts:      artifacts,
		bufConfigs:     bufConfigs,
		metadata:       metadata,
		reports:        reports,
		dependencies:   dependencies,
		runtime:        runtime,
	}
}

func (svc *Service) ListModuleOverviews(ctx context.Context, input ListModuleOverviewsInput) ([]ModuleOverview, error) {
	modules, err := svc.modules.List(ctx, defaultListLimit, 0)
	if err != nil {
		return nil, err
	}

	query := normalize(input.Query)
	items := make([]ModuleOverview, 0, len(modules))
	for _, module := range modules {
		if query != "" && !moduleMatches(module, query) {
			continue
		}
		overview, err := svc.moduleOverview(ctx, module, false)
		if err != nil {
			return nil, err
		}
		items = append(items, overview)
	}
	return items, nil
}

func (svc *Service) GetModuleOverview(ctx context.Context, input GetModuleOverviewInput) (ModuleOverview, error) {
	module, err := svc.moduleByName(ctx, input.Module)
	if err != nil {
		return ModuleOverview{}, err
	}
	return svc.moduleOverview(ctx, module, true)
}

func (svc *Service) GetVersionOverview(ctx context.Context, input GetVersionOverviewInput) (VersionOverview, error) {
	module, err := svc.moduleByName(ctx, input.Module)
	if err != nil {
		return VersionOverview{}, err
	}
	versionValue, err := domain.NewVersion(input.Version)
	if err != nil {
		return VersionOverview{}, domain.ErrInvalidVersion
	}
	version, err := svc.versions.GetByModuleAndVersion(ctx, module.ID, versionValue)
	if err != nil {
		return VersionOverview{}, err
	}

	artifacts, err := svc.artifactsForVersion(ctx, version.ID)
	if err != nil {
		return VersionOverview{}, err
	}
	config, lintStatus, err := svc.bufConfigForVersion(ctx, version.ID)
	if err != nil {
		return VersionOverview{}, err
	}
	metadataSummary, err := svc.metadataSummaryForVersion(ctx, version.ID)
	if err != nil {
		return VersionOverview{}, err
	}
	metadata, err := svc.metadataForVersion(ctx, version.ID)
	if err != nil {
		return VersionOverview{}, err
	}
	relatedReports, err := svc.reportsForModule(ctx, module.ID, defaultReportsLimit)
	if err != nil {
		return VersionOverview{}, err
	}
	related := make([]BreakingReportSummary, 0)
	for _, report := range relatedReports {
		if report.BaseVersionID == version.ID || report.BaseVersion.String() == version.Version.String() {
			related = append(related, breakingReportSummary(report))
		}
	}

	versionSummary := versionSummary(version, artifacts, lintStatus, metadataSummary)
	return VersionOverview{
		Module:         moduleInfo(module),
		Version:        versionSummary,
		Artifacts:      artifacts,
		BufConfig:      bufConfigSummary(config, lintStatus),
		Metadata:       metadata,
		MetadataCounts: metadataSummary,
		RelatedReports: related,
	}, nil
}

func (svc *Service) ListBreakingReportOverviews(ctx context.Context, input ListBreakingReportOverviewsInput) ([]BreakingReportSummary, error) {
	reports, err := svc.listReports(ctx, strings.TrimSpace(input.Module), defaultReportsLimit)
	if err != nil {
		return nil, err
	}
	status := normalize(input.Status)
	query := normalize(input.Query)
	items := make([]BreakingReportSummary, 0, len(reports))
	for _, report := range reports {
		if status != "" && normalize(report.Status.String()) != status {
			continue
		}
		summary := breakingReportSummary(report)
		if query != "" && !reportMatches(summary, query) {
			continue
		}
		items = append(items, summary)
	}
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})
	return items, nil
}

func (svc *Service) GetBreakingReportDetails(ctx context.Context, input GetBreakingReportDetailsInput) (BreakingReportDetails, error) {
	reportID := domain.NewBreakingReportID(input.ReportID)
	if reportID == "" {
		return BreakingReportDetails{}, domain.ErrNotFound
	}
	report, changes, err := svc.reports.GetByID(ctx, reportID)
	if err != nil {
		return BreakingReportDetails{}, err
	}
	items := make([]BreakingChangeSummary, 0, len(changes))
	for _, change := range changes {
		items = append(items, breakingChangeSummary(change))
	}
	affected, err := svc.affectedModules(ctx, report.ModuleID)
	if err != nil {
		return BreakingReportDetails{}, err
	}
	runtimeImpact, err := svc.runtimeImpact(ctx, report.ID, report.BaseVersionID)
	if err != nil {
		return BreakingReportDetails{}, err
	}
	return BreakingReportDetails{
		Report:          breakingReportSummary(report),
		Summary:         report.HumanSummary,
		Changes:         items,
		AffectedModules: affected,
		RuntimeImpact:   runtimeImpact,
	}, nil
}

func (svc *Service) GetModuleDependencyGraph(ctx context.Context, input GetModuleDependencyGraphInput) (ModuleDependencyGraph, error) {
	module, err := svc.moduleByName(ctx, input.Module)
	if err != nil {
		return ModuleDependencyGraph{}, err
	}
	if svc.dependencies == nil {
		return ModuleDependencyGraph{Module: moduleInfo(module), Downstream: []DependencyModule{}, Upstream: []DependencyModule{}, Unresolved: []UnresolvedDependency{}}, nil
	}
	upstream, err := svc.dependencies.ListUpstreamByModule(ctx, module.ID)
	if err != nil {
		return ModuleDependencyGraph{}, err
	}
	downstream, err := svc.dependencies.ListDownstreamByModule(ctx, module.ID)
	if err != nil {
		return ModuleDependencyGraph{}, err
	}
	unresolved, err := svc.dependencies.ListUnresolvedByModule(ctx, module.ID)
	if err != nil {
		return ModuleDependencyGraph{}, err
	}
	return ModuleDependencyGraph{
		Module:     moduleInfo(module),
		Downstream: dependencyModules(downstream, false),
		Upstream:   dependencyModules(upstream, true),
		Unresolved: unresolvedDependencies(unresolved),
	}, nil
}

func (svc *Service) ListRuntimeServices(ctx context.Context, input ListRuntimeServicesInput) ([]RuntimeServiceSummary, error) {
	if svc.runtime == nil {
		return []RuntimeServiceSummary{}, nil
	}
	summaries, err := svc.runtime.ListRuntimeServices(ctx, defaultListLimit, 0)
	if err != nil {
		return nil, err
	}
	query := normalize(input.Query)
	environment := normalize(input.Environment)
	driftStatus := normalize(input.DriftStatus)
	items := make([]RuntimeServiceSummary, 0, len(summaries))
	for _, summary := range summaries {
		item := runtimeServiceSummary(summary)
		if query != "" && !strings.Contains(normalize(item.ServiceName), query) {
			continue
		}
		if environment != "" && !runtimeSummaryHasEnvironment(item, environment) {
			continue
		}
		if driftStatus != "" && runtimeDriftCount(item, driftStatus) == 0 {
			continue
		}
		items = append(items, item)
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].LastReportedAt == nil {
			return false
		}
		if items[j].LastReportedAt == nil {
			return true
		}
		return items[i].LastReportedAt.After(*items[j].LastReportedAt)
	})
	return items, nil
}

func (svc *Service) GetRuntimeServiceDetails(ctx context.Context, input GetRuntimeServiceDetailsInput) (RuntimeServiceDetails, error) {
	if svc.runtime == nil {
		return RuntimeServiceDetails{}, domain.ErrNotFound
	}
	serviceName, err := domain.NewRuntimeServiceName(input.Service)
	if err != nil {
		return RuntimeServiceDetails{}, domain.ErrNotFound
	}
	details, err := svc.runtime.GetRuntimeServiceDetails(ctx, serviceName)
	if err != nil {
		return RuntimeServiceDetails{}, err
	}
	return runtimeServiceDetails(details), nil
}

func (svc *Service) GetRuntimeEnvironmentInventory(ctx context.Context, input GetRuntimeEnvironmentInventoryInput) (RuntimeEnvironmentInventory, error) {
	if svc.runtime == nil {
		return RuntimeEnvironmentInventory{Environment: strings.TrimSpace(input.Environment), Deployments: []RuntimeDeployment{}, Usages: []RuntimeModuleUsage{}}, nil
	}
	environment, err := domain.NewRuntimeEnvironment(input.Environment)
	if err != nil {
		return RuntimeEnvironmentInventory{}, domain.ErrInvalidRuntimeEnvironment
	}
	inventory, err := svc.runtime.ListRuntimeEnvironmentInventory(ctx, environment, defaultListLimit, 0)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return RuntimeEnvironmentInventory{}, err
	}
	if err != nil {
		return RuntimeEnvironmentInventory{Environment: environment.String(), Deployments: []RuntimeDeployment{}, Usages: []RuntimeModuleUsage{}}, nil
	}
	return runtimeEnvironmentInventory(inventory), nil
}

func (svc *Service) GetModuleRuntimeUsages(ctx context.Context, input GetModuleRuntimeUsagesInput) (ModuleRuntimeUsages, error) {
	if svc.runtime == nil {
		return ModuleRuntimeUsages{}, domain.ErrNotFound
	}
	module, err := svc.moduleByName(ctx, input.Module)
	if err != nil {
		return ModuleRuntimeUsages{}, err
	}
	usages, err := svc.runtime.ListModuleRuntimeUsages(ctx, module.ID, defaultListLimit, 0)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return ModuleRuntimeUsages{}, err
	}
	if err != nil {
		usages = []domain.ModuleRuntimeUsage{}
	}
	latestVersion := ""
	latest, err := svc.versions.GetLatestByModule(ctx, module.ID)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return ModuleRuntimeUsages{}, err
	}
	if err == nil {
		latestVersion = latest.Version.String()
	}
	return ModuleRuntimeUsages{Module: module.Name.String(), Usages: moduleRuntimeUsages(usages, latestVersion)}, nil
}

func (svc *Service) GetBreakingReportRuntimeImpact(ctx context.Context, input GetBreakingReportRuntimeImpactInput) ([]RuntimeImpact, error) {
	reportID := domain.NewBreakingReportID(input.ReportID)
	if reportID == "" {
		return nil, domain.ErrNotFound
	}
	report, _, err := svc.reports.GetByID(ctx, reportID)
	if err != nil {
		return nil, err
	}
	return svc.runtimeImpact(ctx, report.ID, report.BaseVersionID)
}

func (svc *Service) moduleByName(ctx context.Context, moduleName string) (domain.Module, error) {
	name, err := domain.NewModuleName(moduleName)
	if err != nil {
		return domain.Module{}, domain.ErrInvalidModuleName
	}
	return svc.modules.GetByName(ctx, name)
}

func (svc *Service) moduleOverview(ctx context.Context, module domain.Module, includeDetails bool) (ModuleOverview, error) {
	versions, err := svc.versions.ListByModule(ctx, module.ID, defaultVersionsLimit, 0)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return ModuleOverview{}, err
	}
	if versions == nil {
		versions = []domain.ModuleVersion{}
	}
	reports, err := svc.reportsForModule(ctx, module.ID, defaultReportsLimit)
	if err != nil {
		return ModuleOverview{}, err
	}
	versionItems := make([]VersionSummary, 0, len(versions))
	for _, version := range versions {
		var artifacts []ArtifactSummary
		var lintStatus string
		var metadataSummary DescriptorMetadataSummary
		if includeDetails {
			artifacts, err = svc.artifactsForVersion(ctx, version.ID)
			if err != nil {
				return ModuleOverview{}, err
			}
			_, lintStatus, err = svc.bufConfigForVersion(ctx, version.ID)
			if err != nil {
				return ModuleOverview{}, err
			}
			metadataSummary, err = svc.metadataSummaryForVersion(ctx, version.ID)
			if err != nil {
				return ModuleOverview{}, err
			}
		}
		versionItems = append(versionItems, versionSummary(version, artifacts, lintStatus, metadataSummary))
	}

	var mapping *GitLabProjectInfo
	if svc.gitLabProjects != nil {
		stored, err := svc.gitLabProjects.GetByModuleID(ctx, module.ID)
		if err != nil && !errors.Is(err, domain.ErrNotFound) {
			return ModuleOverview{}, err
		}
		if err == nil {
			info := gitLabProjectInfo(stored)
			mapping = &info
		}
	}

	var latest *VersionSummary
	if len(versionItems) > 0 {
		latestValue := versionItems[0]
		latest = &latestValue
	}
	if latest == nil {
		latestVersion, err := svc.versions.GetLatestByModule(ctx, module.ID)
		if err != nil && !errors.Is(err, domain.ErrNotFound) {
			return ModuleOverview{}, err
		}
		if err == nil {
			latestValue := versionSummary(latestVersion, nil, "", DescriptorMetadataSummary{})
			latest = &latestValue
		}
	}

	reportItems := make([]BreakingReportSummary, 0, len(reports))
	for _, report := range reports {
		reportItems = append(reportItems, breakingReportSummary(report))
	}
	lastStatus := ""
	if len(reportItems) > 0 {
		lastStatus = reportItems[0].Status
	}

	return ModuleOverview{
		Module:              moduleInfo(module),
		GitLabProject:       mapping,
		LatestVersion:       latest,
		VersionCount:        len(versions),
		LastPublishedOrSeen: latestOrModuleTime(module, latest, reportItems),
		BreakingReportCount: len(reports),
		LastBreakingStatus:  lastStatus,
		Versions:            versionItems,
		RecentReports:       reportItems,
	}, nil
}

func (svc *Service) artifactsForVersion(ctx context.Context, versionID domain.ModuleVersionID) ([]ArtifactSummary, error) {
	artifacts, err := svc.artifacts.ListByModuleVersion(ctx, versionID)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}
	items := make([]ArtifactSummary, 0, len(artifacts))
	for _, artifact := range artifacts {
		items = append(items, artifactSummary(artifact))
	}
	return items, nil
}

func (svc *Service) bufConfigForVersion(ctx context.Context, versionID domain.ModuleVersionID) (domain.BufConfigInfo, string, error) {
	config, err := svc.bufConfigs.GetByModuleVersion(ctx, versionID)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return domain.BufConfigInfo{}, "", err
	}
	if err != nil {
		return domain.BufConfigInfo{}, domain.BufLintStatusNotRun.String(), nil
	}
	return config, lintStatusFromConfig(config), nil
}

func (svc *Service) metadataSummaryForVersion(ctx context.Context, versionID domain.ModuleVersionID) (DescriptorMetadataSummary, error) {
	summary, err := svc.metadata.GetSummaryByModuleVersion(ctx, versionID)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return DescriptorMetadataSummary{}, err
	}
	if err != nil {
		return DescriptorMetadataSummary{}, nil
	}
	return descriptorMetadataSummary(summary), nil
}

func (svc *Service) metadataForVersion(ctx context.Context, versionID domain.ModuleVersionID) (DescriptorMetadata, error) {
	metadata, err := svc.metadata.GetByModuleVersion(ctx, versionID)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return DescriptorMetadata{}, err
	}
	if err != nil {
		return DescriptorMetadata{Files: []ProtoFile{}}, nil
	}
	if metadata.Files == nil {
		metadata.Files = []domain.ProtoFile{}
	}
	return descriptorMetadata(metadata), nil
}

func (svc *Service) reportsForModule(ctx context.Context, moduleID domain.ModuleID, limit int) ([]domain.BreakingReport, error) {
	reports, err := svc.reports.ListByModule(ctx, moduleID, limit, 0)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}
	if reports == nil {
		reports = []domain.BreakingReport{}
	}
	return reports, nil
}

func (svc *Service) listReports(ctx context.Context, moduleFilter string, limit int) ([]domain.BreakingReport, error) {
	if moduleFilter != "" {
		module, err := svc.moduleByName(ctx, moduleFilter)
		if err != nil {
			return nil, err
		}
		return svc.reportsForModule(ctx, module.ID, limit)
	}
	modules, err := svc.modules.List(ctx, defaultListLimit, 0)
	if err != nil {
		return nil, err
	}
	all := make([]domain.BreakingReport, 0)
	for _, module := range modules {
		reports, err := svc.reportsForModule(ctx, module.ID, limit)
		if err != nil {
			return nil, err
		}
		all = append(all, reports...)
	}
	return all, nil
}

func (svc *Service) affectedModules(ctx context.Context, moduleID domain.ModuleID) ([]DependencyModule, error) {
	if svc.dependencies == nil {
		return []DependencyModule{}, nil
	}
	affected, err := svc.dependencies.ListAffectedModules(ctx, moduleID)
	if err != nil {
		return nil, err
	}
	items := make([]DependencyModule, 0, len(affected))
	for _, item := range affected {
		sources := make([]string, 0, len(item.DependencySources))
		for _, source := range item.DependencySources {
			sources = append(sources, source.String())
		}
		reasons := make([]string, 0, len(item.Reasons))
		for _, reason := range item.Reasons {
			reasons = append(reasons, reason.String())
		}
		items = append(items, DependencyModule{
			Module:            item.ModuleName.String(),
			Version:           item.LatestVersion.String(),
			DependencySources: sources,
			Reasons:           reasons,
		})
	}
	return items, nil
}

func dependencyModules(dependencies []domain.ModuleDependency, upstream bool) []DependencyModule {
	type group struct {
		module  string
		version string
		sources map[string]struct{}
		reasons map[string]struct{}
	}
	groups := make([]group, 0)
	indexes := map[string]int{}
	for _, dependency := range dependencies {
		moduleName := dependency.ProviderModuleName.String()
		version := dependency.ProviderVersion.String()
		if !upstream {
			moduleName = dependency.ConsumerModuleName.String()
			version = dependency.ConsumerVersion.String()
		}
		key := moduleName + "\x00" + version
		index, exists := indexes[key]
		if !exists {
			index = len(groups)
			indexes[key] = index
			groups = append(groups, group{module: moduleName, version: version, sources: map[string]struct{}{}, reasons: map[string]struct{}{}})
		}
		groups[index].sources[dependency.Source.String()] = struct{}{}
		reason := dependency.Reason.String()
		if reason != "" {
			groups[index].reasons[reason] = struct{}{}
		}
	}

	items := make([]DependencyModule, 0, len(groups))
	for _, group := range groups {
		items = append(items, DependencyModule{
			Module:            group.module,
			Version:           group.version,
			DependencySources: stringSetValues(group.sources),
			Reasons:           stringSetValues(group.reasons),
		})
	}
	return items
}

func unresolvedDependencies(unresolved []domain.UnresolvedProtoDependency) []UnresolvedDependency {
	items := make([]UnresolvedDependency, 0, len(unresolved))
	for _, item := range unresolved {
		items = append(items, UnresolvedDependency{
			Source:           item.Source.String(),
			ImportPath:       item.ImportPath,
			ReferencedSymbol: item.ReferencedSymbol,
			Reason:           item.Reason.String(),
		})
	}
	return items
}

func runtimeServiceSummary(summary domain.RuntimeServiceSummary) RuntimeServiceSummary {
	environments := make([]string, 0, len(summary.Environments))
	for _, environment := range summary.Environments {
		environments = append(environments, environment.String())
	}
	sort.Strings(environments)
	return RuntimeServiceSummary{
		ServiceName:         summary.Service.Name.String(),
		Environments:        environments,
		LastReportedAt:      summary.LatestReportedAt,
		UpToDateCount:       summary.UpToDateCount,
		BehindLatestCount:   summary.BehindLatestCount,
		UnknownVersionCount: summary.UnknownVersionCount,
		DeprecatedCount:     summary.DeprecatedCount,
	}
}

func runtimeServiceDetails(details domain.RuntimeServiceDetails) RuntimeServiceDetails {
	return RuntimeServiceDetails{
		ServiceName: details.Service.Name.String(),
		Deployments: runtimeDeployments(details.Deployments),
		Usages:      runtimeModuleUsages(details.Usages),
	}
}

func runtimeEnvironmentInventory(inventory domain.RuntimeEnvironmentInventory) RuntimeEnvironmentInventory {
	return RuntimeEnvironmentInventory{
		Environment: inventory.Environment.String(),
		Deployments: runtimeDeployments(inventory.Deployments),
		Usages:      runtimeModuleUsages(inventory.Usages),
	}
}

func runtimeDeployments(deployments []domain.RuntimeDeployment) []RuntimeDeployment {
	items := make([]RuntimeDeployment, 0, len(deployments))
	for _, deployment := range deployments {
		items = append(items, RuntimeDeployment{
			ID:           deployment.ID.String(),
			ServiceName:  deployment.ServiceName.String(),
			Environment:  deployment.Environment.String(),
			GitCommit:    deployment.GitCommit,
			BuildVersion: deployment.BuildVersion,
			ReportedAt:   deployment.ReportedAt,
			CreatedAt:    deployment.CreatedAt,
		})
	}
	return items
}

func runtimeModuleUsages(usages []domain.RuntimeModuleUsage) []RuntimeModuleUsage {
	items := make([]RuntimeModuleUsage, 0, len(usages))
	for _, usage := range usages {
		latestVersion := ""
		if usage.LatestVersion != nil {
			latestVersion = usage.LatestVersion.String()
		}
		items = append(items, RuntimeModuleUsage{
			DeploymentID:  usage.DeploymentID.String(),
			Module:        usage.ModuleName.String(),
			Version:       usage.Version.String(),
			LatestVersion: latestVersion,
			DriftStatus:   usage.DriftStatus.String(),
			DriftReason:   usage.DriftReason,
		})
	}
	return items
}

func moduleRuntimeUsages(usages []domain.ModuleRuntimeUsage, latestVersion string) []ModuleRuntimeUsage {
	items := make([]ModuleRuntimeUsage, 0, len(usages))
	for _, usage := range usages {
		items = append(items, ModuleRuntimeUsage{
			ServiceName:   usage.ServiceName.String(),
			Environment:   usage.Environment.String(),
			Module:        usage.ModuleName.String(),
			Version:       usage.Version.String(),
			LatestVersion: latestVersion,
			GitCommit:     usage.GitCommit,
			BuildVersion:  usage.BuildVersion,
			ReportedAt:    usage.ReportedAt,
			DriftStatus:   usage.DriftStatus.String(),
			DriftReason:   usage.DriftReason,
		})
	}
	return items
}

func runtimeImpacts(impacts []domain.RuntimeImpact) []RuntimeImpact {
	items := make([]RuntimeImpact, 0, len(impacts))
	for _, impact := range impacts {
		items = append(items, RuntimeImpact{
			ServiceName:  impact.ServiceName.String(),
			Environment:  impact.Environment.String(),
			UsedModule:   impact.UsedModule.String(),
			UsedVersion:  impact.UsedVersion.String(),
			GitCommit:    impact.GitCommit,
			BuildVersion: impact.BuildVersion,
			ReportedAt:   impact.ReportedAt,
			ImpactStatus: impact.ImpactStatus.String(),
			Reason:       impact.Reason,
		})
	}
	return items
}

func (svc *Service) runtimeImpact(ctx context.Context, reportID domain.BreakingReportID, baseVersionID domain.ModuleVersionID) ([]RuntimeImpact, error) {
	if svc.runtime == nil {
		return []RuntimeImpact{}, nil
	}
	impacts, err := svc.runtime.ListRuntimeImpactByModuleVersion(ctx, reportID, baseVersionID, defaultListLimit, 0)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}
	if err != nil {
		return []RuntimeImpact{}, nil
	}
	return runtimeImpacts(impacts), nil
}

func runtimeSummaryHasEnvironment(summary RuntimeServiceSummary, environment string) bool {
	for _, item := range summary.Environments {
		if normalize(item) == environment {
			return true
		}
	}
	return false
}

func runtimeDriftCount(summary RuntimeServiceSummary, driftStatus string) int {
	switch driftStatus {
	case domain.RuntimeDriftStatusUpToDate.String():
		return summary.UpToDateCount
	case domain.RuntimeDriftStatusBehindLatest.String():
		return summary.BehindLatestCount
	case domain.RuntimeDriftStatusUnknownVersion.String():
		return summary.UnknownVersionCount
	case domain.RuntimeDriftStatusDeprecatedVersion.String():
		return summary.DeprecatedCount
	default:
		return 0
	}
}

func stringSetValues(values map[string]struct{}) []string {
	items := make([]string, 0, len(values))
	for value := range values {
		items = append(items, value)
	}
	sort.Strings(items)
	return items
}

func moduleInfo(module domain.Module) ModuleInfo {
	return ModuleInfo{
		Name:          module.Name.String(),
		Description:   module.Description,
		RepositoryURL: module.RepositoryURL,
		CreatedAt:     module.CreatedAt,
		UpdatedAt:     module.UpdatedAt,
	}
}

func gitLabProjectInfo(mapping domain.ModuleGitLabProject) GitLabProjectInfo {
	return GitLabProjectInfo{
		BaseURL:     mapping.GitLabBaseURL,
		ProjectID:   mapping.GitLabProjectID,
		ProjectPath: mapping.GitLabProjectPath,
		UpdatedAt:   mapping.UpdatedAt,
	}
}

func versionSummary(version domain.ModuleVersion, artifacts []ArtifactSummary, lintStatus string, metadataSummary DescriptorMetadataSummary) VersionSummary {
	if artifacts == nil {
		artifacts = []ArtifactSummary{}
	}
	return VersionSummary{
		Version:         version.Version.String(),
		Status:          version.Status.String(),
		Digest:          version.Digest,
		CreatedAt:       version.CreatedAt,
		PublishedAt:     version.PublishedAt,
		Artifacts:       artifacts,
		LintStatus:      lintStatus,
		MetadataSummary: metadataSummary,
	}
}

func artifactSummary(artifact domain.Artifact) ArtifactSummary {
	return ArtifactSummary{
		Kind:           artifact.Kind.String(),
		ChecksumSHA256: artifact.ChecksumSHA256,
		SizeBytes:      artifact.SizeBytes,
	}
}

func bufConfigSummary(config domain.BufConfigInfo, lintStatus string) BufConfigSummary {
	return BufConfigSummary{
		ConfigPresent:         config.BufYAMLPresent,
		LockPresent:           config.BufLockPresent,
		BufYAMLDigest:         config.BufYAMLDigest,
		BufLockDigest:         config.BufLockDigest,
		ModulePaths:           append([]string(nil), config.ModulePaths...),
		Deps:                  append([]string(nil), config.Deps...),
		LintEnabled:           config.LintEnabled,
		LintStatus:            lintStatus,
		BreakingConfigPresent: config.BreakingConfigPresent,
	}
}

func descriptorMetadataSummary(summary domain.DescriptorMetadataSummary) DescriptorMetadataSummary {
	return DescriptorMetadataSummary{
		Files:      summary.FileCount,
		Packages:   summary.PackageCount,
		Imports:    summary.ImportCount,
		Services:   summary.ServiceCount,
		Methods:    summary.MethodCount,
		Messages:   summary.MessageCount,
		Fields:     summary.FieldCount,
		Enums:      summary.EnumCount,
		EnumValues: summary.EnumValueCount,
	}
}

func descriptorMetadata(metadata domain.DescriptorMetadata) DescriptorMetadata {
	files := make([]ProtoFile, 0, len(metadata.Files))
	for _, file := range metadata.Files {
		files = append(files, protoFile(file))
	}
	return DescriptorMetadata{Files: files}
}

func protoFile(file domain.ProtoFile) ProtoFile {
	imports := make([]ProtoImport, 0, len(file.Imports))
	for _, item := range file.Imports {
		imports = append(imports, ProtoImport{
			Path:   item.Path,
			Public: item.Public,
			Weak:   item.Weak,
		})
	}
	services := make([]ProtoService, 0, len(file.Services))
	for _, item := range file.Services {
		services = append(services, protoService(item))
	}
	messages := make([]ProtoMessage, 0, len(file.Messages))
	for _, item := range file.Messages {
		messages = append(messages, protoMessage(item))
	}
	enums := make([]ProtoEnum, 0, len(file.Enums))
	for _, item := range file.Enums {
		enums = append(enums, protoEnum(item))
	}
	return ProtoFile{
		Path:        file.Path,
		PackageName: file.PackageName,
		Syntax:      file.Syntax,
		Imports:     imports,
		Services:    services,
		Messages:    messages,
		Enums:       enums,
	}
}

func protoService(service domain.ProtoService) ProtoService {
	methods := make([]ProtoMethod, 0, len(service.Methods))
	for _, method := range service.Methods {
		methods = append(methods, ProtoMethod{
			Name:            method.Name,
			InputType:       method.InputType,
			OutputType:      method.OutputType,
			ClientStreaming: method.ClientStreaming,
			ServerStreaming: method.ServerStreaming,
		})
	}
	return ProtoService{
		Name:     service.Name,
		FullName: service.FullName,
		Methods:  methods,
	}
}

func protoMessage(message domain.ProtoMessage) ProtoMessage {
	fields := make([]ProtoField, 0, len(message.Fields))
	for _, field := range message.Fields {
		fields = append(fields, ProtoField{
			Name:       field.Name,
			Number:     field.Number,
			Type:       field.Type,
			TypeName:   field.TypeName,
			Label:      field.Label,
			JSONName:   field.JSONName,
			OneofName:  field.OneofName,
			IsRepeated: field.IsRepeated,
			IsMap:      field.IsMap,
		})
	}
	messages := make([]ProtoMessage, 0, len(message.Messages))
	for _, nested := range message.Messages {
		messages = append(messages, protoMessage(nested))
	}
	enums := make([]ProtoEnum, 0, len(message.Enums))
	for _, enum := range message.Enums {
		enums = append(enums, protoEnum(enum))
	}
	return ProtoMessage{
		Name:     message.Name,
		FullName: message.FullName,
		Fields:   fields,
		Messages: messages,
		Enums:    enums,
	}
}

func protoEnum(enum domain.ProtoEnum) ProtoEnum {
	values := make([]ProtoEnumValue, 0, len(enum.Values))
	for _, value := range enum.Values {
		values = append(values, ProtoEnumValue{
			Name:   value.Name,
			Number: value.Number,
		})
	}
	return ProtoEnum{
		Name:     enum.Name,
		FullName: enum.FullName,
		Values:   values,
	}
}

func breakingReportSummary(report domain.BreakingReport) BreakingReportSummary {
	return BreakingReportSummary{
		ID:          report.ID.String(),
		Module:      report.ModuleName.String(),
		BaseVersion: report.BaseVersion.String(),
		TargetRef:   report.TargetRef,
		Status:      report.Status.String(),
		ChangeCount: report.ChangeCount,
		CreatedAt:   report.CreatedAt,
	}
}

func breakingChangeSummary(change domain.BreakingChange) BreakingChangeSummary {
	return BreakingChangeSummary{
		Category:    change.Category,
		FilePath:    change.FilePath,
		PackageName: change.PackageName,
		Symbol:      change.Symbol,
		RuleID:      change.RuleID,
		Message:     change.Message,
		Severity:    change.Severity,
		CreatedAt:   change.CreatedAt,
	}
}

func lintStatusFromConfig(config domain.BufConfigInfo) string {
	if !config.LintEnabled {
		return domain.BufLintStatusNotRun.String()
	}
	return domain.BufLintStatusPassed.String()
}

func latestOrModuleTime(module domain.Module, latest *VersionSummary, reports []BreakingReportSummary) time.Time {
	last := module.UpdatedAt
	if latest != nil && latest.CreatedAt.After(last) {
		last = latest.CreatedAt
	}
	if len(reports) > 0 && reports[0].CreatedAt.After(last) {
		last = reports[0].CreatedAt
	}
	return last
}

func moduleMatches(module domain.Module, query string) bool {
	return strings.Contains(normalize(module.Name.String()), query) ||
		strings.Contains(normalize(module.Description), query) ||
		strings.Contains(normalize(module.RepositoryURL), query)
}

func reportMatches(report BreakingReportSummary, query string) bool {
	return strings.Contains(normalize(report.ID), query) ||
		strings.Contains(normalize(report.Module), query) ||
		strings.Contains(normalize(report.BaseVersion), query) ||
		strings.Contains(normalize(report.TargetRef), query) ||
		strings.Contains(normalize(report.Status), query)
}

func normalize(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
