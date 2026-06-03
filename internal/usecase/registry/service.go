package registry

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"

	"github.com/alryzden/ProtoRadar/internal/domain"
	"github.com/alryzden/ProtoRadar/internal/integration/protoradarevents"
	"github.com/alryzden/ProtoRadar/internal/outbox"
	"github.com/alryzden/ProtoRadar/internal/storage"
)

type Service struct {
	modules        domain.ModuleRepository
	versions       domain.ModuleVersionRepository
	artifacts      domain.ArtifactRepository
	tokens         domain.APITokenRepository
	transactions   domain.RegistryTransactionManager
	outbox         outbox.Writer
	artifactStore  storage.ArtifactStore
	clock          Clock
	ids            IDGenerator
	tokenGenerator TokenGenerator
	options        Options
}

func NewService(
	modules domain.ModuleRepository,
	versions domain.ModuleVersionRepository,
	artifacts domain.ArtifactRepository,
	tokens domain.APITokenRepository,
	transactions domain.RegistryTransactionManager,
	outbox outbox.Writer,
	artifactStore storage.ArtifactStore,
	clock Clock,
	ids IDGenerator,
	tokenGenerator TokenGenerator,
	options Options,
) *Service {
	return &Service{
		modules:        modules,
		versions:       versions,
		artifacts:      artifacts,
		tokens:         tokens,
		transactions:   transactions,
		outbox:         outbox,
		artifactStore:  artifactStore,
		clock:          clock,
		ids:            ids,
		tokenGenerator: tokenGenerator,
		options:        options,
	}
}

func (svc *Service) CreateModule(ctx context.Context, req CreateModuleRequest) (domain.Module, error) {
	name, err := domain.NewModuleName(req.Name)
	if err != nil {
		return domain.Module{}, ErrInvalidModuleName
	}

	now := svc.clock.Now()
	moduleID, err := svc.ids.NewModuleID()
	if err != nil {
		return domain.Module{}, err
	}
	module := domain.Module{
		ID:            moduleID,
		Name:          name,
		Description:   req.Description,
		RepositoryURL: req.RepositoryURL,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	err = svc.transactions.WithinTransaction(ctx, func(txCtx context.Context) error {
		if err := svc.modules.Create(txCtx, module); err != nil {
			if errors.Is(err, domain.ErrDuplicate) {
				return ErrModuleAlreadyExists
			}
			return err
		}

		record, err := protoradarevents.NewModuleCreated(module, now)
		if err != nil {
			return err
		}
		if err := svc.outbox.Create(txCtx, record); err != nil {
			if errors.Is(err, domain.ErrDuplicate) {
				return ErrModuleAlreadyExists
			}
			return err
		}
		return nil
	})
	if err != nil {
		return domain.Module{}, err
	}

	return module, nil
}

func (svc *Service) ListModules(ctx context.Context, limit int, offset int) ([]domain.Module, error) {
	return svc.modules.List(ctx, limit, offset)
}

func (svc *Service) GetModule(ctx context.Context, nameValue string) (domain.Module, error) {
	name, err := domain.NewModuleName(nameValue)
	if err != nil {
		return domain.Module{}, ErrInvalidModuleName
	}
	module, err := svc.modules.GetByName(ctx, name)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.Module{}, ErrModuleNotFound
	}
	return module, err
}

func (svc *Service) PublishModuleVersion(ctx context.Context, req PublishModuleVersionRequest) (domain.ModuleVersion, domain.Artifact, error) {
	name, err := domain.NewModuleName(req.ModuleName)
	if err != nil {
		return domain.ModuleVersion{}, domain.Artifact{}, ErrInvalidModuleName
	}
	versionValue, err := domain.NewVersion(req.Version)
	if err != nil {
		return domain.ModuleVersion{}, domain.Artifact{}, ErrInvalidVersion
	}
	if req.Artifact == nil {
		return domain.ModuleVersion{}, domain.Artifact{}, ErrStorageFailure
	}

	module, err := svc.modules.GetByName(ctx, name)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.ModuleVersion{}, domain.Artifact{}, ErrModuleNotFound
	}
	if err != nil {
		return domain.ModuleVersion{}, domain.Artifact{}, err
	}

	if _, err := svc.versions.GetByModuleAndVersion(ctx, module.ID, versionValue); err == nil {
		return domain.ModuleVersion{}, domain.Artifact{}, ErrModuleVersionAlreadyExists
	} else if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return domain.ModuleVersion{}, domain.Artifact{}, err
	}

	body, checksum, sizeBytes, err := svc.readArtifact(req.Artifact)
	if err != nil {
		return domain.ModuleVersion{}, domain.Artifact{}, err
	}

	key := storage.BuildArtifactKey(name, versionValue, checksum)
	object, err := svc.artifactStore.Put(ctx, key, bytes.NewReader(body), sizeBytes)
	if err != nil {
		return domain.ModuleVersion{}, domain.Artifact{}, fmt.Errorf("%w: %v", ErrStorageFailure, err)
	}

	now := svc.clock.Now()
	moduleVersionID, err := svc.ids.NewModuleVersionID()
	if err != nil {
		return domain.ModuleVersion{}, domain.Artifact{}, err
	}
	artifactID, err := svc.ids.NewArtifactID()
	if err != nil {
		return domain.ModuleVersion{}, domain.Artifact{}, err
	}
	moduleVersion := domain.ModuleVersion{
		ID:        moduleVersionID,
		ModuleID:  module.ID,
		Version:   versionValue,
		Status:    domain.ModuleVersionStatusPublished,
		Digest:    "sha256:" + checksum,
		CreatedAt: now,
	}
	publishedAt := now
	moduleVersion.PublishedAt = &publishedAt

	artifact := domain.Artifact{
		ID:              artifactID,
		ModuleVersionID: moduleVersion.ID,
		StorageKey:      object.Key,
		ChecksumSHA256:  checksum,
		SizeBytes:       object.SizeBytes,
		CreatedAt:       now,
	}

	err = svc.transactions.WithinTransaction(ctx, func(txCtx context.Context) error {
		if err := svc.versions.Create(txCtx, moduleVersion); err != nil {
			if errors.Is(err, domain.ErrDuplicate) {
				return ErrModuleVersionAlreadyExists
			}
			return err
		}
		if err := svc.artifacts.Create(txCtx, artifact); err != nil {
			if errors.Is(err, domain.ErrDuplicate) {
				return ErrModuleVersionAlreadyExists
			}
			return err
		}

		record, err := protoradarevents.NewModuleVersionPublished(module, moduleVersion, artifact, now)
		if err != nil {
			return err
		}
		if err := svc.outbox.Create(txCtx, record); err != nil {
			if errors.Is(err, domain.ErrDuplicate) {
				return ErrModuleVersionAlreadyExists
			}
			return err
		}
		return nil
	})
	if err != nil {
		_ = svc.artifactStore.Delete(ctx, key)
		return domain.ModuleVersion{}, domain.Artifact{}, err
	}

	return moduleVersion, artifact, nil
}

