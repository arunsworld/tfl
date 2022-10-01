package handlers

import (
	"fmt"
	"html/template"
	"net/http"

	"github.com/arunsworld/tfl"
	"github.com/gorilla/mux"
)

func (h handlers) registerVehicleHandler() {
	vechicleGET := h.handler.PathPrefix("/vehicles/").Methods("GET").Subrouter()
	// For temporary backwards compatibility - assume tube
	vechicleGET.HandleFunc("/{line_id}/{vehicle_id}", func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		lineID := vars["line_id"]
		vehicleID := vars["vehicle_id"]
		http.Redirect(w, r, fmt.Sprintf("/vehicles/tube/%s/%s", lineID, vehicleID), 302)
	})
	vechicleGET.HandleFunc("/{mode}/{line_id}/{vehicle_id}", func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		mode := vars["mode"]
		lineID := vars["line_id"]
		vehicleID := vars["vehicle_id"]
		vs, err := tfl.TFLStaticDataGlobal.VehicleScheduleFor(lineID, vehicleID)
		if err != nil {
			handleVehicleDataRetreivalError(w, h.tmpls, lineID, vehicleID, err.Error())
			return
		}
		if vs.VehicleID == "" {
			handleVehicleNotFound(w, h.tmpls, lineID, vehicleID)
			return
		}
		err = h.tmpls.ExecuteTemplate(w, "vehicles.html", struct {
			Mode            string
			LineID          string
			VehicleSchedule tfl.VehicleSchedule
		}{
			Mode:            mode,
			LineID:          lineID,
			VehicleSchedule: vs,
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html")
	})
}

func handleVehicleDataRetreivalError(w http.ResponseWriter, tmpls *template.Template, lid, vid string, errMsg string) {
	err := tmpls.ExecuteTemplate(w, "vehicle-error.html", struct {
		LineID    string
		VehicleID string
		Error     string
	}{
		LineID:    lid,
		VehicleID: vid,
		Error:     errMsg,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html")
}

func handleVehicleNotFound(w http.ResponseWriter, tmpls *template.Template, lid, vid string) {
	err := tmpls.ExecuteTemplate(w, "vehicle-not-found.html", struct {
		VehicleID string
		LineID    string
	}{
		VehicleID: vid,
		LineID:    lid,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html")
}
