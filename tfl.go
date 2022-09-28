package tfl

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"
)

var TFLStaticDataGlobal TFLStaticData = newStaticData()

type TFLStaticData interface {
	Lines(string) []Line
	LineDetails(string, string) Line
	Stations(string) []Station
	Routes(string) []Route
}

type Line struct {
	ID     string
	Name   string
	Status Status
}

type Status struct {
	StatusDescriptions []string
}

type Route struct {
	ID       string
	Name     string
	Stations []Station
}

type Station struct {
	ID       string
	Name     string
	Lat, Lon float64
}

type staticData struct {
	fetcher         *staticFetcher
	lineRequests    chan lineRequest
	stationRequests chan stationRequest
	routeRequests   chan routeRequest
}

type lineRequest struct {
	mode   string
	lineID string
	resp   chan []Line
}

type stationRequest struct {
	lineID string
	resp   chan []Station
}

type routeRequest struct {
	lineID string
	resp   chan []Route
}

func newStaticData() *staticData {
	result := &staticData{
		lineRequests:    make(chan lineRequest),
		stationRequests: make(chan stationRequest),
		routeRequests:   make(chan routeRequest),
		fetcher:         newStaticFetcher(),
	}
	go result.monitorLineFetch()
	go result.monitorStationFetch()
	go result.monitorRouteFetch()
	return result
}

func (sd *staticData) monitorLineFetch() {
	lines := map[string][]Line{}
	linesCache := make(map[string]Line)
	for req := range sd.lineRequests {
		mode := req.mode
		linesForMode, ok := lines[mode]
		if ok {
			respondToLineRequest(req, linesForMode, linesCache)
			continue
		}
		_lines, err := sd.fetcher.fetchLines(req.mode)
		if err != nil {
			log.Printf("ERROR fetching lines: %v", err)
			req.resp <- []Line{}
			continue
		}
		lines[mode] = _lines
		for _, l := range _lines {
			linesCache[l.ID] = l
		}
		respondToLineRequest(req, _lines, linesCache)
	}
}

func respondToLineRequest(req lineRequest, lines []Line, linesCache map[string]Line) {
	if req.lineID == "" {
		req.resp <- lines
	} else {
		line, ok := linesCache[req.lineID]
		if ok {
			req.resp <- []Line{line}
		} else {
			req.resp <- []Line{
				{ID: req.lineID, Name: req.lineID},
			}
		}
	}
}

func (sd *staticData) monitorStationFetch() {
	stations := map[string][]Station{}
	for req := range sd.stationRequests {
		v, ok := stations[req.lineID]
		if ok {
			req.resp <- v
			continue
		}
		_stations, err := sd.fetcher.fetchStation(req.lineID)
		if err != nil {
			log.Printf("ERROR fetching stations: %v", err)
			req.resp <- []Station{}
			continue
		}
		stations[req.lineID] = _stations
		req.resp <- _stations
	}
}

func (sd *staticData) monitorRouteFetch() {
	routes := map[string][]Route{}
	for req := range sd.routeRequests {
		v, ok := routes[req.lineID]
		if ok {
			req.resp <- v
			continue
		}
		_routes, err := sd.fetcher.fetchRoutes(req.lineID)
		if err != nil {
			log.Printf("ERROR fetching routes: %v", err)
			req.resp <- []Route{}
			continue
		}
		routes[req.lineID] = _routes
		req.resp <- _routes
	}
}

func (sd *staticData) Lines(mode string) []Line {
	lines := sd.lines(mode)
	if len(lines) == 0 {
		return lines
	}
	statuses, err := sd.fetcher.fetchStatus(mode)
	if err != nil {
		log.Printf("error getting status: %v", err)
		return lines
	}
	result := make([]Line, 0, len(lines))
	for _, l := range lines {
		l.Status = statuses[l.ID]
		result = append(result, l)
	}
	return result
}

func (sd *staticData) lines(mode string) []Line {
	resp := make(chan []Line, 1)
	req := lineRequest{resp: resp, mode: mode}
	select {
	case sd.lineRequests <- req:
		return <-resp
	case <-time.After(time.Second * 5):
		log.Printf("timeout waiting for remote request (line fetch)... processing one-off")
	}
	lines, err := sd.fetcher.fetchLines(mode)
	if err != nil {
		log.Printf("ERROR fetching lines during one-off: %v", err)
		return []Line{}
	}
	return lines
}

