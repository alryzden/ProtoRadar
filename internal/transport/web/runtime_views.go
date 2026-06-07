package web

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/alryzden/ProtoRadar/internal/domain"
	"github.com/alryzden/ProtoRadar/internal/usecase/uiquery"
)

func (server *Server) runtimeServices(w http.ResponseWriter, r *http.Request) {
	if server.query == nil {
		server.RenderError(w, http.StatusInternalServerError, "")
		return
	}
	filters := runtimeFilters{
		Query:       strings.TrimSpace(r.URL.Query().Get("q")),
		Environment: strings.TrimSpace(r.URL.Query().Get("environment")),
		DriftStatus: strings.TrimSpace(r.URL.Query().Get("drift_status")),
	}
	filters.HasFilters = filters.Query != "" || filters.Environment != "" || filters.DriftStatus != ""
	summaries, err := server.query.ListRuntimeServices(r.Context(), uiquery.ListRuntimeServicesInput{
		Query:       filters.Query,
		Environment: filters.Environment,
		DriftStatus: filters.DriftStatus,
	})
	if err != nil {
		server.RenderError(w, http.StatusInternalServerError, "")
		return
	}
	rows := make([]runtimeServiceRow, 0, len(summaries))
	for _, summary := range summaries {
		rows = append(rows, server.runtimeServiceRow(summary))
	}
	emptyTitle := "No runtime services have reported inventory yet."
	emptyBody := "Report deployed contract versions with: protoradar runtime report --service <service> --environment <env> --module <module>@<version>"
	if filters.HasFilters {
		emptyTitle = "No runtime services match these filters."
		emptyBody = "Try a different service, environment, or drift status."
	}
	server.render(w, http.StatusOK, "runtime_services.html", pageData{
		Title:           "Runtime Services",
		Active:          "runtime",
		Status:          "unknown",
		Message:         "Inspect services that reported deployed protobuf contract versions.",
		EmptyTitle:      emptyTitle,
		EmptyBody:       emptyBody,
		RuntimeFilters:  filters,
		RuntimeServices: rows,
	})
}
func (server *Server) runtimeServiceDetail(w http.ResponseWriter, r *http.Request) {
	if server.query == nil {
		server.RenderError(w, http.StatusInternalServerError, "")
		return
	}
	serviceName := strings.TrimSpace(r.PathValue("service"))
	details, err := server.query.GetRuntimeServiceDetails(r.Context(), uiquery.GetRuntimeServiceDetailsInput{Service: serviceName})
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) || errors.Is(err, domain.ErrInvalidRuntimeServiceName) {
			server.RenderError(w, http.StatusNotFound, "Runtime service not found.")
			return
		}
		server.RenderError(w, http.StatusInternalServerError, "")
		return
	}
	view := server.runtimeServiceDetailView(details)
	server.render(w, http.StatusOK, "runtime_service_detail.html", pageData{
		Title:              "Runtime Service " + view.ServiceName,
		Active:             "runtime",
		Status:             "unknown",
		Message:            "Inspect reported deployments and protobuf module usage for this runtime service.",
		EmptyTitle:         "No runtime deployments recorded for this service.",
		EmptyBody:          "Report inventory with: protoradar runtime report --service " + view.ServiceName + " --module <module>@<version>",
		RuntimeServiceView: &view,
	})
}

func (server *Server) runtimeEnvironment(w http.ResponseWriter, r *http.Request) {
	if server.query == nil {
		server.RenderError(w, http.StatusInternalServerError, "")
		return
	}
	environment := strings.TrimSpace(r.PathValue("environment"))
	inventory, err := server.query.GetRuntimeEnvironmentInventory(r.Context(), uiquery.GetRuntimeEnvironmentInventoryInput{Environment: environment})
	if err != nil {
		if errors.Is(err, domain.ErrInvalidRuntimeEnvironment) {
			server.RenderError(w, http.StatusNotFound, "Runtime environment not found.")
			return
		}
		server.RenderError(w, http.StatusInternalServerError, "")
		return
	}
	view := server.runtimeEnvironmentView(inventory)
	server.render(w, http.StatusOK, "runtime_environment.html", pageData{
		Title:                  "Runtime Environment " + view.Environment,
		Active:                 "runtime",
		Status:                 "unknown",
		Message:                "Inspect services currently reported in this runtime environment.",
		EmptyTitle:             "No runtime services are currently known in this environment.",
		EmptyBody:              "Runtime inventory appears after services report deployed module versions.",
		RuntimeEnvironmentView: &view,
	})
}

