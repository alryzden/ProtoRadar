package dependencygraph

import (
	"context"
	"strings"
	"time"

	"github.com/alryzden/ProtoRadar/internal/domain"
	"github.com/alryzden/ProtoRadar/internal/integration/protoradarevents"
	"github.com/alryzden/ProtoRadar/internal/outbox"
	"github.com/alryzden/ProtoRadar/internal/usecase/registry"
)

type Clock interface {
	Now() time.Time
}

type IDGenerator interface {
	NewModuleDependencyID() (domain.ModuleDependencyID, error)
	NewUnresolvedProtoDependencyID() (domain.UnresolvedProtoDependencyID, error)
}

type Service struct {
	modules      domain.ModuleRepository
	versions     domain.ModuleVersionRepository
	metadata     domain.DescriptorMetadataRepository
	dependencies domain.ModuleDependencyRepository
	providers    registry.DependencyProviderIndexRepository
	transactions domain.RegistryTransactionManager
	outbox       outbox.Writer
	clock        Clock
	ids          IDGenerator
}

type RebuildModuleVersionDependenciesInput struct {
	ModuleID        domain.ModuleID
	ModuleName      string
	ModuleVersionID domain.ModuleVersionID
	Version         string
}

type RebuildModuleVersionDependenciesOutput struct {
	Module       domain.Module
	Version      domain.ModuleVersion
	Dependencies []domain.ModuleDependency
	Unresolved   []domain.UnresolvedProtoDependency
}

func NewService(
	modules domain.ModuleRepository,
	versions domain.ModuleVersionRepository,
	metadata domain.DescriptorMetadataRepository,
	dependencies domain.ModuleDependencyRepository,
	providers registry.DependencyProviderIndexRepository,
	transactions domain.RegistryTransactionManager,
	outbox outbox.Writer,
	clock Clock,
	ids IDGenerator,
) *Service {
	return &Service{
		modules:      modules,
		versions:     versions,
		metadata:     metadata,
		dependencies: dependencies,
		providers:    providers,
		transactions: transactions,
		outbox:       outbox,
		clock:        clock,
		ids:          ids,
	}
}

func (svc *Service) RebuildModuleVersionDependencies(ctx context.Context, input RebuildModuleVersionDependenciesInput) (RebuildModuleVersionDependenciesOutput, error) {
	module, err := svc.resolveModule(ctx, input)
	if err != nil {
		return RebuildModuleVersionDependenciesOutput{}, err
	}
	moduleVersion, err := svc.resolveModuleVersion(ctx, module.ID, input)
	if err != nil {
		return RebuildModuleVersionDependenciesOutput{}, err
	}
	metadata, err := svc.metadata.GetByModuleVersion(ctx, moduleVersion.ID)
	if err != nil {
		return RebuildModuleVersionDependenciesOutput{}, err
	}
	providerIndex, err := svc.buildProviderIndex(ctx, module.ID)
	if err != nil {
		return RebuildModuleVersionDependenciesOutput{}, err
	}

	dependencies, unresolved, err := svc.resolveDependencies(module, moduleVersion, metadata, providerIndex)
	if err != nil {
		return RebuildModuleVersionDependenciesOutput{}, err
	}

	err = svc.transactions.WithinTransaction(ctx, func(txCtx context.Context) error {
		return svc.storeResolvedDependencies(txCtx, module, moduleVersion, dependencies, unresolved)
	})
	if err != nil {
		return RebuildModuleVersionDependenciesOutput{}, err
	}

	return RebuildModuleVersionDependenciesOutput{
		Module:       module,
		Version:      moduleVersion,
		Dependencies: dependencies,
		Unresolved:   unresolved,
	}, nil
}

