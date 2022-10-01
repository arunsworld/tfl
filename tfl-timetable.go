package tfl

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type timeTableRequest struct {
	lineID        string
	fromStationID string
	destStationID string
	weekday       time.Weekday
	departureTime DepartureTime
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
			stt, err := ttMgr.scheduledTimeTableFor(req.lineID, req.fromStationID, req.destStationID, req.weekday, req.departureTime)
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

func (sd *staticData) ScheduledTimeTable(lineID, fromStationID, toStationID string, weekday time.Weekday, depTime DepartureTime) (ScheduledTimeTable, error) {
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
	if ok {
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
	weekday time.Weekday, departureTime DepartureTime) (ScheduledTimeTable, error) {

	tbdw, err := tm.timetableFor(lineID, srcStationID, destStationID)
	if err != nil {
		return ScheduledTimeTable{}, err
	}
	ttDetails := tbdw.timeTableDetailsFor(weekday)
	journey, ok := ttDetails.journeys[departureTimeKey{hour: departureTime.Hour, minute: departureTime.Minute}]
	if !ok {
		return ScheduledTimeTable{}, fmt.Errorf("no journey found for departure time: %s", departureTime.ETD())
	}
	stops := make([]ScheduledStop, 0, len(journey.stops))
	for _, stop := range journey.stops {
		stops = append(stops, ScheduledStop{
			Station:       stop.station,
			TimeToArrival: stop.timeToArrival,
			ETA:           calculateETA(departureTime, stop.timeToArrival),
		})
	}
	return ScheduledTimeTable{
		From:          tbdw.stops[srcStationID],
		To:            tbdw.stops[destStationID],
		DepartureTime: departureTime,
		Stops:         stops,
	}, nil
}

type timetableByDayOfWeek struct {
	stops    map[string]Station
	monToThu timeTableDetails
	fri      timeTableDetails
	sun      timeTableDetails
	others   timeTableDetails
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

type staticFetcher struct {
	c            http.Client
	linesURL     func(string) string
	stationsURL  func(string) string
	routesURL    func(string) string
	statusURL    func(string) string
	timetableURL func(string, string, string) string
}

type tflTimetableWrapper struct {
	Stations  []tflStation
	Stops     []tflStation
	Timetable tflTimetable
}

type tflTimetable struct {
	Routes []tflTimetableRoute
}

type tflTimetableRoute struct {
	StationIntervals []tflStationInterval
	Schedules        []tflSchedule
}

type tflStationInterval struct {
	ID        string
	Intervals []tflInterval
}

type tflInterval struct {
	StopId        string
	TimeToArrival float64
}

type tflSchedule struct {
	Name          string
	KnownJourneys []tflJourney
}

type tflJourney struct {
	Hour       string
	Minute     string
	IntervalId int
}

func (sf *staticFetcher) fetchTimetable(lineID, srcStation, destStation string) (timetableByDayOfWeek, error) {
	url := sf.timetableURL(lineID, srcStation, destStation)
	log.Println(url)
	resp, err := sf.c.Get(url)
	if err != nil {
		return timetableByDayOfWeek{}, fmt.Errorf("problem fetching timetable data from API: %v", err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return timetableByDayOfWeek{}, fmt.Errorf("problem reading timetable data from response: %v", err)
	}
	tflTW := tflTimetableWrapper{}
	if err := json.Unmarshal(body, &tflTW); err != nil {
		return timetableByDayOfWeek{}, fmt.Errorf("problem parsing timetable data from TFL: %v", err)
	}
	return tflTimetableWrapperTotimetableByDayOfWeek(tflTW, lineID, srcStation, destStation)
}

func tflTimetableWrapperTotimetableByDayOfWeek(input tflTimetableWrapper, lineID, srcStation, destStation string) (timetableByDayOfWeek, error) {
	stopsCache := map[string]Station{}
	for _, s := range input.Stops {
		stopsCache[s.Id] = Station{
			ID:   s.Id,
			Name: s.Name,
		}
	}
	if len(input.Timetable.Routes) == 0 {
		return timetableByDayOfWeek{}, fmt.Errorf("no routes found for %s from %s to %s in timetable", lineID, srcStation, destStation)
	}
	if len(input.Timetable.Routes) != 1 {
		log.Printf("WARNING: timetable for %s from %s to %s, found multiple routes", lineID, srcStation, destStation)
	}
	route := input.Timetable.Routes[0]
	if len(route.Schedules) == 0 {
		return timetableByDayOfWeek{}, fmt.Errorf("no schedules found for %s from %s to %s in timetable", lineID, srcStation, destStation)
	}
	// creating journey cache
	journeys := map[string]*journey{}
	for _, si := range route.StationIntervals {
		stops := make([]stop, 0, len(si.Intervals))
		for _, interval := range si.Intervals {
			station, ok := stopsCache[interval.StopId]
			if !ok {
				return timetableByDayOfWeek{}, fmt.Errorf("station %s not found in cache while fetching timetable for %s from %s to %s", interval.StopId, lineID, srcStation, destStation)
			}
			stops = append(stops, stop{
				station:       station,
				timeToArrival: time.Minute * time.Duration(interval.TimeToArrival),
			})
		}
		j := &journey{
			stops: stops,
		}
		journeys[si.ID] = j
	}

	result := timetableByDayOfWeek{
		stops: stopsCache,
	}
	var defaultTimeTableDetails timeTableDetails
	for _, schedule := range route.Schedules {
		scheduledDepartures := make([]DepartureTime, 0, len(schedule.KnownJourneys))
		scheduleJourneys := make(map[departureTimeKey]*journey)
		for _, kj := range schedule.KnownJourneys {
			intervalID := strconv.Itoa(kj.IntervalId)
			journey, ok := journeys[intervalID]
			if !ok {
				return timetableByDayOfWeek{}, fmt.Errorf("didn't find interval ID: %s when processing timetable for %s from %s to %s", intervalID, lineID, srcStation, destStation)
			}
			depTime := DepartureTime{
				Hour:        kj.Hour,
				Minute:      kj.Minute,
				Destination: journey.stops[len(journey.stops)-1].station,
			}
			depTime.DestinationETA = calculateETA(depTime, journey.stops[len(journey.stops)-1].timeToArrival)
			scheduledDepartures = append(scheduledDepartures, depTime)
			scheduleJourneys[departureTimeKey{hour: depTime.Hour, minute: depTime.Minute}] = journey
		}
		defaultTimeTableDetails = timeTableDetails{
			scheduledDepartures: scheduledDepartures,
			journeys:            scheduleJourneys,
		}
		name := strings.ToLower(schedule.Name)
		switch {
		case strings.Contains(name, "friday"):
			result.fri = defaultTimeTableDetails
		case strings.Contains(name, "sunday"):
			result.sun = defaultTimeTableDetails
		case strings.Contains(name, "monday"):
			result.monToThu = defaultTimeTableDetails
		default:
			result.others = defaultTimeTableDetails
		}
	}
	if result.monToThu.journeys != nil {
		defaultTimeTableDetails = result.monToThu
	} else {
		log.Printf("WARNING: no schedules found for Mon-Thu for %s from %s to %s in timetable", lineID, srcStation, destStation)
		result.monToThu = defaultTimeTableDetails
	}
	if result.fri.journeys == nil {
		log.Printf("WARNING: no schedules found for Fri for %s from %s to %s in timetable", lineID, srcStation, destStation)
		result.fri = defaultTimeTableDetails
	}
	if result.sun.journeys == nil {
		log.Printf("WARNING: no schedules found for Sun for %s from %s to %s in timetable", lineID, srcStation, destStation)
		result.sun = defaultTimeTableDetails
	}
	if result.others.journeys == nil {
		log.Printf("WARNING: no schedules found for Sat/Others for %s from %s to %s in timetable", lineID, srcStation, destStation)
		result.others = defaultTimeTableDetails
	}

	return result, nil
}
