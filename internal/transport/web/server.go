package web

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/alryzden/ProtoRadar/internal/domain"
	"github.com/alryzden/ProtoRadar/internal/usecase/uiquery"
)

//go:embed templates/*.html templates/components/*.html static/app.css
var assets embed.FS

type Query interface {
	ListModuleOverviews(ctx context.Context, input uiquery.ListModuleOverviewsInput) ([]uiquery.ModuleOverview, error)
	GetModuleOverview(ctx context.Context, input uiquery.GetModuleOverviewInput) (uiquery.ModuleOverview, error)
	GetVersionOverview(ctx context.Context, input uiquery.GetVersionOverviewInput) (uiquery.VersionOverview, error)
	GetModuleDependencyGraph(ctx context.Context, input uiquery.GetModuleDependencyGraphInput) (uiquery.ModuleDependencyGraph, error)
	ListBreakingReportOverviews(ctx context.Context, input uiquery.ListBreakingReportOverviewsInput) ([]uiquery.BreakingReportSummary, error)
	GetBreakingReportDetails(ctx context.Context, input uiquery.GetBreakingReportDetailsInput) (uiquery.BreakingReportDetails, error)
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

type pageData struct {
	Title          string
	Active         string
	BasePath       string
	StaticPath     string
	Status         string
	Module         string
	Version        string
	ReportID       string
	Message        string
	EmptyTitle     string
	EmptyBody      string
	Query          string
	Modules        []moduleRow
	ModuleView     *moduleDetailView
	DependencyView *moduleDependencyView
	VersionView    *versionDetailView
	ReportFilters  reportFilters
	Reports        []reportRow
	ReportView     *breakingReportDetailView
}

type moduleRow struct {
	Name                string
	Description         string
	RepositoryURL       string
	GitLabProjectURL    string
	GitLabProjectLabel  string
	LatestVersion       string
	VersionCount        int
	LastPublishedOrSeen string
	BreakingReportCount int
	LastBreakingStatus  string
	HasLatestVersion    bool
	HasGitLabProject    bool
	HasRepositoryURL    bool
	HasLastBreaking     bool
	ModuleURL           string
	LatestVersionURL    string
	FilteredReportsURL  string
}

type moduleDetailView struct {
	Name               string
	Description        string
	RepositoryURL      string
	GitLabProjectURL   string
	GitLabProjectLabel string
	LatestVersion      string
	CreatedAt          string
	UpdatedAt          string
	HasRepositoryURL   bool
	HasGitLabProject   bool
	HasLatestVersion   bool
	Versions           []versionRow
	RecentReports      []reportRow
	FilteredReportsURL string
	DependencyGraphURL string
}

type moduleDependencyView struct {
	ModuleName    string
	ModuleURL     string
	Downstream    []dependencyModuleRow
	Upstream      []dependencyModuleRow
	Unresolved    []unresolvedDependencyRow
	HasDownstream bool
	HasUpstream   bool
	HasUnresolved bool
}

type dependencyModuleRow struct {
	Module            string
	Version           string
	DependencySources string
	Reasons           string
	ModuleURL         string
}

type unresolvedDependencyRow struct {
	Source           string
	ImportPath       string
	ReferencedSymbol string
	Reason           string
}

type versionRow struct {
	Version            string
	Status             string
	CreatedAt          string
	SourceDigest       string
	SourceDigestFull   string
	BufImageDigest     string
	BufImageDigestFull string
	LintStatus         string
	Files              int
	Services           int
	Methods            int
	Messages           int
	Enums              int
	VersionURL         string
	HasSourceDigest    bool
	HasBufImageDigest  bool
	HasLintStatus      bool
}

type reportRow struct {
	ID             string
	Module         string
	CreatedAt      string
	BaseVersion    string
	TargetRef      string
	Status         string
	ChangeCount    int
	ReportURL      string
	ModuleURL      string
	BaseVersionURL string
}

type versionDetailView struct {
	ModuleURL         string
	ModulesURL        string
	ModuleName        string
	Version           string
	Status            string
	CreatedAt         string
	LintStatus        string
	CompileStatus     string
	MetadataCounts    metadataCounts
	Artifacts         []artifactRow
	BufConfig         bufConfigView
	ProtoFiles        []protoFileRow
	Imports           []importRow
	Methods           []methodRow
	Fields            []fieldRow
	EnumValues        []enumValueRow
	RelatedReports    []reportRow
	HasLintStatus     bool
	HasMetadata       bool
	HasArtifacts      bool
	HasRelatedReports bool
	HasModulePaths    bool
	HasDeps           bool
}

type metadataCounts struct {
	Files      int
	Packages   int
	Imports    int
	Services   int
	Methods    int
	Messages   int
	Fields     int
	Enums      int
	EnumValues int
}

type artifactRow struct {
	Kind           string
	Checksum       string
	ChecksumFull   string
	SizeBytes      int64
	DownloadURL    string
	HasChecksum    bool
	HasDownloadURL bool
}

type bufConfigView struct {
	ConfigPresent         string
	LockPresent           string
	ModulePaths           []string
	Deps                  []string
	LintEnabled           string
	LintStatus            string
	BreakingConfigPresent string
	HasLintStatus         bool
}

type protoFileRow struct {
	Path        string
	PackageName string
	Syntax      string
	ImportCount int
}

type importRow struct {
	FilePath   string
	ImportPath string
	Public     string
	Weak       string
}

type methodRow struct {
	FilePath        string
	ServiceFullName string
	Name            string
	InputType       string
	OutputType      string
	ClientStreaming string
	ServerStreaming string
}

type fieldRow struct {
	FilePath        string
	MessageFullName string
	Number          int32
	Name            string
	Type            string
	TypeName        string
	Label           string
	Repeated        string
	Map             string
}

type enumValueRow struct {
	FilePath     string
	EnumFullName string
	Name         string
	Number       int32
}

type reportFilters struct {
	Module     string
	Status     string
	Query      string
	HasFilters bool
}

type breakingReportDetailView struct {
	ID                 string
	Module             string
	BaseVersion        string
	TargetRef          string
	Status             string
	ChangeCount        int
	CreatedAt          string
	HumanSummary       string
	ModuleURL          string
	BaseVersionURL     string
	ReportsURL         string
	Changes            []changeRow
	AffectedModules    []dependencyModuleRow
	HasChanges         bool
	HasAffectedModules bool
}

type changeRow struct {
	FilePath    string
	PackageName string
	Symbol      string
	RuleID      string
	Message     string
	Severity    string
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
	mux.HandleFunc("GET "+server.basePath+"/modules/{module}/versions/{version}", server.versionDetail)
	mux.HandleFunc("GET "+server.basePath+"/breaking-reports", server.breakingReports)
	mux.HandleFunc("GET "+server.basePath+"/breaking-reports/{report_id}", server.breakingReportDetail)
	mux.HandleFunc("GET "+server.staticPath+"/app.css", server.stylesheet)
	mux.HandleFunc("GET "+server.basePath+"/{path...}", server.notFound)
	return mux
}

func (server *Server) moduleDependencies(w http.ResponseWriter, r *http.Request) {
	if server.query == nil {
		server.RenderError(w, http.StatusInternalServerError, "")
		return
	}
	moduleName := strings.TrimSpace(r.PathValue("module"))
	graph, err := server.query.GetModuleDependencyGraph(r.Context(), uiquery.GetModuleDependencyGraphInput{Module: moduleName})
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) || errors.Is(err, domain.ErrInvalidModuleName) {
			server.RenderError(w, http.StatusNotFound, "Module not found.")
			return
		}
		server.RenderError(w, http.StatusInternalServerError, "")
		return
	}
	view := server.moduleDependencyView(graph)
	server.render(w, http.StatusOK, "module_dependencies.html", pageData{
		Title:          "Dependencies " + view.ModuleName,
		Active:         "modules",
		Module:         view.ModuleName,
		Status:         "unknown",
		Message:        "Inspect downstream consumers, upstream dependencies, and unresolved protobuf references.",
		DependencyView: &view,
	})
}

