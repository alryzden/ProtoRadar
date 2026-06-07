package domain

import "time"

type ModuleVersion struct {
	ID                ModuleVersionID
	ModuleID          ModuleID
	Version           Version
	Status            ModuleVersionStatus
	Digest            string
	CreatedAt         time.Time
	PublishedAt       *time.Time
	DeprecatedAt      *time.Time
	DeprecatedBy      string
	DeprecationReason string
}

func (version ModuleVersion) IsDeprecated() bool {
	return version.DeprecatedAt != nil
}
