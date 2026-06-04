package registry

import (
	"context"
	"errors"

	"github.com/alryzden/ProtoRadar/internal/domain"
)

func (svc *Service) GetModuleDependencyGraph(ctx context.Context, moduleName string) (ModuleDependencyGraphResponse, error) {
	module, err := svc.GetModule(ctx, moduleName)
	if err != nil {
		return ModuleDependencyGraphResponse{}, err
	}
	if svc.dependencyRead == nil {
		return ModuleDependencyGraphResponse{}, ErrStorageFailure
	}

	upstream, err := svc.dependencyRead.ListUpstreamByModule(ctx, module.ID)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return ModuleDependencyGraphResponse{}, err
	}
	downstream, err := svc.dependencyRead.ListDownstreamByModule(ctx, module.ID)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return ModuleDependencyGraphResponse{}, err
	}
	unresolved, err := svc.dependencyRead.ListUnresolvedByModule(ctx, module.ID)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return ModuleDependencyGraphResponse{}, err
	}

	return ModuleDependencyGraphResponse{
		Module:     module,
		Upstream:   upstream,
		Downstream: downstream,
		Unresolved: unresolved,
	}, nil
}

func (svc *Service) ListAffectedModules(ctx context.Context, moduleName string) (AffectedModulesResponse, error) {
	module, err := svc.GetModule(ctx, moduleName)
	if err != nil {
		return AffectedModulesResponse{}, err
	}
	if svc.dependencyRead == nil {
		return AffectedModulesResponse{}, ErrStorageFailure
	}

	affected, err := svc.dependencyRead.ListAffectedModules(ctx, module.ID)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return AffectedModulesResponse{}, err
	}
	return AffectedModulesResponse{Module: module, AffectedModules: affected}, nil
}

func (svc *Service) GetBreakingReportAffectedModules(ctx context.Context, reportIDValue string) (BreakingReportAffectedModulesResponse, error) {
	reportID := domain.NewBreakingReportID(reportIDValue)
	if reportID == "" {
		return BreakingReportAffectedModulesResponse{}, ErrBreakingReportNotFound
	}
	report, _, err := svc.reports.GetByID(ctx, reportID)
	if errors.Is(err, domain.ErrNotFound) {
		return BreakingReportAffectedModulesResponse{}, ErrBreakingReportNotFound
	}
	if err != nil {
		return BreakingReportAffectedModulesResponse{}, err
	}
	if svc.dependencyRead == nil {
		return BreakingReportAffectedModulesResponse{}, ErrStorageFailure
	}

	affected, err := svc.dependencyRead.ListAffectedModules(ctx, report.ModuleID)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return BreakingReportAffectedModulesResponse{}, err
	}
	return BreakingReportAffectedModulesResponse{Report: report, AffectedModules: affected}, nil
}
