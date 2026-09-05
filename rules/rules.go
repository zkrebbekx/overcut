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

	// SprintPoints maps a sprint finish position to points. Sprint
	// qualifying scores no points.
	SprintPoints []int `json:"sprint_points"`
	// SprintMaxLost caps the positions lost in the sprint.
	SprintMaxLost int `json:"sprint_max_lost"`
	// SprintFastestLap is the bonus for the fastest sprint lap.
	SprintFastestLap int `json:"sprint_fastest_lap"`
	// SprintDNF is the penalty for a DNF in the sprint.
	SprintDNF int `json:"sprint_dnf"`

	// ConstructorDSQ holds the extra constructor penalty per disqualified
	// driver, by session.
	ConstructorDSQ SessionPenalty `json:"constructor_dsq"`
	// PitStopPoints maps an upper time bound in seconds to points for the
	// constructor's pit stop. The first band whose bound exceeds the time
	// applies.
	PitStopPoints []PitStopBand `json:"pit_stop_points"`
	// FastestPitStop is the bonus for the fastest stop of the race.
	FastestPitStop int `json:"fastest_pit_stop"`
	// RecordPitStop is the bonus for a new world-record stop.
	RecordPitStop int `json:"record_pit_stop"`
	// RecordPitStopTime is the record to beat, in seconds.
	RecordPitStopTime float64 `json:"record_pit_stop_time"`

	// PriceFloor and PriceCap bound every asset price, in millions.
	PriceFloor float64 `json:"price_floor"`
	PriceCap   float64 `json:"price_cap"`
	// PriceFormRounds is the count of grands prix whose average fantasy
	// performance drives a price change.
	PriceFormRounds int `json:"price_form_rounds"`

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
	// MaxCarryOver is the most unused transfers that carry to the next
	// race. A carried transfer does not accumulate further.
	MaxCarryOver int `json:"max_carry_over"`
	// BoostMultiplier is the regular Boost on one driver, every race.
	BoostMultiplier int `json:"boost_multiplier"`
	// ExtraBoostMultiplier is the x3 chip multiplier. The chip goes on a
	// second driver; the regular Boost stays on another.
	ExtraBoostMultiplier int `json:"extra_boost_multiplier"`
	// Budget is the team budget in millions.
	Budget float64 `json:"budget"`
	// TeamDrivers is the required driver count.
	TeamDrivers int `json:"team_drivers"`
	// TeamConstructors is the required constructor count.
	TeamConstructors int `json:"team_constructors"`
}

// SessionPenalty holds one penalty value per session type.
type SessionPenalty struct {
	Quali  int `json:"quali"`
	Sprint int `json:"sprint"`
	Race   int `json:"race"`
}

// PitStopBand is one row of the pit-stop table: stops faster than
// UnderSeconds score Points, unless a faster band applies.
type PitStopBand struct {
	UnderSeconds float64 `json:"under_seconds"`
	Points       int     `json:"points"`
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

		SprintPoints:     []int{8, 7, 6, 5, 4, 3, 2, 1},
		SprintMaxLost:    10,
		SprintFastestLap: 5,
		SprintDNF:        -10,

		ConstructorDSQ: SessionPenalty{Quali: -5, Sprint: -10, Race: -20},
		PitStopPoints: []PitStopBand{
			{UnderSeconds: 2.00, Points: 20},
			{UnderSeconds: 2.20, Points: 10},
			{UnderSeconds: 2.50, Points: 5},
			{UnderSeconds: 3.00, Points: 2},
		},
		FastestPitStop:    5,
		RecordPitStop:     15,
		RecordPitStopTime: 1.80,

		PriceFloor:      3.0,
		PriceCap:        34.0,
		PriceFormRounds: 3,

		ConstructorQualiBonus: QualiBonus{
			NoneInQ2: -1,
			OneInQ2:  1,
			BothInQ2: 3,
			OneInQ3:  5,
			BothInQ3: 10,
		},
		Q2Cutoff: 16,
		Q3Cutoff: 10,

		TransferPenalty:      -10,
		FreeTransfers:        2,
		MaxCarryOver:         1,
		BoostMultiplier:      2,
		ExtraBoostMultiplier: 3,
		Budget:               100.0,
		TeamDrivers:          5,
		TeamConstructors:     2,
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
	QualiPos  int  // 1-based final qualifying classification; 0 = no time
	QualiDSQ  bool // disqualified from qualifying
	GridPos   int  // 1-based official starting grid slot
	FinishPos int  // 1-based race classification; ignored when DNF
	DNF       bool // DNF, NC, or DSQ in the race
	RaceDSQ   bool // disqualified from the race (a DNF for the driver)
	// Overtakes counts on-track overtakes in the race. They score even
	// when the driver retires later.
	Overtakes  int
	FastestLap bool
	DOTD       bool

	HasSprint   bool
	SprintGrid  int // 1-based sprint grid slot
	SprintPos   int // 1-based sprint classification; ignored when SprintDNF
	SprintDNF   bool
	SprintDSQ   bool
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
	if w.QualiPos == 0 || w.QualiDSQ {
		pts += c.QualiNoTime
	} else {
		pts += positional(c.QualiPoints, w.QualiPos)
	}

	// Sprint. Sprint qualifying scores nothing. Overtakes score even when
	// the driver retires. An unclassified driver takes the penalty and no
	// position points.
	if w.HasSprint {
		pts += w.SprintOvers * c.Overtake
		if w.SprintDNF || w.SprintDSQ {
			pts += c.SprintDNF
		} else {
			pts += positional(c.SprintPoints, w.SprintPos)
			delta := w.SprintGrid - w.SprintPos
			if delta > 0 {
				pts += delta * c.PositionGained
			} else {
				lost := -delta
				if lost > c.SprintMaxLost {
					lost = c.SprintMaxLost
				}
				pts += lost * c.PositionLost
			}
			if w.SprintFL {
				pts += c.SprintFastestLap
			}
		}
	}

	// Race. Same structure; positions lost are not capped.
	pts += w.Overtakes * c.Overtake
	if w.DNF || w.RaceDSQ {
		pts += c.RaceDNF
	} else {
		pts += positional(c.RacePoints, w.FinishPos)
		delta := w.GridPos - w.FinishPos
		if delta > 0 {
			pts += delta * c.PositionGained
		} else {
			pts += -delta * c.PositionLost
		}
		if w.FastestLap {
			pts += c.FastestLap
		}
		if w.DOTD {
			pts += c.DriverOfTheDay
		}
	}

	return pts
}

// PitStopPointsFor returns the points for a constructor's pit stop of the
// given duration in seconds.
func (c Config) PitStopPointsFor(seconds float64) int {
	for _, band := range c.PitStopPoints {
		if seconds < band.UnderSeconds {
			return band.Points
		}
	}
	return 0
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
// weekend: the combined total of its two drivers (without the
// driver-of-the-day bonus), the qualifying progression bonus, the extra
// disqualification penalties, plus a pit-stop component.
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
		if w.QualiDSQ {
			pts += c.ConstructorDSQ.Quali
		}
		if w.HasSprint && w.SprintDSQ {
			pts += c.ConstructorDSQ.Sprint
		}
		if w.RaceDSQ {
			pts += c.ConstructorDSQ.Race
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
