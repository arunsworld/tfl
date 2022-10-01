package tfl

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"sort"
	"time"
)

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

func (sf *staticFetcher) fetchArrivals(lineID, stationID string) (Arrivals, error) {
	url := sf.arrivalsURL(lineID, stationID)
	resp, err := sf.c.Get(url)
	if err != nil {
		return Arrivals{}, fmt.Errorf("problem fetching data from API: %v", err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return Arrivals{}, fmt.Errorf("problem reading data from response: %v", err)
	}
	if resp.StatusCode == 400 {
		return Arrivals{}, nil
	}
	tflStationArrivals := []tflStationArrival{}
	if err := json.Unmarshal(body, &tflStationArrivals); err != nil {
		return Arrivals{}, fmt.Errorf("problem parsing response data from TFL: %v", err)
	}
	if len(tflStationArrivals) == 0 {
		return Arrivals{}, nil
	}
	return Arrivals{
		StationID:   tflStationArrivals[0].NaptanId,
		StationName: tflStationArrivals[0].StationName,
		Platforms:   calculateArrivalsByPlatform(tflStationArrivals),
	}, nil
}

func calculateArrivalsByPlatform(tflStationArrivals []tflStationArrival) []Platform {
	buffer := make(map[string]*Platform)
	for _, arr := range tflStationArrivals {
		platformName := arr.cleansedPlatformName()
		pform, ok := buffer[platformName]
		if !ok {
			pform = &Platform{
				Name: platformName,
			}
			buffer[platformName] = pform
		}
		pform.Arrivals = append(pform.Arrivals, Arrival{
			VehicleID:       arr.VehicleId,
			Towards:         arr.Towards,
			CurrentLocation: arr.calculateCurrentLocation(),
			TimeToStation:   time.Second * time.Duration(int64(arr.TimeToStation)),
			ExpectedArrival: arr.expectedArrivalAsTime(),
		})
	}
	result := make([]Platform, 0, len(buffer))
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
