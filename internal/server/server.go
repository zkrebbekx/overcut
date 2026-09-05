// Package server exposes the toolkit as a local JSON API and serves the
// embedded web UI.
package server

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"sort"
	"strconv"
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

// Server holds the dataset and the rules, and caches expensive results.
type Server struct {
	mu       sync.RWMutex
	data     dataset.Data
	dataPath string
	cfg      rules.Config
	ui       fs.FS

	cacheMu       sync.Mutex
	simCache      map[string]model.SimResult
	backtestCache map[int]backtest.Report
}

// New returns a Server for the dataset at dataPath. ui is the built web
// app; nil disables the UI.
func New(data dataset.Data, dataPath string, cfg rules.Config, ui fs.FS) *Server {
	return &Server{
		data:          data,
		dataPath:      dataPath,
		cfg:           cfg,
		ui:            ui,
		simCache:      map[string]model.SimResult{},
		backtestCache: map[int]backtest.Report{},
	}
}

// Handler returns the HTTP handler.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/season", s.handleSeason)
	mux.HandleFunc("GET /api/rules", s.handleRules)
	mux.HandleFunc("GET /api/project", s.handleProject)
	mux.HandleFunc("POST /api/optimize", s.handleOptimize)
	mux.HandleFunc("GET /api/prices", s.handlePrices)
	mux.HandleFunc("GET /api/backtest", s.handleBacktest)
	mux.HandleFunc("GET /api/hindsight", s.handleHindsight)
	mux.HandleFunc("POST /api/sync", s.handleSync)
	if s.ui != nil {
		mux.Handle("/", spaHandler(s.ui))
	}
	return mux
}

// spaHandler serves the built app and falls back to index.html for client
// routes.
func spaHandler(ui fs.FS) http.Handler {
	files := http.FS(ui)
	server := http.FileServer(files)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}
		if _, err := fs.Stat(ui, path); err != nil {
			r.URL.Path = "/"
		}
		server.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", " ")
	_ = enc.Encode(v)
}

