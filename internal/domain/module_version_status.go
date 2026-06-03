package domain

type ModuleVersionStatus string

const (
	ModuleVersionStatusDraft     ModuleVersionStatus = "draft"
	ModuleVersionStatusPublished ModuleVersionStatus = "published"
)

func NewModuleVersionStatus(value string) (ModuleVersionStatus, error) {
	status := ModuleVersionStatus(value)
	if status != ModuleVersionStatusDraft && status != ModuleVersionStatusPublished {
		return "", ErrInvalidModuleVersionState
	}
	return status, nil
}

func (status ModuleVersionStatus) String() string {
	return string(status)
}
