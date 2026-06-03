package domain

import "strings"

type ArtifactKind string

const (
	ArtifactKindSourceArchive ArtifactKind = "source_archive"
	ArtifactKindBufImage      ArtifactKind = "buf_image"
)

func NewArtifactKind(value string) (ArtifactKind, error) {
	switch kind := ArtifactKind(strings.TrimSpace(value)); kind {
	case ArtifactKindSourceArchive, ArtifactKindBufImage:
		return kind, nil
	default:
		return "", ErrInvalidArtifactKind
	}
}

func (kind ArtifactKind) String() string {
	return string(kind)
}

func (kind ArtifactKind) IsValid() bool {
	_, err := NewArtifactKind(kind.String())
	return err == nil
}
