package httptransport

import (
	"sort"

	"github.com/alryzden/ProtoRadar/internal/domain"
	"github.com/alryzden/ProtoRadar/internal/usecase/registry"
)

type dependencyModuleDTO struct {
	Module            string   `json:"module"`
	LatestVersion     string   `json:"latest_version"`
	DependencySources []string `json:"dependency_sources"`
	Reasons           []string `json:"reasons"`
}

type unresolvedDependencyDTO struct {
	Module           string `json:"module"`
	Version          string `json:"version"`
	Source           string `json:"source"`
	ImportPath       string `json:"import_path,omitempty"`
	ReferencedSymbol string `json:"referenced_symbol,omitempty"`
	Reason           string `json:"reason"`
}

type moduleDependencyGraphDTO struct {
	Module     string                    `json:"module"`
	Upstream   []dependencyModuleDTO     `json:"upstream"`
	Downstream []dependencyModuleDTO     `json:"downstream"`
	Unresolved []unresolvedDependencyDTO `json:"unresolved"`
}

type affectedModulesDTO struct {
	Module          string                `json:"module"`
	AffectedModules []dependencyModuleDTO `json:"affected_modules"`
}

type breakingReportAffectedModulesDTO struct {
	ReportID        string                `json:"report_id"`
	Module          string                `json:"module"`
	Status          string                `json:"status"`
	AffectedModules []dependencyModuleDTO `json:"affected_modules"`
}

func moduleDependencyGraphResponse(response registry.ModuleDependencyGraphResponse) moduleDependencyGraphDTO {
	return moduleDependencyGraphDTO{
		Module:     response.Module.Name.String(),
		Upstream:   dependencyModuleResponses(response.Upstream, true),
		Downstream: dependencyModuleResponses(response.Downstream, false),
		Unresolved: unresolvedDependencyResponses(response.Unresolved),
	}
}

func affectedModulesResponse(response registry.AffectedModulesResponse) affectedModulesDTO {
	return affectedModulesDTO{
		Module:          response.Module.Name.String(),
		AffectedModules: affectedModuleResponses(response.AffectedModules),
	}
}

func breakingReportAffectedModulesResponse(response registry.BreakingReportAffectedModulesResponse) breakingReportAffectedModulesDTO {
	return breakingReportAffectedModulesDTO{
		ReportID:        response.Report.ID.String(),
		Module:          response.Report.ModuleName.String(),
		Status:          response.Report.Status.String(),
		AffectedModules: affectedModuleResponses(response.AffectedModules),
	}
}
func dependencyModuleResponses(dependencies []domain.ModuleDependency, upstream bool) []dependencyModuleDTO {
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
		groups[index].reasons[dependencyReasonText(dependency)] = struct{}{}
	}

	items := make([]dependencyModuleDTO, 0, len(groups))
	for _, group := range groups {
		items = append(items, dependencyModuleDTO{
			Module:            group.module,
			LatestVersion:     group.version,
			DependencySources: stringSetValues(group.sources),
			Reasons:           stringSetValues(group.reasons),
		})
	}
	return items
}

func affectedModuleResponses(affected []domain.AffectedModule) []dependencyModuleDTO {
	items := make([]dependencyModuleDTO, 0, len(affected))
	for _, item := range affected {
		sources := make([]string, 0, len(item.DependencySources))
		for _, source := range item.DependencySources {
			sources = append(sources, source.String())
		}
		reasons := make([]string, 0, len(item.Reasons))
		for _, reason := range item.Reasons {
			reasons = append(reasons, reason.String())
		}
		items = append(items, dependencyModuleDTO{
			Module:            item.ModuleName.String(),
			LatestVersion:     item.LatestVersion.String(),
			DependencySources: sources,
			Reasons:           reasons,
		})
	}
	return items
}

func unresolvedDependencyResponses(unresolved []domain.UnresolvedProtoDependency) []unresolvedDependencyDTO {
	items := make([]unresolvedDependencyDTO, 0, len(unresolved))
	for _, item := range unresolved {
		items = append(items, unresolvedDependencyDTO{
			Module:           item.ModuleName.String(),
			Version:          item.Version.String(),
			Source:           item.Source.String(),
			ImportPath:       item.ImportPath,
			ReferencedSymbol: item.ReferencedSymbol,
			Reason:           item.Reason.String(),
		})
	}
	return items
}

func dependencyReasonText(dependency domain.ModuleDependency) string {
	if dependency.ImportPath != "" {
		return "imports " + dependency.ImportPath
	}
	if dependency.ReferencedSymbol != "" {
		return "references " + dependency.ReferencedSymbol
	}
	return dependency.Reason.String()
}

func stringSetValues(values map[string]struct{}) []string {
	items := make([]string, 0, len(values))
	for value := range values {
		if value != "" {
			items = append(items, value)
		}
	}
	sort.Strings(items)
	return items
}
