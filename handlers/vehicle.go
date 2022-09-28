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

type gmtConverter struct {
	loc *time.Location
}

func newGMTConverter() *gmtConverter {
	loc, err := time.LoadLocation("Europe/London")
	if err != nil {
		panic(err)
	}
	return &gmtConverter{
		loc: loc,
	}
}

func (g *gmtConverter) convert(input time.Time) time.Time {
	return input.In(g.loc)
}

var gmtc = newGMTConverter()

func (h handlers) registerVehicleHandler() {
	vTracker := newVechicleTracker()
	vechicleGET := h.handler.PathPrefix("/vehicles/").Methods("GET").Subrouter()
	vechicleGET.HandleFunc("/{line_id}/{vehicle_id}", func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		lineID := vars["line_id"]
		vehicleID := vars["vehicle_id"]
		vs, err := vTracker.scheduleFor(lineID, vehicleID)
		if err != nil {
			handleVehicleDataRetreivalError(w, h.tmpls, lineID, vehicleID, err.Error())
			return
		}
		if vs.VehicleID == "" {
			handleVehicleNotFound(w, h.tmpls, lineID, vehicleID)
			return
		}
		err = h.tmpls.ExecuteTemplate(w, "vehicles.html", struct {
			LineID          string
			VehicleSchedule vehicleSchedule
		}{
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

type vehicleSchedule struct {
	VehicleID       string
	Line            string
	Destination     string
	CurrentLocation string
	Stops           []stop
}

func (vs vehicleSchedule) CleansedCurrentLocation() string {
	if vs.CurrentLocation == "" {
		return "Current Location Not Specified"
	}
	return vs.CurrentLocation
}

type stop struct {
	StationID       string
	StationName     string
	TimeToStation   time.Duration
	ExpectedArrival time.Time
}

func (s stop) ETA() string {
	return fmt.Sprintf("%s (%s)", gmtc.convert(s.ExpectedArrival).Format("15:04"), s.TimeToStation)
}

func newVechicleTracker() *vehicleTracker {
	c := http.Client{Timeout: time.Duration(5) * time.Second}
	return &vehicleTracker{
		c: c,
		url: func(vid string) string {
			return fmt.Sprintf(tfl.VehicleArrivalsAPI, vid)
		},
	}
}

type vehicleTracker struct {
	c   http.Client
	url func(string) string
}

type tflVehicleArrivals struct {
	VehicleId       string
	LineId          string
	LineName        string
	DestinationName string
	Towards         string
	NaptanId        string // StationID
	StationName     string
	TimeToStation   int
	CurrentLocation string
	ExpectedArrival string
}

func (v *vehicleTracker) scheduleFor(lineID, vehicleID string) (vehicleSchedule, error) {
	url := v.url(vehicleID)
	resp, err := v.c.Get(url)
	if err != nil {
		return vehicleSchedule{}, fmt.Errorf("problem fetching data from API: %v", err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return vehicleSchedule{}, fmt.Errorf("problem reading data from response: %v", err)
	}
	_tva := []tflVehicleArrivals{}
	if err := json.Unmarshal(body, &_tva); err != nil {
		return vehicleSchedule{}, fmt.Errorf("problem parsing response data from TFL: %v", err)
	}
	tva := make([]tflVehicleArrivals, 0, len(_tva))
	for _, station := range _tva {
		if station.LineId != lineID {
			continue
		}
		tva = append(tva, station)
	}
	if len(tva) == 0 {
		return vehicleSchedule{}, nil
	}
	result := vehicleSchedule{
		VehicleID:       vehicleID,
		Line:            tva[0].LineName,
		Destination:     v.calculateDestination(tva),
		CurrentLocation: v.calculateCurrentLocation(tva),
		Stops:           v.calculateStops(tva),
	}
	return result, nil
}

func (v *vehicleTracker) calculateDestination(input []tflVehicleArrivals) string {
	chosenArrival := input[0]
	if chosenArrival.DestinationName != "" {
		return chosenArrival.DestinationName
	}
	if chosenArrival.Towards != "" {
		return chosenArrival.Towards
	}
	return "Not Available"
}

func (v *vehicleTracker) calculateCurrentLocation(input []tflVehicleArrivals) string {
	result := ""
	for _, station := range input {
		if len(station.CurrentLocation) > len(result) {
			result = station.CurrentLocation
		}
	}
	return result
}

func (v *vehicleTracker) calculateStops(input []tflVehicleArrivals) []stop {
	result := make([]stop, 0, len(input))
	for _, station := range input {
		expectedArrival, err := time.Parse(time.RFC3339, station.ExpectedArrival)
		if err != nil {
			log.Printf("unable to parse %s as date: %v", station.ExpectedArrival, err)
		}
		result = append(result, stop{
			StationID:       station.NaptanId,
			StationName:     station.StationName,
			TimeToStation:   time.Second * time.Duration(int64(station.TimeToStation)),
			ExpectedArrival: expectedArrival,
		})
	}
	sort.Slice(result, func(x, y int) bool {
		return result[x].TimeToStation < result[y].TimeToStation
	})
	dedupedResult := make([]stop, 0, len(result))
	currentID := ""
	for _, station := range result {
		if station.StationID == currentID {
			continue
		}
		currentID = station.StationID
		dedupedResult = append(dedupedResult, station)
	}
	return dedupedResult
}
