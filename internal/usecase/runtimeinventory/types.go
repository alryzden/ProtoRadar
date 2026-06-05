package runtimeinventory

import (
	"strings"
	"time"

	"github.com/alryzden/ProtoRadar/internal/domain"
)

type ReportRuntimeInventoryInput struct {
	ServiceName  string
	Environment  string
	GitCommit    string
	BuildVersion string
	ReportedAt   *time.Time
	Modules      []ReportedModuleInput
}

type ReportedModuleInput struct {
	Module  string
	Version string
}

type RuntimeModuleUsageOutput struct {
	Module        string
	Version       string
	LatestVersion string
	DriftStatus   domain.RuntimeDriftStatus
	DriftReason   string
}

type ReportRuntimeInventoryOutput struct {
	DeploymentID string
	ServiceName  string
	Environment  string
	GitCommit    string
	BuildVersion string
	ReportedAt   time.Time
	Usages       []RuntimeModuleUsageOutput
	DriftCounts  DriftCounts
}

type DriftCounts struct {
	UpToDate          int
	BehindLatest      int
	UnknownVersion    int
	DeprecatedVersion int
}

func (counts *DriftCounts) Add(status domain.RuntimeDriftStatus) {
	switch status {
	case domain.RuntimeDriftStatusUpToDate:
		counts.UpToDate++
	case domain.RuntimeDriftStatusBehindLatest:
		counts.BehindLatest++
	case domain.RuntimeDriftStatusUnknownVersion:
		counts.UnknownVersion++
	case domain.RuntimeDriftStatusDeprecatedVersion:
		counts.DeprecatedVersion++
	}
}

type ModuleVersionReference struct {
	Module  string
	Version string
}

func ParseModuleVersionReference(value string) (ModuleVersionReference, error) {
	reference := strings.TrimSpace(value)
	if reference == "" {
		return ModuleVersionReference{}, ErrInvalidModuleVersionReference
	}
	module, version, ok := strings.Cut(reference, "@")
	if !ok || strings.Contains(version, "@") {
		return ModuleVersionReference{}, ErrInvalidModuleVersionReference
	}
	module = strings.TrimSpace(module)
	version = strings.TrimSpace(version)
	if module == "" || version == "" {
		return ModuleVersionReference{}, ErrInvalidModuleVersionReference
	}
	return ModuleVersionReference{Module: module, Version: version}, nil
}
