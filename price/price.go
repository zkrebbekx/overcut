// Package price predicts the next price change per asset.
//
// The game moves a price on the performance of the last three grands prix,
// inside tier bounds. The predictor does not guess the game's exact
// formula. It fits a linear model on the season's real price movements —
// change versus the asset's recent-form z-score and ownership — separately
// for drivers and constructors, then clips the prediction to the observed
// tier bounds. The Backtest function reports the walk-forward accuracy on
// the same season, so the error is measured, not asserted.
package price

import (
	"math"
	"sort"

	"github.com/zkrebbekx/overcut/internal/dataset"
)

// FormWindow is the count of recent gamedays that drive a price change.
const FormWindow = 3

// TierSplit is the price above which the game uses the low-volatility
// tier.
const TierSplit = 18.5

// Tier bounds in millions per gameday.
const (
	TierAMax = 0.3
	TierBMax = 0.6
)

// Example is one observed price movement with its features.
type Example struct {
	AssetID string
	Kind    dataset.Kind
	Gameday int     // the gameday at which the change applied
	Change  float64 // price minus old price
	FormZ   float64 // recent-points z-score among assets of the same kind
	OwnZ    float64 // ownership z-score among assets of the same kind
	Price   float64 // price before the change
}

// Prediction is one asset's predicted next change.
type Prediction struct {
	AssetID string
	Name    string
	Kind    dataset.Kind
	Price   float64
	Change  float64
}

// Model is a fitted per-kind linear predictor.
type Model struct {
	coef map[dataset.Kind][3]float64 // intercept, form, ownership
}

// clip bounds a change to the tier limits for the given price.
func clip(change, price float64) float64 {
	limit := TierBMax
	if price > TierSplit {
		limit = TierAMax
	}
	return math.Max(-limit, math.Min(limit, change))
}

// recentForm returns the mean official points of the last FormWindow
// gamedays at or before gameday g.
func recentForm(a dataset.Asset, g int) (float64, bool) {
	var pts []float64
	for _, h := range a.History {
		if h.Gameday <= g {
			pts = append(pts, h.Points)
		}
	}
	if len(pts) == 0 {
		return 0, false
	}
	if len(pts) > FormWindow {
		pts = pts[len(pts)-FormWindow:]
	}
	sum := 0.0
	for _, p := range pts {
		sum += p
	}
	return sum / float64(len(pts)), true
}

// zscores converts values to z-scores. A zero-variance set maps to zeros.
func zscores(values []float64) []float64 {
	mean, sd := moments(values)
	out := make([]float64, len(values))
	if sd == 0 {
		return out
	}
	for i, v := range values {
		out[i] = (v - mean) / sd
	}
	return out
}

func moments(values []float64) (mean, sd float64) {
	if len(values) == 0 {
		return 0, 0
	}
	for _, v := range values {
		mean += v
	}
	mean /= float64(len(values))
	for _, v := range values {
		sd += (v - mean) * (v - mean)
	}
	return mean, math.Sqrt(sd / float64(len(values)))
}

// Examples extracts every observed price movement with features that were
// known before the movement. The movement at gameday g uses form through
// gameday g-1.
func Examples(d dataset.Data) []Example {
	var out []Example
	gamedays := map[int]bool{}
	for _, a := range d.Assets {
		for _, h := range a.History {
			gamedays[h.Gameday] = true
		}
	}
	var gs []int
	for g := range gamedays {
		gs = append(gs, g)
	}
	sort.Ints(gs)

	for _, g := range gs {
		if g == 1 {
			continue // no prior form
		}
		for _, kind := range []dataset.Kind{dataset.KindDriver, dataset.KindConstructor} {
			var rows []Example
			var forms, owns []float64
			for _, a := range d.Assets {
				if a.Kind != kind {
					continue
				}
				cur, ok := a.RoundHistory(g)
				if !ok {
					continue
				}
				prev, ok := a.RoundHistory(g - 1)
				if !ok {
					continue
				}
				form, ok := recentForm(a, g-1)
				if !ok {
					continue
				}
				rows = append(rows, Example{
					AssetID: a.ID,
					Kind:    kind,
					Gameday: g,
					Change:  cur.Price - cur.OldPrice,
					Price:   cur.OldPrice,
				})
				forms = append(forms, form)
				owns = append(owns, prev.Ownership)
			}
			fz := zscores(forms)
			oz := zscores(owns)
			for i := range rows {
				rows[i].FormZ = fz[i]
				rows[i].OwnZ = oz[i]
			}
			out = append(out, rows...)
		}
	}
	return out
}

