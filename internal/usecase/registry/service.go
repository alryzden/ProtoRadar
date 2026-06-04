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
	"os"
	"path/filepath"
	"strings"

	"github.com/alryzden/ProtoRadar/internal/domain"
	"github.com/alryzden/ProtoRadar/internal/format/breaking"
	"github.com/alryzden/ProtoRadar/internal/integration/protoradarevents"
	"github.com/alryzden/ProtoRadar/internal/outbox"
	"github.com/alryzden/ProtoRadar/internal/storage"
	"github.com/alryzden/ProtoRadar/internal/workspace"
)

const maxTargetRefLength = 256

type Service struct {
	modules        domain.ModuleRepository
	gitLabProjects domain.ModuleGitLabProjectRepository
	versions       domain.ModuleVersionRepository
	artifacts      domain.ModuleVersionArtifactRepository
	bufConfigs     domain.BufConfigRepository
	metadata       domain.DescriptorMetadataRepository
	reports        domain.BreakingReportRepository
	tokens         domain.APITokenRepository
	dependencies   ModuleVersionDependencyRebuilder
	dependencyRead domain.ModuleDependencyRepository
	transactions   domain.RegistryTransactionManager
	outbox         outbox.Writer
	artifactStore  storage.ArtifactStore
	bufWorkflow    BufWorkflow
	bufBreaking    BufBreakingChecker
	clock          Clock
	ids            IDGenerator
	tokenGenerator TokenGenerator
	options        Options
}

