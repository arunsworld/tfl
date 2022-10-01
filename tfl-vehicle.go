package tfl

import (
	"fmt"
	"time"
)

type VehicleSchedule struct {
	VehicleID       string
	Line            string
	Destination     string
	CurrentLocation string
	Stops           []VehicleStop
}

func (vs VehicleSchedule) CleansedCurrentLocation() string {
	if vs.CurrentLocation == "" {
		return "Current Location Not Specified"
	}
	return vs.CurrentLocation
}

type VehicleStop struct {
	StationID       string
	StationName     string
	TimeToStation   time.Duration
	ExpectedArrival time.Time
}

func (s VehicleStop) ETA() string {
	return fmt.Sprintf("%s (%s)", gmtc.convert(s.ExpectedArrival).Format("15:04"), s.TimeToStation)
}

func (s VehicleStop) ETATime() string {
	return gmtc.convert(s.ExpectedArrival).Format("15:04")
}

func (sd *tflAPIImpl) VehicleScheduleFor(lineID, vehicleID string) (VehicleSchedule, error) {
	return sd.fetcher.fetchVehicleScheduleFor(lineID, vehicleID)
}
