package domain

import "time"

type Module struct {
	ID            ModuleID
	Name          ModuleName
	Description   string
	RepositoryURL string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}