func (svc *Service) RebuildPublishedModuleVersionDependencies(ctx context.Context, input registry.PublishedModuleVersionDependencyRebuildInput) (registry.PublishedModuleVersionDependencyRebuildOutput, error) {
	providerIndex, err := svc.buildProviderIndex(ctx, input.Module.ID)
	if err != nil {
		return registry.PublishedModuleVersionDependencyRebuildOutput{}, err
	}

	dependencies, unresolved, err := svc.resolveDependencies(input.Module, input.Version, input.Metadata, providerIndex)
	if err != nil {
		return registry.PublishedModuleVersionDependencyRebuildOutput{}, err
	}

	if err := svc.storeResolvedDependencies(ctx, input.Module, input.Version, dependencies, unresolved); err != nil {
		return registry.PublishedModuleVersionDependencyRebuildOutput{}, err
	}

	return registry.PublishedModuleVersionDependencyRebuildOutput{
		Dependencies: dependencies,
		Unresolved:   unresolved,
	}, nil
}

func (svc *Service) storeResolvedDependencies(ctx context.Context, module domain.Module, moduleVersion domain.ModuleVersion, dependencies []domain.ModuleDependency, unresolved []domain.UnresolvedProtoDependency) error {
	if err := svc.dependencies.ReplaceByConsumerModuleVersion(ctx, moduleVersion.ID, dependencies, unresolved); err != nil {
		return err
	}
	record, err := protoradarevents.NewModuleDependenciesUpdated(protoradarevents.ModuleDependenciesUpdated{
		Module:                    module,
		Version:                   moduleVersion,
		DependencyCount:           len(dependencies),
		UnresolvedDependencyCount: len(unresolved),
		OccurredAt:                svc.clock.Now(),
	})
	if err != nil {
		return err
	}
	return svc.outbox.Create(ctx, record)
}

func (svc *Service) resolveModule(ctx context.Context, input RebuildModuleVersionDependenciesInput) (domain.Module, error) {
	if input.ModuleID != "" {
		return svc.modules.GetByID(ctx, input.ModuleID)
	}
	name, err := domain.NewModuleName(input.ModuleName)
	if err != nil {
		return domain.Module{}, err
	}
	return svc.modules.GetByName(ctx, name)
}

func (svc *Service) resolveModuleVersion(ctx context.Context, moduleID domain.ModuleID, input RebuildModuleVersionDependenciesInput) (domain.ModuleVersion, error) {
	if input.ModuleVersionID != "" {
		version, err := svc.versions.GetByID(ctx, input.ModuleVersionID)
		if err != nil {
			return domain.ModuleVersion{}, err
		}
		if version.ModuleID != moduleID {
			return domain.ModuleVersion{}, domain.ErrNotFound
		}
		return version, nil
	}
	versionValue, err := domain.NewVersion(input.Version)
	if err != nil {
		return domain.ModuleVersion{}, err
	}
	return svc.versions.GetByModuleAndVersion(ctx, moduleID, versionValue)
}

type providerIndex struct {
	files       map[string][]registry.ProviderFile
	symbols     map[string][]registry.ProviderSymbol
	selfFiles   map[string]struct{}
	selfSymbols map[string]struct{}
}

func (svc *Service) buildProviderIndex(ctx context.Context, consumerModuleID domain.ModuleID) (providerIndex, error) {
	latest, err := svc.providers.ListLatestPublishedModuleVersions(ctx)
	if err != nil {
		return providerIndex{}, err
	}
	ids := make([]domain.ModuleVersionID, 0, len(latest))
	for _, version := range latest {
		if version.ModuleID == consumerModuleID {
			continue
		}
		ids = append(ids, version.ID)
	}
	files, err := svc.providers.ListProviderFiles(ctx, ids)
	if err != nil {
		return providerIndex{}, err
	}
	symbols, err := svc.providers.ListProviderSymbols(ctx, ids)
	if err != nil {
		return providerIndex{}, err
	}

	index := providerIndex{
		files:       map[string][]registry.ProviderFile{},
		symbols:     map[string][]registry.ProviderSymbol{},
		selfFiles:   map[string]struct{}{},
		selfSymbols: map[string]struct{}{},
	}
	for _, file := range files {
		index.files[file.Path] = append(index.files[file.Path], file)
	}
	for _, symbol := range symbols {
		index.symbols[normalizeSymbol(symbol.FullName)] = append(index.symbols[normalizeSymbol(symbol.FullName)], symbol)
	}
	return index, nil
}

