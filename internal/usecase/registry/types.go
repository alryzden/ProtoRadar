package registry

import (
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

type Options struct {
	MaxArtifactSizeBytes int64
	TokenHashSecret      string
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
