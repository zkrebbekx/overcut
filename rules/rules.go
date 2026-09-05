// Package rules implements the official F1 Fantasy scoring rules for 2026.
//
// The scoring tables live in a Config value. The Default function returns
// the 2026 tables. A caller can load a modified Config from JSON to track a
// mid-season rule change without a code change.
package rules

import (
	"encoding/json"
	"fmt"
	"os"
)

// Config holds every scoring table and constant for one season.
type Config struct {
	// QualiPoints maps a qualifying position (1-based index 0) to points.
	QualiPoints []int `json:"quali_points"`
	// QualiNoTime is the penalty when a driver sets no time or is
	// disqualified in qualifying.
	QualiNoTime int `json:"quali_no_time"`

	// RacePoints maps a race finish position to points.
	RacePoints []int `json:"race_points"`
	// PositionGained is the points per net position gained in the race.
	PositionGained int `json:"position_gained"`
	// PositionLost is the points per net position lost in the race
	// (a negative number).
	PositionLost int `json:"position_lost"`
	// Overtake is the points per on-track overtake in the race.
	Overtake int `json:"overtake"`
	// FastestLap is the bonus for the fastest race lap.
	FastestLap int `json:"fastest_lap"`
	// DriverOfTheDay is the bonus for the official vote award.
	DriverOfTheDay int `json:"driver_of_the_day"`
	// RaceDNF is the penalty for a DNF, NC, or DSQ in the race.
	RaceDNF int `json:"race_dnf"`

	// SprintPoints maps a sprint finish position to points.
	SprintPoints []int `json:"sprint_points"`
	// SprintQualiPoints maps a sprint-qualifying position to points.
	SprintQualiPoints []int `json:"sprint_quali_points"`
	// SprintFastestLap is the bonus for the fastest sprint lap.
	SprintFastestLap int `json:"sprint_fastest_lap"`
	// SprintDNF is the penalty for a DNF in the sprint.
	SprintDNF int `json:"sprint_dnf"`

	// ConstructorQualiBonus holds the progression bonuses, keyed by the
	// count of the team's drivers that reach Q2 and Q3.
	ConstructorQualiBonus QualiBonus `json:"constructor_quali_bonus"`
	// Q2Cutoff is the highest qualifying position that reaches Q2.
	// With 22 cars in 2026, the value is 16.
	Q2Cutoff int `json:"q2_cutoff"`
	// Q3Cutoff is the highest qualifying position that reaches Q3.
	Q3Cutoff int `json:"q3_cutoff"`

	// TransferPenalty is the points cost of one transfer above the free
	// allowance.
	TransferPenalty int `json:"transfer_penalty"`
	// FreeTransfers is the free transfer allowance per race week.
	FreeTransfers int `json:"free_transfers"`
	// Budget is the team budget in millions.
	Budget float64 `json:"budget"`
	// TeamDrivers is the required driver count.
	TeamDrivers int `json:"team_drivers"`
	// TeamConstructors is the required constructor count.
	TeamConstructors int `json:"team_constructors"`
}

// QualiBonus holds the constructor qualifying progression bonuses.
type QualiBonus struct {
	NoneInQ2 int `json:"none_in_q2"`
	OneInQ2  int `json:"one_in_q2"`
	BothInQ2 int `json:"both_in_q2"`
	OneInQ3  int `json:"one_in_q3"`
	BothInQ3 int `json:"both_in_q3"`
}

// Default returns the 2026 scoring configuration.
func Default() Config {
	return Config{
		QualiPoints: []int{10, 9, 8, 7, 6, 5, 4, 3, 2, 1},
		QualiNoTime: -5,

		RacePoints:     []int{25, 18, 15, 12, 10, 8, 6, 4, 2, 1},
		PositionGained: 1,
		PositionLost:   -1,
		Overtake:       1,
		FastestLap:     10,
		DriverOfTheDay: 10,
		RaceDNF:        -20,

		SprintPoints:      []int{8, 7, 6, 5, 4, 3, 2, 1},
		SprintQualiPoints: []int{8, 7, 6, 5, 4, 3, 2, 1},
		SprintFastestLap:  5,
		SprintDNF:         -10,

		ConstructorQualiBonus: QualiBonus{
			NoneInQ2: -1,
			OneInQ2:  1,
			BothInQ2: 3,
			OneInQ3:  5,
			BothInQ3: 10,
		},
		Q2Cutoff: 16,
		Q3Cutoff: 10,

		TransferPenalty:  -10,
		FreeTransfers:    2,
		Budget:           100.0,
		TeamDrivers:      5,
		TeamConstructors: 2,
	}
}

