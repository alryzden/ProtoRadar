package domain

import (
	"context"
	"time"
)

type ModuleRepository interface {
	Create(ctx context.Context, module Module) error
	GetByID(ctx context.Context, id ModuleID) (Module, error)
	GetByName(ctx context.Context, name ModuleName) (Module, error)
	List(ctx context.Context, limit int, offset int) ([]Module, error)
}

type ModuleGitLabProjectRepository interface {
	Upsert(ctx context.Context, mapping ModuleGitLabProject) error
	GetByModuleID(ctx context.Context, moduleID ModuleID) (ModuleGitLabProject, error)
	GetByGitLabProject(ctx context.Context, gitLabBaseURL string, gitLabProjectID int64) (ModuleGitLabProject, error)
}

type ModuleVersionRepository interface {
	Create(ctx context.Context, version ModuleVersion) error
	GetByID(ctx context.Context, id ModuleVersionID) (ModuleVersion, error)
	GetByModuleAndVersion(ctx context.Context, moduleID ModuleID, version Version) (ModuleVersion, error)
	GetLatestByModule(ctx context.Context, moduleID ModuleID) (ModuleVersion, error)
	ListByModule(ctx context.Context, moduleID ModuleID, limit int, offset int) ([]ModuleVersion, error)
}

type ArtifactRepository interface {
	Create(ctx context.Context, artifact Artifact) error
	GetByID(ctx context.Context, id ArtifactID) (Artifact, error)
	GetByModuleVersion(ctx context.Context, moduleVersionID ModuleVersionID) (Artifact, error)
}

type ModuleVersionArtifactRepository interface {
	Create(ctx context.Context, artifact Artifact) error
	GetByID(ctx context.Context, id ArtifactID) (Artifact, error)
	GetByModuleVersionAndKind(ctx context.Context, moduleVersionID ModuleVersionID, kind ArtifactKind) (Artifact, error)
	ListByModuleVersion(ctx context.Context, moduleVersionID ModuleVersionID) ([]Artifact, error)
}

type DescriptorMetadataRepository interface {
	Save(ctx context.Context, moduleVersionID ModuleVersionID, metadata DescriptorMetadata) error
	GetByModuleVersion(ctx context.Context, moduleVersionID ModuleVersionID) (DescriptorMetadata, error)
	GetSummaryByModuleVersion(ctx context.Context, moduleVersionID ModuleVersionID) (DescriptorMetadataSummary, error)
}

type ModuleDependencyRepository interface {
	ReplaceByConsumerModuleVersion(ctx context.Context, consumerModuleVersionID ModuleVersionID, dependencies []ModuleDependency, unresolved []UnresolvedProtoDependency) error
	ListUpstreamByModule(ctx context.Context, moduleID ModuleID) ([]ModuleDependency, error)
	ListUpstreamByModuleVersion(ctx context.Context, moduleVersionID ModuleVersionID) ([]ModuleDependency, error)
	ListDownstreamByModule(ctx context.Context, moduleID ModuleID) ([]ModuleDependency, error)
	ListAffectedModules(ctx context.Context, providerModuleID ModuleID) ([]AffectedModule, error)
	ListUnresolvedByModule(ctx context.Context, moduleID ModuleID) ([]UnresolvedProtoDependency, error)
	ListUnresolvedByModuleVersion(ctx context.Context, moduleVersionID ModuleVersionID) ([]UnresolvedProtoDependency, error)
}

type RuntimeInventoryRepository interface {
	UpsertRuntimeServiceByName(ctx context.Context, service RuntimeService) (RuntimeService, error)
	GetRuntimeServiceByName(ctx context.Context, serviceName RuntimeServiceName) (RuntimeService, error)
	CreateRuntimeDeployment(ctx context.Context, deployment RuntimeDeployment) error
	CreateRuntimeModuleUsages(ctx context.Context, usages []RuntimeModuleUsage) error
	ListRuntimeDeploymentsByService(ctx context.Context, serviceID RuntimeServiceID, limit int, offset int) ([]RuntimeDeployment, error)
	ListRuntimeModuleUsagesByDeployment(ctx context.Context, deploymentID RuntimeDeploymentID) ([]RuntimeModuleUsage, error)
	ListLatestRuntimeUsagesByServiceEnvironment(ctx context.Context, serviceName RuntimeServiceName, environment RuntimeEnvironment) ([]RuntimeModuleUsage, error)
	ListRuntimeServices(ctx context.Context, limit int, offset int) ([]RuntimeServiceSummary, error)
	GetRuntimeServiceDetails(ctx context.Context, serviceName RuntimeServiceName) (RuntimeServiceDetails, error)
	ListRuntimeEnvironmentInventory(ctx context.Context, environment RuntimeEnvironment, limit int, offset int) (RuntimeEnvironmentInventory, error)
	ListModuleRuntimeUsages(ctx context.Context, moduleID ModuleID, limit int, offset int) ([]ModuleRuntimeUsage, error)
	ListModuleRuntimeUsagesByModuleName(ctx context.Context, moduleName ModuleName, limit int, offset int) ([]ModuleRuntimeUsage, error)
	ListRuntimeModuleUsagesByDriftStatus(ctx context.Context, status RuntimeDriftStatus, limit int, offset int) ([]RuntimeModuleUsage, error)
	ListRuntimeImpactByModuleVersion(ctx context.Context, reportID BreakingReportID, moduleVersionID ModuleVersionID, limit int, offset int) ([]RuntimeImpact, error)
}

type BufConfigRepository interface {
	Save(ctx context.Context, moduleVersionID ModuleVersionID, config BufConfigInfo) error
	GetByModuleVersion(ctx context.Context, moduleVersionID ModuleVersionID) (BufConfigInfo, error)
}

type BreakingReportRepository interface {
	Create(ctx context.Context, report BreakingReport, changes []BreakingChange) error
	GetByID(ctx context.Context, id BreakingReportID) (BreakingReport, []BreakingChange, error)
	ListByModule(ctx context.Context, moduleID ModuleID, limit int, offset int) ([]BreakingReport, error)
	CountChangesByReport(ctx context.Context, reportID BreakingReportID) (int, error)
	ListChangesByReport(ctx context.Context, reportID BreakingReportID, limit int, offset int) ([]BreakingChange, error)
}

type APITokenRepository interface {
	Create(ctx context.Context, token APIToken) error
	GetByID(ctx context.Context, id APITokenID) (APIToken, error)
	GetByHash(ctx context.Context, tokenHash string) (APIToken, error)
	MarkUsed(ctx context.Context, id APITokenID, usedAt time.Time) error
}

type RegistryTransactionManager interface {
	WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error
}
