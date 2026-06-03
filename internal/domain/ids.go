package domain

import "strings"

type ModuleID string

func NewModuleID(value string) ModuleID {
	return ModuleID(strings.TrimSpace(value))
}

func (id ModuleID) String() string {
	return string(id)
}

type ModuleVersionID string

func NewModuleVersionID(value string) ModuleVersionID {
	return ModuleVersionID(strings.TrimSpace(value))
}

func (id ModuleVersionID) String() string {
	return string(id)
}

type ArtifactID string

func NewArtifactID(value string) ArtifactID {
	return ArtifactID(strings.TrimSpace(value))
}

func (id ArtifactID) String() string {
	return string(id)
}

type APITokenID string

func NewAPITokenID(value string) APITokenID {
	return APITokenID(strings.TrimSpace(value))
}

func (id APITokenID) String() string {
	return string(id)
}
