package web

import (
	"context"
	"embed"
	"fmt"
	"html/template"
	"net/http"

	"github.com/alryzden/ProtoRadar/internal/usecase/uiquery"
)

//go:embed templates/*.html templates/components/*.html static/app.css
var assets embed.FS

type Query interface {
	GetEdition(ctx context.Context, input uiquery.GetEditionInput) (uiquery.EditionDetails, error)
	ListModuleOverviews(ctx context.Context, input uiquery.ListModuleOverviewsInput) ([]uiquery.ModuleOverview, error)
	GetModuleOverview(ctx context.Context, input uiquery.GetModuleOverviewInput) (uiquery.ModuleOverview, error)
	GetVersionOverview(ctx context.Context, input uiquery.GetVersionOverviewInput) (uiquery.VersionOverview, error)
	GetModuleDependencyGraph(ctx context.Context, input uiquery.GetModuleDependencyGraphInput) (uiquery.ModuleDependencyGraph, error)
	ListBreakingReportOverviews(ctx context.Context, input uiquery.ListBreakingReportOverviewsInput) ([]uiquery.BreakingReportSummary, error)
	GetBreakingReportDetails(ctx context.Context, input uiquery.GetBreakingReportDetailsInput) (uiquery.BreakingReportDetails, error)
	GetApprovalRequestDetails(ctx context.Context, input uiquery.GetApprovalRequestDetailsInput) (uiquery.ApprovalRequestDetails, error)
	ListRuntimeServices(ctx context.Context, input uiquery.ListRuntimeServicesInput) ([]uiquery.RuntimeServiceSummary, error)
	GetRuntimeServiceDetails(ctx context.Context, input uiquery.GetRuntimeServiceDetailsInput) (uiquery.RuntimeServiceDetails, error)
	GetRuntimeEnvironmentInventory(ctx context.Context, input uiquery.GetRuntimeEnvironmentInventoryInput) (uiquery.RuntimeEnvironmentInventory, error)
	GetModuleRuntimeUsages(ctx context.Context, input uiquery.GetModuleRuntimeUsagesInput) (uiquery.ModuleRuntimeUsages, error)
}

type Server struct {
	basePath   string
	staticPath string
	query      Query
	templates  *template.Template
}

type Options struct {
	BasePath   string
	StaticPath string
	Query      Query
}

func NewServer(options Options) (*Server, error) {
	basePath := normalizePath(options.BasePath, "/ui")
	staticPath := normalizePath(options.StaticPath, basePath+"/static")
	templates, err := template.New("").Funcs(template.FuncMap{
		"badgeClass": statusBadgeClass,
		"badgeText":  statusBadgeText,
	}).ParseFS(assets, "templates/*.html", "templates/components/*.html")
	if err != nil {
		return nil, fmt.Errorf("parse web templates: %w", err)
	}
	return &Server{
		basePath:   basePath,
		staticPath: staticPath,
		query:      options.Query,
		templates:  templates,
	}, nil
}

func (server *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET "+server.basePath, server.index)
	mux.HandleFunc("GET "+server.basePath+"/modules", server.modules)
	mux.HandleFunc("GET "+server.basePath+"/modules/{module}", server.moduleDetail)
	mux.HandleFunc("GET "+server.basePath+"/modules/{module}/dependencies", server.moduleDependencies)
	mux.HandleFunc("GET "+server.basePath+"/modules/{module}/runtime-usages", server.moduleRuntimeUsages)
	mux.HandleFunc("GET "+server.basePath+"/modules/{module}/versions/{version}", server.versionDetail)
	mux.HandleFunc("GET "+server.basePath+"/runtime/services", server.runtimeServices)
	mux.HandleFunc("GET "+server.basePath+"/runtime/services/{service}", server.runtimeServiceDetail)
	mux.HandleFunc("GET "+server.basePath+"/runtime/environments/{environment}", server.runtimeEnvironment)
	mux.HandleFunc("GET "+server.basePath+"/breaking-reports", server.breakingReports)
	mux.HandleFunc("GET "+server.basePath+"/breaking-reports/{report_id}", server.breakingReportDetail)
	mux.HandleFunc("GET "+server.basePath+"/approval-requests/{request_id}", server.approvalRequestDetail)
	mux.HandleFunc("GET "+server.basePath+"/about", server.about)
	mux.HandleFunc("GET "+server.staticPath+"/app.css", server.stylesheet)
	mux.HandleFunc("GET "+server.basePath+"/{path...}", server.notFound)
	return mux
}

func (server *Server) index(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, server.basePath+"/modules", http.StatusFound)
}
