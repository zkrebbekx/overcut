// Package backtest measures the projection model on past rounds.
//
// For every finished round, the backtest fits the model only on the rounds
// before it, projects the round, and scores the projection against the
// official fantasy points. The report also scores two naive baselines, so
// the model's edge is visible and honest.
package backtest

import (
	"math"
	"sort"

	"github.com/zkrebbekx/overcut/internal/dataset"
	"github.com/zkrebbekx/overcut/model"
	"github.com/zkrebbekx/overcut/optimize"
	"github.com/zkrebbekx/overcut/rules"
)

// StartRound is the first projected round. Earlier rounds have too little
// training history.
const StartRound = 4

// RoundResult is the accuracy of one projected round.
type RoundResult struct {
	Round int
	Name  string

	DriverMAE   float64
	ConsMAE     float64
	SpearmanRho float64 // rank correlation over drivers

	// Team points: what the projected optimal team really scored versus
	// the hindsight optimum and versus a naive team.
	ModelTeamPts     float64
	NaiveTeamPts     float64
	HindsightTeamPts float64
}

// Report is the pooled backtest result.
type Report struct {
	Rounds []RoundResult

	DriverMAE      float64
	ConsMAE        float64
	MeanSpearman   float64
	BaselinePrev   float64 // driver MAE of the previous-round baseline
	BaselineSeason float64 // driver MAE of the season-mean baseline

	ModelTeamPts     float64
	NaiveTeamPts     float64
	HindsightTeamPts float64
}

// Run executes the walk-forward backtest with the given simulation count
// per round.
func Run(d dataset.Data, cfg rules.Config, sims int, seed uint64) Report {
	var rep Report
	completed := d.CompletedRounds()

	var pooledPrev, pooledSeason []float64

	for _, round := range completed {
		if round.Round < StartRound {
			continue
		}
		m := model.Fit(d, cfg, round.Round-1)
		sim := m.Simulate(round.Round, round.HasSprint, sims, seed)

		rr := RoundResult{Round: round.Round, Name: round.Name}

		var projD, actD []float64
		nD, nC := 0.0, 0.0
		for _, a := range d.Assets {
			actual, ok := a.RoundHistory(round.Round)
			if !ok {
				continue
			}
			proj, ok := sim.ByID(a.ID)
			if !ok {
				continue
			}
			err := math.Abs(proj.Mean - actual.Points)
			if a.Kind == dataset.KindDriver {
				rr.DriverMAE += err
				nD++
				projD = append(projD, proj.Mean)
				actD = append(actD, actual.Points)

				pooledPrev = append(pooledPrev, math.Abs(prevPoints(a, round.Round)-actual.Points))
				pooledSeason = append(pooledSeason, math.Abs(seasonMean(a, round.Round)-actual.Points))
			} else {
				rr.ConsMAE += err
				nC++
			}
		}
		if nD > 0 {
			rr.DriverMAE /= nD
		}
		if nC > 0 {
			rr.ConsMAE /= nC
		}
		rr.SpearmanRho = spearman(projD, actD)

		rr.ModelTeamPts = teamActualPoints(d, cfg, round.Round, projections(d, sim, round.Round))
		rr.NaiveTeamPts = teamActualPoints(d, cfg, round.Round, naiveProjections(d, round.Round))
		rr.HindsightTeamPts = teamActualPoints(d, cfg, round.Round, actualAsProjection(d, round.Round))

		rep.Rounds = append(rep.Rounds, rr)
	}

	n := float64(len(rep.Rounds))
	if n == 0 {
		return rep
	}
	for _, rr := range rep.Rounds {
		rep.DriverMAE += rr.DriverMAE / n
		rep.ConsMAE += rr.ConsMAE / n
		rep.MeanSpearman += rr.SpearmanRho / n
		rep.ModelTeamPts += rr.ModelTeamPts / n
		rep.NaiveTeamPts += rr.NaiveTeamPts / n
		rep.HindsightTeamPts += rr.HindsightTeamPts / n
	}
	rep.BaselinePrev = mean(pooledPrev)
	rep.BaselineSeason = mean(pooledSeason)
	return rep
}

