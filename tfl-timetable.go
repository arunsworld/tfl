package tfl

import (
	"fmt"
	"log"
	"strconv"
	"time"
)

type timeTableRequest struct {
	lineID        string
	fromStationID string
	destStationID string
	weekday       time.Weekday
	departureTime DepartureTime
	vehicleID     string
	// response
	departureTimeResp chan struct {
		depTimes ScheduledDepartureTimes
		err      error
	}
	scheduledTimeTableResp chan struct {
		scheduledTimeTable ScheduledTimeTable
		err                error
	}
}

func (sd *staticData) monitorTimetableFetch() {
	ttMgr := newTimetableManager(sd.fetcher)
	for req := range sd.timeTableRequests {
		if req.departureTime.Hour == "" {
			sdt, err := ttMgr.scheduledDepartureTimesFor(req.lineID, req.fromStationID, req.destStationID, req.weekday)
			req.departureTimeResp <- struct {
				depTimes ScheduledDepartureTimes
				err      error
			}{
				depTimes: sdt,
				err:      err,
			}
		} else {
			stt, err := ttMgr.scheduledTimeTableFor(req.lineID, req.fromStationID, req.destStationID, req.weekday, req.departureTime, req.vehicleID)
			req.scheduledTimeTableResp <- struct {
				scheduledTimeTable ScheduledTimeTable
				err                error
			}{
				scheduledTimeTable: stt,
				err:                err,
			}
		}
	}
}

func (sd *staticData) ScheduledDepartureTimes(lineID, fromStationID, toStationID string, weekday time.Weekday) (ScheduledDepartureTimes, error) {
	resp := make(chan struct {
		depTimes ScheduledDepartureTimes
		err      error
	}, 1)
	req := timeTableRequest{
		lineID:            lineID,
		fromStationID:     fromStationID,
		destStationID:     toStationID,
		weekday:           weekday,
		departureTimeResp: resp,
	}
	select {
	case sd.timeTableRequests <- req:
		result := <-resp
		return result.depTimes, result.err
	case <-time.After(time.Second * 5):
		return ScheduledDepartureTimes{}, fmt.Errorf("timed out waiting on processing request")
	}
}

func (sd *staticData) ScheduledTimeTable(lineID, fromStationID, toStationID string,
	weekday time.Weekday, depTime DepartureTime, vehicleID string) (ScheduledTimeTable, error) {

	resp := make(chan struct {
		scheduledTimeTable ScheduledTimeTable
		err                error
	}, 1)
	req := timeTableRequest{
		lineID:                 lineID,
		fromStationID:          fromStationID,
		destStationID:          toStationID,
		weekday:                weekday,
		departureTime:          depTime,
		vehicleID:              vehicleID,
		scheduledTimeTableResp: resp,
	}
	select {
	case sd.timeTableRequests <- req:
		result := <-resp
		return result.scheduledTimeTable, result.err
	case <-time.After(time.Second * 5):
		return ScheduledTimeTable{}, fmt.Errorf("timed out waiting on processing request")
	}
}

type ScheduledDepartureTimes struct {
	From           Station
	To             Station
	DepartureTimes []DepartureTime
}

type DepartureTime struct {
	Hour           string
	Minute         string
	Destination    Station
	DestinationETA string
}

type departureTimeKey struct {
	hour, minute string
}

func (dt DepartureTime) ETD() string {
	hour, err := strconv.Atoi(dt.Hour)
	if err != nil {
		log.Printf("error parsing hour: %v", err)
		return "00:00"
	}
	minute, err := strconv.Atoi(dt.Minute)
	if err != nil {
		log.Printf("error parsing minute: %v", err)
		return "00:00"
	}
	if hour > 23 {
		hour = hour - 24
	}
	return fmt.Sprintf("%02d:%02d", hour, minute)
}

type ScheduledTimeTable struct {
	From          Station
	To            Station
	DepartureTime DepartureTime
	Stops         []ScheduledStop
}

type ScheduledStop struct {
	Station       Station
	TimeToArrival time.Duration
	ETA           string
	JourneyETA    string
	JourneyStatus string
}

type timetableCacheKey struct {
	line     string
	from, to string
}

func calculateETA(departureTime DepartureTime, timeToArrival time.Duration) string {
	etd, err := time.Parse("15:04", departureTime.ETD())
	if err != nil {
		log.Printf("unable to parse ETD: %v", err)
	}
	eta := etd.Add(timeToArrival)
	return eta.Format("15:04")
}