func (server *Server) moduleRuntimeUsages(w http.ResponseWriter, r *http.Request) {
	if server.query == nil {
		server.RenderError(w, http.StatusInternalServerError, "")
		return
	}
	moduleName := strings.TrimSpace(r.PathValue("module"))
	usages, err := server.query.GetModuleRuntimeUsages(r.Context(), uiquery.GetModuleRuntimeUsagesInput{Module: moduleName})
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) || errors.Is(err, domain.ErrInvalidModuleName) {
			server.RenderError(w, http.StatusNotFound, "Module not found.")
			return
		}
		server.RenderError(w, http.StatusInternalServerError, "")
		return
	}
	view := server.moduleRuntimeUsagesView(usages)
	server.render(w, http.StatusOK, "module_runtime_usages.html", pageData{
		Title:             "Runtime Usage " + view.Module,
		Active:            "modules",
		Module:            view.Module,
		Status:            "unknown",
		Message:           "Inspect runtime services that report using this module.",
		EmptyTitle:        "No runtime services are currently known to use this module.",
		EmptyBody:         "Runtime usage appears after deployed services report inventory.",
		ModuleRuntimeView: &view,
	})
}
func (server *Server) runtimeServiceRow(summary uiquery.RuntimeServiceSummary) runtimeServiceRow {
	links := make([]runtimeEnvironmentLink, 0, len(summary.Environments))
	for _, environment := range summary.Environments {
		links = append(links, runtimeEnvironmentLink{
			Name: environment,
			URL:  server.basePath + "/runtime/environments/" + pathEscape(environment),
		})
	}
	return runtimeServiceRow{
		ServiceName:         summary.ServiceName,
		Environments:        strings.Join(summary.Environments, ", "),
		LastReportedAt:      formatTimePtr(summary.LastReportedAt),
		UpToDateCount:       summary.UpToDateCount,
		BehindLatestCount:   summary.BehindLatestCount,
		UnknownVersionCount: summary.UnknownVersionCount,
		DeprecatedCount:     summary.DeprecatedCount,
		ServiceURL:          server.basePath + "/runtime/services/" + pathEscape(summary.ServiceName),
		EnvironmentURLs:     links,
	}
}

func (server *Server) runtimeServiceDetailView(details uiquery.RuntimeServiceDetails) runtimeServiceDetailView {
	view := runtimeServiceDetailView{
		ServiceName: details.ServiceName,
		ServicesURL: server.basePath + "/runtime/services",
		Deployments: server.runtimeDeploymentViews(details.Deployments, details.Usages),
	}
	view.HasDeployments = len(view.Deployments) > 0
	return view
}

func (server *Server) runtimeEnvironmentView(inventory uiquery.RuntimeEnvironmentInventory) runtimeEnvironmentView {
	view := runtimeEnvironmentView{
		Environment: inventory.Environment,
		ServicesURL: server.basePath + "/runtime/services?environment=" + url.QueryEscape(inventory.Environment),
		Deployments: server.runtimeDeploymentViews(inventory.Deployments, inventory.Usages),
	}
	view.HasDeployments = len(view.Deployments) > 0
	return view
}

func (server *Server) runtimeDeploymentViews(deployments []uiquery.RuntimeDeployment, usages []uiquery.RuntimeModuleUsage) []runtimeDeploymentView {
	usagesByDeployment := map[string][]runtimeUsageRow{}
	for _, usage := range usages {
		usagesByDeployment[usage.DeploymentID] = append(usagesByDeployment[usage.DeploymentID], server.runtimeUsageRow(usage))
	}
	items := make([]runtimeDeploymentView, 0, len(deployments))
	for _, deployment := range deployments {
		usageRows := usagesByDeployment[deployment.ID]
		items = append(items, runtimeDeploymentView{
			ID:             deployment.ID,
			ServiceName:    deployment.ServiceName,
			ServiceURL:     server.basePath + "/runtime/services/" + pathEscape(deployment.ServiceName),
			Environment:    deployment.Environment,
			EnvironmentURL: server.basePath + "/runtime/environments/" + pathEscape(deployment.Environment),
			GitCommit:      deployment.GitCommit,
			BuildVersion:   deployment.BuildVersion,
			ReportedAt:     formatTime(deployment.ReportedAt),
			Usages:         usageRows,
			HasUsages:      len(usageRows) > 0,
		})
	}
	return items
}

func (server *Server) runtimeUsageRow(usage uiquery.RuntimeModuleUsage) runtimeUsageRow {
	return runtimeUsageRow{
		Module:        usage.Module,
		Version:       usage.Version,
		LatestVersion: usage.LatestVersion,
		DriftStatus:   usage.DriftStatus,
		DriftReason:   usage.DriftReason,
		ModuleURL:     server.basePath + "/modules/" + pathEscape(usage.Module),
	}
}

func (server *Server) moduleRuntimeUsagesView(usages uiquery.ModuleRuntimeUsages) moduleRuntimeUsagesView {
	view := moduleRuntimeUsagesView{
		Module:    usages.Module,
		ModuleURL: server.basePath + "/modules/" + pathEscape(usages.Module),
	}
	for _, usage := range usages.Usages {
		view.Usages = append(view.Usages, moduleRuntimeUsageRow{
			ServiceName:    usage.ServiceName,
			ServiceURL:     server.basePath + "/runtime/services/" + pathEscape(usage.ServiceName),
			Environment:    usage.Environment,
			EnvironmentURL: server.basePath + "/runtime/environments/" + pathEscape(usage.Environment),
			Version:        usage.Version,
			LatestVersion:  usage.LatestVersion,
			DriftStatus:    usage.DriftStatus,
			DriftReason:    usage.DriftReason,
			ReportedAt:     formatTime(usage.ReportedAt),
		})
	}
	view.HasUsages = len(view.Usages) > 0
	return view
}

func (server *Server) runtimeImpactRow(impact uiquery.RuntimeImpact) runtimeImpactRow {
	return runtimeImpactRow{
		ServiceName:    impact.ServiceName,
		ServiceURL:     server.basePath + "/runtime/services/" + pathEscape(impact.ServiceName),
		Environment:    impact.Environment,
		EnvironmentURL: server.basePath + "/runtime/environments/" + pathEscape(impact.Environment),
		UsedVersion:    impact.UsedVersion,
		BuildVersion:   impact.BuildVersion,
		GitCommit:      impact.GitCommit,
		ReportedAt:     formatTime(impact.ReportedAt),
		ImpactStatus:   impact.ImpactStatus,
		Reason:         impact.Reason,
		DriftStatus:    impact.DriftStatus,
		DriftReason:    impact.DriftReason,
	}
}
