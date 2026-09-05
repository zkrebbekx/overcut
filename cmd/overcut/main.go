// Command overcut is an F1 Fantasy analysis toolkit.
//
// Run `overcut help` for the command list.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"

	"net/http"

	"github.com/zkrebbekx/overcut/backtest"
	"github.com/zkrebbekx/overcut/internal/dataset"
	"github.com/zkrebbekx/overcut/internal/engine"
	"github.com/zkrebbekx/overcut/internal/server"
	"github.com/zkrebbekx/overcut/model"
	"github.com/zkrebbekx/overcut/optimize"
	"github.com/zkrebbekx/overcut/price"
	"github.com/zkrebbekx/overcut/rules"
	"github.com/zkrebbekx/overcut/web"
)

const usage = `overcut — F1 Fantasy analysis toolkit

Usage:
  overcut sync       [-season 2026]                 download season data
  overcut project    [-sims 20000] [-round N]       project the next round
  overcut optimize   [-team VER,NOR,...] [options]  find the best team
  overcut prices                                    predict price changes
  overcut backtest   [-sims 5000]                   measure model accuracy
  overcut hindsight  [-round N] [-top 5]            best team for a past round
  overcut serve      [-addr 127.0.0.1:8080]         web UI + JSON API

Common flags:
  -data PATH    dataset file (default ~/.overcut/season<year>.json)
  -season YEAR  season (default 2026)
  -rules PATH   scoring-rules JSON override

Optimize flags:
  -team LIST    current team: driver TLAs and constructor names, comma-set
  -free N       free transfers available (default 2)
  -chip NAME    wildcard | limitless | 3x
  -budget X     budget in millions (default from rules)
  -risk MODE    mean | p10 | p90  (projection quantile to optimize)
  -top N        show the N best teams (default 5)
`

func main() {
	if len(os.Args) < 2 {
		fmt.Print(usage)
		os.Exit(2)
	}
	cmd, args := os.Args[1], os.Args[2:]
	var err error
	switch cmd {
	case "sync":
		err = cmdSync(args)
	case "project":
		err = cmdProject(args)
	case "optimize":
		err = cmdOptimize(args)
	case "prices":
		err = cmdPrices(args)
	case "backtest":
		err = cmdBacktest(args)
	case "hindsight":
		err = cmdHindsight(args)
	case "serve":
		err = cmdServe(args)
	case "help", "-h", "--help":
		fmt.Print(usage)
	default:
		fmt.Fprintf(os.Stderr, "overcut: unknown command %q\n\n%s", cmd, usage)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "overcut:", err)
		os.Exit(1)
	}
}

// commonFlags registers the flags that every command shares.
type common struct {
	data   string
	season int
	rules  string
}

func addCommon(fs *flag.FlagSet) *common {
	c := &common{}
	fs.StringVar(&c.data, "data", "", "dataset file path")
	fs.IntVar(&c.season, "season", 2026, "season year")
	fs.StringVar(&c.rules, "rules", "", "scoring-rules JSON override")
	return c
}

func (c *common) dataPath() string {
	if c.data != "" {
		return c.data
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".overcut", fmt.Sprintf("season%d.json", c.season))
}

func (c *common) loadRules() (rules.Config, error) {
	if c.rules == "" {
		return rules.Default(), nil
	}
	return rules.Load(c.rules)
}

func (c *common) loadData() (dataset.Data, error) {
	return dataset.Load(c.dataPath())
}

func cmdSync(args []string) error {
	fs := flag.NewFlagSet("sync", flag.ExitOnError)
	c := addCommon(fs)
	fs.Parse(args)

	fmt.Printf("syncing season %d…\n", c.season)
	d, err := dataset.Sync(c.season)
	if err != nil {
		return err
	}
	if err := dataset.Save(d, c.dataPath()); err != nil {
		return err
	}
	fmt.Printf("saved %s: %d rounds (%d complete), %d assets\n",
		c.dataPath(), len(d.Rounds), len(d.CompletedRounds()), len(d.Assets))
	return nil
}

// nextOrRound resolves the target round: the given round, or the next
// round without results.
func nextOrRound(d dataset.Data, round int) (dataset.Round, error) {
	if round > 0 {
		for _, r := range d.Rounds {
			if r.Round == round {
				return r, nil
			}
		}
		return dataset.Round{}, fmt.Errorf("round %d is not on the calendar", round)
	}
	r, ok := d.NextRound()
	if !ok {
		return dataset.Round{}, fmt.Errorf("the season is complete; pass -round to project a past round")
	}
	return r, nil
}

