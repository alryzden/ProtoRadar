package domain

import (
	"errors"
	"testing"
)

func TestNewArtifactKindAcceptsSupportedKinds(t *testing.T) {
	for _, value := range []string{"source_archive", "buf_image"} {
		kind, err := NewArtifactKind(value)
		if err != nil {
			t.Fatalf("kind %q: %v", value, err)
		}
		if kind.String() != value {
			t.Fatalf("kind = %q, want %q", kind, value)
		}
		if !kind.IsValid() {
			t.Fatalf("kind %q should be valid", value)
		}
	}
}

func TestNewArtifactKindRejectsUnsupportedKind(t *testing.T) {
	_, err := NewArtifactKind("descriptor_set")
	if !errors.Is(err, ErrInvalidArtifactKind) {
		t.Fatalf("error = %v, want ErrInvalidArtifactKind", err)
	}

	if ArtifactKind("descriptor_set").IsValid() {
		t.Fatalf("unsupported kind should be invalid")
	}
}