// projections converts a simulation into optimizer assets priced at the
// round's real prices.
func projections(d dataset.Data, sim model.SimResult, round int) []optimize.Asset {
	var out []optimize.Asset
	for _, a := range d.Assets {
		h, ok := a.RoundHistory(round)
		if !ok {
			continue
		}
		proj, ok := sim.ByID(a.ID)
		if !ok {
			continue
		}
		out = append(out, optimize.Asset{
			ID: a.ID, Name: a.Name, Kind: string(a.Kind),
			Price: h.Price, Points: proj.Mean,
		})
	}
	return out
}

// naiveProjections prices every asset at the round's real price and
// projects the previous round's points — the "pick last week's scorers"
// strategy.
func naiveProjections(d dataset.Data, round int) []optimize.Asset {
	var out []optimize.Asset
	for _, a := range d.Assets {
		h, ok := a.RoundHistory(round)
		if !ok {
			continue
		}
		out = append(out, optimize.Asset{
			ID: a.ID, Name: a.Name, Kind: string(a.Kind),
			Price: h.Price, Points: prevPoints(a, round),
		})
	}
	return out
}

// actualAsProjection uses the round's real points — the hindsight optimum.
func actualAsProjection(d dataset.Data, round int) []optimize.Asset {
	var out []optimize.Asset
	for _, a := range d.Assets {
		h, ok := a.RoundHistory(round)
		if !ok {
			continue
		}
		out = append(out, optimize.Asset{
			ID: a.ID, Name: a.Name, Kind: string(a.Kind),
			Price: h.Price, Points: h.Points,
		})
	}
	return out
}

// teamActualPoints optimizes a team on the given projections, then scores
// that team with the round's official points, captain included.
func teamActualPoints(d dataset.Data, cfg rules.Config, round int, assets []optimize.Asset) float64 {
	teams := optimize.Best(assets, optimize.Options{
		Budget:           cfg.Budget,
		DriverSlots:      cfg.TeamDrivers,
		ConstructorSlots: cfg.TeamConstructors,
		TopN:             1,
	})
	if len(teams) == 0 {
		return 0
	}
	t := teams[0]

	actual := map[string]float64{}
	for _, a := range d.Assets {
		if h, ok := a.RoundHistory(round); ok {
			actual[a.ID] = h.Points
		}
	}
	total := 0.0
	bestDriver := math.Inf(-1)
	for _, dr := range t.Drivers {
		total += actual[dr.ID]
		if actual[dr.ID] > bestDriver {
			bestDriver = actual[dr.ID]
		}
	}
	for _, c := range t.Constructors {
		total += actual[c.ID]
	}
	// The captain is chosen before the race on projected points; score the
	// double on the projected captain's actual points.
	total += actual[t.CaptainID]
	return total
}

func prevPoints(a dataset.Asset, round int) float64 {
	if h, ok := a.RoundHistory(round - 1); ok {
		return h.Points
	}
	return 0
}

func seasonMean(a dataset.Asset, round int) float64 {
	var sum, n float64
	for _, h := range a.History {
		if h.Gameday < round {
			sum += h.Points
			n++
		}
	}
	if n == 0 {
		return 0
	}
	return sum / n
}

func mean(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	s := 0.0
	for _, x := range v {
		s += x
	}
	return s / float64(len(v))
}

// spearman computes the Spearman rank correlation of two paired samples.
func spearman(a, b []float64) float64 {
	if len(a) != len(b) || len(a) < 3 {
		return 0
	}
	ra, rb := ranks(a), ranks(b)
	var ma, mb float64
	for i := range ra {
		ma += ra[i]
		mb += rb[i]
	}
	ma /= float64(len(ra))
	mb /= float64(len(rb))
	var cov, va, vb float64
	for i := range ra {
		cov += (ra[i] - ma) * (rb[i] - mb)
		va += (ra[i] - ma) * (ra[i] - ma)
		vb += (rb[i] - mb) * (rb[i] - mb)
	}
	if va == 0 || vb == 0 {
		return 0
	}
	return cov / math.Sqrt(va*vb)
}

// ranks returns average ranks, with ties sharing the mean rank.
func ranks(v []float64) []float64 {
	idx := make([]int, len(v))
	for i := range idx {
		idx[i] = i
	}
	sort.Slice(idx, func(i, j int) bool { return v[idx[i]] < v[idx[j]] })
	out := make([]float64, len(v))
	for i := 0; i < len(idx); {
		j := i
		for j < len(idx) && v[idx[j]] == v[idx[i]] {
			j++
		}
		avg := float64(i+j-1)/2 + 1
		for k := i; k < j; k++ {
			out[idx[k]] = avg
		}
		i = j
	}
	return out
}
