package domain

import (
	"strings"
	"time"
)

const (
	MaxRuntimeServiceNameLength  = 128
	MaxRuntimeEnvironmentLength  = 64
	MaxRuntimeGitCommitLength    = 128
	MaxRuntimeBuildVersionLength = 128
)

type RuntimeServiceName string

func NewRuntimeServiceName(value string) (RuntimeServiceName, error) {
	name := strings.TrimSpace(value)
	if name == "" || len(name) > MaxRuntimeServiceNameLength {
		return "", ErrInvalidRuntimeServiceName
	}
	for _, ch := range name {
		if isRuntimeNameChar(ch) {
			continue
		}
		return "", ErrInvalidRuntimeServiceName
	}
	return RuntimeServiceName(name), nil
}

func (name RuntimeServiceName) String() string {
	return string(name)
}

type RuntimeEnvironment string

func NewRuntimeEnvironment(value string) (RuntimeEnvironment, error) {
	environment := strings.TrimSpace(value)
	if environment == "" || len(environment) > MaxRuntimeEnvironmentLength {
		return "", ErrInvalidRuntimeEnvironment
	}
	for _, ch := range environment {
		if isRuntimeNameChar(ch) {
			continue
		}
		return "", ErrInvalidRuntimeEnvironment
	}
	return RuntimeEnvironment(environment), nil
}

func (environment RuntimeEnvironment) String() string {
	return string(environment)
}

func ValidateRuntimeGitCommit(value string) error {
	commit := strings.TrimSpace(value)
	if commit == "" || len(commit) > MaxRuntimeGitCommitLength || strings.ContainsAny(commit, " \t\n\r") {
		return ErrInvalidRuntimeGitCommit
	}
	return nil
}

func ValidateRuntimeBuildVersion(value string) error {
	version := strings.TrimSpace(value)
	if version == "" || len(version) > MaxRuntimeBuildVersionLength || strings.ContainsAny(version, "\t\n\r") {
		return ErrInvalidRuntimeBuildVersion
	}
	return nil
}

func isRuntimeNameChar(ch rune) bool {
	if ch >= 'a' && ch <= 'z' {
		return true
	}
	if ch >= 'A' && ch <= 'Z' {
		return true
	}
	if ch >= '0' && ch <= '9' {
		return true
	}
	return ch == '-' || ch == '_' || ch == '/' || ch == '.'
}

type RuntimeDriftStatus string

const (
	RuntimeDriftStatusUpToDate          RuntimeDriftStatus = "up_to_date"
	RuntimeDriftStatusBehindLatest      RuntimeDriftStatus = "behind_latest"
	RuntimeDriftStatusUnknownVersion    RuntimeDriftStatus = "unknown_version"
	RuntimeDriftStatusDeprecatedVersion RuntimeDriftStatus = "deprecated_version"
)

func NewRuntimeDriftStatus(value string) (RuntimeDriftStatus, error) {
	status := RuntimeDriftStatus(strings.TrimSpace(value))
	if !status.IsValid() {
		return "", ErrInvalidRuntimeDriftStatus
	}
	return status, nil
}

func (status RuntimeDriftStatus) String() string {
	return string(status)
}

func (status RuntimeDriftStatus) IsValid() bool {
	switch status {
	case RuntimeDriftStatusUpToDate,
		RuntimeDriftStatusBehindLatest,
		RuntimeDriftStatusUnknownVersion,
		RuntimeDriftStatusDeprecatedVersion:
		return true
	default:
		return false
	}
}

type RuntimeImpactStatus string

const RuntimeImpactStatusPotentiallyAffectedByBreakingChange RuntimeImpactStatus = "potentially_affected_by_breaking_change"

func (status RuntimeImpactStatus) String() string {
	return string(status)
}

type RuntimeService struct {
	ID        RuntimeServiceID
	Name      RuntimeServiceName
	CreatedAt time.Time
	UpdatedAt time.Time
}

type RuntimeDeployment struct {
	ID           RuntimeDeploymentID
	ServiceID    RuntimeServiceID
	ServiceName  RuntimeServiceName
	Environment  RuntimeEnvironment
	GitCommit    string
	BuildVersion string
	ReportedAt   time.Time
	CreatedAt    time.Time
}

type RuntimeModuleUsage struct {
	ID              RuntimeModuleUsageID
	DeploymentID    RuntimeDeploymentID
	ModuleID        *ModuleID
	ModuleName      ModuleName
	ModuleVersionID *ModuleVersionID
	Version         Version
	LatestVersion   *Version
	DriftStatus     RuntimeDriftStatus
	DriftReason     string
	CreatedAt       time.Time
}

type RuntimeServiceSummary struct {
	Service             RuntimeService
	Environments        []RuntimeEnvironment
	EnvironmentCount    int
	DeploymentCount     int
	UpToDateCount       int
	BehindLatestCount   int
	UnknownVersionCount int
	DeprecatedCount     int
	LatestReportedAt    *time.Time
}

type RuntimeServiceDetails struct {
	Service     RuntimeService
	Deployments []RuntimeDeployment
	Usages      []RuntimeModuleUsage
}

type RuntimeEnvironmentInventory struct {
	Environment RuntimeEnvironment
	Deployments []RuntimeDeployment
	Usages      []RuntimeModuleUsage
}

type ModuleRuntimeUsage struct {
	ServiceName  RuntimeServiceName
	Environment  RuntimeEnvironment
	DeploymentID RuntimeDeploymentID
	ModuleName   ModuleName
	Version      Version
	GitCommit    string
	BuildVersion string
	ReportedAt   time.Time
	DriftStatus  RuntimeDriftStatus
	DriftReason  string
}

type RuntimeImpact struct {
	ServiceName  RuntimeServiceName
	Environment  RuntimeEnvironment
	UsedModule   ModuleName
	UsedVersion  Version
	GitCommit    string
	BuildVersion string
	ReportedAt   time.Time
	ImpactStatus RuntimeImpactStatus
	Reason       string
	DriftStatus  RuntimeDriftStatus
	DriftReason  string
}
