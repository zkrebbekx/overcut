// Package model fits a pace model to the season data and projects fantasy
// points with a Monte Carlo simulation.
//
// The fit uses two independent sources and calibrates one against the
// other:
//
//   - Jolpica supplies the qualifying, sprint, and race classifications.
//     The fit turns them into a per-driver pace estimate with an
//     exponentially-weighted moving average (EWMA), so recent form counts
//     more than old form.
//   - The official fantasy feed supplies the true fantasy points per asset
//     per gameday. The fit uses them to estimate the components that public
//     timing data cannot derive: overtake points, driver-of-the-day votes,
//     and the constructor pit-stop component. The pit-stop component is the
//     EWMA of the residual between official constructor points and
//     rule-derived points, so a small error in a scoring table corrects
//     itself from the official numbers.
package model

import (
	"math"
	"sort"

	"github.com/zkrebbekx/overcut/internal/dataset"
	"github.com/zkrebbekx/overcut/rules"
)

// HalfLife is the form half-life in rounds. A result N rounds old carries
// weight 0.5^(N/HalfLife).
const HalfLife = 4.0

// DriverModel is the fitted state of one driver.
type DriverModel struct {
	AssetID string
	TLA     string
	Name    string
	TeamID  string

	QualiMu float64 // EWMA qualifying position
	QualiSD float64 // spread of qualifying position
	RaceMu  float64 // EWMA race finish position among classified cars
	RaceSD  float64 // spread of race finish position

	DNFProb        float64 // per-race retirement probability
	OvertakeLambda float64 // expected overtake points per weekend
	FLProb         float64 // fastest-lap probability
	DOTDProb       float64 // driver-of-the-day probability

	Rounds int // rounds with data
}

// ConstructorModel is the fitted state of one constructor.
type ConstructorModel struct {
	AssetID  string
	Name     string
	TeamID   string
	TLAs     []string // its drivers
	PitResid float64  // EWMA residual: official points minus derived points
}

// Model is a full fitted season model.
type Model struct {
	Rules        rules.Config
	Drivers      []DriverModel
	Constructors []ConstructorModel
}

// weight returns the EWMA weight for a result that is age rounds old.
func weight(age int) float64 {
	return math.Pow(0.5, float64(age)/HalfLife)
}

// ewma computes a weighted mean and a shrunk weighted standard deviation
// over (value, age) observations. The standard deviation shrinks toward
// priorSD with a prior strength of two observations.
func ewma(values []float64, ages []int, priorSD float64) (mu, sd float64, n float64) {
	var sw, swx float64
	for i, v := range values {
		w := weight(ages[i])
		sw += w
		swx += w * v
	}
	if sw == 0 {
		return 0, priorSD, 0
	}
	mu = swx / sw
	var swv float64
	for i, v := range values {
		w := weight(ages[i])
		swv += w * (v - mu) * (v - mu)
	}
	const priorStrength = 2.0
	variance := (swv + priorStrength*priorSD*priorSD) / (sw + priorStrength)
	return mu, math.Sqrt(variance), sw
}

// shrunkRate computes a smoothed event rate with a Beta-style prior.
func shrunkRate(events, trials, priorRate, priorStrength float64) float64 {
	return (events + priorRate*priorStrength) / (trials + priorStrength)
}

// componentDiff returns the per-gameday increase of a cumulative
// season-to-date statistic.
func componentDiff(history []dataset.AssetRound, get func(feedStats dataset.AssetRound) float64) map[int]float64 {
	out := map[int]float64{}
	prev := 0.0
	for _, h := range history {
		out[h.Gameday] = get(h) - prev
		prev = get(h)
	}
	return out
}

