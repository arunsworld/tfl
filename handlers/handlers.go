package handlers

import (
	"html/template"
	"net/http"

	"io/fs"

	"github.com/gorilla/mux"
	"github.com/unrolled/logger"
)

func RegisterHandlers(handler *mux.Router, static fs.FS, templates fs.FS) {
	h := handlers{
		handler: handler,
	}
	tmpls := template.New("").Delims("[[", "]]").Funcs(template.FuncMap{
		"htmlSafe": func(v string) template.HTML {
			return template.HTML(v)
		},
	})
	h.tmpls = template.Must(tmpls.ParseFS(templates, "*.html"))

	l := logger.New()
	handler.Use(l.Handler)

	h.registerStatic(static)
	h.registerIndex()
	h.registerLinesHandler()
	h.registerVehicleHandler()
	h.registerStationsHandler()
}

type handlers struct {
	handler *mux.Router
	tmpls   *template.Template
}

func (h handlers) registerIndex() {
	h.handler.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/lines/", 302)
	})
}
