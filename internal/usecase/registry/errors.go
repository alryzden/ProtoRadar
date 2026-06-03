package registry

import "errors"

var (
	ErrInvalidModuleName          = errors.New("invalid module name")
	ErrInvalidVersion             = errors.New("invalid version")
	ErrModuleNotFound             = errors.New("module not found")
	ErrModuleAlreadyExists        = errors.New("module already exists")
	ErrModuleVersionAlreadyExists = errors.New("module version already exists")
	ErrArtifactTooLarge           = errors.New("artifact too large")
	ErrInvalidOrExpiredToken      = errors.New("invalid or expired token")
	ErrStorageFailure             = errors.New("storage failure")
)
