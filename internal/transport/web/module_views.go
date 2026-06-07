package web

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/alryzden/ProtoRadar/internal/domain"
	"github.com/alryzden/ProtoRadar/internal/usecase/uiquery"
)

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
		RuntimeUsagesURL:   server.basePath + "/modules/" + pathEscape(overview.Module.Name) + "/runtime-usages",
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
	for _, owner := range overview.Owners {
		view.Owners = append(view.Owners, moduleOwnerRow{
			ID:          owner.ID,
			SubjectType: owner.SubjectType,
			Subject:     owner.Subject,
			Role:        owner.Role,
			CreatedAt:   formatTime(owner.CreatedAt),
		})
	}
	view.HasOwners = len(view.Owners) > 0
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
