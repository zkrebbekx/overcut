// Package engine exposes the toolkit as transport-agnostic operations with
// JSON-shaped inputs and outputs. The HTTP server and the WebAssembly
// build both call it.
package engine

import (
	"encoding/json"
	"fmt"
	"math"
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
	// HasQuali reports that the official qualifying classification is in
	// the data; projections use it unless the caller supplies an order.
	HasQuali bool `json:"has_quali"`
	// HasGrid reports that the official starting grid, with penalties, is
	// in the data; projections use it unless the caller supplies one.
	HasGrid bool `json:"has_grid"`
	// HasSprintResult reports that the sprint classification is in the
	// data.
	HasSprintResult bool                 `json:"has_sprint_result"`
	Sessions        map[string]time.Time `json:"sessions,omitempty"`
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
			HasQuali: len(r.Quali) > 0, HasGrid: r.GridOrder() != nil, HasSprintResult: len(r.Sprint) > 0, Sessions: r.Sessions,
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
	// Sprint is the sprint classification with retired cars last;
	// SprintGrid is the sprint grid; SprintDNF lists the retired cars.
	Sprint     []string `json:"sprint,omitempty"`
	SprintGrid []string `json:"sprint_grid,omitempty"`
	SprintDNF  []string `json:"sprint_dnf,omitempty"`
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
	toSet := func(list []string) map[string]bool {
		if len(list) == 0 {
			return nil
		}
		out := map[string]bool{}
		for _, tla := range list {
			out[strings.ToUpper(strings.TrimSpace(tla))] = true
		}
		return out
	}
	var back []string
	for _, tla := range c.Back {
		back = append(back, strings.ToUpper(strings.TrimSpace(tla)))
	}
	mc := model.Conditions{
		Quali: toPos(c.Quali), Grid: toPos(c.Grid), BackOfGrid: back, Practice: toPos(c.FP3),
		SprintGrid: toPos(c.SprintGrid), SprintDNF: toSet(c.SprintDNF),
	}
	// A retired car takes no classified sprint position.
	if len(c.Sprint) > 0 {
		mc.SprintFinish = map[string]int{}
		pos := 1
		for _, tla := range c.Sprint {
			t := strings.ToUpper(strings.TrimSpace(tla))
			if mc.SprintDNF[t] {
				continue
			}
			mc.SprintFinish[t] = pos
			pos++
		}
	}
	return mc
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
	Round     int    `json:"round"`
	Name      string `json:"name"`
	HasSprint bool   `json:"has_sprint"`
	Sims      int    `json:"sims"`
	// Conditions is the state the simulation used, including any
	// qualifying order filled in from the official data.
	Conditions Conditions `json:"conditions"`
	// QualiFromData, GridFromData, and SprintFromData report which parts of
	// the weekend state came from the official data rather than the caller.
	QualiFromData  bool              `json:"quali_from_data"`
	GridFromData   bool              `json:"grid_from_data"`
	SprintFromData bool              `json:"sprint_from_data"`
	Assets         []AssetProjection `json:"assets"`
}

// known records which parts of the weekend state came from the data.
type known struct{ quali, grid, sprint bool }

