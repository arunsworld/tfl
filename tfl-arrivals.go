package tfl

import (
	"fmt"
	"time"
)

type Arrivals struct {
	StationID   string
	StationName string
	Platforms   []Platform
}

type Platform struct {
	Name     string
	Arrivals []Arrival
}

type Arrival struct {
	VehicleID       string
	Towards         string
	CurrentLocation string
	TimeToStation   time.Duration
	ExpectedArrival time.Time
}

func (a Arrival) CanBeTracked() bool {
	return a.VehicleID != "000"
}

func (a Arrival) ETA() string {
	return fmt.Sprintf("%s (%s)", gmtc.convert(a.ExpectedArrival).Format("15:04"), a.TimeToStation)
}

func (sd *staticData) ArrivalsFor(lineID, stationID string) (Arrivals, error) {
	return sd.fetcher.fetchArrivals(lineID, stationID)
}