func (server *Server) index(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, server.basePath+"/modules", http.StatusFound)
}

func (server *Server) modules(w http.ResponseWriter, r *http.Request) {
	if server.query == nil {
		server.RenderError(w, http.StatusInternalServerError, "")
		return
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	modules, err := server.query.ListModuleOverviews(r.Context(), uiquery.ListModuleOverviewsInput{Query: q})
	if err != nil {
		server.RenderError(w, http.StatusInternalServerError, "")
		return
	}
	rows := make([]moduleRow, 0, len(modules))
	for _, module := range modules {
		rows = append(rows, server.moduleRow(module))
	}

	emptyTitle := "No modules published yet."
	emptyBody := "Create one with: protoradar module create <module> --description \"...\" --repository-url <url>"
	if q != "" {
		emptyTitle = "No modules match this search."
		emptyBody = "Try a different module name, repository URL, or description."
	}
	server.render(w, http.StatusOK, "modules.html", pageData{
		Title:      "Modules",
		Active:     "modules",
		Status:     "unknown",
		Message:    "Browse published protobuf modules and their latest compatibility state.",
		EmptyTitle: emptyTitle,
		EmptyBody:  emptyBody,
		Query:      q,
		Modules:    rows,
	})
}

func (server *Server) moduleDetail(w http.ResponseWriter, r *http.Request) {
	if server.query == nil {
		server.RenderError(w, http.StatusInternalServerError, "")
		return
	}
	moduleName := strings.TrimSpace(r.PathValue("module"))
	overview, err := server.query.GetModuleOverview(r.Context(), uiquery.GetModuleOverviewInput{Module: moduleName})
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) || errors.Is(err, domain.ErrInvalidModuleName) {
			server.RenderError(w, http.StatusNotFound, "Module not found.")
			return
		}
		server.RenderError(w, http.StatusInternalServerError, "")
		return
	}
	view := server.moduleDetailView(overview)
	server.render(w, http.StatusOK, "module_detail.html", pageData{
		Title:      "Module " + view.Name,
		Active:     "modules",
		Module:     view.Name,
		Status:     moduleStatus(overview),
		Message:    "Inspect module ownership, published versions, and recent breaking reports.",
		EmptyTitle: "No versions published yet.",
		EmptyBody:  "Publish a version with: protoradar push " + view.Name + " --version v1.0.0 --path .",
		ModuleView: &view,
	})
}