// withKnownWeekend fills an empty qualifying order, grid, and sprint result
// from the official data: the published grid before the race, the race
// classification after it, the sprint classification once the sprint has
// run. The grid carries every penalty, so it takes precedence over
// back-of-grid hints.
func withKnownWeekend(target dataset.Round, cond Conditions) (Conditions, known) {
	var k known
	if len(cond.Quali) == 0 {
		if order := target.QualiOrder(); order != nil {
			cond.Quali = order
			k.quali = true
		}
	}
	if len(cond.Grid) == 0 {
		if order := target.GridOrder(); order != nil {
			cond.Grid = order
			k.grid = true
		}
	}
	if target.HasSprint && len(cond.Sprint) == 0 {
		if finish, grid, dnf := target.SprintOrder(); finish != nil {
			cond.Sprint, cond.SprintGrid = finish, grid
			cond.SprintDNF = nil
			for tla := range dnf {
				cond.SprintDNF = append(cond.SprintDNF, tla)
			}
			sort.Strings(cond.SprintDNF)
			k.sprint = true
		}
	}
	return cond, k
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
	// Actual is the official score once the round is complete.
	Actual    float64 `json:"actual_points"`
	HasActual bool    `json:"has_actual"`
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
		if target.HasResults {
			if h, ok := a.RoundHistory(target.Round); ok {
				ap.Actual, ap.HasActual = h.Points, true
			}
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
	cond, k := withKnownWeekend(target, in.Conditions)
	sim := e.simulate(target, in.Sims, in.Seed, cond)
	view := e.projectionView(target, sim, cond)
	view.QualiFromData, view.GridFromData, view.SprintFromData = k.quali, k.grid, k.sprint
	return view, nil
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
	// P10, P50, and P90 are the team's score percentiles from the joint
	// simulation, boosts and penalty included.
	P10 float64 `json:"p10"`
	P50 float64 `json:"p50"`
	P90 float64 `json:"p90"`
}

// ChipValue is the expected gain of playing one chip this round on the
// recommended team.
type ChipValue struct {
	Chip      string  `json:"chip"`
	Label     string  `json:"label"`
	Gain      float64 `json:"gain"`
	Available bool    `json:"available"`
	Note      string  `json:"note"`
	// Final Fix: the swap that gives the gain.
	OutID string `json:"out_id,omitempty"`
	InID  string `json:"in_id,omitempty"`
}

// teamSamples sums the joint samples of a team, with the boosts and the
// transfer penalty, under normal or No Negative scoring.
func teamSamples(sim model.SimResult, t optimize.Team, noNegative bool) []float64 {
	src := sim.Samples
	if noNegative {
		src = sim.SamplesNN
	}
	n := sim.Sims
	out := make([]float64, n)
	add := func(id string, mult float64) {
		s := src[id]
		if len(s) != n {
			return
		}
		for i := range out {
			out[i] += mult * s[i]
		}
	}
	for _, d := range t.Drivers {
		mult := 1.0
		if d.ID == t.CaptainID {
			if t.BoostID != "" {
				mult = 3
			} else {
				mult = 2
			}
		} else if d.ID == t.BoostID {
			mult = 2
		}
		add(d.ID, mult)
	}
	for _, c := range t.Constructors {
		add(c.ID, 1)
	}
	for i := range out {
		out[i] += t.Penalty
	}
	return out
}

// autopilotGain is the expected gain of assigning the Boost after the race
// to the team's best driver, over the pre-chosen captain.
func autopilotGain(sim model.SimResult, t optimize.Team) float64 {
	n := sim.Sims
	if n == 0 {
		return 0
	}
	var gain float64
	for i := 0; i < n; i++ {
		best := math.Inf(-1)
		chosen := 0.0
		for _, d := range t.Drivers {
			s := sim.Samples[d.ID]
			if len(s) != n {
				continue
			}
			if s[i] > best {
				best = s[i]
			}
			if d.ID == t.CaptainID {
				chosen = s[i]
			}
		}
		if best > math.Inf(-1) {
			gain += best - chosen
		}
	}
	return gain / float64(n)
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
	// Chips values every chip against the best team without a chip.
	Chips []ChipValue `json:"chips"`
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
	cond, k := withKnownWeekend(target, in.Conditions)
	in.Conditions = cond
	sim := e.simulate(target, in.Sims, in.Seed, cond)
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
	// assetsFor builds the optimizer's asset list from the projection,
	// under normal or No Negative scoring.
	assetsFor := func(noNegative bool) ([]optimize.Asset, map[string]float64) {
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
			v := pick(p)
			if noNegative {
				v = p.MeanNN
			}
			points[a.ID] = v
			assets = append(assets, optimize.Asset{
				ID: a.ID, Name: a.Name, Kind: string(a.Kind), Price: h.Price, Points: v,
			})
		}
		return assets, points
	}
	baseOpt := optimize.Options{
		Budget:           e.cfg.Budget,
		DriverSlots:      e.cfg.TeamDrivers,
		ConstructorSlots: e.cfg.TeamConstructors,
		CurrentTeam:      in.Team,
		FreeTransfers:    in.FreeTransfers,
		TransferPenalty:  e.cfg.TransferPenalty,
		TopN:             in.Top,
	}
	if in.Budget > 0 {
		baseOpt.Budget = in.Budget
	}
	withChip := func(chip string) optimize.Options {
		o := baseOpt
		switch chip {
		case "wildcard":
			o.Wildcard = true
		case "limitless":
			o.Limitless = true
		case "3x":
			o.ExtraBoost = true
		}
		return o
	}
	noNegative := in.Chip == "nonegative"
	assets, points := assetsFor(noNegative)
	teams := optimize.Best(assets, withChip(in.Chip))

	current := map[string]bool{}
	for _, id := range in.Team {
		current[id] = true
	}
	view := OptimizeView{
		Round: target.Round, Name: target.Name, Risk: in.Risk, Chip: in.Chip, Budget: baseOpt.Budget,
		Projection: e.projectionView(target, sim, cond),
	}
	view.Projection.QualiFromData, view.Projection.GridFromData, view.Projection.SprintFromData = k.quali, k.grid, k.sprint
	for _, t := range teams {
		tv := teamView(t, current)
		if s := teamSamples(sim, t, noNegative); len(s) > 0 {
			p := model.Summarize(s)
			tv.P10, tv.P50, tv.P90 = p.P10, p.P50, p.P90
		}
		view.Teams = append(view.Teams, tv)
	}

	// Value every chip against the best team without a chip.
	plainAssets, _ := assetsFor(false)
	var base optimize.Team
	if in.Chip == "" && len(teams) > 0 {
		base = teams[0]
	} else if bt := optimize.Best(plainAssets, withChip("")); len(bt) > 0 {
		base = bt[0]
	}
	if len(base.Drivers) > 0 {
		view.Chips = e.chipValues(target, sim, cond, base, plainAssets, withChip, assetsFor, baseOpt)
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

// chipValues computes the expected gain of each chip against the base
// team. The caller holds the read lock.
func (e *Engine) chipValues(target dataset.Round, sim model.SimResult, cond Conditions, base optimize.Team,
	plainAssets []optimize.Asset, withChip func(string) optimize.Options,
	assetsFor func(bool) ([]optimize.Asset, map[string]float64), baseOpt optimize.Options) []ChipValue {

	baseScore := base.Score
	var out []ChipValue

	// No Negative: the same team scored with every negative category
	// floored at zero.
	nnSamples := teamSamples(sim, base, true)
	plainSamples := teamSamples(sim, base, false)
	out = append(out, ChipValue{
		Chip: "nonegative", Label: "No Negative", Available: true,
		Gain: mean(nnSamples) - mean(plainSamples),
		Note: "Floors every negative scoring category at zero for each asset on the team.",
	})

	// x3 Boost: the best team with a tripled driver and the Boost on
	// another.
	if bt := optimize.Best(plainAssets, withChip("3x")); len(bt) > 0 {
		out = append(out, ChipValue{
			Chip: "3x", Label: "x3 Boost", Available: true, Gain: bt[0].Score - baseScore,
			Note: "Triples one driver; the regular Boost moves to another.",
		})
	}

	// Autopilot: the Boost lands on the best actual scorer.
	out = append(out, ChipValue{
		Chip: "autopilot", Label: "Autopilot", Available: true, Gain: autopilotGain(sim, base),
		Note: "The Boost moves to your top scorer after the race.",
	})

	// Wildcard and Limitless: what unlimited transfers or no cost cap add.
	hasTeam := len(baseOpt.CurrentTeam) > 0
	if bt := optimize.Best(plainAssets, withChip("wildcard")); len(bt) > 0 {
		cv := ChipValue{Chip: "wildcard", Label: "Wildcard", Available: hasTeam, Gain: bt[0].Score - baseScore,
			Note: "Unlimited transfers within the cost cap."}
		if !hasTeam {
			cv.Note = "Enter your current team to value unlimited transfers."
			cv.Gain = 0
		}
		out = append(out, cv)
	}
	if bt := optimize.Best(plainAssets, withChip("limitless")); len(bt) > 0 {
		out = append(out, ChipValue{
			Chip: "limitless", Label: "Limitless", Available: true, Gain: bt[0].Score - baseScore,
			Note: "No cost cap and unlimited transfers for one round; the team restores after.",
		})
	}

	// Final Fix: one driver swap after qualifying. The incoming driver
	// scores the race only; the outgoing driver keeps the qualifying
	// points. Only valued once the qualifying result is known.
	ff := ChipValue{Chip: "finalfix", Label: "Final Fix", Note: "Available after qualifying: swap one driver before the race."}
	if len(cond.Quali) > 0 && !target.HasResults {
		qualiPts := func(id string) float64 {
			for _, a := range e.data.Assets {
				if a.ID == id {
					for i, tla := range cond.Quali {
						if strings.EqualFold(tla, a.TLA) {
							if i < len(e.cfg.QualiPoints) {
								return float64(e.cfg.QualiPoints[i])
							}
							return 0
						}
					}
					return float64(e.cfg.QualiNoTime)
				}
			}
			return 0
		}
		raceLeg := func(id string) float64 {
			p, _ := sim.ByID(id)
			return p.Mean - qualiPts(id)
		}
		held := map[string]bool{}
		var cost float64
		for _, d := range base.Drivers {
			held[d.ID] = true
			cost += d.Price
		}
		for _, c := range base.Constructors {
			cost += c.Price
		}
		spare := baseOpt.Budget - cost
		bestGain := 0.0
		for _, outD := range base.Drivers {
			mult := 1.0
			if outD.ID == base.CaptainID {
				mult = 2
			}
			for _, inA := range plainAssets {
				if inA.Kind != "driver" || held[inA.ID] || inA.Price > outD.Price+spare+1e-9 {
					continue
				}
				gain := (raceLeg(inA.ID) - raceLeg(outD.ID)) * mult
				if gain > bestGain {
					bestGain, ff.OutID, ff.InID = gain, outD.ID, inA.ID
				}
			}
		}
		ff.Available = true
		ff.Gain = bestGain
		if ff.InID == "" {
			ff.Note = "No swap improves the race-only projection. Keep the chip."
		} else {
			ff.Note = "The incoming driver scores the race only; the outgoing driver keeps qualifying points."
		}
	}
	out = append(out, ff)

	sort.SliceStable(out, func(i, j int) bool { return out[i].Gain > out[j].Gain })
	return out
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

// --- review -----------------------------------------------------------------

// ReviewInput asks for a post-round review. A zero Round means the latest
// finished round. Team is optional: the asset IDs the player held.
type ReviewInput struct {
	Round int      `json:"round"`
	Sims  int      `json:"sims"`
	Seed  uint64   `json:"seed"`
	Team  []string `json:"team"`
	// Captain is the driver that carried the Boost. When empty, the
	// review assumes the Boost was on the team's best projected driver.
	Captain string `json:"captain"`
}

// ReviewAsset is one asset's projection against its actual score.
type ReviewAsset struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Kind      string  `json:"kind"`
	TLA       string  `json:"tla,omitempty"`
	TeamName  string  `json:"team_name"`
	Price     float64 `json:"price"`
	Ownership float64 `json:"ownership"`
	Projected float64 `json:"projected"`
	SD        float64 `json:"sd"`
	P10       float64 `json:"p10"`
	P90       float64 `json:"p90"`
	Actual    float64 `json:"actual"`
	Delta     float64 `json:"delta"` // actual minus projected
	Z         float64 `json:"z"`     // delta in standard deviations
	InRange   bool    `json:"in_range"`
	Held      bool    `json:"held"` // on the reviewed team
}

// ReviewView is the post-round review.
type ReviewView struct {
	Round     int           `json:"round"`
	Name      string        `json:"name"`
	HasSprint bool          `json:"has_sprint"`
	Sims      int           `json:"sims"`
	Assets    []ReviewAsset `json:"assets"`   // ordered by |delta|, largest first
	Coverage  float64       `json:"coverage"` // share of drivers inside P10–P90
	DriverMAE float64       `json:"driver_mae"`

	// Team review, when a team was given.
	TeamProjected float64 `json:"team_projected"`
	TeamActual    float64 `json:"team_actual"`
	CaptainID     string  `json:"captain_id,omitempty"`
	// Hindsight is the best team that was possible at the round's prices.
	HindsightPts float64 `json:"hindsight_points"`
}

// Review compares the grid-known projection of a finished round with the
// official points.
func (e *Engine) Review(in ReviewInput) (ReviewView, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	completed := e.data.CompletedRounds()
	if len(completed) == 0 {
		return ReviewView{}, fmt.Errorf("no completed rounds")
	}
	target := completed[len(completed)-1]
	if in.Round > 0 {
		found := false
		for _, c := range completed {
			if c.Round == in.Round {
				target, found = c, true
			}
		}
		if !found {
			return ReviewView{}, fmt.Errorf("round %d has no results", in.Round)
		}
	}
	cond, _ := withKnownWeekend(target, Conditions{})
	sim := e.simulate(target, in.Sims, in.Seed, cond)

	held := map[string]bool{}
	for _, id := range in.Team {
		held[id] = true
	}
	v := ReviewView{Round: target.Round, Name: target.Name, HasSprint: target.HasSprint, Sims: sim.Sims}
	var drivers, inRange float64
	bestHeld := math.Inf(-1)
	for _, a := range e.data.Assets {
		h, ok := a.RoundHistory(target.Round)
		if !ok {
			continue
		}
		p, ok := sim.ByID(a.ID)
		if !ok {
			continue
		}
		ra := ReviewAsset{
			ID: a.ID, Name: a.Name, Kind: string(a.Kind), TLA: a.TLA, TeamName: teamName(a),
			Price: h.Price, Ownership: h.Ownership,
			Projected: p.Mean, SD: p.SD, P10: p.P10, P90: p.P90, Actual: h.Points,
			Delta: h.Points - p.Mean, Held: held[a.ID],
		}
		if p.SD > 0 {
			ra.Z = ra.Delta / p.SD
		}
		ra.InRange = h.Points >= p.P10 && h.Points <= p.P90
		if a.Kind == dataset.KindDriver {
			drivers++
			v.DriverMAE += math.Abs(ra.Delta)
			if ra.InRange {
				inRange++
			}
			if held[a.ID] && (in.Captain == a.ID || (in.Captain == "" && p.Mean > bestHeld)) {
				if in.Captain == "" {
					bestHeld = p.Mean
				}
				v.CaptainID = a.ID
			}
		}
		if held[a.ID] {
			v.TeamProjected += p.Mean
			v.TeamActual += h.Points
		}
		v.Assets = append(v.Assets, ra)
	}
	if drivers > 0 {
		v.Coverage = inRange / drivers
		v.DriverMAE /= drivers
	}
	if v.CaptainID != "" {
		for _, ra := range v.Assets {
			if ra.ID == v.CaptainID {
				v.TeamProjected += ra.Projected
				v.TeamActual += ra.Actual
			}
		}
	}
	sort.Slice(v.Assets, func(i, j int) bool { return math.Abs(v.Assets[i].Delta) > math.Abs(v.Assets[j].Delta) })

	if hv, err := e.hindsight(target, 1); err == nil && len(hv.Teams) > 0 {
		v.HindsightPts = hv.Teams[0].Score
	}
	return v, nil
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
	return e.hindsight(target, top)
}

// hindsight enumerates the best teams for a finished round. The caller
// holds the read lock.
func (e *Engine) hindsight(target dataset.Round, top int) (HindsightView, error) {
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
