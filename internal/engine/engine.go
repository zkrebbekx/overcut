// Package engine exposes the toolkit as transport-agnostic operations with
// JSON-shaped inputs and outputs. The HTTP server and the WebAssembly
// build both call it.
package engine

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/zkrebbekx/overcut/backtest"
	"github.com/zkrebbekx/overcut/internal/dataset"
	"github.com/zkrebbekx/overcut/model"
	"github.com/zkrebbekx/overcut/optimize"
	"github.com/zkrebbekx/overcut/price"
	"github.com/zkrebbekx/overcut/rules"
)

// Engine holds the dataset and the rules, and caches expensive results.
type Engine struct {
	mu   sync.RWMutex
	data dataset.Data
	cfg  rules.Config

	cacheMu       sync.Mutex
	simCache      map[string]model.SimResult
	backtestCache map[int]backtest.Report
}

// New returns an Engine over the dataset.
func New(data dataset.Data, cfg rules.Config) *Engine {
	e := &Engine{cfg: cfg}
	e.SetData(data)
	return e
}

// SetData replaces the dataset and clears every cache.
func (e *Engine) SetData(data dataset.Data) {
	e.mu.Lock()
	e.data = data
	e.mu.Unlock()
	e.cacheMu.Lock()
	e.simCache = map[string]model.SimResult{}
	e.backtestCache = map[int]backtest.Report{}
	e.cacheMu.Unlock()
}

// Data returns the current dataset.
func (e *Engine) Data() dataset.Data {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.data
}

// Rules returns the scoring configuration.
func (e *Engine) Rules() rules.Config {
	return e.cfg
}

// --- season ---------------------------------------------------------------

// SeasonView is the dataset shaped for the UI.
type SeasonView struct {
	Season        int         `json:"season"`
	SyncedAt      time.Time   `json:"synced_at"`
	LatestGameday int         `json:"latest_gameday"`
	NextRound     int         `json:"next_round"`
	Rounds        []RoundView `json:"rounds"`
	Assets        []AssetView `json:"assets"`
}

// RoundView is one calendar entry.
type RoundView struct {
	Round      int    `json:"round"`
	Name       string `json:"name"`
	CircuitID  string `json:"circuit_id"`
	Date       string `json:"date"`
	HasSprint  bool   `json:"has_sprint"`
	HasResults bool   `json:"has_results"`
}

// AssetView is one asset with its history.
type AssetView struct {
	ID         string        `json:"id"`
	Kind       string        `json:"kind"`
	Name       string        `json:"name"`
	TLA        string        `json:"tla,omitempty"`
	TeamID     string        `json:"team_id"`
	TeamName   string        `json:"team_name"`
	Price      float64       `json:"price"`
	OldPrice   float64       `json:"old_price"`
	Ownership  float64       `json:"ownership"`
	Total      float64       `json:"total_points"`
	Selectable bool          `json:"selectable"`
	History    []HistoryView `json:"history"`
}

// HistoryView is one gameday of an asset.
type HistoryView struct {
	Gameday   int     `json:"gameday"`
	Price     float64 `json:"price"`
	Points    float64 `json:"points"`
	QualiPts  float64 `json:"quali_pts"`
	SprintPts float64 `json:"sprint_pts"`
	RacePts   float64 `json:"race_pts"`
	Ownership float64 `json:"ownership"`
}

// teamName returns the asset's team, falling back to the asset's own name
// for a constructor recorded without one.
func teamName(a dataset.Asset) string {
	if a.TeamName != "" {
		return a.TeamName
	}
	return a.Name
}

// Season returns the dataset shaped for the UI.
func (e *Engine) Season() SeasonView {
	e.mu.RLock()
	defer e.mu.RUnlock()
	d := e.data
	v := SeasonView{Season: d.Season, SyncedAt: d.SyncedAt, LatestGameday: d.LatestGameday()}
	if r, ok := d.NextRound(); ok {
		v.NextRound = r.Round
	}
	for _, r := range d.Rounds {
		v.Rounds = append(v.Rounds, RoundView{
			Round: r.Round, Name: r.Name, CircuitID: r.CircuitID, Date: r.Date,
			HasSprint: r.HasSprint, HasResults: r.HasResults,
		})
	}
	for _, a := range d.Assets {
		latest, ok := a.Latest()
		if !ok {
			continue
		}
		av := AssetView{
			ID: a.ID, Kind: string(a.Kind), Name: a.Name, TLA: a.TLA,
			TeamID: a.TeamID, TeamName: teamName(a),
			Price: latest.Price, OldPrice: latest.OldPrice, Ownership: latest.Ownership,
			Selectable: d.Selectable(a),
		}
		for _, h := range a.History {
			av.Total += h.Points
			av.History = append(av.History, HistoryView{
				Gameday: h.Gameday, Price: h.Price, Points: h.Points,
				QualiPts: h.QualiPts, SprintPts: h.SprintPts, RacePts: h.RacePts,
				Ownership: h.Ownership,
			})
		}
		v.Assets = append(v.Assets, av)
	}
	return v
}

