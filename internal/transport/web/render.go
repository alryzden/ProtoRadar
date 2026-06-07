package web

import (
	"net/http"
	"strings"
)

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
		if _, writeErr := w.Write([]byte("Internal server error")); writeErr != nil {
			return
		}
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
