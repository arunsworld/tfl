package handlers

import (
	"net/http"
	"text/template"

	"github.com/arunsworld/tfl"
	"github.com/gorilla/mux"
)

func (h handlers) registerLinesHandler() {
	linesGET := h.handler.PathPrefix("/lines/").Methods("GET").Subrouter()
	linesGET.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/lines/tube", 302)
	})
	linesGET.HandleFunc("/{mode}", func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		mode := vars["mode"]
		lines := tfl.TFLStaticDataGlobal.Lines(mode)
		if len(lines) == 0 {
			handleEmptyLines(w, h.tmpls)
			return
		}
		err := h.tmpls.ExecuteTemplate(w, "lines.html", splitIntoTabularFormat(lines, 3))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html")
	})
}

func handleEmptyLines(w http.ResponseWriter, tmpls *template.Template) {
	err := tmpls.ExecuteTemplate(w, "lines-empty.html", nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html")
}