func NewService(
	modules domain.ModuleRepository,
	gitLabProjects domain.ModuleGitLabProjectRepository,
	versions domain.ModuleVersionRepository,
	artifacts domain.ModuleVersionArtifactRepository,
	bufConfigs domain.BufConfigRepository,
	metadata domain.DescriptorMetadataRepository,
	reports domain.BreakingReportRepository,
	tokens domain.APITokenRepository,
	dependencies ModuleVersionDependencyRebuilder,
	dependencyRead domain.ModuleDependencyRepository,
	transactions domain.RegistryTransactionManager,
	outbox outbox.Writer,
	artifactStore storage.ArtifactStore,
	bufWorkflow BufWorkflow,
	bufBreaking BufBreakingChecker,
	clock Clock,
	ids IDGenerator,
	tokenGenerator TokenGenerator,
	options Options,
) *Service {
	return &Service{
		modules:        modules,
		gitLabProjects: gitLabProjects,
		versions:       versions,
		artifacts:      artifacts,
		bufConfigs:     bufConfigs,
		metadata:       metadata,
		reports:        reports,
		tokens:         tokens,
		dependencies:   dependencies,
		dependencyRead: dependencyRead,
		transactions:   transactions,
		outbox:         outbox,
		artifactStore:  artifactStore,
		bufWorkflow:    bufWorkflow,
		bufBreaking:    bufBreaking,
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

func (svc *Service) LinkModuleGitLabProject(ctx context.Context, input LinkModuleGitLabProjectInput) (LinkModuleGitLabProjectOutput, error) {
	name, err := domain.NewModuleName(input.ModuleName)
	if err != nil {
		return LinkModuleGitLabProjectOutput{}, ErrInvalidModuleName
	}
	if svc.gitLabProjects == nil {
		return LinkModuleGitLabProjectOutput{}, ErrStorageFailure
	}

	inputMapping := domain.ModuleGitLabProject{
		ModuleName:        name,
		GitLabBaseURL:     input.GitLabBaseURL,
		GitLabProjectID:   input.GitLabProjectID,
		GitLabProjectPath: input.GitLabProjectPath,
	}
	normalizedInput, err := inputMapping.Normalized()
	if err != nil {
		return LinkModuleGitLabProjectOutput{}, mapGitLabMappingValidationError(err)
	}

	module, err := svc.modules.GetByName(ctx, name)
	if errors.Is(err, domain.ErrNotFound) {
		return LinkModuleGitLabProjectOutput{}, ErrModuleNotFound
	}
	if err != nil {
		return LinkModuleGitLabProjectOutput{}, err
	}

	now := svc.clock.Now()
	mappingID, err := svc.ids.NewModuleGitLabProjectID()
	if err != nil {
		return LinkModuleGitLabProjectOutput{}, err
	}
	mapping := domain.ModuleGitLabProject{
		ID:                mappingID,
		ModuleID:          module.ID,
		ModuleName:        module.Name,
		GitLabBaseURL:     normalizedInput.GitLabBaseURL,
		GitLabProjectID:   normalizedInput.GitLabProjectID,
		GitLabProjectPath: normalizedInput.GitLabProjectPath,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	normalized, err := mapping.Normalized()
	if err != nil {
		return LinkModuleGitLabProjectOutput{}, mapGitLabMappingValidationError(err)
	}

	var linked domain.ModuleGitLabProject
	err = svc.transactions.WithinTransaction(ctx, func(txCtx context.Context) error {
		if err := svc.gitLabProjects.Upsert(txCtx, normalized); err != nil {
			if errors.Is(err, domain.ErrDuplicate) {
				return ErrGitLabProjectAlreadyLinked
			}
			return err
		}
		stored, err := svc.gitLabProjects.GetByModuleID(txCtx, module.ID)
		if err != nil {
			return err
		}
		record, err := protoradarevents.NewModuleGitLabProjectLinked(stored, now)
		if err != nil {
			return err
		}
		if err := svc.outbox.Create(txCtx, record); err != nil {
			if errors.Is(err, domain.ErrDuplicate) {
				return ErrGitLabProjectAlreadyLinked
			}
			return err
		}
		linked = stored
		return nil
	})
	if err != nil {
		return LinkModuleGitLabProjectOutput{}, err
	}

	return LinkModuleGitLabProjectOutput{Mapping: linked}, nil
}

func (svc *Service) GetModuleGitLabProject(ctx context.Context, moduleName string) (GetModuleGitLabProjectOutput, error) {
	module, err := svc.GetModule(ctx, moduleName)
	if err != nil {
		return GetModuleGitLabProjectOutput{}, err
	}
	if svc.gitLabProjects == nil {
		return GetModuleGitLabProjectOutput{}, ErrStorageFailure
	}

	mapping, err := svc.gitLabProjects.GetByModuleID(ctx, module.ID)
	if errors.Is(err, domain.ErrNotFound) {
		return GetModuleGitLabProjectOutput{}, ErrModuleGitLabProjectNotFound
	}
	if err != nil {
		return GetModuleGitLabProjectOutput{}, err
	}
	return GetModuleGitLabProjectOutput{Mapping: mapping}, nil
}

func mapGitLabMappingValidationError(err error) error {
	switch {
	case errors.Is(err, domain.ErrInvalidModuleName):
		return ErrInvalidModuleName
	case errors.Is(err, domain.ErrInvalidGitLabBaseURL):
		return ErrInvalidGitLabBaseURL
	case errors.Is(err, domain.ErrInvalidGitLabProjectID):
		return ErrInvalidGitLabProjectID
	case errors.Is(err, domain.ErrInvalidGitLabProjectPath):
		return ErrInvalidGitLabProjectPath
	default:
		return err
	}
}

func (svc *Service) PublishModuleVersion(ctx context.Context, req PublishModuleVersionRequest) (PublishModuleVersionResponse, error) {
	name, err := domain.NewModuleName(req.ModuleName)
	if err != nil {
		return PublishModuleVersionResponse{}, ErrInvalidModuleName
	}
	versionValue, err := domain.NewVersion(req.Version)
	if err != nil {
		return PublishModuleVersionResponse{}, ErrInvalidVersion
	}
	if req.Artifact == nil {
		return PublishModuleVersionResponse{}, ErrStorageFailure
	}
	if svc.bufWorkflow == nil {
		return PublishModuleVersionResponse{}, ErrBufWorkflowUnavailable
	}

	module, err := svc.modules.GetByName(ctx, name)
	if errors.Is(err, domain.ErrNotFound) {
		return PublishModuleVersionResponse{}, ErrModuleNotFound
	}
	if err != nil {
		return PublishModuleVersionResponse{}, err
	}

	if _, err := svc.versions.GetByModuleAndVersion(ctx, module.ID, versionValue); err == nil {
		return PublishModuleVersionResponse{}, ErrModuleVersionAlreadyExists
	} else if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return PublishModuleVersionResponse{}, err
	}

	sourceBody, sourceChecksum, sourceSizeBytes, err := svc.readArtifact(req.Artifact)
	if err != nil {
		return PublishModuleVersionResponse{}, err
	}

	workdir, err := os.MkdirTemp("", "protoradar-publish-*")
	if err != nil {
		return PublishModuleVersionResponse{}, err
	}
	defer os.RemoveAll(workdir)

	maxUncompressed := svc.options.MaxSourceUncompressedSizeBytes
	if maxUncompressed <= 0 {
		maxUncompressed = svc.options.MaxArtifactSizeBytes
	}
	if err := workspace.ExtractTarGzSafe(ctx, bytes.NewReader(sourceBody), workdir, workspace.ExtractOptions{MaxUncompressedSizeBytes: maxUncompressed}); err != nil {
		return PublishModuleVersionResponse{}, fmt.Errorf("%w: %v", ErrUnsafeArchive, err)
	}
	if svc.options.BufRequireConfig {
		if _, err := os.Stat(filepath.Join(workdir, "buf.yaml")); err != nil {
			if os.IsNotExist(err) {
				return PublishModuleVersionResponse{}, ErrBufConfigNotFound
			}
			return PublishModuleVersionResponse{}, err
		}
	}

	bufResult, err := svc.bufWorkflow.Inspect(ctx, workdir, BufWorkflowOptions{
		RequireBufYAML: svc.options.BufRequireConfig,
		RunLint:        svc.options.BufLintMode != BufLintModeDisabled,
	})
	if err != nil {
		return PublishModuleVersionResponse{}, mapBufWorkflowError(err, bufResult)
	}
	if len(bufResult.BufImage) == 0 {
		return PublishModuleVersionResponse{}, ErrBufBuildFailed
	}

	bufImageChecksum, bufImageSizeBytes := checksumBytes(bufResult.BufImage)
	sourceKey := storage.BuildSourceArchiveKey(name, versionValue, sourceChecksum)
	bufImageKey := storage.BuildBufImageKey(name, versionValue, bufImageChecksum)

	sourceObject, err := svc.artifactStore.Put(ctx, sourceKey, bytes.NewReader(sourceBody), sourceSizeBytes)
	if err != nil {
		return PublishModuleVersionResponse{}, fmt.Errorf("%w: %v", ErrStorageFailure, err)
	}
	bufImageObject, err := svc.artifactStore.Put(ctx, bufImageKey, bytes.NewReader(bufResult.BufImage), bufImageSizeBytes)
	if err != nil {
		_ = svc.artifactStore.Delete(ctx, sourceKey)
		return PublishModuleVersionResponse{}, fmt.Errorf("%w: %v", ErrStorageFailure, err)
	}

	now := svc.clock.Now()
	moduleVersionID, err := svc.ids.NewModuleVersionID()
	if err != nil {
		svc.cleanupUploadedArtifacts(ctx, sourceKey, bufImageKey)
		return PublishModuleVersionResponse{}, err
	}
	sourceArtifactID, err := svc.ids.NewArtifactID()
	if err != nil {
		svc.cleanupUploadedArtifacts(ctx, sourceKey, bufImageKey)
		return PublishModuleVersionResponse{}, err
	}
	bufImageArtifactID, err := svc.ids.NewArtifactID()
	if err != nil {
		svc.cleanupUploadedArtifacts(ctx, sourceKey, bufImageKey)
		return PublishModuleVersionResponse{}, err
	}

	moduleVersion := domain.ModuleVersion{
		ID:        moduleVersionID,
		ModuleID:  module.ID,
		Version:   versionValue,
		Status:    domain.ModuleVersionStatusPublished,
		Digest:    bufResult.BufImageDigest,
		CreatedAt: now,
	}
	publishedAt := now
	moduleVersion.PublishedAt = &publishedAt

	sourceArtifact := domain.Artifact{
		ID:              sourceArtifactID,
		ModuleVersionID: moduleVersion.ID,
		Kind:            domain.ArtifactKindSourceArchive,
		StorageKey:      sourceObject.Key,
		ChecksumSHA256:  sourceChecksum,
		SizeBytes:       sourceObject.SizeBytes,
		CreatedAt:       now,
	}
	bufImageArtifact := domain.Artifact{
		ID:              bufImageArtifactID,
		ModuleVersionID: moduleVersion.ID,
		Kind:            domain.ArtifactKindBufImage,
		StorageKey:      bufImageObject.Key,
		ChecksumSHA256:  bufImageChecksum,
		SizeBytes:       bufImageObject.SizeBytes,
		CreatedAt:       now,
	}
	metadataSummary := bufResult.DescriptorMetadata.Summary()

	err = svc.transactions.WithinTransaction(ctx, func(txCtx context.Context) error {
		if err := svc.versions.Create(txCtx, moduleVersion); err != nil {
			if errors.Is(err, domain.ErrDuplicate) {
				return ErrModuleVersionAlreadyExists
			}
			return err
		}
		if err := svc.artifacts.Create(txCtx, sourceArtifact); err != nil {
			if errors.Is(err, domain.ErrDuplicate) {
				return ErrModuleVersionAlreadyExists
			}
			return err
		}
		if err := svc.artifacts.Create(txCtx, bufImageArtifact); err != nil {
			if errors.Is(err, domain.ErrDuplicate) {
				return ErrModuleVersionAlreadyExists
			}
			return err
		}
		if err := svc.bufConfigs.Save(txCtx, moduleVersion.ID, bufResult.ConfigInfo); err != nil {
			return err
		}
		if err := svc.metadata.Save(txCtx, moduleVersion.ID, bufResult.DescriptorMetadata); err != nil {
			return err
		}
		if svc.dependencies != nil {
			if _, err := svc.dependencies.RebuildPublishedModuleVersionDependencies(txCtx, PublishedModuleVersionDependencyRebuildInput{
				Module:   module,
				Version:  moduleVersion,
				Metadata: bufResult.DescriptorMetadata,
			}); err != nil {
				return err
			}
		}

		record, err := protoradarevents.NewModuleVersionPublished(protoradarevents.ModuleVersionPublished{
			Module:           module,
			Version:          moduleVersion,
			SourceArtifact:   sourceArtifact,
			BufImageArtifact: bufImageArtifact,
			BufConfig:        bufResult.ConfigInfo,
			LintResult:       bufResult.LintResult,
			MetadataSummary:  metadataSummary,
			OccurredAt:       now,
		})
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
		svc.cleanupUploadedArtifacts(ctx, sourceKey, bufImageKey)
		return PublishModuleVersionResponse{}, err
	}

	return PublishModuleVersionResponse{
		Version:          moduleVersion,
		SourceArtifact:   sourceArtifact,
		BufImageArtifact: bufImageArtifact,
		BufConfig:        bufResult.ConfigInfo,
		LintResult:       bufResult.LintResult,
		MetadataSummary:  metadataSummary,
	}, nil
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

func (svc *Service) GetModuleVersionDetails(ctx context.Context, moduleName string, versionValue string) (ModuleVersionDetailsResponse, error) {
	moduleVersion, err := svc.GetModuleVersion(ctx, moduleName, versionValue)
	if err != nil {
		return ModuleVersionDetailsResponse{}, err
	}

	artifacts, err := svc.artifacts.ListByModuleVersion(ctx, moduleVersion.ID)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return ModuleVersionDetailsResponse{}, err
	}
	bufConfig, err := svc.bufConfigs.GetByModuleVersion(ctx, moduleVersion.ID)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return ModuleVersionDetailsResponse{}, err
	}
	metadataSummary, err := svc.metadata.GetSummaryByModuleVersion(ctx, moduleVersion.ID)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return ModuleVersionDetailsResponse{}, err
	}

	return ModuleVersionDetailsResponse{
		Version:         moduleVersion,
		Artifacts:       artifacts,
		BufConfig:       bufConfig,
		LintResult:      lintResultFromConfig(bufConfig),
		MetadataSummary: metadataSummary,
	}, nil
}

func (svc *Service) GetModuleVersionMetadata(ctx context.Context, moduleName string, versionValue string) (domain.DescriptorMetadata, error) {
	moduleVersion, err := svc.GetModuleVersion(ctx, moduleName, versionValue)
	if err != nil {
		return domain.DescriptorMetadata{}, err
	}
	metadata, err := svc.metadata.GetByModuleVersion(ctx, moduleVersion.ID)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.DescriptorMetadata{}, ErrModuleNotFound
	}
	return metadata, err
}

