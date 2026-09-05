package model

import (
	"math"
	"math/rand/v2"
	"sort"

	"github.com/zkrebbekx/overcut/rules"
)

// Projection is the simulated fantasy-point distribution of one asset.
type Projection struct {
	AssetID string
	Name    string
	Kind    string // "driver" or "constructor"

	Mean float64
	SD   float64
	P10  float64
	P50  float64
	P90  float64
}

// SimResult holds the projections of one simulated round.
type SimResult struct {
	Round     int
	HasSprint bool
	Sims      int
	Assets    []Projection
	projByID  map[string]*Projection
}

// ByID returns the projection for one asset ID.
func (s SimResult) ByID(id string) (Projection, bool) {
	p, ok := s.projByID[id]
	if !ok {
		return Projection{}, false
	}
	return *p, true
}

// Conditions holds what is already known about the weekend at simulation
// time. Every field is optional. Keys are driver TLAs.
type Conditions struct {
	// Quali is the actual qualifying classification (1-based). When set,
	// the model does not sample qualifying.
	Quali map[string]int
	// Grid is the actual starting grid after penalties (1-based). When
	// unset, the grid follows the qualifying order with BackOfGrid
	// drivers moved to the back.
	Grid map[string]int
	// BackOfGrid lists drivers that start from the back of the grid.
	BackOfGrid []string
	// Practice is a practice classification (1-based), used as a pace
	// prior when qualifying is unknown. Half the weight goes to practice
	// and half to season form.
	Practice map[string]int
}

// GridInfluence is the weight of the grid slot in the race finish score.
// The remainder of the weight goes to season race pace.
const GridInfluence = 0.35

// Simulate runs a Monte Carlo simulation of one round and returns the
// fantasy-point distribution per asset. The same seed gives the same
// result.
func (m Model) Simulate(round int, hasSprint bool, sims int, seed uint64) SimResult {
	return m.SimulateWith(round, hasSprint, sims, seed, Conditions{})
}

