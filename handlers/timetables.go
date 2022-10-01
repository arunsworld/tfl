package handlers

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/arunsworld/tfl"
	"github.com/gorilla/mux"
)

func (h handlers) registerTimetablesHandler() {
	timetablesGET := h.handler.PathPrefix("/timetables/").Methods("GET").Subrouter()
	timetablesGET.HandleFunc("/{mode}/{line_id}/{station_id}", func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		mode := vars["mode"]
		lineID := vars["line_id"]
		fromStationID := vars["station_id"]
		// src & destination stations
		queryParams := r.URL.Query()
		originStationID, ok := queryParams["src"]
		if !ok || len(originStationID) == 0 || originStationID[0] == "" {
			http.Redirect(w, r, fmt.Sprintf("/routes/%s/%s?timetables", mode, lineID), 302)
			return
		}
		destStationID, ok := queryParams["dest"]
		if !ok || len(destStationID) == 0 || destStationID[0] == "" {
			http.Redirect(w, r, fmt.Sprintf("/routes/%s/%s?timetables", mode, lineID), 302)
			return
		}
		sdt, err := tfl.TFLStaticDataGlobal.ScheduledDepartureTimes(lineID, fromStationID, destStationID[0], time.Now().Weekday())
		if err != nil {
			handleStationDataRetreivalError(w, h.tmpls, mode, lineID, fromStationID, "timetables", true, originStationID[0], destStationID[0], err.Error())
			return
		}
		err = h.tmpls.ExecuteTemplate(w, "timetable-departure-times.html", struct {
			Mode                    string
			LineID                  string
			Station                 string
			OriginStation           string
			DestStation             string
			ScheduledDepartureTimes tfl.ScheduledDepartureTimes
		}{
			Mode:                    mode,
			LineID:                  lineID,
			Station:                 fromStationID,
			OriginStation:           originStationID[0],
			DestStation:             destStationID[0],
			ScheduledDepartureTimes: sdt,
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html")
	})
	timetablesGET.HandleFunc("/{mode}/{line_id}/{station_id}/{hour}/{minute}", func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		mode := vars["mode"]
		lineID := vars["line_id"]
		fromStationID := vars["station_id"]
		depTime := tfl.DepartureTime{Hour: vars["hour"], Minute: vars["minute"]}
		// src & destination stations
		queryParams := r.URL.Query()
		originStationID, ok := queryParams["src"]
		if !ok || len(originStationID) == 0 || originStationID[0] == "" {
			http.Redirect(w, r, fmt.Sprintf("/routes/%s/%s?timetables", mode, lineID), 302)
			return
		}
		destStationID, ok := queryParams["dest"]
		if !ok || len(destStationID) == 0 || destStationID[0] == "" {
			http.Redirect(w, r, fmt.Sprintf("/routes/%s/%s?timetables", mode, lineID), 302)
			return
		}
		vehicleID := ""
		vehicleTracking := false
		_vid, ok := queryParams["v"]
		if ok && len(_vid) == 1 && _vid[0] != "" {
			vehicleID = _vid[0]
			vehicleTracking = true
		}
		stt, err := tfl.TFLStaticDataGlobal.ScheduledTimeTable(lineID, fromStationID, destStationID[0], time.Now().Weekday(), depTime, vehicleID)
		if err != nil {
			handleStationDataRetreivalError(w, h.tmpls, mode, lineID, fromStationID, "timetables", true, originStationID[0], destStationID[0], err.Error())
			return
		}
		err = h.tmpls.ExecuteTemplate(w, "timetable-schedule.html", struct {
			Mode               string
			LineID             string
			Station            string
			OriginStation      string
			DestStation        string
			ScheduledTimeTable tfl.ScheduledTimeTable
			VehicleTracking    bool
		}{
			Mode:               mode,
			LineID:             lineID,
			Station:            fromStationID,
			OriginStation:      originStationID[0],
			DestStation:        destStationID[0],
			ScheduledTimeTable: stt,
			VehicleTracking:    vehicleTracking,
		})
		if err != nil {
			log.Println(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html")
	})
}