func (svc *Service) CheckBreaking(ctx context.Context, req CheckBreakingRequest) (CheckBreakingResponse, error) {
	name, err := domain.NewModuleName(req.ModuleName)
	if err != nil {
		return CheckBreakingResponse{}, ErrInvalidModuleName
	}
	against := strings.TrimSpace(req.Against)
	if against == "" {
		against = strings.TrimSpace(svc.options.BreakingDefaultAgainst)
	}
	if against == "" {
		return CheckBreakingResponse{}, ErrInvalidAgainst
	}
	targetRef := strings.TrimSpace(req.TargetRef)
	if len(targetRef) > maxTargetRefLength {
		return CheckBreakingResponse{}, ErrInvalidTargetRef
	}
	if targetRef == "" {
		targetRef = "local"
	}
	if req.ProposedSourceArchive == nil {
		return CheckBreakingResponse{}, ErrArtifactRequired
	}
	if req.ArchiveSizeBytes > 0 && svc.options.MaxArtifactSizeBytes > 0 && req.ArchiveSizeBytes > svc.options.MaxArtifactSizeBytes {
		return CheckBreakingResponse{}, ErrArtifactTooLarge
	}
	if svc.bufBreaking == nil {
		return CheckBreakingResponse{}, ErrBufBreakingUnavailable
	}
	if svc.reports == nil {
		return CheckBreakingResponse{}, ErrBufBreakingUnavailable
	}

	module, err := svc.modules.GetByName(ctx, name)
	if errors.Is(err, domain.ErrNotFound) {
		return CheckBreakingResponse{}, ErrModuleNotFound
	}
	if err != nil {
		return CheckBreakingResponse{}, err
	}

	baseline, err := svc.resolveBreakingBaseline(ctx, module.ID, against)
	if err != nil {
		return CheckBreakingResponse{}, err
	}
	bufImageArtifact, err := svc.artifacts.GetByModuleVersionAndKind(ctx, baseline.ID, domain.ArtifactKindBufImage)
	if errors.Is(err, domain.ErrNotFound) {
		return CheckBreakingResponse{}, ErrBaselineBufImageMissing
	}
	if err != nil {
		return CheckBreakingResponse{}, err
	}
	baselineImage, err := svc.readStoredArtifact(ctx, bufImageArtifact.StorageKey)
	if err != nil {
		return CheckBreakingResponse{}, err
	}

	sourceBody, _, _, err := svc.readArtifact(req.ProposedSourceArchive)
	if err != nil {
		return CheckBreakingResponse{}, err
	}

	workdir, err := os.MkdirTemp("", "protoradar-breaking-*")
	if err != nil {
		return CheckBreakingResponse{}, err
	}
	defer os.RemoveAll(workdir)

	maxUncompressed := svc.options.MaxSourceUncompressedSizeBytes
	if maxUncompressed <= 0 {
		maxUncompressed = svc.options.MaxArtifactSizeBytes
	}
	if err := workspace.ExtractTarGzSafe(ctx, bytes.NewReader(sourceBody), workdir, workspace.ExtractOptions{MaxUncompressedSizeBytes: maxUncompressed}); err != nil {
		return CheckBreakingResponse{}, fmt.Errorf("%w: %v", ErrUnsafeArchive, err)
	}
	if _, err := os.Stat(filepath.Join(workdir, "buf.yaml")); err != nil {
		if os.IsNotExist(err) {
			return CheckBreakingResponse{}, ErrBufConfigNotFound
		}
		return CheckBreakingResponse{}, err
	}

	checkResult, err := svc.bufBreaking.CheckBreaking(ctx, BufBreakingCheckInput{
		Workdir:       workdir,
		BaselineImage: baselineImage,
		TargetRef:     targetRef,
	})
	if err != nil {
		return CheckBreakingResponse{}, fmt.Errorf("%w: %v", ErrBufBreakingFailed, err)
	}
	status := checkResult.Status
	if status == "" {
		if len(checkResult.Changes) > 0 {
			status = domain.BreakingReportStatusBreaking
		} else {
			status = domain.BreakingReportStatusPassed
		}
	}
	if !status.IsValid() {
		return CheckBreakingResponse{}, ErrBufBreakingFailed
	}
	if status == domain.BreakingReportStatusFailed {
		return CheckBreakingResponse{}, ErrBufBreakingFailed
	}

	now := svc.clock.Now()
	reportID, err := svc.ids.NewBreakingReportID()
	if err != nil {
		return CheckBreakingResponse{}, err
	}
	resultChanges := checkResult.Changes
	if svc.options.BreakingMaxChanges > 0 && len(resultChanges) > svc.options.BreakingMaxChanges {
		resultChanges = resultChanges[:svc.options.BreakingMaxChanges]
	}
	changes := make([]domain.BreakingChange, len(resultChanges))
	for index, change := range resultChanges {
		changeID, err := svc.ids.NewBreakingChangeID()
		if err != nil {
			return CheckBreakingResponse{}, err
		}
		change.ID = changeID
		change.ReportID = reportID
		if strings.TrimSpace(change.Severity) == "" {
			change.Severity = "error"
		}
		change.CreatedAt = now
		changes[index] = change
	}
	report := domain.BreakingReport{
		ID:            reportID,
		ModuleID:      module.ID,
		ModuleName:    module.Name,
		BaseVersionID: baseline.ID,
		BaseVersion:   baseline.Version,
		TargetRef:     targetRef,
		Status:        status,
		ChangeCount:   len(changes),
		RawOutput:     checkResult.RawOutput,
		HumanSummary:  checkResult.HumanSummary,
		CreatedAt:     now,
	}
	if strings.TrimSpace(report.HumanSummary) == "" {
		report.HumanSummary = breaking.FormatReport(report, changes)
	}

	err = svc.transactions.WithinTransaction(ctx, func(txCtx context.Context) error {
		if err := svc.reports.Create(txCtx, report, changes); err != nil {
			return err
		}
		record, err := protoradarevents.NewBreakingReportCreated(report)
		if err != nil {
			return err
		}
		if err := svc.outbox.Create(txCtx, record); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return CheckBreakingResponse{}, err
	}

	return CheckBreakingResponse{Report: report, Changes: changes}, nil
}

func (svc *Service) GetBreakingReport(ctx context.Context, reportIDValue string) (CheckBreakingResponse, error) {
	reportID := domain.NewBreakingReportID(reportIDValue)
	if reportID == "" {
		return CheckBreakingResponse{}, ErrBreakingReportNotFound
	}
	report, changes, err := svc.reports.GetByID(ctx, reportID)
	if errors.Is(err, domain.ErrNotFound) {
		return CheckBreakingResponse{}, ErrBreakingReportNotFound
	}
	if err != nil {
		return CheckBreakingResponse{}, err
	}
	return CheckBreakingResponse{Report: report, Changes: changes}, nil
}

func (svc *Service) ListBreakingReports(ctx context.Context, moduleName string, limit int, offset int) ([]domain.BreakingReport, error) {
	module, err := svc.GetModule(ctx, moduleName)
	if err != nil {
		return nil, err
	}
	return svc.reports.ListByModule(ctx, module.ID, limit, offset)
}

func (svc *Service) DownloadArtifact(ctx context.Context, moduleName string, versionValue string) (storage.ArtifactObject, domain.Artifact, error) {
	moduleVersion, err := svc.GetModuleVersion(ctx, moduleName, versionValue)
	if err != nil {
		return storage.ArtifactObject{}, domain.Artifact{}, err
	}
	artifact, err := svc.artifacts.GetByModuleVersionAndKind(ctx, moduleVersion.ID, domain.ArtifactKindSourceArchive)
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

func (svc *Service) readStoredArtifact(ctx context.Context, storageKey string) ([]byte, error) {
	object, err := svc.artifactStore.Get(ctx, storageKey)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrStorageFailure, err)
	}
	defer object.Body.Close()

	body, err := io.ReadAll(object.Body)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrStorageFailure, err)
	}
	return body, nil
}