// Fit builds a Model from every completed round up to and including
// throughRound. Pass a large value to use the full dataset.
func Fit(d dataset.Data, cfg rules.Config, throughRound int) Model {
	m := Model{Rules: cfg}

	var completed []dataset.Round
	for _, r := range d.CompletedRounds() {
		if r.Round <= throughRound {
			completed = append(completed, r)
		}
	}
	latest := 0
	if len(completed) > 0 {
		latest = completed[len(completed)-1].Round
	}

	// Field-level priors from the completed rounds.
	fieldDNF := fieldDNFRate(completed)

	byTeam := map[string][]string{}

	for _, a := range d.Drivers() {
		// A driver that lost the seat in a mid-season swap stays in the
		// dataset but takes no part in the simulated field.
		if !d.Selectable(a) {
			continue
		}
		dm := DriverModel{AssetID: a.ID, TLA: a.TLA, Name: a.Name, TeamID: a.TeamID}

		var qPos, rPos []float64
		var qAge, rAge []int
		dnfs, races, fls := 0.0, 0.0, 0.0
		for _, r := range completed {
			age := latest - r.Round
			if p, ok := r.Quali[a.TLA]; ok && p > 0 {
				qPos = append(qPos, float64(p))
				qAge = append(qAge, age)
			}
			if row, ok := r.Race[a.TLA]; ok {
				races++
				if row.DNF {
					dnfs++
				} else {
					rPos = append(rPos, float64(row.Pos))
					rAge = append(rAge, age)
				}
				if row.FastestLap {
					fls++
				}
			}
		}
		dm.QualiMu, dm.QualiSD, _ = ewma(qPos, qAge, 2.5)
		dm.RaceMu, dm.RaceSD, _ = ewma(rPos, rAge, 3.5)
		if len(qPos) == 0 {
			dm.QualiMu = 11
		}
		if len(rPos) == 0 {
			dm.RaceMu = dm.QualiMu
		}
		dm.DNFProb = shrunkRate(dnfs, races, fieldDNF, 8)
		dm.FLProb = shrunkRate(fls, races, 1.0/22.0, 4)
		dm.Rounds = int(races)

		// Overtake points per weekend are exact: the official race and
		// sprint points minus what the rules derive from the
		// classifications alone. The feed's cumulative overtake stat lags
		// by a round, so the residual is the reliable source. The
		// driver-of-the-day award comes from the feed stat; its timing
		// does not matter for a season count.
		dotd := componentDiff(a.History, func(h dataset.AssetRound) float64 { return h.Stats.DOTDPts })
		var oVals []float64
		var oAges []int
		dotdCount := 0.0
		for _, r := range completed {
			if v, ok := dotd[r.Round]; ok && v > 0 {
				dotdCount++
			}
			official, ok := a.RoundHistory(r.Round)
			if !ok {
				continue
			}
			row, ok := r.Race[a.TLA]
			if !ok {
				continue
			}
			w := weekendFromResults(r, a.TLA, row)
			w.DOTD = dotd[r.Round] > 0
			base := cfg.DriverPoints(w) - cfg.DriverPoints(rules.DriverWeekend{QualiPos: w.QualiPos})
			resid := official.RacePts + official.SprintPts - float64(base)
			oVals = append(oVals, math.Max(resid, 0))
			oAges = append(oAges, latest-r.Round)
		}
		lambda, _, _ := ewma(oVals, oAges, 0)
		dm.OvertakeLambda = math.Max(lambda, 0)
		dm.DOTDProb = shrunkRate(dotdCount, races, 1.0/22.0, 4)

		m.Drivers = append(m.Drivers, dm)
		byTeam[a.TeamID] = append(byTeam[a.TeamID], a.TLA)
	}

	for _, a := range d.Constructors() {
		if !d.Selectable(a) {
			continue
		}
		cm := ConstructorModel{AssetID: a.ID, Name: a.Name, TeamID: a.TeamID, TLAs: byTeam[a.TeamID]}
		cm.PitResid = fitPitResid(d, cfg, a, completed, latest)
		m.Constructors = append(m.Constructors, cm)
	}

	return m
}