func (sd *staticData) LineDetails(mode, lineID string) Line {
	if lineID == "" {
		log.Printf("WARNING: LineDetails called without lineID")
		return Line{}
	}
	if mode == "" {
		log.Printf("WARNING: LineDetails called without mode")
		return Line{
			ID:   lineID,
			Name: lineID,
		}
	}
	resp := make(chan []Line, 1)
	req := lineRequest{resp: resp, mode: mode, lineID: lineID}
	select {
	case sd.lineRequests <- req:
		v := <-resp
		if len(v) != 1 {
			return Line{}
		}
		return v[0]
	case <-time.After(time.Second * 5):
		log.Printf("timeout waiting for remote request (line details fetch)... aborting")
	}
	return Line{}
}

func (sd *staticData) Stations(lineID string) []Station {
	resp := make(chan []Station, 1)
	req := stationRequest{lineID: lineID, resp: resp}
	select {
	case sd.stationRequests <- req:
		return <-resp
	case <-time.After(time.Second * 5):
		log.Printf("timeout waiting for remote request (stations fetch)... processing one-off")
	}
	stations, err := sd.fetcher.fetchStation(lineID)
	if err != nil {
		log.Printf("ERROR fetching stations during one-off: %v", err)
		return []Station{}
	}
	return stations
}

func (sd *staticData) Routes(lineID string) []Route {
	resp := make(chan []Route, 1)
	req := routeRequest{lineID: lineID, resp: resp}
	select {
	case sd.routeRequests <- req:
		return <-resp
	case <-time.After(time.Second * 5):
		log.Printf("timeout waiting for remote request (routes fetch)... processing one-off")
	}
	routes, err := sd.fetcher.fetchRoutes(lineID)
	if err != nil {
		log.Printf("ERROR fetching routes during one-off: %v", err)
		return []Route{}
	}
	return routes
}

type staticFetcher struct {
	c             http.Client
	linesURL      func(string) string
	stationsURL   func(string) string
	routesURL     func(string) string
	statusURL     func(string) string
	statusByIDURL func([]string) string
}

func newStaticFetcher() *staticFetcher {
	c := http.Client{Timeout: time.Duration(5) * time.Second}
	return &staticFetcher{
		c: c,
		linesURL: func(mode string) string {
			return fmt.Sprintf(LineRoutesAPI, mode)
		},
		stationsURL: func(lineID string) string {
			return fmt.Sprintf(LineStationsAPI, lineID)
		},
		routesURL: func(lineID string) string {
			return fmt.Sprintf(LineStationSequenceAPI, lineID)
		},
		statusURL: func(mode string) string {
			return fmt.Sprintf(LineStatusAPI, mode)
		},
		statusByIDURL: func(ids []string) string {
			return fmt.Sprintf(LineStatusByIDAPI, strings.Join(ids, ","))
		},
	}
}

type tflLine struct {
	ID   string
	Name string
}

type routeSection struct {
	Name string
}

func (sf *staticFetcher) fetchLines(mode string) ([]Line, error) {
	url := sf.linesURL(mode)
	resp, err := sf.c.Get(url)
	if err != nil {
		return []Line{}, fmt.Errorf("problem fetching lines data from API: %v", err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return []Line{}, fmt.Errorf("problem reading lines data from response: %v", err)
	}
	tflLines := []tflLine{}
	if err := json.Unmarshal(body, &tflLines); err != nil {
		return []Line{}, fmt.Errorf("problem parsing lines response data from TFL: %v", err)
	}
	result := make([]Line, 0, len(tflLines))
	for _, tflLine := range tflLines {
		result = append(result, Line{
			ID:   tflLine.ID,
			Name: tflLine.Name,
		})
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})
	return result, nil
}

type tflStation struct {
	Id         string
	CommonName string
	Lat, Lon   float64
}

