package handlers

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"sort"
	"text/template"
	"time"

	"github.com/arunsworld/tfl"
	"github.com/gorilla/mux"
)

type arrivalsFetcher struct {
	c   http.Client
	url func(string, string) string
}

func newArrivalsFetcher() *arrivalsFetcher {
	c := http.Client{Timeout: time.Duration(5) * time.Second}
	return &arrivalsFetcher{
		c: c,
		url: func(lineID, stationID string) string {
			return fmt.Sprintf(tfl.LineArrivalsAPI, lineID, stationID)
		},
	}
}

func (af *arrivalsFetcher) arrivalsFor(lineID, stationID string) (arrivals, error) {
	url := af.url(lineID, stationID)
	resp, err := af.c.Get(url)
	if err != nil {
		return arrivals{}, fmt.Errorf("problem fetching data from API: %v", err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return arrivals{}, fmt.Errorf("problem reading data from response: %v", err)
	}
	if resp.StatusCode == 400 {
		return arrivals{}, nil
	}
	tflStationArrivals := []tflStationArrival{}
	if err := json.Unmarshal(body, &tflStationArrivals); err != nil {
		return arrivals{}, fmt.Errorf("problem parsing response data from TFL: %v", err)
	}
	if len(tflStationArrivals) == 0 {
		return arrivals{}, nil
	}
	return arrivals{
		StationID:   tflStationArrivals[0].NaptanId,
		StationName: tflStationArrivals[0].StationName,
		Platforms:   af.calculateArrivalsByPlatform(tflStationArrivals),
	}, nil
}

func (af *arrivalsFetcher) calculateArrivalsByPlatform(arrivals []tflStationArrival) []platform {
	buffer := make(map[string]*platform)
	for _, arr := range arrivals {
		platformName := arr.cleansedPlatformName()
		pform, ok := buffer[platformName]
		if !ok {
			pform = &platform{
				Name: platformName,
			}
			buffer[platformName] = pform
		}
		pform.Arrivals = append(pform.Arrivals, arrival{
			VehicleID:       arr.VehicleId,
			Towards:         arr.Towards,
			CurrentLocation: arr.calculateCurrentLocation(),
			TimeToStation:   time.Second * time.Duration(int64(arr.TimeToStation)),
			ExpectedArrival: arr.expectedArrivalAsTime(),
		})
	}
	result := make([]platform, 0, len(buffer))
	for _, v := range buffer {
		sort.Slice(v.Arrivals, func(i, j int) bool {
			return v.Arrivals[i].TimeToStation < v.Arrivals[j].TimeToStation
		})
		result = append(result, *v)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

type tflStationArrival struct {
	NaptanId        string
	StationName     string
	PlatformName    string
	Towards         string
	CurrentLocation string
	VehicleId       string
	TimeToStation   int
	ExpectedArrival string
}

func (tsa tflStationArrival) expectedArrivalAsTime() time.Time {
	expectedArrival, err := time.Parse(time.RFC3339, tsa.ExpectedArrival)
	if err != nil {
		log.Printf("unable to parse %s as date: %v", tsa.ExpectedArrival, err)
	}
	return expectedArrival
}

func (tsa tflStationArrival) calculateCurrentLocation() string {
	if tsa.CurrentLocation != "" {
		return tsa.CurrentLocation
	}
	return "Not Available"
}

func (tsa tflStationArrival) cleansedPlatformName() string {
	if tsa.PlatformName == "" || tsa.PlatformName == "null" {
		return "Platform Not Specified"
	}
	return tsa.PlatformName
}

type arrivals struct {
	StationID   string
	StationName string
	Platforms   []platform
}

type platform struct {
	Name     string
	Arrivals []arrival
}

type arrival struct {
	VehicleID       string
	Towards         string
	CurrentLocation string
	TimeToStation   time.Duration
	ExpectedArrival time.Time
}

func (a arrival) CanBeTracked() bool {
	return a.VehicleID != "000"
}

func (a arrival) ETA() string {
	return fmt.Sprintf("%s (%s)", gmtc.convert(a.ExpectedArrival).Format("15:04"), a.TimeToStation)
}

func (h handlers) registerStationsHandler() {
	stationsGET := h.handler.PathPrefix("/stations/").Methods("GET").Subrouter()
	af := newArrivalsFetcher()
	stationsGET.HandleFunc("/{mode}/{line_id}/{station_id}", func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		mode := vars["mode"]
		lineID := vars["line_id"]
		stationID := vars["station_id"]
		avls, err := af.arrivalsFor(lineID, stationID)
		if err != nil {
			handleStationDataRetreivalError(w, h.tmpls, mode, lineID, stationID, err.Error())
			return
		}
		if avls.StationID == "" {
			handleStationDataNotFound(w, h.tmpls, mode, lineID, stationID)
			return
		}
		err = h.tmpls.ExecuteTemplate(w, "stations-arrival.html", struct {
			Mode     string
			LineID   string
			Arrivals arrivals
		}{
			Mode:     mode,
			LineID:   lineID,
			Arrivals: avls,
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html")
	})
	stationsGET.HandleFunc("/{mode}/{line_id}", func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		mode := vars["mode"]
		lineID := vars["line_id"]
		routes := tfl.TFLStaticDataGlobal.Routes(lineID)
		lineDetails := tfl.TFLStaticDataGlobal.LineDetails(mode, lineID)
		err := h.tmpls.ExecuteTemplate(w, "stations-routes-choose.html", struct {
			Mode     string
			LineID   string
			LineName string
			Routes   []tfl.Route
		}{
			Mode:     mode,
			LineID:   lineID,
			LineName: lineDetails.Name,
			Routes:   routes,
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html")
	})
}

func handleStationDataRetreivalError(w http.ResponseWriter, tmpls *template.Template, mode, lid, sid string, errMsg string) {
	err := tmpls.ExecuteTemplate(w, "station-error.html", struct {
		Mode      string
		LineID    string
		StationID string
		Error     string
	}{
		Mode:      mode,
		LineID:    lid,
		StationID: sid,
		Error:     errMsg,
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
