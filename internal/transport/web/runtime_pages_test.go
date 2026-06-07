package web

import (
	"net/http"
	"strings"
	"testing"

	"github.com/alryzden/ProtoRadar/internal/usecase/uiquery"
)

func TestRuntimeServicesPageRendersServices(t *testing.T) {
	handler := newTestHandler(t, newFakeQuery())
	res := request(t, handler, "/ui/runtime/services")

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d", res.Code)
	}
	body := res.Body.String()
	for _, want := range []string{"Runtime Services", "billing-service", "production", "2026-06-04", "behind_latest", `href="/ui/runtime/services/billing-service"`, `href="/ui/runtime/environments/production"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q: %s", want, body)
		}
	}
}

func TestRuntimeServicesPageAppliesFilters(t *testing.T) {
	query := newFakeQuery()
	handler := newTestHandler(t, query)
	res := request(t, handler, "/ui/runtime/services?q=billing&environment=production&drift_status=behind_latest")

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d", res.Code)
	}
	if query.lastRuntimeListInput.Query != "billing" || query.lastRuntimeListInput.Environment != "production" || query.lastRuntimeListInput.DriftStatus != "behind_latest" {
		t.Fatalf("filters = %#v", query.lastRuntimeListInput)
	}
}

func TestRuntimeServicesPageEmptyState(t *testing.T) {
	query := newFakeQuery()
	query.runtimeServices = nil
	handler := newTestHandler(t, query)
	res := request(t, handler, "/ui/runtime/services")

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d", res.Code)
	}
	if !strings.Contains(res.Body.String(), "No runtime services have reported inventory yet.") {
		t.Fatalf("body missing empty state: %s", res.Body.String())
	}
}

func TestRuntimeServiceDetailsPageRendersDeploymentsAndUsages(t *testing.T) {
	handler := newTestHandler(t, newFakeQuery())
	res := request(t, handler, "/ui/runtime/services/billing-service")

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d", res.Code)
	}
	body := res.Body.String()
	for _, want := range []string{"Runtime Service", "billing-service", "production", "abc1234", "2026.06.04-15", "user-api", "v1.2.0", "v2.0.0", "behind_latest", `href="/ui/modules/user-api"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q: %s", want, body)
		}
	}
}

func TestEnvironmentPageRendersServicesAndUsages(t *testing.T) {
	handler := newTestHandler(t, newFakeQuery())
	res := request(t, handler, "/ui/runtime/environments/production")

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d", res.Code)
	}
	body := res.Body.String()
	for _, want := range []string{"Runtime Environment", "production", "billing-service", "abc1234", "user-api", "behind_latest", `href="/ui/runtime/services/billing-service"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q: %s", want, body)
		}
	}
}

func TestMissingEnvironmentRendersUsefulEmptyPage(t *testing.T) {
	handler := newTestHandler(t, newFakeQuery())
	res := request(t, handler, "/ui/runtime/environments/staging")

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d", res.Code)
	}
	if !strings.Contains(res.Body.String(), "No runtime services are currently known in this environment.") {
		t.Fatalf("body missing empty state: %s", res.Body.String())
	}
}

func TestModuleRuntimeUsagesPageRendersUsages(t *testing.T) {
	handler := newTestHandler(t, newFakeQuery())
	res := request(t, handler, "/ui/modules/user-api/runtime-usages")

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d", res.Code)
	}
	body := res.Body.String()
	for _, want := range []string{"Module Runtime Usage", "user-api", "billing-service", "production", "v1.2.0", "v2.0.0", "behind_latest", `href="/ui/runtime/services/billing-service"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("body missing %q: %s", want, body)
		}
	}
}

func TestModuleRuntimeUsagesEmptyState(t *testing.T) {
	query := newFakeQuery()
	query.moduleRuntimeUsages["user-api"] = uiquery.ModuleRuntimeUsages{Module: "user-api"}
	handler := newTestHandler(t, query)
	res := request(t, handler, "/ui/modules/user-api/runtime-usages")

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d", res.Code)
	}
	if !strings.Contains(res.Body.String(), "No runtime services are currently known to use this module.") {
		t.Fatalf("body missing empty state: %s", res.Body.String())
	}
}