func (server *Server) versionDetail(w http.ResponseWriter, r *http.Request) {
	if server.query == nil {
		server.RenderError(w, http.StatusInternalServerError, "")
		return
	}
	moduleName := strings.TrimSpace(r.PathValue("module"))
	versionValue := strings.TrimSpace(r.PathValue("version"))
	overview, err := server.query.GetVersionOverview(r.Context(), uiquery.GetVersionOverviewInput{
		Module:  moduleName,
		Version: versionValue,
	})
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) || errors.Is(err, domain.ErrInvalidModuleName) || errors.Is(err, domain.ErrInvalidVersion) {
			server.RenderError(w, http.StatusNotFound, "Module version not found.")
			return
		}
		server.RenderError(w, http.StatusInternalServerError, "")
		return
	}
	view := server.versionDetailView(overview)
	server.render(w, http.StatusOK, "version_detail.html", pageData{
		Title:       view.ModuleName + " " + view.Version,
		Active:      "modules",
		Module:      view.ModuleName,
		Version:     view.Version,
		Status:      view.Status,
		Message:     "Inspect artifacts, Buf configuration, descriptor metadata, and reports for this published version.",
		EmptyTitle:  "No descriptor metadata stored for this version.",
		EmptyBody:   "Publish the version again with descriptor extraction enabled to populate protobuf metadata.",
		VersionView: &view,
	})
}

func (server *Server) breakingReports(w http.ResponseWriter, r *http.Request) {
	if server.query == nil {
		server.RenderError(w, http.StatusInternalServerError, "")
		return
	}
	filters := reportFilters{
		Module: strings.TrimSpace(r.URL.Query().Get("module")),
		Status: strings.TrimSpace(r.URL.Query().Get("status")),
		Query:  strings.TrimSpace(r.URL.Query().Get("q")),
	}
	filters.HasFilters = filters.Module != "" || filters.Status != "" || filters.Query != ""
	reports, err := server.query.ListBreakingReportOverviews(r.Context(), uiquery.ListBreakingReportOverviewsInput{
		Module: filters.Module,
		Status: filters.Status,
		Query:  filters.Query,
	})
	if err != nil {
		server.RenderError(w, http.StatusInternalServerError, "")
		return
	}
	rows := make([]reportRow, 0, len(reports))
	for _, report := range reports {
		rows = append(rows, server.reportRow(report))
	}

	emptyTitle := "No breaking reports yet."
	emptyBody := "Run a breaking check to create reports."
	if filters.HasFilters {
		emptyTitle = "No breaking reports match these filters."
		emptyBody = "Try a different module, status, or search query."
	}
	server.render(w, http.StatusOK, "breaking_reports.html", pageData{
		Title:         "Breaking Reports",
		Active:        "breaking-reports",
		Status:        "unknown",
		Message:       "Review compatibility checks across modules and target refs.",
		EmptyTitle:    emptyTitle,
		EmptyBody:     emptyBody,
		ReportFilters: filters,
		Reports:       rows,
	})
}

