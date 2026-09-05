// Package dataset joins the Jolpica race data and the F1 Fantasy feed into
// one normalized season file on disk. Every other command reads this file,
// so results stay deterministic and work offline after one sync.
package dataset

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/zkrebbekx/overcut/internal/feed"
	"github.com/zkrebbekx/overcut/internal/jolpica"
)

// Kind labels an asset.
type Kind string

// The two asset kinds.
const (
	KindDriver      Kind = "driver"
	KindConstructor Kind = "constructor"
)

// Data is the normalized season dataset.
type Data struct {
	Season   int       `json:"season"`
	SyncedAt time.Time `json:"synced_at"`
	Rounds   []Round   `json:"rounds"`
	Assets   []Asset   `json:"assets"`
}

// Round holds the schedule entry and the results of one round.
type Round struct {
	Round      int    `json:"round"`
	Name       string `json:"name"`
	CircuitID  string `json:"circuit_id"`
	Date       string `json:"date"`
	HasSprint  bool   `json:"has_sprint"`
	HasResults bool   `json:"has_results"`

	// Quali maps a driver TLA to the final qualifying position.
	Quali map[string]int `json:"quali,omitempty"`
	// Race maps a driver TLA to the race row.
	Race map[string]RaceRow `json:"race,omitempty"`
	// Sprint maps a driver TLA to the sprint row.
	Sprint map[string]RaceRow `json:"sprint,omitempty"`
}

// RaceRow is one driver's race or sprint outcome.
type RaceRow struct {
	Grid       int  `json:"grid"`
	Pos        int  `json:"pos"`
	DNF        bool `json:"dnf"`
	FastestLap bool `json:"fastest_lap,omitempty"`
}

// Asset is one fantasy asset with its full gameday history.
type Asset struct {
	ID       string       `json:"id"`
	Kind     Kind         `json:"kind"`
	Name     string       `json:"name"`
	TLA      string       `json:"tla,omitempty"`
	TeamID   string       `json:"team_id"`
	TeamName string       `json:"team_name"`
	History  []AssetRound `json:"history"`
}

// AssetRound is one asset's feed snapshot for one gameday.
type AssetRound struct {
	Gameday   int     `json:"gameday"`
	Active    bool    `json:"active"`
	Price     float64 `json:"price"`
	OldPrice  float64 `json:"old_price"`
	Ownership float64 `json:"ownership"`
	Points    float64 `json:"points"`

	QualiPts  float64 `json:"quali_pts"`
	SprintPts float64 `json:"sprint_pts"`
	RacePts   float64 `json:"race_pts"`

	Stats feed.AdditionalStats `json:"stats"`
}

// Latest returns the most recent gameday snapshot, or false when the asset
// has no history.
func (a Asset) Latest() (AssetRound, bool) {
	if len(a.History) == 0 {
		return AssetRound{}, false
	}
	return a.History[len(a.History)-1], true
}

// RoundHistory returns the snapshot for one gameday, or false.
func (a Asset) RoundHistory(gameday int) (AssetRound, bool) {
	for _, h := range a.History {
		if h.Gameday == gameday {
			return h, true
		}
	}
	return AssetRound{}, false
}

// classified reports whether a result row is a classified finish. A
// classified car carries a numeric positionText; a retired, disqualified,
// or withdrawn car carries a letter.
func classified(positionText string) bool {
	return positionText != "" && positionText[0] >= '0' && positionText[0] <= '9'
}