func (sf *staticFetcher) fetchStation(lineID string) ([]Station, error) {
	url := sf.stationsURL(lineID)
	resp, err := sf.c.Get(url)
	if err != nil {
		return []Station{}, fmt.Errorf("problem fetching station data for %s from API: %v", lineID, err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return []Station{}, fmt.Errorf("problem reading station data for %s from response: %v", lineID, err)
	}
	stations := []tflStation{}
	if err := json.Unmarshal(body, &stations); err != nil {
		return []Station{}, fmt.Errorf("problem parsing station data response data for %s from TFL: %v", lineID, err)
	}
	result := make([]Station, 0, len(stations))
	for _, s := range stations {
		result = append(result, Station{
			ID:   s.Id,
			Name: s.CommonName,
			Lat:  s.Lat,
			Lon:  s.Lon,
		})
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result, nil
}

type tflRouteSequence struct {
	OrderedLineRoutes []tflLineRoute
}

type tflLineRoute struct {
	Name      string
	NaptanIds []string
}

func (sf *staticFetcher) fetchRoutes(lineID string) ([]Route, error) {
	// Get stations first and create a hashmap
	allStations, err := sf.fetchStation(lineID)
	if err != nil {
		return nil, fmt.Errorf("error fetching stations while fetching routes: %v", err)
	}
	stationsMap := make(map[string]Station)
	for _, s := range allStations {
		stationsMap[s.ID] = s
	}

	// Now move on to routes
	url := sf.routesURL(lineID)
	resp, err := sf.c.Get(url)
	if err != nil {
		return []Route{}, fmt.Errorf("problem fetching routes data for %s from API: %v", lineID, err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return []Route{}, fmt.Errorf("problem reading routes data for %s from response: %v", lineID, err)
	}
	routeSequence := tflRouteSequence{}
	if err := json.Unmarshal(body, &routeSequence); err != nil {
		return []Route{}, fmt.Errorf("problem parsing routes data response data for %s from TFL: %v", lineID, err)
	}

	// Prepare final output
	result := make([]Route, 0, len(routeSequence.OrderedLineRoutes))
	for i, olr := range routeSequence.OrderedLineRoutes {
		stations := make([]Station, 0, len(olr.NaptanIds))
		for _, stationID := range olr.NaptanIds {
			station, ok := stationsMap[stationID]
			if !ok {
				log.Printf("station with ID %s found in route %s but not in collection", stationID, olr.Name)
				continue
			}
			stations = append(stations, station)
		}
		result = append(result, Route{
			ID:       fmt.Sprintf("route%s%d", lineID, i),
			Name:     olr.Name,
			Stations: stations,
		})
	}
	return result, nil
}

type tflStatus struct {
	Id           string
	LineStatuses []tflLineStatus
}

type tflLineStatus struct {
	StatusSeverityDescription string
	Reason                    string
}

func (s tflStatus) statusDescriptions() []string {
	result := make([]string, 0, len(s.LineStatuses))
	for _, v := range s.LineStatuses {
		if v.Reason == "" {
			result = append(result, v.StatusSeverityDescription)
		} else {
			result = append(result, v.Reason)
		}
	}
	return result
}

func (sf *staticFetcher) fetchStatus(mode string) (map[string]Status, error) {
	url := sf.statusURL(mode)
	resp, err := sf.c.Get(url)
	if err != nil {
		return nil, fmt.Errorf("problem fetching status data from API: %v", err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("problem reading status data from response: %v", err)
	}
	statuses := []tflStatus{}
	if err := json.Unmarshal(body, &statuses); err != nil {
		return nil, fmt.Errorf("problem parsing status data from TFL: %v", err)
	}
	result := make(map[string]Status)
	for _, s := range statuses {
		result[s.Id] = Status{
			StatusDescriptions: s.statusDescriptions(),
		}
	}
	return result, nil
}

func (sf *staticFetcher) fetchStatusByIDs(ids []string) (map[string]Status, error) {
	url := sf.statusByIDURL(ids)
	resp, err := sf.c.Get(url)
	if err != nil {
		return nil, fmt.Errorf("problem fetching status data by ID from API: %v", err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("problem reading status data by ID from response: %v", err)
	}
	statuses := []tflStatus{}
	if err := json.Unmarshal(body, &statuses); err != nil {
		return nil, fmt.Errorf("problem parsing status data by ID from TFL: %v", err)
	}
	result := make(map[string]Status)
	for _, s := range statuses {
		result[s.Id] = Status{
			StatusDescriptions: s.statusDescriptions(),
		}
	}
	return result, nil
}