func (server *Server) breakingReportDetail(w http.ResponseWriter, r *http.Request) {
	if server.query == nil {
		server.RenderError(w, http.StatusInternalServerError, "")
		return
	}
	reportID := strings.TrimSpace(r.PathValue("report_id"))
	details, err := server.query.GetBreakingReportDetails(r.Context(), uiquery.GetBreakingReportDetailsInput{ReportID: reportID})
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			server.RenderError(w, http.StatusNotFound, "Breaking report not found.")
			return
		}
		server.RenderError(w, http.StatusInternalServerError, "")
		return
	}
	view := server.breakingReportDetailView(details)
	server.render(w, http.StatusOK, "breaking_report_detail.html", pageData{
		Title:      "Breaking Report " + view.ID,
		Active:     "breaking-reports",
		ReportID:   view.ID,
		Status:     view.Status,
		Message:    "Inspect the summary and individual protobuf compatibility changes.",
		EmptyTitle: "No breaking changes recorded.",
		EmptyBody:  "This report did not record individual breaking-change rows.",
		ReportView: &view,
	})
}

func (server *Server) stylesheet(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/css; charset=utf-8")
	http.ServeFileFS(w, r, assets, "static/app.css")
}

func (server *Server) notFound(w http.ResponseWriter, r *http.Request) {
	server.RenderError(w, http.StatusNotFound, "The requested ProtoRadar UI page was not found.")
}

func (server *Server) render(w http.ResponseWriter, status int, name string, data pageData) {
	data.BasePath = server.basePath
	data.StaticPath = server.staticPath
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := server.templates.ExecuteTemplate(w, name, data); err != nil {
		_, _ = w.Write([]byte("Internal server error"))
	}
}

func (server *Server) RenderError(w http.ResponseWriter, status int, message string) {
	if status == http.StatusNotFound {
		if strings.TrimSpace(message) == "" {
			message = "The requested ProtoRadar UI page was not found."
		}
		server.render(w, status, "error.html", pageData{
			Title:   "Page Not Found",
			Active:  "",
			Status:  http.StatusText(status),
			Message: message,
		})
		return
	}
	server.render(w, http.StatusInternalServerError, "error.html", pageData{
		Title:   "Internal Server Error",
		Active:  "",
		Status:  http.StatusText(http.StatusInternalServerError),
		Message: "Something went wrong while rendering the ProtoRadar UI. Check server logs for details.",
	})
}

func (server *Server) moduleRow(overview uiquery.ModuleOverview) moduleRow {
	row := moduleRow{
		Name:                overview.Module.Name,
		Description:         overview.Module.Description,
		RepositoryURL:       overview.Module.RepositoryURL,
		VersionCount:        overview.VersionCount,
		LastPublishedOrSeen: formatTime(overview.LastPublishedOrSeen),
		BreakingReportCount: overview.BreakingReportCount,
		LastBreakingStatus:  overview.LastBreakingStatus,
		HasRepositoryURL:    strings.TrimSpace(overview.Module.RepositoryURL) != "",
		HasLastBreaking:     strings.TrimSpace(overview.LastBreakingStatus) != "",
		ModuleURL:           server.basePath + "/modules/" + pathEscape(overview.Module.Name),
		FilteredReportsURL:  server.basePath + "/breaking-reports?module=" + url.QueryEscape(overview.Module.Name),
	}
	if overview.LatestVersion != nil {
		row.LatestVersion = overview.LatestVersion.Version
		row.HasLatestVersion = true
		row.LatestVersionURL = server.basePath + "/modules/" + pathEscape(overview.Module.Name) + "/versions/" + pathEscape(overview.LatestVersion.Version)
	}
	if overview.GitLabProject != nil {
		row.HasGitLabProject = true
		row.GitLabProjectLabel = overview.GitLabProject.ProjectPath
		row.GitLabProjectURL = externalProjectURL(overview.GitLabProject.BaseURL, overview.GitLabProject.ProjectPath)
	}
	return row
}

