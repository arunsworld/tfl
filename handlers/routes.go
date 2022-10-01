package handlers

import (
	"html/template"
	"net/http"

	"github.com/arunsworld/tfl"
	"github.com/gorilla/mux"
)

type nextNav struct {
	Navigation  string
	Subtitle    string
	SwitchMsg   string
	SwitchParam string
}

func (n nextNav) CaptureStartAndDest() bool {
	return n.Navigation == "timetables"
}

func (h handlers) registerRoutesHandler() {
	routesGET := h.handler.PathPrefix("/routes/").Methods("GET").Subrouter()
	routesGET.HandleFunc("/{mode}/{line_id}", func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		mode := vars["mode"]
		lineID := vars["line_id"]
		routes := tfl.TFLAPIGlobal.Routes(lineID)
		lineDetails := tfl.TFLAPIGlobal.LineDetails(mode, lineID)
		// check if for arrivals or timetable
		var nn nextNav
		queryParams := r.URL.Query()
		_, ok := queryParams["timetables"]
		if ok {
			nn = nextNav{
				Navigation:  "timetables",
				Subtitle:    "Select a station for it's timetable.",
				SwitchMsg:   "Switch to Arrivals",
				SwitchParam: "arrivals",
			}
		} else {
			nn = nextNav{
				Navigation:  "arrivals",
				Subtitle:    "Select a station for real-time arrival updates.",
				SwitchMsg:   "Switch to Timetables",
				SwitchParam: "timetables",
			}
		}
		err := h.tmpls.ExecuteTemplate(w, "routes.html", struct {
			Mode     string
			LineID   string
			LineName string
			Routes   []tfl.Route
			NextNav  nextNav
		}{
			Mode:     mode,
			LineID:   lineID,
			LineName: lineDetails.Name,
			Routes:   routes,
			NextNav:  nn,
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html")
	})
}

func handleStationDataRetreivalError(w http.ResponseWriter, tmpls *template.Template, mode, lid, sid string, nav string, showSrcDest bool, src, dest string, errMsg string) {
	err := tmpls.ExecuteTemplate(w, "station-error.html", struct {
		Mode        string
		LineID      string
		StationID   string
		Error       string
		Navigation  string
		ShowSrcDest bool
		Src, Dest   string
	}{
		Mode:        mode,
		LineID:      lid,
		StationID:   sid,
		Error:       errMsg,
		Navigation:  nav,
		ShowSrcDest: showSrcDest,
		Src:         src,
		Dest:        dest,
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
