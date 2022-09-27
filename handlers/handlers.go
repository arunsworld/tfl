package handlers

import (
	"net/http"
	"text/template"

	"io/fs"

	"github.com/gorilla/mux"
	"github.com/unrolled/logger"
)

func RegisterHandlers(handler *mux.Router, static fs.FS, templates fs.FS) {
	h := handlers{
		handler: handler,
	}
	tmpls, err := template.New("").Delims("[[", "]]").ParseFS(templates, "*.html")
	if err != nil {
		panic(err)
	}
	h.tmpls = tmpls

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