func TestMissingModuleRuntimeUsagesReturnsNotFound(t *testing.T) {
	handler := newTestHandler(t, newFakeQuery())
	res := request(t, handler, "/ui/modules/missing/runtime-usages")

	if res.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusNotFound)
	}
	if !strings.Contains(res.Body.String(), "Module not found") {
		t.Fatalf("body = %s", res.Body.String())
	}
}

func TestRuntimeStatusBadgesRenderClasses(t *testing.T) {
	tests := []struct {
		status string
		want   string
	}{
		{status: "up_to_date", want: "badge badge-up-to-date"},
		{status: "behind_latest", want: "badge badge-behind-latest"},
		{status: "unknown_version", want: "badge badge-unknown-version"},
		{status: "deprecated_version", want: "badge badge-deprecated-version"},
		{status: "potentially_affected_by_breaking_change", want: "badge badge-runtime-impact"},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			if got := statusBadgeClass(tt.status); got != tt.want {
				t.Fatalf("class = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRuntimeValuesAreHTMLEscaped(t *testing.T) {
	query := newFakeQuery()
	details := query.runtimeDetails["billing-service"]
	details.ServiceName = `<b>service</b>`
	details.Deployments[0].ServiceName = `<b>service</b>`
	details.Deployments[0].Environment = `<i>prod</i>`
	details.Deployments[0].BuildVersion = `<b>build</b>`
	details.Usages[0].DriftReason = `<script>alert(1)</script>`
	query.runtimeDetails["billing-service"] = details
	handler := newTestHandler(t, query)
	res := request(t, handler, "/ui/runtime/services/billing-service")

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d", res.Code)
	}
	body := res.Body.String()
	if strings.Contains(body, `<b>service</b>`) || strings.Contains(body, `<i>prod</i>`) || strings.Contains(body, `<b>build</b>`) || strings.Contains(body, `<script>`) {
		t.Fatalf("body rendered raw runtime HTML: %s", body)
	}
	if !strings.Contains(body, `&lt;b&gt;service&lt;/b&gt;`) || !strings.Contains(body, `&lt;i&gt;prod&lt;/i&gt;`) || !strings.Contains(body, `&lt;b&gt;build&lt;/b&gt;`) || !strings.Contains(body, `&lt;script&gt;alert(1)&lt;/script&gt;`) {
		t.Fatalf("body missing escaped runtime values: %s", body)
	}
}

func TestRuntimePagesRenderDeprecatedVersionStatus(t *testing.T) {
	query := newFakeQuery()
	details := query.runtimeDetails["billing-service"]
	details.Usages[0].DriftStatus = "deprecated_version"
	details.Usages[0].DriftReason = "deprecated_version"
	query.runtimeDetails["billing-service"] = details
	inventory := query.runtimeEnvironments["production"]
	inventory.Usages[0].DriftStatus = "deprecated_version"
	inventory.Usages[0].DriftReason = "deprecated_version"
	query.runtimeEnvironments["production"] = inventory
	moduleUsages := query.moduleRuntimeUsages["user-api"]
	moduleUsages.Usages[0].DriftStatus = "deprecated_version"
	moduleUsages.Usages[0].DriftReason = "deprecated_version"
	query.moduleRuntimeUsages["user-api"] = moduleUsages
	handler := newTestHandler(t, query)

	for _, path := range []string{
		"/ui/runtime/services/billing-service",
		"/ui/runtime/environments/production",
		"/ui/modules/user-api/runtime-usages",
	} {
		res := request(t, handler, path)
		if res.Code != http.StatusOK {
			t.Fatalf("%s status = %d", path, res.Code)
		}
		if !strings.Contains(res.Body.String(), "deprecated_version") || !strings.Contains(res.Body.String(), "badge-deprecated-version") {
			t.Fatalf("%s missing deprecated status: %s", path, res.Body.String())
		}
	}
}

func TestMissingRuntimeServiceReturnsNotFound(t *testing.T) {
	handler := newTestHandler(t, newFakeQuery())
	res := request(t, handler, "/ui/runtime/services/missing-service")

	if res.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusNotFound)
	}
	if !strings.Contains(res.Body.String(), "Runtime service not found") {
		t.Fatalf("body = %s", res.Body.String())
	}
}
