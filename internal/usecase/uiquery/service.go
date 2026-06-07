package uiquery

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/alryzden/ProtoRadar/internal/domain"
	"github.com/alryzden/ProtoRadar/internal/edition"
	"github.com/alryzden/ProtoRadar/internal/version"
)

const (
	defaultListLimit     = 1000
	defaultVersionsLimit = 100
	defaultReportsLimit  = 100
)

type Service struct {
	registry   registryQueries
	graph      graphQueries
	runtime    runtimeQueries
	governance governanceQueries
	edition    editionQueries
}

type registryQueries struct {
	modules        domain.ModuleRepository
	gitLabProjects domain.ModuleGitLabProjectRepository
	versions       domain.ModuleVersionRepository
	artifacts      domain.ModuleVersionArtifactRepository
	bufConfigs     domain.BufConfigRepository
	metadata       domain.DescriptorMetadataRepository
	reports        domain.BreakingReportRepository
}

type graphQueries struct {
	dependencies domain.ModuleDependencyRepository
}

type runtimeQueries struct {
	inventory domain.RuntimeInventoryRepository
}

type governanceQueries struct {
	owners    domain.ModuleOwnerRepository
	approvals domain.ApprovalRepository
	audit     domain.GovernanceAuditRepository
}

type editionQueries struct {
	capabilities edition.CapabilityChecker
	buildInfo    version.BuildInfo
}

type GovernanceRepositories struct {
	Owners    domain.ModuleOwnerRepository
	Approvals domain.ApprovalRepository
	Audit     domain.GovernanceAuditRepository
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
	governance GovernanceRepositories,
	capabilities edition.CapabilityChecker,
	buildInfo version.BuildInfo,
) *Service {
	if capabilities == nil {
		capabilities = edition.NewCommunityCapabilityChecker()
	}
	if buildInfo.Version == "" {
		buildInfo = version.Info()
	}
	return &Service{
		registry: registryQueries{
			modules:        modules,
			gitLabProjects: gitLabProjects,
			versions:       versions,
			artifacts:      artifacts,
			bufConfigs:     bufConfigs,
			metadata:       metadata,
			reports:        reports,
		},
		graph:   graphQueries{dependencies: dependencies},
		runtime: runtimeQueries{inventory: runtime},
		governance: governanceQueries{
			owners:    governance.Owners,
			approvals: governance.Approvals,
			audit:     governance.Audit,
		},
		edition: editionQueries{
			capabilities: capabilities,
			buildInfo:    buildInfo,
		},
	}
}

func (svc *Service) GetEdition(ctx context.Context, input GetEditionInput) (EditionDetails, error) {
	model := edition.NewCommunityEdition(ctx, svc.edition.buildInfo, svc.edition.capabilities)
	capabilities := make([]CapabilityStatus, 0, len(model.Capabilities))
	for _, status := range model.Capabilities {
		capabilities = append(capabilities, CapabilityStatus{
			Name:    status.Capability.String(),
			Enabled: status.Enabled,
		})
	}
	return EditionDetails{
		Edition:      model.Name,
		Version:      model.Version.Version,
		Commit:       model.Version.Commit,
		BuildDate:    model.Version.BuildDate,
		Capabilities: capabilities,
	}, nil
}