// --- projection -------------------------------------------------------------

// Conditions is the known-weekend state. Each order is a list of driver
// codes from P1.
type Conditions struct {
	Quali []string `json:"quali,omitempty"`
	Grid  []string `json:"grid,omitempty"`
	Back  []string `json:"back,omitempty"`
	FP3   []string `json:"fp3,omitempty"`
}

func (c Conditions) toModel() model.Conditions {
	toPos := func(order []string) map[string]int {
		if len(order) == 0 {
			return nil
		}
		out := map[string]int{}
		for i, tla := range order {
			out[strings.ToUpper(strings.TrimSpace(tla))] = i + 1
		}
		return out
	}
	var back []string
	for _, tla := range c.Back {
		back = append(back, strings.ToUpper(strings.TrimSpace(tla)))
	}
	return model.Conditions{
		Quali: toPos(c.Quali), Grid: toPos(c.Grid), BackOfGrid: back, Practice: toPos(c.FP3),
	}
}

func (c Conditions) key() string {
	b, _ := json.Marshal(c)
	return string(b)
}

// ProjectInput is the projection request. A zero Round means the next
// round; zero Sims and Seed take the defaults.
type ProjectInput struct {
	Round      int        `json:"round"`
	Sims       int        `json:"sims"`
	Seed       uint64     `json:"seed"`
	Conditions Conditions `json:"conditions"`
}

// ProjectionView is one round's projection.
type ProjectionView struct {
	Round      int               `json:"round"`
	Name       string            `json:"name"`
	HasSprint  bool              `json:"has_sprint"`
	Sims       int               `json:"sims"`
	Conditions Conditions        `json:"conditions"`
	Assets     []AssetProjection `json:"assets"`
}

// AssetProjection is one asset's projected distribution plus market data.
type AssetProjection struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Kind      string  `json:"kind"`
	TLA       string  `json:"tla,omitempty"`
	TeamID    string  `json:"team_id"`
	TeamName  string  `json:"team_name"`
	Price     float64 `json:"price"`
	Ownership float64 `json:"ownership"`
	Mean      float64 `json:"mean"`
	SD        float64 `json:"sd"`
	P10       float64 `json:"p10"`
	P50       float64 `json:"p50"`
	P90       float64 `json:"p90"`
	LastPts   float64 `json:"last_points"`
	AvgPts    float64 `json:"avg_points"`
}

func (e *Engine) resolveRound(round int) (dataset.Round, error) {
	if round == 0 {
		next, ok := e.data.NextRound()
		if !ok {
			return dataset.Round{}, fmt.Errorf("the season is complete; pass a round")
		}
		return next, nil
	}
	for _, r := range e.data.Rounds {
		if r.Round == round {
			return r, nil
		}
	}
	return dataset.Round{}, fmt.Errorf("round %d is not on the calendar", round)
}

// simulate returns a cached or fresh simulation. The caller holds the read
// lock.
func (e *Engine) simulate(target dataset.Round, sims int, seed uint64, cond Conditions) model.SimResult {
	if sims <= 0 {
		sims = 20000
	}
	if seed == 0 {
		seed = 1
	}
	key := fmt.Sprintf("%d|%d|%d|%s", target.Round, sims, seed, cond.key())
	e.cacheMu.Lock()
	res, ok := e.simCache[key]
	e.cacheMu.Unlock()
	if ok {
		return res
	}
	m := model.Fit(e.data, e.cfg, target.Round-1)
	res = m.SimulateWith(target.Round, target.HasSprint, sims, seed, cond.toModel())
	e.cacheMu.Lock()
	e.simCache[key] = res
	e.cacheMu.Unlock()
	return res
}

func (e *Engine) projectionView(target dataset.Round, sim model.SimResult, cond Conditions) ProjectionView {
	v := ProjectionView{Round: target.Round, Name: target.Name, HasSprint: target.HasSprint, Sims: sim.Sims, Conditions: cond}
	for _, a := range e.data.Assets {
		p, ok := sim.ByID(a.ID)
		if !ok {
			continue
		}
		latest, _ := a.Latest()
		var sum, n float64
		for _, h := range a.History {
			if h.Gameday < target.Round {
				sum += h.Points
				n++
			}
		}
		ap := AssetProjection{
			ID: a.ID, Name: a.Name, Kind: string(a.Kind), TLA: a.TLA, TeamID: a.TeamID, TeamName: teamName(a),
			Price: latest.Price, Ownership: latest.Ownership,
			Mean: p.Mean, SD: p.SD, P10: p.P10, P50: p.P50, P90: p.P90,
		}
		if h, ok := a.RoundHistory(target.Round - 1); ok {
			ap.LastPts = h.Points
		}
		if n > 0 {
			ap.AvgPts = sum / n
		}
		v.Assets = append(v.Assets, ap)
	}
	sort.Slice(v.Assets, func(i, j int) bool { return v.Assets[i].Mean > v.Assets[j].Mean })
	return v
}