func (svc *Service) resolveDependencies(module domain.Module, moduleVersion domain.ModuleVersion, metadata domain.DescriptorMetadata, index providerIndex) ([]domain.ModuleDependency, []domain.UnresolvedProtoDependency, error) {
	dependencies := make([]domain.ModuleDependency, 0)
	unresolved := make([]domain.UnresolvedProtoDependency, 0)
	seen := map[string]struct{}{}
	now := svc.clock.Now()

	addDependency := func(provider providerRef, source domain.DependencySource, reason domain.DependencyResolutionReason, importPath string, referencedPackage string, referencedSymbol string) error {
		if provider.ModuleID == module.ID {
			return nil
		}
		key := dependencyKey(moduleVersion.ID, provider.ModuleVersionID, source, importPath, referencedSymbol)
		if _, exists := seen[key]; exists {
			return nil
		}
		seen[key] = struct{}{}
		id, err := svc.ids.NewModuleDependencyID()
		if err != nil {
			return err
		}
		dependencies = append(dependencies, domain.ModuleDependency{
			ID:                      id,
			ConsumerModuleID:        module.ID,
			ConsumerModuleName:      module.Name,
			ConsumerModuleVersionID: moduleVersion.ID,
			ConsumerVersion:         moduleVersion.Version,
			ProviderModuleID:        provider.ModuleID,
			ProviderModuleName:      provider.ModuleName,
			ProviderModuleVersionID: provider.ModuleVersionID,
			ProviderVersion:         provider.Version,
			Source:                  source,
			Reason:                  reason,
			ImportPath:              importPath,
			ReferencedPackage:       referencedPackage,
			ReferencedSymbol:        referencedSymbol,
			CreatedAt:               now,
		})
		return nil
	}

	addUnresolved := func(source domain.DependencySource, importPath string, referencedPackage string, referencedSymbol string, reason domain.UnresolvedDependencyReason) error {
		id, err := svc.ids.NewUnresolvedProtoDependencyID()
		if err != nil {
			return err
		}
		unresolved = append(unresolved, domain.UnresolvedProtoDependency{
			ID:                id,
			ModuleID:          module.ID,
			ModuleName:        module.Name,
			ModuleVersionID:   moduleVersion.ID,
			Version:           moduleVersion.Version,
			Source:            source,
			ImportPath:        importPath,
			ReferencedPackage: referencedPackage,
			ReferencedSymbol:  referencedSymbol,
			Reason:            reason,
			CreatedAt:         now,
		})
		return nil
	}

	for _, file := range metadata.Files {
		index.selfFiles[file.Path] = struct{}{}
		for _, service := range file.Services {
			index.selfSymbols[normalizeSymbol(service.FullName)] = struct{}{}
		}
		for _, message := range file.Messages {
			addMessageSymbols(index.selfSymbols, message)
		}
		for _, enum := range file.Enums {
			index.selfSymbols[normalizeSymbol(enum.FullName)] = struct{}{}
		}
	}

	for _, file := range metadata.Files {
		for _, protoImport := range file.Imports {
			if registry.IsWellKnownProtoImport(protoImport.Path) {
				continue
			}
			if _, self := index.selfFiles[protoImport.Path]; self {
				continue
			}
			matches := index.files[protoImport.Path]
			switch len(matches) {
			case 0:
				if err := addUnresolved(domain.DependencySourceImport, protoImport.Path, "", "", domain.UnresolvedDependencyReasonProviderNotFound); err != nil {
					return nil, nil, err
				}
			case 1:
				provider := providerFromFile(matches[0])
				if err := addDependency(provider, domain.DependencySourceImport, domain.DependencyResolutionReasonImportPath, protoImport.Path, "", ""); err != nil {
					return nil, nil, err
				}
			default:
				if err := addUnresolved(domain.DependencySourceImport, protoImport.Path, "", "", domain.UnresolvedDependencyReasonAmbiguousProvider); err != nil {
					return nil, nil, err
				}
			}
		}

		for _, message := range file.Messages {
			if err := svc.resolveMessageReferences(message, index, addDependency, addUnresolved); err != nil {
				return nil, nil, err
			}
		}
		for _, service := range file.Services {
			for _, method := range service.Methods {
				if err := svc.resolveSymbolReference(index, addDependency, addUnresolved, domain.DependencySourceMethodReference, method.InputType); err != nil {
					return nil, nil, err
				}
				if err := svc.resolveSymbolReference(index, addDependency, addUnresolved, domain.DependencySourceMethodReference, method.OutputType); err != nil {
					return nil, nil, err
				}
			}
		}
	}

	return dependencies, unresolved, nil
}

