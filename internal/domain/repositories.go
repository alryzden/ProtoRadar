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

type APITokenRepository interface {
	Create(ctx context.Context, token APIToken) error
	GetByID(ctx context.Context, id APITokenID) (APIToken, error)
	GetByHash(ctx context.Context, tokenHash string) (APIToken, error)
	MarkUsed(ctx context.Context, id APITokenID, usedAt time.Time) error
}

type RegistryTransactionManager interface {
	WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error
}
