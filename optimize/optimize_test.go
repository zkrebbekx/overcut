package optimize

import (
	"fmt"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

// field builds a synthetic asset pool: nd drivers and nc constructors with
// rising prices and points.
func field(nd, nc int) []Asset {
	var out []Asset
	for i := 0; i < nd; i++ {
		out = append(out, Asset{
			ID: fmt.Sprintf("d%d", i), Name: fmt.Sprintf("D%d", i), Kind: "driver",
			Price: 5 + float64(i), Points: float64(10 + i*2),
		})
	}
	for i := 0; i < nc; i++ {
		out = append(out, Asset{
			ID: fmt.Sprintf("c%d", i), Name: fmt.Sprintf("C%d", i), Kind: "constructor",
			Price: 10 + float64(i)*2, Points: float64(20 + i*5),
		})
	}
	return out
}

func TestBest(t *testing.T) {
	Convey("Given a field of eight drivers and four constructors", t, func() {
		assets := field(8, 4)

		Convey("When the budget covers every asset", func() {
			teams := Best(assets, Options{Budget: 1000, DriverSlots: 5, ConstructorSlots: 2, TopN: 1})

			Convey("Then the optimizer picks the five best drivers and the two best constructors", func() {
				So(teams, ShouldHaveLength, 1)
				t := teams[0]
				So(t.RawPoints, ShouldEqual, (16+18+20+22+24)+(30+35))
				Convey("And the captain doubles the best driver", func() {
					So(t.CaptainID, ShouldEqual, "d7")
					So(t.Captain, ShouldEqual, 24)
					So(t.Score, ShouldEqual, t.RawPoints+24)
				})
			})
		})

		Convey("When the budget forces a compromise", func() {
			teams := Best(assets, Options{Budget: 60, DriverSlots: 5, ConstructorSlots: 2, TopN: 1})

			Convey("Then every returned team fits the budget", func() {
				So(teams, ShouldHaveLength, 1)
				So(teams[0].Cost, ShouldBeLessThanOrEqualTo, 60)
			})

			Convey("And no legal team beats the returned score", func() {
				best := bruteForce(assets, 60)
				So(teams[0].Score, ShouldEqual, best)
			})
		})

		Convey("When the current team differs by three assets and two transfers are free", func() {
			teams := Best(assets, Options{
				Budget: 1000, DriverSlots: 5, ConstructorSlots: 2, TopN: 1,
				CurrentTeam:     []string{"d0", "d1", "d2", "d3", "d4", "c0", "c1"},
				FreeTransfers:   2,
				TransferPenalty: -10,
			})

			Convey("Then the optimizer charges the penalty on the extra transfers", func() {
				t := teams[0]
				So(t.Transfers, ShouldBeGreaterThan, 0)
				expected := t.Transfers - 2
				So(t.Penalty, ShouldEqual, float64(-10*expected))
			})
		})

		Convey("When the wildcard chip is set", func() {
			teams := Best(assets, Options{
				Budget: 1000, DriverSlots: 5, ConstructorSlots: 2, TopN: 1,
				CurrentTeam:     []string{"d0", "d1", "d2", "d3", "d4", "c0", "c1"},
				FreeTransfers:   2,
				TransferPenalty: -10,
				Wildcard:        true,
			})
			Convey("Then no transfer penalty applies", func() {
				So(teams[0].Penalty, ShouldEqual, 0)
			})
		})

		Convey("When the limitless chip is set with a tiny budget", func() {
			teams := Best(assets, Options{
				Budget: 1, DriverSlots: 5, ConstructorSlots: 2, TopN: 1, Limitless: true,
			})
			Convey("Then the optimizer ignores the budget and returns the best team", func() {
				So(teams[0].RawPoints, ShouldEqual, (16+18+20+22+24)+(30+35))
			})
		})

		Convey("When the 3x captain chip is set", func() {
			teams := Best(assets, Options{
				Budget: 1000, DriverSlots: 5, ConstructorSlots: 2, TopN: 1,
				CaptainMultiplier: 3,
			})
			Convey("Then the captain bonus doubles the extra", func() {
				So(teams[0].Captain, ShouldEqual, 48)
			})
		})

		Convey("When five teams are requested", func() {
			teams := Best(assets, Options{Budget: 1000, DriverSlots: 5, ConstructorSlots: 2, TopN: 5})
			Convey("Then the scores come in descending order", func() {
				So(teams, ShouldHaveLength, 5)
				for i := 1; i < len(teams); i++ {
					So(teams[i].Score, ShouldBeLessThanOrEqualTo, teams[i-1].Score)
				}
			})
		})
	})
}

// bruteForce checks every team with an independent enumeration and returns
// the best score.
func bruteForce(assets []Asset, budget float64) float64 {
	var drivers, cons []Asset
	for _, a := range assets {
		if a.Kind == "driver" {
			drivers = append(drivers, a)
		} else {
			cons = append(cons, a)
		}
	}
	best := -1e18
	nd, nc := len(drivers), len(cons)
	for mask := 0; mask < 1<<nd; mask++ {
		if popcount(mask) != 5 {
			continue
		}
		for cm := 0; cm < 1<<nc; cm++ {
			if popcount(cm) != 2 {
				continue
			}
			cost, pts, top := 0.0, 0.0, -1e18
			for i := 0; i < nd; i++ {
				if mask&(1<<i) != 0 {
					cost += drivers[i].Price
					pts += drivers[i].Points
					if drivers[i].Points > top {
						top = drivers[i].Points
					}
				}
			}
			for i := 0; i < nc; i++ {
				if cm&(1<<i) != 0 {
					cost += cons[i].Price
					pts += cons[i].Points
				}
			}
			if cost > budget {
				continue
			}
			if s := pts + top; s > best {
				best = s
			}
		}
	}
	return best
}

func popcount(v int) int {
	n := 0
	for ; v != 0; v &= v - 1 {
		n++
	}
	return n
}