func (svc *Service) ListModuleOverviews(ctx context.Context, input ListModuleOverviewsInput) ([]ModuleOverview, error) {
	modules, err := svc.registry.modules.List(ctx, defaultListLimit, 0)
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
	moduleVersion, err := svc.registry.versions.GetByModuleAndVersion(ctx, module.ID, versionValue)
	if err != nil {
		return VersionOverview{}, err
	}

	artifacts, err := svc.artifactsForVersion(ctx, moduleVersion.ID)
	if err != nil {
		return VersionOverview{}, err
	}
	config, lintStatus, err := svc.bufConfigForVersion(ctx, moduleVersion.ID)
	if err != nil {
		return VersionOverview{}, err
	}
	metadataSummary, err := svc.metadataSummaryForVersion(ctx, moduleVersion.ID)
	if err != nil {
		return VersionOverview{}, err
	}
	metadata, err := svc.metadataForVersion(ctx, moduleVersion.ID)
	if err != nil {
		return VersionOverview{}, err
	}
	relatedReports, err := svc.reportsForModule(ctx, module.ID, defaultReportsLimit)
	if err != nil {
		return VersionOverview{}, err
	}
	related := make([]BreakingReportSummary, 0)
	for _, report := range relatedReports {
		if report.BaseVersionID == moduleVersion.ID || report.BaseVersion.String() == moduleVersion.Version.String() {
			related = append(related, breakingReportSummary(report))
		}
	}

	summary := versionSummary(moduleVersion, artifacts, lintStatus, metadataSummary)
	return VersionOverview{
		Module:         moduleInfo(module),
		Version:        summary,
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
	report, changes, err := svc.registry.reports.GetByID(ctx, reportID)
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
	var approval *ApprovalRequestSummary
	if svc.governanceApprovalsAvailable() {
		request, err := svc.governance.approvals.GetRequestByBreakingReportID(ctx, report.ID)
		if err != nil && !errors.Is(err, domain.ErrNotFound) {
			return BreakingReportDetails{}, err
		}
		if err == nil {
			summary := approvalRequestSummary(request)
			approval = &summary
		}
	}
	return BreakingReportDetails{
		Report:          breakingReportSummary(report),
		Summary:         report.HumanSummary,
		Changes:         items,
		AffectedModules: affected,
		RuntimeImpact:   runtimeImpact,
		Approval:        approval,
	}, nil
}

func (svc *Service) GetApprovalRequestDetails(ctx context.Context, input GetApprovalRequestDetailsInput) (ApprovalRequestDetails, error) {
	if !svc.governanceApprovalsAvailable() {
		return ApprovalRequestDetails{}, domain.ErrNotFound
	}
	requestID := domain.NewApprovalRequestID(input.RequestID)
	if requestID == "" {
		return ApprovalRequestDetails{}, domain.ErrNotFound
	}
	request, err := svc.governance.approvals.GetRequestByID(ctx, requestID)
	if err != nil {
		return ApprovalRequestDetails{}, err
	}
	events := []GovernanceAuditEventSummary{}
	if svc.governanceAuditAvailable() {
		stored, err := svc.governance.audit.ListByApprovalRequest(ctx, requestID, defaultReportsLimit, 0)
		if err != nil && !errors.Is(err, domain.ErrNotFound) {
			return ApprovalRequestDetails{}, err
		}
		if err == nil {
			events = governanceAuditEvents(stored)
		}
	}
	return ApprovalRequestDetails{Request: approvalRequestSummary(request), AuditEvents: events}, nil
}

func (svc *Service) GetModuleDependencyGraph(ctx context.Context, input GetModuleDependencyGraphInput) (ModuleDependencyGraph, error) {
	module, err := svc.moduleByName(ctx, input.Module)
	if err != nil {
		return ModuleDependencyGraph{}, err
	}
	if !svc.dependencyGraphAvailable() {
		return emptyDependencyGraph(module), nil
	}
	upstream, err := svc.graph.dependencies.ListUpstreamByModule(ctx, module.ID)
	if err != nil {
		return ModuleDependencyGraph{}, err
	}
	downstream, err := svc.graph.dependencies.ListDownstreamByModule(ctx, module.ID)
	if err != nil {
		return ModuleDependencyGraph{}, err
	}
	unresolved, err := svc.graph.dependencies.ListUnresolvedByModule(ctx, module.ID)
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
	if !svc.runtimeQueriesAvailable() {
		return []RuntimeServiceSummary{}, nil
	}
	summaries, err := svc.runtime.inventory.ListRuntimeServices(ctx, defaultListLimit, 0)
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
	if !svc.runtimeQueriesAvailable() {
		return RuntimeServiceDetails{}, domain.ErrNotFound
	}
	serviceName, err := domain.NewRuntimeServiceName(input.Service)
	if err != nil {
		return RuntimeServiceDetails{}, domain.ErrNotFound
	}
	details, err := svc.runtime.inventory.GetRuntimeServiceDetails(ctx, serviceName)
	if err != nil {
		return RuntimeServiceDetails{}, err
	}
	return runtimeServiceDetails(details), nil
}

func (svc *Service) GetRuntimeEnvironmentInventory(ctx context.Context, input GetRuntimeEnvironmentInventoryInput) (RuntimeEnvironmentInventory, error) {
	if !svc.runtimeQueriesAvailable() {
		return emptyRuntimeEnvironmentInventory(strings.TrimSpace(input.Environment)), nil
	}
	environment, err := domain.NewRuntimeEnvironment(input.Environment)
	if err != nil {
		return RuntimeEnvironmentInventory{}, domain.ErrInvalidRuntimeEnvironment
	}
	inventory, err := svc.runtime.inventory.ListRuntimeEnvironmentInventory(ctx, environment, defaultListLimit, 0)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return RuntimeEnvironmentInventory{}, err
	}
	if err != nil {
		return emptyRuntimeEnvironmentInventoryResult(environment.String())
	}
	return runtimeEnvironmentInventory(inventory), nil
}

