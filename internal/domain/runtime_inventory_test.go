package domain

import (
	"errors"
	"testing"
)

func TestNewRuntimeServiceNameValid(t *testing.T) {
	valid := []string{
		"billing-service",
		"notification_service",
		"platform/api-gateway",
		"Platform.APIGateway1",
	}

	for _, value := range valid {
		name, err := NewRuntimeServiceName(value)
		if err != nil {
			t.Fatalf("runtime service name %q: %v", value, err)
		}
		if name.String() != value {
			t.Fatalf("runtime service name = %q, want %q", name.String(), value)
		}
	}
}

func TestNewRuntimeServiceNameInvalid(t *testing.T) {
	invalid := []string{
		"",
		"   ",
		"billing service",
		"billing:service",
		string(make([]byte, MaxRuntimeServiceNameLength+1)),
	}

	for _, value := range invalid {
		if _, err := NewRuntimeServiceName(value); !errors.Is(err, ErrInvalidRuntimeServiceName) {
			t.Fatalf("runtime service name %q error = %v, want %v", value, err, ErrInvalidRuntimeServiceName)
		}
	}
}

func TestNewRuntimeEnvironmentValid(t *testing.T) {
	valid := []string{
		"prod",
		"staging_us",
		"platform/prod",
		"qa-1",
	}

	for _, value := range valid {
		environment, err := NewRuntimeEnvironment(value)
		if err != nil {
			t.Fatalf("runtime environment %q: %v", value, err)
		}
		if environment.String() != value {
			t.Fatalf("runtime environment = %q, want %q", environment.String(), value)
		}
	}
}

func TestNewRuntimeEnvironmentInvalid(t *testing.T) {
	invalid := []string{
		"",
		"   ",
		"prod us",
		"prod:us",
		string(make([]byte, MaxRuntimeEnvironmentLength+1)),
	}

	for _, value := range invalid {
		if _, err := NewRuntimeEnvironment(value); !errors.Is(err, ErrInvalidRuntimeEnvironment) {
			t.Fatalf("runtime environment %q error = %v, want %v", value, err, ErrInvalidRuntimeEnvironment)
		}
	}
}

func TestValidateRuntimeGitCommit(t *testing.T) {
	if err := ValidateRuntimeGitCommit("abc123"); err != nil {
		t.Fatalf("valid git commit: %v", err)
	}
	for _, value := range []string{"", "   ", "abc 123", string(make([]byte, MaxRuntimeGitCommitLength+1))} {
		if err := ValidateRuntimeGitCommit(value); !errors.Is(err, ErrInvalidRuntimeGitCommit) {
			t.Fatalf("git commit %q error = %v, want %v", value, err, ErrInvalidRuntimeGitCommit)
		}
	}
}

func TestValidateRuntimeBuildVersion(t *testing.T) {
	if err := ValidateRuntimeBuildVersion("pipeline-123"); err != nil {
		t.Fatalf("valid build version: %v", err)
	}
	for _, value := range []string{"", "   ", "build\n123", string(make([]byte, MaxRuntimeBuildVersionLength+1))} {
		if err := ValidateRuntimeBuildVersion(value); !errors.Is(err, ErrInvalidRuntimeBuildVersion) {
			t.Fatalf("build version %q error = %v, want %v", value, err, ErrInvalidRuntimeBuildVersion)
		}
	}
}

func TestNewRuntimeDriftStatus(t *testing.T) {
	valid := []RuntimeDriftStatus{
		RuntimeDriftStatusUpToDate,
		RuntimeDriftStatusBehindLatest,
		RuntimeDriftStatusUnknownVersion,
		RuntimeDriftStatusDeprecatedVersion,
	}

	for _, value := range valid {
		status, err := NewRuntimeDriftStatus(value.String())
		if err != nil {
			t.Fatalf("drift status %q: %v", value, err)
		}
		if status != value || !status.IsValid() {
			t.Fatalf("drift status = %q, want %q", status, value)
		}
	}

	if _, err := NewRuntimeDriftStatus("potentially_affected_by_breaking_change"); !errors.Is(err, ErrInvalidRuntimeDriftStatus) {
		t.Fatalf("breaking impact should not be accepted as drift status: %v", err)
	}
}
