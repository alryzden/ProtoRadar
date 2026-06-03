package domain

import (
	"errors"
	"testing"
)

func TestNewModuleName(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{name: "lowercase", value: "billing", wantErr: false},
		{name: "digits", value: "billing2", wantErr: false},
		{name: "hyphen", value: "billing-api", wantErr: false},
		{name: "underscore", value: "billing_api", wantErr: false},
		{name: "empty", value: "", wantErr: true},
		{name: "spaces only", value: "   ", wantErr: true},
		{name: "with space", value: "billing api", wantErr: true},
		{name: "uppercase", value: "Billing", wantErr: true},
		{name: "unsafe slash", value: "billing/api", wantErr: true},
		{name: "unsafe dot", value: "billing.api", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewModuleName(tt.value)
			if tt.wantErr {
				if !errors.Is(err, ErrInvalidModuleName) {
					t.Fatalf("expected ErrInvalidModuleName, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.String() != tt.value {
				t.Fatalf("got %q, want %q", got.String(), tt.value)
			}
		})
	}
}
