package domain

import "errors"

var (
	ErrInvalidModuleName          = errors.New("invalid module name")
	ErrInvalidVersion             = errors.New("invalid version")
	ErrInvalidModuleVersionState  = errors.New("invalid module version status")
	ErrInvalidArtifactKind        = errors.New("invalid artifact kind")
	ErrInvalidBreakingStatus      = errors.New("invalid breaking report status")
	ErrInvalidGitLabBaseURL       = errors.New("invalid gitlab base url")
	ErrInvalidGitLabProjectID     = errors.New("invalid gitlab project id")
	ErrInvalidGitLabProjectPath   = errors.New("invalid gitlab project path")
	ErrInvalidRuntimeServiceName  = errors.New("invalid runtime service name")
	ErrInvalidRuntimeEnvironment  = errors.New("invalid runtime environment")
	ErrInvalidRuntimeGitCommit    = errors.New("invalid runtime git commit")
	ErrInvalidRuntimeBuildVersion = errors.New("invalid runtime build version")
	ErrInvalidRuntimeDriftStatus  = errors.New("invalid runtime drift status")
	ErrDuplicate                  = errors.New("duplicate record")
	ErrNotFound                   = errors.New("record not found")
)