// only one of this should exist
// it's methods and operations are not thread-safe therefore should not be called concurrently
type timetableManager struct {
	fetcher *staticFetcher
	cache   map[timetableCacheKey]timetableByDayOfWeek
}

func newTimetableManager(fetcher *staticFetcher) *timetableManager {
	return &timetableManager{
		fetcher: fetcher,
		cache:   make(map[timetableCacheKey]timetableByDayOfWeek),
	}
}

func (tm *timetableManager) timetableFor(lineID, srcStationID, destStationID string) (timetableByDayOfWeek, error) {
	key := timetableCacheKey{line: lineID, from: srcStationID, to: destStationID}
	tbdw, ok := tm.cache[key]
	if ok && tbdw.isStillCurrent() {
		return tbdw, nil
	}
	v, err := tm.fetcher.fetchTimetable(lineID, srcStationID, destStationID)
	if err != nil {
		return timetableByDayOfWeek{}, err
	}
	tm.cache[key] = v
	return v, nil
}

func (tm *timetableManager) scheduledDepartureTimesFor(lineID, srcStationID, destStationID string, weekday time.Weekday) (ScheduledDepartureTimes, error) {
	tbdw, err := tm.timetableFor(lineID, srcStationID, destStationID)
	if err != nil {
		return ScheduledDepartureTimes{}, err
	}
	ttDetails := tbdw.timeTableDetailsFor(weekday)
	return ScheduledDepartureTimes{
		From:           tbdw.stops[srcStationID],
		To:             tbdw.stops[destStationID],
		DepartureTimes: ttDetails.scheduledDepartures,
	}, nil
}

func (tm *timetableManager) scheduledTimeTableFor(lineID, srcStationID, destStationID string,
	weekday time.Weekday, departureTime DepartureTime, vehicleID string) (ScheduledTimeTable, error) {

	tbdw, err := tm.timetableFor(lineID, srcStationID, destStationID)
	if err != nil {
		return ScheduledTimeTable{}, err
	}
	ttDetails := tbdw.timeTableDetailsFor(weekday)
	journey, ok := ttDetails.journeys[departureTimeKey{hour: departureTime.Hour, minute: departureTime.Minute}]
	if !ok {
		return ScheduledTimeTable{}, fmt.Errorf("no journey found for departure time: %s", departureTime.ETD())
	}
	stops := journeyStopsToScheduledStops(journey.stops, departureTime)
	return ScheduledTimeTable{
		From:          tbdw.stops[srcStationID],
		To:            tbdw.stops[destStationID],
		DepartureTime: departureTime,
		Stops:         stops,
	}, nil
}

func journeyStopsToScheduledStops(journeyStops []stop, departureTime DepartureTime) []ScheduledStop {
	stops := make([]ScheduledStop, 0, len(journeyStops))
	for _, stop := range journeyStops {
		stops = append(stops, ScheduledStop{
			Station:       stop.station,
			TimeToArrival: stop.timeToArrival,
			ETA:           calculateETA(departureTime, stop.timeToArrival),
		})
	}
	return stops
}

type timetableByDayOfWeek struct {
	stops     map[string]Station
	monToThu  timeTableDetails
	fri       timeTableDetails
	sun       timeTableDetails
	others    timeTableDetails
	createdOn time.Time
}

func (tbdw timetableByDayOfWeek) isStillCurrent() bool {
	cachedDataFetchDate := tbdw.createdOn.Format("2006-01-02")
	today := time.Now().Format("2006-01-02")
	if today != cachedDataFetchDate {
		log.Printf("cache invalidated...")
		return false
	}
	log.Printf("cache still OK")
	return true
}

func (tbdw timetableByDayOfWeek) timeTableDetailsFor(weekday time.Weekday) timeTableDetails {
	switch weekday {
	case time.Monday, time.Tuesday, time.Wednesday, time.Thursday:
		return tbdw.monToThu
	case time.Friday:
		return tbdw.fri
	case time.Saturday:
		return tbdw.others
	default:
		return tbdw.sun
	}
}

type timeTableDetails struct {
	scheduledDepartures []DepartureTime
	journeys            map[departureTimeKey]*journey
}

type journey struct {
	stops []stop
}

type stop struct {
	station       Station
	timeToArrival time.Duration
}

// Timetable Fetching
