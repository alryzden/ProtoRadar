package httptransport

import (
	"sort"
	"time"

	"github.com/alryzden/ProtoRadar/internal/domain"
	"github.com/alryzden/ProtoRadar/internal/usecase/registry"
	"github.com/alryzden/ProtoRadar/internal/usecase/runtimeinventory"
)

type errorResponse struct {
	Error apiErrorDTO `json:"error"`
}

type apiErrorDTO struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type readinessResponse struct {
	Status string            `json:"status"`
	Checks map[string]string `json:"checks"`
}

type createModuleRequest struct {
	Name          string `json:"name"`
	Description   string `json:"description"`
	RepositoryURL string `json:"repository_url"`
}

type moduleDTO struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Description   string    `json:"description"`
	RepositoryURL string    `json:"repository_url"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type listModulesResponse struct {
	Modules []moduleDTO `json:"modules"`
}

type linkModuleGitLabProjectRequest struct {
	GitLabBaseURL     string `json:"gitlab_base_url"`
	GitLabProjectID   int64  `json:"gitlab_project_id"`
	GitLabProjectPath string `json:"gitlab_project_path"`
}

type moduleGitLabProjectDTO struct {
	Module            string    `json:"module"`
	GitLabBaseURL     string    `json:"gitlab_base_url"`
	GitLabProjectID   int64     `json:"gitlab_project_id"`
	GitLabProjectPath string    `json:"gitlab_project_path"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type moduleVersionDTO struct {
	ID          string     `json:"id"`
	ModuleID    string     `json:"module_id"`
	Version     string     `json:"version"`
	Digest      string     `json:"digest"`
	Status      string     `json:"status"`
	CreatedAt   time.Time  `json:"created_at"`
	PublishedAt *time.Time `json:"published_at,omitempty"`
}

type listModuleVersionsResponse struct {
	Versions []moduleVersionDTO `json:"versions"`
}

type artifactDTO struct {
	ID              string    `json:"id"`
	ModuleVersionID string    `json:"module_version_id"`
	Kind            string    `json:"kind"`
	StorageKey      string    `json:"storage_key"`
	ChecksumSHA256  string    `json:"checksum_sha256"`
	SizeBytes       int64     `json:"size_bytes"`
	CreatedAt       time.Time `json:"created_at"`
}

type artifactSummaryDTO struct {
	Kind           string `json:"kind"`
	SizeBytes      int64  `json:"size_bytes"`
	ChecksumSHA256 string `json:"checksum_sha256"`
}

type bufInfoDTO struct {
	ConfigPresent         bool     `json:"config_present"`
	LockPresent           bool     `json:"lock_present"`
	BufYAMLDigest         string   `json:"buf_yaml_digest,omitempty"`
	BufLockDigest         string   `json:"buf_lock_digest,omitempty"`
	ModulePaths           []string `json:"module_paths,omitempty"`
	Deps                  []string `json:"deps,omitempty"`
	LintEnabled           bool     `json:"lint_enabled"`
	LintStatus            string   `json:"lint_status"`
	LintReport            string   `json:"lint_report,omitempty"`
	BreakingConfigPresent bool     `json:"breaking_config_present"`
}

type metadataSummaryDTO struct {
	Files      int `json:"files"`
	Packages   int `json:"packages,omitempty"`
	Imports    int `json:"imports"`
	Services   int `json:"services"`
	Methods    int `json:"methods"`
	Messages   int `json:"messages"`
	Fields     int `json:"fields"`
	Enums      int `json:"enums"`
	EnumValues int `json:"enum_values"`
}

type publishModuleVersionResponse struct {
	Module           string             `json:"module"`
	Version          string             `json:"version"`
	Status           string             `json:"status"`
	SourceArtifact   artifactSummaryDTO `json:"source_artifact"`
	BufImageArtifact artifactSummaryDTO `json:"buf_image_artifact"`
	Buf              bufInfoDTO         `json:"buf"`
	MetadataSummary  metadataSummaryDTO `json:"metadata_summary"`
	CreatedAt        time.Time          `json:"created_at"`
}

type moduleVersionDetailsDTO struct {
	ID              string               `json:"id"`
	ModuleID        string               `json:"module_id"`
	Module          string               `json:"module"`
	Version         string               `json:"version"`
	Digest          string               `json:"digest"`
	Status          string               `json:"status"`
	Artifacts       []artifactSummaryDTO `json:"artifacts"`
	Buf             bufInfoDTO           `json:"buf"`
	MetadataSummary metadataSummaryDTO   `json:"metadata_summary"`
	CreatedAt       time.Time            `json:"created_at"`
	PublishedAt     *time.Time           `json:"published_at,omitempty"`
}

