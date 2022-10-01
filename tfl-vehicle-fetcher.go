package tfl

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"sort"
	"time"
)

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

func (sf *staticFetcher) fetchVehicleScheduleFor(lineID, vehicleID string) (VehicleSchedule, error) {
	url := sf.vehiclesURL(vehicleID)
	resp, err := sf.c.Get(url)
	if err != nil {
		return VehicleSchedule{}, fmt.Errorf("problem fetching data from API: %v", err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return VehicleSchedule{}, fmt.Errorf("problem reading data from response: %v", err)
	}
	_tva := []tflVehicleArrivals{}
	if err := json.Unmarshal(body, &_tva); err != nil {
		return VehicleSchedule{}, fmt.Errorf("problem parsing response data from TFL: %v", err)
	}
	tva := make([]tflVehicleArrivals, 0, len(_tva))
	for _, station := range _tva {
		if station.LineId != lineID {
			continue
		}
		tva = append(tva, station)
	}
	if len(tva) == 0 {
		return VehicleSchedule{}, nil
	}
	result := VehicleSchedule{
		VehicleID:       vehicleID,
		Line:            tva[0].LineName,
		Destination:     calculateVehicleDestination(tva),
		CurrentLocation: calculateVehicleCurrentLocation(tva),
		Stops:           calculateVehicleStops(tva),
	}
	return result, nil
}

func calculateVehicleDestination(input []tflVehicleArrivals) string {
	chosenArrival := input[0]
	if chosenArrival.DestinationName != "" {
		return chosenArrival.DestinationName
	}
	if chosenArrival.Towards != "" {
		return chosenArrival.Towards
	}
	return "Not Available"
}

func calculateVehicleCurrentLocation(input []tflVehicleArrivals) string {
	result := ""
	for _, station := range input {
		if len(station.CurrentLocation) > len(result) {
			result = station.CurrentLocation
		}
	}
	return result
}

func calculateVehicleStops(input []tflVehicleArrivals) []VehicleStop {
	result := make([]VehicleStop, 0, len(input))
	for _, station := range input {
		expectedArrival, err := time.Parse(time.RFC3339, station.ExpectedArrival)
		if err != nil {
			log.Printf("unable to parse %s as date: %v", station.ExpectedArrival, err)
		}
		result = append(result, VehicleStop{
			StationID:       station.NaptanId,
			StationName:     station.StationName,
			TimeToStation:   time.Second * time.Duration(int64(station.TimeToStation)),
			ExpectedArrival: expectedArrival,
		})
	}
	sort.Slice(result, func(x, y int) bool {
		return result[x].TimeToStation < result[y].TimeToStation
	})
	dedupedResult := make([]VehicleStop, 0, len(result))
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