func (svc *Service) resolveBreakingBaseline(ctx context.Context, moduleID domain.ModuleID, against string) (domain.ModuleVersion, error) {
	if against == "latest" {
		version, err := svc.versions.GetLatestByModule(ctx, moduleID)
		if errors.Is(err, domain.ErrNotFound) {
			return domain.ModuleVersion{}, ErrBaselineVersionNotFound
		}
		return version, err
	}

	versionValue, err := domain.NewVersion(against)
	if err != nil {
		return domain.ModuleVersion{}, ErrInvalidAgainst
	}
	version, err := svc.versions.GetByModuleAndVersion(ctx, moduleID, versionValue)
	if errors.Is(err, domain.ErrNotFound) {
		return domain.ModuleVersion{}, ErrBaselineVersionNotFound
	}
	return version, err
}

func checksumBytes(body []byte) (string, int64) {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:]), int64(len(body))
}

func mapBufWorkflowError(err error, result BufWorkflowResult) error {
	if result.LintResult.Status == domain.BufLintStatusFailed {
		return fmt.Errorf("%w: %v", ErrBufLintFailed, err)
	}
	if len(result.BufImage) > 0 {
		return fmt.Errorf("%w: %v", ErrDescriptorExtractionFailed, err)
	}
	return fmt.Errorf("%w: %v", ErrBufBuildFailed, err)
}

func lintResultFromConfig(config domain.BufConfigInfo) domain.BufLintResult {
	if !config.LintEnabled {
		return domain.BufLintResult{Status: domain.BufLintStatusNotRun}
	}
	return domain.BufLintResult{Status: domain.BufLintStatusPassed}
}

func (svc *Service) cleanupUploadedArtifacts(ctx context.Context, keys ...string) {
	for _, key := range keys {
		if key == "" {
			continue
		}
		_ = svc.artifactStore.Delete(ctx, key)
	}
}

func (svc *Service) hashToken(rawToken string) string {
	mac := hmac.New(sha256.New, []byte(svc.options.TokenHashSecret))
	mac.Write([]byte(rawToken))
	return hex.EncodeToString(mac.Sum(nil))
}
