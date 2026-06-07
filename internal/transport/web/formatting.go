package web

import (
	"net/url"
	"strings"
	"time"

	"github.com/alryzden/ProtoRadar/internal/usecase/uiquery"
)

func statusBadgeClass(status string) string {
	switch normalizeStatus(status) {
	case "published":
		return "badge badge-published"
	case "passed", "success", "approved", "not_required":
		return "badge badge-passed"
	case "breaking", "rejected":
		return "badge badge-breaking"
	case "failed", "failure", "error":
		return "badge badge-failed"
	case "warning", "warn", "pending":
		return "badge badge-warning"
	case "up_to_date":
		return "badge badge-up-to-date"
	case "behind_latest":
		return "badge badge-behind-latest"
	case "unknown_version":
		return "badge badge-unknown-version"
	case "deprecated_version":
		return "badge badge-deprecated-version"
	case "potentially_affected_by_breaking_change":
		return "badge badge-runtime-impact"
	default:
		return "badge badge-unknown"
	}
}

func statusBadgeText(status string) string {
	value := strings.TrimSpace(status)
	if value == "" {
		return "unknown"
	}
	return value
}

func moduleStatus(overview uiquery.ModuleOverview) string {
	if overview.LatestVersion != nil {
		return overview.LatestVersion.Status
	}
	return "unknown"
}

func externalProjectURL(baseURL string, projectPath string) string {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	path := strings.Trim(strings.TrimSpace(projectPath), "/")
	if base == "" || path == "" {
		return ""
	}
	return base + "/" + path
}

func shortDigest(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= 16 {
		return value
	}
	return value[:12] + "..." + value[len(value)-4:]
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format("2006-01-02 15:04 UTC")
}

func formatTimePtr(value *time.Time) string {
	if value == nil {
		return ""
	}
	return formatTime(*value)
}

func yesNo(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}

func pathEscape(value string) string {
	return url.PathEscape(strings.TrimSpace(value))
}

func normalizeStatus(status string) string {
	return strings.ToLower(strings.TrimSpace(status))
}

func normalizePath(value string, fallback string) string {
	path := strings.TrimRight(strings.TrimSpace(value), "/")
	if path == "" {
		path = strings.TrimRight(strings.TrimSpace(fallback), "/")
	}
	if path == "" {
		return "/"
	}
	return path
}
