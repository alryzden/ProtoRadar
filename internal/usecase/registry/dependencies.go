package registry

import (
	"context"
	"strings"

	"github.com/alryzden/ProtoRadar/internal/domain"
)

type DependencyProviderIndexRepository interface {
	ListLatestPublishedModuleVersions(ctx context.Context) ([]domain.ModuleVersion, error)
	ListProviderFiles(ctx context.Context, moduleVersionIDs []domain.ModuleVersionID) ([]ProviderFile, error)
	ListProviderSymbols(ctx context.Context, moduleVersionIDs []domain.ModuleVersionID) ([]ProviderSymbol, error)
}

type DependencyCandidate struct {
	Source            domain.DependencySource
	ImportPath        string
	ReferencedPackage string
	ReferencedSymbol  string
}

type ProviderFile struct {
	Path            string
	ModuleID        domain.ModuleID
	ModuleName      domain.ModuleName
	ModuleVersionID domain.ModuleVersionID
	Version         domain.Version
}

type ProviderSymbol struct {
	FullName        string
	PackageName     string
	ModuleID        domain.ModuleID
	ModuleName      domain.ModuleName
	ModuleVersionID domain.ModuleVersionID
	Version         domain.Version
}

type ResolveResult struct {
	Dependencies []domain.ModuleDependency
	Unresolved   []domain.UnresolvedProtoDependency
}

type ModuleVersionDependencyRebuilder interface {
	RebuildPublishedModuleVersionDependencies(ctx context.Context, input PublishedModuleVersionDependencyRebuildInput) (PublishedModuleVersionDependencyRebuildOutput, error)
}

type PublishedModuleVersionDependencyRebuildInput struct {
	Module   domain.Module
	Version  domain.ModuleVersion
	Metadata domain.DescriptorMetadata
}

type PublishedModuleVersionDependencyRebuildOutput struct {
	Dependencies []domain.ModuleDependency
	Unresolved   []domain.UnresolvedProtoDependency
}

type RebuildModuleVersionDependenciesInput struct {
	Module          domain.Module
	Version         domain.ModuleVersion
	Metadata        domain.DescriptorMetadata
	ProviderFiles   []ProviderFile
	ProviderSymbols []ProviderSymbol
}

type RebuildModuleVersionDependenciesOutput struct {
	Dependencies []domain.ModuleDependency
	Unresolved   []domain.UnresolvedProtoDependency
}

func IsWellKnownProtoImport(path string) bool {
	return strings.HasPrefix(strings.TrimSpace(path), "google/protobuf/")
}

func IsWellKnownProtoSymbol(symbol string) bool {
	value := strings.TrimPrefix(strings.TrimSpace(symbol), ".")
	return strings.HasPrefix(value, "google.protobuf.")
}
