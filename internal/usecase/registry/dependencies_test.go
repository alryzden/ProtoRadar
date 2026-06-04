package registry

import "testing"

func TestIsWellKnownProtoImport(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{path: "google/protobuf/timestamp.proto", want: true},
		{path: " google/protobuf/empty.proto ", want: true},
		{path: "google/type/date.proto", want: false},
		{path: "acme/google/protobuf/timestamp.proto", want: false},
		{path: "user/v1/user.proto", want: false},
		{path: "", want: false},
	}

	for _, tt := range tests {
		if got := IsWellKnownProtoImport(tt.path); got != tt.want {
			t.Fatalf("IsWellKnownProtoImport(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestIsWellKnownProtoSymbol(t *testing.T) {
	tests := []struct {
		symbol string
		want   bool
	}{
		{symbol: ".google.protobuf.Timestamp", want: true},
		{symbol: "google.protobuf.Empty", want: true},
		{symbol: " google.protobuf.Duration ", want: true},
		{symbol: "google.type.Date", want: false},
		{symbol: "acme.google.protobuf.Timestamp", want: false},
		{symbol: "user.v1.User", want: false},
		{symbol: "", want: false},
	}

	for _, tt := range tests {
		if got := IsWellKnownProtoSymbol(tt.symbol); got != tt.want {
			t.Fatalf("IsWellKnownProtoSymbol(%q) = %v, want %v", tt.symbol, got, tt.want)
		}
	}
}