func (server *Server) moduleDetailView(overview uiquery.ModuleOverview) moduleDetailView {
	view := moduleDetailView{
		Name:               overview.Module.Name,
		Description:        overview.Module.Description,
		RepositoryURL:      overview.Module.RepositoryURL,
		CreatedAt:          formatTime(overview.Module.CreatedAt),
		UpdatedAt:          formatTime(overview.Module.UpdatedAt),
		HasRepositoryURL:   strings.TrimSpace(overview.Module.RepositoryURL) != "",
		FilteredReportsURL: server.basePath + "/breaking-reports?module=" + url.QueryEscape(overview.Module.Name),
		DependencyGraphURL: server.basePath + "/modules/" + pathEscape(overview.Module.Name) + "/dependencies",
	}
	if overview.GitLabProject != nil {
		view.HasGitLabProject = true
		view.GitLabProjectLabel = overview.GitLabProject.ProjectPath
		view.GitLabProjectURL = externalProjectURL(overview.GitLabProject.BaseURL, overview.GitLabProject.ProjectPath)
	}
	if overview.LatestVersion != nil {
		view.HasLatestVersion = true
		view.LatestVersion = overview.LatestVersion.Version
	}
	for _, version := range overview.Versions {
		view.Versions = append(view.Versions, server.versionRow(overview.Module.Name, version))
	}
	for _, report := range overview.RecentReports {
		view.RecentReports = append(view.RecentReports, server.reportRow(report))
	}
	return view
}

func (server *Server) versionRow(moduleName string, version uiquery.VersionSummary) versionRow {
	row := versionRow{
		Version:    version.Version,
		Status:     version.Status,
		CreatedAt:  formatTime(version.CreatedAt),
		LintStatus: version.LintStatus,
		Files:      version.MetadataSummary.Files,
		Services:   version.MetadataSummary.Services,
		Methods:    version.MetadataSummary.Methods,
		Messages:   version.MetadataSummary.Messages,
		Enums:      version.MetadataSummary.Enums,
		VersionURL: server.basePath + "/modules/" + pathEscape(moduleName) + "/versions/" + pathEscape(version.Version),
	}
	row.HasLintStatus = strings.TrimSpace(row.LintStatus) != ""
	for _, artifact := range version.Artifacts {
		switch artifact.Kind {
		case "source_archive":
			row.HasSourceDigest = true
			row.SourceDigestFull = artifact.ChecksumSHA256
			row.SourceDigest = shortDigest(artifact.ChecksumSHA256)
		case "buf_image":
			row.HasBufImageDigest = true
			row.BufImageDigestFull = artifact.ChecksumSHA256
			row.BufImageDigest = shortDigest(artifact.ChecksumSHA256)
		}
	}
	return row
}

func (server *Server) reportRow(report uiquery.BreakingReportSummary) reportRow {
	row := reportRow{
		ID:          report.ID,
		Module:      report.Module,
		CreatedAt:   formatTime(report.CreatedAt),
		BaseVersion: report.BaseVersion,
		TargetRef:   report.TargetRef,
		Status:      report.Status,
		ChangeCount: report.ChangeCount,
		ReportURL:   server.basePath + "/breaking-reports/" + pathEscape(report.ID),
	}
	if strings.TrimSpace(row.Module) != "" {
		row.ModuleURL = server.basePath + "/modules/" + pathEscape(row.Module)
	}
	if strings.TrimSpace(row.Module) != "" && strings.TrimSpace(row.BaseVersion) != "" {
		row.BaseVersionURL = server.basePath + "/modules/" + pathEscape(row.Module) + "/versions/" + pathEscape(row.BaseVersion)
	}
	return row
}

