package tfl

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"sort"
	"time"
)

var TFLStaticDataGlobal TFLStaticData = newStaticData()

type TFLStaticData interface {
	Lines() []Line
	LineDetails(string) Line
	Stations(string) []Station
}

type Line struct {
	ID     string
	Name   string
	Routes []string
	Status Status
}

type Status struct {
	StatusDescriptions []string
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
}

type lineRequest struct {
	lineID string
	resp   chan []Line
}

type stationRequest struct {
	lineID string
	resp   chan []Station
}

func newStaticData() *staticData {
	result := &staticData{
		lineRequests:    make(chan lineRequest),
		stationRequests: make(chan stationRequest),
		fetcher:         newStaticFetcher(),
	}
	go result.monitorLineFetch()
	go result.monitorStationFetch()
	return result
}

func (sd *staticData) monitorLineFetch() {
	lines := []Line{}
	linesCache := make(map[string]Line)
	linesFetched := false
	for req := range sd.lineRequests {
		if linesFetched {
			if req.lineID == "" {
				req.resp <- lines
			} else {
				req.resp <- []Line{linesCache[req.lineID]}
			}
			continue
		}
		_lines, err := sd.fetcher.fetchLines()
		if err != nil {
			log.Printf("ERROR fetching lines: %v", err)
			req.resp <- []Line{}
			continue
		}
		lines = _lines
		linesFetched = true
		for _, l := range lines {
			linesCache[l.ID] = l
		}
		req.resp <- lines
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

func (sd *staticData) Lines() []Line {
	lines := sd.lines()
	if len(lines) == 0 {
		return lines
	}
	statuses, err := sd.fetcher.fetchStatus()
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

func (sd *staticData) lines() []Line {
	resp := make(chan []Line, 1)
	req := lineRequest{resp: resp}
	select {
	case sd.lineRequests <- req:
		return <-resp
	case <-time.After(time.Second * 5):
		log.Printf("timeout waiting for remote request (line fetch)... processing one-off")
	}
	lines, err := sd.fetcher.fetchLines()
	if err != nil {
		log.Printf("ERROR fetching lines during one-off: %v", err)
		return []Line{}
	}
	return lines
}

func (sd *staticData) LineDetails(lineID string) Line {
	resp := make(chan []Line, 1)
	req := lineRequest{resp: resp, lineID: lineID}
	select {
	case sd.lineRequests <- req:
		v := <-resp
		return v[0]
	case <-time.After(time.Second * 5):
		log.Printf("timeout waiting for remote request (line details fetch)... processing one-off")
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

type staticFetcher struct {
	c           http.Client
	linesURL    func() string
	stationsURL func(string) string
	statusURL   func() string
}

func newStaticFetcher() *staticFetcher {
	c := http.Client{Timeout: time.Duration(5) * time.Second}
	return &staticFetcher{
		c: c,
		linesURL: func() string {
			return LineRoutesAPI
		},
		stationsURL: func(lineID string) string {
			return fmt.Sprintf(LineStationsAPI, lineID)
		},
		statusURL: func() string {
			return LineStatusAPI
		},
	}
}

type tflLine struct {
	ID            string
	Name          string
	RouteSections []routeSection
}

type routeSection struct {
	Name string
}

func (tl tflLine) routeSectionsAsList() []string {
	result := make([]string, 0, len(tl.RouteSections))
	for _, rs := range tl.RouteSections {
		result = append(result, rs.Name)
	}
	return result
}

func (sf *staticFetcher) fetchLines() ([]Line, error) {
	url := sf.linesURL()
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
			ID:     tflLine.ID,
			Name:   tflLine.Name,
			Routes: tflLine.routeSectionsAsList(),
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

func (sf *staticFetcher) fetchStatus() (map[string]Status, error) {
	url := sf.statusURL()
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