func (svc *Service) GetModuleRuntimeUsages(ctx context.Context, input GetModuleRuntimeUsagesInput) (ModuleRuntimeUsages, error) {
	if !svc.runtimeQueriesAvailable() {
		return ModuleRuntimeUsages{}, domain.ErrNotFound
	}
	module, err := svc.moduleByName(ctx, input.Module)
	if err != nil {
		return ModuleRuntimeUsages{}, err
	}
	usages, err := svc.runtime.inventory.ListModuleRuntimeUsages(ctx, module.ID, defaultListLimit, 0)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return ModuleRuntimeUsages{}, err
	}
	if err != nil {
		usages = []domain.ModuleRuntimeUsage{}
	}
	latestVersion := ""
	latest, err := svc.registry.versions.GetLatestByModule(ctx, module.ID)
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
	report, _, err := svc.registry.reports.GetByID(ctx, reportID)
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
	return svc.registry.modules.GetByName(ctx, name)
}

func (svc *Service) moduleOverview(ctx context.Context, module domain.Module, includeDetails bool) (ModuleOverview, error) {
	versions, err := svc.registry.versions.ListByModule(ctx, module.ID, defaultVersionsLimit, 0)
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
	owners, err := svc.moduleOwners(ctx, module.ID)
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
	if svc.registry.gitLabProjects != nil {
		stored, err := svc.registry.gitLabProjects.GetByModuleID(ctx, module.ID)
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
		latestVersion, err := svc.registry.versions.GetLatestByModule(ctx, module.ID)
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
		Owners:              owners,
		Versions:            versionItems,
		RecentReports:       reportItems,
	}, nil
}

func (svc *Service) moduleOwners(ctx context.Context, moduleID domain.ModuleID) ([]ModuleOwner, error) {
	if !svc.governanceOwnersAvailable() {
		return []ModuleOwner{}, nil
	}
	owners, err := svc.governance.owners.ListByModule(ctx, moduleID)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}
	if err != nil {
		return emptyModuleOwnersResult()
	}
	items := make([]ModuleOwner, 0, len(owners))
	for _, owner := range owners {
		items = append(items, moduleOwner(owner))
	}
	return items, nil
}

func (svc *Service) artifactsForVersion(ctx context.Context, versionID domain.ModuleVersionID) ([]ArtifactSummary, error) {
	artifacts, err := svc.registry.artifacts.ListByModuleVersion(ctx, versionID)
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
	config, err := svc.registry.bufConfigs.GetByModuleVersion(ctx, versionID)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return domain.BufConfigInfo{}, "", err
	}
	if err != nil {
		return missingBufConfigResult()
	}
	return config, lintStatusFromConfig(config), nil
}

func (svc *Service) metadataSummaryForVersion(ctx context.Context, versionID domain.ModuleVersionID) (DescriptorMetadataSummary, error) {
	summary, err := svc.registry.metadata.GetSummaryByModuleVersion(ctx, versionID)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return DescriptorMetadataSummary{}, err
	}
	if err != nil {
		return emptyMetadataSummaryResult()
	}
	return descriptorMetadataSummary(summary), nil
}

func (svc *Service) metadataForVersion(ctx context.Context, versionID domain.ModuleVersionID) (DescriptorMetadata, error) {
	metadata, err := svc.registry.metadata.GetByModuleVersion(ctx, versionID)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return DescriptorMetadata{}, err
	}
	if err != nil {
		return emptyDescriptorMetadataResult()
	}
	if metadata.Files == nil {
		metadata.Files = []domain.ProtoFile{}
	}
	return descriptorMetadata(metadata), nil
}

