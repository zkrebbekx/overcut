// Package jolpica reads race data from the Jolpica F1 API, the maintained
// successor of the Ergast API.
package jolpica

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

// BaseURL is the API location. A test can point it at a local server.
var BaseURL = "https://api.jolpi.ca/ergast/f1"

// Race holds the schedule entry and the results of one round.
type Race struct {
	Season   string `json:"season"`
	Round    string `json:"round"`
	RaceName string `json:"raceName"`
	Date     string `json:"date"`
	Time     string `json:"time"`
	Circuit  struct {
		CircuitID string `json:"circuitId"`
	} `json:"Circuit"`
	Results          []Result      `json:"Results"`
	QualifyingResult []QualiResult `json:"QualifyingResults"`
	SprintResults    []Result      `json:"SprintResults"`

	FirstPractice    *ScheduleEntry `json:"FirstPractice"`
	SecondPractice   *ScheduleEntry `json:"SecondPractice"`
	ThirdPractice    *ScheduleEntry `json:"ThirdPractice"`
	Qualifying       *ScheduleEntry `json:"Qualifying"`
	Sprint           *ScheduleEntry `json:"Sprint"`
	SprintQualifying *ScheduleEntry `json:"SprintQualifying"`
}

// ScheduleEntry marks a session on the calendar. Time is in UTC and may
// be empty when the source has no time yet.
type ScheduleEntry struct {
	Date string `json:"date"`
	Time string `json:"time"`
}

// Result is one classified row of a race or sprint.
type Result struct {
	Position string `json:"position"`
	// PositionText is numeric for a classified car. The letters R, D, E,
	// and W mean retired, disqualified, excluded, and withdrawn.
	PositionText string `json:"positionText"`
	Grid         string `json:"grid"`
	Status       string `json:"status"`
	Laps         string `json:"laps"`
	Driver       Driver `json:"Driver"`
	Team         Team   `json:"Constructor"`
	FastestLap   *struct {
		Rank string `json:"rank"`
	} `json:"FastestLap"`
}

// QualiResult is one row of a qualifying classification.
type QualiResult struct {
	Position string `json:"position"`
	Driver   Driver `json:"Driver"`
	Team     Team   `json:"Constructor"`
	Q1       string `json:"Q1"`
	Q2       string `json:"Q2"`
	Q3       string `json:"Q3"`
}

// Driver identifies a driver.
type Driver struct {
	DriverID string `json:"driverId"`
	Code     string `json:"code"`
	Given    string `json:"givenName"`
	Family   string `json:"familyName"`
}

// Team identifies a constructor.
type Team struct {
	ConstructorID string `json:"constructorId"`
	Name          string `json:"name"`
}

type envelope struct {
	MRData struct {
		Total     string `json:"total"`
		RaceTable struct {
			Races []Race `json:"Races"`
		} `json:"RaceTable"`
	} `json:"MRData"`
}

// Int parses a numeric API string. The API encodes every number as a
// string. Int returns zero for an empty or malformed value.
func Int(s string) int {
	v, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return v
}

// Client fetches paginated Jolpica endpoints.
type Client struct {
	HTTP *http.Client
}

// New returns a Client with a 30-second timeout.
func New() *Client {
	return &Client{HTTP: &http.Client{Timeout: 30 * time.Second}}
}

// get fetches every page of one endpoint and returns the merged race list.
// Jolpica paginates by result row, so one round can span two pages; get
// merges rows into one Race per round.
func (c *Client) get(path string) ([]Race, error) {
	const pageSize = 100
	merged := map[string]*Race{}
	var order []string
	for offset := 0; ; offset += pageSize {
		url := fmt.Sprintf("%s/%s.json?limit=%d&offset=%d", BaseURL, path, pageSize, offset)
		resp, err := c.HTTP.Get(url)
		if err != nil {
			return nil, fmt.Errorf("jolpica: get %s: %w", url, err)
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("jolpica: read %s: %w", url, err)
		}
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("jolpica: get %s: status %d", url, resp.StatusCode)
		}
		var env envelope
		if err := json.Unmarshal(body, &env); err != nil {
			return nil, fmt.Errorf("jolpica: parse %s: %w", url, err)
		}
		for _, r := range env.MRData.RaceTable.Races {
			key := r.Season + "-" + r.Round
			if m, ok := merged[key]; ok {
				m.Results = append(m.Results, r.Results...)
				m.QualifyingResult = append(m.QualifyingResult, r.QualifyingResult...)
				m.SprintResults = append(m.SprintResults, r.SprintResults...)
			} else {
				rc := r
				merged[key] = &rc
				order = append(order, key)
			}
		}
		total := Int(env.MRData.Total)
		if offset+pageSize >= total {
			break
		}
	}
	races := make([]Race, 0, len(order))
	for _, key := range order {
		races = append(races, *merged[key])
	}
	return races, nil
}

// Schedule returns the calendar for one season.
func (c *Client) Schedule(season int) ([]Race, error) {
	return c.get(fmt.Sprintf("%d", season))
}

// Results returns every race result for one season.
func (c *Client) Results(season int) ([]Race, error) {
	return c.get(fmt.Sprintf("%d/results", season))
}

// Qualifying returns every qualifying result for one season.
func (c *Client) Qualifying(season int) ([]Race, error) {
	return c.get(fmt.Sprintf("%d/qualifying", season))
}

// Sprints returns every sprint result for one season.
func (c *Client) Sprints(season int) ([]Race, error) {
	return c.get(fmt.Sprintf("%d/sprint", season))
}
