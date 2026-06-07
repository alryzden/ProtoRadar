package domain

import (
	"testing"
	"time"
)

func TestModuleVersionIsDeprecated(t *testing.T) {
	versionValue, err := NewVersion("v1.0.0")
	if err != nil {
		t.Fatalf("version: %v", err)
	}
	version := ModuleVersion{
		ID:        NewModuleVersionID("version-1"),
		ModuleID:  NewModuleID("module-1"),
		Version:   versionValue,
		Status:    ModuleVersionStatusPublished,
		CreatedAt: time.Date(2026, 6, 6, 10, 0, 0, 0, time.UTC),
	}

	if version.IsDeprecated() {
		t.Fatalf("new module version should not be deprecated")
	}

	deprecatedAt := time.Date(2026, 6, 6, 11, 0, 0, 0, time.UTC)
	version.DeprecatedAt = &deprecatedAt
	version.DeprecatedBy = "api-token:platform"
	version.DeprecationReason = "superseded by v1.1.0"

	if !version.IsDeprecated() {
		t.Fatalf("module version with deprecated_at should be deprecated")
	}
}
