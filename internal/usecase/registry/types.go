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
	NewModuleVersionID() (domain.ModuleVersionID, error)
	NewArtifactID() (domain.ArtifactID, error)
	NewAPITokenID() (domain.APITokenID, error)
}

type TokenGenerator interface {
	NewToken() (string, error)
}

type BufWorkflow interface {
	Inspect(ctx context.Context, workdir string, options BufWorkflowOptions) (BufWorkflowResult, error)
}

type BufWorkflowOptions struct {
	RequireBufYAML bool
	RunLint        bool
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
}

type CreateModuleRequest struct {
	Name          string
	Description   string
	RepositoryURL string
}

type PublishModuleVersionRequest struct {
	ModuleName string
	Version    string
	Artifact   io.Reader
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
