package storage

import (
	"testing"

	"github.com/alryzden/ProtoRadar/internal/domain"
)

func TestBuildArtifactKey(t *testing.T) {
	module, err := domain.NewModuleName("billing-api")
	if err != nil {
		t.Fatalf("module name: %v", err)
	}
	version, err := domain.NewVersion("v1.2.3")
	if err != nil {
		t.Fatalf("version: %v", err)
	}

	got := BuildArtifactKey(module, version, " checksum ")
	want := "modules/billing-api/versions/v1.2.3/sha256-checksum.tar.gz"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
