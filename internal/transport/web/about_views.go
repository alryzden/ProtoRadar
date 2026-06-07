package web

import (
	"net/http"
	"strings"

	"github.com/alryzden/ProtoRadar/internal/usecase/uiquery"
)

func (server *Server) about(w http.ResponseWriter, r *http.Request) {
	if server.query == nil {
		server.RenderError(w, http.StatusInternalServerError, "")
		return
	}
	details, err := server.query.GetEdition(r.Context(), uiquery.GetEditionInput{})
	if err != nil {
		server.RenderError(w, http.StatusInternalServerError, "")
		return
	}
	view := aboutViewFromEdition(details)
	server.render(w, http.StatusOK, "about.html", pageData{
		Title:     "About",
		Active:    "about",
		Status:    details.Edition,
		Message:   "Edition, build metadata, and available product capabilities.",
		AboutView: &view,
	})
}

func aboutViewFromEdition(details uiquery.EditionDetails) aboutView {
	view := aboutView{
		Edition:             details.Edition,
		Version:             details.Version,
		Commit:              details.Commit,
		BuildDate:           details.BuildDate,
		HasCommit:           strings.TrimSpace(details.Commit) != "" && details.Commit != "unknown",
		HasBuildDate:        strings.TrimSpace(details.BuildDate) != "" && details.BuildDate != "unknown",
		DisplayEditionTitle: "ProtoRadar Community Edition",
	}
	if strings.TrimSpace(view.Edition) != "community" {
		view.DisplayEditionTitle = "ProtoRadar " + view.Edition + " Edition"
	}
	for _, capability := range details.Capabilities {
		name := strings.TrimSpace(capability.Name)
		if name == "" {
			continue
		}
		row := capabilityRow{Name: name}
		if capability.Enabled {
			view.Enabled = append(view.Enabled, row)
		} else {
			view.Unavailable = append(view.Unavailable, row)
		}
	}
	view.HasEnabled = len(view.Enabled) > 0
	view.HasUnavailable = len(view.Unavailable) > 0
	return view
}