func (svc *Service) ListModuleVersions(ctx context.Context, moduleName string, limit int, offset int) ([]domain.ModuleVersion, error) {
	module, err := svc.GetModule(ctx, moduleName)
	if err != nil {
		return nil, err
	}
	return svc.versions.ListByModule(ctx, module.ID, limit, offset)
}

func (svc *Service) GetModuleVersion(ctx context.Context, moduleName string, versionValue string) (domain.ModuleVersion, error) {
	module, err := svc.GetModule(ctx, moduleName)
	if err != nil {
		return domain.ModuleVersion{}, err
	}
	version, err := domain.NewVersion(versionValue)
	if err != nil {
		return domain.ModuleVersion{}, ErrInvalidVersion
	}

	moduleVersion, err := svc.versions.GetByModuleAndVersion(ctx, module.ID, version)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.ModuleVersion{}, ErrModuleNotFound
	}
	return moduleVersion, err
}

func (svc *Service) DownloadArtifact(ctx context.Context, moduleName string, versionValue string) (storage.ArtifactObject, domain.Artifact, error) {
	moduleVersion, err := svc.GetModuleVersion(ctx, moduleName, versionValue)
	if err != nil {
		return storage.ArtifactObject{}, domain.Artifact{}, err
	}
	artifact, err := svc.artifacts.GetByModuleVersion(ctx, moduleVersion.ID)
	if errors.Is(err, domain.ErrNotFound) {
		return storage.ArtifactObject{}, domain.Artifact{}, ErrModuleNotFound
	}
	if err != nil {
		return storage.ArtifactObject{}, domain.Artifact{}, err
	}

	object, err := svc.artifactStore.Get(ctx, artifact.StorageKey)
	if err != nil {
		return storage.ArtifactObject{}, domain.Artifact{}, fmt.Errorf("%w: %v", ErrStorageFailure, err)
	}
	return object, artifact, nil
}

func (svc *Service) CreateAPIToken(ctx context.Context, req CreateAPITokenRequest) (CreateAPITokenResponse, error) {
	rawToken, err := svc.tokenGenerator.NewToken()
	if err != nil {
		return CreateAPITokenResponse{}, err
	}

	now := svc.clock.Now()
	tokenID, err := svc.ids.NewAPITokenID()
	if err != nil {
		return CreateAPITokenResponse{}, err
	}
	token := domain.APIToken{
		ID:        tokenID,
		Name:      req.Name,
		TokenHash: svc.hashToken(rawToken),
		CreatedAt: now,
		ExpiresAt: req.ExpiresAt,
	}
	if err := svc.tokens.Create(ctx, token); err != nil {
		return CreateAPITokenResponse{}, err
	}

	return CreateAPITokenResponse{
		Token:    token,
		RawToken: rawToken,
	}, nil
}

func (svc *Service) AuthenticateToken(ctx context.Context, rawToken string) (AuthSubject, error) {
	token, err := svc.tokens.GetByHash(ctx, svc.hashToken(rawToken))
	if errors.Is(err, domain.ErrNotFound) {
		return AuthSubject{}, ErrInvalidOrExpiredToken
	}
	if err != nil {
		return AuthSubject{}, err
	}

	now := svc.clock.Now()
	if token.IsExpired(now) {
		return AuthSubject{}, ErrInvalidOrExpiredToken
	}
	_ = svc.tokens.MarkUsed(ctx, token.ID, now)

	return AuthSubject{
		TokenID: token.ID,
		Name:    token.Name,
	}, nil
}

func (svc *Service) readArtifact(reader io.Reader) ([]byte, string, int64, error) {
	limit := svc.options.MaxArtifactSizeBytes
	if limit <= 0 {
		limit = 1
	}

	var buffer bytes.Buffer
	hasher := sha256.New()
	written, err := io.Copy(io.MultiWriter(&buffer, hasher), io.LimitReader(reader, limit+1))
	if err != nil {
		return nil, "", 0, fmt.Errorf("%w: %v", ErrStorageFailure, err)
	}
	if written > limit {
		return nil, "", 0, ErrArtifactTooLarge
	}

	return buffer.Bytes(), hex.EncodeToString(hasher.Sum(nil)), written, nil
}

func (svc *Service) hashToken(rawToken string) string {
	mac := hmac.New(sha256.New, []byte(svc.options.TokenHashSecret))
	mac.Write([]byte(rawToken))
	return hex.EncodeToString(mac.Sum(nil))
}
