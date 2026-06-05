package domain

import "strings"

type ModuleID string

func NewModuleID(value string) ModuleID {
	return ModuleID(strings.TrimSpace(value))
}

func (id ModuleID) String() string {
	return string(id)
}

type ModuleGitLabProjectID string

func NewModuleGitLabProjectID(value string) ModuleGitLabProjectID {
	return ModuleGitLabProjectID(strings.TrimSpace(value))
}

func (id ModuleGitLabProjectID) String() string {
	return string(id)
}

type ModuleVersionID string

func NewModuleVersionID(value string) ModuleVersionID {
	return ModuleVersionID(strings.TrimSpace(value))
}

func (id ModuleVersionID) String() string {
	return string(id)
}

type ArtifactID string

func NewArtifactID(value string) ArtifactID {
	return ArtifactID(strings.TrimSpace(value))
}

func (id ArtifactID) String() string {
	return string(id)
}

type APITokenID string

func NewAPITokenID(value string) APITokenID {
	return APITokenID(strings.TrimSpace(value))
}

func (id APITokenID) String() string {
	return string(id)
}

type BreakingReportID string

func NewBreakingReportID(value string) BreakingReportID {
	return BreakingReportID(strings.TrimSpace(value))
}

func (id BreakingReportID) String() string {
	return string(id)
}

type BreakingChangeID string

func NewBreakingChangeID(value string) BreakingChangeID {
	return BreakingChangeID(strings.TrimSpace(value))
}

func (id BreakingChangeID) String() string {
	return string(id)
}

type ModuleDependencyID string

func NewModuleDependencyID(value string) ModuleDependencyID {
	return ModuleDependencyID(strings.TrimSpace(value))
}

func (id ModuleDependencyID) String() string {
	return string(id)
}

type UnresolvedProtoDependencyID string

func NewUnresolvedProtoDependencyID(value string) UnresolvedProtoDependencyID {
	return UnresolvedProtoDependencyID(strings.TrimSpace(value))
}

func (id UnresolvedProtoDependencyID) String() string {
	return string(id)
}

type RuntimeServiceID string

func NewRuntimeServiceID(value string) RuntimeServiceID {
	return RuntimeServiceID(strings.TrimSpace(value))
}

func (id RuntimeServiceID) String() string {
	return string(id)
}

type RuntimeDeploymentID string

func NewRuntimeDeploymentID(value string) RuntimeDeploymentID {
	return RuntimeDeploymentID(strings.TrimSpace(value))
}

func (id RuntimeDeploymentID) String() string {
	return string(id)
}

type RuntimeModuleUsageID string

func NewRuntimeModuleUsageID(value string) RuntimeModuleUsageID {
	return RuntimeModuleUsageID(strings.TrimSpace(value))
}

func (id RuntimeModuleUsageID) String() string {
	return string(id)
}