// Fit computes per-kind least-squares coefficients on the examples.
func Fit(examples []Example) Model {
	m := Model{coef: map[dataset.Kind][3]float64{}}
	for _, kind := range []dataset.Kind{dataset.KindDriver, dataset.KindConstructor} {
		var xs [][3]float64
		var ys []float64
		for _, e := range examples {
			if e.Kind != kind {
				continue
			}
			xs = append(xs, [3]float64{1, e.FormZ, e.OwnZ})
			ys = append(ys, e.Change)
		}
		m.coef[kind] = ols(xs, ys)
	}
	return m
}

// ols solves a three-parameter least squares by normal equations, with a
// small ridge term on the feature coefficients. The ridge keeps the solve
// stable when form and ownership correlate strongly.
func ols(xs [][3]float64, ys []float64) [3]float64 {
	if len(xs) < 4 {
		return [3]float64{}
	}
	var a [3][4]float64
	for i, x := range xs {
		for r := 0; r < 3; r++ {
			for c := 0; c < 3; c++ {
				a[r][c] += x[r] * x[c]
			}
			a[r][3] += x[r] * ys[i]
		}
	}
	const ridge = 1e-3
	a[1][1] += ridge
	a[2][2] += ridge
	// Gaussian elimination with partial pivoting.
	for col := 0; col < 3; col++ {
		piv := col
		for r := col + 1; r < 3; r++ {
			if math.Abs(a[r][col]) > math.Abs(a[piv][col]) {
				piv = r
			}
		}
		a[col], a[piv] = a[piv], a[col]
		if math.Abs(a[col][col]) < 1e-12 {
			return [3]float64{}
		}
		for r := 0; r < 3; r++ {
			if r == col {
				continue
			}
			f := a[r][col] / a[col][col]
			for c := col; c < 4; c++ {
				a[r][c] -= f * a[col][c]
			}
		}
	}
	var out [3]float64
	for r := 0; r < 3; r++ {
		out[r] = a[r][3] / a[r][r]
	}
	return out
}

// predict applies the model to one feature row and clips to the tier.
func (m Model) predict(kind dataset.Kind, formZ, ownZ, price float64) float64 {
	c := m.coef[kind]
	return clip(c[0]+c[1]*formZ+c[2]*ownZ, price)
}

// Predict returns the predicted next price change for every asset, from
// the latest gameday snapshot.
func Predict(d dataset.Data, m Model) []Prediction {
	var out []Prediction
	for _, kind := range []dataset.Kind{dataset.KindDriver, dataset.KindConstructor} {
		var assets []dataset.Asset
		var forms, owns []float64
		for _, a := range d.Assets {
			if a.Kind != kind || !d.Selectable(a) {
				continue
			}
			latest, ok := a.Latest()
			if !ok {
				continue
			}
			form, ok := recentForm(a, latest.Gameday)
			if !ok {
				continue
			}
			assets = append(assets, a)
			forms = append(forms, form)
			owns = append(owns, latest.Ownership)
		}
		fz := zscores(forms)
		oz := zscores(owns)
		for i, a := range assets {
			latest, _ := a.Latest()
			out = append(out, Prediction{
				AssetID: a.ID,
				Name:    a.Name,
				Kind:    kind,
				Price:   latest.Price,
				Change:  m.predict(kind, fz[i], oz[i], latest.Price),
			})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Change > out[j].Change })
	return out
}

// BacktestReport is the walk-forward accuracy of the predictor.
type BacktestReport struct {
	Examples  int
	MAE       float64 // mean absolute error in millions
	NaiveMAE  float64 // error of the always-zero prediction
	Direction float64 // sign hit rate on the moves that happened
	Moves     int     // count of nonzero actual moves
}

// Backtest fits on gamedays before g and predicts g, for every g with
// enough history, and reports the pooled accuracy.
func Backtest(d dataset.Data) BacktestReport {
	all := Examples(d)
	var rep BacktestReport
	for _, e := range all {
		if e.Gameday < 4 {
			continue // too little training history
		}
		var train []Example
		for _, t := range all {
			if t.Gameday < e.Gameday {
				train = append(train, t)
			}
		}
		m := Fit(train)
		pred := m.predict(e.Kind, e.FormZ, e.OwnZ, e.Price)
		rep.Examples++
		rep.MAE += math.Abs(pred - e.Change)
		rep.NaiveMAE += math.Abs(e.Change)
		if e.Change != 0 {
			rep.Moves++
			if (pred > 0) == (e.Change > 0) && pred != 0 {
				rep.Direction++
			}
		}
	}
	if rep.Examples > 0 {
		rep.MAE /= float64(rep.Examples)
		rep.NaiveMAE /= float64(rep.Examples)
	}
	if rep.Moves > 0 {
		rep.Direction /= float64(rep.Moves)
	}
	return rep
}
