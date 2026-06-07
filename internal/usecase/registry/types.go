package registry

import (
	"context"
	"io"
	"time"

	"github.com/alryzden/ProtoRadar/internal/domain"
)

type Clock interface {
	Now() time.Time
}

type IDGenerator interface {
	NewModuleID() (domain.ModuleID, error)
	NewModuleGitLabProjectID() (domain.ModuleGitLabProjectID, error)
	NewModuleVersionID() (domain.ModuleVersionID, error)
	NewArtifactID() (domain.ArtifactID, error)
	NewAPITokenID() (domain.APITokenID, error)
	NewBreakingReportID() (domain.BreakingReportID, error)
	NewBreakingChangeID() (domain.BreakingChangeID, error)
}

type TokenGenerator interface {
	NewToken() (string, error)
}

type BufWorkflow interface {
	Inspect(ctx context.Context, workdir string, options BufWorkflowOptions) (BufWorkflowResult, error)
}

type BufBreakingChecker interface {
	CheckBreaking(ctx context.Context, input BufBreakingCheckInput) (BufBreakingCheckResult, error)
}

type ArtifactCleanupObserver interface {
	RecordArtifactCleanupFailure(ctx context.Context, failure ArtifactCleanupFailure)
}

type ArtifactCleanupFailure struct {
	StorageKey string
	Error      error
}

type TokenUsageObserver interface {
	RecordTokenUsageFailure(ctx context.Context, failure TokenUsageFailure)
}

type TokenUsageFailure struct {
	TokenID string
	Error   error
}

type BufWorkflowOptions struct {
	RequireBufYAML bool
	RunLint        bool
}

type BufBreakingCheckInput struct {
	Workdir       string
	BaselineImage []byte
	TargetRef     string
}

type BufBreakingCheckResult struct {
	Status       domain.BreakingReportStatus
	Changes      []domain.BreakingChange
	RawOutput    string
	HumanSummary string
}

const (
	BufLintModeDisabled = "disabled"
	BufLintModeWarn     = "warn"
	BufLintModeEnforce  = "enforce"
)

type BufWorkflowResult struct {
	ConfigInfo         domain.BufConfigInfo
	BufImage           []byte
	BufImageDigest     string
	LintResult         domain.BufLintResult
	DescriptorMetadata domain.DescriptorMetadata
}

type Options struct {
	MaxArtifactSizeBytes           int64
	MaxSourceUncompressedSizeBytes int64
	TokenHashSecret                string
	BufRequireConfig               bool
	BufLintMode                    string
	BreakingMaxChanges             int
	BreakingDefaultAgainst         string
	ArtifactCleanupObserver        ArtifactCleanupObserver
	TokenUsageObserver             TokenUsageObserver
}

type CreateModuleRequest struct {
	Name          string
	Description   string
	RepositoryURL string
}

type LinkModuleGitLabProjectInput struct {
	ModuleName        string
	GitLabBaseURL     string
	GitLabProjectID   int64
	GitLabProjectPath string
}

type LinkModuleGitLabProjectOutput struct {
	Mapping domain.ModuleGitLabProject
}

type GetModuleGitLabProjectOutput struct {
	Mapping domain.ModuleGitLabProject
}

type PublishModuleVersionRequest struct {
	ModuleName string
	Version    string
	Artifact   io.Reader
}

type DeprecateModuleVersionInput struct {
	ModuleName string
	Version    string
	Actor      string
	Reason     string
}

type DeprecateModuleVersionOutput struct {
	Version domain.ModuleVersion
}

type CheckBreakingRequest struct {
	ModuleName            string
	Against               string
	TargetRef             string
	ProposedSourceArchive io.Reader
	ArchiveName           string
	ArchiveSizeBytes      int64
	ArchiveChecksumSHA256 string
}

type CheckBreakingResponse struct {
	Report  domain.BreakingReport
	Changes []domain.BreakingChange
}

type ModuleDependencyGraphResponse struct {
	Module     domain.Module
	Upstream   []domain.ModuleDependency
	Downstream []domain.ModuleDependency
	Unresolved []domain.UnresolvedProtoDependency
}

type AffectedModulesResponse struct {
	Module          domain.Module
	AffectedModules []domain.AffectedModule
}

type BreakingReportAffectedModulesResponse struct {
	Report          domain.BreakingReport
	AffectedModules []domain.AffectedModule
}

type PublishModuleVersionResponse struct {
	Version          domain.ModuleVersion
	SourceArtifact   domain.Artifact
	BufImageArtifact domain.Artifact
	BufConfig        domain.BufConfigInfo
	LintResult       domain.BufLintResult
	MetadataSummary  domain.DescriptorMetadataSummary
}

type ModuleVersionDetailsResponse struct {
	Version         domain.ModuleVersion
	Artifacts       []domain.Artifact
	BufConfig       domain.BufConfigInfo
	LintResult      domain.BufLintResult
	MetadataSummary domain.DescriptorMetadataSummary
}

type CreateAPITokenRequest struct {
	Name      string
	ExpiresAt *time.Time
}

type CreateAPITokenResponse struct {
	Token    domain.APIToken
	RawToken string
}

type AuthSubject struct {
	TokenID domain.APITokenID
	Name    string
}
