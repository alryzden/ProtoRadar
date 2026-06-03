package domain

import "errors"

var (
	ErrInvalidModuleName         = errors.New("invalid module name")
	ErrInvalidVersion            = errors.New("invalid version")
	ErrInvalidModuleVersionState = errors.New("invalid module version status")
	ErrInvalidArtifactKind       = errors.New("invalid artifact kind")
	ErrDuplicate                 = errors.New("duplicate record")
	ErrNotFound                  = errors.New("record not found")
)
