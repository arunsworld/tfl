package handlers

import (
	"net/http"
	"text/template"

	"github.com/arunsworld/tfl"
)

func (h handlers) registerLinesHandler() {
	linesGET := h.handler.PathPrefix("/lines/").Methods("GET").Subrouter()
	linesGET.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		lines := tfl.TFLStaticDataGlobal.Lines()
		if len(lines) == 0 {
			handleEmptyLines(w, h.tmpls)
			return
		}
		err := h.tmpls.ExecuteTemplate(w, "lines.html", splitLinesIntoTabularFormat(lines))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html")
	})
}

func splitLinesIntoTabularFormat(lines []tfl.Line) [][]interface{} {
	rowSplitThreshold := 3
	currentRow := 0
	result := [][]any{
		{},
	}
	for _, l := range lines {
		if len(result[currentRow]) == rowSplitThreshold {
			result = append(result, []any{})
			currentRow++
		}
		result[currentRow] = append(result[currentRow], l)
	}
	return result
}

func handleEmptyLines(w http.ResponseWriter, tmpls *template.Template) {
	err := tmpls.ExecuteTemplate(w, "lines-empty.html", nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html")
}