// SimulateWith runs Simulate with known weekend conditions.
func (m Model) SimulateWith(round int, hasSprint bool, sims int, seed uint64, cond Conditions) SimResult {
	rng := rand.New(rand.NewPCG(seed, uint64(round)))

	n := len(m.Drivers)

	// Apply the practice prior to the pace estimates.
	drivers := append([]DriverModel(nil), m.Drivers...)
	if len(cond.Practice) > 0 {
		for i := range drivers {
			if p, ok := cond.Practice[drivers[i].TLA]; ok && p > 0 {
				drivers[i].QualiMu = 0.5*drivers[i].QualiMu + 0.5*float64(p)
				drivers[i].RaceMu = 0.5*drivers[i].RaceMu + 0.5*float64(p)
			}
		}
	}
	m.Drivers = drivers

	back := map[string]bool{}
	for _, tla := range cond.BackOfGrid {
		back[tla] = true
	}

	// fixedGrid resolves the starting grid from the known conditions
	// given a qualifying order, or returns nil when nothing is known.
	fixedGrid := func(qualiPos []int) []int {
		if len(cond.Grid) == 0 && len(back) == 0 {
			return nil
		}
		grid := make([]int, n)
		if len(cond.Grid) > 0 {
			for i, dm := range m.Drivers {
				if g, ok := cond.Grid[dm.TLA]; ok && g > 0 {
					grid[i] = g
				} else {
					grid[i] = qualiPos[i]
				}
			}
			return grid
		}
		// Move the penalized drivers behind the rest, in qualifying order.
		type slot struct{ idx, key int }
		slots := make([]slot, n)
		for i, dm := range m.Drivers {
			key := qualiPos[i]
			if back[dm.TLA] {
				key += 100
			}
			slots[i] = slot{i, key}
		}
		sort.Slice(slots, func(a, b int) bool { return slots[a].key < slots[b].key })
		for p, s := range slots {
			grid[s.idx] = p + 1
		}
		return grid
	}
	samples := map[string][]float64{}
	for _, dm := range m.Drivers {
		samples[dm.AssetID] = make([]float64, 0, sims)
	}
	for _, cm := range m.Constructors {
		samples[cm.AssetID] = make([]float64, 0, sims)
	}

	type slot struct {
		idx   int
		score float64
	}
	rank := func(scores []float64) []int {
		slots := make([]slot, n)
		for i, s := range scores {
			slots[i] = slot{i, s}
		}
		sort.Slice(slots, func(a, b int) bool { return slots[a].score < slots[b].score })
		pos := make([]int, n)
		for p, s := range slots {
			pos[s.idx] = p + 1
		}
		return pos
	}

	scores := make([]float64, n)
	weekends := make([]rules.DriverWeekend, n)

	for s := 0; s < sims; s++ {
		// Qualifying: use the known classification, or sample a pace score
		// per driver and rank.
		var qualiPos []int
		if len(cond.Quali) > 0 {
			qualiPos = make([]int, n)
			for i, dm := range m.Drivers {
				qualiPos[i] = cond.Quali[dm.TLA]
			}
		} else {
			for i, dm := range m.Drivers {
				scores[i] = dm.QualiMu + rng.NormFloat64()*dm.QualiSD
			}
			qualiPos = rank(scores)
		}
		gridPos := fixedGrid(qualiPos)
		if gridPos == nil {
			gridPos = qualiPos
		}

		// Race: sample retirements, then rank the classified cars by a
		// race-pace score that blends season pace with the grid slot. A
		// retired car takes no classified position.
		dnf := make([]bool, n)
		for i, dm := range m.Drivers {
			dnf[i] = rng.Float64() < dm.DNFProb
			pace := (1-GridInfluence)*dm.RaceMu + GridInfluence*float64(gridPos[i])
			scores[i] = pace + rng.NormFloat64()*dm.RaceSD
			if dnf[i] {
				scores[i] += 1000 // rank retired cars last
			}
		}
		racePos := rank(scores)

		// Fastest lap and driver of the day: one winner each, sampled in
		// proportion to the fitted probability. A retired car cannot take
		// the fastest lap in the model.
		flWinner := pickWeighted(rng, m.Drivers, dnf, func(dm DriverModel) float64 { return dm.FLProb })
		dotdWinner := pickWeighted(rng, m.Drivers, nil, func(dm DriverModel) float64 { return dm.DOTDProb })

		var sqPos, sprintPos []int
		var sprintDNF []bool
		if hasSprint {
			for i, dm := range m.Drivers {
				scores[i] = dm.QualiMu + rng.NormFloat64()*dm.QualiSD
			}
			sqPos = rank(scores)
			sprintDNF = make([]bool, n)
			for i, dm := range m.Drivers {
				// A sprint is about a third of a race distance; scale the
				// retirement risk down accordingly.
				sprintDNF[i] = rng.Float64() < dm.DNFProb/3
				scores[i] = dm.RaceMu + rng.NormFloat64()*dm.RaceSD
				if sprintDNF[i] {
					scores[i] += 1000
				}
			}
			sprintPos = rank(scores)
		}

		for i, dm := range m.Drivers {
			w := rules.DriverWeekend{
				QualiPos:  qualiPos[i],
				GridPos:   gridPos[i],
				FinishPos: racePos[i],
				DNF:       dnf[i],
				// Overtake points come from the fitted per-weekend rate.
				// The rules award one point per overtake in the race and
				// in the sprint alike, so the model books the whole
				// weekend rate in the race leg.
				Overtakes:  poisson(rng, dm.OvertakeLambda),
				FastestLap: i == flWinner,
				DOTD:       i == dotdWinner,
			}
			if hasSprint {
				w.HasSprint = true
				w.SprintGrid = sqPos[i]
				w.SprintPos = sprintPos[i]
				w.SprintDNF = sprintDNF[i]
			}
			weekends[i] = w
			samples[dm.AssetID] = append(samples[dm.AssetID], float64(m.Rules.DriverPoints(w)))
		}

		for _, cm := range m.Constructors {
			var ws []rules.DriverWeekend
			for i, dm := range m.Drivers {
				if dm.TeamID == cm.TeamID {
					ws = append(ws, weekends[i])
				}
			}
			if len(ws) != 2 {
				continue
			}
			pts := float64(m.Rules.ConstructorPoints(ws[0], ws[1], 0)) + cm.PitResid
			samples[cm.AssetID] = append(samples[cm.AssetID], pts)
		}
	}

	res := SimResult{Round: round, HasSprint: hasSprint, Sims: sims, projByID: map[string]*Projection{}}
	add := func(id, name, kind string) {
		sample := samples[id]
		if len(sample) == 0 {
			return
		}
		p := summarize(sample)
		p.AssetID, p.Name, p.Kind = id, name, kind
		res.Assets = append(res.Assets, p)
		res.projByID[id] = &res.Assets[len(res.Assets)-1]
	}
	for _, dm := range m.Drivers {
		add(dm.AssetID, dm.Name, "driver")
	}
	for _, cm := range m.Constructors {
		add(cm.AssetID, cm.Name, "constructor")
	}
	// Rebuild the index: append can move the slice.
	res.projByID = map[string]*Projection{}
	for i := range res.Assets {
		res.projByID[res.Assets[i].AssetID] = &res.Assets[i]
	}
	return res
}

// summarize computes the moments and percentiles of a sample.
func summarize(sample []float64) Projection {
	s := append([]float64(nil), sample...)
	sort.Float64s(s)
	var sum float64
	for _, v := range s {
		sum += v
	}
	mean := sum / float64(len(s))
	var sv float64
	for _, v := range s {
		sv += (v - mean) * (v - mean)
	}
	pct := func(p float64) float64 {
		idx := int(p * float64(len(s)-1))
		return s[idx]
	}
	return Projection{
		Mean: mean,
		SD:   math.Sqrt(sv / float64(len(s))),
		P10:  pct(0.10),
		P50:  pct(0.50),
		P90:  pct(0.90),
	}
}

// pickWeighted samples one driver index in proportion to a weight, and
// skips excluded drivers. It returns -1 when every weight is zero.
func pickWeighted(rng *rand.Rand, drivers []DriverModel, excluded []bool, w func(DriverModel) float64) int {
	total := 0.0
	for i, dm := range drivers {
		if excluded != nil && excluded[i] {
			continue
		}
		total += w(dm)
	}
	if total <= 0 {
		return -1
	}
	r := rng.Float64() * total
	for i, dm := range drivers {
		if excluded != nil && excluded[i] {
			continue
		}
		r -= w(dm)
		if r <= 0 {
			return i
		}
	}
	return -1
}

// poisson samples a Poisson count with mean lambda by inversion.
func poisson(rng *rand.Rand, lambda float64) int {
	if lambda <= 0 {
		return 0
	}
	l := math.Exp(-lambda)
	k := 0
	p := 1.0
	for {
		p *= rng.Float64()
		if p <= l {
			return k
		}
		k++
		if k > 50 {
			return k
		}
	}
}