// Sync downloads the season from both sources and joins it.
func Sync(season int) (Data, error) {
	jc := jolpica.New()

	schedule, err := jc.Schedule(season)
	if err != nil {
		return Data{}, err
	}
	results, err := jc.Results(season)
	if err != nil {
		return Data{}, err
	}
	quali, err := jc.Qualifying(season)
	if err != nil {
		return Data{}, err
	}
	sprints, err := jc.Sprints(season)
	if err != nil {
		return Data{}, err
	}

	d := Data{Season: season, SyncedAt: time.Now().UTC()}

	rounds := map[int]*Round{}
	for _, r := range schedule {
		rd := &Round{
			Round:     jolpica.Int(r.Round),
			Name:      r.RaceName,
			CircuitID: r.Circuit.CircuitID,
			Date:      r.Date,
			HasSprint: r.Sprint != nil,
		}
		rounds[rd.Round] = rd
	}
	for _, r := range quali {
		rd := rounds[jolpica.Int(r.Round)]
		if rd == nil {
			continue
		}
		rd.Quali = map[string]int{}
		for _, q := range r.QualifyingResult {
			rd.Quali[q.Driver.Code] = jolpica.Int(q.Position)
		}
	}
	for _, r := range sprints {
		rd := rounds[jolpica.Int(r.Round)]
		if rd == nil {
			continue
		}
		rd.Sprint = map[string]RaceRow{}
		for _, s := range r.SprintResults {
			rd.Sprint[s.Driver.Code] = RaceRow{
				Grid: jolpica.Int(s.Grid),
				Pos:  jolpica.Int(s.Position),
				DNF:  !classified(s.PositionText),
			}
		}
	}
	for _, r := range results {
		rd := rounds[jolpica.Int(r.Round)]
		if rd == nil {
			continue
		}
		rd.HasResults = true
		rd.Race = map[string]RaceRow{}
		for _, res := range r.Results {
			row := RaceRow{
				Grid: jolpica.Int(res.Grid),
				Pos:  jolpica.Int(res.Position),
				DNF:  !classified(res.PositionText),
			}
			if res.FastestLap != nil && res.FastestLap.Rank == "1" {
				row.FastestLap = true
			}
			rd.Race[res.Driver.Code] = row
		}
	}
	for i := 1; i <= len(rounds); i++ {
		if rd, ok := rounds[i]; ok {
			d.Rounds = append(d.Rounds, *rd)
		}
	}

	// The feed publishes one document per gameday. Fetch until the feed
	// reports an unpublished gameday.
	assets := map[string]*Asset{}
	var order []string
	for g := 1; g <= len(rounds)+1; g++ {
		gd, err := feed.Fetch(nil, g)
		if errors.Is(err, feed.ErrNotPublished) {
			break
		}
		if err != nil {
			return Data{}, err
		}
		for _, p := range gd.Players {
			a, ok := assets[p.PlayerID]
			if !ok {
				kind := KindDriver
				if p.Skill != 1 {
					kind = KindConstructor
				}
				a = &Asset{
					ID:       p.PlayerID,
					Kind:     kind,
					Name:     p.FullName,
					TLA:      p.DriverTLA,
					TeamID:   p.TeamID,
					TeamName: p.TeamName,
				}
				assets[p.PlayerID] = a
				order = append(order, p.PlayerID)
			}
			ar := AssetRound{
				Gameday:   g,
				Active:    p.IsActive == "1",
				Price:     p.Value,
				OldPrice:  p.OldValue,
				Ownership: p.Ownership(),
				Points:    p.GamedayPoints(),
				Stats:     p.Stats,
			}
			// The feed labels the sprint-race session "Sprint Qualifying".
			// Sprint qualifying itself scores nothing, so both labels sum
			// into the sprint leg.
			for _, s := range p.Sessions {
				switch s.SessionType {
				case "Qualifying":
					ar.QualiPts = s.Points
				case "Sprint Qualifying", "Sprint":
					ar.SprintPts += s.Points
				case "Race":
					ar.RacePts = s.Points
				}
			}
			a.History = append(a.History, ar)
		}
	}
	for _, id := range order {
		d.Assets = append(d.Assets, *assets[id])
	}
	if len(d.Assets) == 0 {
		return Data{}, fmt.Errorf("dataset: the fantasy feed returned no assets")
	}
	return d, nil
}

// Save writes the dataset to path.
func Save(d Data, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("dataset: mkdir: %w", err)
	}
	b, err := json.MarshalIndent(d, "", " ")
	if err != nil {
		return fmt.Errorf("dataset: encode: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return fmt.Errorf("dataset: write: %w", err)
	}
	return os.Rename(tmp, path)
}

// Load reads the dataset from path.
func Load(path string) (Data, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Data{}, fmt.Errorf("dataset: read %s (run `overcut sync` first): %w", path, err)
	}
	var d Data
	if err := json.Unmarshal(b, &d); err != nil {
		return Data{}, fmt.Errorf("dataset: parse %s: %w", path, err)
	}
	return d, nil
}

// Drivers returns the driver assets.
func (d Data) Drivers() []Asset {
	var out []Asset
	for _, a := range d.Assets {
		if a.Kind == KindDriver {
			out = append(out, a)
		}
	}
	return out
}

// Constructors returns the constructor assets.
func (d Data) Constructors() []Asset {
	var out []Asset
	for _, a := range d.Assets {
		if a.Kind == KindConstructor {
			out = append(out, a)
		}
	}
	return out
}

// CompletedRounds returns the rounds that have race results, in order.
func (d Data) CompletedRounds() []Round {
	var out []Round
	for _, r := range d.Rounds {
		if r.HasResults {
			out = append(out, r)
		}
	}
	return out
}

// LatestGameday returns the highest gameday number in any asset history.
func (d Data) LatestGameday() int {
	max := 0
	for _, a := range d.Assets {
		if h, ok := a.Latest(); ok && h.Gameday > max {
			max = h.Gameday
		}
	}
	return max
}

// Selectable reports whether the asset can join a team now: the asset is
// active in the latest gameday snapshot. A driver that lost the seat in a
// mid-season swap stays in the history but is not selectable.
func (d Data) Selectable(a Asset) bool {
	h, ok := a.Latest()
	return ok && h.Gameday == d.LatestGameday() && h.Active
}

// NextRound returns the first round without race results, or false when the
// season is complete.
func (d Data) NextRound() (Round, bool) {
	for _, r := range d.Rounds {
		if !r.HasResults {
			return r, true
		}
	}
	return Round{}, false
}