// Project simulates one round.
func (e *Engine) Project(in ProjectInput) (ProjectionView, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	target, err := e.resolveRound(in.Round)
	if err != nil {
		return ProjectionView{}, err
	}
	sim := e.simulate(target, in.Sims, in.Seed, in.Conditions)
	return e.projectionView(target, sim, in.Conditions), nil
}

// --- optimize ---------------------------------------------------------------

// OptimizeInput is the optimizer request.
type OptimizeInput struct {
	Round         int        `json:"round"`
	Sims          int        `json:"sims"`
	Seed          uint64     `json:"seed"`
	Team          []string   `json:"team"` // asset IDs
	FreeTransfers int        `json:"free_transfers"`
	Budget        float64    `json:"budget"`
	Chip          string     `json:"chip"` // "", wildcard, limitless, 3x
	Risk          string     `json:"risk"` // mean, p10, p90
	Top           int        `json:"top"`
	Conditions    Conditions `json:"conditions"`
}

// TeamView is one optimized team.
type TeamView struct {
	Drivers      []TeamAsset `json:"drivers"`
	Constructors []TeamAsset `json:"constructors"`
	CaptainID    string      `json:"captain_id"`
	BoostID      string      `json:"boost_id,omitempty"`
	Cost         float64     `json:"cost"`
	RawPoints    float64     `json:"raw_points"`
	Captain      float64     `json:"captain_points"`
	Transfers    int         `json:"transfers"`
	Penalty      float64     `json:"penalty"`
	Score        float64     `json:"score"`
	In           []string    `json:"in"`  // asset IDs bought
	Out          []string    `json:"out"` // asset IDs sold
}

// TeamAsset is one asset inside a team.
type TeamAsset struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Kind   string  `json:"kind"`
	Price  float64 `json:"price"`
	Points float64 `json:"points"`
}

// OptimizeView is the optimizer response.
type OptimizeView struct {
	Round        int            `json:"round"`
	Name         string         `json:"name"`
	Risk         string         `json:"risk"`
	Chip         string         `json:"chip"`
	Budget       float64        `json:"budget"`
	Teams        []TeamView     `json:"teams"`
	CurrentScore float64        `json:"current_score"` // projected score of the team as-is
	Projection   ProjectionView `json:"projection"`
}

func teamView(t optimize.Team, current map[string]bool) TeamView {
	// In and Out start as empty slices so JSON carries [] rather than null.
	tv := TeamView{
		CaptainID: t.CaptainID, BoostID: t.BoostID, Cost: t.Cost, RawPoints: t.RawPoints,
		Captain: t.Captain, Transfers: t.Transfers, Penalty: t.Penalty, Score: t.Score,
		In: []string{}, Out: []string{},
	}
	inTeam := map[string]bool{}
	for _, d := range t.Drivers {
		tv.Drivers = append(tv.Drivers, TeamAsset{ID: d.ID, Name: d.Name, Kind: d.Kind, Price: d.Price, Points: d.Points})
		inTeam[d.ID] = true
	}
	for _, c := range t.Constructors {
		tv.Constructors = append(tv.Constructors, TeamAsset{ID: c.ID, Name: c.Name, Kind: c.Kind, Price: c.Price, Points: c.Points})
		inTeam[c.ID] = true
	}
	for id := range inTeam {
		if !current[id] {
			tv.In = append(tv.In, id)
		}
	}
	for id := range current {
		if !inTeam[id] {
			tv.Out = append(tv.Out, id)
		}
	}
	sort.Strings(tv.In)
	sort.Strings(tv.Out)
	return tv
}