type descriptorMetadataDTO struct {
	Files []protoFileDTO `json:"files"`
}

type breakingChangeDTO struct {
	Category    string `json:"category"`
	FilePath    string `json:"file_path"`
	PackageName string `json:"package_name"`
	Symbol      string `json:"symbol"`
	RuleID      string `json:"rule_id"`
	Message     string `json:"message"`
	Severity    string `json:"severity"`
}

type breakingReportDTO struct {
	ID           string              `json:"id"`
	Module       string              `json:"module"`
	Against      string              `json:"against"`
	TargetRef    string              `json:"target_ref"`
	Status       string              `json:"status"`
	ChangeCount  int                 `json:"change_count"`
	Changes      []breakingChangeDTO `json:"changes"`
	HumanSummary string              `json:"human_summary"`
	CreatedAt    time.Time           `json:"created_at"`
}

type breakingReportSummaryDTO struct {
	ID           string    `json:"id"`
	Module       string    `json:"module"`
	Against      string    `json:"against"`
	TargetRef    string    `json:"target_ref"`
	Status       string    `json:"status"`
	ChangeCount  int       `json:"change_count"`
	HumanSummary string    `json:"human_summary"`
	CreatedAt    time.Time `json:"created_at"`
}

type listBreakingReportsResponse struct {
	Reports []breakingReportSummaryDTO `json:"reports"`
}

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

type reportRuntimeInventoryRequest struct {
	ServiceName  string                         `json:"service_name"`
	Environment  string                         `json:"environment"`
	GitCommit    string                         `json:"git_commit"`
	BuildVersion string                         `json:"build_version"`
	Modules      []reportedRuntimeModuleRequest `json:"modules"`
}

type reportedRuntimeModuleRequest struct {
	Module  string `json:"module"`
	Version string `json:"version"`
}

type reportRuntimeInventoryDTO struct {
	DeploymentID string                  `json:"deployment_id"`
	ServiceName  string                  `json:"service_name"`
	Environment  string                  `json:"environment"`
	GitCommit    string                  `json:"git_commit"`
	BuildVersion string                  `json:"build_version"`
	ReportedAt   time.Time               `json:"reported_at"`
	Usages       []runtimeModuleUsageDTO `json:"usages"`
}

type runtimeModuleUsageDTO struct {
	Module        string `json:"module"`
	Version       string `json:"version"`
	LatestVersion string `json:"latest_version,omitempty"`
	DriftStatus   string `json:"drift_status"`
	DriftReason   string `json:"drift_reason"`
}

type runtimeServiceSummaryDTO struct {
	ServiceName    string                 `json:"service_name"`
	Environments   []string               `json:"environments"`
	LastReportedAt *time.Time             `json:"last_reported_at,omitempty"`
	DriftSummary   runtimeDriftSummaryDTO `json:"drift_summary"`
}

type listRuntimeServicesResponse struct {
	Services []runtimeServiceSummaryDTO `json:"services"`
}

type runtimeDriftSummaryDTO struct {
	UpToDate          int `json:"up_to_date"`
	BehindLatest      int `json:"behind_latest"`
	UnknownVersion    int `json:"unknown_version"`
	DeprecatedVersion int `json:"deprecated_version"`
}

type runtimeDeploymentDTO struct {
	ID           string    `json:"id"`
	ServiceName  string    `json:"service_name"`
	Environment  string    `json:"environment"`
	GitCommit    string    `json:"git_commit"`
	BuildVersion string    `json:"build_version"`
	ReportedAt   time.Time `json:"reported_at"`
	CreatedAt    time.Time `json:"created_at"`
}

type runtimeStoredModuleUsageDTO struct {
	ID            string    `json:"id"`
	DeploymentID  string    `json:"deployment_id"`
	Module        string    `json:"module"`
	ModuleID      *string   `json:"module_id,omitempty"`
	ModuleVersion *string   `json:"module_version_id,omitempty"`
	Version       string    `json:"version"`
	LatestVersion string    `json:"latest_version,omitempty"`
	DriftStatus   string    `json:"drift_status"`
	DriftReason   string    `json:"drift_reason"`
	CreatedAt     time.Time `json:"created_at"`
}