func (server *Server) versionDetailView(overview uiquery.VersionOverview) versionDetailView {
	view := versionDetailView{
		ModuleURL:     server.basePath + "/modules/" + pathEscape(overview.Module.Name),
		ModulesURL:    server.basePath + "/modules",
		ModuleName:    overview.Module.Name,
		Version:       overview.Version.Version,
		Status:        overview.Version.Status,
		CreatedAt:     formatTime(overview.Version.CreatedAt),
		LintStatus:    overview.Version.LintStatus,
		CompileStatus: overview.Version.Status,
		MetadataCounts: metadataCounts{
			Files:      overview.MetadataCounts.Files,
			Packages:   overview.MetadataCounts.Packages,
			Imports:    overview.MetadataCounts.Imports,
			Services:   overview.MetadataCounts.Services,
			Methods:    overview.MetadataCounts.Methods,
			Messages:   overview.MetadataCounts.Messages,
			Fields:     overview.MetadataCounts.Fields,
			Enums:      overview.MetadataCounts.Enums,
			EnumValues: overview.MetadataCounts.EnumValues,
		},
		BufConfig: bufConfigView{
			ConfigPresent:         yesNo(overview.BufConfig.ConfigPresent),
			LockPresent:           yesNo(overview.BufConfig.LockPresent),
			ModulePaths:           overview.BufConfig.ModulePaths,
			Deps:                  overview.BufConfig.Deps,
			LintEnabled:           yesNo(overview.BufConfig.LintEnabled),
			LintStatus:            overview.BufConfig.LintStatus,
			BreakingConfigPresent: yesNo(overview.BufConfig.BreakingConfigPresent),
			HasLintStatus:         strings.TrimSpace(overview.BufConfig.LintStatus) != "",
		},
	}
	view.HasLintStatus = strings.TrimSpace(view.LintStatus) != ""
	view.HasModulePaths = len(view.BufConfig.ModulePaths) > 0
	view.HasDeps = len(view.BufConfig.Deps) > 0

	for _, artifact := range overview.Artifacts {
		view.Artifacts = append(view.Artifacts, server.artifactRow(overview.Module.Name, overview.Version.Version, artifact))
	}
	view.HasArtifacts = len(view.Artifacts) > 0

	for _, file := range overview.Metadata.Files {
		view.ProtoFiles = append(view.ProtoFiles, protoFileRow{
			Path:        file.Path,
			PackageName: file.PackageName,
			Syntax:      file.Syntax,
			ImportCount: len(file.Imports),
		})
		for _, item := range file.Imports {
			view.Imports = append(view.Imports, importRow{
				FilePath:   file.Path,
				ImportPath: item.Path,
				Public:     yesNo(item.Public),
				Weak:       yesNo(item.Weak),
			})
		}
		for _, service := range file.Services {
			for _, method := range service.Methods {
				view.Methods = append(view.Methods, methodRow{
					FilePath:        file.Path,
					ServiceFullName: service.FullName,
					Name:            method.Name,
					InputType:       method.InputType,
					OutputType:      method.OutputType,
					ClientStreaming: yesNo(method.ClientStreaming),
					ServerStreaming: yesNo(method.ServerStreaming),
				})
			}
		}
		for _, message := range file.Messages {
			appendMessageRows(&view.Fields, file.Path, message)
			appendMessageEnumRows(&view.EnumValues, file.Path, message)
		}
		for _, enum := range file.Enums {
			appendEnumRows(&view.EnumValues, file.Path, enum)
		}
	}
	view.HasMetadata = len(view.ProtoFiles) > 0

	for _, report := range overview.RelatedReports {
		view.RelatedReports = append(view.RelatedReports, server.reportRow(report))
	}
	view.HasRelatedReports = len(view.RelatedReports) > 0
	return view
}

func (server *Server) breakingReportDetailView(details uiquery.BreakingReportDetails) breakingReportDetailView {
	row := server.reportRow(details.Report)
	view := breakingReportDetailView{
		ID:             row.ID,
		Module:         row.Module,
		BaseVersion:    row.BaseVersion,
		TargetRef:      row.TargetRef,
		Status:         row.Status,
		ChangeCount:    row.ChangeCount,
		CreatedAt:      row.CreatedAt,
		HumanSummary:   details.Summary,
		ModuleURL:      row.ModuleURL,
		BaseVersionURL: row.BaseVersionURL,
		ReportsURL:     server.basePath + "/breaking-reports",
	}
	for _, change := range details.Changes {
		view.Changes = append(view.Changes, changeRow{
			FilePath:    change.FilePath,
			PackageName: change.PackageName,
			Symbol:      change.Symbol,
			RuleID:      change.RuleID,
			Message:     change.Message,
			Severity:    change.Severity,
		})
	}
	for _, item := range details.AffectedModules {
		view.AffectedModules = append(view.AffectedModules, server.dependencyModuleRow(item))
	}
	view.HasChanges = len(view.Changes) > 0
	view.HasAffectedModules = len(view.AffectedModules) > 0
	return view
}

