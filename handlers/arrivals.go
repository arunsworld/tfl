package handlers

import (
	"net/http"

	"github.com/arunsworld/tfl"
	"github.com/gorilla/mux"
)

func (h handlers) registerArrivalsHandler() {
	stationsGET := h.handler.PathPrefix("/arrivals/").Methods("GET").Subrouter()
	stationsGET.HandleFunc("/{mode}/{line_id}/{station_id}", func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		mode := vars["mode"]
		lineID := vars["line_id"]
		stationID := vars["station_id"]
		avls, err := tfl.TFLStaticDataGlobal.ArrivalsFor(lineID, stationID)
		if err != nil {
			handleStationDataRetreivalError(w, h.tmpls, mode, lineID, stationID, "arrivals", false, "", "", err.Error())
			return
		}
		if avls.StationID == "" {
			handleStationDataNotFound(w, h.tmpls, mode, lineID, stationID)
			return
		}
		// check if we want vehicle data displayed
		queryParams := r.URL.Query()
		_, showVehicleInfo := queryParams["v"]
		err = h.tmpls.ExecuteTemplate(w, "arrivals.html", struct {
			Mode            string
			LineID          string
			Arrivals        tfl.Arrivals
			ShowVehicleInfo bool
		}{
			Mode:            mode,
			LineID:          lineID,
			Arrivals:        avls,
			ShowVehicleInfo: showVehicleInfo,
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html")
	})
}
