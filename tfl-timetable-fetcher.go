package tfl

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"strconv"
	"strings"
	"time"
)

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
		stops:     stopsCache,
		createdOn: time.Now(),
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
			depTime.DestinationETA = calculateETAFromDepTime(depTime, journey.stops[len(journey.stops)-1].timeToArrival)
			scheduledDepartures = append(scheduledDepartures, depTime)
			scheduleJourneys[departureTimeKey{hour: depTime.Hour, minute: depTime.Minute}] = journey
		}
		defaultTimeTableDetails = timeTableDetails{
			scheduleName:        schedule.Name,
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
