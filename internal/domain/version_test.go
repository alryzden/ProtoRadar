package domain

import (
	"errors"
	"testing"
)

func TestNewVersion(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    string
		wantErr bool
	}{
		{name: "plain", value: "v1", want: "v1", wantErr: false},
		{name: "semver-like", value: "1.2.3", want: "1.2.3", wantErr: false},
		{name: "trim", value: "  v1.0.0  ", want: "v1.0.0", wantErr: false},
		{name: "empty", value: "", wantErr: true},
		{name: "spaces only", value: "   ", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewVersion(tt.value)
			if tt.wantErr {
				if !errors.Is(err, ErrInvalidVersion) {
					t.Fatalf("expected ErrInvalidVersion, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.String() != tt.want {
				t.Fatalf("got %q, want %q", got.String(), tt.want)
			}
		})
	}
}
