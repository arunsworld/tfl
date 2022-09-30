package handlers

import (
	"html/template"
	"net/http"

	"github.com/arunsworld/tfl"
	"github.com/gorilla/mux"
)

func (h handlers) registerRoutesHandler() {
	routesGET := h.handler.PathPrefix("/routes/").Methods("GET").Subrouter()
	routesGET.HandleFunc("/{mode}/{line_id}", func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		mode := vars["mode"]
		lineID := vars["line_id"]
		routes := tfl.TFLStaticDataGlobal.Routes(lineID)
		lineDetails := tfl.TFLStaticDataGlobal.LineDetails(mode, lineID)
		navigation := "arrivals"
		err := h.tmpls.ExecuteTemplate(w, "routes.html", struct {
			Mode       string
			LineID     string
			LineName   string
			Routes     []tfl.Route
			Navigation string
		}{
			Mode:       mode,
			LineID:     lineID,
			LineName:   lineDetails.Name,
			Routes:     routes,
			Navigation: navigation,
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html")
	})
}

func handleStationDataRetreivalError(w http.ResponseWriter, tmpls *template.Template, mode, lid, sid string, nav string, errMsg string) {
	err := tmpls.ExecuteTemplate(w, "station-error.html", struct {
		Mode       string
		LineID     string
		StationID  string
		Error      string
		Navigation string
	}{
		Mode:       mode,
		LineID:     lid,
		StationID:  sid,
		Error:      errMsg,
		Navigation: nav,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html")
}

func handleStationDataNotFound(w http.ResponseWriter, tmpls *template.Template, mode, lid, sid string) {
	err := tmpls.ExecuteTemplate(w, "station-not-found.html", struct {
		Mode      string
		LineID    string
		StationID string
	}{
		Mode:      mode,
		LineID:    lid,
		StationID: sid,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html")
}