func (svc *Service) reportsForModule(ctx context.Context, moduleID domain.ModuleID, limit int) ([]domain.BreakingReport, error) {
	reports, err := svc.registry.reports.ListByModule(ctx, moduleID, limit, 0)
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
	modules, err := svc.registry.modules.List(ctx, defaultListLimit, 0)
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
	if !svc.dependencyGraphAvailable() {
		return []DependencyModule{}, nil
	}
	affected, err := svc.graph.dependencies.ListAffectedModules(ctx, moduleID)
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
		moduleVersion := dependency.ProviderVersion.String()
		if !upstream {
			moduleName = dependency.ConsumerModuleName.String()
			moduleVersion = dependency.ConsumerVersion.String()
		}
		key := moduleName + "\x00" + moduleVersion
		index, exists := indexes[key]
		if !exists {
			index = len(groups)
			indexes[key] = index
			groups = append(groups, group{module: moduleName, version: moduleVersion, sources: map[string]struct{}{}, reasons: map[string]struct{}{}})
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
			DriftStatus:  impact.DriftStatus.String(),
			DriftReason:  impact.DriftReason,
		})
	}
	return items
}

func moduleOwner(owner domain.ModuleOwner) ModuleOwner {
	return ModuleOwner{
		ID:          owner.ID.String(),
		ModuleID:    owner.ModuleID.String(),
		ModuleName:  owner.ModuleName.String(),
		SubjectType: owner.SubjectType.String(),
		Subject:     owner.Subject,
		Role:        owner.Role.String(),
		CreatedAt:   owner.CreatedAt,
		UpdatedAt:   owner.UpdatedAt,
	}
}

func approvalRequestSummary(request domain.ApprovalRequest) ApprovalRequestSummary {
	requirements := make([]ApprovalRequirementSummary, 0, len(request.Requirements))
	for _, requirement := range request.Requirements {
		requirements = append(requirements, approvalRequirementSummary(requirement))
	}
	decisions := make([]ApprovalDecisionSummary, 0, len(request.Decisions))
	for _, decision := range request.Decisions {
		decisions = append(decisions, approvalDecisionSummary(decision))
	}
	breakingReportID := ""
	if request.BreakingReportID != nil {
		breakingReportID = request.BreakingReportID.String()
	}
	return ApprovalRequestSummary{
		ID:                request.ID.String(),
		ModuleID:          request.ModuleID.String(),
		ModuleName:        request.ModuleName.String(),
		BreakingReportID:  breakingReportID,
		TargetRef:         request.TargetRef,
		Status:            request.Status.String(),
		RequiredApprovals: request.RequiredApprovals,
		ReceivedApprovals: request.ReceivedApprovals,
		Requirements:      requirements,
		Decisions:         decisions,
		CreatedAt:         request.CreatedAt,
		UpdatedAt:         request.UpdatedAt,
	}
}

func approvalRequirementSummary(requirement domain.ApprovalRequirement) ApprovalRequirementSummary {
	targetModuleID := ""
	if requirement.TargetModuleID != nil {
		targetModuleID = requirement.TargetModuleID.String()
	}
	return ApprovalRequirementSummary{
		ID:                requirement.ID.String(),
		ApprovalRequestID: requirement.ApprovalRequestID.String(),
		RequirementType:   requirement.RequirementType.String(),
		TargetModuleID:    targetModuleID,
		TargetModuleName:  requirement.TargetModuleName.String(),
		RequiredRole:      requirement.RequiredRole.String(),
		Status:            requirement.Status.String(),
		Reason:            requirement.Reason,
		CreatedAt:         requirement.CreatedAt,
		UpdatedAt:         requirement.UpdatedAt,
	}
}

func approvalDecisionSummary(decision domain.ApprovalDecision) ApprovalDecisionSummary {
	return ApprovalDecisionSummary{
		ID:                decision.ID.String(),
		ApprovalRequestID: decision.ApprovalRequestID.String(),
		RequirementID:     decision.RequirementID.String(),
		Decision:          decision.Decision.String(),
		DecidedBy:         decision.DecidedBy,
		Comment:           decision.Comment,
		CreatedAt:         decision.CreatedAt,
	}
}