// Optimize finds the best teams for the round.
func (e *Engine) Optimize(in OptimizeInput) (OptimizeView, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	target, err := e.resolveRound(in.Round)
	if err != nil {
		return OptimizeView{}, err
	}
	if in.Top == 0 {
		in.Top = 5
	}
	if in.Risk == "" {
		in.Risk = "mean"
	}
	sim := e.simulate(target, in.Sims, in.Seed, in.Conditions)
	pick := func(p model.Projection) float64 {
		switch in.Risk {
		case "p10":
			return p.P10
		case "p90":
			return p.P90
		default:
			return p.Mean
		}
	}
	var assets []optimize.Asset
	points := map[string]float64{}
	for _, a := range e.data.Assets {
		if !e.data.Selectable(a) {
			continue
		}
		h, _ := a.Latest()
		p, ok := sim.ByID(a.ID)
		if !ok {
			continue
		}
		points[a.ID] = pick(p)
		assets = append(assets, optimize.Asset{
			ID: a.ID, Name: a.Name, Kind: string(a.Kind), Price: h.Price, Points: pick(p),
		})
	}
	opt := optimize.Options{
		Budget:           e.cfg.Budget,
		DriverSlots:      e.cfg.TeamDrivers,
		ConstructorSlots: e.cfg.TeamConstructors,
		CurrentTeam:      in.Team,
		FreeTransfers:    in.FreeTransfers,
		TransferPenalty:  e.cfg.TransferPenalty,
		TopN:             in.Top,
	}
	if in.Budget > 0 {
		opt.Budget = in.Budget
	}
	switch in.Chip {
	case "wildcard":
		opt.Wildcard = true
	case "limitless":
		opt.Limitless = true
	case "3x":
		opt.ExtraBoost = true
	}
	teams := optimize.Best(assets, opt)

	current := map[string]bool{}
	for _, id := range in.Team {
		current[id] = true
	}
	view := OptimizeView{
		Round: target.Round, Name: target.Name, Risk: in.Risk, Chip: in.Chip, Budget: opt.Budget,
		Projection: e.projectionView(target, sim, in.Conditions),
	}
	for _, t := range teams {
		view.Teams = append(view.Teams, teamView(t, current))
	}

	// Score the current team as-is, with the Boost on its best driver.
	if len(in.Team) > 0 {
		best := -1e18
		for _, id := range in.Team {
			view.CurrentScore += points[id]
			for _, a := range e.data.Assets {
				if a.ID == id && a.Kind == dataset.KindDriver && points[id] > best {
					best = points[id]
				}
			}
		}
		if best > -1e18 {
			view.CurrentScore += best
		}
	}
	return view, nil
}

// --- prices -----------------------------------------------------------------

// PricesView is the price predictor response.
type PricesView struct {
	Report      price.BacktestReport `json:"report"`
	Predictions []price.Prediction   `json:"predictions"`
}

// Prices predicts the next price change per asset.
func (e *Engine) Prices() PricesView {
	e.mu.RLock()
	defer e.mu.RUnlock()
	m := price.Fit(price.Examples(e.data))
	return PricesView{Report: price.Backtest(e.data), Predictions: price.Predict(e.data, m)}
}

// --- backtest ---------------------------------------------------------------

// Backtest measures the model on past rounds. Results cache per sim count.
func (e *Engine) Backtest(sims int) backtest.Report {
	if sims <= 0 {
		sims = 3000
	}
	e.cacheMu.Lock()
	rep, ok := e.backtestCache[sims]
	e.cacheMu.Unlock()
	if ok {
		return rep
	}
	e.mu.RLock()
	rep = backtest.Run(e.data, e.cfg, sims, 1)
	e.mu.RUnlock()
	e.cacheMu.Lock()
	e.backtestCache[sims] = rep
	e.cacheMu.Unlock()
	return rep
}

// --- hindsight --------------------------------------------------------------

// HindsightView is the best team for a finished round.
type HindsightView struct {
	Round int        `json:"round"`
	Name  string     `json:"name"`
	Teams []TeamView `json:"teams"`
}

// Hindsight returns the best teams for a finished round by official
// points. A zero round means the latest finished round.
func (e *Engine) Hindsight(round, top int) (HindsightView, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	completed := e.data.CompletedRounds()
	if len(completed) == 0 {
		return HindsightView{}, fmt.Errorf("no completed rounds")
	}
	target := completed[len(completed)-1]
	if round > 0 {
		found := false
		for _, c := range completed {
			if c.Round == round {
				target, found = c, true
			}
		}
		if !found {
			return HindsightView{}, fmt.Errorf("round %d has no results", round)
		}
	}
	if top <= 0 {
		top = 5
	}
	var assets []optimize.Asset
	for _, a := range e.data.Assets {
		h, ok := a.RoundHistory(target.Round)
		if !ok {
			continue
		}
		assets = append(assets, optimize.Asset{ID: a.ID, Name: a.Name, Kind: string(a.Kind), Price: h.Price, Points: h.Points})
	}
	teams := optimize.Best(assets, optimize.Options{
		Budget: e.cfg.Budget, DriverSlots: e.cfg.TeamDrivers, ConstructorSlots: e.cfg.TeamConstructors,
		TopN: top,
	})
	v := HindsightView{Round: target.Round, Name: target.Name}
	for _, t := range teams {
		v.Teams = append(v.Teams, teamView(t, nil))
	}
	return v, nil
}