type runtimeServiceDetailsDTO struct {
	ServiceName string                        `json:"service_name"`
	Deployments []runtimeDeploymentDTO        `json:"deployments"`
	Usages      []runtimeStoredModuleUsageDTO `json:"usages"`
}

type runtimeEnvironmentInventoryDTO struct {
	Environment string                        `json:"environment"`
	Deployments []runtimeDeploymentDTO        `json:"deployments"`
	Usages      []runtimeStoredModuleUsageDTO `json:"usages"`
}

type moduleRuntimeUsageDTO struct {
	ServiceName  string    `json:"service_name"`
	Environment  string    `json:"environment"`
	DeploymentID string    `json:"deployment_id"`
	Module       string    `json:"module"`
	Version      string    `json:"version"`
	GitCommit    string    `json:"git_commit"`
	BuildVersion string    `json:"build_version"`
	ReportedAt   time.Time `json:"reported_at"`
	DriftStatus  string    `json:"drift_status"`
	DriftReason  string    `json:"drift_reason"`
}

type moduleRuntimeUsagesDTO struct {
	Module string                  `json:"module"`
	Usages []moduleRuntimeUsageDTO `json:"usages"`
}

type runtimeImpactDTO struct {
	ServiceName  string    `json:"service_name"`
	Environment  string    `json:"environment"`
	UsedModule   string    `json:"used_module"`
	UsedVersion  string    `json:"used_version"`
	GitCommit    string    `json:"git_commit"`
	BuildVersion string    `json:"build_version"`
	ReportedAt   time.Time `json:"reported_at"`
	ImpactStatus string    `json:"impact_status"`
	Reason       string    `json:"reason"`
}

type breakingReportRuntimeImpactDTO struct {
	ReportID string             `json:"report_id"`
	Impacts  []runtimeImpactDTO `json:"impacts"`
}

type protoFileDTO struct {
	Path        string            `json:"path"`
	PackageName string            `json:"package_name"`
	Syntax      string            `json:"syntax"`
	Imports     []protoImportDTO  `json:"imports,omitempty"`
	Services    []protoServiceDTO `json:"services,omitempty"`
	Messages    []protoMessageDTO `json:"messages,omitempty"`
	Enums       []protoEnumDTO    `json:"enums,omitempty"`
}

type protoImportDTO struct {
	Path   string `json:"path"`
	Public bool   `json:"public"`
	Weak   bool   `json:"weak"`
}

type protoServiceDTO struct {
	Name     string           `json:"name"`
	FullName string           `json:"full_name"`
	Methods  []protoMethodDTO `json:"methods,omitempty"`
}

type protoMethodDTO struct {
	Name            string `json:"name"`
	InputType       string `json:"input_type"`
	OutputType      string `json:"output_type"`
	ClientStreaming bool   `json:"client_streaming"`
	ServerStreaming bool   `json:"server_streaming"`
}

type protoMessageDTO struct {
	Name     string            `json:"name"`
	FullName string            `json:"full_name"`
	Fields   []protoFieldDTO   `json:"fields,omitempty"`
	Messages []protoMessageDTO `json:"messages,omitempty"`
	Enums    []protoEnumDTO    `json:"enums,omitempty"`
}

type protoFieldDTO struct {
	Name       string `json:"name"`
	Number     int32  `json:"number"`
	Type       string `json:"type"`
	TypeName   string `json:"type_name"`
	Label      string `json:"label"`
	JSONName   string `json:"json_name"`
	OneofName  string `json:"oneof_name"`
	IsRepeated bool   `json:"is_repeated"`
	IsMap      bool   `json:"is_map"`
}

type protoEnumDTO struct {
	Name     string              `json:"name"`
	FullName string              `json:"full_name"`
	Values   []protoEnumValueDTO `json:"values,omitempty"`
}

type protoEnumValueDTO struct {
	Name   string `json:"name"`
	Number int32  `json:"number"`
}

type createAPITokenRequest struct {
	Name      string `json:"name"`
	ExpiresAt string `json:"expires_at"`
}

