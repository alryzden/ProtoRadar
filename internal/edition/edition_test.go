package edition

import (
	"context"
	"errors"
	"testing"

	"github.com/alryzden/ProtoRadar/internal/version"
)

func TestCommunityCapabilityCheckerEnablesCommunityCapabilities(t *testing.T) {
	checker := NewCommunityCapabilityChecker()
	for _, capability := range CommunityCapabilities() {
		if !checker.IsEnabled(context.Background(), capability) {
			t.Fatalf("community capability %q disabled", capability)
		}
	}
}

func TestCommunityCapabilityCheckerDisablesEnterpriseCapabilities(t *testing.T) {
	checker := NewCommunityCapabilityChecker()
	for _, capability := range EnterpriseCapabilities() {
		if checker.IsEnabled(context.Background(), capability) {
			t.Fatalf("enterprise capability %q enabled", capability)
		}
	}
}

func TestCommunityCapabilityCheckerRequire(t *testing.T) {
	checker := NewCommunityCapabilityChecker()
	if err := checker.Require(context.Background(), CapabilityRegistry); err != nil {
		t.Fatalf("require enabled capability: %v", err)
	}
	if err := checker.Require(context.Background(), CapabilityOIDCAuth); !errors.Is(err, ErrCapabilityNotAvailable) {
		t.Fatalf("require disabled capability error = %v, want %v", err, ErrCapabilityNotAvailable)
	}
}

func TestCommunityEditionIncludesVersionAndCapabilities(t *testing.T) {
	build := version.BuildInfo{Version: "v1.2.3", Commit: "abc123", BuildDate: "2026-06-05T12:00:00Z"}
	model := NewCommunityEdition(context.Background(), build, NewCommunityCapabilityChecker())
	if model.Name != NameCommunity {
		t.Fatalf("edition name = %q, want %q", model.Name, NameCommunity)
	}
	if model.Version != build {
		t.Fatalf("version = %#v, want %#v", model.Version, build)
	}
	if len(model.Capabilities) != len(AllCapabilities()) {
		t.Fatalf("capabilities = %d, want %d", len(model.Capabilities), len(AllCapabilities()))
	}
}

func TestNoEnterpriseCapabilityIsEnabled(t *testing.T) {
	model := NewCommunityEdition(context.Background(), version.BuildInfo{}, NewCommunityCapabilityChecker())
	enterprise := map[Capability]struct{}{}
	for _, capability := range EnterpriseCapabilities() {
		enterprise[capability] = struct{}{}
	}
	for _, status := range model.Capabilities {
		if _, ok := enterprise[status.Capability]; ok && status.Enabled {
			t.Fatalf("enterprise capability %q enabled", status.Capability)
		}
	}
}