func governanceAuditEvents(events []domain.GovernanceAuditEvent) []GovernanceAuditEventSummary {
	items := make([]GovernanceAuditEventSummary, 0, len(events))
	for _, event := range events {
		moduleID := ""
		if event.ModuleID != nil {
			moduleID = event.ModuleID.String()
		}
		approvalRequestID := ""
		if event.ApprovalRequestID != nil {
			approvalRequestID = event.ApprovalRequestID.String()
		}
		breakingReportID := ""
		if event.BreakingReportID != nil {
			breakingReportID = event.BreakingReportID.String()
		}
		items = append(items, GovernanceAuditEventSummary{
			ID:                event.ID.String(),
			EventType:         event.EventType.String(),
			Actor:             event.Actor,
			ModuleID:          moduleID,
			ModuleName:        event.ModuleName.String(),
			ApprovalRequestID: approvalRequestID,
			BreakingReportID:  breakingReportID,
			PayloadJSON:       string(event.PayloadJSON),
			CreatedAt:         event.CreatedAt,
		})
	}
	return items
}

func (svc *Service) runtimeImpact(ctx context.Context, reportID domain.BreakingReportID, baseVersionID domain.ModuleVersionID) ([]RuntimeImpact, error) {
	if !svc.runtimeQueriesAvailable() {
		return emptyRuntimeImpact(), nil
	}
	impacts, err := svc.runtime.inventory.ListRuntimeImpactByModuleVersion(ctx, reportID, baseVersionID, defaultListLimit, 0)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}
	if err != nil {
		return emptyRuntimeImpactResult()
	}
	return runtimeImpacts(impacts), nil
}

// Optional UI sections render as empty when their query dependency is not wired.
// That keeps Community pages usable while preserving the UI query boundary.
func (svc *Service) runtimeQueriesAvailable() bool {
	return svc.runtime.inventory != nil
}

func (svc *Service) dependencyGraphAvailable() bool {
	return svc.graph.dependencies != nil
}

func (svc *Service) governanceOwnersAvailable() bool {
	return svc.governance.owners != nil
}

func (svc *Service) governanceApprovalsAvailable() bool {
	return svc.governance.approvals != nil
}

func (svc *Service) governanceAuditAvailable() bool {
	return svc.governance.audit != nil
}

func emptyDependencyGraph(module domain.Module) ModuleDependencyGraph {
	return ModuleDependencyGraph{
		Module:     moduleInfo(module),
		Downstream: []DependencyModule{},
		Upstream:   []DependencyModule{},
		Unresolved: []UnresolvedDependency{},
	}
}

func emptyRuntimeEnvironmentInventory(environment string) RuntimeEnvironmentInventory {
	return RuntimeEnvironmentInventory{
		Environment: environment,
		Deployments: []RuntimeDeployment{},
		Usages:      []RuntimeModuleUsage{},
	}
}

func emptyRuntimeEnvironmentInventoryResult(environment string) (RuntimeEnvironmentInventory, error) {
	return emptyRuntimeEnvironmentInventory(environment), nil
}

func emptyModuleOwnersResult() ([]ModuleOwner, error) {
	return []ModuleOwner{}, nil
}

func missingBufConfigResult() (domain.BufConfigInfo, string, error) {
	return domain.BufConfigInfo{}, domain.BufLintStatusNotRun.String(), nil
}

func emptyMetadataSummaryResult() (DescriptorMetadataSummary, error) {
	return DescriptorMetadataSummary{}, nil
}

func emptyDescriptorMetadataResult() (DescriptorMetadata, error) {
	return DescriptorMetadata{Files: []ProtoFile{}}, nil
}

func emptyRuntimeImpact() []RuntimeImpact {
	return []RuntimeImpact{}
}

func emptyRuntimeImpactResult() ([]RuntimeImpact, error) {
	return emptyRuntimeImpact(), nil
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

func versionSummary(moduleVersion domain.ModuleVersion, artifacts []ArtifactSummary, lintStatus string, metadataSummary DescriptorMetadataSummary) VersionSummary {
	if artifacts == nil {
		artifacts = []ArtifactSummary{}
	}
	return VersionSummary{
		Version:         moduleVersion.Version.String(),
		Status:          moduleVersion.Status.String(),
		Digest:          moduleVersion.Digest,
		CreatedAt:       moduleVersion.CreatedAt,
		PublishedAt:     moduleVersion.PublishedAt,
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