func writeError(w http.ResponseWriter, code int, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
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

// AssetView is one selectable asset with its history.
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

func (s *Server) seasonView() SeasonView {
	d := s.data
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
			TeamID: a.TeamID, TeamName: a.TeamName,
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

func (s *Server) handleSeason(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	writeJSON(w, s.seasonView())
}

func (s *Server) handleRules(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	writeJSON(w, s.cfg)
}

// --- projection -------------------------------------------------------------

// ConditionsInput is the known-weekend state sent by the UI. Each order is
// a list of TLAs from P1.
type ConditionsInput struct {
	Quali []string `json:"quali,omitempty"`
	Grid  []string `json:"grid,omitempty"`
	Back  []string `json:"back,omitempty"`
	FP3   []string `json:"fp3,omitempty"`
}

func (c ConditionsInput) toModel() model.Conditions {
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

func (c ConditionsInput) key() string {
	b, _ := json.Marshal(c)
	return string(b)
}

// ProjectionView is one round's projection.
type ProjectionView struct {
	Round      int               `json:"round"`
	Name       string            `json:"name"`
	HasSprint  bool              `json:"has_sprint"`
	Sims       int               `json:"sims"`
	Conditions ConditionsInput   `json:"conditions"`
	Assets     []AssetProjection `json:"assets"`
}

// AssetProjection is one asset's projected distribution plus market data.
type AssetProjection struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Kind      string  `json:"kind"`
	TLA       string  `json:"tla,omitempty"`
	TeamID    string  `json:"team_id"`
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

// simulate returns a cached or fresh simulation. The caller holds the read
// lock.
func (s *Server) simulate(round int, sims int, seed uint64, cond ConditionsInput) (dataset.Round, model.SimResult, error) {
	var target dataset.Round
	found := false
	for _, r := range s.data.Rounds {
		if r.Round == round {
			target, found = r, true
		}
	}
	if !found {
		return target, model.SimResult{}, fmt.Errorf("round %d is not on the calendar", round)
	}
	key := fmt.Sprintf("%d|%d|%d|%s", round, sims, seed, cond.key())
	s.cacheMu.Lock()
	res, ok := s.simCache[key]
	s.cacheMu.Unlock()
	if ok {
		return target, res, nil
	}
	m := model.Fit(s.data, s.cfg, round-1)
	res = m.SimulateWith(round, target.HasSprint, sims, seed, cond.toModel())
	s.cacheMu.Lock()
	s.simCache[key] = res
	s.cacheMu.Unlock()
	return target, res, nil
}

func (s *Server) projectionView(target dataset.Round, sim model.SimResult, cond ConditionsInput) ProjectionView {
	v := ProjectionView{Round: target.Round, Name: target.Name, HasSprint: target.HasSprint, Sims: sim.Sims, Conditions: cond}
	for _, a := range s.data.Assets {
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
			ID: a.ID, Name: a.Name, Kind: string(a.Kind), TLA: a.TLA, TeamID: a.TeamID,
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

func queryInt(r *http.Request, name string, def int) int {
	if v, err := strconv.Atoi(r.URL.Query().Get(name)); err == nil {
		return v
	}
	return def
}

func queryList(r *http.Request, name string) []string {
	raw := strings.TrimSpace(r.URL.Query().Get(name))
	if raw == "" {
		return nil
	}
	return strings.Split(raw, ",")
}

func (s *Server) handleProject(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	round := queryInt(r, "round", 0)
	if round == 0 {
		next, ok := s.data.NextRound()
		if !ok {
			writeError(w, http.StatusBadRequest, fmt.Errorf("the season is complete; pass round"))
			return
		}
		round = next.Round
	}
	cond := ConditionsInput{
		Quali: queryList(r, "quali"), Grid: queryList(r, "grid"),
		Back: queryList(r, "back"), FP3: queryList(r, "fp3"),
	}
	target, sim, err := s.simulate(round, queryInt(r, "sims", 20000), uint64(queryInt(r, "seed", 1)), cond)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, s.projectionView(target, sim, cond))
}

// --- optimize ---------------------------------------------------------------

// OptimizeInput is the optimizer request.
type OptimizeInput struct {
	Round         int             `json:"round"`
	Sims          int             `json:"sims"`
	Seed          uint64          `json:"seed"`
	Team          []string        `json:"team"` // asset IDs
	FreeTransfers int             `json:"free_transfers"`
	Budget        float64         `json:"budget"`
	Chip          string          `json:"chip"` // "", wildcard, limitless, 3x
	Risk          string          `json:"risk"` // mean, p10, p90
	Top           int             `json:"top"`
	Conditions    ConditionsInput `json:"conditions"`
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

func (s *Server) handleOptimize(w http.ResponseWriter, r *http.Request) {
	var in OptimizeInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("bad request body: %w", err))
		return
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	if in.Round == 0 {
		next, ok := s.data.NextRound()
		if !ok {
			writeError(w, http.StatusBadRequest, fmt.Errorf("the season is complete; pass round"))
			return
		}
		in.Round = next.Round
	}
	if in.Sims == 0 {
		in.Sims = 20000
	}
	if in.Seed == 0 {
		in.Seed = 1
	}
	if in.Top == 0 {
		in.Top = 5
	}
	if in.Risk == "" {
		in.Risk = "mean"
	}
	target, sim, err := s.simulate(in.Round, in.Sims, in.Seed, in.Conditions)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
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
	for _, a := range s.data.Assets {
		if !s.data.Selectable(a) {
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
		Budget:           s.cfg.Budget,
		DriverSlots:      s.cfg.TeamDrivers,
		ConstructorSlots: s.cfg.TeamConstructors,
		CurrentTeam:      in.Team,
		FreeTransfers:    in.FreeTransfers,
		TransferPenalty:  s.cfg.TransferPenalty,
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
		Projection: s.projectionView(target, sim, in.Conditions),
	}
	for _, t := range teams {
		tv := TeamView{
			CaptainID: t.CaptainID, BoostID: t.BoostID, Cost: t.Cost, RawPoints: t.RawPoints,
			Captain: t.Captain, Transfers: t.Transfers, Penalty: t.Penalty, Score: t.Score,
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
		view.Teams = append(view.Teams, tv)
	}

	// Score the current team as-is, with the Boost on its best driver.
	if len(in.Team) > 0 {
		best := -1e18
		for _, id := range in.Team {
			view.CurrentScore += points[id]
			for _, a := range s.data.Assets {
				if a.ID == id && a.Kind == dataset.KindDriver && points[id] > best {
					best = points[id]
				}
			}
		}
		if best > -1e18 {
			view.CurrentScore += best
		}
	}
	writeJSON(w, view)
}

// --- prices -----------------------------------------------------------------

// PricesView is the price predictor response.
type PricesView struct {
	Report      price.BacktestReport `json:"report"`
	Predictions []price.Prediction   `json:"predictions"`
}

func (s *Server) handlePrices(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	m := price.Fit(price.Examples(s.data))
	writeJSON(w, PricesView{Report: price.Backtest(s.data), Predictions: price.Predict(s.data, m)})
}

// --- backtest ---------------------------------------------------------------

func (s *Server) handleBacktest(w http.ResponseWriter, r *http.Request) {
	sims := queryInt(r, "sims", 3000)
	s.cacheMu.Lock()
	rep, ok := s.backtestCache[sims]
	s.cacheMu.Unlock()
	if !ok {
		s.mu.RLock()
		rep = backtest.Run(s.data, s.cfg, sims, 1)
		s.mu.RUnlock()
		s.cacheMu.Lock()
		s.backtestCache[sims] = rep
		s.cacheMu.Unlock()
	}
	writeJSON(w, rep)
}

// --- hindsight --------------------------------------------------------------

func (s *Server) handleHindsight(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	completed := s.data.CompletedRounds()
	if len(completed) == 0 {
		writeError(w, http.StatusBadRequest, fmt.Errorf("no completed rounds"))
		return
	}
	target := completed[len(completed)-1]
	if round := queryInt(r, "round", 0); round > 0 {
		found := false
		for _, c := range completed {
			if c.Round == round {
				target, found = c, true
			}
		}
		if !found {
			writeError(w, http.StatusBadRequest, fmt.Errorf("round %d has no results", round))
			return
		}
	}
	var assets []optimize.Asset
	for _, a := range s.data.Assets {
		h, ok := a.RoundHistory(target.Round)
		if !ok {
			continue
		}
		assets = append(assets, optimize.Asset{ID: a.ID, Name: a.Name, Kind: string(a.Kind), Price: h.Price, Points: h.Points})
	}
	teams := optimize.Best(assets, optimize.Options{
		Budget: s.cfg.Budget, DriverSlots: s.cfg.TeamDrivers, ConstructorSlots: s.cfg.TeamConstructors,
		TopN: queryInt(r, "top", 5),
	})
	var out []TeamView
	for _, t := range teams {
		tv := TeamView{CaptainID: t.CaptainID, Cost: t.Cost, RawPoints: t.RawPoints, Captain: t.Captain, Score: t.Score}
		for _, d := range t.Drivers {
			tv.Drivers = append(tv.Drivers, TeamAsset{ID: d.ID, Name: d.Name, Kind: d.Kind, Price: d.Price, Points: d.Points})
		}
		for _, c := range t.Constructors {
			tv.Constructors = append(tv.Constructors, TeamAsset{ID: c.ID, Name: c.Name, Kind: c.Kind, Price: c.Price, Points: c.Points})
		}
		out = append(out, tv)
	}
	writeJSON(w, map[string]any{"round": target.Round, "name": target.Name, "teams": out})
}

// --- sync -------------------------------------------------------------------

func (s *Server) handleSync(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	season := s.data.Season
	s.mu.RUnlock()
	d, err := dataset.Sync(season)
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	if err := dataset.Save(d, s.dataPath); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	s.mu.Lock()
	s.data = d
	s.mu.Unlock()
	s.cacheMu.Lock()
	s.simCache = map[string]model.SimResult{}
	s.backtestCache = map[int]backtest.Report{}
	s.cacheMu.Unlock()
	s.mu.RLock()
	defer s.mu.RUnlock()
	writeJSON(w, s.seasonView())
}