// Load reads a Config from a JSON file. Fields that the file omits keep the
// default value.
func Load(path string) (Config, error) {
	cfg := Default()
	b, err := os.ReadFile(path)
	if err != nil {
		return cfg, fmt.Errorf("rules: read %s: %w", path, err)
	}
	if err := json.Unmarshal(b, &cfg); err != nil {
		return cfg, fmt.Errorf("rules: parse %s: %w", path, err)
	}
	return cfg, nil
}

// DriverWeekend describes one driver's simulated or actual results for one
// race weekend.
type DriverWeekend struct {
	QualiPos    int  // 1-based final qualifying classification; 0 = no time
	GridPos     int  // 1-based race grid slot
	FinishPos   int  // 1-based race classification; ignored when DNF
	DNF         bool // DNF, NC, or DSQ in the race
	Overtakes   int  // on-track overtakes in the race
	FastestLap  bool
	DOTD        bool
	HasSprint   bool
	SprintQPos  int // 1-based sprint-qualifying position; 0 = none
	SprintGrid  int // 1-based sprint grid slot
	SprintPos   int // 1-based sprint classification; ignored when SprintDNF
	SprintDNF   bool
	SprintOvers int // on-track overtakes in the sprint
	SprintFL    bool
}

// positional returns the table value for a 1-based position, or zero when
// the position is outside the table.
func positional(table []int, pos int) int {
	if pos < 1 || pos > len(table) {
		return 0
	}
	return table[pos-1]
}

// DriverPoints computes one driver's fantasy points for one weekend.
func (c Config) DriverPoints(w DriverWeekend) int {
	pts := 0

	// Qualifying.
	if w.QualiPos == 0 {
		pts += c.QualiNoTime
	} else {
		pts += positional(c.QualiPoints, w.QualiPos)
	}

	// Sprint.
	if w.HasSprint {
		pts += positional(c.SprintQualiPoints, w.SprintQPos)
		if w.SprintDNF {
			pts += c.SprintDNF
		} else {
			pts += positional(c.SprintPoints, w.SprintPos)
			delta := w.SprintGrid - w.SprintPos
			if delta > 0 {
				pts += delta * c.PositionGained
			} else {
				pts += -delta * c.PositionLost
			}
			pts += w.SprintOvers * c.Overtake
			if w.SprintFL {
				pts += c.SprintFastestLap
			}
		}
	}

	// Race.
	if w.DNF {
		pts += c.RaceDNF
	} else {
		pts += positional(c.RacePoints, w.FinishPos)
		delta := w.GridPos - w.FinishPos
		if delta > 0 {
			pts += delta * c.PositionGained
		} else {
			pts += -delta * c.PositionLost
		}
		pts += w.Overtakes * c.Overtake
		if w.FastestLap {
			pts += c.FastestLap
		}
		if w.DOTD {
			pts += c.DriverOfTheDay
		}
	}

	return pts
}

// ConstructorQualiBonusFor computes the progression bonus from the count of
// the team's drivers that reached Q2 and Q3.
func (c Config) ConstructorQualiBonusFor(inQ2, inQ3 int) int {
	b := c.ConstructorQualiBonus
	switch {
	case inQ3 >= 2:
		return b.BothInQ3
	case inQ3 == 1:
		return b.OneInQ3
	case inQ2 >= 2:
		return b.BothInQ2
	case inQ2 == 1:
		return b.OneInQ2
	default:
		return b.NoneInQ2
	}
}

// ConstructorPoints computes one constructor's fantasy points for one
// weekend from its two drivers' weekends plus a pit-stop component.
//
// The pit-stop component is not derivable from public timing data. The
// caller supplies it; the model package estimates it from the residual
// between official constructor points and rule-derived points.
func (c Config) ConstructorPoints(a, b DriverWeekend, pitStopPts int) int {
	pts := c.DriverPoints(stripDriverOnly(a)) + c.DriverPoints(stripDriverOnly(b))

	inQ2, inQ3 := 0, 0
	for _, w := range []DriverWeekend{a, b} {
		if w.QualiPos >= 1 && w.QualiPos <= c.Q2Cutoff {
			inQ2++
		}
		if w.QualiPos >= 1 && w.QualiPos <= c.Q3Cutoff {
			inQ3++
		}
	}
	pts += c.ConstructorQualiBonusFor(inQ2, inQ3)
	pts += pitStopPts
	return pts
}

// stripDriverOnly removes the awards that only a driver scores.
func stripDriverOnly(w DriverWeekend) DriverWeekend {
	w.DOTD = false
	return w
}
