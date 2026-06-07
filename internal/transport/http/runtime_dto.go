package httptransport

import (
	"time"

	"github.com/alryzden/ProtoRadar/internal/domain"
	"github.com/alryzden/ProtoRadar/internal/usecase/runtimeinventory"
)

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
	DriftStatus  string    `json:"drift_status,omitempty"`
	DriftReason  string    `json:"drift_reason,omitempty"`
}

type breakingReportRuntimeImpactDTO struct {
	ReportID string             `json:"report_id"`
	Impacts  []runtimeImpactDTO `json:"impacts"`
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
			DriftStatus:  impact.DriftStatus.String(),
			DriftReason:  impact.DriftReason,
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