func (server *Server) moduleDependencyView(graph uiquery.ModuleDependencyGraph) moduleDependencyView {
	view := moduleDependencyView{
		ModuleName: graph.Module.Name,
		ModuleURL:  server.basePath + "/modules/" + pathEscape(graph.Module.Name),
	}
	for _, item := range graph.Downstream {
		view.Downstream = append(view.Downstream, server.dependencyModuleRow(item))
	}
	for _, item := range graph.Upstream {
		view.Upstream = append(view.Upstream, server.dependencyModuleRow(item))
	}
	for _, item := range graph.Unresolved {
		view.Unresolved = append(view.Unresolved, unresolvedDependencyRow{
			Source:           item.Source,
			ImportPath:       item.ImportPath,
			ReferencedSymbol: item.ReferencedSymbol,
			Reason:           item.Reason,
		})
	}
	view.HasDownstream = len(view.Downstream) > 0
	view.HasUpstream = len(view.Upstream) > 0
	view.HasUnresolved = len(view.Unresolved) > 0
	return view
}

func (server *Server) dependencyModuleRow(item uiquery.DependencyModule) dependencyModuleRow {
	return dependencyModuleRow{
		Module:            item.Module,
		Version:           item.Version,
		DependencySources: strings.Join(item.DependencySources, ", "),
		Reasons:           strings.Join(item.Reasons, ", "),
		ModuleURL:         server.basePath + "/modules/" + pathEscape(item.Module),
	}
}

func (server *Server) artifactRow(moduleName string, version string, artifact uiquery.ArtifactSummary) artifactRow {
	row := artifactRow{
		Kind:         artifact.Kind,
		Checksum:     shortDigest(artifact.ChecksumSHA256),
		ChecksumFull: artifact.ChecksumSHA256,
		SizeBytes:    artifact.SizeBytes,
		HasChecksum:  strings.TrimSpace(artifact.ChecksumSHA256) != "",
	}
	if artifact.Kind == "source_archive" {
		row.DownloadURL = "/api/v1/modules/" + pathEscape(moduleName) + "/versions/" + pathEscape(version) + "/artifact"
		row.HasDownloadURL = true
	}
	return row
}

func appendMessageRows(rows *[]fieldRow, filePath string, message uiquery.ProtoMessage) {
	for _, field := range message.Fields {
		*rows = append(*rows, fieldRow{
			FilePath:        filePath,
			MessageFullName: message.FullName,
			Number:          field.Number,
			Name:            field.Name,
			Type:            field.Type,
			TypeName:        field.TypeName,
			Label:           field.Label,
			Repeated:        yesNo(field.IsRepeated),
			Map:             yesNo(field.IsMap),
		})
	}
	for _, nested := range message.Messages {
		appendMessageRows(rows, filePath, nested)
	}
}

func appendEnumRows(rows *[]enumValueRow, filePath string, enum uiquery.ProtoEnum) {
	for _, value := range enum.Values {
		*rows = append(*rows, enumValueRow{
			FilePath:     filePath,
			EnumFullName: enum.FullName,
			Name:         value.Name,
			Number:       value.Number,
		})
	}
}

func appendMessageEnumRows(rows *[]enumValueRow, filePath string, message uiquery.ProtoMessage) {
	for _, enum := range message.Enums {
		appendEnumRows(rows, filePath, enum)
	}
	for _, nested := range message.Messages {
		appendMessageEnumRows(rows, filePath, nested)
	}
}

func statusBadgeClass(status string) string {
	switch normalizeStatus(status) {
	case "published":
		return "badge badge-published"
	case "passed", "success":
		return "badge badge-passed"
	case "breaking":
		return "badge badge-breaking"
	case "failed", "failure", "error":
		return "badge badge-failed"
	case "warning", "warn":
		return "badge badge-warning"
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
