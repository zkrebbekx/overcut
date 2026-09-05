// Package optimize finds the best fantasy team by exact enumeration.
//
// The search space is every legal team: C(22,5) driver sets times C(11,2)
// constructor pairs, about 1.4 million teams. The optimizer scores every
// one, so the result is the true optimum for the given projections — not a
// heuristic.
package optimize

import (
	"sort"
)

// Asset is one selectable asset.
type Asset struct {
	ID     string
	Name   string
	Kind   string // "driver" or "constructor"
	Price  float64
	Points float64 // projected points for the scoring round
}

// Options controls one optimization run.
type Options struct {
	// Budget is the spending cap in millions. Ignored with the Limitless
	// chip.
	Budget float64
	// DriverSlots and ConstructorSlots set the team shape.
	DriverSlots      int
	ConstructorSlots int
	// CaptainMultiplier is the DRS-boost multiplier on the best driver.
	// The default 2 doubles the captain. The 3x chip sets 3. Zero keeps
	// the default.
	CaptainMultiplier int
	// CurrentTeam holds the asset IDs of the team now. When set, the
	// optimizer charges the transfer penalty for changes beyond the free
	// allowance.
	CurrentTeam []string
	// FreeTransfers is the free transfer allowance. Ignored when
	// CurrentTeam is empty.
	FreeTransfers int
	// TransferPenalty is the points cost per extra transfer (a negative
	// number, from the rules).
	TransferPenalty int
	// Limitless removes the budget cap for this run.
	Limitless bool
	// Wildcard removes the transfer penalty for this run.
	Wildcard bool
	// TopN is the result count. Zero returns one team.
	TopN int
}

// Team is one scored team.
type Team struct {
	Drivers      []Asset
	Constructors []Asset
	CaptainID    string

	Cost      float64
	RawPoints float64 // sum of asset points without captain or penalty
	Captain   float64 // extra points from the captain multiplier
	Transfers int     // changes from the current team
	Penalty   float64 // transfer penalty (zero or negative)
	Score     float64 // RawPoints + Captain + Penalty
}

// Best enumerates every legal team and returns the TopN best by Score.
func Best(assets []Asset, opt Options) []Team {
	if opt.TopN <= 0 {
		opt.TopN = 1
	}
	if opt.CaptainMultiplier == 0 {
		opt.CaptainMultiplier = 2
	}

	var drivers, cons []Asset
	for _, a := range assets {
		switch a.Kind {
		case "driver":
			drivers = append(drivers, a)
		case "constructor":
			cons = append(cons, a)
		}
	}
	if len(drivers) < opt.DriverSlots || len(cons) < opt.ConstructorSlots {
		return nil
	}

	current := map[string]bool{}
	for _, id := range opt.CurrentTeam {
		current[id] = true
	}

	conPairs := combinations(len(cons), opt.ConstructorSlots)
	driverSets := combinations(len(drivers), opt.DriverSlots)

	// Precompute per-set aggregates.
	type agg struct {
		idx     []int
		cost    float64
		points  float64
		best    float64 // best driver points, for the captain
		keepers int     // members already on the current team
	}
	dAggs := make([]agg, 0, len(driverSets))
	for _, set := range driverSets {
		var a agg
		a.idx = set
		a.best = drivers[set[0]].Points
		for _, i := range set {
			d := drivers[i]
			a.cost += d.Price
			a.points += d.Points
			if d.Points > a.best {
				a.best = d.Points
			}
			if current[d.ID] {
				a.keepers++
			}
		}
		dAggs = append(dAggs, a)
	}
	cAggs := make([]agg, 0, len(conPairs))
	for _, set := range conPairs {
		var a agg
		a.idx = set
		for _, i := range set {
			c := cons[i]
			a.cost += c.Price
			a.points += c.Points
			if current[c.ID] {
				a.keepers++
			}
		}
		cAggs = append(cAggs, a)
	}

	slots := opt.DriverSlots + opt.ConstructorSlots
	captainX := float64(opt.CaptainMultiplier - 1)

	var top []Team
	worst := -1e18

	for _, da := range dAggs {
		for _, ca := range cAggs {
			cost := da.cost + ca.cost
			if !opt.Limitless && cost > opt.Budget+1e-9 {
				continue
			}
			raw := da.points + ca.points
			captain := captainX * da.best

			transfers := 0
			penalty := 0.0
			if len(current) > 0 {
				transfers = slots - da.keepers - ca.keepers
				if !opt.Wildcard {
					extra := transfers - opt.FreeTransfers
					if extra > 0 {
						penalty = float64(extra * opt.TransferPenalty)
					}
				}
			}
			score := raw + captain + penalty
			if len(top) == opt.TopN && score <= worst {
				continue
			}

			t := Team{
				Cost:      cost,
				RawPoints: raw,
				Captain:   captain,
				Transfers: transfers,
				Penalty:   penalty,
				Score:     score,
			}
			for _, i := range da.idx {
				t.Drivers = append(t.Drivers, drivers[i])
				if drivers[i].Points == da.best && t.CaptainID == "" {
					t.CaptainID = drivers[i].ID
				}
			}
			for _, i := range ca.idx {
				t.Constructors = append(t.Constructors, cons[i])
			}

			top = insertTop(top, t, opt.TopN)
			worst = top[len(top)-1].Score
		}
	}
	return top
}

// insertTop inserts a team into a descending top-N list.
func insertTop(top []Team, t Team, n int) []Team {
	i := sort.Search(len(top), func(i int) bool { return top[i].Score < t.Score })
	top = append(top, Team{})
	copy(top[i+1:], top[i:])
	top[i] = t
	if len(top) > n {
		top = top[:n]
	}
	return top
}

// combinations returns every k-subset of [0, n) as index slices.
func combinations(n, k int) [][]int {
	var out [][]int
	idx := make([]int, k)
	for i := range idx {
		idx[i] = i
	}
	for {
		out = append(out, append([]int(nil), idx...))
		i := k - 1
		for i >= 0 && idx[i] == n-k+i {
			i--
		}
		if i < 0 {
			return out
		}
		idx[i]++
		for j := i + 1; j < k; j++ {
			idx[j] = idx[j-1] + 1
		}
	}
}