// weekendFromResults builds a rules.DriverWeekend from the classifications
// of one round, without overtakes or the driver-of-the-day award.
func weekendFromResults(r dataset.Round, tla string, row dataset.RaceRow) rules.DriverWeekend {
	w := rules.DriverWeekend{
		QualiPos:   r.Quali[tla],
		GridPos:    row.Grid,
		FinishPos:  row.Pos,
		DNF:        row.DNF,
		FastestLap: row.FastestLap,
	}
	if row.Grid == 0 {
		// A pit-lane start counts as a slot behind the last car.
		w.GridPos = len(r.Race) + 1
	}
	if r.HasSprint {
		if s, ok := r.Sprint[tla]; ok {
			w.HasSprint = true
			w.SprintGrid = s.Grid
			w.SprintPos = s.Pos
			w.SprintDNF = s.DNF
			if s.Grid == 0 {
				w.SprintGrid = len(r.Sprint) + 1
			}
		}
	}
	return w
}

// fieldDNFRate returns the mean retirement rate per car per race, shrunk
// toward a historic base rate of 0.10 so a short or clean sample keeps a
// realistic floor.
func fieldDNFRate(completed []dataset.Round) float64 {
	dnfs, cars := 0.0, 0.0
	for _, r := range completed {
		for _, row := range r.Race {
			cars++
			if row.DNF {
				dnfs++
			}
		}
	}
	const baseRate, baseStrength = 0.10, 40.0
	return (dnfs + baseRate*baseStrength) / (cars + baseStrength)
}

// fitPitResid computes the EWMA residual between a constructor's official
// gameday points and the points that the rules derive from its drivers'
// official points plus the qualifying bonus. The residual captures the
// pit-stop component and any systematic table difference.
func fitPitResid(d dataset.Data, cfg rules.Config, cons dataset.Asset, completed []dataset.Round, latest int) float64 {
	driverPts := map[string]map[int]float64{} // TLA -> round -> official driver points minus DOTD
	driverDOTD := map[string]map[int]float64{}
	var tlas []string
	for _, dr := range d.Drivers() {
		if dr.TeamID != cons.TeamID {
			continue
		}
		tlas = append(tlas, dr.TLA)
		pts := map[int]float64{}
		for _, h := range dr.History {
			pts[h.Gameday] = h.Points
		}
		driverPts[dr.TLA] = pts
		driverDOTD[dr.TLA] = componentDiff(dr.History, func(h dataset.AssetRound) float64 { return h.Stats.DOTDPts })
	}

	var vals []float64
	var ages []int
	for _, r := range completed {
		official, ok := cons.RoundHistory(r.Round)
		if !ok {
			continue
		}
		derived := 0.0
		inQ2, inQ3 := 0, 0
		haveAll := true
		for _, tla := range tlas {
			p, ok := driverPts[tla][r.Round]
			if !ok {
				haveAll = false
				break
			}
			derived += p - driverDOTD[tla][r.Round]
			if qp, ok := r.Quali[tla]; ok && qp >= 1 {
				if qp <= cfg.Q2Cutoff {
					inQ2++
				}
				if qp <= cfg.Q3Cutoff {
					inQ3++
				}
			}
		}
		if !haveAll || len(tlas) == 0 {
			continue
		}
		derived += float64(cfg.ConstructorQualiBonusFor(inQ2, inQ3))
		vals = append(vals, official.Points-derived)
		ages = append(ages, latest-r.Round)
	}
	resid, _, _ := ewma(vals, ages, 0)
	return resid
}

// DriverByTLA returns the fitted driver model for a TLA.
func (m Model) DriverByTLA(tla string) (DriverModel, bool) {
	for _, dm := range m.Drivers {
		if dm.TLA == tla {
			return dm, true
		}
	}
	return DriverModel{}, false
}

// SortedDrivers returns the drivers ordered by race pace, best first.
func (m Model) SortedDrivers() []DriverModel {
	out := append([]DriverModel(nil), m.Drivers...)
	sort.Slice(out, func(i, j int) bool { return out[i].RaceMu < out[j].RaceMu })
	return out
}
