package handlers

import (
	"html/template"
	"net/http"

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
		lines := tfl.TFLAPIGlobal.Lines(mode, true)
		if len(lines) == 0 {
			handleEmptyLines(w, h.tmpls, mode)
			return
		}
		err := h.tmpls.ExecuteTemplate(w, "lines.html", struct {
			Mode  string
			Lines [][]tfl.Line
		}{
			Mode:  mode,
			Lines: splitIntoTabularFormat(lines, 3),
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html")
	})
}

func handleEmptyLines(w http.ResponseWriter, tmpls *template.Template, mode string) {
	err := tmpls.ExecuteTemplate(w, "lines-empty.html", struct {
		Mode string
	}{
		Mode: mode,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html")
}
