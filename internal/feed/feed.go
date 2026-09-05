// Package feed reads the public F1 Fantasy game feed.
//
// The feed publishes one JSON document per gameday at
// https://fantasy.formula1.com/feeds/drivers/<gameday>_en.json. Each
// document lists every driver and constructor with the current price, the
// previous price, the ownership percentage, and official fantasy points per
// session.
package feed

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

// BaseURL is the feed location. A test can point it at a local server.
var BaseURL = "https://fantasy.formula1.com/feeds/drivers"

// Player is one asset (driver or constructor) in one gameday document.
type Player struct {
	PlayerID     string          `json:"PlayerId"`
	Skill        int             `json:"Skill"` // 1 = driver, 2 = constructor
	PositionName string          `json:"PositionName"`
	Value        float64         `json:"Value"`
	OldValue     float64         `json:"OldPlayerValue"`
	FullName     string          `json:"FUllName"`
	DisplayName  string          `json:"DisplayName"`
	TeamName     string          `json:"TeamName"`
	TeamID       string          `json:"TeamId"`
	IsActive     string          `json:"IsActive"`
	DriverTLA    string          `json:"DriverTLA"`
	OverallPts   string          `json:"OverallPpints"`
	GamedayPts   string          `json:"GamedayPoints"`
	Selected     string          `json:"SelectedPercentage"`
	Sessions     []SessionPoints `json:"SessionWisePoints"`
	Stats        AdditionalStats `json:"AdditionalStats"`
}

// SessionPoints is the official points for one session of one gameday.
type SessionPoints struct {
	SessionNumber int     `json:"sessionnumber"`
	SessionType   string  `json:"sessiontype"`
	Points        float64 `json:"points"`
}

// AdditionalStats is the season-to-date component breakdown for one asset.
type AdditionalStats struct {
	FastestLapPts float64 `json:"fastest_lap_pts"`
	DOTDPts       float64 `json:"dotd_pts"`
	OvertakingPts float64 `json:"overtaking_pts"`
	Q3FinishesPts float64 `json:"q3_finishes_pts"`
	PositionPts   float64 `json:"total_position_pts"`
	PosGainedLost float64 `json:"total_position_gained_lost"`
	DNFPts        float64 `json:"total_dnf_dq_pts"`
	ValueForMoney float64 `json:"value_for_money"`
	Top10RacePts  float64 `json:"top10_race_position_pts"`
	Top8SprintPts float64 `json:"top8_sprint_position_pts"`
}

// Gameday is one parsed feed document.
type Gameday struct {
	Gameday int
	Players []Player
}

type envelope struct {
	Data struct {
		Value []Player `json:"Value"`
	} `json:"Data"`
}

// GamedayPoints returns the asset's official total points for the gameday.
func (p Player) GamedayPoints() float64 {
	v, err := strconv.ParseFloat(p.GamedayPts, 64)
	if err != nil {
		return 0
	}
	return v
}

// OverallPoints returns the asset's official season points through the
// gameday.
func (p Player) OverallPoints() float64 {
	v, err := strconv.ParseFloat(p.OverallPts, 64)
	if err != nil {
		return 0
	}
	return v
}

// Ownership returns the selection percentage as a number.
func (p Player) Ownership() float64 {
	v, err := strconv.ParseFloat(p.Selected, 64)
	if err != nil {
		return 0
	}
	return v
}

// Fetch downloads and parses one gameday document. Fetch returns
// ErrNotPublished when the game has not published the gameday yet.
func Fetch(client *http.Client, gameday int) (Gameday, error) {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	url := fmt.Sprintf("%s/%d_en.json", BaseURL, gameday)
	resp, err := client.Get(url)
	if err != nil {
		return Gameday{}, fmt.Errorf("feed: get %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusNotFound {
		return Gameday{}, ErrNotPublished
	}
	if resp.StatusCode != http.StatusOK {
		return Gameday{}, fmt.Errorf("feed: get %s: status %d", url, resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return Gameday{}, fmt.Errorf("feed: read %s: %w", url, err)
	}
	var env envelope
	if err := json.Unmarshal(body, &env); err != nil {
		return Gameday{}, fmt.Errorf("feed: parse %s: %w", url, err)
	}
	return Gameday{Gameday: gameday, Players: env.Data.Value}, nil
}

// ErrNotPublished reports that the feed has no document for the gameday.
var ErrNotPublished = fmt.Errorf("feed: gameday not published")
