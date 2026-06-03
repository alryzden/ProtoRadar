package domain

import "time"

type Artifact struct {
	ID              ArtifactID
	ModuleVersionID ModuleVersionID
	Kind            ArtifactKind
	StorageKey      string
	ChecksumSHA256  string
	SizeBytes       int64
	CreatedAt       time.Time
}
