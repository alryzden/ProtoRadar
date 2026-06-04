package domain

import "time"

type DependencySource string

const (
	DependencySourceImport          DependencySource = "import"
	DependencySourceTypeReference   DependencySource = "type_reference"
	DependencySourceMethodReference DependencySource = "method_reference"
	DependencySourcePackageFallback DependencySource = "package_fallback"
)

func (source DependencySource) String() string {
	return string(source)
}

func (source DependencySource) IsValid() bool {
	switch source {
	case DependencySourceImport,
		DependencySourceTypeReference,
		DependencySourceMethodReference,
		DependencySourcePackageFallback:
		return true
	default:
		return false
	}
}

type DependencyResolutionReason string

const (
	DependencyResolutionReasonImportPath      DependencyResolutionReason = "import_path"
	DependencyResolutionReasonSymbol          DependencyResolutionReason = "symbol"
	DependencyResolutionReasonPackageFallback DependencyResolutionReason = "package_fallback"
)

func (reason DependencyResolutionReason) String() string {
	return string(reason)
}

type UnresolvedDependencyReason string

const (
	UnresolvedDependencyReasonProviderNotFound  UnresolvedDependencyReason = "provider_not_found"
	UnresolvedDependencyReasonAmbiguousProvider UnresolvedDependencyReason = "ambiguous_provider"
	UnresolvedDependencyReasonNotFound          UnresolvedDependencyReason = UnresolvedDependencyReasonProviderNotFound
	UnresolvedDependencyReasonAmbiguous         UnresolvedDependencyReason = UnresolvedDependencyReasonAmbiguousProvider
)

func (reason UnresolvedDependencyReason) String() string {
	return string(reason)
}

type ModuleDependency struct {
	ID                      ModuleDependencyID
	ConsumerModuleID        ModuleID
	ConsumerModuleName      ModuleName
	ConsumerModuleVersionID ModuleVersionID
	ConsumerVersion         Version
	ProviderModuleID        ModuleID
	ProviderModuleName      ModuleName
	ProviderModuleVersionID ModuleVersionID
	ProviderVersion         Version
	Source                  DependencySource
	Reason                  DependencyResolutionReason
	ImportPath              string
	ReferencedPackage       string
	ReferencedSymbol        string
	CreatedAt               time.Time
}

type UnresolvedProtoDependency struct {
	ID                UnresolvedProtoDependencyID
	ModuleID          ModuleID
	ModuleName        ModuleName
	ModuleVersionID   ModuleVersionID
	Version           Version
	Source            DependencySource
	ImportPath        string
	ReferencedPackage string
	ReferencedSymbol  string
	Reason            UnresolvedDependencyReason
	CreatedAt         time.Time
}

type DependencyGraph struct {
	Module     ModuleName
	Version    Version
	Upstream   []ModuleDependency
	Downstream []ModuleDependency
	Unresolved []UnresolvedProtoDependency
}

type AffectedModule struct {
	ModuleName        ModuleName
	LatestVersion     Version
	DependencySources []DependencySource
	Reasons           []DependencyResolutionReason
}
