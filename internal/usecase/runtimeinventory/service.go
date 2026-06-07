package runtimeinventory

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/alryzden/ProtoRadar/internal/domain"
	"github.com/alryzden/ProtoRadar/internal/integration/protoradarevents"
	"github.com/alryzden/ProtoRadar/internal/outbox"
)

const (
	DriftReasonUpToDate        = "up_to_date"
	DriftReasonModuleNotFound  = "module_not_found"
	DriftReasonVersionNotFound = "version_not_found"
	DriftReasonBehindLatest    = "behind_latest"
	DriftReasonDeprecated      = "deprecated_version"
)

type Service struct {
	modules      domain.ModuleRepository
	versions     domain.ModuleVersionRepository
	runtime      domain.RuntimeInventoryRepository
	reports      domain.BreakingReportRepository
	transactions domain.RegistryTransactionManager
	outbox       outbox.Writer
	clock        Clock
	ids          IDGenerator
}

func NewService(
	modules domain.ModuleRepository,
	versions domain.ModuleVersionRepository,
	runtime domain.RuntimeInventoryRepository,
	reports domain.BreakingReportRepository,
	transactions domain.RegistryTransactionManager,
	outboxWriter outbox.Writer,
	clock Clock,
	ids IDGenerator,
) *Service {
	return &Service{
		modules:      modules,
		versions:     versions,
		runtime:      runtime,
		reports:      reports,
		transactions: transactions,
		outbox:       outboxWriter,
		clock:        clock,
		ids:          ids,
	}
}

func (svc *Service) ReportRuntimeInventory(ctx context.Context, input ReportRuntimeInventoryInput) (ReportRuntimeInventoryOutput, error) {
	serviceName, err := domain.NewRuntimeServiceName(input.ServiceName)
	if err != nil {
		return ReportRuntimeInventoryOutput{}, err
	}
	environment, err := domain.NewRuntimeEnvironment(input.Environment)
	if err != nil {
		return ReportRuntimeInventoryOutput{}, err
	}
	gitCommit := strings.TrimSpace(input.GitCommit)
	if err := domain.ValidateRuntimeGitCommit(gitCommit); err != nil {
		return ReportRuntimeInventoryOutput{}, err
	}
	buildVersion := strings.TrimSpace(input.BuildVersion)
	if err := domain.ValidateRuntimeBuildVersion(buildVersion); err != nil {
		return ReportRuntimeInventoryOutput{}, err
	}
	reportedAt := svc.clock.Now()
	if input.ReportedAt != nil {
		reportedAt = *input.ReportedAt
	}
	references, err := svc.reportedModuleReferences(input.Modules)
	if err != nil {
		return ReportRuntimeInventoryOutput{}, err
	}

	serviceID, err := svc.ids.NewRuntimeServiceID()
	if err != nil {
		return ReportRuntimeInventoryOutput{}, err
	}
	deploymentID, err := svc.ids.NewRuntimeDeploymentID()
	if err != nil {
		return ReportRuntimeInventoryOutput{}, err
	}

	resolved := make([]resolvedRuntimeUsage, 0, len(references))
	for _, reference := range references {
		usage, err := svc.resolveRuntimeUsage(ctx, reference)
		if err != nil {
			return ReportRuntimeInventoryOutput{}, err
		}
		usageID, err := svc.ids.NewRuntimeModuleUsageID()
		if err != nil {
			return ReportRuntimeInventoryOutput{}, err
		}
		usage.usage.ID = usageID
		usage.usage.DeploymentID = deploymentID
		usage.usage.CreatedAt = reportedAt
		resolved = append(resolved, usage)
	}

	usageEntities := make([]domain.RuntimeModuleUsage, 0, len(resolved))
	usageOutputs := make([]RuntimeModuleUsageOutput, 0, len(resolved))
	counts := DriftCounts{}
	for _, item := range resolved {
		usageEntities = append(usageEntities, item.usage)
		usageOutputs = append(usageOutputs, runtimeUsageOutput(item.usage))
		counts.Add(item.usage.DriftStatus)
	}

	deployment := domain.RuntimeDeployment{
		ID:           deploymentID,
		ServiceID:    serviceID,
		ServiceName:  serviceName,
		Environment:  environment,
		GitCommit:    gitCommit,
		BuildVersion: buildVersion,
		ReportedAt:   reportedAt,
		CreatedAt:    reportedAt,
	}
	service := domain.RuntimeService{ID: serviceID, Name: serviceName, CreatedAt: reportedAt, UpdatedAt: reportedAt}

	err = svc.transactions.WithinTransaction(ctx, func(txCtx context.Context) error {
		storedService, err := svc.runtime.UpsertRuntimeServiceByName(txCtx, service)
		if err != nil {
			return err
		}
		deployment.ServiceID = storedService.ID
		deployment.ServiceName = storedService.Name
		if err := svc.runtime.CreateRuntimeDeployment(txCtx, deployment); err != nil {
			return err
		}
		if err := svc.runtime.CreateRuntimeModuleUsages(txCtx, usageEntities); err != nil {
			return err
		}
		record, err := protoradarevents.NewRuntimeInventoryReported(protoradarevents.RuntimeInventoryReported{
			DeploymentID:     deployment.ID,
			ServiceID:        storedService.ID,
			ServiceName:      storedService.Name,
			Environment:      deployment.Environment,
			GitCommit:        deployment.GitCommit,
			BuildVersion:     deployment.BuildVersion,
			ModuleUsageCount: len(usageEntities),
			DriftCounts: protoradarevents.RuntimeInventoryDriftCounts{
				UpToDate:          counts.UpToDate,
				BehindLatest:      counts.BehindLatest,
				UnknownVersion:    counts.UnknownVersion,
				DeprecatedVersion: counts.DeprecatedVersion,
			},
			OccurredAt: reportedAt,
		})
		if err != nil {
			return err
		}
		return svc.outbox.Create(txCtx, record)
	})
	if err != nil {
		return ReportRuntimeInventoryOutput{}, err
	}

	return ReportRuntimeInventoryOutput{
		DeploymentID: deployment.ID.String(),
		ServiceName:  serviceName.String(),
		Environment:  environment.String(),
		GitCommit:    gitCommit,
		BuildVersion: buildVersion,
		ReportedAt:   reportedAt,
		Usages:       usageOutputs,
		DriftCounts:  counts,
	}, nil
}

