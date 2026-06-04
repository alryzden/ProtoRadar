package domain

import "testing"

func TestNewBreakingReportStatusAcceptsSupportedStatuses(t *testing.T) {
	for _, value := range []string{"passed", "breaking", "failed"} {
		status, err := NewBreakingReportStatus(value)
		if err != nil {
			t.Fatalf("status %q: %v", value, err)
		}
		if status.String() != value {
			t.Fatalf("status = %q, want %q", status.String(), value)
		}
	}
}

func TestNewBreakingReportStatusRejectsInvalidStatus(t *testing.T) {
	if _, err := NewBreakingReportStatus("warning"); err != ErrInvalidBreakingStatus {
		t.Fatalf("error = %v, want ErrInvalidBreakingStatus", err)
	}
}
