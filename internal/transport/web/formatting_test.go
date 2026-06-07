package web

import (
	"testing"
)

func TestStatusBadgeMapping(t *testing.T) {
	tests := []struct {
		status string
		want   string
	}{
		{status: "published", want: "badge badge-published"},
		{status: "passed", want: "badge badge-passed"},
		{status: "breaking", want: "badge badge-breaking"},
		{status: "failed", want: "badge badge-failed"},
		{status: "warning", want: "badge badge-warning"},
		{status: "not-a-real-status", want: "badge badge-unknown"},
		{status: "", want: "badge badge-unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			if got := statusBadgeClass(tt.status); got != tt.want {
				t.Fatalf("class = %q, want %q", got, tt.want)
			}
		})
	}
}
