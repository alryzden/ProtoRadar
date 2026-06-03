package domain

import "time"

type ModuleVersion struct {
	ID          ModuleVersionID
	ModuleID    ModuleID
	Version     Version
	Status      ModuleVersionStatus
	Digest      string
	CreatedAt   time.Time
	PublishedAt *time.Time
}