func cmdProject(args []string) error {
	fs := flag.NewFlagSet("project", flag.ExitOnError)
	c := addCommon(fs)
	sims := fs.Int("sims", 20000, "simulation count")
	round := fs.Int("round", 0, "round to project (default: next)")
	seed := fs.Uint64("seed", 1, "random seed")
	cond := condFlags(fs)
	fs.Parse(args)

	d, err := c.loadData()
	if err != nil {
		return err
	}
	cfg, err := c.loadRules()
	if err != nil {
		return err
	}
	target, err := nextOrRound(d, *round)
	if err != nil {
		return err
	}

	m := model.Fit(d, cfg, target.Round-1)
	sim := m.SimulateWith(target.Round, target.HasSprint, *sims, *seed, cond())

	sprint := ""
	if target.HasSprint {
		sprint = " (sprint weekend)"
	}
	fmt.Printf("Round %d — %s%s — %d sims\n\n", target.Round, target.Name, sprint, *sims)

	type row struct {
		p     model.Projection
		price float64
		own   float64
	}
	var rows []row
	for _, p := range sim.Assets {
		for _, a := range d.Assets {
			if a.ID == p.AssetID {
				if h, ok := a.Latest(); ok {
					rows = append(rows, row{p, h.Price, h.Ownership})
				}
			}
		}
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].p.Mean > rows[j].p.Mean })

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "ASSET\tKIND\tPRICE\txPTS\tP10\tP50\tP90\tPTS/$M\tOWN%")
	for _, r := range rows {
		fmt.Fprintf(w, "%s\t%s\t%.1f\t%.1f\t%.0f\t%.0f\t%.0f\t%.2f\t%.0f\n",
			r.p.Name, r.p.Kind, r.price, r.p.Mean, r.p.P10, r.p.P50, r.p.P90,
			r.p.Mean/r.price, r.own)
	}
	return w.Flush()
}

// condFlags registers the known-weekend flags shared by project and
// optimize, and returns a builder for the model conditions.
func condFlags(fs *flag.FlagSet) func() model.Conditions {
	quali := fs.String("quali", "", "actual qualifying order, TLAs from P1 (e.g. RUS,HAM,VER,...)")
	grid := fs.String("grid", "", "actual starting grid after penalties, TLAs from P1")
	back := fs.String("back", "", "drivers sent to the back of the grid, TLAs")
	practice := fs.String("fp3", "", "practice order as a pace prior, TLAs from P1")
	return func() model.Conditions {
		return model.Conditions{
			Quali:      orderToPositions(*quali),
			Grid:       orderToPositions(*grid),
			BackOfGrid: splitTLAs(*back),
			Practice:   orderToPositions(*practice),
		}
	}
}

func splitTLAs(s string) []string {
	var out []string
	for _, tok := range strings.Split(s, ",") {
		tok = strings.ToUpper(strings.TrimSpace(tok))
		if tok != "" {
			out = append(out, tok)
		}
	}
	return out
}

// orderToPositions converts "RUS,HAM,VER" to {RUS:1, HAM:2, VER:3}.
func orderToPositions(s string) map[string]int {
	tlas := splitTLAs(s)
	if len(tlas) == 0 {
		return nil
	}
	out := map[string]int{}
	for i, tla := range tlas {
		out[tla] = i + 1
	}
	return out
}