func (svc *Service) ListRuntimeServices(ctx context.Context, limit int, offset int) ([]domain.RuntimeServiceSummary, error) {
	return svc.runtime.ListRuntimeServices(ctx, limit, offset)
}

func (svc *Service) GetRuntimeServiceDetails(ctx context.Context, serviceNameValue string) (domain.RuntimeServiceDetails, error) {
	serviceName, err := domain.NewRuntimeServiceName(serviceNameValue)
	if err != nil {
		return domain.RuntimeServiceDetails{}, err
	}
	return svc.runtime.GetRuntimeServiceDetails(ctx, serviceName)
}

func (svc *Service) GetEnvironmentInventory(ctx context.Context, environmentValue string, limit int, offset int) (domain.RuntimeEnvironmentInventory, error) {
	environment, err := domain.NewRuntimeEnvironment(environmentValue)
	if err != nil {
		return domain.RuntimeEnvironmentInventory{}, err
	}
	return svc.runtime.ListRuntimeEnvironmentInventory(ctx, environment, limit, offset)
}

func (svc *Service) GetModuleRuntimeUsages(ctx context.Context, moduleNameValue string, limit int, offset int) ([]domain.ModuleRuntimeUsage, error) {
	moduleName, err := domain.NewModuleName(moduleNameValue)
	if err != nil {
		return nil, err
	}
	module, err := svc.modules.GetByName(ctx, moduleName)
	if errors.Is(err, domain.ErrNotFound) {
		return svc.runtime.ListModuleRuntimeUsagesByModuleName(ctx, moduleName, limit, offset)
	}
	if err != nil {
		return nil, err
	}
	return svc.runtime.ListModuleRuntimeUsages(ctx, module.ID, limit, offset)
}

func (svc *Service) GetBreakingReportRuntimeImpact(ctx context.Context, reportID domain.BreakingReportID, limit int, offset int) ([]domain.RuntimeImpact, error) {
	report, _, err := svc.reports.GetByID(ctx, reportID)
	if err != nil {
		return nil, err
	}
	return svc.runtime.ListRuntimeImpactByModuleVersion(ctx, report.ID, report.BaseVersionID, limit, offset)
}

