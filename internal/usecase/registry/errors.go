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
	ErrBufConfigNotFound          = errors.New("buf config not found")
	ErrBufBuildFailed             = errors.New("buf build failed")
	ErrBufLintFailed              = errors.New("buf lint failed")
	ErrDescriptorExtractionFailed = errors.New("descriptor extraction failed")
	ErrUnsafeArchive              = errors.New("unsafe archive")
	ErrBufWorkflowUnavailable     = errors.New("buf workflow unavailable")
)