// resolveTeam maps user tokens (driver TLAs or constructor-name prefixes)
// to asset IDs.
func resolveTeam(d dataset.Data, team string) ([]string, error) {
	if team == "" {
		return nil, nil
	}
	var ids []string
	for _, tok := range strings.Split(team, ",") {
		tok = strings.TrimSpace(tok)
		if tok == "" {
			continue
		}
		id := ""
		for _, a := range d.Assets {
			if a.Kind == dataset.KindDriver && strings.EqualFold(a.TLA, tok) {
				id = a.ID
				break
			}
			if a.Kind == dataset.KindConstructor &&
				strings.HasPrefix(strings.ToLower(a.Name), strings.ToLower(tok)) {
				id = a.ID
				break
			}
		}
		if id == "" {
			return nil, fmt.Errorf("unknown asset %q (use a driver TLA like VER or a constructor name prefix like McLaren)", tok)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func cmdOptimize(args []string) error {
	fs := flag.NewFlagSet("optimize", flag.ExitOnError)
	c := addCommon(fs)
	sims := fs.Int("sims", 20000, "simulation count")
	round := fs.Int("round", 0, "round to optimize for (default: next)")
	seed := fs.Uint64("seed", 1, "random seed")
	team := fs.String("team", "", "current team (TLAs and constructor names)")
	free := fs.Int("free", 2, "free transfers")
	chip := fs.String("chip", "", "wildcard | limitless | 3x")
	budget := fs.Float64("budget", 0, "budget in millions (default from rules)")
	risk := fs.String("risk", "mean", "mean | p10 | p90")
	top := fs.Int("top", 5, "teams to show")
	cond := condFlags(fs)
	fs.Parse(args)

	d, err := c.loadData()
	if err != nil {
		return err
	}
	cfg, err := c.loadRules()
	if err != nil {
		return err
	}
	target, err := nextOrRound(d, *round)
	if err != nil {
		return err
	}
	current, err := resolveTeam(d, *team)
	if err != nil {
		return err
	}

	m := model.Fit(d, cfg, target.Round-1)
	sim := m.SimulateWith(target.Round, target.HasSprint, *sims, *seed, cond())

	pick := func(p model.Projection) float64 {
		switch *risk {
		case "p10":
			return p.P10
		case "p90":
			return p.P90
		default:
			return p.Mean
		}
	}

	var assets []optimize.Asset
	for _, a := range d.Assets {
		h, ok := a.Latest()
		if !ok {
			continue
		}
		p, ok := sim.ByID(a.ID)
		if !ok {
			continue
		}
		assets = append(assets, optimize.Asset{
			ID: a.ID, Name: a.Name, Kind: string(a.Kind),
			Price: h.Price, Points: pick(p),
		})
	}

	opt := optimize.Options{
		Budget:           cfg.Budget,
		DriverSlots:      cfg.TeamDrivers,
		ConstructorSlots: cfg.TeamConstructors,
		CurrentTeam:      current,
		FreeTransfers:    *free,
		TransferPenalty:  cfg.TransferPenalty,
		TopN:             *top,
	}
	if *budget > 0 {
		opt.Budget = *budget
	}
	switch *chip {
	case "wildcard":
		opt.Wildcard = true
	case "limitless":
		opt.Limitless = true
	case "3x":
		opt.ExtraBoost = true
	case "":
	default:
		return fmt.Errorf("unknown chip %q", *chip)
	}

	teams := optimize.Best(assets, opt)
	if len(teams) == 0 {
		return fmt.Errorf("no legal team fits the budget")
	}

	fmt.Printf("Round %d — %s — best teams by %s projection\n\n", target.Round, target.Name, *risk)
	for i, t := range teams {
		var parts []string
		for _, dr := range t.Drivers {
			mark := ""
			switch dr.ID {
			case t.CaptainID:
				mark = "*"
			case t.BoostID:
				mark = "+"
			}
			parts = append(parts, dr.Name+mark)
		}
		for _, cn := range t.Constructors {
			parts = append(parts, "["+cn.Name+"]")
		}
		fmt.Printf("#%d  %.1f pts  $%.1fM", i+1, t.Score, t.Cost)
		if len(current) > 0 {
			fmt.Printf("  (%d transfers, penalty %.0f)", t.Transfers, t.Penalty)
		}
		fmt.Printf("\n    %s\n", strings.Join(parts, ", "))
	}
	if opt.ExtraBoost {
		fmt.Println("\n* = x3 chip, + = regular 2x Boost")
	} else {
		fmt.Println("\n* = 2x Boost")
	}
	return nil
}

func cmdPrices(args []string) error {
	fs := flag.NewFlagSet("prices", flag.ExitOnError)
	c := addCommon(fs)
	fs.Parse(args)

	d, err := c.loadData()
	if err != nil {
		return err
	}
	m := price.Fit(price.Examples(d))
	preds := price.Predict(d, m)
	rep := price.Backtest(d)

	fmt.Printf("Predicted next price changes (model MAE $%.3fM vs $%.3fM naive, %.0f%% direction hit rate on %d moves)\n\n",
		rep.MAE, rep.NaiveMAE, rep.Direction*100, rep.Moves)
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "ASSET\tKIND\tPRICE\tPREDICTED Δ")
	for _, p := range preds {
		fmt.Fprintf(w, "%s\t%s\t%.1f\t%+.2f\n", p.Name, p.Kind, p.Price, p.Change)
	}
	return w.Flush()
}

func cmdBacktest(args []string) error {
	fs := flag.NewFlagSet("backtest", flag.ExitOnError)
	c := addCommon(fs)
	sims := fs.Int("sims", 5000, "simulation count per round")
	seed := fs.Uint64("seed", 1, "random seed")
	fs.Parse(args)

	d, err := c.loadData()
	if err != nil {
		return err
	}
	cfg, err := c.loadRules()
	if err != nil {
		return err
	}
	rep := backtest.Run(d, cfg, *sims, *seed)
	if len(rep.Rounds) == 0 {
		return fmt.Errorf("not enough completed rounds to backtest")
	}

	fmt.Printf("Walk-forward backtest over %d rounds (fit on rounds before each)\n\n", len(rep.Rounds))
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "ROUND\tRACE\tDRV MAE\tCON MAE\tRANK ρ\tMODEL TEAM\tNAIVE TEAM\tHINDSIGHT")
	for _, rr := range rep.Rounds {
		fmt.Fprintf(w, "%d\t%s\t%.1f\t%.1f\t%.2f\t%.0f\t%.0f\t%.0f\n",
			rr.Round, rr.Name, rr.DriverMAE, rr.ConsMAE, rr.SpearmanRho,
			rr.ModelTeamPts, rr.NaiveTeamPts, rr.HindsightTeamPts)
	}
	w.Flush()

	fmt.Printf("\nDriver points MAE:   model %.1f | last-round baseline %.1f | season-mean baseline %.1f\n",
		rep.DriverMAE, rep.BaselinePrev, rep.BaselineSeason)
	fmt.Printf("Mean driver rank ρ:  %.2f\n", rep.MeanSpearman)
	fmt.Printf("Mean team points:    model %.0f | naive %.0f | hindsight optimum %.0f\n",
		rep.ModelTeamPts, rep.NaiveTeamPts, rep.HindsightTeamPts)
	fmt.Printf("Model captures %.0f%% of the naive→hindsight gap.\n",
		100*(rep.ModelTeamPts-rep.NaiveTeamPts)/(rep.HindsightTeamPts-rep.NaiveTeamPts))
	return nil
}

func cmdServe(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	c := addCommon(fs)
	addr := fs.String("addr", "127.0.0.1:8080", "listen address")
	fs.Parse(args)

	d, err := c.loadData()
	if err != nil {
		return err
	}
	cfg, err := c.loadRules()
	if err != nil {
		return err
	}
	srv := server.New(engine.New(d, cfg), c.dataPath(), web.FS())
	fmt.Printf("overcut serving on http://%s\n", *addr)
	return http.ListenAndServe(*addr, srv.Handler())
}

func cmdHindsight(args []string) error {
	fs := flag.NewFlagSet("hindsight", flag.ExitOnError)
	c := addCommon(fs)
	round := fs.Int("round", 0, "round (default: latest completed)")
	top := fs.Int("top", 5, "teams to show")
	fs.Parse(args)

	d, err := c.loadData()
	if err != nil {
		return err
	}
	cfg, err := c.loadRules()
	if err != nil {
		return err
	}
	completed := d.CompletedRounds()
	if len(completed) == 0 {
		return fmt.Errorf("no completed rounds")
	}
	target := completed[len(completed)-1]
	if *round > 0 {
		found := false
		for _, r := range completed {
			if r.Round == *round {
				target, found = r, true
			}
		}
		if !found {
			return fmt.Errorf("round %d has no results", *round)
		}
	}

	var assets []optimize.Asset
	for _, a := range d.Assets {
		h, ok := a.RoundHistory(target.Round)
		if !ok {
			continue
		}
		assets = append(assets, optimize.Asset{
			ID: a.ID, Name: a.Name, Kind: string(a.Kind),
			Price: h.Price, Points: h.Points,
		})
	}
	teams := optimize.Best(assets, optimize.Options{
		Budget:           cfg.Budget,
		DriverSlots:      cfg.TeamDrivers,
		ConstructorSlots: cfg.TeamConstructors,
		TopN:             *top,
	})

	fmt.Printf("Round %d — %s — hindsight-optimal teams (official points)\n\n", target.Round, target.Name)
	for i, t := range teams {
		var parts []string
		for _, dr := range t.Drivers {
			mark := ""
			if dr.ID == t.CaptainID {
				mark = "*"
			}
			parts = append(parts, fmt.Sprintf("%s%s %.0f", dr.Name, mark, dr.Points))
		}
		for _, cn := range t.Constructors {
			parts = append(parts, fmt.Sprintf("[%s] %.0f", cn.Name, cn.Points))
		}
		fmt.Printf("#%d  %.0f pts  $%.1fM\n    %s\n", i+1, t.Score, t.Cost, strings.Join(parts, ", "))
	}
	fmt.Println("\n* = captain (DRS boost)")
	return nil
}