type createAPITokenResponse struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Token     string     `json:"token"`
	CreatedAt time.Time  `json:"created_at"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

func moduleResponse(module domain.Module) moduleDTO {
	return moduleDTO{
		ID:            module.ID.String(),
		Name:          module.Name.String(),
		Description:   module.Description,
		RepositoryURL: module.RepositoryURL,
		CreatedAt:     module.CreatedAt,
		UpdatedAt:     module.UpdatedAt,
	}
}

func moduleGitLabProjectResponse(mapping domain.ModuleGitLabProject) moduleGitLabProjectDTO {
	return moduleGitLabProjectDTO{
		Module:            mapping.ModuleName.String(),
		GitLabBaseURL:     mapping.GitLabBaseURL,
		GitLabProjectID:   mapping.GitLabProjectID,
		GitLabProjectPath: mapping.GitLabProjectPath,
		CreatedAt:         mapping.CreatedAt,
		UpdatedAt:         mapping.UpdatedAt,
	}
}

func moduleVersionResponse(version domain.ModuleVersion) moduleVersionDTO {
	return moduleVersionDTO{
		ID:          version.ID.String(),
		ModuleID:    version.ModuleID.String(),
		Version:     version.Version.String(),
		Digest:      version.Digest,
		Status:      version.Status.String(),
		CreatedAt:   version.CreatedAt,
		PublishedAt: version.PublishedAt,
	}
}

func artifactResponse(artifact domain.Artifact) artifactDTO {
	return artifactDTO{
		ID:              artifact.ID.String(),
		ModuleVersionID: artifact.ModuleVersionID.String(),
		Kind:            artifact.Kind.String(),
		StorageKey:      artifact.StorageKey,
		ChecksumSHA256:  artifact.ChecksumSHA256,
		SizeBytes:       artifact.SizeBytes,
		CreatedAt:       artifact.CreatedAt,
	}
}

func artifactSummaryResponse(artifact domain.Artifact) artifactSummaryDTO {
	return artifactSummaryDTO{
		Kind:           artifact.Kind.String(),
		SizeBytes:      artifact.SizeBytes,
		ChecksumSHA256: artifact.ChecksumSHA256,
	}
}

func bufInfoResponse(config domain.BufConfigInfo, lint domain.BufLintResult) bufInfoDTO {
	lintStatus := lint.Status.String()
	if lintStatus == "" {
		lintStatus = domain.BufLintStatusNotRun.String()
	}
	return bufInfoDTO{
		ConfigPresent:         config.BufYAMLPresent,
		LockPresent:           config.BufLockPresent,
		BufYAMLDigest:         config.BufYAMLDigest,
		BufLockDigest:         config.BufLockDigest,
		ModulePaths:           config.ModulePaths,
		Deps:                  config.Deps,
		LintEnabled:           config.LintEnabled,
		LintStatus:            lintStatus,
		LintReport:            lint.Report,
		BreakingConfigPresent: config.BreakingConfigPresent,
	}
}

func metadataSummaryResponse(summary domain.DescriptorMetadataSummary) metadataSummaryDTO {
	return metadataSummaryDTO{
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

func moduleVersionDetailsResponse(moduleName string, details registry.ModuleVersionDetailsResponse) moduleVersionDetailsDTO {
	artifacts := make([]artifactSummaryDTO, 0, len(details.Artifacts))
	for _, artifact := range details.Artifacts {
		artifacts = append(artifacts, artifactSummaryResponse(artifact))
	}
	return moduleVersionDetailsDTO{
		ID:              details.Version.ID.String(),
		ModuleID:        details.Version.ModuleID.String(),
		Module:          moduleName,
		Version:         details.Version.Version.String(),
		Digest:          details.Version.Digest,
		Status:          details.Version.Status.String(),
		Artifacts:       artifacts,
		Buf:             bufInfoResponse(details.BufConfig, details.LintResult),
		MetadataSummary: metadataSummaryResponse(details.MetadataSummary),
		CreatedAt:       details.Version.CreatedAt,
		PublishedAt:     details.Version.PublishedAt,
	}
}

func descriptorMetadataResponse(metadata domain.DescriptorMetadata) descriptorMetadataDTO {
	files := make([]protoFileDTO, 0, len(metadata.Files))
	for _, file := range metadata.Files {
		files = append(files, protoFileResponse(file))
	}
	return descriptorMetadataDTO{Files: files}
}

func breakingReportResponse(report domain.BreakingReport, changes []domain.BreakingChange) breakingReportDTO {
	items := make([]breakingChangeDTO, 0, len(changes))
	for _, change := range changes {
		items = append(items, breakingChangeResponse(change))
	}
	return breakingReportDTO{
		ID:           report.ID.String(),
		Module:       report.ModuleName.String(),
		Against:      report.BaseVersion.String(),
		TargetRef:    report.TargetRef,
		Status:       report.Status.String(),
		ChangeCount:  report.ChangeCount,
		Changes:      items,
		HumanSummary: report.HumanSummary,
		CreatedAt:    report.CreatedAt,
	}
}

func breakingReportSummaryResponse(report domain.BreakingReport) breakingReportSummaryDTO {
	return breakingReportSummaryDTO{
		ID:           report.ID.String(),
		Module:       report.ModuleName.String(),
		Against:      report.BaseVersion.String(),
		TargetRef:    report.TargetRef,
		Status:       report.Status.String(),
		ChangeCount:  report.ChangeCount,
		HumanSummary: report.HumanSummary,
		CreatedAt:    report.CreatedAt,
	}
}

func breakingChangeResponse(change domain.BreakingChange) breakingChangeDTO {
	return breakingChangeDTO{
		Category:    change.Category,
		FilePath:    change.FilePath,
		PackageName: change.PackageName,
		Symbol:      change.Symbol,
		RuleID:      change.RuleID,
		Message:     change.Message,
		Severity:    change.Severity,
	}
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

func reportRuntimeInventoryResponse(output runtimeinventory.ReportRuntimeInventoryOutput) reportRuntimeInventoryDTO {
	usages := make([]runtimeModuleUsageDTO, 0, len(output.Usages))
	for _, usage := range output.Usages {
		usages = append(usages, runtimeModuleUsageDTO{
			Module:        usage.Module,
			Version:       usage.Version,
			LatestVersion: usage.LatestVersion,
			DriftStatus:   usage.DriftStatus.String(),
			DriftReason:   usage.DriftReason,
		})
	}
	return reportRuntimeInventoryDTO{
		DeploymentID: output.DeploymentID,
		ServiceName:  output.ServiceName,
		Environment:  output.Environment,
		GitCommit:    output.GitCommit,
		BuildVersion: output.BuildVersion,
		ReportedAt:   output.ReportedAt,
		Usages:       usages,
	}
}

func runtimeServicesResponse(summaries []domain.RuntimeServiceSummary) listRuntimeServicesResponse {
	items := make([]runtimeServiceSummaryDTO, 0, len(summaries))
	for _, summary := range summaries {
		environments := make([]string, 0, len(summary.Environments))
		for _, environment := range summary.Environments {
			environments = append(environments, environment.String())
		}
		items = append(items, runtimeServiceSummaryDTO{
			ServiceName:    summary.Service.Name.String(),
			Environments:   environments,
			LastReportedAt: summary.LatestReportedAt,
			DriftSummary: runtimeDriftSummaryDTO{
				UpToDate:          summary.UpToDateCount,
				BehindLatest:      summary.BehindLatestCount,
				UnknownVersion:    summary.UnknownVersionCount,
				DeprecatedVersion: summary.DeprecatedCount,
			},
		})
	}
	return listRuntimeServicesResponse{Services: items}
}

func runtimeServiceDetailsResponse(details domain.RuntimeServiceDetails) runtimeServiceDetailsDTO {
	return runtimeServiceDetailsDTO{
		ServiceName: details.Service.Name.String(),
		Deployments: runtimeDeploymentResponses(details.Deployments),
		Usages:      runtimeStoredModuleUsageResponses(details.Usages),
	}
}

func runtimeEnvironmentInventoryResponse(inventory domain.RuntimeEnvironmentInventory) runtimeEnvironmentInventoryDTO {
	return runtimeEnvironmentInventoryDTO{
		Environment: inventory.Environment.String(),
		Deployments: runtimeDeploymentResponses(inventory.Deployments),
		Usages:      runtimeStoredModuleUsageResponses(inventory.Usages),
	}
}

func moduleRuntimeUsagesResponse(moduleName string, usages []domain.ModuleRuntimeUsage) moduleRuntimeUsagesDTO {
	items := make([]moduleRuntimeUsageDTO, 0, len(usages))
	for _, usage := range usages {
		items = append(items, moduleRuntimeUsageDTO{
			ServiceName:  usage.ServiceName.String(),
			Environment:  usage.Environment.String(),
			DeploymentID: usage.DeploymentID.String(),
			Module:       usage.ModuleName.String(),
			Version:      usage.Version.String(),
			GitCommit:    usage.GitCommit,
			BuildVersion: usage.BuildVersion,
			ReportedAt:   usage.ReportedAt,
			DriftStatus:  usage.DriftStatus.String(),
			DriftReason:  usage.DriftReason,
		})
	}
	return moduleRuntimeUsagesDTO{Module: moduleName, Usages: items}
}

func breakingReportRuntimeImpactResponse(reportID string, impacts []domain.RuntimeImpact) breakingReportRuntimeImpactDTO {
	items := make([]runtimeImpactDTO, 0, len(impacts))
	for _, impact := range impacts {
		items = append(items, runtimeImpactDTO{
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
	return breakingReportRuntimeImpactDTO{ReportID: reportID, Impacts: items}
}

func runtimeDeploymentResponses(deployments []domain.RuntimeDeployment) []runtimeDeploymentDTO {
	items := make([]runtimeDeploymentDTO, 0, len(deployments))
	for _, deployment := range deployments {
		items = append(items, runtimeDeploymentDTO{
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

func runtimeStoredModuleUsageResponses(usages []domain.RuntimeModuleUsage) []runtimeStoredModuleUsageDTO {
	items := make([]runtimeStoredModuleUsageDTO, 0, len(usages))
	for _, usage := range usages {
		var moduleID *string
		if usage.ModuleID != nil {
			value := usage.ModuleID.String()
			moduleID = &value
		}
		var moduleVersionID *string
		if usage.ModuleVersionID != nil {
			value := usage.ModuleVersionID.String()
			moduleVersionID = &value
		}
		latestVersion := ""
		if usage.LatestVersion != nil {
			latestVersion = usage.LatestVersion.String()
		}
		items = append(items, runtimeStoredModuleUsageDTO{
			ID:            usage.ID.String(),
			DeploymentID:  usage.DeploymentID.String(),
			Module:        usage.ModuleName.String(),
			ModuleID:      moduleID,
			ModuleVersion: moduleVersionID,
			Version:       usage.Version.String(),
			LatestVersion: latestVersion,
			DriftStatus:   usage.DriftStatus.String(),
			DriftReason:   usage.DriftReason,
			CreatedAt:     usage.CreatedAt,
		})
	}
	return items
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

func protoFileResponse(file domain.ProtoFile) protoFileDTO {
	imports := make([]protoImportDTO, 0, len(file.Imports))
	for _, protoImport := range file.Imports {
		imports = append(imports, protoImportDTO{
			Path:   protoImport.Path,
			Public: protoImport.Public,
			Weak:   protoImport.Weak,
		})
	}
	services := make([]protoServiceDTO, 0, len(file.Services))
	for _, service := range file.Services {
		services = append(services, protoServiceResponse(service))
	}
	messages := make([]protoMessageDTO, 0, len(file.Messages))
	for _, message := range file.Messages {
		messages = append(messages, protoMessageResponse(message))
	}
	enums := make([]protoEnumDTO, 0, len(file.Enums))
	for _, enum := range file.Enums {
		enums = append(enums, protoEnumResponse(enum))
	}
	return protoFileDTO{
		Path:        file.Path,
		PackageName: file.PackageName,
		Syntax:      file.Syntax,
		Imports:     imports,
		Services:    services,
		Messages:    messages,
		Enums:       enums,
	}
}

func protoServiceResponse(service domain.ProtoService) protoServiceDTO {
	methods := make([]protoMethodDTO, 0, len(service.Methods))
	for _, method := range service.Methods {
		methods = append(methods, protoMethodDTO{
			Name:            method.Name,
			InputType:       method.InputType,
			OutputType:      method.OutputType,
			ClientStreaming: method.ClientStreaming,
			ServerStreaming: method.ServerStreaming,
		})
	}
	return protoServiceDTO{Name: service.Name, FullName: service.FullName, Methods: methods}
}

func protoMessageResponse(message domain.ProtoMessage) protoMessageDTO {
	fields := make([]protoFieldDTO, 0, len(message.Fields))
	for _, field := range message.Fields {
		fields = append(fields, protoFieldDTO{
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
	messages := make([]protoMessageDTO, 0, len(message.Messages))
	for _, nested := range message.Messages {
		messages = append(messages, protoMessageResponse(nested))
	}
	enums := make([]protoEnumDTO, 0, len(message.Enums))
	for _, enum := range message.Enums {
		enums = append(enums, protoEnumResponse(enum))
	}
	return protoMessageDTO{
		Name:     message.Name,
		FullName: message.FullName,
		Fields:   fields,
		Messages: messages,
		Enums:    enums,
	}
}

func protoEnumResponse(enum domain.ProtoEnum) protoEnumDTO {
	values := make([]protoEnumValueDTO, 0, len(enum.Values))
	for _, value := range enum.Values {
		values = append(values, protoEnumValueDTO{Name: value.Name, Number: value.Number})
	}
	return protoEnumDTO{Name: enum.Name, FullName: enum.FullName, Values: values}
}