type addDependencyFunc func(provider providerRef, source domain.DependencySource, reason domain.DependencyResolutionReason, importPath string, referencedPackage string, referencedSymbol string) error

type addUnresolvedFunc func(source domain.DependencySource, importPath string, referencedPackage string, referencedSymbol string, reason domain.UnresolvedDependencyReason) error

func (svc *Service) resolveMessageReferences(message domain.ProtoMessage, index providerIndex, addDependency addDependencyFunc, addUnresolved addUnresolvedFunc) error {
	for _, field := range message.Fields {
		if strings.TrimSpace(field.TypeName) == "" {
			continue
		}
		if err := svc.resolveSymbolReference(index, addDependency, addUnresolved, domain.DependencySourceTypeReference, field.TypeName); err != nil {
			return err
		}
	}
	for _, nested := range message.Messages {
		if err := svc.resolveMessageReferences(nested, index, addDependency, addUnresolved); err != nil {
			return err
		}
	}
	return nil
}

func (svc *Service) resolveSymbolReference(index providerIndex, addDependency addDependencyFunc, addUnresolved addUnresolvedFunc, source domain.DependencySource, symbol string) error {
	normalized := normalizeSymbol(symbol)
	if normalized == "" || registry.IsWellKnownProtoSymbol(normalized) {
		return nil
	}
	if _, self := index.selfSymbols[normalized]; self {
		return nil
	}
	matches := index.symbols[normalized]
	referencedPackage := packageFromSymbol(normalized)
	switch len(matches) {
	case 0:
		return addUnresolved(source, "", referencedPackage, normalized, domain.UnresolvedDependencyReasonProviderNotFound)
	case 1:
		return addDependency(providerFromSymbol(matches[0]), source, domain.DependencyResolutionReasonSymbol, "", referencedPackage, normalized)
	default:
		return addUnresolved(source, "", referencedPackage, normalized, domain.UnresolvedDependencyReasonAmbiguousProvider)
	}
}

type providerRef struct {
	ModuleID        domain.ModuleID
	ModuleName      domain.ModuleName
	ModuleVersionID domain.ModuleVersionID
	Version         domain.Version
}

func providerFromFile(file registry.ProviderFile) providerRef {
	return providerRef{
		ModuleID:        file.ModuleID,
		ModuleName:      file.ModuleName,
		ModuleVersionID: file.ModuleVersionID,
		Version:         file.Version,
	}
}

func providerFromSymbol(symbol registry.ProviderSymbol) providerRef {
	return providerRef{
		ModuleID:        symbol.ModuleID,
		ModuleName:      symbol.ModuleName,
		ModuleVersionID: symbol.ModuleVersionID,
		Version:         symbol.Version,
	}
}

func normalizeSymbol(symbol string) string {
	return strings.TrimPrefix(strings.TrimSpace(symbol), ".")
}

func addMessageSymbols(symbols map[string]struct{}, message domain.ProtoMessage) {
	symbols[normalizeSymbol(message.FullName)] = struct{}{}
	for _, nested := range message.Messages {
		addMessageSymbols(symbols, nested)
	}
	for _, enum := range message.Enums {
		symbols[normalizeSymbol(enum.FullName)] = struct{}{}
	}
}

func packageFromSymbol(symbol string) string {
	symbol = normalizeSymbol(symbol)
	index := strings.LastIndex(symbol, ".")
	if index <= 0 {
		return ""
	}
	return symbol[:index]
}

func dependencyKey(consumerVersionID domain.ModuleVersionID, providerVersionID domain.ModuleVersionID, source domain.DependencySource, importPath string, referencedSymbol string) string {
	return consumerVersionID.String() + "\x00" + providerVersionID.String() + "\x00" + source.String() + "\x00" + importPath + "\x00" + referencedSymbol
}