func (svc *Service) reportedModuleReferences(inputs []ReportedModuleInput) ([]reportedModuleReference, error) {
	if len(inputs) == 0 {
		return nil, ErrRuntimeModulesRequired
	}
	seen := map[string]struct{}{}
	refs := make([]reportedModuleReference, 0, len(inputs))
	for _, input := range inputs {
		moduleName, err := domain.NewModuleName(input.Module)
		if err != nil {
			return nil, err
		}
		version, err := domain.NewVersion(input.Version)
		if err != nil {
			return nil, err
		}
		key := moduleName.String() + "\x00" + version.String()
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		refs = append(refs, reportedModuleReference{ModuleName: moduleName, Version: version})
	}
	if len(refs) == 0 {
		return nil, ErrRuntimeModulesRequired
	}
	return refs, nil
}

func (svc *Service) resolveRuntimeUsage(ctx context.Context, reference reportedModuleReference) (resolvedRuntimeUsage, error) {
	usage := domain.RuntimeModuleUsage{
		ModuleName:    reference.ModuleName,
		Version:       reference.Version,
		DriftStatus:   domain.RuntimeDriftStatusUnknownVersion,
		DriftReason:   DriftReasonModuleNotFound,
		LatestVersion: nil,
	}
	module, err := svc.modules.GetByName(ctx, reference.ModuleName)
	if errors.Is(err, domain.ErrNotFound) {
		return resolvedRuntimeUsage{usage: usage}, nil
	}
	if err != nil {
		return resolvedRuntimeUsage{}, err
	}
	usage.ModuleID = &module.ID

	moduleVersion, err := svc.versions.GetByModuleAndVersion(ctx, module.ID, reference.Version)
	if errors.Is(err, domain.ErrNotFound) {
		usage.DriftReason = DriftReasonVersionNotFound
		return resolvedRuntimeUsage{usage: usage}, nil
	}
	if err != nil {
		return resolvedRuntimeUsage{}, err
	}
	usage.ModuleVersionID = &moduleVersion.ID

	latest, err := svc.versions.GetLatestByModule(ctx, module.ID)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return resolvedRuntimeUsage{}, err
	}
	if err == nil {
		usage.LatestVersion = &latest.Version
	}

	// Drift precedence is intentional:
	// 1. unknown_version
	// 2. deprecated_version
	// 3. behind_latest
	// 4. up_to_date
	if moduleVersion.IsDeprecated() {
		usage.DriftStatus = domain.RuntimeDriftStatusDeprecatedVersion
		usage.DriftReason = DriftReasonDeprecated
		return resolvedRuntimeUsage{usage: usage}, nil
	}

	if latest.ID == moduleVersion.ID || latest.Version == moduleVersion.Version {
		usage.DriftStatus = domain.RuntimeDriftStatusUpToDate
		usage.DriftReason = DriftReasonUpToDate
		return resolvedRuntimeUsage{usage: usage}, nil
	}
	usage.DriftStatus = domain.RuntimeDriftStatusBehindLatest
	if usage.LatestVersion != nil {
		usage.DriftReason = fmt.Sprintf("%s: latest version is %s", DriftReasonBehindLatest, usage.LatestVersion.String())
	} else {
		usage.DriftReason = DriftReasonBehindLatest
	}
	return resolvedRuntimeUsage{usage: usage}, nil
}

type reportedModuleReference struct {
	ModuleName domain.ModuleName
	Version    domain.Version
}

type resolvedRuntimeUsage struct {
	usage domain.RuntimeModuleUsage
}

func runtimeUsageOutput(usage domain.RuntimeModuleUsage) RuntimeModuleUsageOutput {
	latestVersion := ""
	if usage.LatestVersion != nil {
		latestVersion = usage.LatestVersion.String()
	}
	return RuntimeModuleUsageOutput{
		Module:        usage.ModuleName.String(),
		Version:       usage.Version.String(),
		LatestVersion: latestVersion,
		DriftStatus:   usage.DriftStatus,
		DriftReason:   usage.DriftReason,
	}
}
