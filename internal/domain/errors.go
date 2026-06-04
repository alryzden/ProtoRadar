package domain

import "errors"

var (
	ErrInvalidModuleName         = errors.New("invalid module name")
	ErrInvalidVersion            = errors.New("invalid version")
	ErrInvalidModuleVersionState = errors.New("invalid module version status")
	ErrInvalidArtifactKind       = errors.New("invalid artifact kind")
	ErrInvalidBreakingStatus     = errors.New("invalid breaking report status")
	ErrInvalidGitLabBaseURL      = errors.New("invalid gitlab base url")
	ErrInvalidGitLabProjectID    = errors.New("invalid gitlab project id")
	ErrInvalidGitLabProjectPath  = errors.New("invalid gitlab project path")
	ErrDuplicate                 = errors.New("duplicate record")
	ErrNotFound                  = errors.New("record not found")
)
